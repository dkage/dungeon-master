import { expect, it } from 'vitest'
import { FONT_STACK, SYMBOLS_FONT, FRAME_PADDING, LINE_HEIGHT, terminalTypography } from './termOptions'

it('sets the adopted typography options', () => {
  expect(terminalTypography()).toEqual({
    fontFamily: FONT_STACK,
    lineHeight: 1.3,
    fontWeight: 'normal',
    fontWeightBold: 'bold',
  })
  expect(LINE_HEIGHT).toBeGreaterThan(1)
})

it('puts bundled Fira Code ahead of system monospace and never uses a serif', () => {
  const names = FONT_STACK.split(', ')
  expect(names.indexOf('"Fira Code"')).toBeGreaterThan(names.indexOf('"Symbols Nerd Font Mono"'))
  expect(names.indexOf('"Fira Code"')).toBeLessThan(names.indexOf('ui-monospace'))
  expect(names.at(-1)).toBe('monospace')
  expect(FONT_STACK).not.toMatch(/Almendra|Garamond|Alegreya|Georgia|serif/i)
})

it('uses the reference padding in Dungeon only', () => {
  expect(FRAME_PADDING.dungeon).toBe('22px 28px')
  expect(FRAME_PADDING.neutral).toBe('8px')
})

it('puts the bundled symbols font last, just before the generic fallback', () => {
  const names = FONT_STACK.split(', ')
  expect(names.at(-1)).toBe('monospace')
  expect(names.at(-2)).toBe(SYMBOLS_FONT)
  expect(names.indexOf('Consolas')).toBeLessThan(names.indexOf(SYMBOLS_FONT))
})
