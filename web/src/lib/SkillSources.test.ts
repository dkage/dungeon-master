import { describe, expect, it, vi } from 'vitest'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte'
import SkillSources from './SkillSources.svelte'
import type { ProjectSkillSource, SkillDiscovery, SkillSource, SkillSourceReport, SkillSourcesState } from './api'

const REV = 'a'.repeat(40)
const folder: SkillSource = { id: 'f1', name: 'Mine', kind: 'folder', location: '/home/me/skills', enabled: true }
const remote: SkillSource = {
  id: 'g1', name: 'Upstream', kind: 'git', location: 'https://example.test/skills.git', pin: 'v1', enabled: true, license: 'MIT', notice: 'Credit to the authors.',
  verified: { revision: REV, at: '2026-10-14T10:00:00Z' },
}
const suggestion = { key: 'upstream', name: 'Matt Pocock Skills', location: 'https://github.com/mattpocock/skills', license: 'MIT', notice: 'MIT licensed; fetched, not bundled.' }

function fake(initial: Partial<SkillSourcesState> = {}, project: ProjectSkillSource[] = []) {
  let book: SkillSourcesState = { sources: [folder, remote], suggestions: [suggestion], reports: [
    { sourceId: 'f1', count: 3, warnings: [] }, { sourceId: 'g1', count: 38, warnings: ['skipped x: SKILL.md is too large'] },
  ], ...initial }
  let proj = project
  const api = {
    getSkillSources: vi.fn(async () => book),
    addSkillSource: vi.fn(async (input: { name?: string }) => { book = { ...book, sources: [...book.sources, { ...folder, id: 'n1', name: input.name ?? '' }] }; return folder }),
    updateSkillSource: vi.fn(async () => folder),
    refreshSkillSource: vi.fn(async () => remote),
    removeSkillSource: vi.fn(async (id: string) => { book = { ...book, sources: book.sources.filter((s) => s.id !== id) } }),
    reorderSkillSources: vi.fn(async (ids: string[]) => { book = { ...book, sources: ids.map((id) => book.sources.find((s) => s.id === id)!) }; return book }),
    getProjectSkillSources: vi.fn(async () => ({ sources: proj })),
    setProjectSkillSources: vi.fn(async (_c: string, list: { sourceId: string; enabled: boolean }[]) => {
      proj = list.length ? list.map((m) => ({ ...(book.sources.find((s) => s.id === m.sourceId)!), enabled: m.enabled, override: true })) : proj.map((s) => ({ ...s, override: false }))
      return { sources: proj }
    }),
    pickFolder: vi.fn(async () => ({ path: '/picked/folder' })),
    getProjectSkills: vi.fn(async () => ({ entries: [], reports: [] as SkillSourceReport[] })),
    setSkillDiscovery: vi.fn(async (d: SkillDiscovery) => { book = { ...book, discovery: d }; return book }),
  }
  return api
}

const open = (api: ReturnType<typeof fake>, props: Record<string, unknown> = {}) =>
  render(SkillSources, { api: api as never, wordMode: 'plain', now: () => Date.parse('2026-10-14T12:00:00Z'), ...props })

