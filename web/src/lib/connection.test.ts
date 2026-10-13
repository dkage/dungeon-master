import { beforeEach, describe, expect, it } from 'vitest'
import { connection } from './connection.svelte'

describe('connection', () => {
  beforeEach(() => connection.reset())

  it('is reachable until something fails', () => {
    expect(connection.unreachable).toBe(false)
    connection.reportApi(false)
    expect(connection.unreachable).toBe(true)
    connection.reportApi(true)
    expect(connection.unreachable).toBe(false)
  })

  it('stays unreachable until every terminal and the API have recovered', () => {
    connection.reportTerminal('a', false)
    connection.reportTerminal('b', false)
    connection.reportTerminal('a', true)
    expect(connection.unreachable).toBe(true)
    connection.reportTerminal('b', true)
    expect(connection.unreachable).toBe(false)
    connection.reportApi(false)
    connection.reportTerminal('a', true)
    expect(connection.unreachable).toBe(true)
  })
})
