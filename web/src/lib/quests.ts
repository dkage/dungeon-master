import type { Quest, QuestState } from './api'

export interface Row {
  quest: Quest
  depth: number
}

/**
 * Orders quests as a forest: a quest whose parent is in the list is shown
 * under it, indented; everything else is a root. The server's order is kept
 * among siblings. Cycles (which GitHub forbids) cannot loop forever.
 */
export function questRows(quests: Quest[]): Row[] {
  const byNumber = new Map(quests.map((q) => [q.number, q]))
  const children = new Map<number, Quest[]>()
  const roots: Quest[] = []
  for (const q of quests) {
    if (q.parent && byNumber.has(q.parent) && q.parent !== q.number) {
      const list = children.get(q.parent) ?? []
      list.push(q)
      children.set(q.parent, list)
    } else {
      roots.push(q)
    }
  }
  const out: Row[] = []
  const seen = new Set<number>()
  const visit = (q: Quest, depth: number) => {
    if (seen.has(q.number)) return
    seen.add(q.number)
    out.push({ quest: q, depth })
    for (const c of children.get(q.number) ?? []) visit(c, depth + 1)
  }
  for (const r of roots) visit(r, 0)
  // Anything left is part of a cycle; show it rather than lose it.
  for (const q of quests) visit(q, 0)
  return out
}

export interface Filter {
  text: string
  hideCleared: boolean
  /** Show only quests carrying at least one of these labels (empty: all). */
  labels?: string[]
}

