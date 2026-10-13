import { mkdirSync, readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { expect, type Page } from '@playwright/test'
import { test } from './fixtures'

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
  const pickerRequests: string[] = []
  await page.route('**/api/pick-folder', async (route) => {
    pickerRequests.push(route.request().url())
    await route.fulfill({ json: { cancelled: true } })
  })
  await addCampaign(page, workdir, 'again')
  expect(pickerRequests).toEqual([])
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

test('Shift+Enter sends the new-line sequence while plain Enter still sends a carriage return', async ({ page }) => {
  const sent: string[] = []
  page.on('websocket', (ws) => ws.on('framesent', (f) => {
    if (typeof f.payload !== 'string') sent.push(f.payload.toString('latin1'))
  }))
  await open(page)
  await hero(page, 'Opus').click()
  await expect(screenText(page)).toContainText('FAKE-CLAUDE')
  await page.locator('.pane:not([hidden]) .xterm').click()
  await page.keyboard.type('one')
  await page.keyboard.press('Shift+Enter')
  await page.keyboard.type('two')
  await expect(screenText(page)).toContainText('echo:one')
  await expect(screenText(page)).not.toContainText('echo:two')
  await page.keyboard.press('Enter')
  await expect(screenText(page)).toContainText('echo:two')
  // One input frame per key: the new line is ESC CR, the submit is a bare CR.
  expect(sent).toContain('\x1b\r')
  expect(sent.filter((x) => x === '\r')).toHaveLength(1)
})

// Fires a paste or drop carrying one file (or text) at the terminal, the way the browser would.
async function sendTransfer(page: Page, kind: 'paste' | 'drop', file: { name: string; type: string; bytes: number[] } | { text: string }) {
  await page.locator('.pane:not([hidden]) .term-host').evaluate((host, args) => {
    const dt = new DataTransfer()
    if ('text' in args.file) dt.setData('text/plain', args.file.text)
    else dt.items.add(new File([new Uint8Array(args.file.bytes)], args.file.name, { type: args.file.type }))
    const ev = args.kind === 'paste'
      ? new ClipboardEvent('paste', { clipboardData: dt, bubbles: true, cancelable: true })
      : new DragEvent('drop', { dataTransfer: dt, bubbles: true, cancelable: true })
    host.querySelector('textarea')!.dispatchEvent(ev)
  }, { kind, file })
}
const PNG = [0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0, 0, 0, 0x0d, 0x49, 0x48, 0x44, 0x52, 0, 0, 0, 1, 0, 0, 0, 1, 8, 6, 0, 0, 0]

test('a pasted image or dropped file arrives as its stored path, text paste is unchanged, refusals are explained', async ({ page }) => {
  await open(page)
  await hero(page, 'Opus').click()
  await expect(screenText(page)).toContainText('FAKE-CLAUDE')
  await page.locator('.pane:not([hidden]) .xterm').click()

  await sendTransfer(page, 'paste', { text: 'plain words' })
  await expect(screenText(page)).toContainText('plain words')

  await sendTransfer(page, 'paste', { name: 'shot.png', type: 'image/png', bytes: PNG })
  await expect(screenText(page)).toContainText('-shot.png')
  await sendTransfer(page, 'drop', { name: 'notes.txt', type: 'text/plain', bytes: Array.from(Buffer.from('some notes\n')) })
  await expect(screenText(page)).toContainText('-notes.txt')
  // Nothing was submitted by either insertion.
  await expect(screenText(page)).not.toContainText('echo:plain')

  const note = page.locator('.pane:not([hidden]) .term-note')
  await sendTransfer(page, 'drop', { name: 'tool.bin', type: 'application/octet-stream', bytes: [0x7f, 0x45, 0x4c, 0x46, 2, 1, 1, 0] })
  await expect(note).toContainText('only images, PDF and plain text')
  await sendTransfer(page, 'paste', { name: 'empty.png', type: 'image/png', bytes: [] })
  await expect(note).toContainText('empty.png is empty')
})

test('search finds matches in the scrollback, steps through them and jumps back to the latest output', async ({ page }) => {
  await open(page)
  await hero(page, 'Opus').click()
  await expect(screenText(page)).toContainText('FAKE-CLAUDE')
  await page.locator('.pane:not([hidden]) .xterm').click()
  for (let i = 1; i <= 60; i++) {
    await page.keyboard.type(`line-${i}`)
    await page.keyboard.press('Enter')
  }
  await expect(screenText(page)).toContainText('echo:line-60')

  const pane = page.locator('.pane:not([hidden])')
  const count = pane.locator('.term-count')
  const latest = pane.getByRole('button', { name: 'Jump to latest output' })
  await expect(latest).toHaveCount(0)

  // The shortcut opens the bar with the cursor in it; the agent keeps every other key.
  await page.keyboard.press('Control+Shift+F')
  const box = pane.getByRole('searchbox', { name: 'Search terminal output' })
  await expect(box).toBeFocused()
  await box.fill('echo:line-1')
  await expect(count).toHaveText(/ of 11$/)
  const first = await count.textContent()
  await box.press('Enter')
  await expect(count).not.toHaveText(first!)
  await pane.getByRole('button', { name: 'Previous match' }).click()
  await expect(count).toHaveText(first!)

  await box.fill('no such text anywhere')
  await expect(count).toHaveText('No matches')
  await expect(pane.getByRole('button', { name: 'Next match' })).toBeDisabled()

  // A match far up scrolls the view away from the newest output; one click returns.
  await box.fill('echo:line-1')
  await expect(count).toHaveText(/ of 11$/)
  await expect(latest).toBeVisible()
  await latest.click()
  await expect(latest).toHaveCount(0)
  await expect(screenText(page)).toContainText('echo:line-60')

  // Closing the search returns the keyboard to the session without changing it.
  await box.press('Escape')
  await expect(box).toHaveCount(0)
  await page.keyboard.type('after')
  await page.keyboard.press('Enter')
  await expect(screenText(page)).toContainText('echo:after')
})

test('the text size can be changed from the terminal, resizes the session, keeps the output and survives a reload', async ({ page }) => {
  const sizes: { cols: number; rows: number }[] = []
  page.on('websocket', (ws) => ws.on('framesent', (f) => {
    if (typeof f.payload === 'string') {
      const m = JSON.parse(f.payload) as { type?: string; cols: number; rows: number }
      if (m.type === 'resize') sizes.push({ cols: m.cols, rows: m.rows })
    }
  }))
  await open(page)
  await hero(page, 'Opus').click()
  await expect(screenText(page)).toContainText('FAKE-CLAUDE')
  const pane = page.locator('.pane:not([hidden])')
  const fontSize = () => pane.locator('.xterm-rows').evaluate((e) => getComputedStyle(e).fontSize)
  await expect.poll(fontSize).toBe('13px')
  const before = sizes.at(-1)!

  await pane.getByRole('button', { name: 'Larger text' }).click()
  await pane.getByRole('button', { name: 'Larger text' }).click()
  await expect.poll(fontSize).toBe('15px')
  await expect.poll(() => sizes.at(-1)!.cols).toBeLessThan(before.cols)
  await expect.poll(() => sizes.at(-1)!.rows).toBeLessThan(before.rows)
  // The scrollback and the session are untouched.
  await expect(screenText(page)).toContainText('FAKE-CLAUDE')

  await page.reload()
  await expect(screenText(page)).toContainText('FAKE-CLAUDE')
  await expect.poll(fontSize).toBe('15px')

  await pane.getByRole('button', { name: 'Reset text size (15px)' }).click()
  await expect.poll(fontSize).toBe('13px')
  await expect(pane.getByRole('button', { name: /Reset text size/ })).toHaveCount(0)
})

test('the whole output can be copied and saved as plain text', async ({ page, context }) => {
  await context.grantPermissions(['clipboard-read', 'clipboard-write'])
  await open(page)
  await hero(page, 'Opus').click()
  await expect(screenText(page)).toContainText('FAKE-CLAUDE')
  await page.locator('.pane:not([hidden]) .xterm').click()
  for (let i = 1; i <= 50; i++) {
    await page.keyboard.type(`row-${i}`)
    await page.keyboard.press('Enter')
  }
  await expect(screenText(page)).toContainText('echo:row-50')

  const pane = page.locator('.pane:not([hidden])')
  await pane.getByRole('button', { name: 'Copy output as text' }).click()
  await expect(pane.locator('.term-note')).toHaveText('Output copied.')
  const copied = await page.evaluate(() => navigator.clipboard.readText())
  // Rows scrolled out of view are included, and no control sequences are.
  expect(copied).toContain('FAKE-CLAUDE')
  expect(copied).toContain('echo:row-1\n')
  expect(copied).toContain('echo:row-50')
  // eslint-disable-next-line no-control-regex
  expect(copied).not.toMatch(/[\u0000-\u0008\u000b-\u001f\u007f]/)
  expect(copied).not.toContain('Earlier output is not included')

  const [download] = await Promise.all([
    page.waitForEvent('download'),
    pane.getByRole('button', { name: 'Save output as a text file' }).click(),
  ])
  expect(download.suggestedFilename()).toMatch(/^session-[A-Za-z0-9_-]+-output\.txt$/)
  const path = await download.path()
  expect(readFileSync(path, 'utf8')).toBe(copied + '\n')
})

test('a look chosen in one browser origin shows in another origin of the same server and survives a restart of the page', async ({ page, browser }) => {
  await open(page)
  await page.getByRole('button', { name: 'Settings' }).first().click()
  await page.getByLabel('Reading font').selectOption('eb-garamond')
  await page.getByLabel(/^Theme/).selectOption('neutral')
  await expect(page.locator('.app')).toHaveAttribute('data-font', 'eb-garamond')
  // The save is a request of its own: wait until the server has it.
  await expect.poll(async () => (await (await page.request.get('/api/appearance')).json()).settings.theme).toBe('neutral')

  // localhost is another origin than 127.0.0.1, so it has its own browser storage and cookie.
  const other = await browser.newContext()
  const second = await other.newPage()
  await second.goto(url.replace('127.0.0.1', 'localhost'))
  await expect(second.locator('.app')).toHaveAttribute('data-font', 'eb-garamond')
  await expect(second.locator('.app')).toHaveAttribute('data-theme', 'neutral')
  await other.close()
})

test('the terminal font and line height change on a live session, refit it, and are kept across reloads and origins', async ({ page, browser }) => {
  const sizes: { cols: number; rows: number }[] = []
  page.on('websocket', (ws) => ws.on('framesent', (f) => {
    if (typeof f.payload === 'string') {
      const m = JSON.parse(f.payload) as { type?: string; cols: number; rows: number }
      if (m.type === 'resize') sizes.push({ cols: m.cols, rows: m.rows })
    }
  }))
  await open(page)
  await hero(page, 'Opus').click()
  await expect(screenText(page)).toContainText('FAKE-CLAUDE')
  const pane = page.locator('.pane:not([hidden])')
  const family = () => pane.locator('.xterm-rows').evaluate((e) => getComputedStyle(e).fontFamily)
  const rowHeight = () => pane.locator('.xterm-rows > div').first().evaluate((e) => e.getBoundingClientRect().height)
  await expect.poll(family).toContain('Fira Code')
  const baseRows = sizes.at(-1)!.rows
  const baseHeight = await rowHeight()

  await page.getByRole('button', { name: 'Settings' }).first().click()
  const settings = page.getByRole('dialog', { name: 'Settings' })

  await settings.getByLabel('Terminal font').selectOption('system')
  await expect.poll(family).toContain('ui-monospace')
  await expect.poll(family).not.toContain('Fira Code')
  await expect.poll(family).toMatch(/"Dungeon Symbols", monospace$/)

  await settings.getByLabel('Terminal font').selectOption('custom')
  const name = settings.getByLabel('Installed font name')
  await name.fill('Fira", sans-serif')
  await name.press('Enter')
  await name.blur()
  await expect(settings.getByRole('alert').filter({ hasText: 'Use only letters' })).toBeVisible()
  await expect.poll(family).not.toContain('sans-serif')
  await name.fill('Some Font')
  await name.blur()
  await expect.poll(family).toMatch(/^"Some Font", /)
  await expect.poll(family).toMatch(/"Dungeon Symbols", monospace$/)

  // Denser rows fit more of them, and the session is told.
  await settings.getByLabel('Line height').selectOption('1.15')
  await expect.poll(rowHeight).toBeLessThan(baseHeight)
  await expect.poll(() => sizes.at(-1)!.rows).toBeGreaterThan(baseRows)
  await expect(screenText(page)).toContainText('FAKE-CLAUDE')
  await settings.getByLabel('Line height').selectOption('1.5')
  await expect.poll(rowHeight).toBeGreaterThan(baseHeight)
  await expect.poll(() => sizes.at(-1)!.rows).toBeLessThan(baseRows)

  await expect.poll(async () => (await (await page.request.get('/api/appearance')).json()).settings.terminalLineHeight).toBe(1.5)
  await page.reload()
  await expect(screenText(page)).toContainText('FAKE-CLAUDE')
  await expect.poll(family).toMatch(/^"Some Font", /)
  await expect.poll(rowHeight).toBeGreaterThan(baseHeight)

  const other = await browser.newContext()
  const second = await other.newPage()
  await second.goto(url.replace('127.0.0.1', 'localhost'))
  await second.getByRole('button', { name: /^demo/ }).click()
  await hero(second, 'Opus').click()
  await expect(second.locator('.pane:not([hidden]) .xterm-rows')).toContainText('FAKE-CLAUDE')
  await expect.poll(() => second.locator('.pane:not([hidden]) .xterm-rows').evaluate((e) => getComputedStyle(e).fontFamily)).toMatch(/^"Some Font", /)
  await other.close()
})

test('the terminal colour scheme recolours a live session without restarting it, and is kept across a reload', async ({ page }) => {
  await open(page)
  await hero(page, 'Opus').click()
  await expect(screenText(page)).toContainText('FAKE-CLAUDE')
  const pane = page.locator('.pane:not([hidden])')
  const term = pane.getByTestId('terminal')
  const screenBg = () => pane.locator('.xterm-scrollable-element').evaluate((e) => getComputedStyle(e).backgroundColor)
  const textColour = () => pane.locator('.xterm-rows').evaluate((e) => getComputedStyle(e).color)

  // Default in Dungeon: the parchment shows through a clear terminal.
  await expect(term).toHaveAttribute('data-term-theme', 'dungeon')
  await expect.poll(screenBg).toBe('rgba(237, 175, 83, 0)')

  await page.getByRole('button', { name: 'Settings' }).first().click()
  const colours = page.getByRole('dialog', { name: 'Settings' }).getByLabel('Terminal colours')

  await colours.selectOption('high-contrast')
  await expect(term).toHaveAttribute('data-term-theme', 'high-contrast')
  await expect.poll(screenBg).toBe('rgb(0, 0, 0)')
  await expect.poll(textColour).toBe('rgb(255, 255, 255)')
  await expect(term).toHaveCSS('background-color', 'rgb(0, 0, 0)')
  await expect(term).toHaveCSS('background-image', 'none')

  await colours.selectOption('light')
  await expect.poll(screenBg).toBe('rgb(251, 248, 239)')
  await expect(term).toHaveCSS('background-color', 'rgb(251, 248, 239)')
  // The same session throughout: its output is still there.
  await expect(screenText(page)).toContainText('FAKE-CLAUDE')

  await expect.poll(async () => (await (await page.request.get('/api/appearance')).json()).settings.terminalColors).toBe('light')
  await page.reload()
  await expect(screenText(page)).toContainText('FAKE-CLAUDE')
  await expect.poll(screenBg).toBe('rgb(251, 248, 239)')

  // A scheme ignores the appearance; Default follows it again.
  await page.getByRole('button', { name: 'Settings' }).first().click()
  const settings = page.getByRole('dialog', { name: 'Settings' })
  await settings.getByLabel(/^Theme/).selectOption('neutral')
  await expect.poll(screenBg).toBe('rgb(251, 248, 239)')
  await settings.getByLabel('Terminal colours').selectOption('default')
  await expect.poll(screenBg).toBe('rgb(20, 20, 26)')
})

test('a typed font that is not installed is reported next to the setting, and the notice clears', async ({ page }) => {
  // Detection reads widths in the page; the name only goes to this computer's own settings.
  const foreign: string[] = []
  page.on('request', (r) => {
    if (!r.url().startsWith(new URL(url).origin) && (r.url() + (r.postData() ?? '')).includes('Zzyzx')) foreign.push(r.url())
  })
  await open(page)
  await hero(page, 'Opus').click()
  await expect(screenText(page)).toContainText('FAKE-CLAUDE')
  await page.getByRole('button', { name: 'Settings' }).first().click()
  const settings = page.getByRole('dialog', { name: 'Settings' })
  const notice = settings.getByTestId('font-missing')

  await settings.getByLabel('Terminal font').selectOption('custom')
  await settings.getByLabel('Installed font name').fill('Zzyzx Not Installed')
  await settings.getByLabel('Installed font name').blur()
  await expect(notice).toHaveText('“Zzyzx Not Installed” is not installed on this computer, so the terminal uses Fira Code instead.')
  // The terminal keeps working with the fallback.
  await expect(screenText(page)).toContainText('FAKE-CLAUDE')
  await expect.poll(() => page.locator('.pane:not([hidden]) .xterm-rows').evaluate((e) => getComputedStyle(e).fontFamily)).toContain('Fira Code')

  await settings.getByLabel('Terminal font').selectOption('bundled')
  await expect(notice).toHaveCount(0)

  // Choosing it again brings the notice back; a bundled option clears it.
  await settings.getByLabel('Terminal font').selectOption('custom')
  await expect(notice).toBeVisible()
  await settings.getByLabel('Terminal font').selectOption('system')
  await expect(notice).toHaveCount(0)
  expect(foreign).toEqual([])
})

test('Getting started opens the guide over a session, after dismissal and after a reload', async ({ page }) => {
  await open(page)
  await hero(page, 'Opus').click()
  await expect(screenText(page)).toContainText('FAKE-CLAUDE')
  const guide = page.getByRole('heading', { name: 'Start here' })
  const start = page.locator('.sidebar').getByRole('button', { name: 'Getting started' })

  await start.click()
  await expect(guide).toBeVisible()
  await page.getByRole('button', { name: 'Hide getting started' }).click()
  await expect(guide).toHaveCount(0)
  await expect(screenText(page)).toContainText('FAKE-CLAUDE')

  // Dismissed earlier, and again after a reload.
  await start.click()
  await expect(guide).toBeVisible()
  await page.getByRole('button', { name: 'Hide getting started' }).click()
  await page.reload()
  await expect(start).toBeVisible()
  await start.click()
  await expect(guide).toBeVisible()
  await page.getByRole('button', { name: 'Hide getting started' }).click()
  await expect(guide).toHaveCount(0)
})

test('the settings dialog scrolls its body with a steady scroll bar and a fixed frame', async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 520 })
  await open(page)
  await page.getByRole('button', { name: '⚙ Settings', exact: true }).click()
  const settings = page.getByRole('dialog', { name: 'Settings' })
  const body = settings.getByTestId('settings-body')
  await expect(body).toBeVisible()
  const measure = () =>
    settings.evaluate((dialog) => {
      const b = dialog.querySelector('[data-testid="settings-body"]') as HTMLElement
      const top = (sel: string) => Math.round(dialog.querySelector(sel)!.getBoundingClientRect().top)
      return {
        dialogOverflow: dialog.scrollHeight - dialog.clientHeight,
        scrollable: b.scrollHeight - b.clientHeight,
        bar: b.offsetWidth - b.clientWidth,
        header: top('header'),
        footer: top('footer'),
      }
    })
  const before = await measure()
  expect(before.scrollable).toBeGreaterThan(0)
  // The dialog itself never scrolls and the bar takes real room, so it cannot fade away.
  expect(before.dialogOverflow).toBeLessThanOrEqual(1)
  expect(before.bar).toBeGreaterThan(0)

  await body.evaluate((el) => (el.scrollTop = el.scrollHeight))
  const after = await measure()
  expect(after.header).toBe(before.header)
  expect(after.footer).toBe(before.footer)
  expect(after.bar).toBe(before.bar)
  await expect(settings.getByRole('button', { name: 'Done' })).toBeVisible()
  await settings.getByRole('button', { name: 'Done' }).click()
})

