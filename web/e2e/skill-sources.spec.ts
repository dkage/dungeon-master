import { execFileSync } from 'node:child_process'
import { mkdirSync, mkdtempSync, readdirSync, readFileSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { expect, type Page } from '@playwright/test'
import { test } from './fixtures'

// Skill sources (#51, #180): register a folder and a repository, map them to a project, change the mapping,
// and show that the project's own files are left alone. Repositories are local fixtures: no network.

const here = dirname(fileURLToPath(import.meta.url))
const { url, workdir } = JSON.parse(readFileSync(join(here, '.url.json'), 'utf8')) as { url: string; workdir: string }

const skill = (name: string, description: string) => `---\nname: ${name}\ndescription: ${description}\n---\n# ${name}\n`
const nav = (page: Page) => page.getByRole('navigation', { name: 'Campaigns' })

function fixtures() {
  const root = mkdtempSync(join(tmpdir(), 'dungeon-skills-'))
  const folder = join(root, 'my-skills')
  mkdirSync(join(folder, 'tdd'), { recursive: true })
  mkdirSync(join(folder, 'review'), { recursive: true })
  writeFileSync(join(folder, 'tdd', 'SKILL.md'), skill('tdd', 'from the folder'))
  writeFileSync(join(folder, 'review', 'SKILL.md'), skill('review', 'review a change'))

  const repo = join(root, 'upstream')
  mkdirSync(join(repo, 'plan'), { recursive: true })
  writeFileSync(join(repo, 'plan', 'SKILL.md'), skill('plan', 'from the repository'))
  writeFileSync(join(repo, 'plan', 'template.md'), 'a template that ships beside the skill')
  const empty = join(root, 'empty.gitconfig')
  writeFileSync(empty, '')
  const git = (...a: string[]) => execFileSync('git', ['-C', repo, '-c', 'user.name=e2e', '-c', 'user.email=e2e@example.test', '-c', 'commit.gpgsign=false', ...a],
    { env: { ...process.env, GIT_CONFIG_GLOBAL: empty, GIT_CONFIG_SYSTEM: empty } }).toString().trim()
  git('init', '-q', '-b', 'main')
  git('add', '.')
  git('commit', '-q', '-m', 'one')
  return { folder, repo, revision: git('rev-parse', 'HEAD') }
}

async function openSettings(page: Page) {
  await page.goto(url)
  const res = await page.request.post('/api/campaigns', { data: { dir: workdir, name: 'demo' } })
  expect([201, 200, 409]).toContain(res.status())
  await page.reload()
  await nav(page).getByRole('button', { name: /^demo/ }).click()
  await page.getByRole('button', { name: 'Settings' }).first().click()
  const section = page.locator('#skill-sources')
  await section.scrollIntoViewIfNeeded()
  return section
}

async function clearSources(page: Page) {
  const { sources } = await (await page.request.get('/api/skill-sources')).json()
  // The guard wants JSON on every write, DELETE included.
  for (const s of sources) {
    const res = await page.request.delete(`/api/skill-sources/${s.id}`, { headers: { 'Content-Type': 'application/json' }, data: {} })
    expect(res.status()).toBe(204)
  }
}

const snapshot = (dir: string): string[] => readdirSync(dir, { recursive: true, withFileTypes: false }).map(String).filter((p) => !p.startsWith('.git')).sort()

test('register a folder and a repository, map them to a project, and leave the project files alone', async ({ page }) => {
  const fx = fixtures()
  await page.goto(url)
  await clearSources(page)
  const before = snapshot(workdir)
  const status = () => execFileSync('git', ['-C', workdir, 'status', '--porcelain']).toString()
  const dirty = status()

  const section = await openSettings(page)
  await expect(section.getByText('No sources yet')).toBeVisible()

  // A folder, typed by path.
  await section.getByRole('button', { name: 'New source' }).click()
  await section.getByLabel('Name').fill('Mine')
  await section.getByLabel('Path').fill(fx.folder)
  await section.getByRole('button', { name: 'Register' }).click()
  const mine = section.getByRole('listitem').filter({ hasText: 'Mine' }).first()
  await expect(mine.getByLabel('2 skills')).toBeVisible()

  // A repository, with the ref left blank: it follows the default branch and shows the exact commit.
  await section.getByRole('button', { name: 'New source' }).click()
  await section.getByLabel('Name').fill('Upstream')
  await section.getByLabel('Kind').selectOption('git')
  await section.getByLabel('URL').fill(fx.repo)
  await section.getByRole('button', { name: 'Register' }).click()
  const upstream = section.getByRole('listitem').filter({ hasText: 'Upstream' }).first()
  await expect(upstream.getByText(`default branch · ${fx.revision.slice(0, 12)}`)).toBeVisible()
  await expect(upstream.getByLabel('1 skills')).toBeVisible()

  // Every skill of the project's catalog carries its source-qualified identity and directory.
  const state = await (await page.request.get('/api/state')).json()
  const camp = state.campaigns.find((c: { name: string }) => c.name === 'demo')
  const catalog = async () => (await (await page.request.get(`/api/campaigns/${camp.id}/skills`)).json()).entries.map((e: { sourceName: string; name: string }) => `${e.sourceName}/${e.name}`)
  expect(await catalog()).toEqual(['Mine/review', 'Mine/tdd', 'Upstream/plan'])

  // Change the mapping for this project only: the repository first, the folder off.
  const group = section.getByRole('group', { name: 'Sources for demo' })
  await expect(group.getByText(/Using the defaults/)).toBeVisible()
  await group.getByLabel('Use Mine in demo').uncheck()
  await expect(group.getByText(/This project has its own list/)).toBeVisible()
  await group.getByRole('button', { name: 'Move Upstream up in demo' }).click()
  await expect.poll(catalog).toEqual(['Upstream/plan'])

  // It survives a reload; the defaults come back on request.
  await page.reload()
  await nav(page).getByRole('button', { name: /^demo/ }).click()
  await page.getByRole('button', { name: 'Settings' }).first().click()
  const again = page.locator('#skill-sources').getByRole('group', { name: 'Sources for demo' })
  await expect(again.getByLabel('Use Mine in demo')).not.toBeChecked()
  await again.getByRole('button', { name: 'Use defaults' }).click()
  await expect.poll(catalog).toEqual(['Mine/review', 'Mine/tdd', 'Upstream/plan'])

  // None of this touched the project.
  expect(snapshot(workdir)).toEqual(before)
  expect(status()).toBe(dirty)
  await clearSources(page)
})

test('a failed refresh keeps the last skills and says why; removing keeps the user folder', async ({ page }) => {
  const fx = fixtures()
  await page.goto(url)
  await clearSources(page)
  const section = await openSettings(page)
  await section.getByRole('button', { name: 'New source' }).click()
  await section.getByLabel('Name').fill('Upstream')
  await section.getByLabel('Kind').selectOption('git')
  await section.getByLabel('URL').fill(fx.repo)
  await section.getByLabel('Ref').fill('main')
  await section.getByRole('button', { name: 'Register' }).click()
  const row = section.getByRole('listitem').filter({ hasText: 'Upstream' }).first()
  await expect(row.getByLabel('1 skills')).toBeVisible()

  // The repository disappears; the refresh fails, and the earlier copy stays in use.
  execFileSync('git', ['-C', fx.repo, 'branch', '-m', 'main', 'moved'])
  await row.getByRole('button', { name: 'Refresh' }).click()
  await expect(row.getByRole('status')).toContainText('Last refresh failed')
  await expect(row.getByRole('status')).toContainText('Showing the version from')
  await expect(row.getByLabel('1 skills')).toBeVisible()

  // A folder source is removed from the list, never from the disk.
  await section.getByRole('button', { name: 'New source' }).click()
  await section.getByLabel('Name').fill('Mine')
  await section.getByLabel('Path').fill(fx.folder)
  await section.getByRole('button', { name: 'Register' }).click()
  const mine = section.getByRole('listitem').filter({ hasText: 'Mine' }).first()
  await mine.getByRole('button', { name: 'Remove' }).click()
  await mine.getByRole('button', { name: 'Remove source' }).click()
  await expect(section.getByRole('listitem').filter({ hasText: 'Mine' })).toHaveCount(0)
  expect(readdirSync(fx.folder).sort()).toEqual(['review', 'tdd'])
  await clearSources(page)
})

test('the section is usable at a narrow width', async ({ page }) => {
  fixtures()
  await page.setViewportSize({ width: 420, height: 800 })
  await page.goto(url)
  await clearSources(page)
  const section = await openSettings(page)
  await section.getByRole('button', { name: 'New source' }).click()
  await expect(section.getByLabel('Name')).toBeVisible()
  const box = await section.getByRole('group', { name: 'Register a source' }).boundingBox()
  expect(box!.x + box!.width).toBeLessThanOrEqual(420)
})
