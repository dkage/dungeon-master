import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { test as base, type Page } from '@playwright/test'

const here = dirname(fileURLToPath(import.meta.url))

// Every test starts from the default look. The server now keeps the appearance
// on the computer, so one test's theme or font would otherwise reach the next.
export const DEFAULT_APPEARANCE = {
  font: 'almendra', sidebarSize: 14, terminalSize: 13, questsSize: 14, wordMode: 'hints', theme: 'dungeon', notifications: false,
  terminalFont: 'stack', terminalFontName: '', terminalLineHeight: 1.3, terminalColors: 'default',
}

const origin = () => new URL(JSON.parse(readFileSync(join(here, '.url.json'), 'utf8')).url as string)

/** Changes the appearance the server keeps, the way the Settings dialog does. */
export async function setAppearance(page: Page, patch: Record<string, unknown>) {
  const o = origin()
  const res = await page.request.put(`${o.origin}/api/appearance`, {
    data: patch,
    headers: { Authorization: `Bearer ${o.searchParams.get('token')}` },
  })
  if (!res.ok()) throw new Error(`could not set the appearance: ${res.status()}`)
}

// Native dialogs run on the host desktop even when Chromium is headless.
// Stub the chooser for every browser test; tests can override the response.
export const test = base.extend({
  page: async ({ page }, use) => {
    await setAppearance(page, DEFAULT_APPEARANCE)
    await page.route('**/api/pick-folder', (route) => route.fulfill({ json: { cancelled: true } }))
    await use(page)
  },
})

/** Closing a session that is mid-work asks first; answers that question when it is shown. */
export async function confirmStop(page: Page) {
  const stop = page.getByRole('button', { name: 'Stop and close' })
  if (await stop.isVisible()) await stop.click()
}
