// Typed client for the Dungeon Master server. Every write sends application/json,
// which the server's guard requires.

export interface Hero {
  id: string
  label: string
}

export interface HeroConfig extends Hero {
  args: string[]
  env: string[]
}

export interface HeroesConfig {
  defaultHero: string
  heroes: HeroConfig[]
}

export interface Scope {
  mode: 'labels' | 'assigned' | 'all'
  labels: string[]
}

export interface ScopeOptions {
  openTotal: number
  assigned: number
  login?: string
  labels: { name: string; color?: string; open: number }[]
}

export interface Campaign {
  id: string
  name: string
  dir: string
  remote: string
  /** Which issues the quest panel shows; null until the user has chosen. */
  scope: Scope | null
  /** Shows built-in sample quests instead of GitHub issues. */
  demo: boolean
  /** Allows several quest sessions at once in one Git working tree. */
  parallelQuests?: boolean
  /** Built-in scratch campaign; its workspace lives under the local state directory. */
  sample?: boolean
  gitRoot?: string
  /** "owner/repo" on github.com, when the remote could be resolved. */
  repo?: string
  /** The team's own labels mapped to effort tiers (local state). */
  effortLabels?: Record<string, EffortTier>
  /** Per-issue effort set locally, by issue number. */
  effortOverrides?: Record<string, EffortTier>
}

export interface ProjectSuggestion {
  name: string
  dir: string
  repo?: string
}

export interface SessionInfo {
  id: string
  kind: 'hero' | 'shell'
  hero?: string
  /** GitHub issue number for a quest session; absent for free sessions. */
  quest?: number
  title: string
  campaign: string
  dir: string
  created: string
  exited: boolean
  exitCode: number
  /** What a live session is doing, judged from its output; absent for finished or lost ones. */
  activity?: 'working' | 'idle' | 'needs-you' | 'ended'
  /** Was running when Dungeon Master last stopped; only the record remains. */
  lost?: boolean
}

export interface AppState {
  version: string
  claude: { found: boolean; path?: string; error?: string }
  defaultHeroes: boolean
  defaultHero?: string
  partyError?: string
  heroes: Hero[]
  campaigns: Campaign[]
  sessions: SessionInfo[]
}

export interface CampaignInput {
  dir: string
  name?: string
  remote?: string
}

export type CampaignPatch = Partial<Omit<CampaignInput, 'dir'>> & { demo?: boolean; parallelQuests?: boolean }

export interface CreateRequest {
  campaign: string
  hero?: string
  shell?: boolean
  opener?: string
  quest?: number
  briefingId?: string
  cols?: number
  rows?: number
}

export interface LaunchPreview {
  content: string
  truncated: boolean
  commentsShown: number
  commentsTotal: number
  canStart: boolean
  reason?: string
  hero?: string
  heroLabel?: string
  directory?: string
  briefingId?: string
  briefingFile?: string
  opener?: string
  systemPrompt?: string
  command?: string[]
  existingSession?: SessionInfo
}

export type QuestState = 'open' | 'locked' | 'claimed' | 'cleared'

export type EffortTier = 'trivial' | 'low' | 'moderate' | 'high' | 'extreme'
export type Priority = 'low' | 'normal' | 'high' | 'urgent'

export interface QuestLabel {
  name: string
  color?: string
}

export interface QuestRef {
  number: number
  title?: string
  closed: boolean
}

export interface QuestPR {
  number: number
  url: string
  state: 'open' | 'closed' | 'merged'
}

export interface Quest {
  number: number
  title: string
  state: QuestState
  closed: boolean
  reason?: string
  url: string
  labels: QuestLabel[]
  assignees: string[]
  milestone?: string
  parent?: number
  blockedBy: QuestRef[]
  subTotal: number
  subDone: number
  comments: number
  prs: QuestPR[]
  updated: string
  /** Absent when the effort is unknown (no label, or labels that disagree). */
  effort?: EffortTier
  effortSource?: 'override' | 'label' | 'mapping'
  effortConflict?: string[]
  priority?: Priority
  priorityConflict?: string[]
  /** Creature id from the roster, chosen once per issue; absent without a tier. */
  creature?: string
}

export interface QuestResult {
  source: 'github' | 'demo'
  repo?: string
  /** Which issues were asked for; absent for demo data. */
  scope?: Scope
  quests: Quest[]
  /** Open issues in the whole repository, whatever the scope. */
  totalOpen: number
  /** More issues matched than were fetched: narrow the scope. */
  truncated: boolean
  fetchedAt: string
  /** A refresh failed, so older data is shown; `warning` says why. */
  stale: boolean
  warning?: string
}

export interface QuestComment {
  author: string
  body: string
  created: string
  url?: string
}

export interface QuestDetail extends Quest {
  author: string
  body: string
  created: string
  blocking: QuestRef[]
  subIssues: QuestRef[]
  parentTitle?: string
  commentList: QuestComment[]
  /** Fresh context assembled from this issue when its detail is loaded. */
  briefing?: BriefingResult
}

export interface BriefingResult {
  content: string
  truncated: boolean
  commentsShown: number
  commentsTotal: number
}

export interface Ration {
  resource: string
  limit: number
  remaining: number
  used: number
  resetsAt: string
  seenAt: string
}

export interface GithubStatus {
  rations: Ration[]
  camp: { active: boolean; until?: string; reason?: string; level?: number }
}

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
    /** Machine-readable reason from the server, when it gave one. */
    public code?: string,
  ) {
    super(message)
    this.name = 'ApiError'
  }
}

