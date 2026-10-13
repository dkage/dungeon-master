// The terminal's text size (#92): one range, one step, one default for the
// slider in Settings, the zoom buttons and whatever the browser has stored.

export const MIN_SIZE = 10
export const MAX_SIZE = 24
export const DEFAULT_TERMINAL_SIZE = 13

/** A whole number inside the readable range, or the fallback when the value is not a finite number. */
export function clampSize(value: unknown, fallback: number): number {
  return typeof value === 'number' && Number.isFinite(value)
    ? Math.max(MIN_SIZE, Math.min(MAX_SIZE, Math.round(value)))
    : fallback
}

export type Zoom = 'in' | 'out' | 'reset'

/** The size after a zoom step; reset returns to the default and in or out stop at the ends of the range. */
export function zoomed(size: number, step: Zoom): number {
  if (step === 'reset') return DEFAULT_TERMINAL_SIZE
  return clampSize(size + (step === 'in' ? 1 : -1), DEFAULT_TERMINAL_SIZE)
}
