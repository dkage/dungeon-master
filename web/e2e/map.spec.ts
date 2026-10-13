import { mkdirSync, readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { expect, type Page } from '@playwright/test'
import { setAppearance, test } from './fixtures'

// The map (M5a) on the sample project, whose shape is made to show it:
// two epics with a line between them, a hall, a chain, strays for the
// Tavern and a blocker outside the scope (docs/design/map.md §8).

const here = dirname(fileURLToPath(import.meta.url))
const { url } = JSON.parse(readFileSync(join(here, '.url.json'), 'utf8')) as { url: string }

const panel = (page: Page) => page.getByRole('complementary', { name: 'Quests' })
const map = (page: Page) => panel(page).locator('.map')
const detail = (page: Page) => panel(page).getByRole('region', { name: 'Selected issue detail' })
const crumbs = (page: Page) => panel(page).getByRole('navigation', { name: 'Map level' })
const nav = (page: Page) => page.getByRole('navigation', { name: 'Campaigns' })

async function shot(page: Page, name: string) {
  if (process.env.M5A_SCREENSHOTS !== '1') return
  const dir = join(here, '../../_probe/m5a')
  mkdirSync(dir, { recursive: true })
  await page.screenshot({ path: join(dir, `${name}.png`) })
}

async function openSample(page: Page) {
  await page.goto(url) // exchanges the token for a cookie
  await expect(nav(page)).toBeVisible()
  const made = await page.evaluate(async () => {
    const res = await fetch('/api/onboarding/sample', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: '{}' })
    return res.ok
  })
  expect(made).toBe(true)
  await page.reload()
  await nav(page).getByRole('button', { name: /^Sample/ }).first().click()
  await expect(panel(page).getByRole('button', { name: 'Map', pressed: true })).toBeVisible()
  await expect(panel(page).getByRole('group', { name: 'World map' })).toBeVisible()
}

test.use({ viewport: { width: 1600, height: 1000 } })

