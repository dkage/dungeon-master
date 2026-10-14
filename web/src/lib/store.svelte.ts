import * as realApi from './api'
import { KEY as SHORTCUTS_KEY, check as checkShortcut, parseOverrides, platformOf, type ActionId, type Overrides } from './shortcuts'
import { PLACE_KEY, format as formatPlace, type Place } from './place'
import { DEFAULTS, KEY as COLUMNS_KEY, fit, parse as parseColumns, setWidth, type Columns, type Edge } from './columns'
import { SPLIT, restore, toggle, type Region, type Workspace } from './workspace'
import type {
  AppState, Campaign, CampaignInput, CreateRequest, CampaignPatch, EffortTier, GithubStatus, Hero, HeroesConfig, LaunchPreview, ProjectSuggestion, QuestDetail, QuestResult, Scope, ScopeOptions,
  SessionInfo,
} from './api'
import { stillWaiting, unattended } from './attention'
import { buildModel, placeOf, type MapModel } from './map/model'
import { pickToNotify } from './notify'
import { TERMINAL_ACTION } from './termSearch'
import { zoomed, type Zoom } from './termSize'
import { DEFAULT_SETTINGS, parseSettings, type UISettings } from './appearance'

export type Client = Pick<
  typeof realApi,
  | 'getState' | 'getSessions' | 'createSession' | 'deleteSession' | 'addCampaign' | 'updateCampaign' | 'removeCampaign'
  | 'getQuests' | 'getQuest' | 'previewQuestLaunch' | 'getGithub' | 'setScope' | 'getScopeOptions' | 'getProjectSuggestions' | 'createSampleCampaign'
  | 'adoptStarterHeroes' | 'getHeroesConfig' | 'saveHeroesConfig' | 'pickFolder' | 'openCampaignFolder'
  | 'setEffortLabels' | 'setEffortOverride' | 'getAppearance' | 'saveAppearance'
>

const CAMPAIGN_KEY = 'dungeon.campaign'
const VIEW_KEY = 'dungeon.panel.view'

/** Which level of the map is shown (docs/design/map.md §3, Levels). */
export type MapLevel = { kind: 'world' } | { kind: 'dungeon'; id: number } | { kind: 'tavern' }

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

export type { ReadingFont, WordMode, ThemePack, UISettings } from './appearance'

const SETTINGS_KEY = 'dungeon.appearance'

