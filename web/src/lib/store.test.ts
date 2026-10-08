import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError, type AppState, type Campaign, type SessionInfo } from './api'
import { AppStore, type Client } from './store.svelte'

const camp = (id: string, over: Partial<Campaign> = {}): Campaign => ({
  id, name: `C-${id}`, dir: `/p/${id}`, remote: 'origin', labels: [], repo: 'acme/x', ...over,
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
    ...over,
  }
}

const err = (status: number, msg: string) => vi.fn(async () => { throw new ApiError(status, msg) })

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
