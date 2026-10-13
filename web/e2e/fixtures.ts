import { test as base } from '@playwright/test'

// Native dialogs run on the host desktop even when Chromium is headless.
// Stub the chooser for every browser test; tests can override the response.
export const test = base.extend({
  page: async ({ page }, use) => {
    await page.route('**/api/pick-folder', (route) => route.fulfill({ json: { cancelled: true } }))
    await use(page)
  },
})