test('the terminal text stays inside its frame once the fonts have loaded', async ({ page }) => {
  await open(page)
  await hero(page, 'Opus').click()
  await expect(screenText(page)).toContainText('FAKE-CLAUDE')
  const pane = page.locator('.pane:not([hidden])')
  await page.evaluate(() => document.fonts.ready)
  // The screen must end before the frame's right padding starts.
  await expect
    .poll(async () =>
      pane.evaluate((el) => {
        const frame = el.querySelector('.term') as HTMLElement
        const screen = el.querySelector('.xterm-screen') as HTMLElement
        const pad = parseFloat(getComputedStyle(frame).paddingRight)
        return Math.round(frame.getBoundingClientRect().right - pad - screen.getBoundingClientRect().right)
      }),
    )
    .toBeGreaterThanOrEqual(-2)
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
  const viewport = pane.locator('.xterm-viewport')
  // Fully transparent; the colour channels carry the parchment tone for xterm's contrast checks.
  const clear = /^rgba\(\d+, \d+, \d+, 0\)$/

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
  // xterm 6 retains a legacy viewport underneath its new scrollable element.
  // A transparent text layer alone does not prove the parchment is visible.
  await expect(viewport).toHaveCSS('background-color', 'rgba(0, 0, 0, 0)')
  await expect(pane.locator('.xterm-scrollable-element')).toHaveCSS('background-color', clear)

  await page.getByRole('button', { name: '⚙ Settings', exact: true }).click()
  const settings = page.getByRole('dialog', { name: 'Settings' })
  await settings.getByLabel(/^Theme/).selectOption('neutral')
  await settings.getByRole('button', { name: 'Close settings' }).click()
  expect(await image(page_)).toBe('none')
  expect(await page_.evaluate((el) => getComputedStyle(el).backgroundColor)).toBe('rgb(20, 20, 26)')
  await expect(pane.locator('.xterm-scrollable-element')).toHaveCSS('background-color', 'rgb(20, 20, 26)')
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
  await expect(viewport).toHaveCSS('background-color', 'rgba(0, 0, 0, 0)')
  await expect(pane.locator('.xterm-scrollable-element')).toHaveCSS('background-color', clear)
  await expect(term).toContainText('echo:line-59')
})

