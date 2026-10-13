import { expect, it } from 'vitest'
import { NEW_LINE, keyInput } from './termKeys'

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
