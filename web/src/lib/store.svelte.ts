import * as realApi from './api'
import type {
  AppState, Campaign, CampaignInput, CampaignPatch, GithubStatus, Hero, HeroesConfig, LaunchPreview, ProjectSuggestion, QuestDetail, QuestResult, Scope, ScopeOptions,
  SessionInfo,
} from './api'
import { stillWaiting, unattended } from './attention'

export type Client = Pick<
  typeof realApi,
  | 'getState' | 'getSessions' | 'createSession' | 'deleteSession' | 'addCampaign' | 'updateCampaign' | 'removeCampaign'
  | 'getQuests' | 'getQuest' | 'previewQuestLaunch' | 'getGithub' | 'setScope' | 'getScopeOptions' | 'getProjectSuggestions' | 'createSampleCampaign'
  | 'adoptStarterHeroes' | 'getHeroesConfig' | 'saveHeroesConfig' | 'pickFolder' | 'openCampaignFolder'
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

export interface Failure {
  /** The server's machine-readable code, or "error". */
  code: string
  message: string
}

export type ReadingFont = 'almendra' | 'eb-garamond' | 'alegreya'
export type WordMode = 'hints' | 'themed' | 'plain'
export type ThemePack = 'dungeon' | 'neutral'
export interface UISettings {
  font: ReadingFont
  sidebarSize: number
  terminalSize: number
  questsSize: number
  wordMode: WordMode
  theme: ThemePack
}

const SETTINGS_KEY = 'dungeon.appearance'
const DEFAULT_SETTINGS: UISettings = {
  font: 'almendra', sidebarSize: 14, terminalSize: 13, questsSize: 14, wordMode: 'hints', theme: 'dungeon',
}

function loadSettings(): UISettings {
  try {
    const raw = localStorage.getItem(SETTINGS_KEY)
    if (!raw) return { ...DEFAULT_SETTINGS }
    const parsed = JSON.parse(raw) as Partial<UISettings>
    const allowed = <T extends string>(value: unknown, values: readonly T[], fallback: T): T =>
      values.includes(value as T) ? value as T : fallback
    const size = (value: unknown, fallback: number) =>
      typeof value === 'number' && Number.isFinite(value) ? Math.max(10, Math.min(24, Math.round(value))) : fallback
    return {
      font: allowed(parsed.font, ['almendra', 'eb-garamond', 'alegreya'], DEFAULT_SETTINGS.font),
      sidebarSize: size(parsed.sidebarSize, DEFAULT_SETTINGS.sidebarSize),
      terminalSize: size(parsed.terminalSize, DEFAULT_SETTINGS.terminalSize),
      questsSize: size(parsed.questsSize, DEFAULT_SETTINGS.questsSize),
      wordMode: allowed(parsed.wordMode, ['hints', 'themed', 'plain'], DEFAULT_SETTINGS.wordMode),
      theme: allowed(parsed.theme, ['dungeon', 'neutral'], DEFAULT_SETTINGS.theme),
    }
  } catch {
    return { ...DEFAULT_SETTINGS }
  }
}

function failure(e: unknown): Failure {
  return { code: (e as { code?: string }).code ?? 'error', message: message(e) }
}

export class AppStore {
  version = $state('')
  claude = $state<AppState['claude']>({ found: true })
  defaultHeroes = $state(false)
  defaultHero = $state('')
  partyError = $state('')
  heroes = $state<Hero[]>([])
  campaigns = $state<Campaign[]>([])
  sessions = $state<SessionInfo[]>([])
  campaignId = $state<string | null>(load(CAMPAIGN_KEY) || null)
  activeId = $state<string | null>(null)
  /** Sessions whose current wait the user has already looked at. */
  seen = $state<Set<string>>(new Set())
  error = $state<string | null>(null)
  busy = $state(false)

  // Quests of the selected campaign.
  quests = $state<QuestResult | null>(null)
  questsFor = $state<string | null>(null)
  questsError = $state<Failure | null>(null)
  questsLoading = $state(false)
  questNumber = $state<number | null>(null)
  detail = $state<QuestDetail | null>(null)
  detailError = $state<Failure | null>(null)
  detailLoading = $state(false)
  /** What each scope choice would show, for the first-connection question. */
  scopeOptions = $state<ScopeOptions | null>(null)
  scopeOptionsError = $state<Failure | null>(null)
  scopeOptionsLoading = $state(false)
  /** The user opened the scope picker to change an existing choice. */
  pickingScope = $state(false)
  /** The GitHub request budget and whether requests are paused. */
  github = $state<GithubStatus | null>(null)
  /** Whether the quest panel is open on narrow screens. */
  questsOpen = $state(false)
  /**
   * The server refused this tab's credentials (it was restarted with another
   * token, or this browser never opened the signed-in link). Shown as its own
   * screen: an empty list would look like lost data.
   */
  unauthorised = $state(false)
  suggestions = $state<ProjectSuggestion[]>([])
  suggestionsLoading = $state(false)
  firstRunDismissed = $state(load('dungeon.first-run.dismissed') === '1')
  appearance = $state<UISettings>(loadSettings())

