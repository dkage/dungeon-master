import { describe, expect, it, vi } from 'vitest'
import { MAX_QUEUED_INPUT, TermSocket, classify, defaultBackoff, type SocketLike } from './termsocket'

class FakeSocket implements SocketLike {
  binaryType = ''
  readyState = 0
  sent: unknown[] = []
  closed?: { code?: number; reason?: string }
  onopen: SocketLike['onopen'] = null
  onmessage: SocketLike['onmessage'] = null
  onclose: SocketLike['onclose'] = null
  onerror: SocketLike['onerror'] = null
  send(d: unknown) {
    this.sent.push(d)
  }
  drop(code: number) {
    this.readyState = 3
    this.onclose?.({ code })
  }
  close(code?: number, reason?: string) {
    this.closed = { code, reason }
  }
  open() {
    this.readyState = 1
    this.onopen?.({})
  }
  msg(data: unknown) {
    this.onmessage?.({ data })
  }
}

function setup(probe: (url: string) => Promise<number> = () => Promise.resolve(426)) {
  const socks: FakeSocket[] = []
  const urls: string[] = []
  const factory = (url: string) => {
    urls.push(url)
    const s = new FakeSocket()
    socks.push(s)
    return s
  }
  const h = {
    onOutput: vi.fn(),
    onExit: vi.fn(),
    onDisconnect: vi.fn(),
    onOpen: vi.fn(),
    onReconnecting: vi.fn(),
    onReset: vi.fn(),
    onInputRefused: vi.fn(),
  }
  const ts = new TermSocket('ws://x/y', h, factory, { backoff: () => 10, probe })
  return { sock: socks[0], socks, urls, h, ts }
}

const attach = (start: number, resume: boolean) => JSON.stringify({ type: 'attach', start, resume })
const bytes = (n: number) => new Uint8Array(n).buffer

