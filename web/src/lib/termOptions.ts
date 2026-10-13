// Typography the terminal adopts from the full-window reference (#96). The
// decisions and the planned differences are recorded in
// docs/design/art-direction.md ("Terminal typography").

// Nerd Font names come first so prompt icons (Starship, Powerline) render
// when one is installed. The bundled Fira Code follows, then the usual
// system monospace. The bundled symbols-only font comes last before the generic
// fallback, so it supplies only glyphs the primary fonts lack. The interface
// serif never enters this stack.
export const SYMBOLS_FONT = '"Dungeon Symbols"'
export const FONT_STACK = [
  '"JetBrainsMono Nerd Font"', '"JetBrainsMono NF"', '"MesloLGS NF"', '"MesloLGS Nerd Font"',
  '"FiraCode Nerd Font"', '"Hack Nerd Font"', '"Symbols Nerd Font Mono"',
  '"Fira Code"',
  'ui-monospace', 'SFMono-Regular', 'Menlo', 'Consolas',
  '"Dungeon Symbols"', 'monospace',
].join(', ')

// Extra row height. The reference uses 1.7, which costs a third of the rows a
// TUI needs; 1.3 keeps its airiness while box drawing stays joined.
export const LINE_HEIGHT = 1.3

// Normal text stays regular weight: bold everywhere would remove the agent's
// own emphasis, and contrast comes from the colours (#93), not the weight.
export const FONT_WEIGHT = 'normal'
export const FONT_WEIGHT_BOLD = 'bold'

// Padding inside the parchment frame, from the reference (22px 28px). The
// Neutral appearance keeps the compact padding.
export const FRAME_PADDING = { dungeon: '22px 28px', neutral: '8px' } as const

// The other choices of the "Terminal font" setting (#150). Every one ends with
// the symbols font and the generic fallback, so prompt icons always render.
export const BUNDLED_STACK = ['"Fira Code"', SYMBOLS_FONT, 'monospace'].join(', ')
export const SYSTEM_STACK = ['ui-monospace', 'SFMono-Regular', 'Menlo', 'Consolas', SYMBOLS_FONT, 'monospace'].join(', ')

export interface TypographyChoice {
  terminalFont?: 'stack' | 'bundled' | 'system' | 'custom'
  terminalFontName?: string
  terminalLineHeight?: number
}

/** A plain font name only: this is what keeps a typed name from leaving the quotes it is put in. */
export const PLAIN_FONT_NAME = /^[A-Za-z0-9][A-Za-z0-9 ._-]{0,63}$/

/** The font-family list for a choice. A custom name goes first, quoted; a bad one falls back to the default list. */
export function fontFamilyFor(choice: Pick<TypographyChoice, 'terminalFont' | 'terminalFontName'> = {}): string {
  switch (choice.terminalFont) {
    case 'bundled': return BUNDLED_STACK
    case 'system': return SYSTEM_STACK
    case 'custom': {
      const name = choice.terminalFontName ?? ''
      return PLAIN_FONT_NAME.test(name) ? `"${name}", ${FONT_STACK}` : FONT_STACK
    }
    default: return FONT_STACK
  }
}

/** The font family a custom name resolves to when it is not installed: the list after it. */
export function fallbackFontName(choice: Pick<TypographyChoice, 'terminalFont'>): string {
  return choice.terminalFont === 'system' ? 'system monospace' : 'Fira Code'
}

export function terminalTypography(choice: TypographyChoice = {}) {
  return {
    fontFamily: fontFamilyFor(choice),
    lineHeight: choice.terminalLineHeight ?? LINE_HEIGHT,
    fontWeight: FONT_WEIGHT,
    fontWeightBold: FONT_WEIGHT_BOLD,
  } as const
}
