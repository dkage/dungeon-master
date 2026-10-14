import { describe, expect, it } from 'vitest'
import { judge, testMessage } from './shortcutTest'

const key = (over: Partial<KeyboardEvent>) => ({ key: 'a', code: 'KeyA', ctrlKey: false, altKey: false, shiftKey: false, metaKey: false, ...over })

describe('judge', () => {
  it('says received when the keys match the binding', () => {
    expect(judge('Ctrl+Shift+2', key({ key: '@', code: 'Digit2', ctrlKey: true, shiftKey: true }), 'other')).toEqual({ kind: 'received' })
  })

  it('names what arrived when it differs', () => {
    expect(judge('Ctrl+Shift+2', key({ key: '3', code: 'Digit3', ctrlKey: true, shiftKey: true }), 'other')).toEqual({ kind: 'different', got: 'Ctrl+Shift+3' })
    expect(judge('Meta+,', key({ key: '.', code: 'Period', metaKey: true }), 'mac')).toEqual({ kind: 'different', got: 'Cmd+.' })
  })

  it('cancels on a bare Escape, but Ctrl+Escape is a key like another', () => {
    expect(judge('Ctrl+Shift+2', key({ key: 'Escape', code: 'Escape' }), 'other')).toEqual({ kind: 'cancelled' })
    expect(judge('Ctrl+Escape', key({ key: 'Escape', code: 'Escape', ctrlKey: true }), 'other')).toEqual({ kind: 'received' })
  })

  it('waits through a bare modifier press', () => {
    expect(judge('Ctrl+Shift+2', key({ key: 'Control', code: 'ControlLeft', ctrlKey: true }), 'other')).toBeNull()
  })
})

describe('testMessage', () => {
  it('never claims a key is safe and says plainly when nothing arrived', () => {
    for (const s of [{ kind: 'received' }, { kind: 'different', got: 'X' }, { kind: 'not-received' }, { kind: 'cancelled' }, { kind: 'waiting' }] as const) {
      expect(testMessage(s)).not.toMatch(/\bsafe\b(?! from)/i)
    }
    expect(testMessage({ kind: 'not-received' })).toContain('Nothing arrived')
    expect(testMessage({ kind: 'received' })).toContain('could still take them')
    expect(testMessage({ kind: 'waiting' })).toContain('Nothing will run')
  })
})
