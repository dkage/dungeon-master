import type { ITheme } from '@xterm/xterm'

// The Dungeon page behind the terminal is the parchment texture, so its
// background is transparent and every text colour must read on the darkest
// tone the texture can show there. The CSS paints the texture at
// PARCHMENT_ZOOM times the frame size, centred, so only the central
// 1/PARCHMENT_ZOOM of the image sits under the terminal; the burnt edges
// stay outside it. termTheme.test.ts samples that same crop.
export const PARCHMENT_ZOOM = 1.4

export const TERM_THEMES: Record<'dungeon' | 'neutral', ITheme> = {
  dungeon: {
    background: 'rgba(0, 0, 0, 0)', foreground: '#1a140e', cursor: '#841818', cursorAccent: '#f3ebd8',
    selectionBackground: 'rgba(132, 24, 24, 0.24)', selectionInactiveBackground: 'rgba(132, 24, 24, 0.12)',
    black: '#342417', red: '#841818', green: '#1c4e2b', yellow: '#654010', blue: '#1f4560', magenta: '#5c3259', cyan: '#17504d', white: '#4d3d2c',
    brightBlack: '#5a4632', brightRed: '#8a1a1a', brightGreen: '#1a5430', brightYellow: '#664000', brightBlue: '#1b4a72', brightMagenta: '#693069', brightCyan: '#0d4f4d', brightWhite: '#1a140e',
  },
  neutral: {
    background: '#14141a', foreground: '#e8e8e4', cursor: '#e8e8e4', cursorAccent: '#14141a',
    selectionBackground: 'rgba(143, 120, 255, 0.45)', selectionInactiveBackground: 'rgba(143, 120, 255, 0.25)',
  },
}
