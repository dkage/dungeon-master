/**
 * Widths of the two side columns of the workspace; the terminal takes what is left. Pure, so the
 * clamping and the keyboard steps are testable without a browser.
 */
export type Columns = { sidebar: number; quests: number }
export type Edge = 'sidebar' | 'quests'

export const DEFAULTS: Columns = { sidebar: 246, quests: 460 }
const MIN: Columns = { sidebar: 200, quests: 340 }
const MAX: Columns = { sidebar: 480, quests: 760 }
/** The terminal never drops below a working size. */
export const MIN_TERMINAL = 400
/** At and below this width the quest panel is a drawer and saved widths do not apply. */
export const DRAWER_BREAKPOINT = 1180
export const STEP = 16

/** The widest an edge may be at this window width, given the other column. */
export function limits(edge: Edge, other: number, windowWidth: number): { min: number; max: number } {
  const room = windowWidth - other - MIN_TERMINAL
  return { min: MIN[edge], max: Math.max(MIN[edge], Math.min(MAX[edge], room)) }
}

export function setWidth(c: Columns, edge: Edge, width: number, windowWidth: number): Columns {
  const other = edge === 'sidebar' ? c.quests : c.sidebar
  const { min, max } = limits(edge, other, windowWidth)
  return { ...c, [edge]: Math.round(Math.max(min, Math.min(max, width))) }
}

/** Saved widths that no longer fit the window fall back to the defaults. */
export function fit(c: Columns, windowWidth: number): Columns {
  if (c.sidebar + c.quests + MIN_TERMINAL <= windowWidth) return c
  return setWidth(setWidth(DEFAULTS, 'sidebar', DEFAULTS.sidebar, windowWidth), 'quests', DEFAULTS.quests, windowWidth)
}

/** Keyboard on a separator. `grow` is the direction that widens the column. */
export function keyWidth(c: Columns, edge: Edge, key: string, windowWidth: number): Columns | null {
  const other = edge === 'sidebar' ? c.quests : c.sidebar
  const { min, max } = limits(edge, other, windowWidth)
  const grow = edge === 'sidebar' ? 'ArrowRight' : 'ArrowLeft'
  const shrink = edge === 'sidebar' ? 'ArrowLeft' : 'ArrowRight'
  if (key === grow) return setWidth(c, edge, c[edge] + STEP, windowWidth)
  if (key === shrink) return setWidth(c, edge, c[edge] - STEP, windowWidth)
  if (key === 'Home') return setWidth(c, edge, min, windowWidth)
  if (key === 'End') return setWidth(c, edge, max, windowWidth)
  if (key === 'Enter') return setWidth(c, edge, DEFAULTS[edge], windowWidth)
  return null
}

export const KEY = 'dungeon.columns'

export function parse(raw: string | null): Columns | null {
  if (!raw) return null
  try {
    const v = JSON.parse(raw) as Partial<Columns>
    if (typeof v.sidebar === 'number' && typeof v.quests === 'number' && Number.isFinite(v.sidebar + v.quests)) {
      return { sidebar: v.sidebar, quests: v.quests }
    }
  } catch {
    /* unreadable: use defaults */
  }
  return null
}
