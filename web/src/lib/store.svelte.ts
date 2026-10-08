import * as realApi from './api'
import type { AppState, Hero, SessionInfo } from './api'

export type Client = Pick<typeof realApi, 'getState' | 'createSession' | 'deleteSession'>

const DIR_KEY = 'dungeon.dir'

function loadDir(): string {
  try {
    return localStorage.getItem(DIR_KEY) ?? ''
  } catch {
    return ''
  }
}

function saveDir(dir: string) {
  try {
    localStorage.setItem(DIR_KEY, dir)
  } catch {
    /* storage unavailable: the field simply is not remembered */
  }
}

const absolute = /^(\/|[A-Za-z]:[\\/])/

export type Target = { hero: string } | { shell: true }

export class AppStore {
  version = $state('')
  claude = $state<AppState['claude']>({ found: true })
  defaultHeroes = $state(false)
  heroes = $state<Hero[]>([])
  sessions = $state<SessionInfo[]>([])
  activeId = $state<string | null>(null)
  error = $state<string | null>(null)
  busy = $state(false)
  dir = $state(loadDir())

  constructor(private client: Client = realApi) {}

  get active(): SessionInfo | undefined {
    return this.sessions.find((s) => s.id === this.activeId)
  }

  setDir(dir: string) {
    this.dir = dir
    saveDir(dir)
  }

  async refresh() {
    try {
      const st = await this.client.getState()
      this.version = st.version
      this.claude = st.claude
      this.defaultHeroes = st.defaultHeroes
      this.heroes = st.heroes
      this.sessions = st.sessions
      if (!this.sessions.some((s) => s.id === this.activeId)) {
        this.activeId = this.sessions.at(-1)?.id ?? null
      }
      this.error = null
    } catch (e) {
      this.error = message(e)
    }
  }

  async launch(target: Target) {
    const dir = this.dir.trim()
    if (!absolute.test(dir)) {
      this.error = 'Enter an absolute directory path to start in.'
      return
    }
    this.busy = true
    try {
      const sess = await this.client.createSession({ ...target, dir })
      this.sessions = [...this.sessions, sess]
      this.activeId = sess.id
      this.error = null
    } catch (e) {
      this.error = message(e)
    } finally {
      this.busy = false
    }
  }

  select(id: string) {
    if (this.sessions.some((s) => s.id === id)) this.activeId = id
  }

  async close(id: string) {
    try {
      await this.client.deleteSession(id)
    } catch (e) {
      // Already gone on the server is fine; anything else is worth showing.
      const status = (e as { status?: number }).status
      if (status !== 404) {
        this.error = message(e)
        return
      }
    }
    const i = this.sessions.findIndex((s) => s.id === id)
    this.sessions = this.sessions.filter((s) => s.id !== id)
    if (this.activeId === id) {
      this.activeId = this.sessions[Math.min(i, this.sessions.length - 1)]?.id ?? null
    }
  }

  markExited(id: string, code: number) {
    this.sessions = this.sessions.map((s) => (s.id === id ? { ...s, exited: true, exitCode: code } : s))
  }
}

function message(e: unknown): string {
  return e instanceof Error ? e.message : String(e)
}
