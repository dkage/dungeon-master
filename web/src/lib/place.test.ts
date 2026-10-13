import { describe, expect, it } from 'vitest'
import { format, parse } from './place'

describe('place', () => {
  it('round-trips identifiers', () => {
    const p = { campaign: 'c1', quest: 63, session: 's-2', view: 'map' as const, level: { kind: 'dungeon' as const, id: 7 } }
    expect(parse(`#${format(p)}`)).toEqual(p)
    expect(format({})).toBe('')
    expect(format({ level: { kind: 'world' } })).toBe('')
    expect(parse(format({ level: { kind: 'tavern' } }))).toEqual({ level: { kind: 'tavern' } })
  })
  it('drops malformed or hostile fields one by one', () => {
    expect(parse('#c=<script>&q=-3&s=a b&v=grid&l=zzz')).toEqual({})
    expect(parse('#c=ok&q=abc&token=secret')).toEqual({ campaign: 'ok' })
    expect(parse(null)).toEqual({})
    expect(parse('%%%')).toEqual({})
  })
})
