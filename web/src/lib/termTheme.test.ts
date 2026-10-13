import { readFileSync } from 'node:fs'
import { inflateSync } from 'node:zlib'
import { describe, expect, it } from 'vitest'
import {
  ANSI_NAMES, MIN_CONTRAST, PARCHMENT_TONE, PARCHMENT_ZOOM, TERM_BACKDROP, TERM_THEMES, contrast, dimColour, dimStyles, schemeFor,
  hexRGB as hex, luminance, type RGB,
} from './termTheme'

// Decodes the 8-bit RGB, non-interlaced PNG the texture is shipped as.
function decodePng(buf: Buffer) {
  let pos = 8
  let width = 0
  let height = 0
  const data: Buffer[] = []
  while (pos < buf.length) {
    const len = buf.readUInt32BE(pos)
    const type = buf.toString('latin1', pos + 4, pos + 8)
    const body = buf.subarray(pos + 8, pos + 8 + len)
    if (type === 'IHDR') {
      width = body.readUInt32BE(0)
      height = body.readUInt32BE(4)
      expect([body[8], body[9], body[12]]).toEqual([8, 2, 0])
    } else if (type === 'IDAT') data.push(body)
    pos += 12 + len
  }
  const raw = inflateSync(Buffer.concat(data))
  const stride = width * 3
  const out = Buffer.alloc(stride * height)
  for (let y = 0; y < height; y++) {
    const filter = raw[y * (stride + 1)]
    for (let x = 0; x < stride; x++) {
      const v = raw[y * (stride + 1) + 1 + x]
      const a = x >= 3 ? out[y * stride + x - 3] : 0
      const b = y > 0 ? out[(y - 1) * stride + x] : 0
      const c = x >= 3 && y > 0 ? out[(y - 1) * stride + x - 3] : 0
      let add = 0
      if (filter === 1) add = a
      else if (filter === 2) add = b
      else if (filter === 3) add = (a + b) >> 1
      else if (filter === 4) {
        const p = a + b - c
        const pa = Math.abs(p - a), pb = Math.abs(p - b), pc = Math.abs(p - c)
        add = pa <= pb && pa <= pc ? a : pb <= pc ? b : c
      }
      out[y * stride + x] = (v + add) & 255
    }
  }
  return { width, height, rgb: out }
}

// The darkest 8x8 average inside the crop of the texture that shows behind the terminal.
function darkestTone(): RGB {
  const { width, height, rgb } = decodePng(readFileSync('src/assets/parchment.png'))
  const keep = 1 / PARCHMENT_ZOOM
  const x0 = Math.floor((width * (1 - keep)) / 2), x1 = Math.floor((width * (1 + keep)) / 2)
  const y0 = Math.floor((height * (1 - keep)) / 2), y1 = Math.floor((height * (1 + keep)) / 2)
  const B = 8
  let best: RGB | undefined
  for (let y = y0; y + B <= y1; y += B)
    for (let x = x0; x + B <= x1; x += B) {
      const sum = [0, 0, 0]
      for (let j = 0; j < B; j++)
        for (let i = 0; i < B; i++) for (let k = 0; k < 3; k++) sum[k] += rgb[((y + j) * width + x + i) * 3 + k]
      const mean = sum.map((s) => Math.floor(s / (B * B))) as RGB
      if (!best || luminance(mean) < luminance(best)) best = mean
    }
  return best!
}

describe('Dungeon terminal palette on the parchment', () => {
  const dark = darkestTone()
  const t = TERM_THEMES.dungeon
  const colours = ['foreground', 'cursor', ...ANSI_NAMES] as const

  it('finds a texture tone that is not near-black', () => {
    expect(luminance(dark)).toBeGreaterThan(0.3)
  })

  it('measures contrast against that darkest tone', () => {
    expect(hex(PARCHMENT_TONE)).toEqual(dark)
    expect(TERM_BACKDROP.dungeon).toBe(PARCHMENT_TONE)
  })

  it.each(colours)('%s reads at 4.5:1 on the darkest tone', (name) => {
    expect(contrast(hex(t[name]!), dark)).toBeGreaterThanOrEqual(4.5)
  })

  it('keeps the foreground readable on top of the selection', () => {
    const m = /rgba\((\d+), (\d+), (\d+), ([\d.]+)\)/.exec(t.selectionBackground!)!
    const a = Number(m[4])
    const sel = [1, 2, 3].map((i) => Math.round(Number(m[i]) * a + dark[i - 1] * (1 - a))) as RGB
    expect(contrast(hex(t.foreground!), sel)).toBeGreaterThanOrEqual(4.5)
  })

  // The background is painted transparent, but xterm's minimumContrastRatio
  // reads its colour channels and ignores the alpha.
  it('is transparent in Dungeon, tinted with the parchment tone, and opaque in Neutral', () => {
    const m = /^rgba\((\d+), (\d+), (\d+), 0\)$/.exec(t.background!)!
    expect(m).not.toBeNull()
    expect([1, 2, 3].map((i) => Number(m[i]))).toEqual(dark)
    expect(TERM_THEMES.neutral.background).toBe(TERM_BACKDROP.neutral)
  })
})

