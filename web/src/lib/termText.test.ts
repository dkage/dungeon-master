import { describe, expect, it } from 'vitest'
import { CUT_NOTICE, bufferText, outputFileName, stripControls, wasCut } from './termText'

const line = (text: string, wrapped = false) => ({ text, wrapped })

describe('bufferText', () => {
  it('joins lines and drops the blank rows after the last output', () => {
    expect(bufferText([line('one'), line('two'), line(''), line('  ')], false)).toBe('one\ntwo')
  })
  it('rejoins a wrapped line', () => {
    expect(bufferText([line('abcdef'), line('ghi', true), line('next')], false)).toBe('abcdefghi\nnext')
  })
  it('keeps blank lines inside the output', () => {
    expect(bufferText([line('a'), line(''), line('b')], false)).toBe('a\n\nb')
  })
  it('removes control characters but keeps tabs', () => {
    expect(bufferText([line('a\u001b[31mb\u0007\tc')], false)).toBe('a[31mb\tc')
    expect(stripControls('x\u0000y\u007fz\u009b')).toBe('xyz')
  })
  it('says at the top when older output was cut', () => {
    expect(bufferText([line('x')], true)).toBe(`${CUT_NOTICE}\nx`)
  })
  it('is empty for an empty buffer', () => {
    expect(bufferText([], false)).toBe('')
  })
})

it('reports a cut only when the buffer is full', () => {
  expect(wasCut(30, 30, 5000)).toBe(false)
  expect(wasCut(5029, 30, 5000)).toBe(false)
  expect(wasCut(5030, 30, 5000)).toBe(true)
})

it('names the file after the session without unsafe characters', () => {
  expect(outputFileName('abc-123')).toBe('session-abc-123-output.txt')
  expect(outputFileName('../a b')).toBe('session-ab-output.txt')
})
