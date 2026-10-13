import { describe, expect, it } from 'vitest'
import { NEW_LINE, browserPastes, keyInput } from './termKeys'

const key = (over: Partial<Parameters<typeof keyInput>[0]>) => ({
  type: 'keydown', key: 'Enter', shiftKey: false, ctrlKey: false, altKey: false, metaKey: false, isComposing: false,
  ...over,
})

it('turns Shift+Enter into the new-line sequence', () => {
  expect(keyInput(key({ shiftKey: true }))).toBe(NEW_LINE)
  expect(NEW_LINE).toBe('\x1b\r')
})

it('leaves plain Enter to the terminal, which sends a carriage return', () => {
  expect(keyInput(key({}))).toBeNull()
})

it('ignores other chords, key releases, other keys and IME composition', () => {
  expect(keyInput(key({ shiftKey: true, ctrlKey: true }))).toBeNull()
  expect(keyInput(key({ shiftKey: true, altKey: true }))).toBeNull()
  expect(keyInput(key({ shiftKey: true, metaKey: true }))).toBeNull()
  expect(keyInput(key({ shiftKey: true, type: 'keyup' }))).toBeNull()
  expect(keyInput(key({ shiftKey: true, key: 'a' }))).toBeNull()
  expect(keyInput(key({ shiftKey: true, isComposing: true }))).toBeNull()
})

describe('browserPastes', () => {
  const v = (over: Record<string, unknown> = {}) => ({ ...key({ key: 'v', ctrlKey: true }), code: 'KeyV', ...over }) as Parameters<typeof browserPastes>[0]
  it('lets the browser paste on Ctrl+V and Ctrl+Shift+V off macOS', () => {
    expect(browserPastes(v(), false)).toBe(true)
    expect(browserPastes(v({ shiftKey: true }), false)).toBe(true)
  })
  it('leaves macOS alone, so a bare Ctrl+V still reaches the agent', () => {
    expect(browserPastes(v(), true)).toBe(false)
  })
  it('ignores other chords, other keys, releases and composition', () => {
    expect(browserPastes(v({ ctrlKey: false }), false)).toBe(false)
    expect(browserPastes(v({ altKey: true }), false)).toBe(false)
    expect(browserPastes(v({ metaKey: true }), false)).toBe(false)
    expect(browserPastes(v({ code: 'KeyC' }), false)).toBe(false)
    expect(browserPastes(v({ type: 'keyup' }), false)).toBe(false)
    expect(browserPastes(v({ isComposing: true }), false)).toBe(false)
  })
})
