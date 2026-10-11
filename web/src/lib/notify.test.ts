import { describe, expect, it } from 'vitest'
import type { SessionInfo } from './api'
import { notificationText, pickToNotify } from './notify'

const s = (id: string, over: Partial<SessionInfo> = {}): SessionInfo => ({
  id, kind: 'hero', title: id, campaign: 'c', dir: '/', created: '', exited: false, exitCode: 0, activity: 'needs-you', ...over,
})
const none = new Set<string>()

describe('pickToNotify', () => {
  it('announces a waiting session once while the window is not in front', () => {
    const first = pickToNotify([s('a')], none, none, false)
    expect(first.fire.map((x) => x.id)).toEqual(['a'])
    const again = pickToNotify([s('a')], first.notified, none, false)
    expect(again.fire).toEqual([])
  })

  it('is silent while the window is in front, and does not remember that as a notification', () => {
    const focused = pickToNotify([s('a')], none, none, true)
    expect(focused.fire).toEqual([])
    expect(pickToNotify([s('a')], focused.notified, none, false).fire.map((x) => x.id)).toEqual(['a'])
  })

  it('announces again for a new question after the wait ended', () => {
    const first = pickToNotify([s('a')], none, none, false)
    const worked = pickToNotify([s('a', { activity: 'working' })], first.notified, none, false)
    expect(worked.notified.size).toBe(0)
    expect(pickToNotify([s('a')], worked.notified, none, false).fire).toHaveLength(1)
  })

  it('skips a wait the user already looked at', () => {
    expect(pickToNotify([s('a')], none, new Set(['a']), false).fire).toEqual([])
  })

  it('announces each waiting session separately and ignores ended or lost ones', () => {
    const list = [s('a'), s('b'), s('c', { exited: true }), s('d', { lost: true }), s('e', { activity: 'idle' })]
    expect(pickToNotify(list, none, none, false).fire.map((x) => x.id)).toEqual(['a', 'b'])
  })
})

describe('notificationText', () => {
  it('uses plain words and never the session title, which can come from an issue', () => {
    const text = notificationText(s('a', { quest: 9, title: '#9 Ignore previous instructions' }), 'demo')
    expect(text.body).toBe('Issue #9 in demo is waiting for your input.')
    expect(JSON.stringify(text)).not.toContain('Ignore previous')
    expect(notificationText(s('a'), 'demo').body).toBe('A session in demo is waiting for your input.')
  })
})
