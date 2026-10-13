import { describe, expect, it } from 'vitest'
import { MAX_UPLOAD_BYTES, fileToBase64, filesOf, pathsText, quotePath, refusal } from './termUpload'

describe('quotePath', () => {
  it('leaves a safe path bare', () => {
    expect(quotePath('/Users/me/.dungeon/uploads/s1/ab12-shot.png')).toBe('/Users/me/.dungeon/uploads/s1/ab12-shot.png')
  })
  it('single-quotes a path with a space', () => {
    expect(quotePath('/Users/me/Library/Application Support/dungeon/x.png')).toBe("'/Users/me/Library/Application Support/dungeon/x.png'")
  })
  it('closes and reopens around a single quote and neutralises shell characters', () => {
    expect(quotePath("/tmp/it's $(x).png")).toBe(`'/tmp/it'\\''s $(x).png'`)
  })
  it('double-quotes a Windows path with a space but leaves a plain one bare', () => {
    expect(quotePath('C:\\Users\\me\\App Data\\x.png')).toBe('"C:\\Users\\me\\App Data\\x.png"')
    expect(quotePath('C:\\Users\\me\\x.png')).toBe('C:\\Users\\me\\x.png')
  })
})

it('joins several paths with a space', () => {
  expect(pathsText(['/a/b.png', '/c d/e.png'])).toBe("/a/b.png '/c d/e.png'")
})

describe('refusal', () => {
  it('accepts a normal file and refuses an empty or oversized one', () => {
    expect(refusal({ name: 'a.png', size: 10 })).toBeNull()
    expect(refusal({ name: 'a.png', size: MAX_UPLOAD_BYTES })).toBeNull()
    expect(refusal({ name: 'a.png', size: 0 })).toBe('a.png is empty')
    expect(refusal({ name: 'big.png', size: MAX_UPLOAD_BYTES + 1 })).toBe('big.png is larger than 10 MB')
  })
})

it('encodes bytes as base64, including a file bigger than one chunk', async () => {
  expect(await fileToBase64(new Blob([new Uint8Array([0, 255, 16])]))).toBe('AP8Q')
  const big = new Uint8Array(0x8000 * 2 + 5).fill(65)
  expect(atob(await fileToBase64(new Blob([big])))).toBe('A'.repeat(big.length))
})

it('reads files from a transfer and treats a missing or text-only one as empty', () => {
  const f = new File(['x'], 'x.png', { type: 'image/png' })
  expect(filesOf({ files: [f] } as unknown as DataTransfer)).toEqual([f])
  expect(filesOf(null)).toEqual([])
  expect(filesOf({ files: [] } as unknown as DataTransfer)).toEqual([])
})
