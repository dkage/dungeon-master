import { describe, expect, it, vi } from 'vitest'
import { fireEvent, render, screen, within } from '@testing-library/svelte'
import Toolbar from './Toolbar.svelte'
import Sidebar from './Sidebar.svelte'
import { AppStore } from './store.svelte'
import type { Campaign, SessionInfo } from './api'

const camp = (id: string, over: Partial<Campaign> = {}): Campaign => ({
  id, name: `Camp ${id}`, dir: `/p/${id}`, remote: 'origin', labels: [], repo: `acme/${id}`, ...over,
})
const sess = (id: string, campaign: string, over: Partial<SessionInfo> = {}): SessionInfo => ({
  id, kind: 'hero', title: `T-${id}`, campaign, dir: '/d', created: '', exited: false, exitCode: 0, ...over,
})

function store(setup: (s: AppStore) => void = () => {}) {
  const s = new AppStore({
    getState: vi.fn(), createSession: vi.fn(), deleteSession: vi.fn(),
    addCampaign: vi.fn(), updateCampaign: vi.fn(), removeCampaign: vi.fn(),
  })
  s.heroes = [{ id: 'opus', label: 'Opus' }, { id: 'sonnet', label: 'Sonnet' }]
  s.campaigns = [camp('c1'), camp('c2', { repo: undefined })]
  s.campaignId = 'c1'
  setup(s)
  return s
}

