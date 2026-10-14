import { mkdirSync, mkdtempSync, readFileSync, realpathSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { expect, type Page } from '@playwright/test'
import { confirmStop, test } from './fixtures'

// Choosing a skill and adding direction before reviewing a launch (#50, #66, #181), against the real binary
// and its stub agent. Nothing starts until "Start expedition"; the reviewed briefing is what the agent reads.

const here = dirname(fileURLToPath(import.meta.url))
const { url } = JSON.parse(readFileSync(join(here, '.url.json'), 'utf8')) as { url: string }

const panel = (page: Page) => page.getByRole('complementary', { name: 'Quests' })
const nav = (page: Page) => page.getByRole('navigation', { name: 'Campaigns' })
const screenText = (page: Page) => page.locator('.pane:not([hidden]) .xterm-rows')
const json = { headers: { 'Content-Type': 'application/json' } }

function skillFolder() {
  const root = mkdtempSync(join(tmpdir(), 'dungeon-launch-'))
  const dir = join(root, 'tdd')
  mkdirSync(dir)
  writeFileSync(join(dir, 'SKILL.md'), '---\nname: tdd\ndescription: test first\n---\nWrite the failing test first, then the code.\n')
  writeFileSync(join(dir, 'checklist.md'), 'a checklist that ships beside the skill')
  mkdirSync(join(root, 'review'))
  writeFileSync(join(root, 'review', 'SKILL.md'), '---\nname: review\ndescription: review a change\n---\nRead the diff.\n')
  return { root, dir }
}

async function resetSources(page: Page) {
  const { sources } = await (await page.request.get('/api/skill-sources')).json()
  for (const s of sources) expect((await page.request.delete(`/api/skill-sources/${s.id}`, { ...json, data: {} })).status()).toBe(204)
}

async function register(page: Page, root: string) {
  const res = await page.request.post('/api/skill-sources', { ...json, data: { name: 'Team skills', kind: 'folder', location: root } })
  expect(res.status()).toBe(201)
  return (await res.json()) as { id: string }
}

// Opens the sample project with an issue that can be started, and no sessions left by earlier tests.
async function openIssue(page: Page) {
  await page.goto(url)
  await expect(nav(page)).toBeVisible()
  await page.evaluate(() => fetch('/api/onboarding/sample', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: '{}' }))
  await page.reload()
  await nav(page).getByRole('button', { name: /^Sample/ }).first().click()
  const close = page.getByRole('button', { name: /^Close (?!quests)/ })
  while ((await close.count()) > 0) {
    const before = await close.count()
    await close.first().click()
    await confirmStop(page)
    await expect(close).toHaveCount(before - 1)
  }
  await panel(page).getByRole('group', { name: 'Show as' }).getByRole('button', { name: 'List' }).click()
  await panel(page).getByRole('region', { name: 'Quest browser' }).getByRole('button', { name: /Evaluate rate-limit libraries/ }).click()
  return panel(page).getByRole('region', { name: 'Selected issue detail' })
}

const sessions = async (page: Page) => ((await (await page.request.get('/api/sessions')).json()) as { quest?: number }[]).filter((s) => s.quest)

