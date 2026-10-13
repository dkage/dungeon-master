import { describe, expect, it } from 'vitest'
import type { Quest } from '../api'
import { buildModel } from './model'
import { quest } from './fixtures'
import { layers, layoutDungeon, layoutTavern, layoutWorld, type Box } from './layout'

const blocked = (...ns: number[]) => ns.map((number) => ({ number, closed: false }))

function overlaps(a: Box, b: Box): boolean {
  return a.x < b.x + b.w && b.x < a.x + a.w && a.y < b.y + b.h && b.y < a.y + a.h
}

/** A dungeon (#1) holding the given quests as its children. */
function dungeon(...children: Quest[]) {
  return buildModel([quest(1, { title: 'Epic' }), ...children.map((c) => ({ ...c, parent: c.parent ?? 1 }))])
}

describe('layers', () => {
  it('puts each node one column after its furthest blocker', () => {
    const deps: Record<number, number[]> = { 2: [1], 3: [2], 4: [1, 3] }
    const { layer, cycle } = layers([1, 2, 3, 4], (n) => deps[n] ?? [])
    expect([1, 2, 3, 4].map((n) => layer.get(n))).toEqual([0, 1, 2, 3])
    expect(cycle.size).toBe(0)
  })

  it('cuts a cycle instead of looping and reports the edge it cut', () => {
    const deps: Record<number, number[]> = { 1: [3], 2: [1], 3: [2] }
    const { layer, cycle } = layers([1, 2, 3], (n) => deps[n] ?? [])
    expect(layer.size).toBe(3)
    expect(cycle.size).toBe(1)
  })
})

describe('layoutDungeon', () => {
  it('lays a chain out left to right and crowns the last room of a chain of three', () => {
    const m = dungeon(quest(2), quest(3, { blockedBy: blocked(2) }), quest(4, { blockedBy: blocked(3) }))
    const l = layoutDungeon(m, 1)
    const x = (n: number) => l.rooms.find((r) => r.n === n)!.x
    expect(x(2)).toBeLessThan(x(3))
    expect(x(3)).toBeLessThan(x(4))
    expect(l.rooms.filter((r) => r.boss).map((r) => r.n)).toEqual([4])
    expect(l.links.map((k) => `${k.from}>${k.to}`)).toEqual(['2>3', '3>4'])
    expect(l.links.every((k) => k.kind === 'block' && !k.open && k.d.startsWith('M'))).toBe(true)
  })

  it('crowns the end of the longest chain, and the lowest number when chains tie', () => {
    const m = dungeon(quest(5), quest(6, { blockedBy: blocked(5) }), quest(7, { blockedBy: blocked(6) }), quest(2), quest(3, { blockedBy: blocked(2) }), quest(4, { blockedBy: blocked(3) }), quest(9))
    expect(layoutDungeon(m, 1).rooms.filter((r) => r.boss).map((r) => r.n)).toEqual([4])
  })

  it('has no boss for a chain shorter than three', () => {
    const l = layoutDungeon(dungeon(quest(2), quest(3, { blockedBy: blocked(2) })), 1)
    expect(l.rooms.some((r) => r.boss)).toBe(false)
  })

  it('stacks a diamond: two middle rooms share a column, the join comes after both', () => {
    const m = dungeon(quest(2), quest(3, { blockedBy: blocked(2) }), quest(4, { blockedBy: blocked(2) }), quest(5, { blockedBy: blocked(3, 4) }))
    const l = layoutDungeon(m, 1)
    const r = (n: number) => l.rooms.find((b) => b.n === n)!
    expect(r(3).layer).toBe(1)
    expect(r(4).layer).toBe(1)
    expect(r(3).y).not.toBe(r(4).y)
    expect(r(5).layer).toBe(2)
    expect(l.links).toHaveLength(4)
  })

  it('draws a cycle without looping and marks it', () => {
    const m = dungeon(quest(2, { blockedBy: blocked(3) }), quest(3, { blockedBy: blocked(2) }))
    const l = layoutDungeon(m, 1)
    expect(l.rooms).toHaveLength(2)
    expect(l.links).toHaveLength(2)
    expect(l.links.filter((k) => k.cycle)).toHaveLength(1)
  })

  it('opens the gate when the blocker is closed', () => {
    const m = dungeon(quest(2, { closed: true, state: 'cleared' }), quest(3, { blockedBy: [{ number: 2, closed: true }] }))
    expect(layoutDungeon(m, 1).links[0].open).toBe(true)
  })

  it('stands every gate on its line, before the blocked room', () => {
    const m = dungeon(quest(2), quest(3, { blockedBy: blocked(2) }), quest(4, { blockedBy: blocked(3) }))
    for (const k of layoutDungeon(m, 1).links) {
      const room = layoutDungeon(m, 1).rooms.find((r) => r.n === k.to)!
      expect(k.gate.x).toBeLessThan(room.x)
      expect(k.gate.x).toBeGreaterThan(room.x - 60)
      expect(Math.abs(k.gate.y - (room.y + room.h / 2))).toBeLessThan(40)
    }
  })

  it('draws a hall around its children, apart from every other room', () => {
    const m = dungeon(
      quest(2), quest(3, { blockedBy: blocked(2) }),
      quest(10, { title: 'Hall' }), quest(11, { parent: 10 }), quest(12, { parent: 10, blockedBy: blocked(11) }),
    )
    const l = layoutDungeon(m, 1)
    expect(l.halls.map((h) => h.n)).toEqual([10])
    const hall = l.halls[0]
    const inHall = l.rooms.filter((r) => r.n === 11 || r.n === 12)
    const outside = l.rooms.filter((r) => r.n === 2 || r.n === 3)
    for (const r of inHall) {
      expect(r.x).toBeGreaterThanOrEqual(hall.x)
      expect(r.x + r.w).toBeLessThanOrEqual(hall.x + hall.w)
      expect(r.y).toBeGreaterThan(hall.y)
      expect(r.y + r.h).toBeLessThanOrEqual(hall.y + hall.h)
    }
    for (const r of outside) expect(overlaps(r, hall)).toBe(false)
    expect(l.rooms.map((r) => r.n)).not.toContain(10)
  })

  it('never overlaps two rooms', () => {
    const many = Array.from({ length: 30 }, (_, i) => quest(i + 2, { blockedBy: i > 3 ? blocked(2 + (i % 4)) : [] }))
    const l = layoutDungeon(dungeon(...many), 1)
    for (let i = 0; i < l.rooms.length; i++) {
      for (let j = i + 1; j < l.rooms.length; j++) expect(overlaps(l.rooms[i], l.rooms[j])).toBe(false)
    }
    for (const r of l.rooms) {
      expect(r.x + r.w).toBeLessThanOrEqual(l.width)
      expect(r.y + r.h).toBeLessThanOrEqual(l.height)
    }
  })

  it('puts blockers from elsewhere and outside the scope in the outskirts, joined by dashed stub links', () => {
    const m = buildModel([
      quest(1), quest(2, { parent: 1, blockedBy: [{ number: 9, closed: false }, { number: 40, title: 'Migrate', closed: true }] }),
      quest(9),
    ])
    const l = layoutDungeon(m, 1)
    expect(l.stubs.map((s) => [s.n, s.where, s.closed])).toEqual([[9, 'tavern', false], [40, 'outside', true]])
    expect(l.stubs[1].title).toBe('Migrate')
    expect(l.links.map((k) => k.kind)).toEqual(['stub', 'stub'])
    expect(l.outskirtsY).not.toBeNull()
    for (const s of l.stubs) {
      expect(s.y).toBeGreaterThan(l.outskirtsY!)
      expect(s.y + s.h).toBeLessThanOrEqual(l.height)
      for (const r of l.rooms) expect(overlaps(s, r)).toBe(false)
    }
  })

  it('gives one stub per blocker even when it blocks several rooms', () => {
    const m = buildModel([quest(1), quest(2, { parent: 1, blockedBy: blocked(9) }), quest(3, { parent: 1, blockedBy: blocked(9) }), quest(9)])
    const l = layoutDungeon(m, 1)
    expect(l.stubs).toHaveLength(1)
    expect(l.links).toHaveLength(2)
  })

  it('is deterministic', () => {
    const m = dungeon(quest(2), quest(3, { blockedBy: blocked(2) }), quest(4, { blockedBy: blocked(2) }))
    expect(layoutDungeon(m, 1)).toEqual(layoutDungeon(m, 1))
  })

  it('is fast for about 300 quests', () => {
    const qs = Array.from({ length: 300 }, (_, i) => quest(i + 2, { blockedBy: i > 5 ? blocked(2 + Math.floor(i / 2), 2 + Math.floor(i / 3)) : [] }))
    const m = dungeon(...qs)
    const t = performance.now()
    const l = layoutDungeon(m, 1)
    expect(l.rooms).toHaveLength(300)
    expect(performance.now() - t).toBeLessThan(500)
  })

  it('is empty for an unknown dungeon', () => {
    expect(layoutDungeon(dungeon(quest(2)), 77).rooms).toEqual([])
  })
})

