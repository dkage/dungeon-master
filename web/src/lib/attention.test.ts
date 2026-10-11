import { describe, expect, it } from 'vitest'
import type { SessionInfo } from './api'
import { attentionTitle, faviconHref, sessionStatus, stillWaiting, unattended } from './attention'

const s = (id: string, over: Partial<SessionInfo> = {}): SessionInfo => ({
  id, kind: 'hero', title: id, campaign: 'c', dir: '/', created: '', exited: false, exitCode: 0, ...over,
})

describe('sessionStatus', () => {
  it('names what a live session is doing, with a visible mark when it waits', () => {
    expect(sessionStatus(s('a', { activity: 'working' })).label).toBe('working')
    expect(sessionStatus(s('a', { activity: 'idle' })).label).toBe('idle')
    const waiting = sessionStatus(s('a', { activity: 'needs-you' }))
    expect(waiting).toMatchObject({ kind: 'needs', glyph: '?' })
    expect(waiting.label).toBe('waiting for you')
  })
  it('uses plain wording in plain mode', () => {
    expect(sessionStatus(s('a', { activity: 'needs-you' }), 'plain').label).toBe('waiting for your input')
  })
  it('treats a live session without an activity as working', () => {
    expect(sessionStatus(s('a')).kind).toBe('running')
  })
  it('lets process state win over activity', () => {
    expect(sessionStatus(s('a', { exited: true, exitCode: 0, activity: 'needs-you' })).kind).toBe('done')
    expect(sessionStatus(s('a', { exited: true, exitCode: 2 })).label).toBe('exited with code 2')
    expect(sessionStatus(s('a', { lost: true })).kind).toBe('lost')
  })
})

describe('unattended', () => {
  const list = [s('a', { activity: 'needs-you' }), s('b', { activity: 'working' }), s('c', { activity: 'needs-you', exited: true })]
  it('counts only live sessions that wait and have not been seen', () => {
    expect(unattended(list, new Set()).map((x) => x.id)).toEqual(['a'])
    expect(unattended(list, new Set(['a']))).toEqual([])
  })
})

describe('stillWaiting', () => {
  it('forgets sessions whose wait ended so a new question counts again', () => {
    const seen = new Set(['a', 'b', 'gone'])
    const next = stillWaiting([s('a', { activity: 'needs-you' }), s('b', { activity: 'working' })], seen)
    expect([...next]).toEqual(['a'])
  })
})

describe('title and favicon', () => {
  it('puts a count in front of the title only when something waits', () => {
    expect(attentionTitle('Dungeon Master', 0)).toBe('Dungeon Master')
    expect(attentionTitle('Dungeon Master', 2)).toBe('(2) Dungeon Master')
  })
  it('draws a badge only when something waits', () => {
    const plain = decodeURIComponent(faviconHref(0))
    const badged = decodeURIComponent(faviconHref(1))
    expect(plain).not.toContain('<circle')
    expect(badged).toContain('<circle')
    expect(faviconHref(1).startsWith('data:image/svg+xml,')).toBe(true)
  })
})