async function call<T>(method: string, path: string, body?: unknown): Promise<T | undefined> {
  const headers: Record<string, string> = {}
  if (method !== 'GET') headers['Content-Type'] = 'application/json'
  const res = await fetch(path, {
    method,
    headers,
    credentials: 'same-origin',
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  if (res.status === 204) return undefined
  let data: unknown
  try {
    data = await res.json()
  } catch {
    data = undefined
  }
  if (!res.ok) {
    const obj = data && typeof data === 'object' ? (data as { error?: unknown; code?: unknown }) : {}
    const msg =
      obj.error !== undefined
        ? String(obj.error)
        : res.status === 401
          ? 'not authorised: open the URL printed by `dungeon serve` once'
          : `request failed (${res.status})`
    throw new ApiError(res.status, msg, typeof obj.code === 'string' ? obj.code : undefined)
  }
  return data as T
}

export const getSessions = () => call<SessionInfo[]>('GET', '/api/sessions') as Promise<SessionInfo[]>
export const getState = () => call<AppState>('GET', '/api/state') as Promise<AppState>
export const getProjectSuggestions = () =>
  call<ProjectSuggestion[]>('GET', '/api/onboarding/suggestions') as Promise<ProjectSuggestion[]>
export const createSampleCampaign = () =>
  call<Campaign>('POST', '/api/onboarding/sample', {}) as Promise<Campaign>
export const pickFolder = () => call<{ path?: string; cancelled?: boolean }>('POST', '/api/pick-folder', {}) as Promise<{ path?: string; cancelled?: boolean }>
export const adoptStarterHeroes = () =>
  call<{ saved: boolean }>('POST', '/api/heroes/adopt-starters', {}) as Promise<{ saved: boolean }>
export const getHeroesConfig = () => call<HeroesConfig>('GET', '/api/heroes/config') as Promise<HeroesConfig>
export const saveHeroesConfig = (config: HeroesConfig) =>
  call<{ saved: boolean }>('PUT', '/api/heroes/config', config) as Promise<{ saved: boolean }>
export const addCampaign = (req: CampaignInput) =>
  call<Campaign>('POST', '/api/campaigns', req) as Promise<Campaign>
export const updateCampaign = (id: string, patch: CampaignPatch) =>
  call<Campaign>('PATCH', `/api/campaigns/${encodeURIComponent(id)}`, patch) as Promise<Campaign>
export const removeCampaign = (id: string) =>
  call<void>('DELETE', `/api/campaigns/${encodeURIComponent(id)}`) as Promise<void>
export const openCampaignFolder = (id: string) =>
  call<void>('POST', `/api/campaigns/${encodeURIComponent(id)}/open-folder`, {}) as Promise<void>
export const getQuests = (campaignId: string, refresh = false, background = false) => {
  const q = [refresh && 'refresh=1', background && 'background=1'].filter(Boolean).join('&')
  return call<QuestResult>('GET', `/api/campaigns/${encodeURIComponent(campaignId)}/quests${q ? `?${q}` : ''}`) as Promise<QuestResult>
}
export const setScope = (campaignId: string, scope: Scope) =>
  call<Campaign>('PUT', `/api/campaigns/${encodeURIComponent(campaignId)}/scope`, scope) as Promise<Campaign>
export const getScopeOptions = (campaignId: string) =>
  call<ScopeOptions>('GET', `/api/campaigns/${encodeURIComponent(campaignId)}/scope/options`) as Promise<ScopeOptions>
export const getGithub = () => call<GithubStatus>('GET', '/api/github') as Promise<GithubStatus>
export const getQuest = (campaignId: string, number: number) =>
  call<QuestDetail>('GET', `/api/campaigns/${encodeURIComponent(campaignId)}/quests/${number}`) as Promise<QuestDetail>
export const previewQuestLaunch = (campaignId: string, number: number, hero: string) =>
  call<LaunchPreview>('POST', `/api/campaigns/${encodeURIComponent(campaignId)}/quests/${number}/launch-preview`, { hero }) as Promise<LaunchPreview>
export const setEffortLabels = (campaignId: string, labels: Record<string, EffortTier>) =>
  call<Campaign>('PUT', `/api/campaigns/${encodeURIComponent(campaignId)}/effort-labels`, { labels }) as Promise<Campaign>
export const setEffortOverride = (campaignId: string, number: number, tier: EffortTier | '') =>
  call<Campaign>('PUT', `/api/campaigns/${encodeURIComponent(campaignId)}/quests/${number}/effort`, { tier }) as Promise<Campaign>
export const createSession = (req: CreateRequest) =>
  call<SessionInfo>('POST', '/api/sessions', req) as Promise<SessionInfo>
export const deleteSession = (id: string) =>
  call<void>('DELETE', `/api/sessions/${encodeURIComponent(id)}`) as Promise<void>

export function termUrl(id: string, loc: Pick<Location, 'protocol' | 'host'> = location): string {
  const scheme = loc.protocol === 'https:' ? 'wss' : 'ws'
  return `${scheme}://${loc.host}/api/sessions/${encodeURIComponent(id)}/term`
}

export interface UploadedFile {
  path: string
  kind: 'image' | 'pdf' | 'text'
}
/** Stores a pasted or dropped file for a live session and returns where it now lives. */
export const uploadToSession = (id: string, name: string, data: string) =>
  call<UploadedFile>('POST', `/api/sessions/${encodeURIComponent(id)}/uploads`, { name, data }) as Promise<UploadedFile>
