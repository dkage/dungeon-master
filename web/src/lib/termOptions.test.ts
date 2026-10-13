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

import { BUNDLED_STACK, SYSTEM_STACK, fontFamilyFor } from './termOptions'

it('offers the default list, bundled Fira Code alone, system monospace and a typed name', () => {
  expect(fontFamilyFor()).toBe(FONT_STACK)
  expect(fontFamilyFor({ terminalFont: 'stack' })).toBe(FONT_STACK)
  expect(fontFamilyFor({ terminalFont: 'bundled' })).toBe(BUNDLED_STACK)
  expect(fontFamilyFor({ terminalFont: 'system' })).toBe(SYSTEM_STACK)
  expect(fontFamilyFor({ terminalFont: 'custom', terminalFontName: 'JetBrains Mono' })).toBe(`"JetBrains Mono", ${FONT_STACK}`)
})

it('keeps the symbols font and the generic fallback last in every option', () => {
  const all = [
    fontFamilyFor({ terminalFont: 'stack' }), fontFamilyFor({ terminalFont: 'bundled' }), fontFamilyFor({ terminalFont: 'system' }),
    fontFamilyFor({ terminalFont: 'custom', terminalFontName: 'Iosevka' }),
  ]
  for (const family of all) {
    const names = family.split(', ')
    expect(names.at(-1)).toBe('monospace')
    expect(names.at(-2)).toBe(SYMBOLS_FONT)
    expect(family).not.toMatch(/Almendra|Garamond|Alegreya|Georgia|serif/i)
  }
  expect(BUNDLED_STACK).not.toMatch(/ui-monospace|Menlo/)
})

it('puts a typed name first and refuses anything that could break out of the list', () => {
  for (const evil of ['Fira", sans-serif; x', "a'b", 'a;b', 'a/b', 'a\\b', ' lead', '', 'x'.repeat(65), '"']) {
    expect(fontFamilyFor({ terminalFont: 'custom', terminalFontName: evil })).toBe(FONT_STACK)
  }
  expect(fontFamilyFor({ terminalFont: 'custom', terminalFontName: 'My Font-2.0' }).startsWith('"My Font-2.0", ')).toBe(true)
})

it('takes the line height from the choice and defaults to 1.3', () => {
  expect(terminalTypography({ terminalLineHeight: 1.5 }).lineHeight).toBe(1.5)
  expect(terminalTypography({}).lineHeight).toBe(LINE_HEIGHT)
})
