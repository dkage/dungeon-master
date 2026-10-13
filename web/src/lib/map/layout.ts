import type { MapModel, PlaceId, QuestView } from './model'
import { longestChainIn } from './model'

/**
 * Pure layout for the three map levels (docs/design/map.md §5). Same input,
 * same positions; no DOM. Positions are in SVG user units.
 */

export const ROOM = { w: 112, h: 124 }
export const NODE = { w: 168, h: 176 }
export const STUB = { w: 132, h: 44 }
const GAP_X = 64
const GAP_Y = 26
const PAD = 28
const HALL_HEAD = 30
const HALL_PAD = 14
const LANE_GAP = 22
const OUTSKIRTS_HEAD = 34

export interface Box {
  x: number
  y: number
  w: number
  h: number
}

export interface RoomBox extends Box {
  n: number
  layer: number
  boss: boolean
}

export interface HallBox extends Box {
  n: number
}

export interface StubBox extends Box {
  n: number
  title: string
  closed: boolean
  /** Where the blocker is: another place on the map, or outside the scope. */
  where: PlaceId | 'outside'
}

export interface Link {
  from: number
  to: number
  /** "block" joins two rooms; "stub" joins an outskirts stub to its room. */
  kind: 'block' | 'stub'
  /** The blocker is closed, so the gate is open. */
  open: boolean
  /** Part of a cycle, which GitHub should not allow; drawn but not laid out. */
  cycle: boolean
  d: string
  /** Where the gate marker sits, just before the blocked room. */
  gate: { x: number; y: number }
}

export interface Layout {
  width: number
  height: number
  rooms: RoomBox[]
  halls: HallBox[]
  stubs: StubBox[]
  links: Link[]
  /** Top of the outskirts strip, when there are stubs. */
  outskirtsY: number | null
}

export interface WorldNode extends Box {
  id: PlaceId
}

export interface WorldEdge {
  from: number
  to: number
  count: number
  open: boolean
  d: string
  gate: { x: number; y: number }
  label: { x: number; y: number }
}

export interface WorldLayout {
  width: number
  height: number
  nodes: WorldNode[]
  edges: WorldEdge[]
}

/**
 * Longest-path layers over blocked-by edges: a room sits one column right of
 * its furthest blocker. Back edges of a cycle are ignored for layering.
 */
export function layers(nodes: number[], blockersOf: (n: number) => number[]): { layer: Map<number, number>; cycle: Set<string> } {
  const inside = new Set(nodes)
  const layer = new Map<number, number>()
  const cycle = new Set<string>()
  const state = new Map<number, 'visiting' | 'done'>()
  const visit = (n: number): number => {
    if (state.get(n) === 'done') return layer.get(n)!
    state.set(n, 'visiting')
    let l = 0
    for (const b of blockersOf(n)) {
      if (!inside.has(b)) continue
      if (state.get(b) === 'visiting') {
        cycle.add(`${b}>${n}`)
        continue
      }
      l = Math.max(l, visit(b) + 1)
    }
    state.set(n, 'done')
    layer.set(n, l)
    return l
  }
  for (const n of [...nodes].sort((a, b) => a - b)) visit(n)
  return { layer, cycle }
}

function curve(x1: number, y1: number, x2: number, y2: number): string {
  if (x2 > x1 + 8) {
    const mx = (x1 + x2) / 2
    return `M${x1},${y1} C${mx},${y1} ${mx},${y2} ${x2},${y2}`
  }
  // Backwards or straight down: loop around below the rooms.
  const dip = Math.max(y1, y2) + ROOM.h * 0.75
  return `M${x1},${y1} C${x1 + 60},${dip} ${x2 - 60},${dip} ${x2},${y2}`
}

const STATE_ORDER: Record<QuestView['state'], number> = { 'needs-you': 0, running: 1, ready: 2, assigned: 3, locked: 4, cleared: 5 }

/** The dungeon level: rooms in columns by blocker depth, one lane per hall, stubs in the outskirts. */
export function layoutDungeon(model: MapModel, id: number): Layout {
  const place = model.dungeons.find((d) => d.id === id)
  if (!place) return empty()
  return layoutRooms(model, place.rooms, (n) => model.quests.get(n)!.hall, place.halls)
}

/** The Tavern: a plain grid with no halls, lines only for real blockers. */
export function layoutTavern(model: MapModel, columns = 3): Layout {
  const rooms = [...model.tavern.rooms].sort((a, b) => {
    const va = model.quests.get(a)!
    const vb = model.quests.get(b)!
    return STATE_ORDER[va.state] - STATE_ORDER[vb.state] || a - b
  })
  const boxes: RoomBox[] = rooms.map((n, i) => ({
    n,
    layer: i % columns,
    boss: false,
    x: PAD + (i % columns) * (ROOM.w + GAP_X),
    y: PAD + Math.floor(i / columns) * (ROOM.h + GAP_Y),
    ...ROOM,
  }))
  const width = PAD * 2 + Math.max(1, Math.min(columns, rooms.length)) * (ROOM.w + GAP_X) - GAP_X
  const height = PAD * 2 + Math.max(1, Math.ceil(rooms.length / columns)) * (ROOM.h + GAP_Y) - GAP_Y
  return finish(model, boxes, [], width, height, new Set())
}

