import { describe, expect, it } from 'vitest'
import type { SessionInfo } from '../api'
import { quest } from './fixtures'
import { buildModel, dungeonTier, inStateFilter, isRoom, mapState, placeOf, stateCounts } from './model'

const session = (quest: number, over: Partial<SessionInfo> = {}): SessionInfo => ({
  id: `s${quest}`, kind: 'hero', title: 'S', campaign: 'c1', dir: '/d', created: '', exited: false, exitCode: 0, quest, ...over,
})

describe('buildModel', () => {
  // Two epics, a hall, a stray, a blocker across epics and one outside the scope.
  const sample = [
    quest(1, { title: 'Onboarding' }),
    quest(2, { parent: 1, closed: true, state: 'cleared' }),
    quest(3, { parent: 1, blockedBy: [{ number: 2, closed: true }] }),
    quest(11, { title: 'Admin' }),
    quest(12, { parent: 11, state: 'locked', blockedBy: [{ number: 3, closed: false }, { number: 40, title: 'Outside', closed: false }] }),
    quest(14, { parent: 11, title: 'Audit' }),
    quest(15, { parent: 14 }),
    quest(16, { parent: 15 }),
    quest(6, { state: 'claimed', assignees: ['alice'] }),
    quest(7, { parent: 99 }),
    quest(8, { labels: [{ name: 'epic' }], subTotal: 3 }),
  ]

  it('makes top-level epics dungeons and gathers strays in the Tavern', () => {
    const m = buildModel(sample)
    expect(m.dungeons.map((d) => d.id)).toEqual([1, 11])
    expect(m.dungeons[0].title).toBe('Onboarding')
    expect(m.tavern.rooms.sort((a, b) => a - b)).toEqual([6, 7, 8])
  })

  it('puts a quest whose parent is outside the scope in the Tavern', () => {
    const m = buildModel(sample)
    expect(m.quests.get(7)!.place).toBe('tavern')
    expect(m.quests.get(7)!.parent).toBeNull()
  })

  it('keeps an epic-labelled issue without sub-issues in the list as a Tavern room', () => {
    const m = buildModel(sample)
    expect(m.quests.get(8)!.place).toBe('tavern')
    expect(isRoom(m.quests.get(8)!)).toBe(true)
  })

  it('makes a sub-epic a hall and keeps deeper parents as rooms inside it', () => {
    const m = buildModel(sample)
    const admin = m.dungeons[1]
    expect(admin.halls).toEqual([14])
    expect(admin.rooms.sort((a, b) => a - b)).toEqual([12, 15, 16])
    expect(m.quests.get(15)!.hall).toBe(14)
    expect(m.quests.get(16)!.hall).toBe(14)
    expect(m.quests.get(12)!.hall).toBeNull()
  })

  it('derives a line between dungeons from a blocker across them', () => {
    const m = buildModel(sample)
    expect(m.links).toEqual([{ from: 1, to: 11, count: 1, open: false }])
  })

  it('separates blockers in the list, open blockers, blockers outside the scope and what a quest blocks', () => {
    const m = buildModel(sample)
    const v = m.quests.get(12)!
    expect(v.blockers).toEqual([3])
    expect(v.openBlockers).toEqual([3])
    expect(v.outside.map((r) => r.number)).toEqual([40])
    expect(m.quests.get(3)!.blocks).toEqual([12])
  })

  it('counts progress over rooms, not halls or the epic', () => {
    const onboarding = buildModel(sample).dungeons[0]
    expect(onboarding).toMatchObject({ total: 2, cleared: 1, ready: 1, blocked: 0 })
  })

  it('survives parent cycles, which GitHub forbids', () => {
    const m = buildModel([quest(1, { parent: 2 }), quest(2, { parent: 1 }), quest(3, { parent: 3 })])
    expect(m.quests.size).toBe(3)
    expect(m.quests.get(3)!.place).toBe('tavern')
  })

  it('adds the app sessions to the state: needing you wins over working, ended sessions do not count', () => {
    const qs = [quest(1), quest(2), quest(3), quest(4)]
    const sessions = [
      session(1, { activity: 'working' }),
      session(2, { activity: 'working', id: 'a' }),
      session(2, { activity: 'needs-you', id: 'b' }),
      session(3, { exited: true }),
      session(4, { campaign: 'other' }),
    ]
    const m = buildModel(qs, sessions, 'c1')
    expect([1, 2, 3, 4].map((n) => m.quests.get(n)!.state)).toEqual(['running', 'needs-you', 'ready', 'ready'])
  })

  it('marks a dungeon with a waiting session inside it', () => {
    const m = buildModel([quest(1), quest(2, { parent: 1 })], [session(2, { activity: 'needs-you' })], 'c1')
    expect(m.dungeons[0].needsYou).toBe(true)
  })

  it('says where to show each quest', () => {
    const m = buildModel(sample)
    expect(placeOf(m, 1)).toBe('world')
    expect(placeOf(m, 16)).toBe(11)
    expect(placeOf(m, 6)).toBe('tavern')
    expect(placeOf(m, 999)).toBeNull()
  })
})

describe('mapState', () => {
  it('maps every server state, with sessions first', () => {
    expect(mapState(quest(1))).toBe('ready')
    expect(mapState(quest(1, { state: 'locked' }))).toBe('locked')
    expect(mapState(quest(1, { state: 'claimed' }))).toBe('assigned')
    expect(mapState(quest(1, { state: 'cleared' }))).toBe('cleared')
    expect(mapState(quest(1), session(1))).toBe('running')
    expect(mapState(quest(1), session(1, { activity: 'needs-you' }))).toBe('needs-you')
  })
})

describe('dungeonTier', () => {
  it('climbs from burrow to citadel with size, and a long chain raises it a step', () => {
    expect([1, 3, 4, 7, 8, 14, 15, 29, 30, 300].map((n) => dungeonTier(n, 1))).toEqual([1, 1, 2, 2, 3, 3, 4, 4, 5, 5])
    expect(dungeonTier(3, 4)).toBe(2)
    expect(dungeonTier(40, 9)).toBe(5)
  })
})

describe('list filters', () => {
  const m = buildModel([
    quest(1), quest(2, { state: 'locked', blockedBy: [{ number: 1, closed: false }] }), quest(3, { state: 'claimed' }),
    quest(4, { closed: true, state: 'cleared' }), quest(5),
  ], [session(5, { activity: 'needs-you' })], 'c1')
  const views = [...m.quests.values()]

  it('counts each state filter, with a quest counted once per filter', () => {
    expect(stateCounts(views)).toEqual({ all: 5, ready: 1, progress: 2, blocked: 1, cleared: 1 })
  })

  it('puts running, waiting and assigned quests under In progress', () => {
    expect(views.filter((v) => inStateFilter(v, 'progress')).map((v) => v.number)).toEqual([3, 5])
  })
})