test('gray and dimmed agent text reads at WCAG AA in both appearances', async ({ page }, info) => {
  await open(page)
  await hero(page, 'Opus').click()
  const term = screenText(page)
  await expect(term).toContainText('FAKE-CLAUDE')
  const pane = page.locator('.pane:not([hidden])')
  await pane.locator('.xterm').click()
  await page.keyboard.type('palette')
  await page.keyboard.press('Enter')
  await expect(term).toContainText('dimgray-sample')

  // Contrast of each sample's rendered colour (any alpha composited) on the backdrop.
  const ratios = (backdrop: [number, number, number]) =>
    pane.evaluate((el, bg) => {
      const lin = (v: number) => ((v /= 255) <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4)
      const lum = (c: number[]) => 0.2126 * lin(c[0]) + 0.7152 * lin(c[1]) + 0.0722 * lin(c[2])
      const out: Record<string, number> = {}
      for (const text of ['dim-sample', 'gray-sample', 'dimgray-sample']) {
        // Windows' console drops "dim", so a sample can share a span with the spaces around it.
        const spans = [...el.querySelectorAll('.xterm-rows span')]
        const span = spans.find((s) => s.textContent?.trim() === text) ?? spans.find((s) => s.textContent?.includes(text))!
        const [r, g, b, a = 1] = getComputedStyle(span).color.match(/[\d.]+/g)!.map(Number)
        const c = [r, g, b].map((v, i) => bg[i] + (v - bg[i]) * a)
        const [hi, lo] = [lum(c), lum(bg)].sort((x, y) => y - x)
        out[text] = (hi + 0.05) / (lo + 0.05)
      }
      return out
    }, backdrop)
  const expectReadable = async (backdrop: [number, number, number], shot: string) => {
    for (const [text, ratio] of Object.entries(await ratios(backdrop)))
      expect(ratio, `${text} contrast`).toBeGreaterThanOrEqual(4.5)
    await info.attach(shot, { body: await pane.locator('.term').screenshot({ path: info.outputPath(`${shot}.png`) }), contentType: 'image/png' })
  }

  // termTheme.test.ts proves #edaf53 is the darkest parchment tone behind the terminal.
  await expectReadable([0xed, 0xaf, 0x53], 'dungeon-gray-text')

  await page.getByRole('button', { name: '⚙ Settings', exact: true }).click()
  const settings = page.getByRole('dialog', { name: 'Settings' })
  await settings.getByLabel(/^Theme/).selectOption('neutral')
  await settings.getByRole('button', { name: 'Close settings' }).click()
  await expect(pane.locator('.term')).toHaveCSS('background-color', 'rgb(20, 20, 26)')
  await expectReadable([20, 20, 26], 'neutral-gray-text')
})

