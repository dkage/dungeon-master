import { describe, expect, it, vi } from 'vitest'
import { fireEvent, render, screen } from '@testing-library/svelte'
import Toolbar from './Toolbar.svelte'
import Tabs from './Tabs.svelte'
import { AppStore } from './store.svelte'
import type { SessionInfo } from './api'

function store(over: Partial<AppStore> = {}) {
  const s = new AppStore({
    getState: vi.fn(),
    createSession: vi.fn(),
    deleteSession: vi.fn(),
  })
  s.heroes = [
    { id: 'opus', label: 'Opus' },
    { id: 'sonnet', label: 'Sonnet' },
  ]
  Object.assign(s, over)
  return s
}

const sess = (id: string, over: Partial<SessionInfo> = {}): SessionInfo => ({
  id, kind: 'hero', title: `T-${id}`, dir: `/d/${id}`, created: '', exited: false, exitCode: 0, ...over,
})

describe('Toolbar', () => {
  it('renders one button per hero plus a shell button', () => {
    render(Toolbar, { store: store() })
    expect(screen.getByRole('button', { name: 'Opus' })).toBeEnabled()
    expect(screen.getByRole('button', { name: 'Sonnet' })).toBeEnabled()
    expect(screen.getByRole('button', { name: 'Shell' })).toBeEnabled()
  })

  it('launches the clicked hero', async () => {
    const s = store()
    const launch = vi.spyOn(s, 'launch').mockResolvedValue()
    render(Toolbar, { store: s })
    await fireEvent.click(screen.getByRole('button', { name: 'Sonnet' }))
    expect(launch).toHaveBeenCalledWith({ hero: 'sonnet' })
    await fireEvent.click(screen.getByRole('button', { name: 'Shell' }))
    expect(launch).toHaveBeenLastCalledWith({ shell: true })
  })

  it('typing in the directory field updates the store', async () => {
    const s = store()
    render(Toolbar, { store: s })
    await fireEvent.input(screen.getByPlaceholderText(/absolute/), { target: { value: '/tmp/x' } })
    expect(s.dir).toBe('/tmp/x')
  })

  it('disables hero buttons and warns when claude is missing, but keeps the shell', () => {
    render(Toolbar, { store: store({ claude: { found: false, error: 'not on PATH' } }) })
    expect(screen.getByRole('button', { name: 'Opus' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Shell' })).toBeEnabled()
    expect(screen.getByRole('alert')).toHaveTextContent('not on PATH')
  })

  it('disables launching while busy', () => {
    render(Toolbar, { store: store({ busy: true }) })
    expect(screen.getByRole('button', { name: 'Opus' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Shell' })).toBeDisabled()
  })

  it('shows the default-heroes note and errors', () => {
    render(Toolbar, { store: store({ defaultHeroes: true, error: 'something broke' }) })
    expect(screen.getByText(/built-in heroes/)).toBeInTheDocument()
    expect(screen.getByRole('alert')).toHaveTextContent('something broke')
  })
})

describe('Tabs', () => {
  it('marks the active tab, shows exited badges and reports clicks', async () => {
    const onselect = vi.fn()
    const onclose = vi.fn()
    render(Tabs, {
      sessions: [sess('a'), sess('b', { exited: true, exitCode: 2 })],
      activeId: 'a',
      onselect,
      onclose,
    })
    const tabs = screen.getAllByRole('tab')
    expect(tabs[0]).toHaveAttribute('aria-selected', 'true')
    expect(tabs[1]).toHaveAttribute('aria-selected', 'false')
    expect(screen.getByText('exited 2')).toBeInTheDocument()

    await fireEvent.click(tabs[1])
    expect(onselect).toHaveBeenCalledWith('b')
    await fireEvent.click(screen.getByRole('button', { name: 'Close T-a' }))
    expect(onclose).toHaveBeenCalledWith('a')
  })

  it('shows the directory as a tooltip', () => {
    render(Tabs, { sessions: [sess('a')], activeId: 'a', onselect: vi.fn(), onclose: vi.fn() })
    expect(screen.getByRole('tab')).toHaveAttribute('title', '/d/a')
  })
})
