import type { SessionInfo } from './api'

export type WordMode = 'hints' | 'themed' | 'plain'

export interface SessionStatus {
  label: string
  kind: string
  /** A visible mark, so colour is never the only signal. */
  glyph: string
}

/** What to show for a session: its process state first, then what it is doing. */
export function sessionStatus(s: SessionInfo, mode: WordMode = 'themed'): SessionStatus {
  if (s.lost) return { label: 'lost (app restarted)', kind: 'lost', glyph: '' }
  if (s.exited) {
    return s.exitCode === 0
      ? { label: 'exited', kind: 'done', glyph: '' }
      : { label: `exited with code ${s.exitCode}`, kind: 'failed', glyph: '' }
  }
  switch (s.activity) {
    case 'needs-you':
      return { label: mode === 'plain' ? 'waiting for your input' : 'waiting for you', kind: 'needs', glyph: '?' }
    case 'idle':
      return { label: 'idle', kind: 'idle', glyph: '' }
    default:
      return { label: 'working', kind: 'running', glyph: '' }
  }
}

/** Sessions waiting for the user that the user has not looked at yet. */
export function unattended(sessions: SessionInfo[], seen: ReadonlySet<string>): SessionInfo[] {
  return sessions.filter((s) => !s.exited && !s.lost && s.activity === 'needs-you' && !seen.has(s.id))
}

/** Ids still worth remembering as seen: those whose wait has not ended. */
export function stillWaiting(sessions: SessionInfo[], seen: ReadonlySet<string>): Set<string> {
  const waiting = new Set(sessions.filter((s) => !s.exited && s.activity === 'needs-you').map((s) => s.id))
  return new Set([...seen].filter((id) => waiting.has(id)))
}

export function attentionTitle(base: string, count: number): string {
  return count > 0 ? `(${count}) ${base}` : base
}

/** The page icon as a data URL; a badge dot appears when something needs the user. */
export function faviconHref(count: number): string {
  const badge = count > 0 ? '<circle cx="23" cy="9" r="8" fill="#e5484d" stroke="#fff" stroke-width="2"/>' : ''
  const svg =
    '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32">' +
    '<rect x="2" y="2" width="28" height="28" rx="7" fill="#2b2140"/>' +
    '<text x="16" y="23" font-size="20" text-anchor="middle" fill="#f1c76b" font-family="serif">✦</text>' +
    badge +
    '</svg>'
  return `data:image/svg+xml,${encodeURIComponent(svg)}`
}