  // Responses that arrive after a newer request, or after the selection moved
  // on, are dropped so one campaign's quests never show under another.
  private questsSeq = 0
  private detailSeq = 0

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

  questSessionForCampaign(campaignId: string): SessionInfo | undefined {
    const campaign = this.campaigns.find((c) => c.id === campaignId)
    if (!campaign) return
    const tree = campaign.gitRoot || campaign.dir
    return this.sessions.find((s) => {
      if (!s.quest || s.exited || s.lost) return false
      const other = this.campaigns.find((c) => c.id === s.campaign)
      return (other?.gitRoot || other?.dir) === tree
    })
  }

  async goToSession(id: string) {
    if (!this.sessions.some((s) => s.id === id)) await this.refresh()
    await this.selectSession(id)
  }

  private fail(e: unknown): string {
    if ((e as { status?: number }).status === 401) this.unauthorised = true
    return message(e)
  }

  private noticeAuth(f: Failure, e: unknown): Failure {
    if ((e as { status?: number }).status === 401) this.unauthorised = true
    return f
  }

  async refresh() {
    try {
      const st = await this.client.getState()
      this.version = st.version
      this.claude = st.claude
      this.defaultHeroes = st.defaultHeroes
      this.defaultHero = st.defaultHero ?? ''
      this.partyError = st.partyError ?? ''
      this.heroes = st.heroes
      this.campaigns = st.campaigns
      this.sessions = st.sessions
      this.unauthorised = false
      this.normalise()
      this.error = null
      if (this.campaignId && this.questsFor !== this.campaignId) void this.loadQuests()
      if (this.campaigns.length === 0) void this.loadSuggestions()
    } catch (e) {
      this.error = this.fail(e)
    }
  }

  async loadSuggestions() {
    this.suggestionsLoading = true
    try {
      this.suggestions = await this.client.getProjectSuggestions()
    } catch {
      this.suggestions = []
    } finally {
      this.suggestionsLoading = false
    }
  }

  async addSuggestedProject(suggestion: ProjectSuggestion): Promise<boolean> {
    return this.addCampaign({ dir: suggestion.dir, name: suggestion.name })
  }

  async addSampleCampaign(): Promise<boolean> {
    this.busy = true
    try {
      const sample = await this.client.createSampleCampaign()
      if (!this.campaigns.some((c) => c.id === sample.id)) this.campaigns = [...this.campaigns, sample]
      this.selectCampaign(sample.id)
      this.error = null
      return true
    } catch (e) {
      this.error = this.fail(e)
      return false
    } finally {
      this.busy = false
    }
  }

  async pickFolder(): Promise<string | null> {
    try {
      const result = await this.client.pickFolder()
      return result.path ?? null
    } catch (e) {
      this.error = this.fail(e)
      return null
    }
  }

  async openCampaignFolder(id: string) {
    try {
      await this.client.openCampaignFolder(id)
      this.error = null
    } catch (e) {
      this.error = this.fail(e)
    }
  }

  changeCampaignScope(id: string) {
    if (!this.campaigns.some((c) => c.id === id)) return
    this.selectCampaign(id)
    this.questsOpen = true
    this.pickingScope = true
    void this.loadScopeOptions()
  }

  async adoptStarterHeroes(): Promise<boolean> {
    this.busy = true
    try {
      await this.client.adoptStarterHeroes()
      await this.refresh()
      return true
    } catch (e) {
      this.error = this.fail(e)
      return false
    } finally {
      this.busy = false
    }
  }

  loadHeroesConfig(): Promise<HeroesConfig> {
    return this.refresh().then(() => this.client.getHeroesConfig())
  }

  async saveHeroesConfig(config: HeroesConfig): Promise<boolean> {
    try {
      await this.client.saveHeroesConfig(config)
      await this.refresh()
      return true
    } catch (e) {
      this.error = this.fail(e)
      return false
    }
  }