function layoutRooms(model: MapModel, rooms: number[], hallOf: (n: number) => number | null, hallIds: number[]): Layout {
  const blockersOf = (n: number) => model.quests.get(n)?.blockers ?? []
  const { layer, cycle } = layers(rooms, blockersOf)
  // Lanes: rooms directly in the dungeon first, then each hall in number order.
  const laneKeys = [0, ...[...hallIds].sort((a, b) => a - b)]
  const lanes = new Map<number, number[]>(laneKeys.map((k) => [k, []]))
  for (const n of rooms) {
    const h = hallOf(n) ?? 0
    if (!lanes.has(h)) lanes.set(h, [])
    lanes.get(h)!.push(n)
  }
  const maxLayer = Math.max(0, ...rooms.map((n) => layer.get(n)!))
  const boxes: RoomBox[] = []
  const halls: HallBox[] = []
  const rowOf = new Map<number, number>()
  let y = PAD
  for (const [key, members] of lanes) {
    const isHall = key !== 0
    if (members.length === 0 && !isHall) continue
    const top = y
    if (isHall) y += HALL_HEAD
    // Order each column by the average row of its blockers, then by number.
    const rows = Math.max(1, ...Array.from({ length: maxLayer + 1 }, (_, l) => members.filter((n) => layer.get(n) === l).length))
    for (let l = 0; l <= maxLayer; l++) {
      const col = members.filter((n) => layer.get(n) === l)
      const bary = (n: number) => {
        const rs = blockersOf(n).map((b) => rowOf.get(b)).filter((r): r is number => r !== undefined)
        return rs.length ? rs.reduce((a, b) => a + b, 0) / rs.length : Number.MAX_SAFE_INTEGER
      }
      col.sort((a, b) => bary(a) - bary(b) || a - b)
      col.forEach((n, i) => {
        rowOf.set(n, i)
        boxes.push({ n, layer: l, boss: false, x: PAD + l * (ROOM.w + GAP_X) + (isHall ? HALL_PAD : 0), y: y + i * (ROOM.h + GAP_Y), ...ROOM })
      })
    }
    const lanesHeight = members.length ? rows * (ROOM.h + GAP_Y) - GAP_Y : ROOM.h / 2
    if (isHall) {
      const ls = members.map((n) => layer.get(n)!)
      const from = ls.length ? Math.min(...ls) : 0
      const to = ls.length ? Math.max(...ls) : 0
      halls.push({
        n: key,
        x: PAD + from * (ROOM.w + GAP_X),
        y: top,
        w: (to - from + 1) * (ROOM.w + GAP_X) - GAP_X + HALL_PAD * 2,
        h: HALL_HEAD + lanesHeight + HALL_PAD,
      })
      y = top + HALL_HEAD + lanesHeight + HALL_PAD
    } else {
      y += lanesHeight
    }
    y += LANE_GAP
  }
  const hallPad = halls.length ? HALL_PAD * 2 : 0
  const width = PAD * 2 + (maxLayer + 1) * (ROOM.w + GAP_X) - GAP_X + hallPad
  const height = y - LANE_GAP + PAD
  // The boss: the last quest of the longest chain, when it is three or more long.
  if (longestChainIn(rooms, model.quests) >= 3) {
    const last = boxes.filter((b) => b.layer === maxLayer).sort((a, b) => a.n - b.n)[0]
    if (last) last.boss = true
  }
  return finish(model, boxes, halls, width, height, cycle)
}

