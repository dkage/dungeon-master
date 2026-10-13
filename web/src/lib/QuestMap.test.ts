import { DEFAULT_SETTINGS } from './appearance'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { fireEvent, render, screen, within } from '@testing-library/svelte'
import QuestMap from './QuestMap.svelte'
import QuestsPanel from './QuestsPanel.svelte'
import QuestDetail from './QuestDetail.svelte'
import { AppStore, type Client } from './store.svelte'
import type { Campaign, Quest, QuestDetail as QuestDetailT, SessionInfo } from './api'
import { quest } from './map/fixtures'

const camp = (over: Partial<Campaign> = {}): Campaign => ({
  id: 'c1', name: 'Camp', dir: '/p', remote: 'origin', scope: { mode: 'labels', labels: ['tracked'] }, demo: false, repo: 'acme/app', ...over,
})

// The sample shape: two dungeons with a line between them, a hall, a stray and a blocker outside the scope.
const sample: Quest[] = [
  quest(1, { title: 'Onboarding flow' }),
  quest(2, { title: 'Welcome screen', parent: 1, closed: true, state: 'cleared', effort: 'trivial', creature: 'dungeon-rat' }),
  quest(3, { title: 'Signup form', parent: 1, blockedBy: [{ number: 2, closed: true }], effort: 'moderate', creature: 'goblin-scout' }),
  quest(4, { title: 'Validate emails', parent: 1, state: 'locked', blockedBy: [{ number: 3, closed: false }], effort: 'moderate', creature: 'skeleton-guard', priority: 'urgent' }),
  quest(11, { title: 'Admin dashboard' }),
  quest(12, { title: 'List users', parent: 11, state: 'locked', blockedBy: [{ number: 3, closed: false }, { number: 40, title: 'Migrate the user table', closed: false }] }),
  quest(14, { title: 'Audit trail', parent: 11 }),
  quest(15, { title: 'Record actions', parent: 14 }),
  quest(6, { title: 'Safari login', state: 'claimed', assignees: ['alice'], effort: 'low', creature: 'cave-spider' }),
]

function client(): Client {
  return {
    getState: vi.fn(), getSessions: vi.fn(async () => []), createSession: vi.fn(), deleteSession: vi.fn(),
    addCampaign: vi.fn(), updateCampaign: vi.fn(), removeCampaign: vi.fn(),
    getQuests: vi.fn(async () => ({ source: 'github' as const, quests: sample, totalOpen: 7, truncated: false, fetchedAt: '', stale: false })),
    getQuest: vi.fn(async (_c: string, n: number) => detail(n)),
    previewQuestLaunch: vi.fn(), getGithub: vi.fn(), setScope: vi.fn(), getScopeOptions: vi.fn(),
    getProjectSuggestions: vi.fn(async () => []), createSampleCampaign: vi.fn(), pickFolder: vi.fn(), openCampaignFolder: vi.fn(),
    adoptStarterHeroes: vi.fn(), getHeroesConfig: vi.fn(), saveHeroesConfig: vi.fn(),
    setEffortLabels: vi.fn(async (_id: string, labels) => camp({ effortLabels: labels })),
    setEffortOverride: vi.fn(async (_id: string, n: number, tier) => camp({ effortOverrides: tier ? { [n]: tier } : {} })),
    getAppearance: vi.fn(async () => ({ settings: DEFAULT_SETTINGS, stored: false })),
    saveAppearance: vi.fn(async (s) => ({ settings: { ...DEFAULT_SETTINGS, ...s }, stored: true })),
  } as unknown as Client
}

function detail(n: number, over: Partial<QuestDetailT> = {}): QuestDetailT {
  const q = sample.find((x) => x.number === n) ?? quest(n, { title: `Outside ${n}` })
  return { ...q, author: 'alice', body: '', created: '', blocking: [], subIssues: [], commentList: [], ...over }
}