describe('Toolbar', () => {
  it('shows the campaign, its repo and one button per hero plus a shell', () => {
    render(Toolbar, { store: store() })
    expect(screen.getByText('Camp c1')).toBeInTheDocument()
    expect(screen.getByText('acme/c1')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Opus' })).toBeEnabled()
    expect(screen.getByRole('button', { name: 'Sonnet' })).toBeEnabled()
    expect(screen.getByRole('button', { name: 'Shell' })).toBeEnabled()
  })

  it('says when the campaign has no GitHub repo', () => {
    render(Toolbar, { store: store((s) => (s.campaignId = 'c2')) })
    expect(screen.getByText('no GitHub repo')).toBeInTheDocument()
  })

  it('launches the clicked hero and the shell', async () => {
    const s = store()
    const launch = vi.spyOn(s, 'launch').mockResolvedValue()
    render(Toolbar, { store: s })
    await fireEvent.click(screen.getByRole('button', { name: 'Sonnet' }))
    expect(launch).toHaveBeenCalledWith({ hero: 'sonnet' })
    await fireEvent.click(screen.getByRole('button', { name: 'Shell' }))
    expect(launch).toHaveBeenLastCalledWith({ shell: true })
  })

  it('disables everything without a campaign', () => {
    render(Toolbar, { store: store((s) => { s.campaigns = []; s.campaignId = null }) })
    expect(screen.getByText('No campaign selected')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Opus' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Shell' })).toBeDisabled()
  })

  it('disables hero buttons and warns when claude is missing, but keeps the shell', () => {
    render(Toolbar, { store: store((s) => (s.claude = { found: false, error: 'not on PATH' })) })
    expect(screen.getByRole('button', { name: 'Opus' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Shell' })).toBeEnabled()
    expect(screen.getByRole('alert')).toHaveTextContent('not on PATH')
  })

  it('disables launching while busy; shows notes and errors', () => {
    render(Toolbar, { store: store((s) => { s.busy = true; s.defaultHeroes = true; s.error = 'broke' }) })
    expect(screen.getByRole('button', { name: 'Opus' })).toBeDisabled()
    expect(screen.getByText(/built-in heroes/)).toBeInTheDocument()
    expect(screen.getByRole('alert')).toHaveTextContent('broke')
  })
})

describe('Sidebar', () => {
  it('lists campaigns with repos, highlights the selected one and selects on click', async () => {
    const s = store()
    render(Sidebar, { store: s })
    const first = screen.getByRole('button', { name: /^Camp c1/ })
    expect(first).toHaveAttribute('aria-current', 'true')
    expect(screen.getByText('acme/c1')).toBeInTheDocument()
    expect(screen.getByText('no GitHub repo')).toBeInTheDocument()
    await fireEvent.click(screen.getByRole('button', { name: /^Camp c2/ }))
    expect(s.campaignId).toBe('c2')
  })

  it('invites you to add a campaign when there are none', () => {
    render(Sidebar, { store: store((s) => { s.campaigns = []; s.campaignId = null }) })
    expect(screen.getByText(/Add one to start/)).toBeInTheDocument()
  })

  it('nests expeditions under their campaign with accessible status', async () => {
    const s = store((st) => {
      st.sessions = [
        sess('a', 'c1'),
        sess('b', 'c1', { exited: true, exitCode: 2 }),
        sess('c', 'c1', { exited: true, exitCode: 0 }),
        sess('d', 'c1', { exited: true, exitCode: -1, lost: true }),
        sess('e', 'c2'),
      ]
      st.activeId = 'a'
    })
    render(Sidebar, { store: s })
    const list = screen.getByRole('list', { name: 'Expeditions in Camp c1' })
    expect(within(list).getAllByRole('listitem')).toHaveLength(4)
    expect(within(list).getByText('running')).toBeInTheDocument()
    expect(within(list).getByText('exited with code 2')).toBeInTheDocument()
    expect(within(list).getByText('exited')).toBeInTheDocument()
    expect(within(list).getByText('lost (app restarted)')).toBeInTheDocument()
    expect(within(list).getByRole('button', { name: /^T-a/ })).toHaveAttribute('aria-current', 'true')
    expect(within(screen.getByRole('list', { name: 'Expeditions in Camp c2' })).getAllByRole('listitem')).toHaveLength(1)

    await fireEvent.click(screen.getByRole('button', { name: /^T-e/ }))
    expect(s.campaignId).toBe('c2')
    expect(s.activeId).toBe('e')
  })

  it('closes an expedition from its row', async () => {
    const s = store((st) => (st.sessions = [sess('a', 'c1')]))
    const close = vi.spyOn(s, 'close').mockResolvedValue()
    render(Sidebar, { store: s })
    await fireEvent.click(screen.getByRole('button', { name: 'Close T-a' }))
    expect(close).toHaveBeenCalledWith('a')
  })

  it('adds a campaign from the form, then clears it; keeps input on failure', async () => {
    const s = store()
    const add = vi.spyOn(s, 'addCampaign').mockResolvedValueOnce(false).mockResolvedValueOnce(true)
    render(Sidebar, { store: s })
    const submit = screen.getByRole('button', { name: 'Add campaign' })
    expect(submit).toBeDisabled()
    const dir = screen.getByPlaceholderText('/absolute/path/to/project')
    await fireEvent.input(dir, { target: { value: '  /work/app  ' } })
    await fireEvent.input(screen.getByPlaceholderText(/defaults to the folder/), { target: { value: 'App' } })
    await fireEvent.click(submit)
    expect(add).toHaveBeenCalledWith({ dir: '/work/app', name: 'App' })
    expect(dir).toHaveValue('  /work/app  ') // failure: nothing lost
    await fireEvent.click(submit)
    expect(dir).toHaveValue('') // success: cleared
  })

  it('omits an empty name', async () => {
    const s = store()
    const add = vi.spyOn(s, 'addCampaign').mockResolvedValue(true)
    render(Sidebar, { store: s })
    await fireEvent.input(screen.getByPlaceholderText('/absolute/path/to/project'), { target: { value: '/x' } })
    await fireEvent.click(screen.getByRole('button', { name: 'Add campaign' }))
    expect(add).toHaveBeenCalledWith({ dir: '/x', name: undefined })
  })

  it('removing a campaign takes two steps and can be cancelled', async () => {
    const s = store()
    const remove = vi.spyOn(s, 'removeCampaign').mockResolvedValue()
    render(Sidebar, { store: s })
    await fireEvent.click(screen.getByRole('button', { name: 'Remove campaign Camp c1' }))
    expect(remove).not.toHaveBeenCalled()
    await fireEvent.click(screen.getByRole('button', { name: 'Cancel removal' }))
    expect(remove).not.toHaveBeenCalled()
    await fireEvent.click(screen.getByRole('button', { name: 'Remove campaign Camp c1' }))
    await fireEvent.click(screen.getByRole('button', { name: 'Remove?' }))
    expect(remove).toHaveBeenCalledWith('c1')
  })
})
