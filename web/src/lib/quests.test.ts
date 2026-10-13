import { describe, expect, it } from 'vitest'
import type { Quest } from './api'
import { contrastRatio, labelStyle, labelText, labelTone, matches, questRows, scopeText, timeAgo, typeChips, visibleRows } from './quests'

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

  it('searches flat and browses nested', () => {
    const nested = visibleRows(list, { text: '' })
    expect(nested.find((r) => r.quest.number === 2)?.depth).toBe(1)
    const flat = visibleRows(list, { text: 'sign' })
    expect(flat.map((r) => [r.quest.number, r.depth])).toEqual([[2, 0]])
  })
})

describe('helpers', () => {
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

describe('label tones', () => {
  it('maps GitHub colours to palette tones by hue', () => {
    expect(labelTone('d73a4a').name).toBe('red')
    expect(labelTone('fbca04').name).toBe('amber')
    expect(labelTone('0e8a16').name).toBe('green')
    expect(labelTone('0075ca').name).toBe('blue')
    expect(labelTone('5319e7').name).toBe('violet')
    expect(labelTone('#e99695').name).toBe('red')
  })

  it('falls back to stone for greys and unknown colours', () => {
    for (const c of [undefined, '', 'nothex', 'ffffff', '000000', 'cccccc', '12345']) expect(labelTone(c).name).toBe('stone')
  })

  it('keeps every tone readable at 4.5:1', () => {
    const seen = new Set<string>()
    for (let i = 0; i < 360; i += 5) {
      const c = Math.round(255 * (1 - Math.abs(((i / 60) % 2) - 1)))
      const rgb = [[255, c, 0], [c, 255, 0], [0, 255, c], [0, c, 255], [c, 0, 255], [255, 0, c]][Math.floor(i / 60) % 6]
      const t = labelTone(rgb.map((v) => v.toString(16).padStart(2, '0')).join(''))
      seen.add(t.name)
      expect(contrastRatio(t.bg, t.fg)).toBeGreaterThanOrEqual(4.5)
    }
    expect(seen.size).toBeGreaterThanOrEqual(6)
  })

  it('exposes the tone and the GitHub swatch as custom properties', () => {
    const style = labelStyle('d73a4a')
    expect(style).toContain('--gh:#d73a4a')
    expect(style).toContain('--tone-bg:')
    expect(labelStyle(undefined)).toContain('--gh:#999999')
  })
})

describe('label filters and scope text', () => {
  const withLabels = (n: number, ...names: string[]) => q(n, { labels: names.map((name) => ({ name })) })

  it('filters by any of the chosen labels, case-insensitively', () => {
    const list = [withLabels(1, 'Bug'), withLabels(2, 'feature'), withLabels(3), withLabels(4, 'bug', 'feature')]
    const f = (labels: string[]) => visibleRows(list, { text: '', labels }).map((r) => r.quest.number)
    expect(f([])).toEqual([1, 2, 3, 4])
    expect(f(['bug'])).toEqual([1, 4])
    expect(f(['BUG', 'feature'])).toEqual([1, 2, 4])
    expect(f(['nope'])).toEqual([])
    // Combines with the text filter.
    expect(visibleRows(list, { text: '#4', labels: ['bug'] }).map((r) => r.quest.number)).toEqual([4])
  })

  it('lists the most common labels first, by name on ties, up to a limit', () => {
    const list = [withLabels(1, 'b', 'a'), withLabels(2, 'b'), withLabels(3, 'c', 'A')]
    expect(typeChips(list).map((c) => `${c.name}:${c.count}`)).toEqual(['a:2', 'b:2', 'c:1'])
    const many = Array.from({ length: 12 }, (_, i) => withLabels(i + 1, `l${String(i).padStart(2, '0')}`))
    expect(typeChips(many)).toHaveLength(8)
    expect(typeChips(many, 3)).toHaveLength(3)
    expect(typeChips([])).toEqual([])
  })

  it('keeps a label colour for chips', () => {
    expect(typeChips([q(1, { labels: [{ name: 'bug', color: 'd73a4a' }] })])[0].color).toBe('d73a4a')
  })

  it('names a scope in words', () => {
    expect(scopeText(null)).toBe('')
    expect(scopeText(undefined)).toBe('')
    expect(scopeText({ mode: 'assigned', labels: [] })).toBe('assigned to me')
    expect(scopeText({ mode: 'all', labels: [] })).toBe('everything')
    expect(scopeText({ mode: 'labels', labels: ['tracked'] })).toBe('tracked')
    expect(scopeText({ mode: 'labels', labels: ['a', 'b'] })).toBe('a + b')
  })
})
