import { describe, expect, it } from 'vitest'
import { fontStatus, missingNotice, type Measure } from './termFont'

// A fake browser: installed fonts have their own widths, everything else falls back to the generic's.
const generic: Record<string, number> = { monospace: 600, serif: 550, 'sans-serif': 520 }
const browser = (installed: Record<string, number>): Measure => (family) => {
  const m = /^"(.+)", (.+)$/.exec(family)
  if (!m) return generic[family]
  return installed[m[1]] ?? generic[m[2]]
}

describe('fontStatus', () => {
  it('finds an installed font', () => {
    expect(fontStatus('Iosevka', browser({ Iosevka: 480 }))).toBe('available')
  })
  it('finds an installed font that happens to match one generic width', () => {
    expect(fontStatus('Menlo', browser({ Menlo: 600 }))).toBe('available')
  })
  it('calls a font missing when every probe falls back', () => {
    expect(fontStatus('Not A Real Font', browser({}))).toBe('missing')
  })
  it('says unknown when nothing can be measured', () => {
    expect(fontStatus('X', () => null)).toBe('unknown')
    // One failed probe is skipped; the others still decide.
    const partial: Measure = (f) => (f.includes('serif') && !f.includes('sans') ? null : browser({ X: 400 })(f))
    expect(fontStatus('X', partial)).toBe('available')
  })
})

it('names the fallback in the notice', () => {
  expect(missingNotice('Foo', 'Fira Code')).toBe('“Foo” is not installed on this computer, so the terminal uses Fira Code instead.')
})
