import { expect, it } from 'vitest'
import { SEARCH_DECORATIONS, matchLabel, scrolledUp } from './termSearch'

it('labels the empty, no-match, counted and uncounted states', () => {
  expect(matchLabel('', -1, 0)).toBe('')
  expect(matchLabel('x', -1, 0)).toBe('No matches')
  expect(matchLabel('x', 0, 12)).toBe('1 of 12')
  expect(matchLabel('x', 11, 12)).toBe('12 of 12')
  expect(matchLabel('x', -1, 1000)).toBe('1000+ matches')
})

it('knows when the view is above the newest output', () => {
  expect(scrolledUp({ viewportY: 0, baseY: 0 })).toBe(false)
  expect(scrolledUp({ viewportY: 40, baseY: 40 })).toBe(false)
  expect(scrolledUp({ viewportY: 10, baseY: 40 })).toBe(true)
})

it('uses plain six-digit hex colours, which the search addon requires', () => {
  for (const v of Object.values(SEARCH_DECORATIONS)) expect(v).toMatch(/^#[0-9a-f]{6}$/)
})