test('the terminal keeps its monospace stack, line height and frame padding across fonts and appearances', async ({ page }) => {
  await open(page)
  await hero(page, 'Opus').click()
  await expect(screenText(page)).toContainText('FAKE-CLAUDE')
  const pane = page.locator('.pane:not([hidden])')
  const rows = pane.locator('.xterm-rows')
  await page.getByRole('button', { name: '⚙ Settings', exact: true }).click()
  const settings = page.getByRole('dialog', { name: 'Settings' })

  const expectTypography = async (padding: string) => {
    const family = await rows.evaluate((e) => getComputedStyle(e).fontFamily)
    expect(family).toContain('Fira Code')
    expect(family).toMatch(/monospace$/)
    expect(family).not.toMatch(/Almendra|Garamond|Alegreya|Georgia|serif/i)
    // xterm sizes each row to fontSize x lineHeight (13px x 1.3).
    const rowHeight = await rows.locator('> div').first().evaluate((e) => e.getBoundingClientRect().height)
    expect(rowHeight).toBeGreaterThan(13 * 1.2)
    await expect(pane.locator('.term')).toHaveCSS('padding', padding)
  }

  for (const theme of ['dungeon', 'neutral']) {
    await settings.getByLabel(/^Theme/).selectOption(theme)
    for (const font of ['almendra', 'eb-garamond', 'alegreya']) {
      await settings.getByLabel('Reading font').selectOption(font)
      await expect(page.locator('.app')).toHaveAttribute('data-font', font)
      await expectTypography(theme === 'dungeon' ? '22px 28px' : '8px')
    }
  }
})

