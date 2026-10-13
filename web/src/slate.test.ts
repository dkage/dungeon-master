import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const css = readFileSync('src/app.css', 'utf8')

type RGB = [number, number, number]
const lin = (v: number) => ((v /= 255) <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4)
const luminance = ([r, g, b]: RGB) => 0.2126 * lin(r) + 0.7152 * lin(g) + 0.0722 * lin(b)
const contrast = (a: RGB, b: RGB) => {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x)
  return (hi + 0.05) / (lo + 0.05)
}
const hex = (h: string): RGB => [1, 3, 5].map((i) => parseInt(h.slice(i, i + 2), 16)) as RGB

const shade = /--slate-shade:\s*rgb\((\d+) (\d+) (\d+) \/ ([\d.]+)\)/.exec(css)!
const rgb = [1, 2, 3].map((i) => Number(shade[i])) as RGB
const alpha = Number(shade[4])
// Brightest pixel of assets/slate.jpg (the browser test re-measures the asset, so a new texture cannot slip past this).
const SLATE_BRIGHTEST: RGB = [86, 85, 83]
const lightest = SLATE_BRIGHTEST.map((s, i) => Math.round(s * (1 - alpha) + rgb[i] * alpha)) as RGB

// Text colours the side panels draw directly on the slate (no card or button behind them).
const text = ['#f0e6d6', '#e2d3bc', '#eadbc5', '#ad9a88', '#bea98e', '#c6a56b', '#e5cfaa', '#d9c7aa']

describe('slate behind the Dungeon side panels', () => {
  it('declares the shade and applies the texture only in Dungeon', () => {
    // A neutral dim, not a red wash: the red belongs to the cards and fields inside the panels.
    expect(rgb[0]).toBe(rgb[1])
    expect(rgb[1]).toBe(rgb[2])
    expect(alpha).toBeLessThanOrEqual(0.5)
    const rule = /\.app\[data-theme='dungeon'\] \.sidebar,\s*\.app\[data-theme='dungeon'\] \.quests \{([^}]*)\}/.exec(css)!
    expect(rule[1]).toContain('slate.jpg')
    expect(css.match(/slate\.jpg/g)).toHaveLength(1)
  })

  it.each(text)('%s reads at 4.5:1 on the brightest shaded grain', (c) => {
    expect(contrast(hex(c), lightest)).toBeGreaterThanOrEqual(4.5)
  })
})

describe('nothing paints over the slate by accident', () => {
  it('the word-hint wrapper has a class no global rule uses', () => {
    // It once shared .term-wrap with the terminal frame and drew a red block behind every themed heading.
    const term = readFileSync('src/lib/Term.svelte', 'utf8')
    const wrapper = /^<span class="([\w-]+)">/m.exec(term)![1]
    expect(wrapper).toBe('word-hint')
    expect(css).not.toMatch(new RegExp(`\\.${wrapper}\\b`))
  })
})
