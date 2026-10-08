import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError, type AppState, type SessionInfo } from './api'
import { AppStore, type Client } from './store.svelte'

const sess = (id: string, over: Partial<SessionInfo> = {}): SessionInfo => ({
  id, kind: 'hero', hero: 'opus', title: 'Opus', dir: '/p', created: '', exited: false, exitCode: 0, ...over,
})

const baseState: AppState = {
  version: 'v1',
  claude: { found: true, path: '/bin/claude' },
  defaultHeroes: true,
  heroes: [{ id: 'opus', label: 'Opus' }],
  sessions: [],
}

function fakeClient(over: Partial<Client> = {}): Client {
  return {
    getState: vi.fn(async () => baseState),
    createSession: vi.fn(async (req) => sess('s1', { kind: req.shell ? 'shell' : 'hero' })),
    deleteSession: vi.fn(async () => undefined),
    ...over,
  }
}

beforeEach(() => localStorage.clear())

describe('AppStore', () => {
  it('refresh loads state and selects the newest session', async () => {
    const c = fakeClient({ getState: vi.fn(async () => ({ ...baseState, sessions: [sess('a'), sess('b')] })) })
    const s = new AppStore(c)
    await s.refresh()
    expect(s.heroes).toHaveLength(1)
    expect(s.version).toBe('v1')
    expect(s.activeId).toBe('b')
    expect(s.error).toBeNull()
  })

  it('refresh keeps a still-valid selection', async () => {
    const c = fakeClient({ getState: vi.fn(async () => ({ ...baseState, sessions: [sess('a'), sess('b')] })) })
    const s = new AppStore(c)
    await s.refresh()
    s.select('a')
    await s.refresh()
    expect(s.activeId).toBe('a')
  })

  it('refresh reports errors', async () => {
    const s = new AppStore(fakeClient({ getState: vi.fn(async () => { throw new ApiError(401, 'nope') }) }))
    await s.refresh()
    expect(s.error).toBe('nope')
  })

  it('launch rejects a relative directory without calling the server', async () => {
    const c = fakeClient()
    const s = new AppStore(c)
    s.setDir('relative/dir')
    await s.launch({ hero: 'opus' })
    expect(c.createSession).not.toHaveBeenCalled()
    expect(s.error).toMatch(/absolute/)
  })

  it('launch creates a session, selects it and trims the directory', async () => {
    const c = fakeClient()
    const s = new AppStore(c)
    s.setDir('  /work/app  ')
    await s.launch({ hero: 'opus' })
    expect(c.createSession).toHaveBeenCalledWith({ hero: 'opus', dir: '/work/app' })
    expect(s.sessions.map((x) => x.id)).toEqual(['s1'])
    expect(s.activeId).toBe('s1')
    expect(s.busy).toBe(false)
  })

  it('launch accepts a Windows drive path and a shell target', async () => {
    const c = fakeClient()
    const s = new AppStore(c)
    s.setDir('C:\\work')
    await s.launch({ shell: true })
    expect(c.createSession).toHaveBeenCalledWith({ shell: true, dir: 'C:\\work' })
  })

  it('launch shows the server error and clears busy', async () => {
    const c = fakeClient({ createSession: vi.fn(async () => { throw new ApiError(400, 'dir is not an existing directory') }) })
    const s = new AppStore(c)
    s.setDir('/nope')
    await s.launch({ hero: 'opus' })
    expect(s.error).toBe('dir is not an existing directory')
    expect(s.busy).toBe(false)
    expect(s.sessions).toHaveLength(0)
  })

  it('close removes the tab and selects a neighbour', async () => {
    const c = fakeClient({ getState: vi.fn(async () => ({ ...baseState, sessions: [sess('a'), sess('b'), sess('c')] })) })
    const s = new AppStore(c)
    await s.refresh()
    s.select('b')
    await s.close('b')
    expect(s.sessions.map((x) => x.id)).toEqual(['a', 'c'])
    expect(s.activeId).toBe('c')
    await s.close('c')
    expect(s.activeId).toBe('a')
    await s.close('a')
    expect(s.activeId).toBeNull()
  })

  it('close tolerates a session already gone but surfaces other failures', async () => {
    const gone = new AppStore(fakeClient({ deleteSession: vi.fn(async () => { throw new ApiError(404, 'no such session') }) }))
    gone.sessions = [sess('a')]
    await gone.close('a')
    expect(gone.sessions).toHaveLength(0)
    expect(gone.error).toBeNull()

    const bad = new AppStore(fakeClient({ deleteSession: vi.fn(async () => { throw new ApiError(500, 'boom') }) }))
    bad.sessions = [sess('a')]
    await bad.close('a')
    expect(bad.sessions).toHaveLength(1)
    expect(bad.error).toBe('boom')
  })

  it('markExited flags only the right session', () => {
    const s = new AppStore(fakeClient())
    s.sessions = [sess('a'), sess('b')]
    s.markExited('b', 3)
    expect(s.sessions[0].exited).toBe(false)
    expect(s.sessions[1]).toMatchObject({ exited: true, exitCode: 3 })
  })

  it('remembers the directory, and survives unavailable storage', () => {
    const s = new AppStore(fakeClient())
    s.setDir('/remember/me')
    expect(new AppStore(fakeClient()).dir).toBe('/remember/me')

    const spy = vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => { throw new Error('denied') })
    expect(() => s.setDir('/x')).not.toThrow()
    expect(s.dir).toBe('/x')
    spy.mockRestore()
  })

  it('select ignores unknown ids', () => {
    const s = new AppStore(fakeClient())
    s.sessions = [sess('a')]
    s.select('a')
    s.select('zzz')
    expect(s.activeId).toBe('a')
  })
})
