// Client side of the terminal websocket protocol:
//   server → client  text: {"type":"attach","start":N,"resume":B} first; binary: terminal output;
//                    text: {"type":"exit","code":N}
//   client → server  binary: keystrokes;      text: {"type":"resize","cols":N,"rows":N}
// A connection that drops while the session may still run is reattached with a
// bounded backoff. The page tells the server how far into the output it got
// (?from=), and the server replays only the rest; when it cannot (the position
// fell out of its scrollback, or it is an older server) it replays everything
// and the caller clears its screen first (onReset).

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

/** Why the terminal stopped reconnecting, or what it is waiting out. */
export type Refusal = 'token' | 'gone' | 'lost' | 'unreachable'

export interface TermHandlers {
  onOutput(data: Uint8Array): void
  onExit(code: number): void
  /** Gave up reconnecting: retrying cannot help. */
  onDisconnect(reason: string, kind?: Refusal): void
  onOpen?(): void
  /** The connection dropped and a reattach is scheduled. */
  onReconnecting?(attempt: number, kind?: Refusal): void
  /** A reattach will replay everything, which repeats what is on screen: clear it first. */
  onReset?(): void
  /** Typed input could not be kept or sent: the session is gone or the wait queue is full. */
  onInputRefused?(reason: 'closed' | 'full'): void
}

export interface TermOptions {
  /** Delay before reconnect attempt n (1-based), in ms. */
  backoff?: (attempt: number) => number
  /** Asks the server about the session over plain HTTP; resolves to the status, rejects when unreachable. */
  probe?: (url: string) => Promise<number>
}

const OPEN = 1
/** Typing kept while the connection is down; more than this is refused rather than sent late in a heap. */
export const MAX_QUEUED_INPUT = 4096
const TRY_AGAIN_LATER = 1013 // the server dropped a viewer that fell behind a burst

export const defaultBackoff = (attempt: number) => Math.min(15000, 500 * 2 ** (attempt - 1))

const defaultProbe = async (url: string) => (await fetch(url, { credentials: 'same-origin' })).status

/** What the server's answer to a plain GET of the terminal URL means. */
export function classify(status: number): Refusal | undefined {
  if (status === 401 || status === 403) return 'token'
  if (status === 404) return 'gone'
  if (status === 410) return 'lost'
  return undefined // 426 (upgrade required) and anything else: the session is there, try again
}

export const REFUSAL_MESSAGE: Record<Refusal, string> = {
  token: 'The access token changed. Open the sign-in link printed by `dungeon serve`, then reload.',
  gone: 'This session no longer exists, probably because the app restarted. Start a new one.',
  lost: 'This session ended when the app was last stopped. Start a new one.',
  unreachable: 'The app cannot be reached.',
}

export class TermSocket {
  private ws!: SocketLike
  private exited = false
  private stopped = false
  private attempt = 0
  private opens = 0
  private awaitingAttach = false
  private pos = 0 // stream position after the last byte received
  private known = false // pos is meaningful: the server told us where the stream started
  private timer: ReturnType<typeof setTimeout> | undefined
  private enc = new TextEncoder()
  private queued: Uint8Array[] = []
  private queuedBytes = 0
  private backoff: (attempt: number) => number
  private probe: (url: string) => Promise<number>

  constructor(
    private url: string,
    private h: TermHandlers,
    private factory: (url: string) => SocketLike = (u) => new WebSocket(u) as unknown as SocketLike,
    opts: TermOptions = {},
  ) {
    this.backoff = opts.backoff ?? defaultBackoff
    this.probe = opts.probe ?? defaultProbe
    this.connect()
  }

  private connect() {
    const ws = this.factory(this.known ? `${this.url}?from=${this.pos}` : this.url)
    this.ws = ws
    ws.binaryType = 'arraybuffer'
    ws.onopen = () => {
      if (ws !== this.ws) return
      this.attempt = 0
      this.opens++
      this.awaitingAttach = true
      this.h.onOpen?.()
      this.flush()
    }
    ws.onmessage = (ev) => {
      if (ws === this.ws) this.message(ev.data)
    }
    ws.onclose = (ev) => {
      if (ws !== this.ws || this.exited || this.stopped) return
      void this.recover(ev.code)
    }
  }

  private async recover(code: number) {
    this.attempt++
    if (code === TRY_AGAIN_LATER) {
      // Fell behind a burst: the session is fine, reattach now and replay.
      this.h.onReconnecting?.(this.attempt)
      this.connect()
      return
    }
    let kind: Refusal | undefined
    try {
      kind = classify(await this.probe(this.url.replace(/^ws/, 'http')))
    } catch {
      kind = 'unreachable'
    }
    if (this.exited || this.stopped) return
    if (kind && kind !== 'unreachable') {
      this.stopped = true
      this.queued = []
      this.queuedBytes = 0
      this.h.onDisconnect(REFUSAL_MESSAGE[kind], kind)
      return
    }
    this.h.onReconnecting?.(this.attempt, kind)
    this.timer = setTimeout(() => {
      if (!this.exited && !this.stopped) this.connect()
    }, this.backoff(this.attempt))
  }

  private message(data: unknown) {
    if (typeof data === 'string') {
      try {
        const m = JSON.parse(data) as { type?: string; code?: number; start?: number; resume?: boolean }
        if (m.type === 'attach') {
          this.awaitingAttach = false
          if (this.opens > 1 && !m.resume) this.h.onReset?.()
          this.pos = typeof m.start === 'number' ? m.start : 0
          this.known = typeof m.start === 'number'
        } else if (m.type === 'exit') {
          this.exited = true
          this.h.onExit(typeof m.code === 'number' ? m.code : -1)
        }
      } catch {
        /* ignore malformed control frames */
      }
      return
    }
    if (this.awaitingAttach) {
      // Output before any attach frame: an older server, which always replays everything.
      this.awaitingAttach = false
      this.known = false
      if (this.opens > 1) this.h.onReset?.()
    }
    // Tag checks instead of instanceof: they hold across realms (iframes, jsdom).
    const tag = Object.prototype.toString.call(data)
    let bytes: Uint8Array | undefined
    if (tag === '[object ArrayBuffer]') bytes = new Uint8Array(data as ArrayBuffer)
    else if (ArrayBuffer.isView(data)) {
      const v = data as ArrayBufferView
      bytes = new Uint8Array(v.buffer, v.byteOffset, v.byteLength)
    }
    if (!bytes) return
    if (this.known) this.pos += bytes.length
    this.h.onOutput(bytes)
  }

  sendInput(text: string) {
    const bytes = this.enc.encode(text)
    if (this.ws.readyState === OPEN) {
      this.ws.send(bytes)
      return
    }
    // Not connected: keep a little for after the reattach, but never for a session that is gone.
    if (this.exited || this.stopped) return this.h.onInputRefused?.('closed')
    if (this.queuedBytes + bytes.length > MAX_QUEUED_INPUT) return this.h.onInputRefused?.('full')
    this.queued.push(bytes)
    this.queuedBytes += bytes.length
  }

  private flush() {
    const q = this.queued
    this.queued = []
    this.queuedBytes = 0
    for (const b of q) this.ws.send(b)
  }

  resize(cols: number, rows: number) {
    if (this.ws.readyState !== OPEN || cols < 1 || rows < 1) return
    this.ws.send(JSON.stringify({ type: 'resize', cols: Math.floor(cols), rows: Math.floor(rows) }))
  }

  close() {
    this.exited = true // closing on purpose is not a disconnect
    this.queued = []
    this.queuedBytes = 0
    clearTimeout(this.timer)
    this.ws.close(1000, 'bye')
  }
}
