import type { EffortTier, Priority, Quest, QuestRef, SessionInfo } from '../api'

/**
 * A quest's state as the map and the list show it: the server's state plus
 * what the app's own sessions add (docs/design/map.md §3, States).
 */
export type MapState = 'ready' | 'running' | 'needs-you' | 'assigned' | 'locked' | 'cleared'

/** Where a quest lives on the map: a dungeon (its epic's number) or the Tavern. */
export type PlaceId = number | 'tavern'

export interface QuestView {
  quest: Quest
  number: number
  state: MapState
  effort?: EffortTier
  creature?: string
  priority: Priority
  /** It has sub-issues in the list, so it is a dungeon or a hall, not a room. */
  epic: boolean
  /** The parent, when the parent is in the list. */
  parent: number | null
  place: PlaceId
  /** The sub-epic directly under the dungeon that holds this quest, if any. */
  hall: number | null
  /** Blockers in the list, open or closed. */
  blockers: number[]
  openBlockers: number[]
  /** Blockers the scope leaves out. */
  outside: QuestRef[]
  /** Quests in the list that this one blocks. */
  blocks: number[]
  /** The live session working on it, when there is one. */
  session?: SessionInfo
}

export interface Place {
  id: PlaceId
  /** The epic's own quest; absent for the Tavern. */
  epic?: QuestView
  title: string
  /** Quests drawn as rooms. */
  rooms: number[]
  /** Sub-epics drawn as halls. */
  halls: number[]
  total: number
  cleared: number
  ready: number
  running: number
  blocked: number
  needsYou: boolean
  /** 1 (burrow) to 5 (citadel), from size and the longest blocker chain. */
  tier: number
  longestChain: number
}

export interface WorldLink {
  from: number
  to: number
  /** How many blocked-by links run from one dungeon into the other. */
  count: number
  /** Every one of those blockers is closed. */
  open: boolean
}

export interface MapModel {
  quests: Map<number, QuestView>
  dungeons: Place[]
  tavern: Place
  links: WorldLink[]
}

export const STATE_WORDS: Record<MapState, string> = {
  ready: 'Ready',
  running: 'In progress',
  'needs-you': 'Needs you',
  assigned: 'Assigned',
  locked: 'Blocked',
  cleared: 'Cleared',
}

/** The live session for each quest of a campaign: needing you beats working. */
function sessionsByQuest(sessions: SessionInfo[], campaignId: string | null): Map<number, SessionInfo> {
  const out = new Map<number, SessionInfo>()
  for (const s of sessions) {
    if (s.campaign !== campaignId || s.quest === undefined || s.exited || s.lost) continue
    const have = out.get(s.quest)
    if (!have || (s.activity === 'needs-you' && have.activity !== 'needs-you')) out.set(s.quest, s)
  }
  return out
}

export function mapState(q: Quest, session?: SessionInfo): MapState {
  if (session) return session.activity === 'needs-you' ? 'needs-you' : 'running'
  switch (q.state) {
    case 'cleared':
      return 'cleared'
    case 'locked':
      return 'locked'
    case 'claimed':
      return 'assigned'
    default:
      return 'ready'
  }
}

/**
 * Builds the view of every quest once, so the map and the list agree. Rules
 * (docs/design/map.md §3, Levels; docs/design/issue-shape.md):
 * - a quest with sub-issues in the list and no parent in the list is a dungeon;
 * - a dungeon's direct child with sub-issues is a hall, deeper parents are
 *   rooms inside their hall;
 * - a quest with no parent in the list and no sub-issues in it goes to the
 *   Tavern, including one whose parent the scope leaves out;
 * - cycles in parent links (which GitHub forbids) cannot loop.
 */
