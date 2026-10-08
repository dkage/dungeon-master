import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { expect, test, type Page } from '@playwright/test'

const here = dirname(fileURLToPath(import.meta.url))
const { url, workdir, subdir, plaindir } = JSON.parse(readFileSync(join(here, '.url.json'), 'utf8')) as {
  url: string
  workdir: string
  subdir: string
  plaindir: string
}

// The close button of an expedition (not the quest drawer's).
const closeSession = /^Close (?!quests)/
const nav = (page: Page) => page.getByRole('navigation', { name: 'Campaigns' })
const hero = (page: Page, name: string) => page.getByRole('button', { name, exact: true })
const screenText = (page: Page) => page.locator('.pane:not([hidden]) .xterm-rows')
const expeditions = (page: Page, campaign = 'demo') =>
  page.getByRole('list', { name: `Expeditions in ${campaign}` })

async function addCampaign(page: Page, dir: string, name: string) {
  await page.getByPlaceholder('/absolute/path/to/project').fill(dir)
  await page.getByPlaceholder(/defaults to the folder/).fill(name)
  await page.getByRole('button', { name: 'Add campaign' }).click()
}

// Opens the app signed in, with the "demo" campaign registered and selected,
// and no leftover sessions. The tests share one server, so each starts clean.
async function open(page: Page) {
  await page.goto(url) // exchanges the token for a cookie and redirects
  await expect(hero(page, 'Opus')).toBeVisible()
  if ((await nav(page).getByRole('button', { name: /^demo/ }).count()) === 0) {
    await addCampaign(page, workdir, 'demo')
    await expect(nav(page).getByText('acme/demo')).toBeVisible()
  }
  await nav(page).getByRole('button', { name: /^demo/ }).click()
  await expect(hero(page, 'Opus')).toBeEnabled()
  const close = page.getByRole('button', { name: closeSession })
  while ((await close.count()) > 0) {
    const before = await close.count()
    await close.first().click()
    await expect(close).toHaveCount(before - 1)
  }
}

test('unauthenticated API access is refused, the page itself loads', async ({ playwright, baseURL }) => {
  const anon = await playwright.request.newContext({ baseURL })
  expect((await anon.get('/api/state')).status()).toBe(401)
  expect((await anon.get('/')).status()).toBe(200)
  await anon.dispose()
})

test('a campaign shows the GitHub repo resolved from its git remote', async ({ page }) => {
  await open(page)
  await expect(nav(page).getByText('acme/demo')).toBeVisible()
  await expect(page.locator('.toolbar')).toContainText('acme/demo')
  await expect(page.locator('.toolbar')).toContainText(workdir)
})

test('a monorepo subfolder runs agents in the subfolder but uses the repository remote', async ({ page }) => {
  await open(page)
  await addCampaign(page, subdir, 'api')
  await expect(page.locator('.toolbar')).toContainText('api')
  await expect(page.locator('.toolbar')).toContainText('acme/demo') // from the git root's remote

  await hero(page, 'Opus').click()
  await expect(screenText(page)).toContainText('FAKE-CLAUDE')
  await expect(screenText(page)).toContainText('apps/api') // cwd is the subfolder

  // Clean up so later tests start from one campaign.
  await page.getByRole('button', { name: closeSession }).click()
  await page.getByRole('button', { name: 'Remove campaign api' }).click()
  await page.getByRole('button', { name: 'Remove?' }).click()
  await expect(nav(page).getByRole('button', { name: /^api/ })).toHaveCount(0)
})

test('adding the same directory twice is refused', async ({ page }) => {
  await open(page)
  await addCampaign(page, workdir, 'again')
  await expect(page.getByRole('alert')).toContainText('already a campaign')
  await expect(nav(page).getByRole('button', { name: /^again/ })).toHaveCount(0)
})

test('launch a hero, see its output, type to it', async ({ page }) => {
  await open(page)
  await hero(page, 'Opus').click()

  const term = screenText(page)
  await expect(term).toContainText('FAKE-CLAUDE')
  await expect(term).toContainText('arg=[--model]')
  await expect(term).toContainText('arg=[claude-opus-5-5]')
  await expect(term).toContainText('cwd=')

  await page.locator('.pane:not([hidden]) .xterm').click()
  await page.keyboard.type('hello dungeon')
  await page.keyboard.press('Enter')
  await expect(term).toContainText('echo:hello dungeon')
})

