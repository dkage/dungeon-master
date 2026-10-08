import * as realApi from './api'
import type { AppState, Campaign, CampaignInput, CampaignPatch, Hero, SessionInfo } from './api'

export type Client = Pick<
  typeof realApi,
  'getState' | 'createSession' | 'deleteSession' | 'addCampaign' | 'updateCampaign' | 'removeCampaign'
>

const CAMPAIGN_KEY = 'dungeon.campaign'

function load(key: string): string {
  try {
    return localStorage.getItem(key) ?? ''
  } catch {
    return ''
  }
}

function save(key: string, value: string) {
  try {
    localStorage.setItem(key, value)
  } catch {
    /* storage unavailable: the choice is simply not remembered */
  }
}

export type Target = { hero: string } | { shell: true }

export class AppStore {
  version = $state('')
  claude = $state<AppState['claude']>({ found: true })
  defaultHeroes = $state(false)
  heroes = $state<Hero[]>([])
  campaigns = $state<Campaign[]>([])
  sessions = $state<SessionInfo[]>([])
  campaignId = $state<string | null>(load(CAMPAIGN_KEY) || null)
  activeId = $state<string | null>(null)
  error = $state<string | null>(null)
  busy = $state(false)

  constructor(private client: Client = realApi) {}

  get campaign(): Campaign | undefined {
    return this.campaigns.find((c) => c.id === this.campaignId)
  }

  get active(): SessionInfo | undefined {
    return this.sessions.find((s) => s.id === this.activeId)
  }

  sessionsOf(campaignId: string): SessionInfo[] {
    return this.sessions.filter((s) => s.campaign === campaignId)
  }

  async refresh() {
    try {
      const st = await this.client.getState()
      this.version = st.version
      this.claude = st.claude
      this.defaultHeroes = st.defaultHeroes
      this.heroes = st.heroes
      this.campaigns = st.campaigns
      this.sessions = st.sessions
      this.normalise()
      this.error = null
    } catch (e) {
      this.error = message(e)
    }
  }

  /** Keep the selection pointing at things that exist. */
  private normalise() {
    if (!this.campaigns.some((c) => c.id === this.campaignId)) {
      this.pickCampaign(this.campaigns[0]?.id ?? null)
    }
    const mine = this.campaignId ? this.sessionsOf(this.campaignId) : []
    if (!mine.some((s) => s.id === this.activeId)) this.activeId = mine.at(-1)?.id ?? null
  }

  private pickCampaign(id: string | null) {
    this.campaignId = id
    save(CAMPAIGN_KEY, id ?? '')
  }

  selectCampaign(id: string) {
    if (!this.campaigns.some((c) => c.id === id)) return
    this.pickCampaign(id)
    if (this.active?.campaign !== id) this.activeId = this.sessionsOf(id).at(-1)?.id ?? null
  }

  selectSession(id: string) {
    const s = this.sessions.find((x) => x.id === id)
    if (!s) return
    this.pickCampaign(s.campaign)
    this.activeId = id
  }

  async addCampaign(input: CampaignInput): Promise<boolean> {
    this.busy = true
    try {
      const c = await this.client.addCampaign(input)
      this.campaigns = [...this.campaigns, c]
      this.selectCampaign(c.id)
      this.error = null
      return true
    } catch (e) {
      this.error = message(e)
      return false
    } finally {
      this.busy = false
    }
  }

  async updateCampaign(id: string, patch: CampaignPatch): Promise<boolean> {
    try {
      const c = await this.client.updateCampaign(id, patch)
      this.campaigns = this.campaigns.map((x) => (x.id === id ? c : x))
      this.error = null
      return true
    } catch (e) {
      this.error = message(e)
      return false
    }
  }

  async removeCampaign(id: string) {
    try {
      await this.client.removeCampaign(id)
    } catch (e) {
      const status = (e as { status?: number }).status
      if (status !== 404) {
        this.error = message(e)
        return
      }
    }
    this.campaigns = this.campaigns.filter((c) => c.id !== id)
    this.sessions = this.sessions.filter((s) => s.campaign !== id)
    this.error = null
    this.normalise()
  }

  async launch(target: Target) {
    const campaign = this.campaign
    if (!campaign) {
      this.error = 'Add or select a campaign first.'
      return
    }
    this.busy = true
    try {
      const sess = await this.client.createSession({ ...target, campaign: campaign.id })
      this.sessions = [...this.sessions, sess]
      this.activeId = sess.id
      this.error = null
    } catch (e) {
      this.error = message(e)
    } finally {
      this.busy = false
    }
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
    const campaign = this.sessions.find((s) => s.id === id)?.campaign
    const before = campaign ? this.sessionsOf(campaign) : []
    const i = before.findIndex((s) => s.id === id)
    this.sessions = this.sessions.filter((s) => s.id !== id)
    if (this.activeId === id) {
      const after = campaign ? this.sessionsOf(campaign) : []
      this.activeId = after[Math.min(i, after.length - 1)]?.id ?? null
    }
  }

  markExited(id: string, code: number) {
    this.sessions = this.sessions.map((s) => (s.id === id ? { ...s, exited: true, exitCode: code } : s))
  }
}

function message(e: unknown): string {
  return e instanceof Error ? e.message : String(e)
}