describe('layoutTavern', () => {
  it('lays strays in a grid, waiting and ready ones first, and joins real blockers', () => {
    const m = buildModel([
      quest(5, { closed: true, state: 'cleared' }), quest(6), quest(7, { state: 'locked', blockedBy: blocked(6) }), quest(8), quest(9),
    ])
    const l = layoutTavern(m, 3)
    expect(l.rooms.map((r) => r.n)).toEqual([6, 8, 9, 7, 5])
    expect(l.rooms[3].y).toBeGreaterThan(l.rooms[0].y)
    expect(l.halls).toEqual([])
    expect(l.links.map((k) => `${k.from}>${k.to}`)).toEqual(['6>7'])
  })
})

describe('layoutWorld', () => {
  it('lays out parent cycles with a cross-epic blocker without failing', () => {
    const m = buildModel([quest(1, { parent: 2 }), quest(2, { parent: 1 }), quest(5, { parent: 2 }), quest(10), quest(11, { parent: 10, blockedBy: blocked(5) })])
    expect(() => layoutWorld(m)).not.toThrow()
  })

  const m = buildModel([
    quest(1), quest(2, { parent: 1 }),
    quest(11), quest(12, { parent: 11, blockedBy: blocked(2) }),
    quest(21), quest(22, { parent: 21, blockedBy: blocked(12) }),
    quest(30),
  ])

  it('places dungeons in the order of work, with a line and its count between them', () => {
    const w = layoutWorld(m)
    const x = (id: number) => w.nodes.find((n) => n.id === id)!.x
    expect(x(1)).toBeLessThan(x(11))
    expect(x(11)).toBeLessThan(x(21))
    expect(w.edges.map((e) => [e.from, e.to, e.count])).toEqual([[1, 11, 1], [11, 21, 1]])
  })

  it('adds the Tavern apart from the dungeons', () => {
    const w = layoutWorld(m)
    const tavern = w.nodes.find((n) => n.id === 'tavern')!
    for (const n of w.nodes) if (n !== tavern) expect(overlaps(n, tavern)).toBe(false)
  })

  it('shows only the Tavern when there are no epics', () => {
    const w = layoutWorld(buildModel([quest(1), quest(2)]))
    expect(w.nodes.map((n) => n.id)).toEqual(['tavern'])
    expect(w.edges).toEqual([])
  })
})
