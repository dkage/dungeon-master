import { afterEach, describe, expect, it, vi } from 'vitest'
import { render, waitFor } from '@testing-library/svelte'

vi.mock('./lib/Terminal.svelte', async () => ({
  default: (await import('./test/TerminalStub.svelte')).default,
}))

import App from './App.svelte'
import { AppStore, type Client } from './lib/store.svelte'
import type { SessionInfo } from './lib/api'

const sess = (id: string, over: Partial<SessionInfo> = {}): SessionInfo => ({
  id, kind: 'hero', title: `T-${id}`, campaign: 'c1', dir: '/d', created: '', exited: false, exitCode: 0, ...over,
})

function appWith(sessions: SessionInfo[], later: () => SessionInfo[] = () => sessions) {
  const client = {
    getState: vi.fn(async () => ({
      version: 't', claude: { found: true }, defaultHeroes: false, heroes: [{ id: 'opus', label: 'Opus' }],
      campaigns: [{ id: 'c1', name: 'demo', dir: '/d', remote: 'origin', scope: { mode: 'labels', labels: ['tracked'] }, demo: false }],
      sessions,
    })),
    getSessions: vi.fn(async () => later()),
    getGithub: vi.fn(async () => ({ rations: [], camp: { active: false } })),
    getQuests: vi.fn(async () => ({ quests: [], total: 0 })),
    getProjectSuggestions: vi.fn(async () => []),
  } as unknown as Client
  return new AppStore(client)
}

const iconHref = () => document.querySelector<HTMLLinkElement>('link[rel="icon"]')?.href ?? ''
const iconSvg = () => decodeURIComponent(iconHref())

afterEach(() => {
  document.title = 'Dungeon Master'
  document.querySelector('link[rel="icon"]')?.remove()
})

describe('tab title and icon', () => {
  it('count the sessions waiting for the user while another tab is in front', async () => {
    vi.spyOn(document, 'hasFocus').mockReturnValue(false)
    const store = appWith([sess('a', { activity: 'needs-you' }), sess('b', { activity: 'needs-you' }), sess('c', { activity: 'working' })])
    render(App, { store })
    await waitFor(() => expect(document.title).toBe('(2) Dungeon Master'))
    expect(iconSvg()).toContain('<circle')
  })

  it('clear when the user comes back to the window showing the session that waits', async () => {
    const focus = vi.spyOn(document, 'hasFocus').mockReturnValue(false)
    const store = appWith([sess('a', { activity: 'needs-you' })])
    render(App, { store })
    await waitFor(() => expect(document.title).toBe('(1) Dungeon Master'))
    focus.mockReturnValue(true)
    window.dispatchEvent(new Event('focus'))
    await waitFor(() => expect(document.title).toBe('Dungeon Master'))
    expect(iconSvg()).not.toContain('<circle')
  })

  it('keep counting a session that waits while the user is looking at another one', async () => {
    const focus = vi.spyOn(document, 'hasFocus').mockReturnValue(false)
    const store = appWith([sess('a', { activity: 'working' }), sess('b', { activity: 'needs-you' })])
    render(App, { store })
    await waitFor(() => expect(store.sessions).toHaveLength(2))
    await store.selectSession('a')
    focus.mockReturnValue(true)
    window.dispatchEvent(new Event('focus'))
    await waitFor(() => expect(store.activeId).toBe('a'))
    expect(document.title).toBe('(1) Dungeon Master')
  })

  it('stay plain when nothing waits', async () => {
    vi.spyOn(document, 'hasFocus').mockReturnValue(false)
    const store = appWith([sess('a', { activity: 'working' })])
    render(App, { store })
    await waitFor(() => expect(store.sessions).toHaveLength(1))
    expect(document.title).toBe('Dungeon Master')
  })
})