  dismissFirstRun() {
    this.firstRunDismissed = true
    save('dungeon.first-run.dismissed', '1')
  }

  openFirstRun() {
    this.firstRunDismissed = false
    save('dungeon.first-run.dismissed', '0')
  }

  updateAppearance(patch: Partial<UISettings>) {
    this.appearance = { ...this.appearance, ...patch }
    save(SETTINGS_KEY, JSON.stringify(this.appearance))
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
    if (id !== this.campaignId) {
      this.pickingScope = false
      this.scopeOptions = null
      this.scopeOptionsError = null
    }
    this.campaignId = id
    save(CAMPAIGN_KEY, id ?? '')
  }

  // --- quests ---

  /**
   * Loads the selected campaign's quests; refresh bypasses the server cache.
   * A background load is polling nobody asked for: it does not show a spinner,
   * keeps what is on screen if it fails, and tells the server it is
   * low-priority so it leaves more of the GitHub budget in reserve.
   */
  async loadQuests(refresh = false, background = false) {
    const id = this.campaignId
    const seq = ++this.questsSeq
    if (!id) {
      this.quests = null
      this.questsFor = null
      this.questsError = null
      this.questsLoading = false
      this.closeQuest()
      return
    }
    if (this.questsFor !== id) {
      // A different campaign: drop what belongs to the previous one now.
      this.quests = null
      this.questsError = null
      this.closeQuest()
    }
    this.questsFor = id
    if (!background) this.questsLoading = true
    try {
      const res = await this.client.getQuests(id, refresh, background)
      if (seq !== this.questsSeq) return
      this.quests = res
      this.questsError = null
    } catch (e) {
      if (seq !== this.questsSeq) return
      const f = this.noticeAuth(failure(e), e)
      if (!background) {
        this.questsError = f
        this.quests = null
      }
    } finally {
      if (seq === this.questsSeq && !background) this.questsLoading = false
      if (seq === this.questsSeq) void this.loadGithub()
    }
  }

  async loadScopeOptions() {
    const id = this.campaignId
    if (!id) return
    this.scopeOptionsLoading = true
    this.scopeOptionsError = null
    try {
      const o = await this.client.getScopeOptions(id)
      if (id === this.campaignId) this.scopeOptions = o
    } catch (e) {
      if (id === this.campaignId) this.scopeOptionsError = this.noticeAuth(failure(e), e)
    } finally {
      if (id === this.campaignId) this.scopeOptionsLoading = false
    }
  }

  /** Stores the chosen scope for the selected campaign and reloads its quests. */
  async chooseScope(scope: Scope): Promise<boolean> {
    const id = this.campaignId
    if (!id) return false
    try {
      const c = await this.client.setScope(id, scope)
      this.campaigns = this.campaigns.map((x) => (x.id === id ? c : x))
      this.pickingScope = false
      this.error = null
      this.closeQuest()
      this.quests = null
      this.questsError = null
      this.questsFor = null
      await this.loadQuests()
      return true
    } catch (e) {
      this.error = this.fail(e)
      return false
    }
  }

  /** Reads the GitHub budget; a failure just leaves the last known value. */
  async loadGithub() {
    try {
      this.github = await this.client.getGithub()
    } catch (e) {
      this.noticeAuth(failure(e), e)
    }
  }

  async selectQuest(number: number) {
    const id = this.campaignId
    if (!id) return
    const seq = ++this.detailSeq
    this.questNumber = number
    this.detail = null
    this.detailError = null
    this.detailLoading = true
    try {
      const d = await this.client.getQuest(id, number)
      if (seq !== this.detailSeq) return
      this.detail = d
    } catch (e) {
      if (seq !== this.detailSeq) return
      this.detailError = this.noticeAuth(failure(e), e)
    } finally {
      if (seq === this.detailSeq) this.detailLoading = false
    }
  }

  closeQuest() {
    this.detailSeq++
    this.questNumber = null
    this.detail = null
    this.detailError = null
    this.detailLoading = false
  }

  /** Switches the selected campaign between sample quests and real issues. */
  async setDemo(on: boolean): Promise<boolean> {
    const id = this.campaignId
    if (!id) return false
    const ok = await this.updateCampaign(id, { demo: on })
    if (ok) {
      this.closeQuest()
      this.quests = null
      this.questsFor = null
      await this.loadQuests()
    }
    return ok
  }

  selectCampaign(id: string) {
    if (!this.campaigns.some((c) => c.id === id)) return
    const changed = id !== this.campaignId
    this.pickCampaign(id)
    if (this.active?.campaign !== id) this.activeId = this.sessionsOf(id).at(-1)?.id ?? null
    if (changed) void this.loadQuests()
  }

