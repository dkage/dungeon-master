import { it, vi } from 'vitest'
import { render } from '@testing-library/svelte'
import { tick } from 'svelte'
import { writeFileSync } from 'node:fs'
import QuestMap from '../lib/QuestMap.svelte'
import { AppStore, type Client } from '../lib/store.svelte'
import type { Quest, SessionInfo } from '../lib/api'
import { quest } from '../lib/map/fixtures'

// The same sample shape as QuestMap.test.ts, plus one running and one waiting session.
const sample: Quest[] = [
  quest(1, { title: 'Onboarding flow' }),
  quest(2, { title: 'Welcome screen', parent: 1, closed: true, state: 'cleared', effort: 'trivial', creature: 'dungeon-rat' }),
  quest(3, { title: 'Signup form', parent: 1, blockedBy: [{ number: 2, closed: true }], effort: 'moderate', creature: 'goblin-scout' }),
  quest(4, { title: 'Validate emails', parent: 1, state: 'locked', blockedBy: [{ number: 3, closed: false }], effort: 'moderate', creature: 'skeleton-guard', priority: 'urgent' }),
  quest(5, { title: 'Password reset', parent: 1, effort: 'high', creature: 'crypt-ghoul' }),
  quest(7, { title: 'Terms page', parent: 1, effort: 'low', creature: 'cave-bat', priority: 'high' }),
  quest(11, { title: 'Admin dashboard' }),
  quest(12, { title: 'List users', parent: 11, state: 'locked', blockedBy: [{ number: 3, closed: false }, { number: 40, title: 'Migrate the user table', closed: false }] }),
  quest(14, { title: 'Audit trail', parent: 11 }),
  quest(15, { title: 'Record actions', parent: 14 }),
  quest(6, { title: 'Safari login', state: 'claimed', assignees: ['alice'], effort: 'low', creature: 'cave-spider' }),
]
const session = (id: string, q: number, activity: string): SessionInfo =>
  ({ id, kind: 'hero', title: id, campaign: 'c1', dir: '/p', created: '', exited: false, exitCode: 0, quest: q, activity } as SessionInfo)

function store(level: AppStore['mapLevel']) {
  const s = new AppStore({ getSessions: vi.fn(async () => []) } as unknown as Client)
  s.campaigns = [{ id: 'c1', name: 'Camp', dir: '/p', remote: 'origin', scope: { mode: 'labels', labels: ['tracked'] }, demo: false, repo: 'acme/app' }]
  s.campaignId = 'c1'
  s.quests = { source: 'github', quests: sample, totalOpen: 9, truncated: false, fetchedAt: '', stale: false }
  s.questsFor = 'c1'
  s.panelView = 'map'
  s.sessions = [session('a', 5, 'working'), session('b', 7, 'needs-you')]
  s.mapLevel = level
  return s
}

// Vite serves imported images as /src/assets/...; the bundle ships them under assets/.
const clean = (html: string) => html
  .replace(/ class="([^"]*)"/g, (_, c: string) => ` class="${c.split(/\s+/).filter((x) => !x.startsWith('svelte-')).join(' ')}"`)
  .replace(/<!--[^]*?-->/g, '')
  .replace(/\/src\/assets\//g, 'ASSETS/')

it('captures the map levels', async () => {
  for (const [name, level] of [['dungeon', { kind: 'dungeon', id: 1 }], ['world', { kind: 'world' }]] as const) {
    const { container, unmount } = render(QuestMap, { store: store(level as AppStore['mapLevel']) })
    await tick()
    writeFileSync(`${process.env.DS_CAPTURE_OUT}/map-${name}.html`, clean(container.innerHTML))
    unmount()
  }
})
