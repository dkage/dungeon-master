// Is a typed terminal font installed? (#158) A browser will not list fonts, and
// it silently falls back for one it lacks, so compare text widths: a font that
// is there lays the probe text out differently from the generic font it would
// otherwise fall back to. Only the width of a probe string is read; the font
// name goes nowhere.

export type FontStatus = 'available' | 'missing' | 'unknown'

/** Width of a probe string in a font-family list, or null when it cannot be measured. */
export type Measure = (family: string) => number | null

const GENERICS = ['monospace', 'serif', 'sans-serif'] as const

/**
 * available: the font changes the width against at least one generic family.
 * missing: it changes none of them, so every one was used instead.
 * unknown: nothing could be measured (no canvas), so nothing is claimed.
 */
export function fontStatus(name: string, measure: Measure): FontStatus {
  let measured = false
  for (const generic of GENERICS) {
    const plain = measure(generic)
    const withFont = measure(`"${name}", ${generic}`)
    if (plain === null || withFont === null) continue
    measured = true
    if (Math.abs(plain - withFont) > 0.01) return 'available'
  }
  return measured ? 'missing' : 'unknown'
}

const PROBE = 'mmmmmmmmmmlli WWWW 0Oo1 {}[]'

/** Measures with a canvas, or reports that it cannot (a test DOM, a blocked canvas). */
export function canvasMeasure(): Measure {
  let ctx: CanvasRenderingContext2D | null = null
  try {
    ctx = document.createElement('canvas').getContext('2d')
  } catch {
    ctx = null
  }
  return (family) => {
    if (!ctx) return null
    ctx.font = `72px ${family}`
    const width = ctx.measureText(PROBE).width
    return Number.isFinite(width) && width > 0 ? width : null
  }
}

/** What the notice says: which font is used instead of the typed one. */
export function missingNotice(name: string, fallback: string): string {
  return `“${name}” is not installed on this computer, so the terminal uses ${fallback} instead.`
}
