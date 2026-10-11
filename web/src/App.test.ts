import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte'

vi.mock('./lib/Terminal.svelte', async () => ({
  default: (await import('./test/TerminalStub.svelte')).default,
}))

import App from './App.svelte'

interface Call { method: string; path: string; body?: unknown }

const mkQuest = (number: number, title: string, over: Record<string, unknown> = {}) => ({
  number, title, state: 'open', closed: false, url: `https://github.com/acme/demo/issues/${number}`, labels: [], assignees: [],
  blockedBy: [], subTotal: 0, subDone: 0, comments: 0, prs: [], updated: new Date().toISOString(), ...over,
})
const realQuests = [
  mkQuest(10, 'Real first issue'),
  mkQuest(11, 'Real blocked issue', { state: 'locked', blockedBy: [{ number: 10, title: 'Real first issue', closed: false }] }),
]
const demoQuests = [mkQuest(100, 'Sample quest one'), mkQuest(101, 'Sample quest two', { state: 'cleared', closed: true })]

// A tiny in-memory server behind fetch, so the real api client and store run.
function fakeServer(opts: { claudeFound?: boolean; campaigns?: boolean; lost?: boolean; noRepo?: boolean; noScope?: boolean } = {}) {
  const calls: Call[] = []
  let next = 0
  const campaigns: Record<string, unknown>[] =
    opts.campaigns === false ? [] : [{ id: 'c1', name: 'demo', dir: '/work/demo', remote: 'origin', scope: opts.noScope ? null : { mode: 'labels', labels: ['tracked'] }, demo: false, ...(opts.noRepo ? {} : { repo: 'acme/demo' }) }]
  const sessions: Record<string, unknown>[] = opts.lost
    ? [{ id: 'old', kind: 'hero', title: 'Opus', campaign: 'c1', dir: '/work/demo', created: '', exited: true, exitCode: -1, lost: true }]
    : []
  vi.stubGlobal('fetch', vi.fn(async (path: string, init: RequestInit = {}) => {
    const method = init.method ?? 'GET'
    const body = init.body ? JSON.parse(init.body as string) : undefined
    calls.push({ method, path, body })
    const json = (status: number, data?: unknown) => ({ ok: status < 300, status, json: async () => data })
    if (method === 'GET' && path === '/api/state') {
      return json(200, {
        version: 'test', claude: { found: opts.claudeFound ?? true }, defaultHeroes: false,
        heroes: [{ id: 'opus', label: 'Opus' }], campaigns, sessions,
      })
    }
    if (method === 'GET' && path === '/api/onboarding/suggestions') return json(200, [])
    if (method === 'GET' && path === '/api/github') {
      return json(200, {
        rations: [{ resource: 'graphql', limit: 5000, remaining: 4936, used: 64, resetsAt: '2026-10-08T15:37:00Z', seenAt: '' }],
        camp: { active: false },
      })
    }
    const sm = path.match(/^\/api\/campaigns\/([^/]+)\/scope(\/options)?$/)
    if (sm) {
      const camp = campaigns.find((c) => c.id === sm[1]) as { scope?: unknown; repo?: string } | undefined
      if (!camp) return json(404, { error: 'no such campaign' })
      if (method === 'GET' && sm[2]) {
        return json(200, {
          openTotal: 312, assigned: 3, login: 'me',
          labels: [{ name: 'tracked', open: 14 }, { name: 'bug', open: 40 }],
        })
      }
      if (method === 'PUT') {
        if (body.mode === 'bad') return json(400, { error: 'scope mode must be labels, assigned or all' })
        camp.scope = { mode: body.mode, labels: body.labels ?? [] }
        return json(200, camp)
      }
    }
    const qm = path.match(/^\/api\/campaigns\/([^/]+)\/quests(?:\/(\d+))?(\?[a-z0-9=&]+)?$/)
    if (method === 'GET' && qm) {
      const camp = campaigns.find((c) => c.id === qm[1]) as { demo?: boolean; repo?: string; scope?: unknown } | undefined
      if (!camp) return json(404, { error: 'no such campaign' })
      if (!camp.demo && !camp.repo) return json(409, { code: 'no_repo', error: 'This campaign has no GitHub repository.' })
      if (!camp.demo && !camp.scope && !qm[2]) return json(409, { code: 'scope_needed', error: 'Choose which of this repository\'s issues to show.' })
      const pool = camp.demo ? demoQuests : realQuests
      if (qm[2]) {
        const q = pool.find((x) => String(x.number) === qm[2])
        if (!q) return json(404, { code: 'not_found', error: 'That issue was not found.' })
        return json(200, { ...q, author: 'alice', body: `## About #${q.number}\n\nSome **markdown**.`, created: '', blocking: [], subIssues: [], commentList: [{ author: 'bob', body: 'a comment', created: new Date().toISOString() }] })
      }
      return json(200, {
        source: camp.demo ? 'demo' : 'github', repo: camp.demo ? undefined : camp.repo,
        scope: camp.demo ? undefined : camp.scope, totalOpen: pool.filter((x) => !x.closed).length + 100, truncated: false,
        quests: pool, fetchedAt: new Date().toISOString(), stale: false,
      })
    }
    if (method === 'PATCH' && path.startsWith('/api/campaigns/')) {
      const c = campaigns.find((x) => path.endsWith(String(x.id))) as { demo?: boolean } | undefined
      if (!c) return json(404, { error: 'no such campaign' })
      if (typeof body.demo === 'boolean') c.demo = body.demo
      return json(200, c)
    }
    if (method === 'POST' && path === '/api/campaigns') {
      if (body.dir === '/dup') return json(409, { error: 'that directory is already a campaign' })
      const c = { id: `c${campaigns.length + 1}`, name: body.name ?? 'new', dir: body.dir, remote: 'origin', scope: null, demo: false }
      campaigns.push(c)
      return json(201, c)
    }
    if (method === 'POST' && path === '/api/onboarding/sample') {
      const existing = campaigns.find((c) => c.sample)
      if (existing) return json(200, existing)
      const sample = { id: 'sample', name: 'Sample (not a real project)', dir: '/local/sample', remote: 'origin', scope: null, demo: true, sample: true }
      campaigns.push(sample)
      return json(201, sample)
    }
    if (method === 'DELETE' && path.startsWith('/api/campaigns/')) {
      const i = campaigns.findIndex((c) => path.endsWith(String(c.id)))
      if (i < 0) return json(404, { error: 'no such campaign' })
      campaigns.splice(i, 1)
      return json(204)
    }
    if (method === 'POST' && path === '/api/sessions') {
      const s = {
        id: `s${++next}`, kind: body.shell ? 'shell' : 'hero', hero: body.hero, campaign: body.campaign,
        title: body.shell ? 'shell' : 'Opus', dir: '/work/demo', created: '', exited: false, exitCode: 0,
      }
      sessions.push(s)
      return json(201, s)
    }
    if (method === 'DELETE') {
      const i = sessions.findIndex((s) => path.endsWith(String(s.id)))
      if (i < 0) return json(404, { error: 'no such session' })
      sessions.splice(i, 1)
      return json(204)
    }
    return json(404, { error: 'unexpected' })
  }))
  return calls
}