export function matches(q: Quest, text: string): boolean {
  const t = text.trim().toLowerCase().replace(/^#/, '')
  if (!t) return true
  return (
    String(q.number) === t ||
    q.title.toLowerCase().includes(t) ||
    q.labels.some((l) => l.name.toLowerCase().includes(t)) ||
    q.assignees.some((a) => a.toLowerCase().includes(t))
  )
}

/**
 * Applies the filter. While searching, the list is flat (a match under a
 * non-matching parent would otherwise lose its context in a confusing way);
 * with no search it is the nested forest.
 */
export function visibleRows(quests: Quest[], f: Filter): Row[] {
  let pool = f.hideCleared ? quests.filter((q) => !q.closed) : quests
  if (f.labels && f.labels.length > 0) {
    const want = new Set(f.labels.map((l) => l.toLowerCase()))
    pool = pool.filter((q) => q.labels.some((l) => want.has(l.name.toLowerCase())))
  }
  if (f.text.trim()) {
    return pool.filter((q) => matches(q, f.text)).map((quest) => ({ quest, depth: 0 }))
  }
  return questRows(pool)
}

export const STATE_LABEL: Record<QuestState, string> = {
  open: 'Open',
  locked: 'Locked',
  claimed: 'In progress',
  cleared: 'Cleared',
}

export function counts(quests: Quest[]): Record<QuestState, number> {
  const c: Record<QuestState, number> = { open: 0, locked: 0, claimed: 0, cleared: 0 }
  for (const q of quests) c[q.state]++
  return c
}

/** "just now", "5 min ago", "3 h ago", "2 d ago". */
export function timeAgo(iso: string, now: Date = new Date()): string {
  const t = Date.parse(iso)
  if (Number.isNaN(t)) return ''
  const s = Math.max(0, Math.round((now.getTime() - t) / 1000))
  if (s < 45) return 'just now'
  const m = Math.round(s / 60)
  if (m < 60) return `${m} min ago`
  const h = Math.round(m / 60)
  if (h < 24) return `${h} h ago`
  return `${Math.round(h / 24)} d ago`
}

/** Text that stays readable on a label's background colour. */
export function labelText(hex: string | undefined): string {
  const m = /^#?([0-9a-f]{6})$/i.exec(hex ?? '')
  if (!m) return 'inherit'
  const n = parseInt(m[1], 16)
  const lum = (0.299 * (n >> 16) + 0.587 * ((n >> 8) & 255) + 0.114 * (n & 255)) / 255
  return lum > 0.6 ? '#1d1d1b' : '#ffffff'
}

/** A themed label tone: a dark background and light text drawn from the Dungeon palette. */
export interface LabelTone {
  name: string
  bg: string
  fg: string
}

const TONES: Record<string, LabelTone> = {
  stone: { name: 'stone', bg: '#3a2f2a', fg: '#e6d9c3' },
  red: { name: 'red', bg: '#5a1a1c', fg: '#ffd9d2' },
  amber: { name: 'amber', bg: '#5a3a12', fg: '#ffe6b3' },
  green: { name: 'green', bg: '#1f4a2c', fg: '#d3f2d9' },
  teal: { name: 'teal', bg: '#12474a', fg: '#c9f0ee' },
  blue: { name: 'blue', bg: '#1d3b5e', fg: '#d6e6ff' },
  violet: { name: 'violet', bg: '#43276a', fg: '#e8d9ff' },
  rose: { name: 'rose', bg: '#5a2147', fg: '#ffd9f0' },
}

/** WCAG contrast ratio between two #rrggbb colours. */
export function contrastRatio(a: string, b: string): number {
  const lum = (hex: string) => {
    const n = parseInt(hex.replace('#', ''), 16)
    const c = [n >> 16, (n >> 8) & 255, n & 255].map((v) => {
      const x = v / 255
      return x <= 0.03928 ? x / 12.92 : ((x + 0.055) / 1.055) ** 2.4
    })
    return 0.2126 * c[0] + 0.7152 * c[1] + 0.0722 * c[2]
  }
  const [hi, lo] = [lum(a), lum(b)].sort((x, y) => y - x)
  return (hi + 0.05) / (lo + 0.05)
}

/**
 * The palette tone closest to a GitHub label colour, so labels follow the theme
 * but stay distinguishable. Unknown or greyish colours get the neutral stone tone.
 */
export function labelTone(hex: string | undefined): LabelTone {
  const m = /^#?([0-9a-f]{6})$/i.exec(hex ?? '')
  if (!m) return TONES.stone
  const n = parseInt(m[1], 16)
  const [r, g, b] = [(n >> 16) / 255, ((n >> 8) & 255) / 255, (n & 255) / 255]
  const max = Math.max(r, g, b)
  const min = Math.min(r, g, b)
  const d = max - min
  if (d < 0.12) return TONES.stone
  let h = max === r ? ((g - b) / d) % 6 : max === g ? (b - r) / d + 2 : (r - g) / d + 4
  h = (h * 60 + 360) % 360
  if (h < 15 || h >= 345) return TONES.red
  if (h < 70) return TONES.amber
  if (h < 160) return TONES.green
  if (h < 200) return TONES.teal
  if (h < 255) return TONES.blue
  if (h < 305) return TONES.violet
  return TONES.rose
}

/** Inline custom properties for a label: its themed tone and its GitHub swatch. */
export function labelStyle(hex: string | undefined): string {
  const t = labelTone(hex)
  const raw = /^#?([0-9a-f]{6})$/i.exec(hex ?? '')?.[1] ?? '999999'
  return `--gh:#${raw};--gh-text:${labelText(hex)};--tone-bg:${t.bg};--tone-fg:${t.fg}`
}

export interface TypeChip {
  name: string
  color?: string
  count: number
}

/** The labels worth offering as filters: the most common ones in the list. */
export function typeChips(quests: Quest[], max = 8): TypeChip[] {
  const by = new Map<string, TypeChip>()
  for (const q of quests) {
    for (const l of q.labels) {
      const key = l.name.toLowerCase()
      const c = by.get(key)
      if (c) c.count++
      else by.set(key, { name: l.name, color: l.color, count: 1 })
    }
  }
  return [...by.values()]
    .sort((a, b) => b.count - a.count || a.name.localeCompare(b.name))
    .slice(0, max)
}

/** "tracked", "assigned to me" or "everything": what the scope shows, in words. */
export function scopeText(scope: { mode: string; labels: string[] } | null | undefined): string {
  if (!scope) return ''
  if (scope.mode === 'assigned') return 'assigned to me'
  if (scope.mode === 'all') return 'everything'
  return scope.labels.join(' + ')
}