/** What this browser last showed: the first paint and the one-time import into the server's copy. */
function loadCache(): { settings: UISettings; present: boolean } {
  try {
    const raw = localStorage.getItem(SETTINGS_KEY)
    if (!raw) return { settings: { ...DEFAULT_SETTINGS }, present: false }
    return { settings: parseSettings(JSON.parse(raw)), present: true }
  } catch {
    return { settings: { ...DEFAULT_SETTINGS }, present: false }
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
  /** Something the user should know that is not an error, for example a working copy that was kept. */
  notice = $state<string | null>(null)
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
  /** Saved widths of the side columns, and the window they are shown in. */
  columns = $state<Columns>(parseColumns(load(COLUMNS_KEY)) ?? DEFAULTS)
  windowWidth = $state(typeof window === 'undefined' ? 1600 : window.innerWidth)
  /** The widths in use: saved ones that no longer fit the window fall back to the defaults. */
  columnWidths = $derived(fit(this.columns, this.windowWidth))
  setColumns(next: Columns) {
    this.columns = next
    save(COLUMNS_KEY, JSON.stringify(next))
  }
  resizeColumn(edge: Edge, width: number) {
    this.setColumns(setWidth(this.columnWidths, edge, width, this.windowWidth))
  }
  /**
   * Go back to a remembered place. Only what still exists is restored; anything stale falls
   * back to the default without an error. Nothing is started, restarted or ended.
   */
  async applyPlace(p: Place) {
    if (p.campaign) {
      // The session, issue and map level belong to that project; without it they mean nothing here.
      if (!this.campaigns.some((c) => c.id === p.campaign)) {
        if (p.view) this.setPanelView(p.view)
        return
      }
      this.selectCampaign(p.campaign)
    }
    const s = p.session ? this.sessions.find((x) => x.id === p.session) : undefined
    if (s && !s.lost && s.campaign === this.campaignId) this.activeId = s.id
    if (p.view) this.setPanelView(p.view)
    if (p.quest !== undefined && this.campaignId) {
      await this.selectQuest(p.quest)
      if (this.detailError) this.closeQuest()
    }
    const level = p.level
    if (level && level.kind !== 'world' && this.campaignId) {
      if (!this.quests) await this.loadQuests(false, true)
      const known = level.kind === 'tavern' || this.mapModel.dungeons.some((d) => d.id === level.id)
      if (known) this.mapLevel = level
    }
  }
  /** The hash for the current place; saved so a bare URL can come back to it. */
  placeHash(): string {
    const h = formatPlace(this.place)
    save(PLACE_KEY, h)
    return h
  }
  /** The workspace region that fills the application, or null for the split layout. */
  workspace = $state<Workspace>(SPLIT)
  /** Expand a region, or restore the split when it is already expanded. */
  toggleRegion(region: Region) {
    this.workspace = toggle(this.workspace, region)
  }
  /** Opening a session or issue must not leave it in a hidden region. */
  reveal(region: Region) {
    const open = this.workspace.expanded
    if (open === null || open === region) return
    if (region === 'detail' && (open === 'map')) return
    this.workspace = { expanded: region }
  }
  restoreSplit() {
    this.workspace = restore()
  }
  /** The right panel shows the map or the list of the same quests. */
  panelView = $state<'map' | 'list'>(load(VIEW_KEY) === 'list' ? 'list' : 'map')
  mapLevel = $state<MapLevel>({ kind: 'world' })
  /** Shortcut overrides (null disables an action); missing actions use their platform default. */
  platform = platformOf()
  shortcuts = $state<Overrides>(parseOverrides(load(SHORTCUTS_KEY) || null, platformOf()))
  /** Set when the overrides could not be saved, so Settings can say so instead of claiming success. */
  shortcutError = $state('')
  private saveShortcuts(next: Overrides) {
    this.shortcuts = next
    try {
      localStorage.setItem(SHORTCUTS_KEY, JSON.stringify(next))
      this.shortcutError = ''
    } catch {
      this.shortcutError = 'Could not save shortcuts in this browser; they will be lost on reload.'
    }
  }
  /** Validates and stores a binding. Returns the reason it was refused, or null. */
  setShortcut(id: ActionId, text: string | null): string | null {
    if (text === null) {
      this.saveShortcuts({ ...this.shortcuts, [id]: null })
      return null
    }
    const res = checkShortcut(id, text, this.shortcuts, this.platform)
    if (!res.ok) return res.reason
    this.saveShortcuts({ ...this.shortcuts, [id]: res.binding })
    return null
  }
  resetShortcut(id: ActionId) {
    const next = { ...this.shortcuts }
    delete next[id]
    this.saveShortcuts(next)
  }
  resetShortcuts() {
    this.saveShortcuts({})
  }
  /** The semantic actions; buttons, shortcuts and a future native menu all come through here. */
  runAction(id: ActionId, openSettings: () => void = () => {}) {
    switch (id) {
      case 'expand-projects': return this.toggleRegion('projects')
      case 'expand-terminal': return this.toggleRegion('terminal')
      case 'expand-map': return this.toggleRegion('map')
      case 'expand-detail':
        if (this.questNumber !== null) this.toggleRegion('detail')
        return
      case 'restore-split': return this.restoreSplit()
      case 'show-map': this.reveal('map'); return this.setPanelView('map')
      case 'show-list': this.reveal('map'); return this.setPanelView('list')
      case 'open-settings': return openSettings()
      case 'terminal-zoom-in': return this.zoomTerminal('in')
      case 'terminal-zoom-out': return this.zoomTerminal('out')
      case 'terminal-zoom-reset': return this.zoomTerminal('reset')
      case 'search-terminal':
      case 'terminal-latest':
        // The visible terminal listens; this is not layout state.
        if (this.active && !this.active.lost) window.dispatchEvent(new CustomEvent(TERMINAL_ACTION, { detail: id }))
        return
      case 'next-session':
      case 'previous-session': {
        const mine = this.campaignId ? this.sessionsOf(this.campaignId).filter((s) => !s.lost) : []
        if (mine.length === 0) return
        const at = mine.findIndex((s) => s.id === this.activeId)
        const step = id === 'next-session' ? 1 : -1
        void this.selectSession(mine[(at + step + mine.length) % mine.length].id)
      }
    }
  }
  /** Where the user is, as identifiers only. */
  place = $derived<Place>({
    campaign: this.campaignId ?? undefined,
    quest: this.questNumber ?? undefined,
    session: this.active?.id,
    view: this.panelView,
    level: this.mapLevel,
  })
  /** One view of every quest, shared by the map and the list so they agree. */
  mapModel: MapModel = $derived(buildModel(this.quests?.quests ?? [], this.sessions, this.campaignId))
  /**
   * The same quests without sessions, for the map layout. Positions do not depend on who is
   * working where, so a sessions poll must not rebuild the graph and re-key the drawing.
   */
  layoutModel: MapModel = $derived(buildModel(this.quests?.quests ?? []))
  /**
   * The server refused this tab's credentials (it was restarted with another
   * token, or this browser never opened the signed-in link). Shown as its own
   * screen: an empty list would look like lost data.
   */
  unauthorised = $state(false)
  suggestions = $state<ProjectSuggestion[]>([])
  suggestionsLoading = $state(false)
  firstRunDismissed = $state(load('dungeon.first-run.dismissed') === '1')
  /** The user asked for the guide on purpose; it covers an open session until they pick something else. */
  guideOpen = $state(false)
  private cached = loadCache()
  appearance = $state<UISettings>(this.cached.settings)
  /** Why the last attempt to keep the appearance on this computer failed; the page keeps showing what it has. */
  appearanceError = $state<string | null>(null)
  private appearanceSynced = false
  private appearanceSaving = false
  private appearanceDirty = false

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

  /** The running session for one issue of a project, in any working copy. */
  sessionForQuest(campaignId: string, number: number): SessionInfo | undefined {
    const main = this.questSessionForCampaign(campaignId)
    if (main?.quest === number) return main
    const campaign = this.campaigns.find((c) => c.id === campaignId)
    const tree = campaign?.gitRoot || campaign?.dir
    return this.sessions.find((s) => {
      if (s.quest !== number || s.exited || s.lost) return false
      const other = this.campaigns.find((c) => c.id === s.campaign)
      return (other?.gitRoot || other?.dir) === tree
    })
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
    void this.syncAppearance()
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

  /** Whether the stage shows the getting-started guide instead of a terminal. */
  get guideShown() {
    return this.guideOpen || (!this.active && !this.firstRunDismissed)
  }

  dismissFirstRun() {
    this.guideOpen = false
    this.firstRunDismissed = true
    save('dungeon.first-run.dismissed', '1')
  }

  openFirstRun() {
    this.guideOpen = true
    this.firstRunDismissed = false
    save('dungeon.first-run.dismissed', '0')
  }

  zoomTerminal(step: Zoom) {
    this.updateAppearance({ terminalSize: zoomed(this.appearance.terminalSize, step) })
  }
  updateAppearance(patch: Partial<UISettings>) {
    this.appearance = parseSettings({ ...this.appearance, ...patch })
    save(SETTINGS_KEY, JSON.stringify(this.appearance))
    this.appearanceDirty = true
    void this.pushAppearance()
  }
  /** Sends the latest settings; edits made while a save is running go in one follow-up save. */
  private async pushAppearance() {
    if (this.appearanceSaving || !this.appearanceSynced) return
    this.appearanceSaving = true
    try {
      while (this.appearanceDirty) {
        this.appearanceDirty = false
        await this.client.saveAppearance(this.appearance)
        this.appearanceError = null
      }
    } catch (e) {
      this.appearanceError = failure(e).message
    } finally {
      this.appearanceSaving = false
    }
  }
  /**
   * Takes the settings kept on this computer. The first time (nothing saved
   * yet) the choices this browser already holds are handed over instead, so
   * nobody loses theirs; after that the saved copy is the source of truth.
   */
  async syncAppearance() {
    if (this.appearanceSynced) return
    try {
      const got = await this.client.getAppearance()
      const hasLocal = this.cached.present || this.appearanceDirty
      const next = got.stored || !hasLocal ? got : await this.client.saveAppearance(this.appearance)
      if (!got.stored && hasLocal) this.appearanceDirty = false
      this.appearance = parseSettings(next.settings)
      save(SETTINGS_KEY, JSON.stringify(this.appearance))
      this.appearanceSynced = true
      this.appearanceError = null
      // A change made before the first answer arrived still has to be sent.
      if (this.appearanceDirty) void this.pushAppearance()
    } catch (e) {
      this.appearanceError = failure(e).message
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
    if (id !== this.campaignId) {
      this.mapLevel = { kind: 'world' }
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
    this.reveal('detail')
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

  setPanelView(view: 'map' | 'list') {
    this.panelView = view
    save(VIEW_KEY, view)
    if (view === 'map' && this.questNumber !== null) this.revealQuest(this.questNumber)
  }

  /** Moves the map to the level that shows a quest; quests outside the list stay where the map is. */
  revealQuest(n: number) {
    const at = placeOf(this.mapModel, n)
    if (at === null) return
    const here = this.mapLevel
    if (at === 'world') {
      this.mapLevel = { kind: 'world' }
    } else if (at === 'tavern') {
      if (here.kind !== 'tavern') this.mapLevel = { kind: 'tavern' }
    } else if (here.kind !== 'dungeon' || here.id !== at) {
      this.mapLevel = { kind: 'dungeon', id: at }
    }
  }

  /** Selects a quest and moves the map to it: "Jump to prerequisite". */
  async jumpToQuest(n: number) {
    this.revealQuest(n)
    await this.selectQuest(n)
  }

  /** Opens a dungeon or the Tavern from the world map. */
  enterPlace(id: number | 'tavern') {
    this.mapLevel = id === 'tavern' ? { kind: 'tavern' } : { kind: 'dungeon', id }
  }

  /** Goes up one level: a dungeon or the Tavern back to the world. */
  leavePlace() {
    this.mapLevel = { kind: 'world' }
  }

  /** Sets one issue's effort tier locally ('' lets the labels decide again). */
  async setEffortOverride(n: number, tier: EffortTier | ''): Promise<boolean> {
    const id = this.campaignId
    if (!id) return false
    try {
      const c = await this.client.setEffortOverride(id, n, tier)
      this.campaigns = this.campaigns.map((x) => (x.id === c.id ? c : x))
    } catch (e) {
      this.error = this.fail(e)
      return false
    }
    await this.loadQuests(false, true)
    if (this.questNumber === n) await this.selectQuest(n)
    return true
  }

  /** Replaces the campaign's own-label to tier mapping. */
  async setEffortLabels(labels: Record<string, EffortTier>): Promise<boolean> {
    const id = this.campaignId
    if (!id) return false
    try {
      const c = await this.client.setEffortLabels(id, labels)
      this.campaigns = this.campaigns.map((x) => (x.id === c.id ? c : x))
    } catch (e) {
      this.error = this.fail(e)
      return false
    }
    await this.loadQuests(false, true)
    if (this.questNumber !== null) await this.selectQuest(this.questNumber)
    return true
  }

  closeQuest() {
    this.detailSeq++
    this.questNumber = null
    this.detail = null
    this.detailError = null
    this.detailLoading = false
  }

  /** Allows or forbids several quest sessions at once in the selected campaign's working tree. */
  async setParallelQuests(on: boolean): Promise<boolean> {
    const id = this.campaignId
    if (!id) return false
    return this.updateCampaign(id, { parallelQuests: on })
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
    this.guideOpen = false
    this.pickCampaign(id)
    if (this.active?.campaign !== id) this.activeId = this.sessionsOf(id).at(-1)?.id ?? null
    if (changed) void this.loadQuests()
  }

  async selectSession(id: string) {
    const s = this.sessions.find((x) => x.id === id)
    if (!s) return
    const changed = s.campaign !== this.campaignId
    this.guideOpen = false
    this.pickCampaign(s.campaign)
    this.activeId = id
    this.reveal('terminal')
    if (changed) void this.loadQuests()
    const session = this.sessions.find((x) => x.id === id)
    if (session?.quest) {
      this.questsOpen = true
      const expanded = this.workspace.expanded
      await this.selectQuest(session.quest)
      // The session was the target; its issue loads in the background without taking the screen.
      if (expanded !== null) this.workspace = { expanded: 'terminal' }
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
      this.guideOpen = false
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

  async launchQuest(number: number, hero: string, briefingId?: string, where?: Pick<CreateRequest, 'workingCopy' | 'carry'>): Promise<boolean> {
    const campaign = this.campaign
    if (!campaign) {
      this.error = 'Add or select a campaign first.'
      return false
    }
    this.busy = true
    try {
      const sess = await this.client.createSession({ campaign: campaign.id, hero, quest: number, briefingId, ...where })
      this.sessions = [...this.sessions, sess]
      this.guideOpen = false
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
      const kept = await this.client.deleteSession(id)
      if (kept?.copyKept) {
        this.notice = `Working copy kept because it has unfinished work (${kept.changes} changed ${kept.changes === 1 ? 'path' : 'paths'}): ${kept.copyKept} on branch ${kept.branch}. Commit or discard it there; the issue cannot start again until it is removed.`
      }
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

  private notified = new Set<string>()

  /** Sessions to announce with a desktop notification now; empty unless the user turned them on. */
  takeNotifications(viewing: boolean): SessionInfo[] {
    const { fire, notified } = pickToNotify(this.sessions, this.notified, this.seen, viewing)
    this.notified = notified
    return this.appearance.notifications ? fire : []
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