beforeEach(() => localStorage.clear())
afterEach(() => vi.unstubAllGlobals())

describe('App', () => {
  it('loads campaigns and heroes, launches into the selected campaign, shows its terminal', async () => {
    const calls = fakeServer()
    render(App)
    await screen.findByRole('button', { name: 'Opus' })
    expect(within(screen.getByRole('navigation', { name: 'Campaigns' })).getByText('acme/demo')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Start here' })).toBeInTheDocument()

    await fireEvent.click(screen.getByRole('button', { name: 'Opus' }))
    expect(await screen.findByTestId('term-stub')).toHaveTextContent('s1')
    expect(calls.find((c) => c.method === 'POST' && c.path === '/api/sessions')?.body).toEqual({ hero: 'opus', campaign: 'c1' })
    const list = screen.getByRole('list', { name: 'Expeditions in demo' })
    expect(within(list).getByRole('button', { name: /^Opus/ })).toHaveAttribute('aria-current', 'true')
    expect(screen.queryByRole('heading', { name: 'Start here' })).not.toBeInTheDocument()
  })

  it('walks a new user through adding a campaign, then launching', async () => {
    const calls = fakeServer({ campaigns: false })
    render(App)
    expect(await screen.findByRole('button', { name: 'Opus' })).toBeDisabled()
    expect(screen.getByRole('heading', { name: 'Start here' })).toBeInTheDocument()

    await fireEvent.click(screen.getByRole('button', { name: 'Add project' }))
    await fireEvent.input(screen.getByPlaceholderText('/absolute/path/to/project'), { target: { value: '/work/new' } })
    await fireEvent.input(screen.getByPlaceholderText(/defaults to the folder/), { target: { value: 'Fresh' } })
    await fireEvent.click(screen.getByRole('button', { name: 'Add campaign' }))

    expect(await screen.findByText('Fresh', { selector: 'strong' })).toBeInTheDocument()
    expect(calls.find((c) => c.path === '/api/campaigns')?.body).toEqual({ dir: '/work/new', name: 'Fresh' })
    await waitFor(() => expect(screen.getByRole('button', { name: 'Opus' })).toBeEnabled())
    await fireEvent.click(screen.getByRole('button', { name: 'Shell' }))
    expect(await screen.findByTestId('term-stub')).toBeInTheDocument()
  })

  it('shows the server error for a duplicate campaign and keeps the form', async () => {
    fakeServer()
    render(App)
    await screen.findByRole('button', { name: 'Opus' })
    await fireEvent.click(screen.getByRole('button', { name: 'Add project' }))
    await fireEvent.input(screen.getByPlaceholderText('/absolute/path/to/project'), { target: { value: '/dup' } })
    await fireEvent.click(screen.getByRole('button', { name: 'Add campaign' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('already a campaign')
    expect(screen.getByPlaceholderText('/absolute/path/to/project')).toHaveValue('/dup')
  })

  it('keeps several expeditions, switches between them and closes one', async () => {
    const calls = fakeServer()
    render(App)
    await screen.findByRole('button', { name: 'Opus' })
    await fireEvent.click(screen.getByRole('button', { name: 'Opus' }))
    await screen.findByTestId('term-stub')
    await fireEvent.click(screen.getByRole('button', { name: 'Shell' }))
    const list = screen.getByRole('list', { name: 'Expeditions in demo' })
    await waitFor(() => expect(within(list).getAllByRole('listitem')).toHaveLength(2))

    const [first, second] = within(list).getAllByRole('button', { name: /Opus|shell/ }).filter((b) => b.classList.contains('pick'))
    expect(second).toHaveAttribute('aria-current', 'true')
    await fireEvent.click(first)
    expect(first).toHaveAttribute('aria-current', 'true')

    await fireEvent.click(within(list).getByRole('button', { name: 'Close Opus' }))
    await waitFor(() => expect(within(list).getAllByRole('listitem')).toHaveLength(1))
    expect(calls.some((c) => c.method === 'DELETE' && c.path === '/api/sessions/s1')).toBe(true)
  })

  it('shows the server error when a launch is rejected', async () => {
    fakeServer()
    const real = globalThis.fetch as unknown as ReturnType<typeof vi.fn>
    render(App)
    await screen.findByRole('button', { name: 'Opus' })
    real.mockImplementationOnce(async () => ({
      ok: false, status: 400, json: async () => ({ error: 'campaign directory is no longer available' }),
    }))
    await fireEvent.click(screen.getByRole('button', { name: 'Opus' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('no longer available')
    expect(screen.queryByTestId('term-stub')).not.toBeInTheDocument()
  })

  it('marks an expedition exited when its terminal reports an exit', async () => {
    fakeServer()
    render(App)
    await screen.findByRole('button', { name: 'Opus' })
    await fireEvent.click(screen.getByRole('button', { name: 'Shell' }))
    await fireEvent.click(await screen.findByRole('button', { name: 'fake-exit-s1' }))
    expect(await screen.findByText('exit 5')).toBeInTheDocument()
    expect(screen.getByText('exited with code 5')).toBeInTheDocument()
  })

  it('shows a lost expedition without a terminal and lets you dismiss it', async () => {
    const calls = fakeServer({ lost: true })
    render(App)
    expect(await screen.findByText(/terminal is gone/)).toBeInTheDocument()
    expect(screen.queryByTestId('term-stub')).not.toBeInTheDocument()
    expect(screen.getByText('lost (app restarted)')).toBeInTheDocument()
    await fireEvent.click(screen.getByRole('button', { name: 'Close Opus' }))
    await waitFor(() => expect(screen.queryByText(/terminal is gone/)).not.toBeInTheDocument())
    expect(calls.some((c) => c.method === 'DELETE' && c.path === '/api/sessions/old')).toBe(true)
  })

  it('removes a campaign after confirmation', async () => {
    const calls = fakeServer()
    render(App)
    await screen.findByRole('button', { name: 'Opus' })
    await fireEvent.click(screen.getByRole('button', { name: 'Campaign actions for demo' }))
    await fireEvent.click(screen.getByRole('button', { name: 'Remove project' }))
    await fireEvent.click(within(screen.getByRole('group', { name: 'Confirm removal of demo' })).getByRole('button', { name: 'Remove project' }))
    expect(await screen.findByRole('heading', { name: 'Start here' })).toBeInTheDocument()
    expect(calls.some((c) => c.method === 'DELETE' && c.path === '/api/campaigns/c1')).toBe(true)
  })

  it('explains a missing claude install', async () => {
    fakeServer({ claudeFound: false })
    render(App)
    expect(await screen.findByRole('alert')).toHaveTextContent(/Claude Code was not found/)
    expect(screen.getByRole('button', { name: 'Opus' })).toBeDisabled()
  })

  describe('quests', () => {
    it('shows the selected campaign\'s issues in the right panel and opens one', async () => {
      const calls = fakeServer()
      render(App)
      const panel = await screen.findByRole('complementary', { name: 'Quests' })
      expect(await within(panel).findByText('Real first issue')).toBeInTheDocument()
      expect(within(panel).getByText('acme/demo')).toBeInTheDocument()
      expect(calls.some((c) => c.path === '/api/campaigns/c1/quests')).toBe(true)

      const browser = within(panel).getByRole('region', { name: 'Quest browser' })
      await fireEvent.click(within(browser).getByRole('button', { name: /#11 Real blocked issue/ }))
      const detail = within(panel).getByRole('region', { name: 'Selected issue detail' })
      expect(await within(detail).findByRole('heading', { name: /#11 Real blocked issue/ })).toBeInTheDocument()
      expect(within(browser).getByRole('button', { name: /#10 Real first issue/ })).toBeInTheDocument()
      expect(await within(detail).findByRole('heading', { name: 'About #11' })).toBeInTheDocument() // body rendered as markdown
      expect(within(detail).getByText('markdown').tagName).toBe('STRONG')
      expect(within(detail).getByText('a comment')).toBeInTheDocument()

      // Follow the blocker, then go back to the list.
      await fireEvent.click(within(detail).getByRole('button', { name: /#10 Real first issue/ }))
      expect(await within(detail).findByRole('heading', { name: /#10 Real first issue/ })).toBeInTheDocument()
      await fireEvent.click(within(detail).getByRole('button', { name: /← Quests/ }))
      expect(within(browser).getByLabelText('Filter quests')).toBeInTheDocument()
      expect(within(panel).getByText('Select an issue')).toBeInTheDocument()
    })

    it('refreshes with the refresh flag', async () => {
      const calls = fakeServer()
      render(App)
      const panel = await screen.findByRole('complementary', { name: 'Quests' })
      await within(panel).findByText('Real first issue')
      await fireEvent.click(within(panel).getByRole('button', { name: 'Refresh quests' }))
      await waitFor(() => expect(calls.some((c) => c.path === '/api/campaigns/c1/quests?refresh=1')).toBe(true))
    })

    it('offers the separate sample project for a campaign without a repository', async () => {
      const calls = fakeServer({ noRepo: true })
      render(App)
      const panel = await screen.findByRole('complementary', { name: 'Quests' })
      expect(await within(panel).findByRole('alert')).toHaveTextContent('no GitHub repository')

      await fireEvent.click(within(panel).getByRole('button', { name: 'Try the separate sample project' }))
      expect(await within(panel).findByText('Sample quest one')).toBeInTheDocument()
      expect(within(panel).getByText('sample project')).toBeInTheDocument()
      expect(calls.find((c) => c.method === 'POST' && c.path === '/api/onboarding/sample')).toBeDefined()

      expect(within(panel).getByText(/separate sample project/)).toBeInTheDocument()
    })

    it('shows each campaign\'s own quests when switching, never the previous one\'s', async () => {
      fakeServer()
      render(App)
      const panel = await screen.findByRole('complementary', { name: 'Quests' })
      await within(panel).findByText('Real first issue')

      await fireEvent.click(screen.getByRole('button', { name: 'Add project' }))
      await fireEvent.input(screen.getByPlaceholderText('/absolute/path/to/project'), { target: { value: '/work/other' } })
      await fireEvent.click(screen.getByRole('button', { name: 'Add campaign' }))
      // The new campaign has no repo: the real campaign's issues must be gone at once.
      expect(await within(panel).findByRole('alert')).toHaveTextContent('no GitHub repository')
      expect(within(panel).queryByText('Real first issue')).not.toBeInTheDocument()

      await fireEvent.click(within(screen.getByRole('navigation', { name: 'Campaigns' })).getByRole('button', { name: /^demo/ }))
      expect(await within(panel).findByText('Real first issue')).toBeInTheDocument()
    })
  })

  describe('signed out', () => {
    function refuse() {
      vi.stubGlobal('fetch', vi.fn(async () => ({ ok: false, status: 401, json: async () => { throw new Error('no body') } })))
    }

    it('explains itself instead of showing an empty app that looks like lost data', async () => {
      refuse()
      render(App)
      expect(await screen.findByRole('heading', { name: 'This tab is not signed in' })).toBeInTheDocument()
      expect(screen.getByText(/safe: they are stored on/)).toBeInTheDocument()
      expect(screen.queryByText(/Add one to start/)).not.toBeInTheDocument()
      expect(screen.queryByRole('navigation', { name: 'Campaigns' })).not.toBeInTheDocument()
      expect(screen.queryByPlaceholderText('/absolute/path/to/project')).not.toBeInTheDocument()
    })

    it('recovers when "Try again" succeeds after signing in elsewhere', async () => {
      refuse()
      render(App)
      await screen.findByRole('heading', { name: 'This tab is not signed in' })
      fakeServer() // the browser is now signed in
      await fireEvent.click(screen.getByRole('button', { name: 'Try again' }))
      expect(await screen.findByRole('button', { name: 'Opus' })).toBeInTheDocument()
      expect(screen.queryByRole('heading', { name: 'This tab is not signed in' })).not.toBeInTheDocument()
    })
  })

  describe('budget and background polling', () => {
    it('shows the rations next to the quests', async () => {
      fakeServer()
      render(App)
      const panel = await screen.findByRole('complementary', { name: 'Quests' })
      expect(await within(panel).findByText(/GitHub requests: 4,936 of 5,000 left/)).toBeInTheDocument()
    })

    it('re-reads the quests in the background every minute, but not while the page is hidden', async () => {
      vi.useFakeTimers({ toFake: ['setInterval', 'clearInterval'] })
      try {
        const calls = fakeServer()
        render(App)
        const panel = await screen.findByRole('complementary', { name: 'Quests' })
        await within(panel).findByText('Real first issue')
        const polled = () => calls.filter((c) => c.path.includes('background=1')).length
        expect(polled()).toBe(0)

        await vi.advanceTimersByTimeAsync(61_000)
        await waitFor(() => expect(polled()).toBe(1))
        // Never hits the refresh path: background polling lets the server use its cache.
        expect(calls.some((c) => c.path.includes('refresh=1'))).toBe(false)

        Object.defineProperty(document, 'visibilityState', { value: 'hidden', configurable: true })
        await vi.advanceTimersByTimeAsync(130_000)
        expect(polled()).toBe(1) // paused while hidden
        Object.defineProperty(document, 'visibilityState', { value: 'visible', configurable: true })
        document.dispatchEvent(new Event('visibilitychange')) // coming back refreshes once, straight away
        await waitFor(() => expect(polled()).toBe(2))
      } finally {
        Object.defineProperty(document, 'visibilityState', { value: 'visible', configurable: true })
        vi.useRealTimers()
      }
    })

    it('keeps the budget fresh every 30 seconds', async () => {
      vi.useFakeTimers({ toFake: ['setInterval', 'clearInterval'] })
      try {
        const calls = fakeServer()
        render(App)
        await screen.findByRole('complementary', { name: 'Quests' })
        await waitFor(() => expect(calls.filter((c) => c.path === '/api/github').length).toBeGreaterThan(0))
        const before = calls.filter((c) => c.path === '/api/github').length
        await vi.advanceTimersByTimeAsync(31_000)
        await waitFor(() => expect(calls.filter((c) => c.path === '/api/github').length).toBe(before + 1))
      } finally {
        vi.useRealTimers()
      }
    })
  })

  describe('first connection to a repository', () => {
    it('asks which issues to show, then loads them and says what is shown', async () => {
      const calls = fakeServer({ noScope: true })
      render(App)
      const panel = await screen.findByRole('complementary', { name: 'Quests' })
      expect(await within(panel).findByText(/This repository has 312 open issues/)).toBeInTheDocument()
      // Nothing was fetched before the user chose.
      expect(within(panel).queryByText('Real first issue')).not.toBeInTheDocument()
      expect(within(panel).queryByRole('alert')).not.toBeInTheDocument()
      expect(calls.some((c) => c.path.endsWith('/scope/options'))).toBe(true)

      await fireEvent.click(within(panel).getByRole('radio', { name: /Issues labeled tracked/ }))
      await fireEvent.click(within(panel).getByRole('button', { name: 'Show these issues' }))
      expect(await within(panel).findByText('Real first issue')).toBeInTheDocument()
      expect(calls.find((c) => c.method === 'PUT')?.body).toEqual({ mode: 'labels', labels: ['tracked'] })
      expect(within(panel).getByRole('button', { name: /tracked · 2 of 102 open/ })).toBeInTheDocument()
    })

    it('lets the user change the choice later', async () => {
      const calls = fakeServer()
      render(App)
      const panel = await screen.findByRole('complementary', { name: 'Quests' })
      await within(panel).findByText('Real first issue')
      await fireEvent.click(within(panel).getByRole('button', { name: /tracked · / }))
      expect(await within(panel).findByText('Show which issues?')).toBeInTheDocument()
      await fireEvent.click(within(panel).getByRole('radio', { name: /Everything/ }))
      await fireEvent.click(within(panel).getByRole('button', { name: 'Apply' }))
      expect(await within(panel).findByRole('button', { name: /everything · / })).toBeInTheDocument()
      expect(calls.filter((c) => c.method === 'PUT').map((c) => c.body)).toEqual([{ mode: 'all', labels: [] }])
    })
  })
})
