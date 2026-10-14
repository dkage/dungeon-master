import type { SessionInfo } from './api'

/** The tabs above the terminal: the selected project's sessions that still have a terminal, in launch order. */
export function sessionTabs(sessions: SessionInfo[], campaignId: string | null): SessionInfo[] {
  if (!campaignId) return []
  return sessions.filter((s) => s.campaign === campaignId && !s.lost)
}

/** The label of a tab: the issue and its title, or the hero (or "Scratch terminal") for a free session. */
export function tabLabel(s: SessionInfo, heroLabel = ''): string {
  if (s.kind === 'shell' && !s.quest) return 'Scratch terminal'
  if (s.quest) return heroLabel ? `${s.title} (${heroLabel})` : s.title
  return s.title || heroLabel || 'Session'
}

/** The tab to move to for an arrow, Home or End key; wraps around at the ends. */
export function tabTarget(key: string, index: number, count: number): number | null {
  if (count === 0) return null
  switch (key) {
    case 'ArrowRight': return (index + 1) % count
    case 'ArrowLeft': return (index - 1 + count) % count
    case 'Home': return 0
    case 'End': return count - 1
    default: return null
  }
}
