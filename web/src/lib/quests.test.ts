import { describe, expect, it } from 'vitest'
import type { Quest } from './api'
import { counts, labelText, matches, questRows, timeAgo, visibleRows } from './quests'

const q = (number: number, over: Partial<Quest> = {}): Quest => ({
  number, title: `Quest ${number}`, state: 'open', closed: false, url: '', labels: [], assignees: [],
  blockedBy: [], subTotal: 0, subDone: 0, comments: 0, prs: [], updated: '', ...over,
})

describe('questRows', () => {
  it('nests children under their parent, keeping server order', () => {
    const rows = questRows([q(1), q(5), q(2, { parent: 1 }), q(3, { parent: 1 }), q(4, { parent: 2 })])
    expect(rows.map((r) => [r.quest.number, r.depth])).toEqual([[1, 0], [2, 1], [4, 2], [3, 1], [5, 0]])
  })

  it('treats a quest whose parent is not in the list as a root', () => {
    expect(questRows([q(2, { parent: 99 })]).map((r) => [r.quest.number, r.depth])).toEqual([[2, 0]])
  })

  it('survives cycles and self-parents without losing quests', () => {
    const rows = questRows([q(1, { parent: 2 }), q(2, { parent: 1 }), q(3, { parent: 3 })])
    expect(rows.map((r) => r.quest.number).sort()).toEqual([1, 2, 3])
  })

  it('handles an empty list', () => {
    expect(questRows([])).toEqual([])
  })
})

describe('filtering', () => {
  const list = [
    q(1, { title: 'Fix login', labels: [{ name: 'bug' }], assignees: ['alice'] }),
    q(2, { title: 'Add signup', labels: [{ name: 'feature' }], closed: true, state: 'cleared', parent: 1 }),
    q(12, { title: 'Docs' }),
  ]

  it('matches title, label, assignee and number, case-insensitively', () => {
    expect(matches(list[0], 'LOGIN')).toBe(true)
    expect(matches(list[0], 'bug')).toBe(true)
    expect(matches(list[0], 'ALI')).toBe(true)
    expect(matches(list[0], '#1')).toBe(true)
    expect(matches(list[2], '1')).toBe(false) // exact number only, not a prefix of 12
    expect(matches(list[2], '#12')).toBe(true)
    expect(matches(list[0], 'zzz')).toBe(false)
    expect(matches(list[0], '   ')).toBe(true)
  })

  it('hides cleared quests on request', () => {
    expect(visibleRows(list, { text: '', hideCleared: true }).map((r) => r.quest.number)).toEqual([1, 12])
    expect(visibleRows(list, { text: '', hideCleared: false }).map((r) => r.quest.number)).toEqual([1, 2, 12])
  })

  it('searches flat and browses nested', () => {
    const nested = visibleRows(list, { text: '', hideCleared: false })
    expect(nested.find((r) => r.quest.number === 2)?.depth).toBe(1)
    const flat = visibleRows(list, { text: 'sign', hideCleared: false })
    expect(flat.map((r) => [r.quest.number, r.depth])).toEqual([[2, 0]])
  })
})

describe('helpers', () => {
  it('counts by state', () => {
    expect(counts([q(1), q(2, { state: 'locked' }), q(3, { state: 'locked' }), q(4, { state: 'cleared' })])).toEqual({
      open: 1, locked: 2, claimed: 0, cleared: 1,
    })
  })

  it('formats relative times', () => {
    const now = new Date('2026-10-08T12:00:00Z')
    expect(timeAgo('2026-10-08T11:59:30Z', now)).toBe('just now')
    expect(timeAgo('2026-10-08T11:55:00Z', now)).toBe('5 min ago')
    expect(timeAgo('2026-10-08T09:00:00Z', now)).toBe('3 h ago')
    expect(timeAgo('2026-10-05T12:00:00Z', now)).toBe('3 d ago')
    expect(timeAgo('2026-10-09T12:00:00Z', now)).toBe('just now') // future clamps
    expect(timeAgo('garbage', now)).toBe('')
    expect(timeAgo('', now)).toBe('')
  })

  it('picks readable label text colours', () => {
    expect(labelText('ffffff')).toBe('#1d1d1b')
    expect(labelText('000000')).toBe('#ffffff')
    expect(labelText('d73a4a')).toBe('#ffffff')
    expect(labelText('fbca04')).toBe('#1d1d1b')
    expect(labelText(undefined)).toBe('inherit')
    expect(labelText('nothex')).toBe('inherit')
  })
})
