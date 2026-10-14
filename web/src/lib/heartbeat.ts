// Asks the backend whether it answers: at an even pace while it does, slower
// while it does not, and not at all while the tab is hidden.

export interface HeartbeatOptions {
  ping: () => Promise<boolean>
  visible: () => boolean
  every?: number
  maxEvery?: number
}

export class Heartbeat {
  private timer: ReturnType<typeof setTimeout> | undefined
  private delay: number
  private stopped = false
  private every: number
  private maxEvery: number

  constructor(private o: HeartbeatOptions) {
    this.every = o.every ?? 4000
    this.maxEvery = o.maxEvery ?? 30000
    this.delay = this.every
    this.schedule()
  }

  /** Call when the tab is shown again: check at once instead of waiting out the delay. */
  wake() {
    if (this.stopped || !this.o.visible()) return
    clearTimeout(this.timer)
    void this.tick()
  }

  stop() {
    this.stopped = true
    clearTimeout(this.timer)
  }

  private schedule() {
    clearTimeout(this.timer)
    if (!this.stopped) this.timer = setTimeout(() => void this.tick(), this.delay)
  }

  private async tick() {
    if (this.stopped) return
    if (!this.o.visible()) {
      this.schedule() // hidden: ask nothing, look again later
      return
    }
    const ok = await this.o.ping()
    this.delay = ok ? this.every : Math.min(this.maxEvery, this.delay * 2)
    this.schedule()
  }
}
