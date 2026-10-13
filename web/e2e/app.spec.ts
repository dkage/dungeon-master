import { mkdirSync, readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { expect, test, type Page } from '@playwright/test'

const here = dirname(fileURLToPath(import.meta.url))
const { url, workdir, subdir, plaindir, ghLog } = JSON.parse(readFileSync(join(here, '.url.json'), 'utf8')) as {
  url: string
  workdir: string
  subdir: string
  plaindir: string
  ghLog: string
}

// The close button of an expedition (not the quest drawer's).
const closeSession = /^Close (?!quests)/
const questPanel = (page: Page) => page.getByRole('complementary', { name: 'Quests' })
const nav = (page: Page) => page.getByRole('navigation', { name: 'Campaigns' })
const hero = (page: Page, name: string) => page.getByRole('button', { name, exact: true })
const screenText = (page: Page) => page.locator('.pane:not([hidden]) .xterm-rows')
const expeditions = (page: Page, campaign = 'demo') =>
  page.getByRole('list', { name: `Expeditions in ${campaign}` })

async function captureMilestoneScreen(page: Page, name: string) {
  if (process.env.M4_SCREENSHOTS !== '1') return
  const dir = join(here, '../../_probe/m4')
  mkdirSync(dir, { recursive: true })
  await page.screenshot({ path: join(dir, `${name}.png`), fullPage: true })
}

// These tests drive the quest list; the map has its own spec (map.spec.ts).
test.beforeEach(async ({ page }) => {
  await page.addInitScript(() => {
    if (!localStorage.getItem('dungeon.panel.view')) localStorage.setItem('dungeon.panel.view', 'list')
  })
})

async function addCampaign(page: Page, dir: string, name: string) {
  await page.getByRole('button', { name: 'Add project', exact: true }).click()
  await page.getByPlaceholder('/absolute/path/to/project').fill(dir)
  await page.getByPlaceholder(/defaults to the folder/).fill(name)
  await page.getByRole('button', { name: 'Add campaign' }).click()
}

async function requestCampaignRemoval(page: Page, name: string, sample = false) {
  if (sample) {
    await page.getByRole('button', { name: 'Remove sample project' }).click()
  } else {
    await page.getByRole('button', { name: `Campaign actions for ${name}` }).click()
    await page.getByRole('group', { name: `Actions for ${name}` }).getByRole('button', { name: 'Remove project' }).click()
  }
}

async function confirmCampaignRemoval(page: Page, name: string, sample = false) {
  const group = page.getByRole('group', { name: `Confirm removal of ${name}` })
  await group.getByRole('button', { name: sample ? 'Remove sample' : 'Remove project' }).click()
}

// A campaign that has not chosen which issues to show is asked once. Most tests
// are about something else, so they answer "everything"; the scope tests below
// answer differently on purpose.
async function chooseEverything(page: Page) {
  const panel = questPanel(page)
  const list = panel.getByLabel('Filter quests')
  const question = panel.getByRole('radio', { name: /Everything/ })
  await expect(list.or(question)).toBeVisible()
  if (await question.isVisible()) {
    await question.check()
    await panel.getByRole('button', { name: 'Show these issues' }).click()
    await expect(list).toBeVisible()
  }
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
  await chooseEverything(page)
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

test('a fresh install shows the getting-started checklist and a separate sample workspace', async ({ page }) => {
  await page.goto(url)
  await expect(page.getByRole('heading', { name: 'Start here' })).toBeVisible()
  await captureMilestoneScreen(page, 'first-run')
  await expect(page.locator('.start-here .intro')).toContainText('then start an agent.')
  await expect(page.locator('.brand-header')).toContainText('Dungeon Master')
  expect(await page.locator('.app').evaluate((el) => getComputedStyle(el).gridTemplateColumns.split(' ').length)).toBe(3)
  await page.getByRole('button', { name: 'Try the sample first' }).click()
  const panel = questPanel(page)
  await expect(panel.getByText('sample project', { exact: true })).toBeVisible()
  await expect(panel.getByText(/separate sample project/)).toBeVisible()
  await panel.getByRole('region', { name: 'Quest browser' }).getByRole('button', { name: /#1 Ship the onboarding flow/ }).click()
  await expect(panel.getByLabel('Filter quests')).toBeVisible()
  await expect(panel.getByRole('region', { name: 'Selected issue detail' })).toBeVisible()
  await captureMilestoneScreen(page, 'quest-selected')
  await page.getByRole('button', { name: 'Keep these starter heroes' }).click()
  await expect(page.getByRole('button', { name: 'Keep these starter heroes' })).toHaveCount(0)
  await page.getByRole('button', { name: '⚙ Settings', exact: true }).click()
  const settings = page.getByRole('dialog', { name: 'Settings' })
  await expect(settings.getByRole('cell', { name: 'Opus', exact: true })).toBeVisible()
  await captureMilestoneScreen(page, 'settings')
  // Row order changes when the party file is reloaded, so pick the row by name.
  const opusRow = settings.locator('tbody tr', { hasText: /^\s*Opus/ })
  await opusRow.getByRole('button', { name: 'Edit' }).click()
  await settings.getByLabel('Model').fill('claude-opus-5-5')
  await settings.getByRole('button', { name: 'Save hero' }).click()
  await expect(settings.getByRole('group', { name: 'Edit Opus' })).toHaveCount(0)
  await settings.getByRole('button', { name: 'Close settings' }).click()
  await hero(page, 'Opus').click()
  await expect(screenText(page)).toContainText('FAKE-CLAUDE')
  await expect(screenText(page)).toContainText('sample-workspace')
  await captureMilestoneScreen(page, 'session-running')
  await page.getByRole('button', { name: closeSession }).click()
  await requestCampaignRemoval(page, 'Sample (not a real project)', true)
  await confirmCampaignRemoval(page, 'Sample (not a real project)', true)
  await expect(page.getByRole('button', { name: /^Sample/ })).toHaveCount(0)
  await captureMilestoneScreen(page, 'empty-state')
})

test('a browser that is not signed in is told so, and never sees an empty app that looks like lost data', async ({ browser, baseURL }) => {
  const ctx = await browser.newContext() // no cookie, no token
  const page = await ctx.newPage()
  await page.goto(baseURL!)
  await expect(page.getByRole('heading', { name: 'This tab is not signed in' })).toBeVisible()
  await expect(page.getByText('Add one to start')).toHaveCount(0)
  await expect(page.getByPlaceholder('/absolute/path/to/project')).toHaveCount(0)
  // Opening the signed-in link fixes it.
  await page.goto(url)
  await expect(hero(page, 'Opus')).toBeVisible()
  await expect(page.getByRole('heading', { name: 'This tab is not signed in' })).toHaveCount(0)
  await ctx.close()
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
  await expect(nav(page).getByRole('button', { name: /^api/ })).toHaveAttribute('aria-current', 'true')
  await expect(page.locator('.toolbar')).toContainText('api')
  await expect(page.locator('.toolbar')).toContainText('acme/demo') // from the git root's remote

  await page.getByRole('button', { name: 'Campaign actions for api' }).click()
  await page.getByRole('group', { name: 'Actions for api' }).getByRole('button', { name: 'Opus: free session' }).click()
  await expect(screenText(page)).toContainText('FAKE-CLAUDE')
  await expect(screenText(page)).toContainText(join('apps', 'api')) // cwd is the subfolder

  // Clean up so later tests start from one campaign.
  await page.getByRole('button', { name: closeSession }).click()
  await requestCampaignRemoval(page, 'api')
  await confirmCampaignRemoval(page, 'api')
  await expect(nav(page).getByRole('button', { name: /^api/ })).toHaveCount(0)
})

test('adding an existing directory selects its campaign', async ({ page }) => {
  await open(page)
  await addCampaign(page, workdir, 'again')
  await expect(nav(page).getByRole('button', { name: /^demo/ })).toHaveAttribute('aria-current', 'true')
  await expect(nav(page).getByRole('button', { name: /^again/ })).toHaveCount(0)
})

test('launch a hero, see its output, type to it', async ({ page }) => {
  await open(page)
  await hero(page, 'Opus').click()

  const term = screenText(page)
  await expect(term).toContainText('FAKE-CLAUDE')
  await expect(term).toContainText('arg=[--model]')
  // The first test saved this model for Opus through the settings dialog.
  await expect(term).toContainText('arg=[claude-opus-5-5]')
  await expect(term).toContainText('cwd=')

  await page.locator('.pane:not([hidden]) .xterm').click()
  await page.keyboard.type('hello dungeon')
  await page.keyboard.press('Enter')
  await expect(term).toContainText('echo:hello dungeon')
})

test('the parchment shows behind output in Dungeon, a flat page in Neutral', async ({ page }) => {
  await open(page)
  await hero(page, 'Opus').click()
  const term = screenText(page)
  await expect(term).toContainText('FAKE-CLAUDE')
  const pane = page.locator('.pane:not([hidden])')
  const page_ = pane.locator('.term')
  const image = (loc: typeof page_) => loc.evaluate((el) => getComputedStyle(el).backgroundImage)
  const screenBg = () => pane.locator('.xterm-screen canvas, .xterm-rows').first().evaluate((el) => getComputedStyle(el).backgroundColor)

  expect(await image(page_)).toContain('parchment')
  // The image must actually load (CSP, path), not just be declared.
  const loaded = await page.evaluate(async (css) => {
    const src = /url\("?([^")]+)"?\)/.exec(css)![1]
    return new Promise<boolean>((resolve) => {
      const img = new Image()
      img.onload = () => resolve(img.naturalWidth > 0)
      img.onerror = () => resolve(false)
      img.src = src
    })
  }, await image(page_))
  expect(loaded).toBe(true)
  expect(['rgba(0, 0, 0, 0)', 'transparent']).toContain(await screenBg())

  await page.getByRole('button', { name: '⚙ Settings', exact: true }).click()
  const settings = page.getByRole('dialog', { name: 'Settings' })
  await settings.getByLabel(/^Theme/).selectOption('neutral')
  await settings.getByRole('button', { name: 'Close settings' }).click()
  expect(await image(page_)).toBe('none')
  expect(await page_.evaluate((el) => getComputedStyle(el).backgroundColor)).toBe('rgb(20, 20, 26)')
  await expect(term).toContainText('FAKE-CLAUDE')

  // Output still arrives and scrolls after the switch.
  await pane.locator('.xterm').click()
  for (let i = 0; i < 60; i++) {
    await page.keyboard.type(`line-${i}`)
    await page.keyboard.press('Enter')
  }
  await expect(term).toContainText('echo:line-59')
  await pane.locator('.xterm').hover()
  await expect(async () => {
    await page.mouse.wheel(0, -2000)
    await expect(term).not.toContainText('echo:line-59', { timeout: 500 })
  }).toPass({ timeout: 10000 })
  await expect(async () => {
    await page.mouse.wheel(0, 20000)
    await expect(term).toContainText('echo:line-59', { timeout: 500 })
  }).toPass({ timeout: 10000 })

  await page.getByRole('button', { name: '⚙ Settings', exact: true }).click()
  await settings.getByLabel(/^Theme/).selectOption('dungeon')
  await settings.getByRole('button', { name: 'Close settings' }).click()
  expect(await image(page_)).toContain('parchment')
  await expect(term).toContainText('echo:line-59')
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
  const busyDir = join(dirname(plaindir), 'busy-app')
  mkdirSync(busyDir, { recursive: true })
  await addCampaign(page, busyDir, 'busy')
  await expect(nav(page).getByRole('button', { name: /^busy/ })).toHaveAttribute('aria-current', 'true')
  await hero(page, 'Shell').click()
  await expect(expeditions(page, 'busy').getByRole('listitem')).toHaveCount(1)

  await requestCampaignRemoval(page, 'busy')
  await confirmCampaignRemoval(page, 'busy')
  await expect(page.locator('p[role="alert"]')).toContainText('running sessions')
  await expect(nav(page).getByRole('button', { name: /^busy/ })).toBeVisible()

  // Close the expedition, then removal succeeds.
  await page.getByRole('button', { name: closeSession }).click()
  await requestCampaignRemoval(page, 'busy')
  await confirmCampaignRemoval(page, 'busy')
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
  // cmd.exe drops keys typed before its first prompt, which a slow runner hits.
  if (process.platform === 'win32') await expect(screenText(page)).toContainText('work>', { timeout: 25_000 })
  await page.keyboard.type(process.platform === 'win32' ? `powershell -NoProfile -Command "[Console]::WriteLine([char]27 + '[31mRED-TEXT' + [char]27 + '[0m plain-text')"` : "printf '\\033[31mRED-TEXT\\033[0m plain-text\\n'")
  await page.keyboard.press('Enter')
  // A cold PowerShell start on a Windows runner takes several seconds.
  await expect(screenText(page)).toContainText('RED-TEXT plain-text', { timeout: process.platform === 'win32' ? 25_000 : 5_000 })

  const font = await screenText(page).evaluate((e) => getComputedStyle(e).fontFamily)
  expect(font).toMatch(/monospace|Menlo/)

  const colour = (text: RegExp) =>
    page.locator('.pane:not([hidden]) .xterm-rows span').filter({ hasText: text }).last()
      .evaluate((e) => getComputedStyle(e).color)
  const red = await colour(/^RED-TEXT\s*$/)
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

test("the quest panel lists the repository's issues with their states and relationships", async ({ page }) => {
  await open(page)
  const panel = questPanel(page)
  await expect(panel.getByText('acme/demo')).toBeVisible()
  await expect(panel.getByRole('button', { name: /#1 \[demo\] Epic/ })).toBeVisible()
  await expect(panel.getByRole('button', { name: /#5 \[demo\] Bug/ })).toContainText('@dkage') // assigned
  await expect(panel.getByRole('button', { name: /#3 \[demo\] Render rooms/ })).toContainText('Blocked by #2') // locked
  await expect(panel.getByTitle('Sub-issues done')).toHaveText('0/2')
  const states = panel.getByRole('group', { name: 'Filter by state' })
  await expect(states.getByRole('button', { name: /^Blocked/ })).toHaveText('Blocked 1')
  await expect(states.getByRole('button', { name: /^In progress/ })).toHaveText('In progress 1')

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
  const browser = panel.getByRole('region', { name: 'Quest browser' })
  await browser.getByRole('button', { name: /#3 \[demo\] Render rooms/ }).click()
  const detail = panel.getByRole('region', { name: 'Selected issue detail' })
  await expect(detail.getByRole('heading', { name: /#3 \[demo\] Render rooms on the map/ })).toBeVisible()
  await expect(detail.getByText('opened by @dkage')).toBeVisible()
  await expect(detail.getByRole('button', { name: /#1 \[demo\] Epic/ })).toBeVisible() // parent
  await expect(detail.getByRole('button', { name: /#2 \[demo\] Sketch/ })).toBeVisible() // blocker
  await expect(detail.getByLabel('Description')).toContainText('See also the epic')
  await expect(detail.getByLabel('Comments')).toContainText('Needs the layout sketch first')
  await expect(detail.getByLabel('Comments').locator('strong').last()).toHaveText('layout sketch') // markdown rendered

  // The fake GitHub only knows #3 in detail, so following the blocker shows the error state.
  await detail.getByRole('button', { name: /#2 \[demo\] Sketch/ }).click()
  await expect(detail.getByRole('alert')).toContainText('That issue was not found.')
  await detail.getByRole('button', { name: /← Quests/ }).click()
  await expect(browser.getByLabel('Filter quests')).toBeVisible()
})

test('a hostile issue cannot run script or load anything (sanitiser and CSP together)', async ({ page }) => {
  const violations: string[] = []
  page.on('console', (m) => {
    if (/content security policy/i.test(m.text())) violations.push(m.text())
  })
  await open(page)
  const panel = questPanel(page)
  const browser = panel.getByRole('region', { name: 'Quest browser' })
  await browser.getByRole('button', { name: /#9 Hostile body/ }).click()
  const detail = panel.getByRole('region', { name: 'Selected issue detail' })
  await expect(detail.getByRole('heading', { name: /#9 Hostile body/ })).toBeVisible()
  await expect(detail.getByLabel('Description')).toContainText('still bold')
  await expect(detail.getByLabel('Comments')).toBeVisible()

  // Nothing dangerous reached the DOM…
  const dangerous = await detail.locator('script, img, svg, iframe, [onerror], [onload]').count()
  expect(dangerous).toBe(0)
  const hrefs = await detail.getByLabel('Description').locator('a').evaluateAll((as) => as.map((a) => a.getAttribute('href') ?? ''))
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

test('a campaign without a GitHub repository can open the separate sample project', async ({ page }) => {
  await open(page)
  await addCampaign(page, plaindir, 'plain')
  const panel = questPanel(page)
  await expect(panel.getByRole('alert')).toContainText('no GitHub repository')
  await expect(panel.getByRole('button', { name: /Epic/ })).toHaveCount(0) // not the other campaign's issues

  await panel.getByRole('button', { name: 'Try the separate sample project' }).click()
  await expect(panel.getByText('sample project', { exact: true })).toBeVisible()
  await expect(panel.getByText(/separate sample project/)).toBeVisible()
  await expect(panel.getByRole('button', { name: /Build the signup form/ })).toBeVisible()
  await expect(panel.getByRole('button', { name: /Suspend a user/ })).toContainText('Blocked by #12, #40')

  // Demo detail, with its relations, and no GitHub link for sample data.
  await panel.getByRole('button', { name: /Build the signup form/ }).click()
  const detail = panel.getByRole('region', { name: 'Selected issue detail' })
  await expect(detail.getByRole('heading', { name: /#3 Build the signup form/ })).toBeVisible()
  await expect(detail.getByRole('button', { name: /#2 Design the welcome screen/ })).toBeVisible()
  await expect(detail.getByRole('link', { name: 'Open on GitHub' })).toHaveCount(0)
  await detail.getByRole('button', { name: 'Review launch' }).click()
  const briefing = page.getByRole('dialog', { name: 'Review launch plan' })
  await expect(briefing.locator('.brief-content')).toContainText('Build the signup form')
  await expect(briefing.locator('.brief-content')).toContainText('untrusted task data')
  await expect(briefing.getByRole('button', { name: 'Start expedition' })).toBeEnabled()
  await briefing.getByRole('button', { name: 'Close briefing' }).click()
  // Titles and headings inside an issue are content: they must not inherit the
  // small uppercase style used for section labels.
  const title = detail.getByRole('heading', { name: /#3 Build the signup form/ })
  expect(await title.evaluate((e) => getComputedStyle(e).textTransform)).toBe('none')
  await detail.getByRole('button', { name: /← Quests/ }).click()
  await panel.getByRole('region', { name: 'Quest browser' }).getByRole('button', { name: /Send the welcome email/ }).click()
  const blockedDetail = panel.getByRole('region', { name: 'Selected issue detail' })
  // A locked quest cannot be started: the button is disabled and says why.
  await expect(blockedDetail.getByRole('button', { name: 'Review launch' })).toBeDisabled()
  await expect(blockedDetail.locator('.launch-note')).toContainText('Blocked by #4, #7.')
  await blockedDetail.getByRole('button', { name: /← Quests/ }).click()
  await panel.getByRole('region', { name: 'Quest browser' }).getByRole('button', { name: /#2 Design the welcome screen/ }).click()
  const closedDetail = panel.getByRole('region', { name: 'Selected issue detail' })
  await expect(closedDetail.getByRole('button', { name: 'Review launch' })).toBeDisabled()
  await expect(closedDetail.getByText('Closed issues cannot be started.')).toBeVisible()
  await closedDetail.getByRole('button', { name: /← Quests/ }).click()
  await panel.getByRole('region', { name: 'Quest browser' }).getByRole('button', { name: /Evaluate rate-limit libraries/ }).click()
  const launchDetail = panel.getByRole('region', { name: 'Selected issue detail' })
  await launchDetail.getByRole('button', { name: 'Review launch' }).click()
  const launchPlan = page.getByRole('dialog', { name: 'Review launch plan' })
  await expect(launchPlan.getByText('Evaluate rate-limit libraries')).toBeVisible()
  await captureMilestoneScreen(page, 'launch-preview')
  await launchPlan.getByRole('button', { name: 'Start expedition' }).click()
  await expect(screenText(page)).toContainText('arg=[--append-system-prompt]')
  await expect(screenText(page)).toContainText('task briefing file at')
  await expect(screenText(page)).toContainText('sample-workspace')
  await expect(page.locator('.quest-session-chip')).toHaveText('#9')
  await expect(launchDetail.getByRole('button', { name: 'Go to session' })).toBeVisible()
  await expect(launchDetail.getByRole('button', { name: 'Review launch' })).toBeDisabled()
  const linkedRows = nav(page).getByRole('list', { name: /Expeditions in Sample/ })
  await linkedRows.getByRole('button', { name: /^#9 Evaluate rate-limit libraries/ }).click()
  await expect(launchDetail.getByRole('heading', { name: /#9 Evaluate rate-limit libraries/ })).toBeVisible()
  await page.locator('.quest-session-chip').click()
  await expect(launchDetail.getByRole('heading', { name: /#9 Evaluate rate-limit libraries/ })).toBeVisible()
  await launchDetail.getByRole('button', { name: 'Go to session' }).click()
  await expect(page.locator('.quest-session-chip')).toHaveText('#9')
  await page.getByRole('button', { name: closeSession }).click()
  await launchDetail.getByRole('button', { name: /← Quests/ }).click()
  await panel.getByRole('region', { name: 'Quest browser' }).getByRole('button', { name: /Login button unresponsive/ }).click()
  const selectedDetail = panel.getByRole('region', { name: 'Selected issue detail' })
  const steps = selectedDetail.getByLabel('Description').getByRole('heading', { name: 'Steps' })
  await expect(steps).toBeVisible()
  expect(await steps.evaluate((e) => getComputedStyle(e).textTransform)).toBe('none')
  await selectedDetail.getByRole('button', { name: /← Quests/ }).click()

  // Switching back to the real campaign shows its own issues again.
  await nav(page).getByRole('button', { name: /^demo/ }).click()
  await expect(panel.getByRole('button', { name: /#1 \[demo\] Epic/ })).toBeVisible()

  // Clean up.
  await nav(page).getByRole('button', { name: /^plain/ }).click()
  await requestCampaignRemoval(page, 'plain')
  await confirmCampaignRemoval(page, 'plain')
  await expect(nav(page).getByRole('button', { name: /^plain/ })).toHaveCount(0)
  await requestCampaignRemoval(page, 'Sample (not a real project)', true)
  await confirmCampaignRemoval(page, 'Sample (not a real project)', true)
  await expect(page.getByRole('button', { name: /^Sample/ })).toHaveCount(0)
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

test('the panel shows the GitHub budget the Quartermaster read from the response headers', async ({ page }) => {
  await open(page)
  const panel = questPanel(page)
  const chip = panel.getByText(/GitHub requests: 4,936 of 5,000 left/)
  await expect(chip).toBeVisible()
  await expect(chip).toHaveAttribute('title', /shared with every other tool/)
  // Not paused, so no camp banner.
  await expect(panel.getByText(/Paused until/)).toHaveCount(0)
  // The API behind it is the one the UI reads.
  const status = await page.evaluate(async () => (await fetch('/api/github', { credentials: 'same-origin' })).json())
  expect(status.camp.active).toBe(false)
  expect(status.rations[0]).toMatchObject({ resource: 'graphql', limit: 5000, remaining: 4936 })
})

// --- scope ---

test('a new campaign asks which issues to show, with real counts, and remembers the answer', async ({ page }) => {
  await open(page)
  await addCampaign(page, plaindir.replace(/plain$/, 'work/apps'), 'scoped')
  const panel = questPanel(page)

  // The question, with the numbers from the (recorded) repository.
  await expect(panel.getByText('This repository has 312 open issues. Show:')).toBeVisible()
  await expect(panel.getByRole('radio', { name: /Issues labeled tracked \(0\)/ })).not.toBeChecked()
  await expect(panel.getByText('None yet. This app never adds the label')).toBeVisible()
  await expect(panel.getByRole('radio', { name: /Assigned to me \(@dkage\) \(1\)/ })).toBeChecked() // the suggestion
  // An empty marker label is an opt-in at the bottom, not the first choice.
  await expect(panel.getByRole('radio').first()).toHaveAccessibleName(/Assigned to me/)
  await expect(panel.getByRole('radio').last()).toHaveAccessibleName(/Issues labeled tracked/)
  await expect(panel.getByRole('radio', { name: /Everything \(312\)/ })).toBeVisible()
  // Nothing is listed, and it is not an error.
  await expect(panel.getByLabel('Filter quests')).toHaveCount(0)
  await expect(panel.getByRole('alert')).toHaveCount(0)

  // Assigned to me: only that issue, and the header says so.
  await panel.getByRole('button', { name: 'Show these issues' }).click()
  await expect(panel.getByRole('button', { name: /assigned to me · 1 of 312 open/ })).toBeVisible()
  await expect(panel.getByRole('listitem')).toHaveCount(1)
  await expect(panel.getByRole('button', { name: /#5 \[demo\] Bug/ })).toBeVisible()

  // Switch to two labels: the recorded label query answers with three issues.
  await panel.getByRole('button', { name: /assigned to me · / }).click()
  await panel.getByRole('radio', { name: /Pick labels/ }).check()
  await panel.getByRole('checkbox', { name: /^bug \(/ }).check()
  await panel.getByRole('checkbox', { name: /^enhancement \(/ }).check()
  await panel.getByRole('button', { name: 'Apply' }).click()
  await expect(panel.getByRole('button', { name: /bug \+ enhancement · 3 of 312 open/ })).toBeVisible()
  await expect(panel.getByRole('listitem')).toHaveCount(3)

  // The choice survives a reload (it is stored on the server, not in the page).
  await page.reload()
  await nav(page).getByRole('button', { name: /^scoped/ }).click()
  await expect(panel.getByRole('button', { name: /bug \+ enhancement · 3 of 312 open/ })).toBeVisible()

  // Cancelling the picker changes nothing.
  await panel.getByRole('button', { name: /bug \+ enhancement · / }).click()
  await panel.getByRole('button', { name: 'Cancel' }).click()
  await expect(panel.getByLabel('Filter quests')).toBeVisible()

  await requestCampaignRemoval(page, 'scoped')
  await confirmCampaignRemoval(page, 'scoped')
  await expect(nav(page).getByRole('button', { name: /^scoped/ })).toHaveCount(0)
})

test('the label chips narrow the list without another request to GitHub', async ({ page }) => {
  await open(page)
  const panel = questPanel(page)
  await expect(panel.getByRole('listitem')).toHaveCount(6) // five open issues plus the hostile one
  const requests: string[] = []
  page.on('request', (r) => requests.push(r.url()))

  const chips = panel.getByRole('group', { name: 'Filter by label' })
  await chips.getByRole('button', { name: /^enhancement/ }).click()
  await expect(panel.getByRole('listitem')).toHaveCount(3)
  await chips.getByRole('button', { name: /^bug/ }).click()
  await expect(panel.getByRole('listitem')).toHaveCount(4)
  await chips.getByRole('button', { name: /^bug/ }).click()
  await chips.getByRole('button', { name: /^enhancement/ }).click()
  await expect(panel.getByRole('listitem')).toHaveCount(6)
  expect(requests.filter((u) => u.includes('/quests'))).toEqual([]) // purely client-side
})

test('a list that was cut says so and points to narrowing the scope', async ({ page }) => {
  await open(page)
  const hugeDir = join(dirname(subdir), 'huge-app')
  mkdirSync(hugeDir, { recursive: true })
  await addCampaign(page, hugeDir, 'huge')
  await expect(nav(page).getByRole('button', { name: /^huge/ })).toHaveAttribute('aria-current', 'true')
  const panel = questPanel(page)
  await panel.getByRole('radio', { name: /Pick labels/ }).check()
  await panel.getByRole('checkbox', { name: /^huge \(/ }).check()
  await panel.getByRole('button', { name: 'Show these issues' }).click()
  await expect(panel.getByText(/Showing the newest 3 of 312 open issues. Narrow the scope to see the rest./)).toBeVisible()
  await expect(panel.getByRole('listitem')).toHaveCount(3)

  // Narrowing makes the notice go away.
  await panel.getByRole('button', { name: /huge · / }).click()
  await panel.getByRole('radio', { name: /Assigned to me/ }).check()
  await panel.getByRole('button', { name: 'Apply' }).click()
  await expect(panel.getByText(/Narrow the scope to see the rest/)).toHaveCount(0)

  await requestCampaignRemoval(page, 'huge')
  await confirmCampaignRemoval(page, 'huge')
  await expect(nav(page).getByRole('button', { name: /^huge/ })).toHaveCount(0)
})

// --- Gatekeeper ---

test('gh in a session is the shim: reads pass at once, writes are paced', async ({ page }) => {
  test.setTimeout(60_000)
  await open(page)
  await hero(page, 'Shell').click()
  const term = screenText(page)
  await page.locator('.pane:not([hidden]) .xterm').click()

  // The first gh on the session's PATH is the app's shim, in front of the real one.
  await page.keyboard.type(process.platform === 'win32' ? 'where gh' : 'command -v gh')
  await page.keyboard.press('Enter')
  await expect(term).toContainText(process.platform === 'win32' ? '\\gatekeeper\\bin\\gh.exe' : '/gatekeeper/bin/gh')

  // A read goes straight through to the real gh.
  await page.keyboard.type('gh issue list --limit 3')
  await page.keyboard.press('Enter')
  await expect(term).toContainText('fake-gh-ran: issue list --limit 3')

  // Three writes in a tight loop, as an agent might send them.
  await page.keyboard.type(process.platform === 'win32'
    ? 'for %n in (a b c) do @gh issue create --title %n'
    : 'for n in a b c; do gh issue create --title $n; done; echo LOOP-DONE-$((40 + 2))')
  await page.keyboard.press('Enter')
  if (process.platform === 'win32') {
    await expect(term).toContainText('fake-gh-ran: issue create --title c', { timeout: 40_000 })
  } else { await expect(term).toContainText('LOOP-DONE-42', { timeout: 40_000 }) } // only the output has 42, not the typed command
  await expect(term).toContainText('fake-gh-ran: issue create --title c')

  const calls = readFileSync(ghLog, 'utf8').trim().split('\n').map((l) => {
    const [sec, ...rest] = l.split(' ')
    return { at: Number(sec), cmd: rest.join(' ') }
  })
  const creates = calls.filter((c) => c.cmd.startsWith('issue create'))
  expect(creates.map((c) => c.cmd)).toEqual(['issue create --title a', 'issue create --title b', 'issue create --title c'])
  // At least 3 s between writes (plus jitter): two gaps are at least 6 s, allowing a second of clock rounding.
  expect(creates[2].at - creates[0].at).toBeGreaterThanOrEqual(5)
  // The shell was told why it waited.
  await expect(term).toContainText('waited')
  await expect(term).toContainText('GitHub write slot')
  // The app counted them.
  const status = await page.evaluate(async () => (await fetch('/api/github', { credentials: 'same-origin' })).json())
  expect(status.writes.lastMinute).toBeGreaterThanOrEqual(3)
})

test('a session that stopped to ask something is found without opening it', async ({ page }) => {
  await open(page)
  await hero(page, 'Opus').click()
  await expect(screenText(page)).toContainText('FAKE-CLAUDE')
  await hero(page, 'Opus').click()
  const list = expeditions(page)
  await expect(list.getByRole('listitem')).toHaveCount(2)
  // The second session asks a question; the first is the one in front.
  await page.locator('.pane:not([hidden]) .xterm').click()
  await page.keyboard.type('ask me')
  await page.keyboard.press('Enter')
  await expect(screenText(page)).toContainText('Overwrite the file? (y/n)')
  await list.getByRole('listitem').first().getByRole('button').first().click()

  // After a few quiet seconds the list says which one is waiting, in words.
  const waiting = list.getByRole('listitem').nth(1)
  await expect(waiting).toContainText('waiting for you', { timeout: 15_000 })
  await expect(list.getByRole('listitem').first()).not.toContainText('waiting for you')
  await expect(page).toHaveTitle('(1) Dungeon Master')
  await expect(page.locator('link[rel="icon"]')).toHaveAttribute('href', /circle/)

  // Opening it clears the count.
  await waiting.getByRole('button').first().click()
  await expect(page).toHaveTitle('Dungeon Master')
})

test('a desktop notification fires once for a session that asks while the window is not in front', async ({ page }) => {
  test.setTimeout(60_000)
  // A fake Notification API, and a window that can be "not in front" on demand.
  await page.addInitScript(() => {
    const w = window as unknown as { __shown: { title: string; body?: string }[]; __front: boolean }
    w.__shown = []
    w.__front = true
    document.hasFocus = () => w.__front
    class FakeNotification {
      static permission = 'default'
      static requestPermission = async () => ((FakeNotification.permission = 'granted'), 'granted')
      onclick: (() => void) | null = null
      constructor(title: string, options?: { body?: string }) { w.__shown.push({ title, body: options?.body }) }
      close() {}
    }
    ;(window as unknown as { Notification: unknown }).Notification = FakeNotification
  })
  await open(page)
  await page.getByRole('button', { name: '⚙ Settings', exact: true }).click()
  const settings = page.getByRole('dialog', { name: 'Settings' })
  await settings.getByRole('checkbox', { name: /Desktop notifications/ }).check()
  await expect(settings.getByRole('checkbox', { name: /Desktop notifications/ })).toBeChecked()
  await settings.getByRole('button', { name: 'Close settings' }).click()

  await hero(page, 'Opus').click()
  await expect(screenText(page)).toContainText('FAKE-CLAUDE')
  await hero(page, 'Opus').click()
  await expect(expeditions(page).getByRole('listitem')).toHaveCount(2)
  await page.locator('.pane:not([hidden]) .xterm').click()
  await page.keyboard.type('ask me')
  await page.keyboard.press('Enter')
  await expect(screenText(page)).toContainText('Overwrite the file? (y/n)')
  // The user switches to another window.
  await page.evaluate(() => {
    ;(window as unknown as { __front: boolean }).__front = false
    window.dispatchEvent(new Event('blur'))
  })

  const shown = () => page.evaluate(() => (window as unknown as { __shown: unknown[] }).__shown)
  await expect.poll(shown, { timeout: 15_000 }).toHaveLength(1)
  const [note] = (await shown()) as { title: string; body: string }[]
  expect(note.title).toBe('A session is waiting for you')
  expect(note.body).toMatch(/^A session in demo is waiting for your input\.$/)
  await page.waitForTimeout(4000) // several polls later it is still the only one
  expect(await shown()).toHaveLength(1)
})
