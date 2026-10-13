import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { expect, type Page } from '@playwright/test'
import { test } from './fixtures'

// Workspace focus (#64): every region expands and restores while the live session survives.

const here = dirname(fileURLToPath(import.meta.url))
const { url } = JSON.parse(readFileSync(join(here, '.url.json'), 'utf8')) as { url: string }

const panel = (page: Page) => page.getByRole('complementary', { name: 'Quests' })
const nav = (page: Page) => page.getByRole('navigation', { name: 'Campaigns' })
const app = (page: Page) => page.locator('.app')
// The shell's prompt ends in $ on Unix and > in cmd.exe.
const PROMPT = process.platform === 'win32' ? '>' : '$'

async function openSample(page: Page) {
  await page.goto(url)
  await expect(nav(page)).toBeVisible()
  await page.evaluate(() => fetch('/api/onboarding/sample', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: '{}' }))
  await page.reload()
  await nav(page).getByRole('button', { name: /^Sample/ }).first().click()
  await expect(panel(page).getByRole('group', { name: 'World map' })).toBeVisible()
  // The tests share one server: start without sessions left by earlier tests.
  const close = page.getByRole('button', { name: /^Close (?!quests)/ })
  while ((await close.count()) > 0) {
    const before = await close.count()
    await close.first().click()
    await expect(close).toHaveCount(before - 1)
  }
}

for (const size of [{ width: 1600, height: 1000 }, { width: 900, height: 800 }]) {
  test(`expands and restores each region at ${size.width}px, keeping the session`, async ({ page }) => {
    await page.setViewportSize(size)
    await openSample(page)
    await page.getByRole('button', { name: 'Shell', exact: true }).click()
    const term = page.locator('.pane:not([hidden]) .xterm-rows')
    await expect(term).toContainText(PROMPT, { timeout: 25_000 })
    await page.locator('.pane:not([hidden]) .xterm-screen').click()
    await page.keyboard.type('echo keep-me')
    await page.keyboard.press('Enter')
    await expect(term).toContainText('keep-me')

    for (const [region, label] of [['projects', 'Expand projects'], ['terminal', 'Expand terminal'], ['map', 'Expand map and list']] as const) {
      if (region === 'map' && size.width < 1181) await page.getByRole('button', { name: 'Quests', exact: true }).click()
      await page.getByRole('button', { name: label }).first().click()
      await expect(app(page)).toHaveAttribute('data-expanded', region)
      await page.getByRole('button', { name: 'Restore split' }).first().click()
      await expect(app(page)).not.toHaveAttribute('data-expanded')
      await expect(page.getByRole('button', { name: label }).first()).toBeFocused()
    }
    await expect(term).toContainText('keep-me')
  })
}

test('Escape in the terminal stays terminal input; outside it restores the split', async ({ page }) => {
  await page.setViewportSize({ width: 1600, height: 1000 })
  await openSample(page)
  await page.getByRole('button', { name: 'Shell', exact: true }).click()
  await page.locator('.pane:not([hidden]) .xterm-screen').click()
  await page.getByRole('button', { name: 'Expand terminal' }).click()
  await page.locator('.pane:not([hidden]) .xterm-screen').click()
  await page.keyboard.press('Escape')
  await expect(app(page)).toHaveAttribute('data-expanded', 'terminal')
  await page.getByRole('button', { name: 'Restore split' }).focus()
  await page.keyboard.press('Escape')
  await expect(app(page)).not.toHaveAttribute('data-expanded')
})