test('the slate shows behind both side panels in Dungeon, not in Neutral', async ({ page }) => {
  await open(page)
  const panels = [page.locator('.sidebar'), page.locator('.quests')]
  const image = (loc: (typeof panels)[number]) => loc.evaluate((el) => getComputedStyle(el).backgroundImage)

  for (const panel of panels) expect(await image(panel)).toContain('slate')
  // The texture must actually load (CSP, path), not just be declared.
  const loaded = await page.evaluate(async (css) => {
    const src = /url\("?([^")]+)"?\)/.exec(css)![1]
    return new Promise<boolean>((resolve) => {
      const img = new Image()
      img.onload = () => resolve(img.naturalWidth > 0)
      img.onerror = () => resolve(false)
      img.src = src
    })
  }, await image(panels[0]))
  expect(loaded).toBe(true)
  // The panels carry no red wash of their own: the only layer over the slate is the neutral dim.
  for (const panel of panels) {
    const bg = await image(panel)
    expect(bg).toContain('rgba(0, 0, 0, 0.4)')
    expect(bg.match(/rgba?\(/g)).toHaveLength(2)
  }
  // slate.test.ts checks text contrast against the texture's brightest pixel; re-measure it so a new asset cannot
  // invalidate that check.
  const brightest = await page.evaluate(async (css) => {
    const img = new Image()
    img.src = /url\("?([^")]+)"?\)/.exec(css)![1]
    await img.decode()
    const canvas = document.createElement('canvas')
    canvas.width = img.naturalWidth
    canvas.height = img.naturalHeight
    const ctx = canvas.getContext('2d')!
    ctx.drawImage(img, 0, 0)
    const data = ctx.getImageData(0, 0, canvas.width, canvas.height).data
    let max = 0
    for (let i = 0; i < data.length; i += 4) max = Math.max(max, 0.2126 * data[i] + 0.7152 * data[i + 1] + 0.0722 * data[i + 2])
    return max
  }, await image(panels[0]))
  expect(brightest).toBeLessThanOrEqual(0.2126 * 86 + 0.7152 * 85 + 0.0722 * 83 + 2)
  // A themed heading word sits straight on the slate, without a box of its own.
  const word = page.locator('.sidebar-heading .word-hint').first()
  await expect(word).toHaveCSS('background-image', 'none')
  await expect(word).toHaveCSS('background-color', 'rgba(0, 0, 0, 0)')
  await expect(word).toHaveCSS('padding-left', '0px')

  await page.getByRole('button', { name: '⚙ Settings', exact: true }).click()
  const settings = page.getByRole('dialog', { name: 'Settings' })
  await settings.getByLabel(/^Theme/).selectOption('neutral')
  await settings.getByRole('button', { name: 'Close settings' }).click()
  for (const panel of panels) expect(await image(panel)).toBe('none')

  await page.getByRole('button', { name: '⚙ Settings', exact: true }).click()
  await settings.getByLabel(/^Theme/).selectOption('dungeon')
  await settings.getByRole('button', { name: 'Close settings' }).click()
  for (const panel of panels) expect(await image(panel)).toContain('slate')
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
  await expeditions(page).getByRole('button', { name: /^Scratch terminal/ }).click()
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

  // Close the expedition, then removal succeeds. The row disappears only once the
  // server has ended the process, which takes longer on Windows; removing earlier
  // would still be refused.
  await page.getByRole('button', { name: closeSession }).click()
  await expect(expeditions(page, 'busy')).toHaveCount(0)
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

