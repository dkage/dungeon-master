// Typography the terminal adopts from the full-window reference (#96). The
// decisions and the planned differences are recorded in
// docs/design/art-direction.md ("Terminal typography").

// Nerd Font names come first so prompt icons (Starship, Powerline) render
// when one is installed. The bundled Fira Code follows, then the usual
// system monospace fallback. The interface serif never enters this stack.
export const FONT_STACK = [
  '"JetBrainsMono Nerd Font"', '"JetBrainsMono NF"', '"MesloLGS NF"', '"MesloLGS Nerd Font"',
  '"FiraCode Nerd Font"', '"Hack Nerd Font"', '"Symbols Nerd Font Mono"',
  '"Fira Code"',
  'ui-monospace', 'SFMono-Regular', 'Menlo', 'Consolas', 'monospace',
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

export function terminalTypography() {
  return {
    fontFamily: FONT_STACK,
    lineHeight: LINE_HEIGHT,
    fontWeight: FONT_WEIGHT,
    fontWeightBold: FONT_WEIGHT_BOLD,
  } as const
}
