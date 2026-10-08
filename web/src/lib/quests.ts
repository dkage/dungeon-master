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
  const pool = f.hideCleared ? quests.filter((q) => !q.closed) : quests
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
