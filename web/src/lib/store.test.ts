import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError, type AppState, type Campaign, type Quest, type QuestDetail, type QuestResult, type SessionInfo } from './api'
import { AppStore, type Client } from './store.svelte'

const camp = (id: string, over: Partial<Campaign> = {}): Campaign => ({
  id, name: `C-${id}`, dir: `/p/${id}`, remote: 'origin', labels: [], demo: false, repo: 'acme/x', ...over,
})
const sess = (id: string, campaign = 'c1', over: Partial<SessionInfo> = {}): SessionInfo => ({
  id, kind: 'hero', hero: 'opus', title: 'Opus', campaign, dir: '/p', created: '', exited: false, exitCode: 0, ...over,
})

const baseState: AppState = {
  version: 'v1',
  claude: { found: true, path: '/bin/claude' },
  defaultHeroes: true,
  heroes: [{ id: 'opus', label: 'Opus' }],
  campaigns: [camp('c1'), camp('c2')],
  sessions: [],
}

function fakeClient(over: Partial<Client> = {}, state: Partial<AppState> = {}): Client {
  return {
    getState: vi.fn(async () => ({ ...baseState, ...state })),
    createSession: vi.fn(async (req) => sess('s1', req.campaign, { kind: req.shell ? 'shell' : 'hero' })),
    deleteSession: vi.fn(async () => undefined),
    addCampaign: vi.fn(async (i) => camp('cnew', { dir: i.dir, name: i.name ?? 'new' })),
    updateCampaign: vi.fn(async (id, p) => camp(id, { name: p.name ?? 'C' })),
    removeCampaign: vi.fn(async () => undefined),
    getQuests: vi.fn(async () => result([quest(1)])),
    getQuest: vi.fn(async (_c, n) => detailOf(n)),
    ...over,
  }
}

const quest = (number: number, over: Partial<Quest> = {}): Quest => ({
  number, title: `Q${number}`, state: 'open', closed: false, url: '', labels: [], assignees: [],
  blockedBy: [], subTotal: 0, subDone: 0, comments: 0, prs: [], updated: '', ...over,
})
const result = (quests: Quest[], over: Partial<QuestResult> = {}): QuestResult => ({
  source: 'github', repo: 'acme/x', quests, fetchedAt: '2026-10-08T12:00:00Z', stale: false, ...over,
})
const detailOf = (n: number): QuestDetail => ({
  ...quest(n), author: 'alice', body: `body ${n}`, created: '', blocking: [], subIssues: [], commentList: [],
})
const deferred = <T>() => {
  let resolve!: (v: T) => void
  let reject!: (e: unknown) => void
  const promise = new Promise<T>((res, rej) => { resolve = res; reject = rej })
  return { promise, resolve, reject }
}

const err = (status: number, msg: string, code?: string) => vi.fn(async () => { throw new ApiError(status, msg, code) })

beforeEach(() => localStorage.clear())

describe('refresh and selection', () => {
  it('loads state and selects the first campaign and its newest session', async () => {
    const s = new AppStore(fakeClient({}, { sessions: [sess('a'), sess('b'), sess('z', 'c2')] }))
    await s.refresh()
    expect(s.version).toBe('v1')
    expect(s.campaignId).toBe('c1')
    expect(s.activeId).toBe('b')
    expect(s.campaign?.name).toBe('C-c1')
    expect(s.error).toBeNull()
  })

  it('remembers the selected campaign across stores', async () => {
    const a = new AppStore(fakeClient())
    await a.refresh()
    a.selectCampaign('c2')
    const b = new AppStore(fakeClient())
    await b.refresh()
    expect(b.campaignId).toBe('c2')
  })

  it('falls back when the remembered campaign no longer exists', async () => {
    localStorage.setItem('dungeon.campaign', 'gone')
    const s = new AppStore(fakeClient())
    await s.refresh()
    expect(s.campaignId).toBe('c1')
  })

  it('selecting a session also selects its campaign', async () => {
    const s = new AppStore(fakeClient({}, { sessions: [sess('a', 'c1'), sess('z', 'c2')] }))
    await s.refresh()
    s.selectSession('z')
    expect(s.campaignId).toBe('c2')
    expect(s.activeId).toBe('z')
    s.selectSession('nope')
    expect(s.activeId).toBe('z')
  })

  it('switching campaign shows that campaign\'s newest session, or none', async () => {
    const s = new AppStore(fakeClient({}, { sessions: [sess('a', 'c1'), sess('b', 'c1')] }))
    await s.refresh()
    s.selectCampaign('c2')
    expect(s.activeId).toBeNull()
    s.selectCampaign('c1')
    expect(s.activeId).toBe('b')
    s.selectCampaign('unknown')
    expect(s.campaignId).toBe('c1')
  })

  it('reports refresh errors', async () => {
    const s = new AppStore(fakeClient({ getState: err(401, 'nope') }))
    await s.refresh()
    expect(s.error).toBe('nope')
  })

  it('survives unavailable storage', async () => {
    const spy = vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => { throw new Error('denied') })
    const s = new AppStore(fakeClient())
    await expect(s.refresh()).resolves.toBeUndefined()
    expect(s.campaignId).toBe('c1')
    spy.mockRestore()
  })
})

