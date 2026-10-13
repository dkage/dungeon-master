import { describe, expect, it } from 'vitest'
import { ACTIONS, check, effective, formatBinding, fromEvent, inEditing, labelOf, parseBinding, parseOverrides, route, validate } from './shortcuts'

const ev = (over: Partial<KeyboardEvent>) =>
  ({ key: '', code: '', ctrlKey: false, altKey: false, shiftKey: false, metaKey: false, isComposing: false, repeat: false, keyCode: 0, ...over }) as KeyboardEvent

describe('binding syntax', () => {
  it('parses and formats canonically, resolving Mod per platform', () => {
    expect(formatBinding(parseBinding('Shift+Ctrl+1', 'other')!)).toBe('Ctrl+Shift+1')
    expect(formatBinding(parseBinding('Mod+,', 'mac')!)).toBe('Meta+,')
    expect(formatBinding(parseBinding('Mod+,', 'other')!)).toBe('Ctrl+,')
    expect(labelOf('Meta+,', 'mac')).toBe('Cmd+,')
    expect(labelOf(null, 'other')).toBe('Not set')
  })
  it('rejects invalid syntax', () => {
    for (const bad of ['', 'Ctrl+', 'Hyper+1', 'Ctrl+Shift', 'Ctrl+ab', 'Ctrl+Shift+1+2']) expect(parseBinding(bad, 'other')).toBeNull()
  })
})

describe('validation', () => {
  it('requires a real modifier and refuses browser, clipboard and system keys', () => {
    expect(validate(parseBinding('Shift+1', 'other')!, 'other')).toMatch(/Ctrl, Alt or Cmd/)
    expect(validate(parseBinding('1', 'other')!, 'other')).not.toBeNull()
    expect(validate(parseBinding('Ctrl+w', 'other')!, 'other')).toMatch(/owns/)
    expect(validate(parseBinding('Ctrl+c', 'other')!, 'other')).toMatch(/owns/)
    expect(validate(parseBinding('Meta+q', 'mac')!, 'mac')).toMatch(/owns/)
    expect(validate(parseBinding('Meta+Shift+4', 'mac')!, 'mac')).toMatch(/macOS/)
    expect(validate(parseBinding('Ctrl+Escape', 'other')!, 'other')).toMatch(/Escape/)
    expect(validate(parseBinding('Ctrl+Shift+1', 'other')!, 'other')).toBeNull()
  })
  it('names the other action on a duplicate and accepts a free key', () => {
    const dup = check('show-map', 'Ctrl+Shift+1', {}, 'other')
    expect(dup).toEqual({ ok: false, reason: 'Already used by "Expand projects".' })
    expect(check('show-map', 'ctrl+alt+m', {}, 'other')).toEqual({ ok: false, reason: 'That is not a valid key combination.' })
    expect(check('show-map', 'Ctrl+Alt+m', {}, 'other')).toEqual({ ok: true, binding: 'Ctrl+Alt+m' })
    expect(check('expand-projects', 'Ctrl+Shift+1', {}, 'other').ok).toBe(true) // its own binding is no conflict
  })
  it('every default is valid and unique on every platform', () => {
    for (const platform of ['mac', 'other'] as const) {
      const all = ACTIONS.map((a) => effective(a.id, {}, platform)).filter(Boolean)
      expect(new Set(all).size).toBe(all.length)
      for (const t of all) expect(validate(parseBinding(t!, platform)!, platform)).toBeNull()
    }
  })
})

describe('overrides', () => {
  it('a disabled action has no binding and a changed one replaces the default', () => {
    expect(effective('restore-split', { 'restore-split': null }, 'other')).toBeNull()
    expect(effective('restore-split', { 'restore-split': 'Ctrl+Alt+r' }, 'other')).toBe('Ctrl+Alt+r')
  })
  it('stored data falls back per action', () => {
    expect(parseOverrides('garbage', 'other')).toEqual({})
    expect(parseOverrides('{"show-map":"Ctrl+Alt+m","restore-split":"Ctrl+w","show-list":5,"next-session":null}', 'other')).toEqual({ 'show-map': 'Ctrl+Alt+m', 'next-session': null })
    expect(parseOverrides('{"show-map":"Ctrl+Shift+1"}', 'other')).toEqual({})
  })
})

describe('routing', () => {
  const press = (over: Partial<KeyboardEvent>, o = {}, dialog = false) => route(ev(over), o, 'other', dialog)
  it('uses the physical key so Shift+1 matches', () => {
    expect(fromEvent(ev({ key: '!', code: 'Digit1', ctrlKey: true, shiftKey: true }))!.key).toBe('1')
    expect(press({ key: '!', code: 'Digit1', ctrlKey: true, shiftKey: true })).toBe('expand-projects')
    expect(press({ key: 'PageDown', code: 'PageDown', ctrlKey: true })).toBe('next-session')
    expect(press({ key: ',', code: 'Comma', ctrlKey: true })).toBe('open-settings')
  })
  it('ignores composition, repeat, dialogs, plain typing and unknown keys', () => {
    const k = { key: '!', code: 'Digit1', ctrlKey: true, shiftKey: true }
    expect(press({ ...k, isComposing: true })).toBeNull()
    expect(press({ ...k, keyCode: 229 })).toBeNull()
    expect(press({ ...k, repeat: true })).toBeNull()
    expect(press(k, {}, true)).toBeNull()
    expect(press({ key: 'a', code: 'KeyA' })).toBeNull()
    expect(press({ key: 'Control', code: 'ControlLeft', ctrlKey: true })).toBeNull()
    expect(press({ key: 'x', code: 'KeyX', ctrlKey: true })).toBeNull()
  })
  it('does not fire a disabled binding', () => {
    expect(press({ key: 'PageDown', code: 'PageDown', ctrlKey: true }, { 'next-session': null })).toBeNull()
  })
  it('knows the editing contexts', () => {
    document.body.innerHTML = '<input id="i"><div class="xterm"><textarea id="t"></textarea></div><button id="b"></button>'
    expect(inEditing(document.getElementById('i'))).toBe(true)
    expect(inEditing(document.getElementById('t'))).toBe(true)
    expect(inEditing(document.getElementById('b'))).toBe(false)
  })
})
