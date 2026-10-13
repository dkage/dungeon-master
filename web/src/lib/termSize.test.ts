import { expect, it } from 'vitest'
import { DEFAULT_TERMINAL_SIZE, MAX_SIZE, MIN_SIZE, clampSize, zoomed } from './termSize'

it('clamps to the readable range and rounds', () => {
  expect(clampSize(3, 13)).toBe(MIN_SIZE)
  expect(clampSize(99, 13)).toBe(MAX_SIZE)
  expect(clampSize(14.6, 13)).toBe(15)
})

it('falls back for anything that is not a finite number', () => {
  for (const bad of ['14', null, undefined, NaN, Infinity, {}, []]) expect(clampSize(bad, 13)).toBe(13)
})

it('zooms by one pixel, stops at both ends and resets to the default', () => {
  expect(zoomed(13, 'in')).toBe(14)
  expect(zoomed(13, 'out')).toBe(12)
  expect(zoomed(MAX_SIZE, 'in')).toBe(MAX_SIZE)
  expect(zoomed(MIN_SIZE, 'out')).toBe(MIN_SIZE)
  expect(zoomed(20, 'reset')).toBe(DEFAULT_TERMINAL_SIZE)
})