describe('campaigns', () => {
  it('addCampaign adds, selects and clears the error', async () => {
    const c = fakeClient()
    const s = new AppStore(c)
    await s.refresh()
    s.error = 'old'
    expect(await s.addCampaign({ dir: '/new', name: 'Fresh' })).toBe(true)
    expect(c.addCampaign).toHaveBeenCalledWith({ dir: '/new', name: 'Fresh' })
    expect(s.campaignId).toBe('cnew')
    expect(s.campaigns).toHaveLength(3)
    expect(s.error).toBeNull()
    expect(s.busy).toBe(false)
  })

  it('addCampaign shows the server error and changes nothing', async () => {
    const s = new AppStore(fakeClient({ addCampaign: err(409, 'that directory is already a campaign') }))
    await s.refresh()
    expect(await s.addCampaign({ dir: '/c1' })).toBe(false)
    expect(s.error).toBe('that directory is already a campaign')
    expect(s.campaigns).toHaveLength(2)
    expect(s.busy).toBe(false)
  })

  it('updateCampaign replaces the entry', async () => {
    const s = new AppStore(fakeClient())
    await s.refresh()
    expect(await s.updateCampaign('c1', { name: 'Renamed' })).toBe(true)
    expect(s.campaigns.find((c) => c.id === 'c1')?.name).toBe('Renamed')
    const bad = new AppStore(fakeClient({ updateCampaign: err(400, 'bad') }))
    await bad.refresh()
    expect(await bad.updateCampaign('c1', { name: 'x' })).toBe(false)
    expect(bad.error).toBe('bad')
  })

  it('removeCampaign drops it and its sessions, then reselects', async () => {
    const s = new AppStore(fakeClient({}, { sessions: [sess('a', 'c1'), sess('z', 'c2')] }))
    await s.refresh()
    await s.removeCampaign('c1')
    expect(s.campaigns.map((c) => c.id)).toEqual(['c2'])
    expect(s.sessions.map((x) => x.id)).toEqual(['z'])
    expect(s.campaignId).toBe('c2')
    expect(s.activeId).toBe('z')
  })

  it('removeCampaign keeps everything and shows the reason when refused', async () => {
    const s = new AppStore(fakeClient({ removeCampaign: err(409, 'close this campaign\'s running sessions first') }, { sessions: [sess('a')] }))
    await s.refresh()
    await s.removeCampaign('c1')
    expect(s.campaigns).toHaveLength(2)
    expect(s.sessions).toHaveLength(1)
    expect(s.error).toMatch(/running sessions/)
  })

  it('removeCampaign tolerates a campaign already gone', async () => {
    const s = new AppStore(fakeClient({ removeCampaign: err(404, 'no such campaign') }))
    await s.refresh()
    await s.removeCampaign('c1')
    expect(s.campaigns.map((c) => c.id)).toEqual(['c2'])
    expect(s.error).toBeNull()
  })
})