test('two expeditions are independent and a shell exit is shown in the list', async ({ page }) => {
  await open(page)
  await hero(page, 'Opus').click()
  await expect(screenText(page)).toContainText('FAKE-CLAUDE')

  await hero(page, 'Shell').click()
  await expect(expeditions(page).getByRole('listitem')).toHaveCount(2)
  await page.locator('.pane:not([hidden]) .xterm').click()
  await page.keyboard.type('echo shell-only-marker')
  await page.keyboard.press('Enter')
  await expect(screenText(page)).toContainText('shell-only-marker')
  await expect(screenText(page)).not.toContainText('FAKE-CLAUDE')

  // Back to the hero: its own output, none of the shell's.
  await expeditions(page).getByRole('button', { name: /^Opus/ }).click()
  await expect(screenText(page)).toContainText('FAKE-CLAUDE')
  await expect(screenText(page)).not.toContainText('shell-only-marker')

  // Exit the shell: it is marked in the list and keeps its scrollback.
  await expeditions(page).getByRole('button', { name: /^shell/ }).click()
  await page.locator('.pane:not([hidden]) .xterm').click()
  await page.keyboard.type('exit 3')
  await page.keyboard.press('Enter')
  await expect(expeditions(page).getByText('exit 3')).toBeVisible()
  await expect(expeditions(page).getByText('exited with code 3')).toBeAttached()
  await expect(screenText(page)).toContainText('shell-only-marker')
})

test('a campaign with a running expedition cannot be removed', async ({ page }) => {
  await open(page)
  await addCampaign(page, subdir, 'busy')
  await hero(page, 'Shell').click()
  await expect(expeditions(page, 'busy').getByRole('listitem')).toHaveCount(1)

  await page.getByRole('button', { name: 'Remove campaign busy' }).click()
  await page.getByRole('button', { name: 'Remove?' }).click()
  await expect(page.getByRole('alert')).toContainText('running sessions')
  await expect(nav(page).getByRole('button', { name: /^busy/ })).toBeVisible()

  // Close the expedition, then removal succeeds.
  await page.getByRole('button', { name: closeSession }).click()
  await page.getByRole('button', { name: 'Remove campaign busy' }).click()
  await page.getByRole('button', { name: 'Remove?' }).click()
  await expect(nav(page).getByRole('button', { name: /^busy/ })).toHaveCount(0)
})

test('closing an expedition ends it on the server', async ({ page }) => {
  await open(page)
  await hero(page, 'Shell').click()
  await expect(expeditions(page).getByRole('listitem')).toHaveCount(1)
  await page.getByRole('button', { name: closeSession }).click()
  await expect(page.getByRole('list', { name: /^Expeditions in/ })).toHaveCount(0)
  await page.reload()
  await expect(hero(page, 'Opus')).toBeEnabled()
  await expect(page.getByRole('list', { name: /^Expeditions in/ })).toHaveCount(0)
})

test('campaigns and expeditions survive a page reload, with scrollback replayed', async ({ page }) => {
  await open(page)
  await hero(page, 'Opus').click()
  await expect(screenText(page)).toContainText('FAKE-CLAUDE')
  await page.reload()
  await expect(nav(page).getByRole('button', { name: /^demo/ })).toBeVisible()
  await expect(expeditions(page).getByRole('button', { name: /^Opus/ })).toBeVisible()
  await expect(screenText(page)).toContainText('FAKE-CLAUDE')
})

test('the terminal renders with its own font, colours and selection (CSP must allow xterm styles)', async ({ page }) => {
  // xterm builds its font, colour and selection rules as <style> elements. A
  // Content-Security-Policy that blocks them leaves a proportional font, no
  // colours and no selection highlight, so fail on any CSP violation too.
  const violations: string[] = []
  page.on('console', (m) => {
    if (/content security policy/i.test(m.text())) violations.push(m.text())
  })
  await open(page)
  await hero(page, 'Shell').click()
  await page.locator('.pane:not([hidden]) .xterm').click()
  await page.keyboard.type("printf '\\033[31mRED-TEXT\\033[0m plain-text\\n'")
  await page.keyboard.press('Enter')
  await expect(screenText(page)).toContainText('RED-TEXT plain-text')

  const font = await screenText(page).evaluate((e) => getComputedStyle(e).fontFamily)
  expect(font).toMatch(/monospace|Menlo/)

  const colour = (text: RegExp) =>
    page.locator('.pane:not([hidden]) .xterm-rows span').filter({ hasText: text }).last()
      .evaluate((e) => getComputedStyle(e).color)
  const red = await colour(/^RED-TEXT$/)
  const plain = await colour(/plain-text$/)
  expect(red).not.toBe(plain)
  expect(red).toMatch(/rgb\((\d+), (\d+), (\d+)\)/)
  const [r, g, b] = red.match(/\d+/g)!.map(Number)
  expect(r).toBeGreaterThan(g + 40) // actually red, not white or grey
  expect(r).toBeGreaterThan(b + 40)

  // Dragging over the output paints a visible selection.
  const box = (await page.locator('.pane:not([hidden]) .xterm-screen').boundingBox())!
  await page.mouse.move(box.x + 4, box.y + 6)
  await page.mouse.down()
  await page.mouse.move(box.x + 300, box.y + 40, { steps: 6 })
  await page.mouse.up()
  const sel = page.locator('.pane:not([hidden]) .xterm-selection div').first()
  await expect(sel).toBeAttached()
  expect(await sel.evaluate((e) => getComputedStyle(e).backgroundColor)).not.toBe('rgba(0, 0, 0, 0)')

  expect(violations).toEqual([])
})