export function buildModel(quests: Quest[], sessions: SessionInfo[] = [], campaignId: string | null = null): MapModel {
  const byNumber = new Map(quests.map((q) => [q.number, q]))
  const live = sessionsByQuest(sessions, campaignId)
  // The parent each quest is read with. Bad data can make parents loop; the
  // lowest-numbered quest of a loop is treated as having no parent, so the
  // loop becomes a dungeon instead of vanishing from the map.
  const parentOf = new Map<number, number>()
  for (const q of quests) if (q.parent && q.parent !== q.number && byNumber.has(q.parent)) parentOf.set(q.number, q.parent)
  for (const q of quests) {
    const loop = [q.number]
    let p = parentOf.get(q.number)
    while (p !== undefined && p !== q.number && !loop.includes(p)) {
      loop.push(p)
      p = parentOf.get(p)
    }
    if (p === q.number && Math.min(...loop) === q.number) parentOf.delete(q.number)
  }
  const children = new Map<number, number[]>()
  for (const [child, parent] of parentOf) children.set(parent, [...(children.get(parent) ?? []), child])
  const blocks = new Map<number, number[]>()
  for (const q of quests) {
    for (const b of q.blockedBy) {
      if (byNumber.has(b.number) && b.number !== q.number) blocks.set(b.number, [...(blocks.get(b.number) ?? []), q.number])
    }
  }

  // The chain of ancestors in the list, nearest first, stopping at a cycle.
  const ancestors = (n: number): number[] => {
    const out: number[] = []
    const seen = new Set([n])
    let p = parentOf.get(n)
    while (p !== undefined && !seen.has(p)) {
      out.push(p)
      seen.add(p)
      p = parentOf.get(p)
    }
    return out
  }

  const views = new Map<number, QuestView>()
  for (const q of quests) {
    const up = ancestors(q.number)
    const epic = (children.get(q.number)?.length ?? 0) > 0
    let place: PlaceId
    let hall: number | null = null
    if (up.length === 0) {
      place = epic ? q.number : 'tavern'
    } else {
      place = up[up.length - 1]
      // The ancestor just under the dungeon is the hall; a direct child is
      // a hall itself when it has children, and then holds no hall.
      if (up.length >= 2) hall = up[up.length - 2]
    }
    const session = live.get(q.number)
    views.set(q.number, {
      quest: q,
      number: q.number,
      state: mapState(q, session),
      effort: q.effort,
      creature: q.creature,
      priority: q.priority ?? 'normal',
      epic,
      parent: up[0] ?? null,
      place,
      hall,
      blockers: q.blockedBy.filter((b) => byNumber.has(b.number) && b.number !== q.number).map((b) => b.number),
      openBlockers: q.blockedBy.filter((b) => byNumber.has(b.number) && !b.closed && b.number !== q.number).map((b) => b.number),
      outside: q.blockedBy.filter((b) => !byNumber.has(b.number)),
      blocks: blocks.get(q.number) ?? [],
      session,
    })
  }

  const dungeonIds = quests.filter((q) => views.get(q.number)!.place === q.number).map((q) => q.number)
  const dungeons = dungeonIds.map((id) => place(id, views))
  const tavern = place('tavern', views)
  return { quests: views, dungeons, tavern, links: worldLinks(views) }
}

/** Is this quest drawn as a room in its place (not the dungeon itself, not a hall)? */
export function isRoom(v: QuestView): boolean {
  if (v.place === v.number) return false
  if (v.epic && v.place !== 'tavern' && v.hall === null && v.parent === v.place) return false
  return true
}

function place(id: PlaceId, views: Map<number, QuestView>): Place {
  const members = [...views.values()].filter((v) => v.place === id && v.number !== id)
  const rooms = members.filter(isRoom).map((v) => v.number)
  const halls = members.filter((v) => !isRoom(v)).map((v) => v.number)
  const counted = members.filter(isRoom)
  const epic = typeof id === 'number' ? views.get(id) : undefined
  const longestChain = longestChainIn(rooms, views)
  const total = counted.length
  return {
    id,
    epic,
    title: epic ? epic.quest.title : 'Tavern',
    rooms,
    halls,
    total,
    cleared: counted.filter((v) => v.state === 'cleared').length,
    ready: counted.filter((v) => v.state === 'ready' || v.state === 'assigned').length,
    running: counted.filter((v) => v.state === 'running' || v.state === 'needs-you').length,
    blocked: counted.filter((v) => v.state === 'locked').length,
    needsYou: counted.some((v) => v.state === 'needs-you') || epic?.state === 'needs-you',
    tier: dungeonTier(total, longestChain),
    longestChain,
  }
}

