// The look-and-feel choices and how they are checked. The server keeps the
// authoritative copy (internal/appearance); the same allowed values are
// applied here so a bad stored or received value falls back to its default.
import { DEFAULT_TERMINAL_SIZE, clampSize } from './termSize'

export type ReadingFont = 'almendra' | 'eb-garamond' | 'alegreya'
export type WordMode = 'hints' | 'themed' | 'plain'
export type ThemePack = 'dungeon' | 'neutral'
export type TerminalFontChoice = 'stack' | 'bundled' | 'system' | 'custom'
export type TerminalColours = 'default' | 'high-contrast' | 'light'

export interface UISettings {
  font: ReadingFont
  sidebarSize: number
  terminalSize: number
  questsSize: number
  wordMode: WordMode
  theme: ThemePack
  /** Desktop notification when a session waits and this window is not in front. Off until the user asks. */
  notifications: boolean
  terminalFont: TerminalFontChoice
  /** The installed font name used when terminalFont is 'custom'. */
  terminalFontName: string
  terminalLineHeight: number
  terminalColors: TerminalColours
}

export const LINE_HEIGHTS = [1.15, 1.3, 1.5] as const
/** A plain font name; no quotes, commas or slashes, so it cannot leave the font list it is put in. */
export const FONT_NAME = /^[A-Za-z0-9][A-Za-z0-9 ._-]{0,63}$/

export const DEFAULT_SETTINGS: UISettings = {
  font: 'almendra', sidebarSize: 14, terminalSize: DEFAULT_TERMINAL_SIZE, questsSize: 14, wordMode: 'hints', theme: 'dungeon',
  notifications: false, terminalFont: 'stack', terminalFontName: '', terminalLineHeight: 1.3, terminalColors: 'default',
}

const allowed = <T extends string | number>(value: unknown, values: readonly T[], fallback: T): T =>
  values.includes(value as T) ? value as T : fallback

/** Settings from anything: each bad field gets its default, a non-object gives all defaults. */
export function parseSettings(raw: unknown): UISettings {
  const p = raw && typeof raw === 'object' ? (raw as Record<string, unknown>) : {}
  const name = typeof p.terminalFontName === 'string' && FONT_NAME.test(p.terminalFontName) ? p.terminalFontName : ''
  let terminalFont = allowed(p.terminalFont, ['stack', 'bundled', 'system', 'custom'] as const, DEFAULT_SETTINGS.terminalFont)
  if (terminalFont === 'custom' && !name) terminalFont = DEFAULT_SETTINGS.terminalFont
  return {
    font: allowed(p.font, ['almendra', 'eb-garamond', 'alegreya'] as const, DEFAULT_SETTINGS.font),
    sidebarSize: clampSize(p.sidebarSize, DEFAULT_SETTINGS.sidebarSize),
    terminalSize: clampSize(p.terminalSize, DEFAULT_SETTINGS.terminalSize),
    questsSize: clampSize(p.questsSize, DEFAULT_SETTINGS.questsSize),
    wordMode: allowed(p.wordMode, ['hints', 'themed', 'plain'] as const, DEFAULT_SETTINGS.wordMode),
    theme: allowed(p.theme, ['dungeon', 'neutral'] as const, DEFAULT_SETTINGS.theme),
    notifications: p.notifications === true,
    terminalFont,
    terminalFontName: name,
    terminalLineHeight: allowed(p.terminalLineHeight, LINE_HEIGHTS, DEFAULT_SETTINGS.terminalLineHeight),
    terminalColors: allowed(p.terminalColors, ['default', 'high-contrast', 'light'] as const, DEFAULT_SETTINGS.terminalColors),
  }
}
