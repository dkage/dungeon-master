import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte'

vi.mock('./lib/Terminal.svelte', async () => ({
  default: (await import('./test/TerminalStub.svelte')).default,
}))

import App from './App.svelte'

interface Call { method: string; path: string; body?: unknown }

// A tiny in-memory server behind fetch, so the real api client and store run.
function fakeServer(opts: { claudeFound?: boolean; campaigns?: boolean; lost?: boolean } = {}) {
  const calls: Call[] = []
  let next = 0
  const campaigns: Record<string, unknown>[] =
    opts.campaigns === false ? [] : [{ id: 'c1', name: 'demo', dir: '/work/demo', remote: 'origin', labels: [], repo: 'acme/demo' }]
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
    if (method === 'POST' && path === '/api/campaigns') {
      if (body.dir === '/dup') return json(409, { error: 'that directory is already a campaign' })
      const c = { id: `c${campaigns.length + 1}`, name: body.name ?? 'new', dir: body.dir, remote: 'origin', labels: [] }
      campaigns.push(c)
      return json(201, c)
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
    expect(screen.getByText(/Pick a hero/)).toBeInTheDocument()

    await fireEvent.click(screen.getByRole('button', { name: 'Opus' }))
    expect(await screen.findByTestId('term-stub')).toHaveTextContent('s1')
    expect(calls.find((c) => c.method === 'POST' && c.path === '/api/sessions')?.body).toEqual({ hero: 'opus', campaign: 'c1' })
    const list = screen.getByRole('list', { name: 'Expeditions in demo' })
    expect(within(list).getByRole('button', { name: /^Opus/ })).toHaveAttribute('aria-current', 'true')
    expect(screen.queryByText(/Pick a hero/)).not.toBeInTheDocument()
  })

  it('walks a new user through adding a campaign, then launching', async () => {
    const calls = fakeServer({ campaigns: false })
    render(App)
    expect(await screen.findByRole('button', { name: 'Opus' })).toBeDisabled()
    expect(screen.getByText(/Add a campaign on the left/)).toBeInTheDocument()

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
    expect(screen.getByText('lost (Dungeon restarted)')).toBeInTheDocument()
    await fireEvent.click(screen.getByRole('button', { name: 'Close Opus' }))
    await waitFor(() => expect(screen.queryByText(/terminal is gone/)).not.toBeInTheDocument())
    expect(calls.some((c) => c.method === 'DELETE' && c.path === '/api/sessions/old')).toBe(true)
  })

  it('removes a campaign after confirmation', async () => {
    const calls = fakeServer()
    render(App)
    await screen.findByRole('button', { name: 'Opus' })
    await fireEvent.click(screen.getByRole('button', { name: 'Remove campaign demo' }))
    await fireEvent.click(screen.getByRole('button', { name: 'Remove?' }))
    expect(await screen.findByText(/Add a campaign on the left/)).toBeInTheDocument()
    expect(calls.some((c) => c.method === 'DELETE' && c.path === '/api/campaigns/c1')).toBe(true)
  })

  it('explains a missing claude install', async () => {
    fakeServer({ claudeFound: false })
    render(App)
    expect(await screen.findByRole('alert')).toHaveTextContent(/Claude Code was not found/)
    expect(screen.getByRole('button', { name: 'Opus' })).toBeDisabled()
  })
})