/**
 * The dungeon's art: its size and its longest blocker chain, never a label
 * (docs/design/issue-shape.md). 1 to 3 quests is a burrow, 30 or more a citadel;
 * a chain of four or more raises it one step.
 */
export function dungeonTier(size: number, longestChain: number): number {
  const bySize = size <= 3 ? 1 : size <= 7 ? 2 : size <= 14 ? 3 : size <= 29 ? 4 : 5
  return Math.min(5, bySize + (longestChain >= 4 ? 1 : 0))
}

/** The longest blocked-by chain among these rooms: its length in quests and its last quest (lowest number on a tie); cycles are cut. */
export function longestChain(rooms: number[], views: Map<number, QuestView>): { length: number; end: number | null } {
  const inside = new Set(rooms)
  const memo = new Map<number, number>()
  const visiting = new Set<number>()
  const depth = (n: number): number => {
    const known = memo.get(n)
    if (known !== undefined) return known
    if (visiting.has(n)) return 0
    visiting.add(n)
    let best = 0
    for (const b of views.get(n)?.blockers ?? []) if (inside.has(b)) best = Math.max(best, depth(b))
    visiting.delete(n)
    memo.set(n, best + 1)
    return best + 1
  }
  let length = 0
  let end: number | null = null
  for (const n of [...rooms].sort((a, b) => a - b)) {
    const d = depth(n)
    if (d > length) {
      length = d
      end = n
    }
  }
  return { length, end }
}

/** The number of quests in the longest blocked-by chain among these rooms; cycles are cut. */
export function longestChainIn(rooms: number[], views: Map<number, QuestView>): number {
  return longestChain(rooms, views).length
}

/** Dungeon A leads to dungeon B when a quest in A blocks a quest in B. */
function worldLinks(views: Map<number, QuestView>): WorldLink[] {
  const by = new Map<string, WorldLink>()
  for (const v of views.values()) {
    if (v.place === 'tavern') continue
    for (const b of v.blockers) {
      const from = views.get(b)!.place
      if (from === 'tavern' || from === v.place) continue
      const key = `${from}>${v.place}`
      const link = by.get(key) ?? { from, to: v.place, count: 0, open: true }
      link.count++
      if (!views.get(b)!.quest.closed) link.open = false
      by.set(key, link)
    }
  }
  return [...by.values()].sort((a, b) => a.from - b.from || a.to - b.to)
}

/** The place to show for a quest: its dungeon or the Tavern; a dungeon's own epic is shown on the world. */
export function placeOf(model: MapModel, n: number): PlaceId | 'world' | null {
  const v = model.quests.get(n)
  if (!v) return null
  if (v.place === v.number) return 'world'
  return v.place
}

export interface ListFilter {
  state: 'all' | 'ready' | 'progress' | 'blocked' | 'cleared'
  place: PlaceId | 'all'
}

export const STATE_FILTERS: { id: ListFilter['state']; label: string }[] = [
  { id: 'all', label: 'All' },
  { id: 'ready', label: 'Ready' },
  { id: 'progress', label: 'In progress' },
  { id: 'blocked', label: 'Blocked' },
  { id: 'cleared', label: 'Cleared' },
]

export function inStateFilter(v: QuestView, f: ListFilter['state']): boolean {
  switch (f) {
    case 'ready':
      return v.state === 'ready'
    case 'progress':
      return v.state === 'running' || v.state === 'needs-you' || v.state === 'assigned'
    case 'blocked':
      return v.state === 'locked'
    case 'cleared':
      return v.state === 'cleared'
    default:
      return true
  }
}

/** Is the quest in this place, counting the dungeon's epic and its halls? */
export function inPlace(v: QuestView, p: ListFilter['place']): boolean {
  return p === 'all' || v.place === p
}

export function stateCounts(views: QuestView[]): Record<ListFilter['state'], number> {
  const out = { all: 0, ready: 0, progress: 0, blocked: 0, cleared: 0 }
  for (const v of views) {
    for (const f of STATE_FILTERS) if (inStateFilter(v, f.id)) out[f.id]++
  }
  return out
}
