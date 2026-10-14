import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { Heartbeat } from './heartbeat'

describe('Heartbeat', () => {
  beforeEach(() => vi.useFakeTimers())
  afterEach(() => vi.useRealTimers())

  it('pings at an even pace while the backend answers', async () => {
    const ping = vi.fn().mockResolvedValue(true)
    const h = new Heartbeat({ ping, visible: () => true, every: 100 })
    await vi.advanceTimersByTimeAsync(350)
    expect(ping).toHaveBeenCalledTimes(3)
    h.stop()
  })

  it('asks nothing while hidden and checks at once when woken', async () => {
    let visible = false
    const ping = vi.fn().mockResolvedValue(true)
    const h = new Heartbeat({ ping, visible: () => visible, every: 100 })
    await vi.advanceTimersByTimeAsync(1000)
    expect(ping).not.toHaveBeenCalled()
    h.wake() // still hidden: nothing
    expect(ping).not.toHaveBeenCalled()
    visible = true
    h.wake()
    expect(ping).toHaveBeenCalledTimes(1)
    h.stop()
  })

  it('backs off while the backend is down and resets after one answer', async () => {
    let up = false
    const ping = vi.fn(() => Promise.resolve(up))
    const h = new Heartbeat({ ping, visible: () => true, every: 100, maxEvery: 400 })
    await vi.advanceTimersByTimeAsync(100) // 1st, fails: next in 200
    await vi.advanceTimersByTimeAsync(200) // 2nd, fails: next in 400
    await vi.advanceTimersByTimeAsync(400) // 3rd, fails: capped at 400
    await vi.advanceTimersByTimeAsync(400) // 4th
    expect(ping).toHaveBeenCalledTimes(4)
    up = true
    await vi.advanceTimersByTimeAsync(400) // 5th answers: back to 100
    await vi.advanceTimersByTimeAsync(100)
    expect(ping).toHaveBeenCalledTimes(6)
    h.stop()
  })

  it('does nothing after stop', async () => {
    const ping = vi.fn().mockResolvedValue(true)
    const h = new Heartbeat({ ping, visible: () => true, every: 100 })
    h.stop()
    h.wake()
    await vi.advanceTimersByTimeAsync(500)
    expect(ping).not.toHaveBeenCalled()
  })
})
