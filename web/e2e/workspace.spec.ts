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
    await expect(term).toContainText('$')
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