test('dragging and key-stepping the borders resizes the terminal and survives a reload', async ({ page }) => {
  test.skip(process.platform === 'win32', 'stty and tr are POSIX')
  await page.setViewportSize({ width: 1600, height: 1000 })
  await openSample(page)
  await page.getByRole('button', { name: 'Shell', exact: true }).click()
  const term = page.locator('.pane:not([hidden]) .xterm-rows')
  await expect(term).toContainText(PROMPT, { timeout: 25_000 })
  await page.locator('.pane:not([hidden]) .xterm-screen').click()
  let runs = 0
  const size = async () => {
    runs++
    await page.keyboard.type("echo A$(stty size | tr ' ' x)B")
    await page.keyboard.press('Enter')
    await expect.poll(async () => ((await term.textContent()) ?? '').match(/A\d+x\d+B/g)?.length ?? 0).toBe(runs)
    return (await term.textContent())!.match(/A\d+x\d+B/g)!.at(-1)!
  }
  const before = await size()
  const border = page.getByRole('separator', { name: 'Resize issues panel' })
  const box = (await border.boundingBox())!
  await page.mouse.move(box.x + 3, box.y + 300)
  await page.mouse.down()
  await page.mouse.move(box.x - 150, box.y + 300, { steps: 6 })
  await page.mouse.up()
  await expect(border).toHaveAttribute('aria-valuenow', /^(59\d|6\d\d)$/)
  await page.locator('.pane:not([hidden]) .xterm-screen').click()
  expect(await size()).not.toBe(before)
  await expect(term).toContainText('echo A') // scrollback survived
  const sidebar = page.getByRole('separator', { name: 'Resize projects panel' })
  await sidebar.focus()
  await page.keyboard.press('ArrowRight')
  await expect(sidebar).toHaveAttribute('aria-valuenow', '262')
  await page.reload()
  await expect(page.getByRole('separator', { name: 'Resize projects panel' })).toHaveAttribute('aria-valuenow', '262')
  await expect(page.getByRole('separator', { name: 'Resize issues panel' })).toHaveAttribute('aria-valuenow', /^(59\d|6\d\d)$/)
  await page.setViewportSize({ width: 1000, height: 800 })
  await expect(page.getByRole('separator', { name: 'Resize issues panel' })).toBeHidden()
})