describe('TermSocket', () => {
  it('asks to resume from the position it reached and keeps the screen when the server does', async () => {
    vi.useFakeTimers()
    const { sock, socks, urls, h } = setup()
    sock.open()
    expect(urls[0]).toBe('ws://x/y') // nothing known yet
    sock.msg(attach(100, false))
    sock.msg(bytes(30))
    sock.msg(bytes(12))
    sock.drop(1006)
    await vi.advanceTimersByTimeAsync(10)
    expect(urls[1]).toBe('ws://x/y?from=142')
    socks[1].open()
    socks[1].msg(attach(142, true))
    socks[1].msg(bytes(8))
    expect(h.onReset).not.toHaveBeenCalled()
    expect(h.onOutput).toHaveBeenCalledTimes(3)
    socks[1].drop(1006)
    await vi.advanceTimersByTimeAsync(10)
    expect(urls[2]).toBe('ws://x/y?from=150') // the resumed bytes were counted too
    vi.useRealTimers()
  })

  it('clears the screen when output arrives with no attach frame (an older server)', async () => {
    vi.useFakeTimers()
    const { sock, socks, urls, h } = setup()
    sock.open()
    sock.msg(bytes(5)) // first connection: nothing to clear
    expect(h.onReset).not.toHaveBeenCalled()
    sock.drop(1006)
    await vi.advanceTimersByTimeAsync(10)
    expect(urls[1]).toBe('ws://x/y') // never told a position, so it asks for everything
    socks[1].open()
    socks[1].msg(bytes(5))
    expect(h.onReset).toHaveBeenCalledTimes(1)
    vi.useRealTimers()
  })

  it('requests arraybuffers and reports open', () => {
    const { sock, h } = setup()
    expect(sock.binaryType).toBe('arraybuffer')
    sock.open()
    expect(h.onOpen).toHaveBeenCalled()
  })

  it('delivers binary frames as bytes', () => {
    const { sock, h } = setup()
    sock.msg(new TextEncoder().encode('hi').buffer)
    expect(new TextDecoder().decode(h.onOutput.mock.calls[0][0])).toBe('hi')
  })

  it('encodes keystrokes as UTF-8 binary, holding them until open', () => {
    const { sock, ts } = setup()
    ts.sendInput('é\r') // not open yet: kept, then sent first
    expect(sock.sent).toHaveLength(0)
    sock.open()
    expect(Array.from(sock.sent[0] as Uint8Array)).toEqual([0xc3, 0xa9, 0x0d])
  })

  it('sends resize as a JSON text frame, ignoring nonsense sizes', () => {
    const { sock, ts } = setup()
    sock.open()
    ts.resize(100.9, 40)
    ts.resize(0, 10)
    ts.resize(10, -1)
    expect(sock.sent).toEqual([JSON.stringify({ type: 'resize', cols: 100, rows: 40 })])
  })

  it('reports an exit frame and does not then report a disconnect', () => {
    const { sock, h } = setup()
    sock.msg(JSON.stringify({ type: 'exit', code: 7 }))
    sock.onclose?.({ code: 1000, reason: 'exited' })
    expect(h.onExit).toHaveBeenCalledWith(7)
    expect(h.onDisconnect).not.toHaveBeenCalled()
  })

  it('reattaches after an abnormal close and clears the screen before the replay', async () => {
    vi.useFakeTimers()
    const { sock, socks, h } = setup()
    sock.open()
    sock.onclose?.({ code: 1006 })
    await vi.advanceTimersByTimeAsync(0)
    expect(h.onReconnecting).toHaveBeenCalledWith(1, undefined)
    expect(h.onDisconnect).not.toHaveBeenCalled()
    await vi.advanceTimersByTimeAsync(10)
    expect(socks).toHaveLength(2)
    socks[1].open()
    expect(h.onOpen).toHaveBeenCalledTimes(2)
    expect(h.onReset).not.toHaveBeenCalled() // decided by the attach frame
    socks[1].msg(attach(0, false)) // the server could not resume: it replays everything
    expect(h.onReset).toHaveBeenCalledTimes(1)
    socks[1].msg(new TextEncoder().encode('replay').buffer)
    expect(h.onOutput).toHaveBeenCalledTimes(1)
    vi.useRealTimers()
  })

  it('does not reset on the first connection and routes input to the live socket', async () => {
    vi.useFakeTimers()
    const { sock, socks, h, ts } = setup()
    sock.open()
    expect(h.onReset).not.toHaveBeenCalled()
    sock.onclose?.({ code: 1006 })
    await vi.advanceTimersByTimeAsync(10)
    socks[1].open()
    ts.sendInput('a')
    expect(sock.sent).toHaveLength(0)
    expect(socks[1].sent).toHaveLength(1)
    vi.useRealTimers()
  })

  it('reattaches at once when dropped for being slow, without asking the server first', async () => {
    const probe = vi.fn()
    const { sock, socks, h } = setup(probe)
    sock.open()
    sock.onclose?.({ code: 1013, reason: 'too slow, reattach' })
    expect(socks).toHaveLength(2)
    expect(probe).not.toHaveBeenCalled()
    expect(h.onDisconnect).not.toHaveBeenCalled()
  })

  it.each([
    [401, 'token'],
    [404, 'gone'],
    [410, 'lost'],
  ])('stops retrying when the server answers %i', async (status, kind) => {
    const { sock, socks, h } = setup(() => Promise.resolve(status))
    sock.open()
    sock.onclose?.({ code: 1006 })
    await new Promise((r) => setTimeout(r, 30))
    expect(h.onDisconnect).toHaveBeenCalledTimes(1)
    expect(h.onDisconnect.mock.calls[0][1]).toBe(kind)
    expect(socks).toHaveLength(1)
  })

  it('keeps retrying with backoff while the server is unreachable, then recovers', async () => {
    vi.useFakeTimers()
    let up = false
    const { sock, socks, h } = setup(() => (up ? Promise.resolve(426) : Promise.reject(new Error('down'))))
    sock.open()
    sock.onclose?.({ code: 1006 })
    await vi.advanceTimersByTimeAsync(0)
    expect(h.onReconnecting).toHaveBeenLastCalledWith(1, 'unreachable')
    await vi.advanceTimersByTimeAsync(10)
    socks[1].onclose?.({ code: 1006 })
    await vi.advanceTimersByTimeAsync(0)
    expect(h.onReconnecting).toHaveBeenLastCalledWith(2, 'unreachable')
    up = true
    await vi.advanceTimersByTimeAsync(10)
    socks[2].open()
    socks[2].onclose?.({ code: 1006 })
    await vi.advanceTimersByTimeAsync(0)
    expect(h.onReconnecting).toHaveBeenLastCalledWith(1, undefined) // a good open resets the count
    expect(h.onDisconnect).not.toHaveBeenCalled()
    vi.useRealTimers()
  })

  it('does not reattach after the session exited or the page closed it', async () => {
    const a = setup()
    a.sock.msg(JSON.stringify({ type: 'exit', code: 0 }))
    a.sock.onclose?.({ code: 1000 })
    const b = setup()
    b.ts.close()
    b.sock.onclose?.({ code: 1000 })
    await new Promise((r) => setTimeout(r, 30))
    expect(a.socks).toHaveLength(1)
    expect(b.socks).toHaveLength(1)
    expect(a.h.onReconnecting).not.toHaveBeenCalled()
  })

  it('cancels a pending reattach when closed', async () => {
    vi.useFakeTimers()
    const { sock, socks, ts } = setup()
    sock.onclose?.({ code: 1006 })
    await vi.advanceTimersByTimeAsync(0)
    ts.close()
    await vi.advanceTimersByTimeAsync(100)
    expect(socks).toHaveLength(1)
    vi.useRealTimers()
  })

  it('keeps typing made during a reconnect and sends it, in order, after reattaching', async () => {
    vi.useFakeTimers()
    const { sock, socks, h, ts } = setup()
    sock.open()
    sock.drop(1006)
    ts.sendInput('ab')
    ts.sendInput('c')
    expect(sock.sent).toHaveLength(0)
    await vi.advanceTimersByTimeAsync(10)
    socks[1].open()
    expect(socks[1].sent.map((b) => new TextDecoder().decode(b as Uint8Array))).toEqual(['ab', 'c'])
    ts.sendInput('d')
    expect(socks[1].sent).toHaveLength(3)
    expect(h.onInputRefused).not.toHaveBeenCalled()
    vi.useRealTimers()
  })

  it('refuses input beyond the queue bound and tells the page', async () => {
    const { sock, h, ts } = setup()
    sock.drop(1006)
    ts.sendInput('x'.repeat(MAX_QUEUED_INPUT))
    expect(h.onInputRefused).not.toHaveBeenCalled()
    ts.sendInput('y')
    expect(h.onInputRefused).toHaveBeenCalledWith('full')
  })

  it('refuses input once reconnecting has stopped or the session ended, and sends nothing queued', async () => {
    const a = setup(() => Promise.resolve(404))
    a.sock.open()
    a.sock.drop(1006)
    a.ts.sendInput('kept?')
    await new Promise((r) => setTimeout(r, 30))
    a.ts.sendInput('late')
    expect(a.h.onInputRefused).toHaveBeenCalledWith('closed')
    expect(a.socks).toHaveLength(1)
    expect(a.sock.sent).toHaveLength(0)
    const b = setup()
    b.sock.msg(JSON.stringify({ type: 'exit', code: 0 }))
    b.ts.sendInput('after exit')
    expect(b.h.onInputRefused).toHaveBeenCalledWith('closed')
  })

  it('classifies the probe answers and doubles the backoff up to a cap', () => {
    expect(classify(426)).toBeUndefined()
    expect(classify(500)).toBeUndefined()
    expect([1, 2, 3, 10].map(defaultBackoff)).toEqual([500, 1000, 2000, 15000])
  })

  it('ignores malformed or unknown control frames', () => {
    const { sock, h } = setup()
    sock.msg('not json')
    sock.msg(JSON.stringify({ type: 'other' }))
    expect(h.onExit).not.toHaveBeenCalled()
    sock.msg(JSON.stringify({ type: 'exit' }))
    expect(h.onExit).toHaveBeenCalledWith(-1)
  })

  it('closing on purpose is not a disconnect', () => {
    const { sock, h, ts } = setup()
    ts.close()
    sock.onclose?.({ code: 1000 })
    expect(sock.closed).toEqual({ code: 1000, reason: 'bye' })
    expect(h.onDisconnect).not.toHaveBeenCalled()
  })
})