describe('sessions', () => {
  it('launch needs a campaign', async () => {
    const c = fakeClient({}, { campaigns: [] })
    const s = new AppStore(c)
    await s.refresh()
    await s.launch({ hero: 'opus' })
    expect(c.createSession).not.toHaveBeenCalled()
    expect(s.error).toMatch(/campaign/)
  })

  it('launch creates a session in the selected campaign and selects it', async () => {
    const c = fakeClient()
    const s = new AppStore(c)
    await s.refresh()
    s.selectCampaign('c2')
    await s.launch({ hero: 'opus' })
    expect(c.createSession).toHaveBeenCalledWith({ hero: 'opus', campaign: 'c2' })
    expect(s.sessionsOf('c2').map((x) => x.id)).toEqual(['s1'])
    expect(s.activeId).toBe('s1')
    expect(s.busy).toBe(false)
    await s.launch({ shell: true })
    expect(c.createSession).toHaveBeenLastCalledWith({ shell: true, campaign: 'c2' })
  })

  it('launch shows the server error and clears busy', async () => {
    const s = new AppStore(fakeClient({ createSession: err(400, 'campaign directory is no longer available') }))
    await s.refresh()
    await s.launch({ hero: 'opus' })
    expect(s.error).toBe('campaign directory is no longer available')
    expect(s.busy).toBe(false)
    expect(s.sessions).toHaveLength(0)
  })

  it('close removes the session and selects a neighbour within the campaign', async () => {
    const s = new AppStore(fakeClient({}, { sessions: [sess('a'), sess('b'), sess('c'), sess('z', 'c2')] }))
    await s.refresh()
    s.selectSession('b')
    await s.close('b')
    expect(s.sessions.map((x) => x.id)).toEqual(['a', 'c', 'z'])
    expect(s.activeId).toBe('c')
    await s.close('c')
    expect(s.activeId).toBe('a')
    await s.close('a')
    expect(s.activeId).toBeNull() // never jumps into another campaign
  })

  it('close tolerates a session already gone but surfaces other failures', async () => {
    const gone = new AppStore(fakeClient({ deleteSession: err(404, 'no such session') }, { sessions: [sess('a')] }))
    await gone.refresh()
    await gone.close('a')
    expect(gone.sessions).toHaveLength(0)
    expect(gone.error).toBeNull()

    const bad = new AppStore(fakeClient({ deleteSession: err(500, 'boom') }, { sessions: [sess('a')] }))
    await bad.refresh()
    await bad.close('a')
    expect(bad.sessions).toHaveLength(1)
    expect(bad.error).toBe('boom')
  })

  it('markExited flags only the right session', async () => {
    const s = new AppStore(fakeClient({}, { sessions: [sess('a'), sess('b')] }))
    await s.refresh()
    s.markExited('b', 3)
    expect(s.sessions[0].exited).toBe(false)
    expect(s.sessions[1]).toMatchObject({ exited: true, exitCode: 3 })
  })
})

