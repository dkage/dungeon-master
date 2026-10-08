import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { expect, test, type Page } from '@playwright/test'

const here = dirname(fileURLToPath(import.meta.url))
const { url, workdir } = JSON.parse(readFileSync(join(here, '.url.json'), 'utf8')) as {
  url: string
  workdir: string
}

async function open(page: Page) {
  await page.goto(url) // exchanges the token for a cookie and redirects
  await expect(page.getByRole('button', { name: 'Opus', exact: true })).toBeEnabled()
  // The tests share one server, so start each from a clean slate.
  const close = page.getByRole('button', { name: /^Close/ })
  while ((await close.count()) > 0) {
    const before = await close.count()
    await close.first().click()
    await expect(close).toHaveCount(before - 1)
  }
  await page.getByPlaceholder('/absolute/path/to/project').fill(workdir)
}

const screenText = (page: Page) => page.locator('.pane:not([hidden]) .xterm-rows')

test('unauthenticated API access is refused, the page itself loads', async ({ playwright, baseURL }) => {
  const anon = await playwright.request.newContext({ baseURL })
  expect((await anon.get('/api/state')).status()).toBe(401)
  expect((await anon.get('/')).status()).toBe(200)
  await anon.dispose()
})

test('launch a hero, see its output, type to it', async ({ page }) => {
  await open(page)
  await page.getByRole('button', { name: 'Opus', exact: true }).click()

  const term = screenText(page)
  await expect(term).toContainText('FAKE-CLAUDE')
  await expect(term).toContainText('arg=[--model]')
  await expect(term).toContainText('arg=[claude-opus-5-5]')
  await expect(term).toContainText(`cwd=`)

  await page.locator('.pane:not([hidden]) .xterm').click()
  await page.keyboard.type('hello dungeon')
  await page.keyboard.press('Enter')
  await expect(term).toContainText('echo:hello dungeon')
})

test('two sessions are independent and a shell exit marks its tab', async ({ page }) => {
  await open(page)
  await page.getByRole('button', { name: 'Opus', exact: true }).click()
  await expect(screenText(page)).toContainText('FAKE-CLAUDE')

  await page.getByRole('button', { name: 'Shell', exact: true }).click()
  await expect(page.getByRole('tab')).toHaveCount(2)
  await page.locator('.pane:not([hidden]) .xterm').click()
  await page.keyboard.type('echo shell-only-marker')
  await page.keyboard.press('Enter')
  await expect(screenText(page)).toContainText('shell-only-marker')
  await expect(screenText(page)).not.toContainText('FAKE-CLAUDE')

  // Back to the hero tab: its own output, none of the shell's.
  await page.getByRole('tab', { name: /Opus/ }).click()
  await expect(screenText(page)).toContainText('FAKE-CLAUDE')
  await expect(screenText(page)).not.toContainText('shell-only-marker')

  // Exit the shell: the tab is marked and keeps its scrollback.
  await page.getByRole('tab', { name: /shell/ }).click()
  await page.locator('.pane:not([hidden]) .xterm').click()
  await page.keyboard.type('exit 3')
  await page.keyboard.press('Enter')
  await expect(page.getByText('exited 3')).toBeVisible()
  await expect(screenText(page)).toContainText('shell-only-marker')
})

test('closing a tab ends the session on the server', async ({ page }) => {
  await open(page)
  await page.getByRole('button', { name: 'Shell', exact: true }).click()
  await expect(page.getByRole('tab')).toHaveCount(1)
  await page.getByRole('button', { name: /^Close/ }).click()
  await expect(page.getByRole('tab')).toHaveCount(0)
  await page.reload()
  await expect(page.getByRole('button', { name: 'Opus', exact: true })).toBeEnabled()
  await expect(page.getByRole('tab')).toHaveCount(0)
})

test('sessions survive a page reload and replay their scrollback', async ({ page }) => {
  await open(page)
  await page.getByRole('button', { name: 'Opus', exact: true }).click()
  await expect(screenText(page)).toContainText('FAKE-CLAUDE')
  await page.reload()
  await expect(page.getByRole('tab', { name: /Opus/ })).toBeVisible()
  await expect(screenText(page)).toContainText('FAKE-CLAUDE')
})

test('a bad directory is reported in the page', async ({ page }) => {
  await open(page)
  await page.getByPlaceholder('/absolute/path/to/project').fill('/definitely/not/here')
  await page.getByRole('button', { name: 'Opus', exact: true }).click()
  await expect(page.getByRole('alert')).toContainText('not an existing directory')
  await expect(page.getByRole('tab')).toHaveCount(0)
})