function mapStore(setup: (s: AppStore) => void = () => {}) {
  const s = new AppStore(client())
  s.campaigns = [camp()]
  s.campaignId = 'c1'
  s.heroes = [{ id: 'opus', label: 'Opus' }]
  s.quests = { source: 'github', quests: sample, totalOpen: 7, truncated: false, fetchedAt: '', stale: false }
  s.questsFor = 'c1'
  s.panelView = 'map'
  setup(s)
  return s
}

afterEach(() => {
  vi.restoreAllMocks()
  localStorage.clear()
})

describe('QuestMap world', () => {
  it('shows each dungeon and the Tavern with progress and counts in words', () => {
    render(QuestMap, { store: mapStore() })
    expect(screen.getByRole('group', { name: 'World map' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /^Dungeon #1 Onboarding flow, 1 of 3 cleared, 1 ready, 0 in progress, 1 blocked/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /^Dungeon #11 Admin dashboard/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /^Tavern, 0 of 1 cleared/ })).toBeInTheDocument()
    expect(screen.getByRole('navigation', { name: 'Map level' })).toHaveTextContent('World')
  })

  it('draws the line from the first dungeon into the second, with its count', () => {
    const { container } = render(QuestMap, { store: mapStore() })
    const edges = container.querySelectorAll('.wedge')
    expect(edges).toHaveLength(1)
    expect(edges[0]).toHaveTextContent('1')
  })

  it('uses the dungeon and tavern art, and plain shapes in plain mode', () => {
    const s = mapStore()
    const { container, unmount } = render(QuestMap, { store: s })
    const hrefs = [...container.querySelectorAll('.node image')].map((i) => i.getAttribute('href'))
    expect(hrefs.some((h) => /dungeon-1-burrow/.test(h ?? ''))).toBe(true)
    expect(hrefs.some((h) => /tavern\./.test(h ?? ''))).toBe(true)
    unmount()
    s.appearance = { ...s.appearance, wordMode: 'plain' }
    const plain = render(QuestMap, { store: s })
    expect(plain.container.querySelectorAll('image')).toHaveLength(0)
    expect(screen.getByRole('button', { name: /^Epic #1 Onboarding flow/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /^No epic,/ })).toBeInTheDocument()
  })

  it('selects the epic on click and enters on double-click, Enter or Escape back', async () => {
    const s = mapStore()
    const select = vi.spyOn(s, 'selectQuest')
    render(QuestMap, { store: s })
    const onboarding = screen.getByRole('button', { name: /^Dungeon #1 / })
    await fireEvent.click(onboarding)
    expect(select).toHaveBeenCalledWith(1)
    await fireEvent.dblClick(onboarding)
    expect(s.mapLevel).toEqual({ kind: 'dungeon', id: 1 })
    expect(screen.getByRole('group', { name: 'Map of Onboarding flow' })).toBeInTheDocument()
    expect(screen.getByRole('navigation', { name: 'Map level' })).toHaveTextContent('World › Onboarding flow · 1 of 3 cleared')
    await fireEvent.keyDown(screen.getByRole('button', { name: /^#3 Signup form/ }), { key: 'Escape' })
    expect(s.mapLevel).toEqual({ kind: 'world' })
    await fireEvent.keyDown(screen.getByRole('button', { name: /^Tavern/ }), { key: 'Enter' })
    expect(s.mapLevel).toEqual({ kind: 'tavern' })
    await fireEvent.click(within(screen.getByRole('navigation', { name: 'Map level' })).getByRole('button', { name: 'World' }))
    expect(s.mapLevel).toEqual({ kind: 'world' })
  })

  it('marks a dungeon with a session waiting inside it', () => {
    const waiting: SessionInfo = { id: 's', kind: 'hero', title: 'S', campaign: 'c1', dir: '/p', created: '', exited: false, exitCode: 0, quest: 3, activity: 'needs-you' }
    render(QuestMap, { store: mapStore((s) => (s.sessions = [waiting])) })
    expect(screen.getByRole('button', { name: /^Dungeon #1 .*a session is waiting for you/ })).toBeInTheDocument()
  })
})

describe('QuestMap dungeon', () => {
  const inside = (setup: (s: AppStore) => void = () => {}) => mapStore((s) => {
    s.mapLevel = { kind: 'dungeon', id: 1 }
    setup(s)
  })

  it('names every room with its number, title, state, effort and blockers', () => {
    render(QuestMap, { store: inside() })
    expect(screen.getByRole('button', { name: '#2 Welcome screen, Cleared, Trivial effort' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '#3 Signup form, Ready, Moderate effort' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '#4 Validate emails, Blocked, Moderate effort, blocked by #3, urgent priority, end of the longest chain' })).toBeInTheDocument()
  })

  it('draws each creature from its own strip, crowns the end of a chain of three and shows the priority jewel', () => {
    const { container } = render(QuestMap, { store: inside() })
    const room = (n: number) => container.querySelector(`[data-room="${n}"]`)!
    expect(room(3).querySelector('image.strip')!.getAttribute('href')).toMatch(/goblin-idle-strip/)
    expect(room(4).classList.contains('boss')).toBe(true)
    expect(room(4).querySelector('.gem.urgent')).not.toBeNull()
    expect(room(4).querySelector('.badge.locked')).not.toBeNull()
    expect(room(2).querySelector('.badge.cleared')).not.toBeNull()
  })

  it('draws an unknown effort as the hollow diamond, never a guessed creature', () => {
    const { container } = render(QuestMap, { store: mapStore((s) => (s.mapLevel = { kind: 'dungeon', id: 11 })) })
    const room = container.querySelector('[data-room="12"]')!
    expect(room.querySelector('image')).toBeNull()
    expect(room.querySelector('.unknown')).not.toBeNull()
    expect(room).toHaveAccessibleName(/Unknown effort/)
  })

  it('shows halls as groups and blockers from elsewhere as stubs in the outskirts', () => {
    const { container } = render(QuestMap, { store: mapStore((s) => (s.mapLevel = { kind: 'dungeon', id: 11 })) })
    expect(screen.getByRole('button', { name: /^Group #14 Audit trail/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '#3 Signup form, open, in Onboarding flow' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '#40 Migrate the user table, open, outside this scope' })).toBeInTheDocument()
    expect(container.querySelector('.outskirts')).toHaveTextContent('Outskirts · external quests')
    expect(container.querySelectorAll('.link.stub')).toHaveLength(2)
  })

  it('selects on click and Enter, and never starts a session', async () => {
    const s = inside()
    const select = vi.spyOn(s, 'selectQuest')
    const launch = vi.spyOn(s, 'launchQuest')
    render(QuestMap, { store: s })
    const room = screen.getByRole('button', { name: /^#3 Signup form/ })
    await fireEvent.click(room)
    await fireEvent.dblClick(room)
    await fireEvent.keyDown(room, { key: 'Enter' })
    // A double click is two clicks; it adds no selection of its own.
    expect(select).toHaveBeenCalledTimes(2)
    expect(select).toHaveBeenCalledWith(3)
    expect(launch).not.toHaveBeenCalled()
  })

  it('opens a stub from another dungeon by double-click, moving the map there', async () => {
    const s = mapStore((x) => (x.mapLevel = { kind: 'dungeon', id: 11 }))
    render(QuestMap, { store: s })
    await fireEvent.dblClick(screen.getByRole('button', { name: /^#3 Signup form, open, in Onboarding/ }))
    expect(s.mapLevel).toEqual({ kind: 'dungeon', id: 1 })
    expect(s.questNumber).toBe(3)
  })

  it('lights the links of the selected room', () => {
    const { container } = render(QuestMap, { store: inside((s) => (s.questNumber = 3)) })
    expect(container.querySelector('[data-room="3"]')!.classList.contains('selected')).toBe(true)
    expect(container.querySelector('[data-room="4"]')!.classList.contains('near')).toBe(true)
    expect(container.querySelectorAll('.link.lit')).toHaveLength(2)
  })

  it('shows running and waiting sessions on their rooms', () => {
    const sessions: SessionInfo[] = [
      { id: 'a', kind: 'hero', title: 'A', campaign: 'c1', dir: '/p', created: '', exited: false, exitCode: 0, quest: 3, activity: 'working' },
    ]
    const { container } = render(QuestMap, { store: inside((s) => (s.sessions = sessions)) })
    expect(container.querySelector('[data-room="3"]')!.classList.contains('running')).toBe(true)
    expect(screen.getByRole('button', { name: /^#3 Signup form, In progress/ })).toBeInTheDocument()
  })

  it('drops the art below the readable zoom, keeping the number', async () => {
    const { container } = render(QuestMap, { store: inside() })
    for (let i = 0; i < 4; i++) await fireEvent.click(screen.getByRole('button', { name: 'Zoom out' }))
    expect(container.querySelector('.map')!.classList.contains('far')).toBe(true)
    expect(container.querySelectorAll('image.strip')).toHaveLength(0)
    expect(container.querySelector('[data-room="3"] .big')).toHaveTextContent('#3')
  })

  it('moves focus along the map with the arrow keys', async () => {
    const { container } = render(QuestMap, { store: inside() })
    const from = container.querySelector<SVGElement>('[data-room="2"]')!
    from.focus()
    await fireEvent.keyDown(from, { key: 'ArrowRight' })
    expect(document.activeElement).toBe(container.querySelector('[data-room="3"]'))
  })

  it('falls back to the world when the dungeon is gone after a refresh', () => {
    render(QuestMap, { store: mapStore((s) => (s.mapLevel = { kind: 'dungeon', id: 999 })) })
    expect(screen.getByRole('group', { name: 'World map' })).toBeInTheDocument()
  })
})

describe('QuestMap tavern', () => {
  it('shows strays with their state, and says when there are none', () => {
    render(QuestMap, { store: mapStore((s) => (s.mapLevel = { kind: 'tavern' })) })
    expect(screen.getByRole('button', { name: /^#6 Safari login, Assigned, Low effort, assigned to @alice/ })).toBeInTheDocument()
    expect(screen.getByRole('navigation', { name: 'Map level' })).toHaveTextContent('World › Tavern')
  })
})

describe('QuestsPanel map and list', () => {
  it('opens on the map, switches to the list and remembers the choice; selection is shared', async () => {
    const s = mapStore((x) => {
      x.panelView = 'map'
      x.mapLevel = { kind: 'dungeon', id: 1 }
    })
    render(QuestsPanel, { store: s })
    await fireEvent.click(screen.getByRole('button', { name: /^#3 Signup form/ }))
    expect(s.questNumber).toBe(3)
    await fireEvent.click(screen.getByRole('button', { name: 'List' }))
    expect(localStorage.getItem('dungeon.panel.view')).toBe('list')
    expect(screen.getByRole('button', { name: /#3 Signup form/ })).toHaveAttribute('aria-current', 'true')
    // Picking in the list, then going back to the map, shows that quest's dungeon.
    await fireEvent.click(screen.getByRole('button', { name: /#15 Record actions/ }))
    await fireEvent.click(screen.getByRole('button', { name: 'Map' }))
    expect(s.mapLevel).toEqual({ kind: 'dungeon', id: 11 })
    expect(screen.getByRole('group', { name: 'Map of Admin dashboard' })).toBeInTheDocument()
  })

  it('resizes the split between the map and the detail with the keyboard and remembers it', async () => {
    render(QuestsPanel, { store: mapStore() })
    const splitter = screen.getByRole('separator', { name: 'Resize the map and the detail' })
    expect(splitter).toHaveAttribute('aria-valuenow', '52')
    await fireEvent.keyDown(splitter, { key: 'ArrowDown' })
    expect(splitter).toHaveAttribute('aria-valuenow', '57')
    expect(localStorage.getItem('dungeon.panel.split')).toBe('57')
    await fireEvent.keyDown(splitter, { key: 'End' })
    expect(splitter).toHaveAttribute('aria-valuenow', '80')
  })
})

describe('QuestDetail on the map', () => {
  async function opened(n: number, over: Partial<QuestDetailT> = {}, setup: (s: AppStore) => void = () => {}) {
    const s = mapStore(setup)
    s.questNumber = n
    s.detail = detail(n, over)
    render(QuestDetail, { store: s })
    return s
  }

  it('jumps straight to a single prerequisite, moving the map to it', async () => {
    const s = await opened(12, { blockedBy: [{ number: 3, title: 'Signup form', closed: false }] })
    await fireEvent.click(screen.getByRole('button', { name: 'Jump to prerequisite #3' }))
    expect(s.mapLevel).toEqual({ kind: 'dungeon', id: 1 })
    expect(s.questNumber).toBe(3)
  })

  it('offers a short list when there are several prerequisites', async () => {
    const s = await opened(12)
    await fireEvent.click(screen.getByRole('button', { name: 'Jump to prerequisite' }))
    const list = screen.getByRole('list', { name: 'Prerequisites' })
    expect(within(list).getAllByRole('button').map((b) => b.textContent)).toEqual(['#3', '#40 Migrate the user table'])
    await fireEvent.click(within(list).getByRole('button', { name: '#3' }))
    expect(s.questNumber).toBe(3)
  })

  it('says why a blocked quest cannot start, with each blocker a link', async () => {
    const s = await opened(4)
    const note = screen.getByText(/It can start once that is closed/)
    expect(note).toHaveTextContent('Blocked by ○#3. It can start once that is closed.')
    expect(screen.getByRole('button', { name: /Review launch/ })).toBeDisabled()
    await fireEvent.click(within(note).getByRole('button', { name: '#3' }))
    expect(s.questNumber).toBe(3)
  })

  it('notes who a quest is assigned to', async () => {
    await opened(6)
    expect(screen.getByText('Assigned to @alice.')).toBeInTheDocument()
  })

  it('shows the effort, where it came from, and sets it locally', async () => {
    const s = await opened(3, { effortSource: 'label' })
    const row = screen.getByRole('region', { name: 'Effort' })
    expect(row).toHaveTextContent('Moderate (from its label)')
    const set = vi.spyOn(s, 'setEffortOverride').mockResolvedValue(true)
    await fireEvent.change(within(row).getByRole('combobox', { name: 'Set here' }), { target: { value: 'high' } })
    expect(set).toHaveBeenCalledWith(3, 'high')
  })

  it('says when effort labels disagree instead of picking one', async () => {
    await opened(12, { effort: undefined, effortConflict: ['effort:low', 'effort:high'] })
    expect(screen.getByRole('note')).toHaveTextContent('These labels disagree: effort:low, effort:high.')
  })

  it('maps one of the quest labels to a tier, and removes a mapping', async () => {
    const s = await opened(12, { labels: [{ name: 'size/S' }] }, (x) => (x.campaigns = [camp({ effortLabels: { 'size/xl': 'extreme' } })]))
    const save = vi.spyOn(s, 'setEffortLabels').mockResolvedValue(true)
    await fireEvent.change(screen.getByRole('combobox', { name: 'Label' }), { target: { value: 'size/S' } })
    await fireEvent.change(screen.getByRole('combobox', { name: 'Effort for that label' }), { target: { value: 'low' } })
    await fireEvent.click(screen.getByRole('button', { name: 'Map' }))
    expect(save).toHaveBeenCalledWith({ 'size/xl': 'extreme', 'size/s': 'low' })
    await fireEvent.click(screen.getByRole('button', { name: 'Stop mapping size/xl' }))
    expect(save).toHaveBeenLastCalledWith({})
  })

  it('says when an opened issue is outside the scope', async () => {
    await opened(40)
    expect(screen.getByText(/outside the current scope/)).toBeInTheDocument()
  })
})
