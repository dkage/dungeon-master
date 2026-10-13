import { expect, it } from 'vitest'
import { DEFAULT_SETTINGS, parseSettings } from './appearance'

it('gives the defaults for anything that is not an object', () => {
  for (const bad of [null, undefined, 'x', 3, [], true]) expect(parseSettings(bad)).toEqual(DEFAULT_SETTINGS)
})

it('keeps good fields and defaults each bad one', () => {
  const got = parseSettings({ theme: 'neutral', font: 'comic', terminalSize: 99, questsSize: '20', notifications: 'yes', terminalLineHeight: 1.5, terminalColors: 'rainbow' })
  expect(got).toEqual({ ...DEFAULT_SETTINGS, theme: 'neutral', terminalSize: 24, terminalLineHeight: 1.5 })
})

it('accepts a plain custom font name and refuses one that could break the font list', () => {
  expect(parseSettings({ terminalFont: 'custom', terminalFontName: 'JetBrains Mono' })).toMatchObject({ terminalFont: 'custom', terminalFontName: 'JetBrains Mono' })
  for (const evil of ['Fira", sans-serif', "a'b", 'a;b', 'a/b', ' lead', '', 'x'.repeat(65), 5]) {
    expect(parseSettings({ terminalFont: 'custom', terminalFontName: evil })).toMatchObject({ terminalFont: 'stack', terminalFontName: '' })
  }
})

it('only accepts the line heights the server offers', () => {
  expect(parseSettings({ terminalLineHeight: 1.15 }).terminalLineHeight).toBe(1.15)
  expect(parseSettings({ terminalLineHeight: 1.2 }).terminalLineHeight).toBe(1.3)
})