test('a reload returns to the open issue and the selected terminal; stale places fall back', async ({ page }) => {
  await page.setViewportSize({ width: 1600, height: 1000 })
  await openSample(page)
  await page.getByRole('button', { name: 'Shell', exact: true }).click()
  const term = page.locator('.pane:not([hidden]) .xterm-rows')
  await expect(term).toContainText(PROMPT, { timeout: 25_000 })
  await page.locator('.pane:not([hidden]) .xterm-screen').click()
  await page.keyboard.type('echo remembered-output')
  await page.keyboard.press('Enter')
  await expect(term).toContainText('remembered-output')

  await panel(page).getByRole('button', { name: 'List', exact: true }).click()
  await panel(page).getByRole('region', { name: 'Quest browser' }).getByRole('button', { name: /^#\d+ / }).first().click()
  const detail = panel(page).getByRole('region', { name: 'Selected issue detail' })
  const heading = detail.getByRole('heading', { level: 2 }).first()
  await expect(heading).toBeVisible()
  const title = await heading.textContent()
  const hash = await page.evaluate(() => location.hash)
  expect(hash).toMatch(/q=\d+/)
  expect(hash).not.toMatch(/token/i)

  await page.reload()
  await expect(panel(page).getByRole('region', { name: 'Selected issue detail' }).getByRole('heading', { level: 2 }).first()).toHaveText(title!)
  await expect(page.locator('.pane:not([hidden]) .xterm-rows')).toContainText('remembered-output')
  await expect(page.getByRole('list', { name: /^Expeditions in Sample/ }).getByRole('button', { name: /^Scratch terminal/ }).first()).toHaveAttribute('aria-current', 'true')

  // A link to a project that is gone leaves the current view alone, without an error.
  await page.evaluate(() => { location.hash = '#c=nope&s=zzz&q=999999&l=d77' })
  await expect.poll(() => page.evaluate(() => location.hash)).not.toContain('nope')
  await page.reload()
  await expect(nav(page)).toBeVisible()
  await expect(page.getByRole('alert')).toHaveCount(0)

  // A removed issue and a gone session in a known project fall back to the default.
  const campaign = await page.evaluate(() => new URLSearchParams(location.hash.slice(1)).get('c'))
  await page.evaluate((c) => { location.hash = `#c=${c}&s=zzz&q=999999&l=d77` }, campaign)
  await page.reload()
  await expect(nav(page)).toBeVisible()
  await expect(page.getByRole('alert')).toHaveCount(0)
  await expect(panel(page).getByRole('region', { name: 'Selected issue detail' })).toHaveCount(0)
})

test('shortcuts: edit, reload, invoke, disable and reset; typing and terminal keys stay intact', async ({ page }) => {
  await page.setViewportSize({ width: 1600, height: 1000 })
  await openSample(page)
  await page.getByRole('button', { name: 'Shell', exact: true }).click()
  const term = page.locator('.pane:not([hidden]) .xterm-rows')
  await expect(term).toContainText(PROMPT, { timeout: 25_000 })
  await page.locator('.pane:not([hidden]) .xterm-screen').click()

  // A default works from inside the terminal, and ordinary typing still reaches the shell.
  await page.keyboard.press('Control+Shift+2')
  await expect(app(page)).toHaveAttribute('data-expanded', 'terminal')
  await page.keyboard.press('Control+Shift+0')
  await expect(app(page)).not.toHaveAttribute('data-expanded')
  await page.locator('.pane:not([hidden]) .xterm-screen').click()
  await page.keyboard.type('echo typed-ok')
  await page.keyboard.press('Enter')
  await expect(term).toContainText('typed-ok')
  await expect(app(page)).not.toHaveAttribute('data-expanded')

  await page.keyboard.press('ControlOrMeta+,')
  const dialog = page.getByRole('dialog', { name: 'Settings' })
  const row = dialog.getByRole('row', { name: /^Expand map and list/ })
  await dialog.getByRole('button', { name: 'Change shortcut for Expand map and list' }).click()
  await page.keyboard.press('Control+Shift+1')
  await expect(row).toContainText('Already used by "Expand projects"')
  await page.keyboard.press('Control+Shift+m')
  await expect(row).toContainText(/(Ctrl|Control)\+Shift\+m/)
  await dialog.getByRole('button', { name: 'Done' }).click()

  await page.reload()
  await expect(page.getByRole('button', { name: 'Expand terminal' })).toBeVisible()
  await page.keyboard.press('Control+Shift+m')
  await expect(app(page)).toHaveAttribute('data-expanded', 'map')
  await page.keyboard.press('Control+Shift+0')
  await expect(app(page)).not.toHaveAttribute('data-expanded')

  await page.getByRole('button', { name: '⚙ Settings' }).click()
  await dialog.getByRole('button', { name: 'Disable shortcut for Expand map and list' }).click()
  await dialog.getByRole('button', { name: 'Done' }).click()
  await page.keyboard.press('Control+Shift+m')
  await expect(app(page)).not.toHaveAttribute('data-expanded')

  await page.getByRole('button', { name: '⚙ Settings' }).click()
  await dialog.getByRole('button', { name: 'Reset shortcut for Expand map and list' }).click()
  await dialog.getByRole('button', { name: 'Done' }).click()
  await page.keyboard.press('Control+Shift+3')
  await expect(app(page)).toHaveAttribute('data-expanded', 'map')
})

test('removing the current project leaves a clean default place after a reload', async ({ page }) => {
  await page.setViewportSize({ width: 1600, height: 1000 })
  await openSample(page)
  await panel(page).getByRole('button', { name: 'List', exact: true }).click()
  await panel(page).getByRole('region', { name: 'Quest browser' }).getByRole('button', { name: /^#\d+ / }).first().click()
  await expect(panel(page).getByRole('region', { name: 'Selected issue detail' })).toBeVisible()
  expect(await page.evaluate(() => location.hash)).toMatch(/c=.*q=\d+/)

  await page.getByRole('button', { name: 'Remove sample project' }).click()
  await page.getByRole('group', { name: /^Confirm removal of/ }).getByRole('button', { name: 'Remove sample' }).click()
  await expect(nav(page).getByRole('button', { name: /^Sample/ })).toHaveCount(0)
  await expect.poll(() => page.evaluate(() => location.hash)).not.toMatch(/q=\d+/)

  await page.reload()
  await expect(nav(page)).toBeVisible()
  await expect(page.getByRole('alert')).toHaveCount(0)
  await expect(panel(page).getByRole('region', { name: 'Selected issue detail' })).toHaveCount(0)
  expect(await page.evaluate(() => location.hash)).not.toMatch(/q=\d+|s=/)
})