describe('SkillSources', () => {
  it('lists each source with its location, ref and short commit, count and warnings', async () => {
    open(fake())
    expect(await screen.findByText('Mine')).toBeInTheDocument()
    expect(screen.getByText('/home/me/skills')).toBeInTheDocument()
    expect(screen.getByText(`v1 · ${REV.slice(0, 12)}`)).toBeInTheDocument()
    expect(screen.getByLabelText('3 skills')).toBeInTheDocument()
    expect(screen.getByLabelText('38 skills')).toBeInTheDocument()
    expect(screen.getByText('skipped x: SKILL.md is too large')).toBeInTheDocument()
    expect(screen.getByText('MIT licence')).toBeInTheDocument()
    expect(screen.getByText('Credit to the authors.')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Skill sources' })).toBeInTheDocument()
  })

  it('uses the themed word for the section title unless words are plain', async () => {
    open(fake(), { wordMode: 'themed' })
    expect(await screen.findByRole('heading', { name: 'Spell sources' })).toBeInTheDocument()
  })

  it('registers a folder from a typed or picked path', async () => {
    const api = fake()
    open(api)
    await fireEvent.click(await screen.findByRole('button', { name: 'New source' }))
    await fireEvent.input(screen.getByLabelText('Name'), { target: { value: 'Team' } })
    await fireEvent.click(screen.getByRole('button', { name: 'Select folder' }))
    await waitFor(() => expect((screen.getByLabelText('Path') as HTMLInputElement).value).toBe('/picked/folder'))
    await fireEvent.click(screen.getByRole('button', { name: 'Register' }))
    await waitFor(() => expect(api.addSkillSource).toHaveBeenCalledWith({ name: 'Team', kind: 'folder', location: '/picked/folder', pin: undefined }))
    expect(await screen.findByText('Team')).toBeInTheDocument()
  })

  it('registers a Git repository with a blank ref, which follows the default branch', async () => {
    const api = fake()
    open(api)
    await fireEvent.click(await screen.findByRole('button', { name: 'New source' }))
    await fireEvent.input(screen.getByLabelText('Name'), { target: { value: 'Remote' } })
    await fireEvent.change(screen.getByLabelText('Kind'), { target: { value: 'git' } })
    await fireEvent.input(screen.getByLabelText('URL'), { target: { value: 'https://example.test/a.git' } })
    expect(screen.getByPlaceholderText('The default branch')).toBeInTheDocument()
    await fireEvent.click(screen.getByRole('button', { name: 'Register' }))
    await waitFor(() => expect(api.addSkillSource).toHaveBeenCalledWith({ name: 'Remote', kind: 'git', location: 'https://example.test/a.git', pin: '' }))
  })

  it('offers the suggested source with its notice and adds it only on request', async () => {
    const api = fake()
    open(api)
    await fireEvent.click(await screen.findByRole('button', { name: 'Add Matt Pocock Skills…' }))
    expect(api.addSkillSource).not.toHaveBeenCalled()
    expect(screen.getByText('MIT licensed; fetched, not bundled.')).toBeInTheDocument()
    expect((screen.getByLabelText('URL') as HTMLInputElement).readOnly).toBe(true)
    await fireEvent.input(screen.getByLabelText('Ref'), { target: { value: 'main' } })
    await fireEvent.click(screen.getByRole('button', { name: 'Register' }))
    await waitFor(() => expect(api.addSkillSource).toHaveBeenCalledWith({ name: 'Matt Pocock Skills', suggestion: 'upstream', pin: 'main' }))
  })

  it('does not offer a suggestion that is already registered', async () => {
    open(fake({ suggestions: [{ ...suggestion, location: remote.location }] }))
    await screen.findByText('Mine')
    expect(screen.queryByRole('button', { name: /^Add Matt/ })).not.toBeInTheDocument()
  })

  it('shows a failed refresh with its age and the version still in use', async () => {
    const failed = { ...remote, error: { message: 'network is down', at: '2026-10-12T12:00:00Z' } }
    open(fake({ sources: [failed] }))
    expect(await screen.findByRole('status')).toHaveTextContent('Last refresh failed 2 days ago: network is down. Showing the version from 2 h ago.')
  })

  it('refuses an empty name and reports the server\'s refusal', async () => {
    const api = fake()
    api.addSkillSource.mockRejectedValueOnce(new Error('could not fetch the source: repository not found'))
    open(api)
    await fireEvent.click(await screen.findByRole('button', { name: 'New source' }))
    await fireEvent.click(screen.getByRole('button', { name: 'Register' }))
    expect(screen.getByRole('alert')).toHaveTextContent('Give the source a name.')
    await fireEvent.input(screen.getByLabelText('Name'), { target: { value: 'x' } })
    await fireEvent.input(screen.getByLabelText('Path'), { target: { value: '/nope' } })
    await fireEvent.click(screen.getByRole('button', { name: 'Register' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('repository not found')
    expect(screen.getByRole('group', { name: 'Register a source' })).toBeInTheDocument() // the draft is kept
  })

  it('enables, refreshes and reorders sources', async () => {
    const api = fake()
    open(api)
    await fireEvent.click(await screen.findByLabelText('Use Mine'))
    await waitFor(() => expect(api.updateSkillSource).toHaveBeenCalledWith('f1', { enabled: false }))
    await fireEvent.click(screen.getByRole('button', { name: 'Refresh' }))
    await waitFor(() => expect(api.refreshSkillSource).toHaveBeenCalledWith('g1'))
    expect(screen.getByRole('button', { name: 'Move Mine up' })).toBeDisabled()
    await fireEvent.click(screen.getByRole('button', { name: 'Move Mine down' }))
    await waitFor(() => expect(api.reorderSkillSources).toHaveBeenCalledWith(['g1', 'f1']))
    const names = () => within(screen.getByRole('list', { name: 'Registered sources' })).getAllByRole('listitem').map((li) => li.querySelector('strong')?.textContent)
    await waitFor(() => expect(names()).toEqual(['Upstream', 'Mine']))
  })

  it('asks before removing, and says what is kept', async () => {
    const api = fake()
    open(api)
    await fireEvent.click((await screen.findAllByRole('button', { name: 'Remove' }))[0])
    expect(api.removeSkillSource).not.toHaveBeenCalled()
    expect(screen.getByText(/Your folder is not touched/)).toBeInTheDocument()
    await fireEvent.click(screen.getByRole('button', { name: 'Keep it' }))
    expect(api.removeSkillSource).not.toHaveBeenCalled()
    await fireEvent.click(screen.getAllByRole('button', { name: 'Remove' })[0])
    await fireEvent.click(screen.getByRole('button', { name: 'Remove source' }))
    await waitFor(() => expect(api.removeSkillSource).toHaveBeenCalledWith('f1'))
    await waitFor(() => expect(screen.queryByText('Mine')).not.toBeInTheDocument())
  })

  it('edits a source with its own values and sends only what changed', async () => {
    const api = fake()
    open(api)
    await fireEvent.click((await screen.findAllByRole('button', { name: 'Edit' }))[1])
    expect((screen.getByLabelText('URL') as HTMLInputElement).value).toBe(remote.location)
    expect(screen.queryByLabelText('Kind')).not.toBeInTheDocument()
    await fireEvent.input(screen.getByLabelText('Ref'), { target: { value: 'v2' } })
    await fireEvent.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() => expect(api.updateSkillSource).toHaveBeenCalledWith('g1', { name: 'Upstream', pin: 'v2' }))
  })

  it('gives a project its own list on first change and returns to the defaults on request', async () => {
    const proj: ProjectSkillSource[] = [{ ...folder, override: false }, { ...remote, override: false }]
    const api = fake({}, proj)
    open(api, { campaignId: 'c1', campaignName: 'Camp' })
    const group = await screen.findByRole('group', { name: 'Sources for Camp' })
    expect(within(group).getByText(/Using the defaults/)).toBeInTheDocument()
    expect(within(group).getByRole('button', { name: 'Use defaults' })).toBeDisabled()
    await fireEvent.click(within(group).getByLabelText('Use Upstream in Camp'))
    await waitFor(() => expect(api.setProjectSkillSources).toHaveBeenCalledWith('c1', [{ sourceId: 'f1', enabled: true }, { sourceId: 'g1', enabled: false }]))
    expect(await within(group).findByText(/This project has its own list/)).toBeInTheDocument()
    await fireEvent.click(within(group).getByRole('button', { name: 'Use defaults' }))
    await waitFor(() => expect(api.setProjectSkillSources).toHaveBeenLastCalledWith('c1', []))
  })

  it('reorders a project\'s sources without touching the global order', async () => {
    const proj: ProjectSkillSource[] = [{ ...folder, override: false }, { ...remote, override: false }]
    const api = fake({}, proj)
    open(api, { campaignId: 'c1', campaignName: 'Camp' })
    await fireEvent.click(await screen.findByRole('button', { name: 'Move Mine down in Camp' }))
    await waitFor(() => expect(api.setProjectSkillSources).toHaveBeenCalledWith('c1', [{ sourceId: 'g1', enabled: true }, { sourceId: 'f1', enabled: true }]))
    expect(api.reorderSkillSources).not.toHaveBeenCalled()
  })

  it('shows no project list when there is no project', async () => {
    open(fake())
    await screen.findByText('Mine')
    expect(screen.queryByRole('group', { name: /Sources for/ })).not.toBeInTheDocument()
  })
})

describe('SkillSources when the server cannot be reached', () => {
  it('says so quietly and keeps the section usable', async () => {
    const api = fake()
    api.getSkillSources.mockRejectedValueOnce(new Error('request failed (500)'))
    open(api)
    expect(await screen.findByRole('status')).toHaveTextContent('request failed (500)')
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    await fireEvent.click(screen.getByRole('button', { name: 'Rescan' }))
    expect(await screen.findByText('Mine')).toBeInTheDocument()
  })
})

describe('SkillSources overlapping reloads', () => {
  it('lets only the newest reload write its answer', async () => {
    const api = fake()
    const first = [folder, remote]
    let release!: () => void
    const slow = new Promise<void>((r) => { release = r })
    // The first reload is slow and shows two sources; the second is quick and shows one.
    api.getSkillSources
      .mockImplementationOnce(async () => ({ sources: first, suggestions: [], reports: [] }))
      .mockImplementationOnce(async () => { await slow; return { sources: first, suggestions: [], reports: [] } })
      .mockImplementationOnce(async () => ({ sources: [remote], suggestions: [], reports: [] }))
    open(api)
    await screen.findByText('Mine')
    await fireEvent.click(screen.getByRole('button', { name: 'Rescan' })) // slow, started first
    await fireEvent.click(screen.getByRole('button', { name: 'Rescan' })) // quick, started second
    await waitFor(() => expect(screen.queryByText('Mine')).not.toBeInTheDocument())
    release()
    await new Promise((r) => setTimeout(r, 20))
    expect(screen.queryByText('Mine')).not.toBeInTheDocument() // the stale answer did not bring it back
  })
})

describe('SkillSources with empty answers', () => {
  it('copes with null lists from the server and stays usable', async () => {
    const api = fake()
    api.getSkillSources.mockResolvedValue({ sources: null, suggestions: null, reports: null } as never)
    api.getProjectSkillSources.mockResolvedValue({ sources: null } as never)
    open(api, { campaignId: 'c1', campaignName: 'Camp' })
    expect(await screen.findByText(/No sources yet/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'New source' })).toBeEnabled()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })
})

describe('SkillSources discovery and notes', () => {
  it('turns supplementary places on one at a time and lists what was found for the project', async () => {
    const api = fake()
    api.getProjectSkills.mockResolvedValue({ entries: [], reports: [
      { sourceId: 'f1', name: 'Mine', origin: 'registered', count: 3, warnings: [] },
      { sourceId: 'user-claude', name: 'Your skills (~/.claude/skills)', origin: 'user', count: 2, warnings: [],
        flagged: [{ name: 'publish', notes: ['Mentions creating or changing GitHub issues or pull requests.'] }] },
    ] })
    open(api, { campaignId: 'c1', campaignName: 'Camp' })
    const box = await screen.findByLabelText(/Your skill folders/)
    expect(box).not.toBeChecked() // off until chosen
    expect(screen.getByLabelText(/The project's own skill folders/)).not.toBeChecked()
    await fireEvent.click(box)
    await waitFor(() => expect(api.setSkillDiscovery).toHaveBeenCalledWith({ project: false, user: true, plugins: false }))
    await waitFor(() => expect(screen.getByLabelText(/Your skill folders/)).toBeChecked())
    const found = await screen.findByRole('list', { name: 'Found automatically for Camp' })
    expect(within(found).getByText('Your skills (~/.claude/skills)')).toBeInTheDocument()
    expect(within(found).getByText('user')).toBeInTheDocument()
    expect(within(found).getByText(/publish: Mentions creating or changing GitHub issues/)).toBeInTheDocument()
    expect(within(found).queryByText('Mine')).not.toBeInTheDocument() // registered sources are listed above, not here
  })

  it('shows a source\'s compatibility notes as hints that never block', async () => {
    const api = fake({ reports: [
      { sourceId: 'f1', count: 3, warnings: [], flagged: [{ name: 'to-tickets', notes: ['Appears to keep issues or tickets in local files.', 'Ships scripts.'] }] },
      { sourceId: 'g1', count: 1, warnings: [] },
    ] })
    open(api)
    const summary = await screen.findByText('Compatibility notes (1)')
    await fireEvent.click(summary)
    expect(screen.getByText('to-tickets')).toBeVisible()
    expect(screen.getByText('Appears to keep issues or tickets in local files.')).toBeInTheDocument()
    expect(screen.getByText(/never stop a launch/)).toBeInTheDocument()
    expect(screen.getAllByText(/Compatibility notes/)).toHaveLength(1) // none for the source without notes
  })
})
