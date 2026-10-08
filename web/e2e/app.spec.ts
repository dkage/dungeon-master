import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { expect, test, type Page } from '@playwright/test'

const here = dirname(fileURLToPath(import.meta.url))
const { url, workdir, subdir } = JSON.parse(readFileSync(join(here, '.url.json'), 'utf8')) as {
  url: string
  workdir: string
  subdir: string
}

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
  const close = page.getByRole('button', { name: /^Close / })
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
  await page.getByRole('button', { name: /^Close / }).click()
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
  await page.getByRole('button', { name: /^Close / }).click()
  await page.getByRole('button', { name: 'Remove campaign busy' }).click()
  await page.getByRole('button', { name: 'Remove?' }).click()
  await expect(nav(page).getByRole('button', { name: /^busy/ })).toHaveCount(0)
})

test('closing an expedition ends it on the server', async ({ page }) => {
  await open(page)
  await hero(page, 'Shell').click()
  await expect(expeditions(page).getByRole('listitem')).toHaveCount(1)
  await page.getByRole('button', { name: /^Close / }).click()
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