test('a chip or the Method select chooses the skill; the review shows it; the agent gets exactly the reviewed briefing', async ({ page }) => {
  const fx = skillFolder()
  await page.goto(url)
  await resetSources(page)
  const src = await register(page, fx.root)
  const detail = await openIssue(page)

  // The chips list the project's skills and are disabled until an issue is open (they are enabled here).
  const chips = page.getByRole('group', { name: /for the open issue/ })
  await expect(chips.getByRole('button', { name: 'tdd' })).toBeEnabled()
  await expect(chips.getByRole('button', { name: 'review' })).toBeVisible()

  // Choosing one sets the issue's method in the footer. It starts nothing and writes to no session.
  const method = detail.getByLabel('Method')
  await expect(method).toHaveValue('')
  await chips.getByRole('button', { name: 'tdd' }).click()
  await expect(method).toHaveValue(`${src.id}/tdd`)
  await expect(chips.getByRole('button', { name: 'tdd' })).toHaveAttribute('aria-pressed', 'true')
  expect(await sessions(page)).toHaveLength(0)
  await expect(page.locator('.pane:not([hidden]) .xterm')).toHaveCount(0)

  // None is a choice too, and the review says so.
  await method.selectOption('none')
  await detail.getByRole('button', { name: 'Review launch' }).click()
  const plan = page.getByRole('dialog', { name: 'Review launch plan' })
  await expect(plan.getByText(/No skill was chosen/)).toBeVisible()
  await expect(plan.locator('.brief-content')).not.toContainText('## Method')
  await plan.getByRole('button', { name: 'Close briefing' }).click()

  // The skill and the operator's direction, then the review.
  await method.selectOption({ label: 'tdd' })
  await detail.getByLabel(/Additional direction/).fill('Only touch the rate-limit module.')
  expect(await sessions(page)).toHaveLength(0)
  await detail.getByRole('button', { name: 'Review launch' }).click()
  await expect(plan.getByText(`tdd · Team skills`)).toBeVisible()
  await expect(plan.getByText(/Added to the briefing: the skill's instructions and its directory/)).toBeVisible()
  await expect(plan.getByText('Only touch the rate-limit module.', { exact: true })).toBeVisible()
  const content = plan.locator('.brief-content')
  await expect(content).toContainText('## Operator direction')
  await expect(content).toContainText('Write the failing test first, then the code.')
  const file = (await plan.locator('dt:text-is("Private briefing") + dd').textContent())!.trim()
  const reviewed = readFileSync(file, 'utf8')
  expect(reviewed).toContain('Only touch the rate-limit module.')
  expect(reviewed).toContain('Write the failing test first, then the code.')
  expect(reviewed).toContain(`Skill directory: ${realpathSync(fx.dir)}`) // the skill's own directory is named, so its checklist can be read

  // Changing the direction after a review discards it: the next review is a new briefing file.
  await plan.getByRole('button', { name: 'Close briefing' }).click()
  await detail.getByLabel(/Additional direction/).fill('Only touch the rate-limit module, and keep the API.')
  await detail.getByRole('button', { name: 'Review launch' }).click()
  const second = (await plan.locator('dt:text-is("Private briefing") + dd').textContent())!.trim()
  expect(second).not.toBe(file)
  expect(readFileSync(second, 'utf8')).toContain('and keep the API.')

  // Start: the stub agent receives that very file and read access to the skill's directory.
  await plan.getByRole('button', { name: 'Start expedition' }).click()
  await expect(screenText(page)).toContainText('follow the method it names')
  await expect(screenText(page)).toContainText('--add-dir')
  await expect(screenText(page)).toContainText(second.split('/').pop()!.slice(0, 12))
  expect(readFileSync(second, 'utf8')).toContain('and keep the API.')
  expect(await sessions(page)).toHaveLength(1)
  await resetSources(page)
})

test('a skill whose source is switched off after the review is refused and starts nothing', async ({ page }) => {
  const fx = skillFolder()
  await page.goto(url)
  await resetSources(page)
  const src = await register(page, fx.root)
  const detail = await openIssue(page)

  await detail.getByLabel('Method').selectOption({ label: 'tdd' })
  await detail.getByRole('button', { name: 'Review launch' }).click()
  const plan = page.getByRole('dialog', { name: 'Review launch plan' })
  await expect(plan.getByText('tdd · Team skills')).toBeVisible()

  // Elsewhere, the source is turned off for everyone.
  const off = await page.request.patch(`/api/skill-sources/${src.id}`, { ...json, data: { enabled: false } })
  expect(off.ok()).toBeTruthy()
  await plan.getByRole('button', { name: 'Start expedition' }).click()
  await expect(page.getByText(/no longer available to this project/).first()).toBeVisible()
  expect(await sessions(page)).toHaveLength(0)
  await expect(page.locator('.pane:not([hidden]) .xterm')).toHaveCount(0)

  // The select keeps showing the choice as unavailable; it is not swapped for another skill.
  await plan.getByRole('button', { name: 'Close briefing' }).click()
  await expect(detail.getByLabel('Method')).toHaveValue(`${src.id}/tdd`)
  await expect(detail.getByLabel('Method').getByRole('option', { name: 'Unavailable skill' })).toBeDisabled()
  await resetSources(page)
})

test('the footer links open Settings at the skill sources', async ({ page }) => {
  await page.goto(url)
  await resetSources(page)
  const detail = await openIssue(page)
  await detail.getByRole('button', { name: /sources…$/ }).click()
  const settings = page.getByRole('dialog', { name: 'Settings' })
  await expect(settings).toBeVisible()
  await expect(settings.getByRole('heading', { name: /sources$/ })).toBeInViewport()
})
