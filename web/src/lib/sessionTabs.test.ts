import { describe, expect, it } from 'vitest'
import type { SessionInfo } from './api'
import { sessionTabs, tabLabel, tabTarget } from './sessionTabs'
import { needsStopConfirm } from './attention'

const sess = (id: string, campaign: string, over: Partial<SessionInfo> = {}): SessionInfo => ({
  id, kind: 'hero', title: `T-${id}`, campaign, dir: '/d', created: '', exited: false, exitCode: 0, ...over,
})

describe('sessionTabs', () => {
  it('lists the selected project\'s sessions in launch order, without lost ones and other projects\'', () => {
    const all = [sess('a', 'c1'), sess('x', 'c2'), sess('b', 'c1', { exited: true }), sess('l', 'c1', { lost: true }), sess('c', 'c1')]
    expect(sessionTabs(all, 'c1').map((s) => s.id)).toEqual(['a', 'b', 'c'])
    expect(sessionTabs(all, 'c2').map((s) => s.id)).toEqual(['x'])
    expect(sessionTabs(all, null)).toEqual([])
  })

  it('labels issue sessions by issue, shells by name and free sessions by title or hero', () => {
    expect(tabLabel(sess('a', 'c1', { quest: 5, title: '#5 Fix it' }), 'Opus')).toBe('#5 Fix it (Opus)')
    expect(tabLabel(sess('a', 'c1', { quest: 5, title: '#5 Fix it' }))).toBe('#5 Fix it')
    expect(tabLabel(sess('s', 'c1', { kind: 'shell', title: 'Shell' }))).toBe('Scratch terminal')
    expect(tabLabel(sess('f', 'c1', { title: 'Opus' }), 'Opus')).toBe('Opus')
    expect(tabLabel(sess('f', 'c1', { title: '' }), '')).toBe('Session')
  })

  it('moves between tabs with the arrow keys, wrapping, and with Home and End', () => {
    expect(tabTarget('ArrowRight', 2, 3)).toBe(0)
    expect(tabTarget('ArrowLeft', 0, 3)).toBe(2)
    expect(tabTarget('Home', 2, 3)).toBe(0)
    expect(tabTarget('End', 0, 3)).toBe(2)
    expect(tabTarget('a', 0, 3)).toBeNull()
    expect(tabTarget('ArrowRight', 0, 0)).toBeNull()
  })

  it('asks before stopping only a running session that is working or waiting', () => {
    expect(needsStopConfirm(sess('a', 'c1', { activity: 'working' }))).toBe(true)
    expect(needsStopConfirm(sess('a', 'c1', { activity: 'needs-you' }))).toBe(true)
    expect(needsStopConfirm(sess('a', 'c1', { activity: 'idle' }))).toBe(false)
    expect(needsStopConfirm(sess('a', 'c1', { activity: 'working', exited: true }))).toBe(false)
    expect(needsStopConfirm(sess('a', 'c1', { activity: 'working', lost: true }))).toBe(false)
  })
})
