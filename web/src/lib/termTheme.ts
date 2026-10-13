import type { ITheme } from '@xterm/xterm'

// The Dungeon page behind the terminal is the parchment texture, so its
// background is transparent and every text colour must read on the darkest
// tone the texture can show there. The CSS paints the texture at
// PARCHMENT_ZOOM times the frame size, centred, so only the central
// 1/PARCHMENT_ZOOM of the image sits under the terminal; the burnt edges
// stay outside it. termTheme.test.ts samples that same crop.
export const PARCHMENT_ZOOM = 1.4

// The darkest 8x8 average of that crop (termTheme.test.ts re-measures it).
// xterm ignores the alpha when it checks minimumContrastRatio, so the
// transparent Dungeon background carries this colour: grays the agent prints
// are then darkened against the parchment instead of lightened against black.
export const PARCHMENT_TONE = '#edaf53'

// The contrast every text colour, dimmed or not, must reach (WCAG AA).
export const MIN_CONTRAST = 4.5

export type TermThemeName = 'dungeon' | 'neutral' | 'high-contrast' | 'light'

export const TERM_THEMES: Record<TermThemeName, ITheme> = {
  // Filled in below, after the constants they use.
  'high-contrast': {},
  light: {},
  dungeon: {
    background: 'rgba(237, 175, 83, 0)', foreground: '#1a140e', cursor: '#841818', cursorAccent: '#f3ebd8',
    selectionBackground: 'rgba(132, 24, 24, 0.24)', selectionInactiveBackground: 'rgba(132, 24, 24, 0.12)',
    black: '#342417', red: '#841818', green: '#1c4e2b', yellow: '#654010', blue: '#1f4560', magenta: '#5c3259', cyan: '#17504d', white: '#4d3d2c',
    brightBlack: '#5a4632', brightRed: '#8a1a1a', brightGreen: '#1a5430', brightYellow: '#664000', brightBlue: '#1b4a72', brightMagenta: '#693069', brightCyan: '#0d4f4d', brightWhite: '#1a140e',
  },
  // xterm's default palette, except bright black, whose default (#555753)
  // reads at 2.5:1 on this background.
  neutral: {
    background: '#14141a', foreground: '#e8e8e4', cursor: '#e8e8e4', cursorAccent: '#14141a',
    selectionBackground: 'rgba(143, 120, 255, 0.45)', selectionInactiveBackground: 'rgba(143, 120, 255, 0.25)',
    black: '#2e3436', red: '#cc0000', green: '#4e9a06', yellow: '#c4a000', blue: '#3465a4', magenta: '#75507b', cyan: '#06989a', white: '#d3d7cf',
    brightBlack: '#9a9aa0', brightRed: '#ef2929', brightGreen: '#8ae234', brightYellow: '#fce94f', brightBlue: '#729fcf', brightMagenta: '#ad7fa8', brightCyan: '#34e2e2', brightWhite: '#eeeeec',
  },
}

// The colour text sits on in each scheme, for contrast checks.
export const TERM_BACKDROP: Record<TermThemeName, string> = {
  dungeon: PARCHMENT_TONE, neutral: '#14141a', 'high-contrast': '#000000', light: '#fbf8ef',
}

// The "Terminal colours" setting (#154). Default follows the appearance; the
// other two schemes are opaque and ignore it, so they read the same in both.
export function schemeFor(colours: 'default' | 'high-contrast' | 'light', appearance: 'dungeon' | 'neutral'): TermThemeName {
  return colours === 'default' ? appearance : colours
}

// Every colour reaches at least 7:1 (WCAG AAA) on black, bright black and the
// ANSI black included, and the selection inverts to black on white.
TERM_THEMES['high-contrast'] = {
  background: '#000000', foreground: '#ffffff', cursor: '#ffffff', cursorAccent: '#000000',
  selectionBackground: '#ffffff', selectionForeground: '#000000', selectionInactiveBackground: '#8c8c8c',
  black: '#a8a8a8', red: '#ff7b72', green: '#5df06a', yellow: '#ffe45c', blue: '#7fb4ff', magenta: '#ff8bff', cyan: '#4ff0f0', white: '#e6e6e6',
  brightBlack: '#b8b8b8', brightRed: '#ff9a92', brightGreen: '#8bff93', brightYellow: '#fff085', brightBlue: '#a5ccff', brightMagenta: '#ffb0ff', brightCyan: '#85ffff', brightWhite: '#ffffff',
}

// The Dungeon palette was made for light parchment, so it serves a plain light page too.
TERM_THEMES.light = { ...TERM_THEMES.dungeon, background: '#fbf8ef' }

export const ANSI_NAMES = ['black', 'red', 'green', 'yellow', 'blue', 'magenta', 'cyan', 'white',
  'brightBlack', 'brightRed', 'brightGreen', 'brightYellow', 'brightBlue', 'brightMagenta', 'brightCyan', 'brightWhite'] as const

export type RGB = [number, number, number]
export const hexRGB = (h: string): RGB => [1, 3, 5].map((i) => parseInt(h.slice(i, i + 2), 16)) as RGB
const toHex = (c: RGB) => '#' + c.map((v) => v.toString(16).padStart(2, '0')).join('')
const lin = (v: number) => ((v /= 255) <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4)
export const luminance = ([r, g, b]: RGB) => 0.2126 * lin(r) + 0.7152 * lin(g) + 0.0722 * lin(b)
export const contrast = (a: RGB, b: RGB) => {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x)
  return (hi + 0.05) / (lo + 0.05)
}
const mix = (fg: RGB, bg: RGB, a: number) => fg.map((v, i) => Math.round(bg[i] + (v - bg[i]) * a)) as RGB

// xterm draws dim (SGR 2) text at half opacity, which leaves it below AA on
// both backgrounds. Instead fade the colour only as far as it still reads at
// MIN_CONTRAST: half opacity when that is enough, the full colour when even
// that colour is at the limit (or below it, as black is in Neutral).
export function dimColour(colour: string, backdrop: string): string {
  const fg = hexRGB(colour), bg = hexRGB(backdrop)
  for (let a = 0.5; a < 1; a += 0.05) {
    const c = mix(fg, bg, a)
    if (contrast(c, bg) >= MIN_CONTRAST) return toHex(c)
  }
  return colour
}

// CSS that replaces xterm's half-opacity dim rules for a terminal host
// marked with data-term-theme. The selectors outrank xterm's own (three
// classes); inline colours xterm sets for minimumContrastRatio still win.
// The default-colour rule skips spans with a palette class so 256-colour
// dim text keeps xterm's rule.
export function dimStyles(): string {
  let css = ''
  for (const name of Object.keys(TERM_THEMES) as TermThemeName[]) {
    const t = TERM_THEMES[name], bg = TERM_BACKDROP[name]
    const scope = `[data-term-theme="${name}"] .xterm .xterm-rows span`
    css += `${scope}.xterm-dim:not([class*="xterm-fg-"]) { color: ${dimColour(t.foreground!, bg)}; }\n`
    ANSI_NAMES.forEach((key, i) => {
      css += `${scope}.xterm-fg-${i}.xterm-dim { color: ${dimColour(t[key]!, bg)}; }\n`
    })
  }
  return css
}
