import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { fireEvent, render, screen, waitFor } from '@testing-library/svelte'

vi.mock('./lib/Terminal.svelte', async () => ({
  default: (await import('./test/TerminalStub.svelte')).default,
}))

import App from './App.svelte'

interface Call { method: string; path: string; body?: unknown }

// A tiny in-memory server behind fetch, so the real api client and store run.
function fakeServer(opts: { claudeFound?: boolean } = {}) {
  const calls: Call[] = []
  let next = 0
  const sessions: Record<string, unknown>[] = []
  vi.stubGlobal('fetch', vi.fn(async (path: string, init: RequestInit = {}) => {
    const method = init.method ?? 'GET'
    const body = init.body ? JSON.parse(init.body as string) : undefined
    calls.push({ method, path, body })
    const json = (status: number, data?: unknown) => ({
      ok: status < 300, status, json: async () => data,
    })
    if (method === 'GET' && path === '/api/state') {
      return json(200, {
        version: 'test',
        claude: { found: opts.claudeFound ?? true },
        defaultHeroes: false,
        heroes: [{ id: 'opus', label: 'Opus' }],
        sessions,
      })
    }
    if (method === 'POST' && path === '/api/sessions') {
      if (body.dir === '/missing') return json(400, { error: 'dir is not an existing directory' })
      const s = {
        id: `s${++next}`, kind: body.shell ? 'shell' : 'hero', hero: body.hero,
        title: body.shell ? 'shell' : 'Opus', dir: body.dir, created: '', exited: false, exitCode: 0,
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
  it('loads heroes, launches a session in the chosen directory, and shows its terminal', async () => {
    const calls = fakeServer()
    render(App)
    await screen.findByRole('button', { name: 'Opus' })
    expect(screen.getByText(/Pick a hero/)).toBeInTheDocument()

    await fireEvent.input(screen.getByPlaceholderText(/absolute/), { target: { value: '/work/app' } })
    await fireEvent.click(screen.getByRole('button', { name: 'Opus' }))

    expect(await screen.findByTestId('term-stub')).toHaveTextContent('s1')
    expect(calls.find((c) => c.method === 'POST')?.body).toEqual({ hero: 'opus', dir: '/work/app' })
    expect(screen.getByRole('tab', { name: /Opus/ })).toHaveAttribute('aria-selected', 'true')
    expect(screen.queryByText(/Pick a hero/)).not.toBeInTheDocument()
  })

  it('keeps several sessions, switches between them and closes one', async () => {
    const calls = fakeServer()
    render(App)
    await screen.findByRole('button', { name: 'Opus' })
    await fireEvent.input(screen.getByPlaceholderText(/absolute/), { target: { value: '/p' } })
    await fireEvent.click(screen.getByRole('button', { name: 'Opus' }))
    await waitFor(() => expect(screen.getAllByRole('tab')).toHaveLength(1))
    await fireEvent.click(screen.getByRole('button', { name: 'Shell' }))
    await waitFor(() => expect(screen.getAllByRole('tab')).toHaveLength(2))

    const [first, second] = screen.getAllByRole('tab')
    expect(second).toHaveAttribute('aria-selected', 'true')
    await fireEvent.click(first)
    expect(first).toHaveAttribute('aria-selected', 'true')

    await fireEvent.click(screen.getAllByRole('button', { name: /^Close/ })[0])
    await waitFor(() => expect(screen.getAllByRole('tab')).toHaveLength(1))
    expect(calls.some((c) => c.method === 'DELETE' && c.path === '/api/sessions/s1')).toBe(true)
    expect(screen.getByRole('tab')).toHaveAttribute('aria-selected', 'true')
  })

  it('shows the server error when the directory is rejected', async () => {
    fakeServer()
    render(App)
    await screen.findByRole('button', { name: 'Opus' })
    await fireEvent.input(screen.getByPlaceholderText(/absolute/), { target: { value: '/missing' } })
    await fireEvent.click(screen.getByRole('button', { name: 'Opus' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('dir is not an existing directory')
    expect(screen.queryAllByRole('tab')).toHaveLength(0)
  })

  it('marks a tab exited when its terminal reports an exit', async () => {
    fakeServer()
    render(App)
    await screen.findByRole('button', { name: 'Opus' })
    await fireEvent.input(screen.getByPlaceholderText(/absolute/), { target: { value: '/p' } })
    await fireEvent.click(screen.getByRole('button', { name: 'Shell' }))
    await fireEvent.click(await screen.findByRole('button', { name: 'fake-exit-s1' }))
    expect(await screen.findByText('exited 5')).toBeInTheDocument()
  })

  it('explains a missing claude install', async () => {
    fakeServer({ claudeFound: false })
    render(App)
    expect(await screen.findByRole('alert')).toHaveTextContent(/Claude Code was not found/)
    expect(screen.getByRole('button', { name: 'Opus' })).toBeDisabled()
  })
})
