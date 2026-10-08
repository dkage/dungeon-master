import { describe, expect, it, vi } from 'vitest'
import { TermSocket, type SocketLike } from './termsocket'

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

function setup() {
  const sock = new FakeSocket()
  const h = { onOutput: vi.fn(), onExit: vi.fn(), onDisconnect: vi.fn(), onOpen: vi.fn() }
  const ts = new TermSocket('ws://x/y', h, () => sock)
  return { sock, h, ts }
}

describe('TermSocket', () => {
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

  it('encodes keystrokes as UTF-8 binary, only once open', () => {
    const { sock, ts } = setup()
    ts.sendInput('lost') // not open yet
    expect(sock.sent).toHaveLength(0)
    sock.open()
    ts.sendInput('é\r')
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

  it('reports a disconnect when closed without an exit frame', () => {
    const { sock, h } = setup()
    sock.onclose?.({ code: 1013, reason: 'too slow, reattach' })
    expect(h.onDisconnect).toHaveBeenCalledWith('too slow, reattach')
    const b = setup()
    b.sock.onclose?.({ code: 1006 })
    expect(b.h.onDisconnect).toHaveBeenCalledWith('closed (1006)')
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