test('a Nerd Font glyph resolves to the bundled symbols font and keeps the column count', async ({ page }) => {
  test.skip(process.platform === 'win32', 'printf is POSIX')
  await open(page)
  await hero(page, 'Shell').click()
  const pane = page.locator('.pane:not([hidden])')
  await pane.locator('.xterm').click()
  const cols = () => pane.locator('.xterm-screen').evaluate((e) => Math.round(e.getBoundingClientRect().width))
  await page.evaluate(() => document.fonts.ready)
  const before = await cols()
  // U+F015 (house) and U+E0B0 (powerline arrow) live in the private-use area; no installed font has them.
  await page.keyboard.type("printf 'ICON-\\357\\200\\225-\\356\\202\\260-END\\n'")
  await page.keyboard.press('Enter')
  await expect(screenText(page)).toContainText('-END')
  // The machine running the test may have a Nerd Font installed. Rebuild the
  // app's own stack without those names, as on a machine with none, and ask
  // the browser which font draws the glyphs.
  const family = await pane.locator('.xterm-rows').evaluate((e) => getComputedStyle(e).fontFamily)
  const bare = family.split(/,\s*/).filter((n) => !/Nerd Font|NF"?$/.test(n)).join(', ')
  expect(bare).toContain('Dungeon Symbols')
  await page.evaluate((f) => {
    const probe = document.createElement('span')
    probe.id = 'glyph-probe'
    probe.style.fontFamily = f
    probe.textContent = '\uf015\ue0b0'
    document.body.append(probe)
  }, bare)
  const cdp = await page.context().newCDPSession(page)
  await cdp.send('DOM.enable')
  await cdp.send('CSS.enable')
  const { root } = await cdp.send('DOM.getDocument', { depth: 0 })
  const { nodeId } = await cdp.send('DOM.querySelector', { nodeId: root.nodeId, selector: '#glyph-probe' })
  const { fonts } = await cdp.send('CSS.getPlatformFontsForNode', { nodeId })
  expect(fonts.filter((f) => f.glyphCount > 0).map((f) => f.familyName).join('|')).toMatch(/Symbols/i)
  expect(await cols()).toBe(before)
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
  expect(font).toContain('Fira Code')
  expect(font).toMatch(/monospace$/)

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
  // The launch bar picks the hero; reviewing the plan never starts a session.
  const heroPick = detail.getByLabel('Hero')
  const heroBefore = await heroPick.inputValue()
  await heroPick.selectOption({ label: 'Sonnet' })
  await detail.getByRole('button', { name: 'Review launch' }).click()
  const briefing = page.getByRole('dialog', { name: 'Review launch plan' })
  await expect(briefing).toContainText('Sonnet')
  await expect(page.locator('.pane:not([hidden]) .xterm')).toHaveCount(0)
  await briefing.getByRole('button', { name: 'Close briefing' }).click()
  await heroPick.selectOption(heroBefore)
  await detail.getByRole('button', { name: 'Review launch' }).click()
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
  // The long appended prompt fills the screen at the 1.3 line height, so the
  // flag name itself has scrolled off; its closing text and the briefing
  // argument that follows are still visible.
  await expect(screenText(page)).toContainText('stop and tell the user.]')
  await expect(screenText(page)).toContainText('task briefing file at')
  await expect(screenText(page)).toContainText('sample-workspace')
  await expect(page.locator('.quest-session-chip')).toHaveText('#9')
  // The sidebar names the expedition by its issue and the hero running it.
  await expect(expeditions(page, 'Sample (not a real project)').getByRole('button', { name: /^#9 Evaluate rate-limit libraries \(Opus\)/ })).toBeVisible()
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