/** Adds links and the outskirts strip of stubs for blockers that are not rooms here. */
function finish(model: MapModel, boxes: RoomBox[], halls: HallBox[], width: number, height: number, cycle: Set<string>): Layout {
  const at = new Map(boxes.map((b) => [b.n, b]))
  const links: Link[] = []
  const stubs: StubBox[] = []
  const stubAt = new Map<number, StubBox>()
  let outskirtsY: number | null = null
  const stubFor = (n: number, title: string, closed: boolean, where: StubBox['where']): StubBox => {
    const have = stubAt.get(n)
    if (have) return have
    if (outskirtsY === null) outskirtsY = height
    const i = stubs.length
    const perRow = Math.max(1, Math.floor((width - PAD * 2 + GAP_Y) / (STUB.w + GAP_Y)))
    const s: StubBox = {
      n, title, closed, where,
      x: PAD + (i % perRow) * (STUB.w + GAP_Y),
      y: height + OUTSKIRTS_HEAD + Math.floor(i / perRow) * (STUB.h + GAP_Y),
      ...STUB,
    }
    stubs.push(s)
    stubAt.set(n, s)
    return s
  }
  for (const b of [...boxes].sort((x, y) => x.n - y.n)) {
    const v = model.quests.get(b.n)!
    const tx = b.x
    const ty = b.y + b.h / 2
    for (const blocker of v.blockers) {
      const src = at.get(blocker)
      const bv = model.quests.get(blocker)!
      if (src) {
        links.push({
          from: blocker, to: b.n, kind: 'block', open: bv.quest.closed, cycle: cycle.has(`${blocker}>${b.n}`),
          d: curve(src.x + src.w, src.y + src.h / 2, tx, ty), gate: { x: tx - 14, y: ty },
        })
      } else {
        const s = stubFor(blocker, bv.quest.title, bv.quest.closed, bv.place)
        links.push({
          from: blocker, to: b.n, kind: 'stub', open: bv.quest.closed, cycle: false,
          d: `M${s.x + s.w / 2},${s.y} C${s.x + s.w / 2},${s.y - 40} ${tx - 40},${ty} ${tx},${ty}`, gate: { x: tx - 14, y: ty },
        })
      }
    }
    for (const ref of v.outside) {
      const s = stubFor(ref.number, ref.title ?? '', ref.closed, 'outside')
      links.push({
        from: ref.number, to: b.n, kind: 'stub', open: ref.closed, cycle: false,
        d: `M${s.x + s.w / 2},${s.y} C${s.x + s.w / 2},${s.y - 40} ${tx - 40},${ty} ${tx},${ty}`, gate: { x: tx - 14, y: ty },
      })
    }
  }
  const rows = stubs.length ? Math.floor((stubs.length - 1) / Math.max(1, Math.floor((width - PAD * 2 + GAP_Y) / (STUB.w + GAP_Y)))) + 1 : 0
  const total = stubs.length ? height + OUTSKIRTS_HEAD + rows * (STUB.h + GAP_Y) - GAP_Y + PAD : height
  const minWidth = stubs.length ? PAD * 2 + STUB.w : 0
  return { width: Math.max(width, minWidth), height: total, rooms: boxes, halls, stubs, links, outskirtsY }
}

function empty(): Layout {
  return { width: PAD * 2, height: PAD * 2, rooms: [], halls: [], stubs: [], links: [], outskirtsY: null }
}

/** The world level: one node per dungeon in columns by the order of work, the Tavern below. */
export function layoutWorld(model: MapModel): WorldLayout {
  const ids = model.dungeons.map((d) => d.id as number)
  const into = new Map<number, number[]>()
  for (const l of model.links) into.set(l.to, [...(into.get(l.to) ?? []), l.from])
  const { layer } = layers(ids, (n) => into.get(n) ?? [])
  const maxLayer = Math.max(0, ...ids.map((n) => layer.get(n) ?? 0))
  const nodes: WorldNode[] = []
  const rowOf = new Map<number, number>()
  let rows = 0
  for (let l = 0; l <= maxLayer; l++) {
    const col = ids.filter((n) => layer.get(n) === l)
    const bary = (n: number) => {
      const rs = (into.get(n) ?? []).map((b) => rowOf.get(b)).filter((r): r is number => r !== undefined)
      return rs.length ? rs.reduce((a, b) => a + b, 0) / rs.length : Number.MAX_SAFE_INTEGER
    }
    col.sort((a, b) => bary(a) - bary(b) || a - b)
    col.forEach((n, i) => {
      rowOf.set(n, i)
      nodes.push({ id: n, x: PAD + l * (NODE.w + GAP_X * 1.5), y: PAD + i * (NODE.h + GAP_Y), ...NODE })
    })
    rows = Math.max(rows, col.length)
  }
  const gridBottom = PAD + rows * (NODE.h + GAP_Y)
  const hasTavern = model.tavern.rooms.length > 0 || ids.length === 0
  if (hasTavern) nodes.push({ id: 'tavern', x: PAD, y: ids.length ? gridBottom + GAP_Y : PAD, ...NODE })
  const at = new Map(nodes.map((n) => [n.id, n]))
  const edges: WorldEdge[] = model.links.map((l) => {
    const a = at.get(l.from)!
    const b = at.get(l.to)!
    const x1 = a.x + a.w
    const y1 = a.y + a.h / 2
    const x2 = b.x
    const y2 = b.y + b.h / 2
    return { ...l, d: curve(x1, y1, x2, y2), gate: { x: x2 - 16, y: y2 }, label: { x: (x1 + x2) / 2, y: (y1 + y2) / 2 - 10 } }
  })
  const width = Math.max(...nodes.map((n) => n.x + n.w), NODE.w) + PAD
  const height = Math.max(...nodes.map((n) => n.y + n.h), NODE.h) + PAD
  return { width, height, nodes, edges }
}