  async selectSession(id: string) {
    const s = this.sessions.find((x) => x.id === id)
    if (!s) return
    const changed = s.campaign !== this.campaignId
    this.pickCampaign(s.campaign)
    this.activeId = id
    if (changed) void this.loadQuests()
    const session = this.sessions.find((x) => x.id === id)
    if (session?.quest) {
      this.questsOpen = true
      await this.selectQuest(session.quest)
    }
  }

  async addCampaign(input: CampaignInput): Promise<boolean> {
    this.busy = true
    try {
      const c = await this.client.addCampaign(input)
      if (!this.campaigns.some((x) => x.id === c.id)) this.campaigns = [...this.campaigns, c]
      this.selectCampaign(c.id)
      this.error = null
      return true
    } catch (e) {
      this.error = this.fail(e)
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
      this.error = this.fail(e)
      return false
    }
  }

  async removeCampaign(id: string) {
    try {
      await this.client.removeCampaign(id)
    } catch (e) {
      const status = (e as { status?: number }).status
      if (status !== 404) {
        this.error = this.fail(e)
        return
      }
    }
    this.campaigns = this.campaigns.filter((c) => c.id !== id)
    this.sessions = this.sessions.filter((s) => s.campaign !== id)
    this.error = null
    this.normalise()
    if (this.questsFor === id || this.questsFor !== this.campaignId) void this.loadQuests()
  }

  async launch(target: Target) {
    await this.launchInCampaign(this.campaignId, target)
  }

  async launchInCampaign(campaignId: string | null, target: Target) {
    const campaign = this.campaigns.find((c) => c.id === campaignId)
    if (!campaign) {
      this.error = 'Add or select a campaign first.'
      return
    }
    this.busy = true
    try {
      const sess = await this.client.createSession({ ...target, campaign: campaign.id })
      this.sessions = [...this.sessions, sess]
      this.selectCampaign(campaign.id)
      this.activeId = sess.id
      this.error = null
    } catch (e) {
      this.error = this.fail(e)
    } finally {
      this.busy = false
    }
  }

  async previewQuestLaunch(number: number, hero: string): Promise<LaunchPreview | null> {
    const campaign = this.campaign
    if (!campaign) return null
    try {
      return await this.client.previewQuestLaunch(campaign.id, number, hero)
    } catch (e) {
      this.error = this.fail(e)
      return null
    }
  }

  async launchQuest(number: number, hero: string, briefingId?: string): Promise<boolean> {
    const campaign = this.campaign
    if (!campaign) {
      this.error = 'Add or select a campaign first.'
      return false
    }
    this.busy = true
    try {
      const sess = await this.client.createSession({ campaign: campaign.id, hero, quest: number, briefingId })
      this.sessions = [...this.sessions, sess]
      this.activeId = sess.id
      this.questNumber = number
      this.questsOpen = true
      this.error = null
      return true
    } catch (e) {
      this.error = this.fail(e)
      return false
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
        this.error = this.fail(e)
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

  /** Live sessions waiting for the user that the user has not looked at. */
  get attention(): SessionInfo[] {
    return unattended(this.sessions, this.seen)
  }

  /**
   * Reads every session's activity. Only sessions already on screen are
   * updated, so a session closed a moment ago is not brought back.
   */
  async refreshSessions() {
    try {
      const fresh = new Map((await this.client.getSessions()).map((s) => [s.id, s]))
      this.sessions = this.sessions.map((s) => {
        const n = fresh.get(s.id)
        return n ? { ...s, exited: s.exited || n.exited, exitCode: s.exited ? s.exitCode : n.exitCode, activity: n.activity, lost: n.lost } : s
      })
      this.seen = stillWaiting(this.sessions, this.seen)
    } catch (e) {
      if ((e as { status?: number }).status === 401) this.unauthorised = true
    }
  }

  /** The user is looking at the open session: its wait no longer needs a badge. */
  attend(viewing: boolean) {
    const a = this.active
    if (viewing && a && !a.exited && a.activity === 'needs-you' && !this.seen.has(a.id)) {
      this.seen = new Set([...this.seen, a.id])
    }
  }

  markExited(id: string, code: number) {
    this.sessions = this.sessions.map((s) => (s.id === id ? { ...s, exited: true, exitCode: code } : s))
  }
}

function message(e: unknown): string {
  return e instanceof Error ? e.message : String(e)
}