describe('gray and dimmed text in both appearances', () => {
  // Claude Code prints hints and placeholders as dim (SGR 2) text or in
  // bright black; both must reach WCAG AA on each background.
  describe.each(['dungeon', 'neutral', 'high-contrast', 'light'] as const)('%s', (name) => {
    const t = TERM_THEMES[name]
    const bg = hex(TERM_BACKDROP[name])

    it('gives every ANSI colour explicitly', () => {
      for (const key of ANSI_NAMES) expect(t[key]).toMatch(/^#[0-9a-f]{6}$/)
    })

    it('shows bright black at 4.5:1, apart from the foreground', () => {
      expect(contrast(hex(t.brightBlack!), bg)).toBeGreaterThanOrEqual(MIN_CONTRAST)
      expect(contrast(hex(t.brightBlack!), hex(t.foreground!))).toBeGreaterThan(1.5)
    })

    it.each(['foreground', ...ANSI_NAMES] as const)('dims %s no lower than 4.5:1', (key) => {
      const colour = t[key]!
      const dim = dimColour(colour, TERM_BACKDROP[name])
      // A colour already under the target (black in Neutral) is left alone;
      // xterm's minimumContrastRatio lifts it.
      const floor = Math.min(MIN_CONTRAST, contrast(hex(colour), bg))
      expect(contrast(hex(dim), bg)).toBeGreaterThanOrEqual(floor)
    })

    it('keeps dimmed default text distinguishable from normal text', () => {
      const dim = hex(dimColour(t.foreground!, TERM_BACKDROP[name]))
      expect(contrast(dim, hex(t.foreground!))).toBeGreaterThan(1.5)
    })

    it('replaces xterm\'s half-opacity dim rules for this theme', () => {
      const css = dimStyles()
      expect(css).toContain(`[data-term-theme="${name}"] .xterm .xterm-rows span.xterm-dim:not([class*="xterm-fg-"]) { color: ${dimColour(t.foreground!, TERM_BACKDROP[name])}; }`)
      expect(css).toContain(`[data-term-theme="${name}"] .xterm .xterm-rows span.xterm-fg-8.xterm-dim { color: ${dimColour(t.brightBlack!, TERM_BACKDROP[name])}; }`)
    })
  })

  it('fades only as far as the target allows', () => {
    // Half opacity reads well enough on Neutral's white.
    expect(dimColour('#ffffff', '#000000')).toBe('#808080')
    // A colour at the limit cannot fade at all.
    expect(dimColour(TERM_THEMES.dungeon.brightBlack!, PARCHMENT_TONE)).toBe(TERM_THEMES.dungeon.brightBlack)
  })
})

describe('terminal colour schemes (#154)', () => {
  const opaque = ['high-contrast', 'light'] as const

  it.each(opaque)('%s paints an opaque background that matches its backdrop', (name) => {
    expect(TERM_THEMES[name].background).toBe(TERM_BACKDROP[name])
  })

  it.each(opaque)('%s shows the foreground and every colour at WCAG AA', (name) => {
    const bg = hex(TERM_BACKDROP[name])
    for (const key of ['foreground', 'cursor', ...ANSI_NAMES] as const) {
      expect(contrast(hex(TERM_THEMES[name][key]!), bg), `${name} ${key}`).toBeGreaterThanOrEqual(MIN_CONTRAST)
    }
  })

  it('reaches AAA (7:1) for everything in high contrast', () => {
    const bg = hex(TERM_BACKDROP['high-contrast'])
    for (const key of ['foreground', ...ANSI_NAMES] as const) {
      expect(contrast(hex(TERM_THEMES['high-contrast'][key]!), bg), key).toBeGreaterThanOrEqual(7)
      expect(contrast(hex(dimColour(TERM_THEMES['high-contrast'][key]!, TERM_BACKDROP['high-contrast'])), bg), `dim ${key}`).toBeGreaterThanOrEqual(MIN_CONTRAST)
    }
  })

  it('keeps selected text readable in each scheme', () => {
    const hc = TERM_THEMES['high-contrast']
    expect(contrast(hex(hc.selectionForeground!), hex(hc.selectionBackground!))).toBeGreaterThanOrEqual(7)
    const light = TERM_THEMES.light
    const m = /rgba\((\d+), (\d+), (\d+), ([\d.]+)\)/.exec(light.selectionBackground!)!
    const a = Number(m[4]), bg = hex(TERM_BACKDROP.light)
    const sel = [1, 2, 3].map((i) => Math.round(Number(m[i]) * a + bg[i - 1] * (1 - a))) as RGB
    expect(contrast(hex(light.foreground!), sel)).toBeGreaterThanOrEqual(MIN_CONTRAST)
  })

  it('follows the appearance unless a scheme is chosen, and ignores it otherwise', () => {
    expect(schemeFor('default', 'dungeon')).toBe('dungeon')
    expect(schemeFor('default', 'neutral')).toBe('neutral')
    for (const appearance of ['dungeon', 'neutral'] as const) {
      expect(schemeFor('high-contrast', appearance)).toBe('high-contrast')
      expect(schemeFor('light', appearance)).toBe('light')
    }
  })
})
