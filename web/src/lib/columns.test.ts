import { describe, expect, it } from 'vitest'
import { DEFAULTS, MIN_TERMINAL, fit, keyWidth, limits, parse, setWidth } from './columns'

describe('columns', () => {
  it('clamps to the minimums and to leave the terminal its share', () => {
    expect(setWidth(DEFAULTS, 'sidebar', 10, 1600).sidebar).toBe(200)
    expect(setWidth(DEFAULTS, 'sidebar', 9999, 1600).sidebar).toBe(480)
    const tight = setWidth(DEFAULTS, 'quests', 9999, 1300)
    expect(1300 - DEFAULTS.sidebar - tight.quests).toBeGreaterThanOrEqual(MIN_TERMINAL)
    expect(limits('quests', 246, 1600).max).toBe(760)
  })
  it('steps with the arrows in the right direction, Home/End go to the limits, Enter resets', () => {
    expect(keyWidth(DEFAULTS, 'sidebar', 'ArrowRight', 1600)!.sidebar).toBe(262)
    expect(keyWidth(DEFAULTS, 'quests', 'ArrowLeft', 1600)!.quests).toBe(476)
    expect(keyWidth(DEFAULTS, 'quests', 'ArrowRight', 1600)!.quests).toBe(444)
    expect(keyWidth(DEFAULTS, 'sidebar', 'Home', 1600)!.sidebar).toBe(200)
    expect(keyWidth(DEFAULTS, 'sidebar', 'End', 1600)!.sidebar).toBe(480)
    expect(keyWidth({ sidebar: 300, quests: 500 }, 'sidebar', 'Enter', 1600)).toEqual({ sidebar: 246, quests: 500 })
    expect(keyWidth(DEFAULTS, 'sidebar', 'a', 1600)).toBeNull()
  })
  it('falls back to defaults when saved widths no longer fit the window', () => {
    const wide = { sidebar: 400, quests: 700 }
    expect(fit(wide, 1700)).toEqual(wide)
    const small = fit(wide, 1200)
    expect(small.sidebar + small.quests + MIN_TERMINAL).toBeLessThanOrEqual(1200)
  })
  it('reads saved widths and ignores unreadable data', () => {
    expect(parse('{"sidebar":300,"quests":500}')).toEqual({ sidebar: 300, quests: 500 })
    expect(parse('nonsense')).toBeNull()
    expect(parse('{"sidebar":"x"}')).toBeNull()
    expect(parse(null)).toBeNull()
  })
})
