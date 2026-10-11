import type { SessionInfo } from './api'

export type Permission = NotificationPermission | 'unsupported'

export function permission(): Permission {
  return typeof Notification === 'undefined' ? 'unsupported' : Notification.permission
}

/** Asks the browser for permission; call it only when the user turns notifications on. */
export async function requestPermission(): Promise<Permission> {
  if (typeof Notification === 'undefined') return 'unsupported'
  try {
    return await Notification.requestPermission()
  } catch {
    return Notification.permission
  }
}

/**
 * Which sessions to announce now. A session is announced once per wait, only
 * when the window is not in front, and not if the user already looked at that
 * wait. Waits that ended are forgotten so the next question announces again.
 */
export function pickToNotify(
  sessions: SessionInfo[],
  notified: ReadonlySet<string>,
  seen: ReadonlySet<string>,
  viewing: boolean,
): { fire: SessionInfo[]; notified: Set<string> } {
  const waiting = sessions.filter((s) => !s.exited && !s.lost && s.activity === 'needs-you')
  const ids = new Set(waiting.map((s) => s.id))
  const next = new Set([...notified].filter((id) => ids.has(id)))
  if (viewing) return { fire: [], notified: next }
  const fire = waiting.filter((s) => !next.has(s.id) && !seen.has(s.id))
  for (const s of fire) next.add(s.id)
  return { fire, notified: next }
}

/** Plain wording, and nothing taken from the issue's text. */
export function notificationText(s: SessionInfo, campaignName: string): { title: string; body: string } {
  const what = s.quest ? `Issue #${s.quest}` : 'A session'
  return { title: 'A session is waiting for you', body: `${what} in ${campaignName} is waiting for your input.` }
}
