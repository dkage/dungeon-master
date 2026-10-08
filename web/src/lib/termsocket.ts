// Client side of the terminal websocket protocol:
//   server → client  binary: terminal output; text: {"type":"exit","code":N}
//   client → server  binary: keystrokes;      text: {"type":"resize","cols":N,"rows":N}

export interface SocketLike {
  binaryType: string
  readyState: number
  send(data: string | ArrayBufferLike | Uint8Array): void
  close(code?: number, reason?: string): void
  onopen: ((ev: unknown) => void) | null
  onmessage: ((ev: { data: unknown }) => void) | null
  onclose: ((ev: { code: number; reason?: string }) => void) | null
  onerror: ((ev: unknown) => void) | null
}

export interface TermHandlers {
  onOutput(data: Uint8Array): void
  onExit(code: number): void
  /** Closed without an exit frame: connection lost or server dropped us. */
  onDisconnect(reason: string): void
  onOpen?(): void
}

const OPEN = 1

export class TermSocket {
  private ws: SocketLike
  private exited = false
  private enc = new TextEncoder()

  constructor(
    url: string,
    private h: TermHandlers,
    factory: (url: string) => SocketLike = (u) => new WebSocket(u) as unknown as SocketLike,
  ) {
    this.ws = factory(url)
    this.ws.binaryType = 'arraybuffer'
    this.ws.onopen = () => this.h.onOpen?.()
    this.ws.onmessage = (ev) => this.message(ev.data)
    this.ws.onclose = (ev) => {
      if (!this.exited) this.h.onDisconnect(ev.reason || `closed (${ev.code})`)
    }
  }

  private message(data: unknown) {
    if (typeof data === 'string') {
      try {
        const m = JSON.parse(data) as { type?: string; code?: number }
        if (m.type === 'exit') {
          this.exited = true
          this.h.onExit(typeof m.code === 'number' ? m.code : -1)
        }
      } catch {
        /* ignore malformed control frames */
      }
      return
    }
    // Tag checks instead of instanceof: they hold across realms (iframes, jsdom).
    const tag = Object.prototype.toString.call(data)
    if (tag === '[object ArrayBuffer]') this.h.onOutput(new Uint8Array(data as ArrayBuffer))
    else if (ArrayBuffer.isView(data)) {
      const v = data as ArrayBufferView
      this.h.onOutput(new Uint8Array(v.buffer, v.byteOffset, v.byteLength))
    }
  }

  sendInput(text: string) {
    if (this.ws.readyState === OPEN) this.ws.send(this.enc.encode(text))
  }

  resize(cols: number, rows: number) {
    if (this.ws.readyState !== OPEN || cols < 1 || rows < 1) return
    this.ws.send(JSON.stringify({ type: 'resize', cols: Math.floor(cols), rows: Math.floor(rows) }))
  }

  close() {
    this.exited = true // closing on purpose is not a disconnect
    this.ws.close(1000, 'bye')
  }
}