describe('quests', () => {
  it('loads the selected campaign\'s quests on startup', async () => {
    const c = fakeClient()
    const s = new AppStore(c)
    await s.refresh()
    await vi.waitFor(() => expect(s.quests).not.toBeNull())
    expect(c.getQuests).toHaveBeenCalledWith('c1', false)
    expect(s.questsFor).toBe('c1')
    expect(s.quests?.quests.map((q) => q.number)).toEqual([1])
    expect(s.questsLoading).toBe(false)
    expect(s.questsError).toBeNull()
  })

  it('reloads when another campaign is selected, and clears the old quests at once', async () => {
    const d = deferred<QuestResult>()
    const getQuests = vi.fn()
      .mockResolvedValueOnce(result([quest(1)]))
      .mockReturnValueOnce(d.promise)
    const s = new AppStore(fakeClient({ getQuests }))
    await s.refresh()
    await vi.waitFor(() => expect(s.quests).not.toBeNull())
    s.selectQuest(1)
    s.selectCampaign('c2')
    expect(s.quests).toBeNull() // nothing of c1 under c2
    expect(s.questNumber).toBeNull()
    expect(s.questsLoading).toBe(true)
    expect(getQuests).toHaveBeenLastCalledWith('c2', false)
    d.resolve(result([quest(9)]))
    await vi.waitFor(() => expect(s.quests?.quests[0].number).toBe(9))
    expect(s.questsFor).toBe('c2')
  })

  it('drops a slow response that arrives after the campaign changed', async () => {
    const slow = deferred<QuestResult>()
    const getQuests = vi.fn()
      .mockReturnValueOnce(slow.promise) // c1, slow
      .mockResolvedValueOnce(result([quest(2)])) // c2, fast
    const s = new AppStore(fakeClient({ getQuests }))
    await s.refresh()
    s.selectCampaign('c2')
    await vi.waitFor(() => expect(s.quests?.quests[0].number).toBe(2))
    slow.resolve(result([quest(1)])) // c1's answer finally lands
    await Promise.resolve()
    await Promise.resolve()
    expect(s.quests?.quests[0].number).toBe(2)
    expect(s.questsLoading).toBe(false)
  })

  it('drops a slow failure too', async () => {
    const slow = deferred<QuestResult>()
    const getQuests = vi.fn()
      .mockReturnValueOnce(slow.promise)
      .mockResolvedValueOnce(result([quest(2)]))
    const s = new AppStore(fakeClient({ getQuests }))
    await s.refresh()
    s.selectCampaign('c2')
    await vi.waitFor(() => expect(s.quests).not.toBeNull())
    slow.reject(new ApiError(502, 'late failure', 'upstream'))
    await Promise.resolve()
    await Promise.resolve()
    expect(s.questsError).toBeNull()
  })

  it('keeps the server\'s error code so the panel can offer the right action', async () => {
    const s = new AppStore(fakeClient({ getQuests: err(409, 'no repository', 'no_repo') }))
    await s.refresh()
    await vi.waitFor(() => expect(s.questsError).not.toBeNull())
    expect(s.questsError).toEqual({ code: 'no_repo', message: 'no repository' })
    expect(s.quests).toBeNull()
    expect(s.questsLoading).toBe(false)
    const plain = new AppStore(fakeClient({ getQuests: vi.fn(async () => { throw new Error('network down') }) }))
    await plain.refresh()
    await vi.waitFor(() => expect(plain.questsError?.code).toBe('error'))
  })

  it('refresh asks the server to bypass its cache', async () => {
    const c = fakeClient()
    const s = new AppStore(c)
    await s.refresh()
    await s.loadQuests(true)
    expect(c.getQuests).toHaveBeenLastCalledWith('c1', true)
  })

  it('opens a quest, loads its detail, and can close it', async () => {
    const c = fakeClient()
    const s = new AppStore(c)
    await s.refresh()
    await s.selectQuest(7)
    expect(c.getQuest).toHaveBeenCalledWith('c1', 7)
    expect(s.questNumber).toBe(7)
    expect(s.detail?.body).toBe('body 7')
    expect(s.detailLoading).toBe(false)
    s.closeQuest()
    expect(s.questNumber).toBeNull()
    expect(s.detail).toBeNull()
  })

  it('shows detail errors and ignores a slow detail after a newer click', async () => {
    const bad = new AppStore(fakeClient({ getQuest: err(404, 'That issue was not found.', 'not_found') }))
    await bad.refresh()
    await bad.selectQuest(3)
    expect(bad.detailError).toEqual({ code: 'not_found', message: 'That issue was not found.' })
    expect(bad.detail).toBeNull()

    const slow = deferred<QuestDetail>()
    const getQuest = vi.fn().mockReturnValueOnce(slow.promise).mockResolvedValueOnce(detailOf(2))
    const s = new AppStore(fakeClient({ getQuest }))
    await s.refresh()
    const first = s.selectQuest(1)
    await s.selectQuest(2)
    slow.resolve(detailOf(1))
    await first
    expect(s.questNumber).toBe(2)
    expect(s.detail?.number).toBe(2)
  })

  it('closing the quest view cancels a detail still loading', async () => {
    const slow = deferred<QuestDetail>()
    const s = new AppStore(fakeClient({ getQuest: vi.fn().mockReturnValue(slow.promise) }))
    await s.refresh()
    const pending = s.selectQuest(1)
    s.closeQuest()
    slow.resolve(detailOf(1))
    await pending
    expect(s.detail).toBeNull()
    expect(s.questNumber).toBeNull()
  })

  it('setDemo patches the campaign then reloads from the new source', async () => {
    const c = fakeClient({
      updateCampaign: vi.fn(async (id, p) => camp(id, { demo: p.demo })),
      getQuests: vi.fn()
        .mockResolvedValueOnce(result([quest(1)]))
        .mockResolvedValueOnce(result([quest(100)], { source: 'demo', repo: undefined })),
    })
    const s = new AppStore(c)
    await s.refresh()
    await vi.waitFor(() => expect(s.quests).not.toBeNull())
    expect(await s.setDemo(true)).toBe(true)
    expect(c.updateCampaign).toHaveBeenCalledWith('c1', { demo: true })
    expect(s.campaign?.demo).toBe(true)
    expect(s.quests?.source).toBe('demo')
    expect(s.quests?.quests[0].number).toBe(100)
  })

  it('setDemo reports failure without touching the quests', async () => {
    const s = new AppStore(fakeClient({ updateCampaign: err(400, 'nope') }))
    await s.refresh()
    await vi.waitFor(() => expect(s.quests).not.toBeNull())
    expect(await s.setDemo(true)).toBe(false)
    expect(s.error).toBe('nope')
    expect(s.quests).not.toBeNull()
    const none = new AppStore(fakeClient({}, { campaigns: [] }))
    await none.refresh()
    expect(await none.setDemo(true)).toBe(false)
  })

  it('has nothing to load without a campaign', async () => {
    const c = fakeClient({}, { campaigns: [] })
    const s = new AppStore(c)
    await s.refresh()
    expect(c.getQuests).not.toHaveBeenCalled()
    expect(s.quests).toBeNull()
  })

  it('removing the selected campaign loads the next one\'s quests', async () => {
    const c = fakeClient()
    const s = new AppStore(c)
    await s.refresh()
    await vi.waitFor(() => expect(s.questsFor).toBe('c1'))
    await s.removeCampaign('c1')
    await vi.waitFor(() => expect(s.questsFor).toBe('c2'))
    expect(c.getQuests).toHaveBeenLastCalledWith('c2', false)
  })

  it('selecting a session of another campaign switches the quest list', async () => {
    const c = fakeClient({}, { sessions: [sess('z', 'c2')] })
    const s = new AppStore(c)
    await s.refresh()
    await vi.waitFor(() => expect(s.questsFor).not.toBeNull())
    s.selectCampaign('c1')
    s.selectSession('z')
    expect(s.campaignId).toBe('c2')
    expect(s.questsFor).toBe('c2')
  })
})

