import { readFileSync } from 'node:fs'
import { inflateSync } from 'node:zlib'
import { describe, expect, it } from 'vitest'
import { PARCHMENT_ZOOM, TERM_THEMES } from './termTheme'

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

type RGB = [number, number, number]
const lin = (v: number) => ((v /= 255) <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4)
const luminance = ([r, g, b]: RGB) => 0.2126 * lin(r) + 0.7152 * lin(g) + 0.0722 * lin(b)
const contrast = (a: RGB, b: RGB) => {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x)
  return (hi + 0.05) / (lo + 0.05)
}
const hex = (h: string): RGB => [1, 3, 5].map((i) => parseInt(h.slice(i, i + 2), 16)) as RGB

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
  const colours = ['foreground', 'cursor', 'black', 'red', 'green', 'yellow', 'blue', 'magenta', 'cyan', 'white',
    'brightBlack', 'brightRed', 'brightGreen', 'brightYellow', 'brightBlue', 'brightMagenta', 'brightCyan', 'brightWhite'] as const

  it('finds a texture tone that is not near-black', () => {
    expect(luminance(dark)).toBeGreaterThan(0.3)
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

  it('is transparent in Dungeon and opaque in Neutral', () => {
    expect(t.background).toBe('rgba(0, 0, 0, 0)')
    expect(TERM_THEMES.neutral.background).toMatch(/^#/)
  })
})