// --- quests ---

const questPanel = (page: Page) => page.getByRole('complementary', { name: 'Quests' })

test("the quest panel lists the repository's issues with their states and relationships", async ({ page }) => {
  await open(page)
  const panel = questPanel(page)
  await expect(panel.getByText('acme/demo')).toBeVisible()
  await expect(panel.getByRole('button', { name: /#1 \[demo\] Epic/ })).toBeVisible()
  await expect(panel.getByRole('button', { name: /#5 \[demo\] Bug/ })).toContainText('@dkage') // assigned
  await expect(panel.getByRole('button', { name: /#3 \[demo\] Render rooms/ })).toContainText('blocked by #2') // locked
  await expect(panel.getByTitle('Sub-issues done')).toHaveText('0/2')
  await expect(panel.getByLabel('Quest counts')).toContainText('1 locked')
  await expect(panel.getByLabel('Quest counts')).toContainText('1 in progress')

  // The two sub-issues are nested under the epic.
  const rows = await panel.getByRole('listitem').evaluateAll((els) =>
    els.map((e) => [e.textContent?.match(/#(\d+)/)?.[1] ?? '', getComputedStyle(e).paddingLeft]),
  )
  const pad = Object.fromEntries(rows)
  expect(pad['1']).toBe('0px')
  expect(pad['2']).toBe('16px')
  expect(pad['3']).toBe('16px')
})

test('the quest filter narrows the list', async ({ page }) => {
  await open(page)
  const panel = questPanel(page)
  await panel.getByLabel('Filter quests').fill('renderer')
  await expect(panel.getByRole('listitem')).toHaveCount(1)
  await expect(panel.getByRole('button', { name: /which renderer/ })).toBeVisible()
  await panel.getByLabel('Filter quests').fill('#5')
  await expect(panel.getByRole('listitem')).toHaveCount(1)
  await panel.getByLabel('Filter quests').fill('nothing matches this')
  await expect(panel.getByText('No quests match.')).toBeVisible()
})

test('opening a quest shows its markdown, relations and comments; an unknown issue fails gracefully', async ({ page }) => {
  await open(page)
  const panel = questPanel(page)
  await panel.getByRole('button', { name: /#3 \[demo\] Render rooms/ }).click()
  await expect(panel.getByRole('heading', { name: /#3 \[demo\] Render rooms on the map/ })).toBeVisible()
  await expect(panel.getByText('opened by @dkage')).toBeVisible()
  await expect(panel.getByRole('button', { name: /#1 \[demo\] Epic/ })).toBeVisible() // parent
  await expect(panel.getByRole('button', { name: /#2 \[demo\] Sketch/ })).toBeVisible() // blocker
  await expect(panel.getByLabel('Description')).toContainText('See also the epic')
  await expect(panel.getByLabel('Comments')).toContainText('Needs the layout sketch first')
  await expect(panel.getByLabel('Comments').locator('strong').last()).toHaveText('layout sketch') // markdown rendered

  // The fake GitHub only knows #3 in detail, so following the blocker shows the error state.
  await panel.getByRole('button', { name: /#2 \[demo\] Sketch/ }).click()
  await expect(panel.getByRole('alert')).toContainText('That issue was not found.')
  await panel.getByRole('button', { name: /← Quests/ }).click()
  await expect(panel.getByLabel('Filter quests')).toBeVisible()
})

test('a hostile issue cannot run script or load anything (sanitiser and CSP together)', async ({ page }) => {
  const violations: string[] = []
  page.on('console', (m) => {
    if (/content security policy/i.test(m.text())) violations.push(m.text())
  })
  await open(page)
  const panel = questPanel(page)
  await panel.getByRole('button', { name: /#9 Hostile body/ }).click()
  await expect(panel.getByRole('heading', { name: /#9 Hostile body/ })).toBeVisible()
  await expect(panel.getByLabel('Description')).toContainText('still bold')
  await expect(panel.getByLabel('Comments')).toBeVisible()

  // Nothing dangerous reached the DOM…
  const dangerous = await panel.locator('script, img, svg, iframe, [onerror], [onload]').count()
  expect(dangerous).toBe(0)
  const hrefs = await panel.getByLabel('Description').locator('a').evaluateAll((as) => as.map((a) => a.getAttribute('href') ?? ''))
  for (const h of hrefs) expect(h).not.toMatch(/^javascript:/i)
  // …nothing ran…
  expect(await page.evaluate(() => (window as unknown as { __pwned?: number }).__pwned)).toBeUndefined()
  // …and the browser reported no policy violation either.
  expect(violations).toEqual([])
})

test('refreshing reloads the quests', async ({ page }) => {
  await open(page)
  const panel = questPanel(page)
  await expect(panel.getByRole('button', { name: /#1 \[demo\] Epic/ })).toBeVisible()
  const requested = page.waitForRequest((r) => r.url().endsWith('/quests?refresh=1'))
  await panel.getByRole('button', { name: 'Refresh quests' }).click()
  await requested
  await expect(panel.getByRole('button', { name: /#1 \[demo\] Epic/ })).toBeVisible()
})

test('a campaign without a GitHub repository offers demo quests, and can go back', async ({ page }) => {
  await open(page)
  await addCampaign(page, plaindir, 'plain')
  const panel = questPanel(page)
  await expect(panel.getByRole('alert')).toContainText('no GitHub repository')
  await expect(panel.getByRole('button', { name: /Epic/ })).toHaveCount(0) // not the other campaign's issues

  await panel.getByRole('button', { name: 'Load demo quests' }).click()
  await expect(panel.getByText('demo data')).toBeVisible()
  await expect(panel.getByRole('button', { name: /Build the signup form/ })).toBeVisible()
  await expect(panel.getByRole('button', { name: /Suspend a user/ })).toContainText('blocked by #12')

  // Demo detail, with its relations, and no GitHub link for sample data.
  await panel.getByRole('button', { name: /Build the signup form/ }).click()
  await expect(panel.getByRole('heading', { name: /#3 Build the signup form/ })).toBeVisible()
  await expect(panel.getByRole('button', { name: /#2 Design the welcome screen/ })).toBeVisible()
  await expect(panel.getByRole('link', { name: 'Open on GitHub' })).toHaveCount(0)
  // Titles and headings inside an issue are content: they must not inherit the
  // small uppercase style used for section labels.
  const title = panel.getByRole('heading', { name: /#3 Build the signup form/ })
  expect(await title.evaluate((e) => getComputedStyle(e).textTransform)).toBe('none')
  await panel.getByRole('button', { name: /← Quests/ }).click()
  await panel.getByRole('button', { name: /Login button unresponsive/ }).click()
  const steps = panel.getByLabel('Description').getByRole('heading', { name: 'Steps' })
  await expect(steps).toBeVisible()
  expect(await steps.evaluate((e) => getComputedStyle(e).textTransform)).toBe('none')
  await panel.getByRole('button', { name: /← Quests/ }).click()

  await panel.getByRole('button', { name: 'Use real issues' }).click()
  await expect(panel.getByRole('alert')).toContainText('no GitHub repository')

  // Switching back to the other campaign shows its own issues again.
  await nav(page).getByRole('button', { name: /^demo/ }).click()
  await expect(panel.getByRole('button', { name: /#1 \[demo\] Epic/ })).toBeVisible()

  // Clean up.
  await nav(page).getByRole('button', { name: /^plain/ }).click()
  await page.getByRole('button', { name: 'Remove campaign plain' }).click()
  await page.getByRole('button', { name: 'Remove?' }).click()
  await expect(nav(page).getByRole('button', { name: /^plain/ })).toHaveCount(0)
})

test('on a narrow window the quest panel is a drawer opened from the toolbar', async ({ page }) => {
  await page.setViewportSize({ width: 1000, height: 800 })
  await open(page)
  const panel = questPanel(page)
  await expect(panel.getByRole('button', { name: /#1 \[demo\] Epic/ })).not.toBeInViewport()
  await page.getByRole('button', { name: 'Quests', exact: true }).click()
  await expect(panel.getByRole('button', { name: /#1 \[demo\] Epic/ })).toBeInViewport()
  await panel.getByRole('button', { name: 'Close quests' }).click()
  await expect(panel.getByRole('button', { name: /#1 \[demo\] Epic/ })).not.toBeInViewport()
})