describe('signed-out tabs', () => {
  it('flags a refused tab instead of looking like an empty app', async () => {
    const s = new AppStore(fakeClient({ getState: err(401, 'not authorised') }))
    await s.refresh()
    expect(s.unauthorised).toBe(true)
    expect(s.campaigns).toEqual([])
  })

  it('clears the flag once the server accepts the tab again', async () => {
    const getState = vi.fn().mockRejectedValueOnce(new ApiError(401, 'no')).mockResolvedValueOnce(baseState)
    const s = new AppStore(fakeClient({ getState }))
    await s.refresh()
    expect(s.unauthorised).toBe(true)
    await s.refresh()
    expect(s.unauthorised).toBe(false)
    expect(s.campaigns).toHaveLength(2)
  })

  it('notices a 401 on any later call too', async () => {
    const s = new AppStore(fakeClient({ createSession: err(401, 'no') }))
    await s.refresh()
    await s.launch({ shell: true })
    expect(s.unauthorised).toBe(true)
    const q = new AppStore(fakeClient({ getQuests: err(401, 'no') }))
    await q.refresh()
    await vi.waitFor(() => expect(q.unauthorised).toBe(true))
    const d = new AppStore(fakeClient({ getQuest: err(401, 'no') }))
    await d.refresh()
    await d.selectQuest(1)
    expect(d.unauthorised).toBe(true)
  })

  it('does not flag other failures', async () => {
    const s = new AppStore(fakeClient({ getState: err(500, 'boom') }))
    await s.refresh()
    expect(s.unauthorised).toBe(false)
    expect(s.error).toBe('boom')
  })
})