test('walks the world, a dungeon, the Tavern and back', async ({ page }) => {
  await openSample(page)
  const onboarding = map(page).getByRole('button', { name: /^Dungeon #1 Ship the onboarding flow, 1 of 4 cleared/ })
  await expect(onboarding).toBeVisible()
  await expect(map(page).getByRole('button', { name: /^Dungeon #11 Build the admin dashboard/ })).toBeVisible()
  await expect(map(page).getByRole('button', { name: /^Tavern/ })).toBeVisible()
  await expect(map(page).locator('.wedge')).toHaveCount(1) // onboarding leads to the admin dashboard
  await shot(page, 'world')

  // A click reads the epic; a double-click enters.
  await onboarding.click()
  await expect(detail(page).getByRole('heading', { name: /#1 Ship the onboarding flow/ })).toBeVisible()
  await onboarding.dblclick()
  await expect(panel(page).getByRole('group', { name: 'Map of Ship the onboarding flow' })).toBeVisible()
  await expect(crumbs(page)).toContainText('World › Ship the onboarding flow · 1 of 4 cleared')
  await expect(map(page).getByRole('button', { name: /^#5 Send the welcome email, Blocked.*end of the longest chain/ })).toBeVisible()
  await expect(map(page).getByRole('button', { name: /^#7 Which email provider should we use\?, open, in the Tavern/ })).toBeVisible()
  await shot(page, 'dungeon')

  // Escape goes up; Enter on the Tavern opens it.
  await map(page).getByRole('button', { name: /^#3 Build the signup form/ }).press('Escape')
  await expect(panel(page).getByRole('group', { name: 'World map' })).toBeVisible()
  await map(page).getByRole('button', { name: /^Tavern/ }).press('Enter')
  await expect(crumbs(page)).toContainText('World › Tavern')
  await expect(map(page).getByRole('button', { name: /^#6 Login button unresponsive on Safari, Assigned, Low effort, assigned to @alice, high priority/ })).toBeVisible()
  await expect(map(page).getByRole('button', { name: /^#7 Which email provider should we use\?, Ready, Unknown effort/ })).toBeVisible()
  await shot(page, 'tavern')
  await crumbs(page).getByRole('button', { name: 'World' }).click()
  await expect(panel(page).getByRole('group', { name: 'World map' })).toBeVisible()
})

test('draws creatures from the bundled art, and shows a blocker outside the scope as a stub', async ({ page }) => {
  await openSample(page)
  await map(page).getByRole('button', { name: /^Dungeon #11/ }).dblclick()
  await expect(map(page).getByRole('button', { name: /^Group #14 Keep an audit trail/ })).toBeVisible()
  await expect(map(page).getByRole('button', { name: /^#13 Suspend a user, Blocked, Moderate effort, blocked by #12, #40/ })).toBeVisible()
  await expect(map(page).getByRole('button', { name: '#40 Migrate the user table, open, outside this scope' })).toBeVisible()
  await expect(map(page).getByRole('button', { name: '#3 Build the signup form, open, in Ship the onboarding flow' })).toBeVisible()
  await expect(map(page).locator('.outskirts')).toContainText('Outskirts · external quests')

  // Every creature strip is served by the app itself.
  const hrefs = await map(page).locator('image.strip').evaluateAll((els) => els.map((e) => e.getAttribute('href')!))
  expect(hrefs.length).toBeGreaterThan(2)
  for (const href of new Set(hrefs)) {
    const res = await page.request.get(new URL(href, page.url()).href)
    expect(res.status()).toBe(200)
    expect(res.headers()['content-type']).toBe('image/png')
  }

  // A stub can be read: the outside issue opens in the detail.
  await map(page).getByRole('button', { name: /^#40 Migrate the user table/ }).click()
  await expect(detail(page).getByRole('heading', { name: /#40 Migrate the user table/ })).toBeVisible()
  await expect(detail(page).getByText(/outside the current scope/)).toBeVisible()
})

test('selects on the map and starts only from the footer; a locked quest cannot start', async ({ page }) => {
  await openSample(page)
  await map(page).getByRole('button', { name: /^Dungeon #1 / }).dblclick()

  // Locked: readable, Start disabled with the reason and the blocker as a link.
  await map(page).getByRole('button', { name: /^#4 Validate email addresses/ }).click()
  await expect(detail(page).getByRole('heading', { name: /#4 Validate email addresses/ })).toBeVisible()
  await expect(detail(page).getByRole('button', { name: 'Review launch' })).toBeDisabled()
  await expect(detail(page).locator('.launch-note')).toContainText('Blocked by #3.')

  // Double-click on a room only selects: no session starts.
  const room = map(page).getByRole('button', { name: /^#3 Build the signup form/ })
  await room.dblclick()
  await expect(room).toHaveAttribute('aria-pressed', 'true')
  await expect(detail(page).getByRole('heading', { name: /#3 Build the signup form/ })).toBeVisible()
  await expect(page.locator('.quest-session-chip')).toHaveCount(0)
  await shot(page, 'selected')

  // Start from the footer.
  await detail(page).getByRole('button', { name: 'Review launch' }).click()
  const plan = page.getByRole('dialog', { name: 'Review launch plan' })
  await plan.getByRole('button', { name: 'Start expedition' }).click()
  await expect(page.locator('.quest-session-chip')).toHaveText('#3')
  // The room now shows the running session.
  await expect(map(page).getByRole('button', { name: /^#3 Build the signup form, In progress/ })).toBeVisible()
  // Close this session, so later tests see #3 as ready again.
  await nav(page).getByRole('button', { name: /^Close #3 Build the signup form/ }).click()
  await expect(map(page).getByRole('button', { name: /^#3 Build the signup form, Ready/ })).toBeVisible()
})

test('keeps the chosen zoom while sessions refresh in the background', async ({ page }) => {
  await openSample(page)
  await map(page).getByRole('button', { name: /^Dungeon #1 / }).dblclick()
  await map(page).getByRole('button', { name: /^#3 Build the signup form/ }).click()
  await detail(page).getByRole('button', { name: 'Review launch' }).click()
  await page.getByRole('dialog', { name: 'Review launch plan' }).getByRole('button', { name: 'Start expedition' }).click()
  await expect(page.locator('.quest-session-chip')).toHaveText('#3')

  const svg = map(page).locator('svg[role="group"]')
  const width = () => svg.evaluate((e) => Number(e.getAttribute('width')))
  const fitted = await width()
  await map(page).getByRole('button', { name: 'Zoom in' }).click()
  await map(page).getByRole('button', { name: 'Zoom in' }).click()
  const zoomed = await width()
  expect(zoomed).toBeGreaterThan(fitted)
  await map(page).locator('.viewport').evaluate((e) => e.scrollBy(40, 40))
  await page.waitForTimeout(4500) // two session refreshes
  expect(await width()).toBe(zoomed)

  await nav(page).getByRole('button', { name: /^Close #3 Build the signup form/ }).click()
  await expect(map(page).getByRole('button', { name: /^#3 Build the signup form, Ready/ })).toBeVisible()
})

test('jumps to a prerequisite in another dungeon, and the list shares the selection', async ({ page }) => {
  await openSample(page)
  await map(page).getByRole('button', { name: /^Dungeon #11/ }).dblclick()
  await map(page).getByRole('button', { name: /^#12 List users/ }).click()
  await detail(page).getByRole('button', { name: 'Jump to prerequisite #3' }).click()
  await expect(crumbs(page)).toContainText('Ship the onboarding flow')
  await expect(map(page).getByRole('button', { name: /^#3 Build the signup form/ })).toHaveAttribute('aria-pressed', 'true')

  await panel(page).getByRole('button', { name: 'List', exact: true }).click()
  const row = panel(page).getByRole('region', { name: 'Quest browser' }).getByRole('button', { name: /#3 Build the signup form/ })
  await expect(row).toHaveAttribute('aria-current', 'true')
  await expect(row).toContainText('Blocks #4, #12')
  await panel(page).getByRole('button', { name: /^Blocked/ }).click()
  await expect(panel(page).locator('.qlist .qitem')).toHaveCount(5) // #4, #5, #12, #13, #16
  await panel(page).getByRole('button', { name: /^All/ }).click()
  await panel(page).getByRole('combobox', { name: 'Dungeon' }).selectOption({ label: 'Tavern (no epic)' })
  await panel(page).getByRole('region', { name: 'Quest browser' }).getByRole('button', { name: /#9 Evaluate rate-limit libraries/ }).click()
  await shot(page, 'list')
  await panel(page).getByRole('button', { name: 'Map', exact: true }).click()
  await expect(crumbs(page)).toContainText('World › Tavern')
  await expect(map(page).getByRole('button', { name: /^#9 Evaluate rate-limit libraries/ })).toHaveAttribute('aria-pressed', 'true')
})

test('still works with every image blocked', async ({ page }) => {
  await page.route(/\.(png|jpe?g)(\?.*)?$/, (route) => route.abort())
  await openSample(page)
  await map(page).getByRole('button', { name: /^Dungeon #1 / }).dblclick()
  const room = map(page).getByRole('button', { name: /^#3 Build the signup form, .*Moderate effort/ })
  await expect(room).toBeVisible()
  await expect(room).toContainText('#3 · Moderate')
  await room.click()
  await expect(detail(page).getByRole('heading', { name: /#3 Build the signup form/ })).toBeVisible()
  await panel(page).getByRole('button', { name: 'List', exact: true }).click()
  await expect(panel(page).getByRole('region', { name: 'Quest browser' }).getByRole('button', { name: /#3 Build the signup form/ })).toContainText('Moderate')
})

test('draws the plain graph under the Neutral theme, with no art', async ({ page }) => {
  await setAppearance(page, { theme: 'neutral' })
  await openSample(page)
  await expect(crumbs(page)).toContainText('All epics')
  await map(page).getByRole('button', { name: /^Epic #1 Ship the onboarding flow/ }).dblclick()
  await expect(crumbs(page)).toContainText('All epics › Ship the onboarding flow')
  await expect(map(page).getByRole('button', { name: /^#4 Validate email addresses, Blocked, Moderate effort, blocked by #3/ })).toBeVisible()
  await expect(map(page).locator('image')).toHaveCount(0)
  await map(page).getByRole('button', { name: /^#4 Validate email addresses/ }).click()
  await detail(page).getByRole('button', { name: 'Jump to prerequisite #3' }).click()
  await expect(map(page).getByRole('button', { name: /^#3 Build the signup form/ })).toHaveAttribute('aria-pressed', 'true')
})

async function listWith(page: Page, theme: 'dungeon' | 'neutral') {
  await setAppearance(page, { theme })
  await openSample(page)
  await panel(page).getByRole('group', { name: 'Show as' }).getByRole('button', { name: 'List' }).click()
  await panel(page).locator('.qlist .qbtn').first().click()
}

test('frames the list rows and the detail in the Dungeon theme', async ({ page }) => {
  await listWith(page, 'dungeon')
  const row = panel(page).locator('.qlist .qbtn').first()
  expect(await row.evaluate((e) => parseFloat(getComputedStyle(e).borderTopWidth))).toBeGreaterThan(0)
  expect(await row.locator('.thumb').evaluate((e) => parseFloat(getComputedStyle(e).borderTopWidth))).toBeGreaterThan(0)
  await expect(row.locator('.statetag')).toHaveCount(1)
  const card = detail(page).locator('.titlecard')
  await expect(card).toBeVisible()
  expect(await card.evaluate((e) => parseFloat(getComputedStyle(e).borderTopWidth))).toBeGreaterThan(0)
  await expect(detail(page).locator('.dmeta .pill').first()).toBeVisible()
  const label = panel(page).locator('.qlist .label').first()
  if (await label.count()) await expect(label.locator('.swatch')).toBeVisible()
})

test('keeps the plain list rows and detail in the Neutral theme', async ({ page }) => {
  await listWith(page, 'neutral')
  const row = panel(page).locator('.qlist .qbtn').first()
  expect(await row.evaluate((e) => parseFloat(getComputedStyle(e).borderTopWidth))).toBe(0)
  expect(await detail(page).locator('.titlecard').evaluate((e) => parseFloat(getComputedStyle(e).borderTopWidth))).toBe(0)
  const label = panel(page).locator('.qlist .label').first()
  if (await label.count()) await expect(label.locator('.swatch')).toBeHidden()
})

test('keeps creatures still when the system asks for reduced motion', async ({ page }) => {
  await page.emulateMedia({ reducedMotion: 'reduce' })
  await openSample(page)
  await map(page).getByRole('button', { name: /^Dungeon #1 / }).dblclick()
  const strip = map(page).locator('image.strip').first()
  await expect(strip).toBeVisible()
  expect(await strip.evaluate((e) => getComputedStyle(e).animationName)).toBe('none')
  await page.emulateMedia({ reducedMotion: 'no-preference' })
  expect(await strip.evaluate((e) => getComputedStyle(e).animationName)).not.toBe('none')
})

test('remembers the split between the map and the detail', async ({ page }) => {
  await openSample(page)
  const splitter = panel(page).getByRole('separator', { name: 'Resize the map and the detail' })
  await splitter.focus()
  await splitter.press('ArrowDown')
  await expect(splitter).toHaveAttribute('aria-valuenow', '57')
  await page.reload()
  await nav(page).getByRole('button', { name: /^Sample/ }).first().click()
  await expect(panel(page).getByRole('separator', { name: 'Resize the map and the detail' })).toHaveAttribute('aria-valuenow', '57')
})

for (const theme of ['dungeon', 'neutral'] as const) {
  test(`finds the panel header and footer controls by role in the ${theme} appearance`, async ({ page }) => {
    await setAppearance(page, { theme })
    await openSample(page)
    const toggle = panel(page).getByRole('group', { name: 'Show as' })
    await toggle.getByRole('button', { name: 'List' }).click()
    await expect(toggle.getByRole('button', { name: 'List', pressed: true })).toBeVisible()
    await toggle.getByRole('button', { name: 'Map' }).click()
    await expect(toggle.getByRole('button', { name: 'Map', pressed: true })).toBeVisible()

    const side = page.locator('.sidebar')
    const framed = (name: string) => side.getByRole('button', { name, exact: true }).evaluate((e) => parseFloat(getComputedStyle(e).borderTopWidth))
    const border = await framed('⚙ Settings')
    if (theme === 'dungeon') expect(border).toBeGreaterThan(0)
    else expect(border).toBe(0)
    expect(await panel(page).locator('.qtitle .gem').isVisible()).toBe(theme === 'dungeon')

    await side.getByRole('button', { name: '＋ Add project' }).click()
    await expect(side.getByRole('button', { name: '× Cancel' })).toBeVisible()
    await side.getByRole('button', { name: '× Cancel' }).click()
    await side.getByRole('button', { name: '⚙ Settings' }).click()
    await expect(page.getByRole('dialog')).toBeVisible()
    await page.keyboard.press('Escape')
    await side.getByRole('button', { name: 'Word guide' }).click()
    await expect(page.getByRole('dialog')).toBeVisible()
    await page.keyboard.press('Escape')
    await expect(side.getByRole('button', { name: 'Getting started' })).toBeVisible()
  })
}

for (const theme of ['dungeon', 'neutral'] as const) {
  test(`keeps the keyboard order of the header and footer controls in the ${theme} appearance`, async ({ page }) => {
    await setAppearance(page, { theme })
    await openSample(page)
    const active = () => page.evaluate(() => {
      const e = document.activeElement as HTMLElement | null
      return e?.getAttribute('aria-label') ?? e?.textContent?.trim() ?? ''
    })
    const tabFrom = async (start: ReturnType<Page['locator']>, steps: number) => {
      await start.focus()
      const seen = [await active()]
      for (let i = 0; i < steps; i++) {
        await page.keyboard.press('Tab')
        seen.push(await active())
      }
      return seen
    }
    expect(await tabFrom(panel(page).getByRole('button', { name: 'Map', exact: true }), 2)).toEqual(['Map', 'List', 'Refresh quests'])
    expect(await tabFrom(page.locator('.sidebar').getByRole('button', { name: '＋ Add project' }), 3)).toEqual(['＋ Add project', '⚙ Settings', 'Word guide', 'Getting started'])
  })
}
