import { DEFAULT_SETTINGS } from './appearance'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { fireEvent, render, screen, within } from '@testing-library/svelte'
import { tick } from 'svelte'
import Toolbar from './Toolbar.svelte'
import Sidebar from './Sidebar.svelte'
import { AppStore } from './store.svelte'
import type { Campaign, Quest, QuestDetail as QuestDetailT, QuestResult, SessionInfo } from './api'
import QuestsPanel from './QuestsPanel.svelte'
import QuestDetail from './QuestDetail.svelte'
import Rations from './Rations.svelte'
import type { GithubStatus, ScopeOptions } from './api'
import ScopePicker from './ScopePicker.svelte'
import Term from './Term.svelte'
import SettingsDialog from './SettingsDialog.svelte'

const camp = (id: string, over: Partial<Campaign> = {}): Campaign => ({
  id, name: `Camp ${id}`, dir: `/p/${id}`, remote: 'origin', scope: { mode: 'labels', labels: ['tracked'] }, demo: false, repo: `acme/${id}`, ...over,
})
const sess = (id: string, campaign: string, over: Partial<SessionInfo> = {}): SessionInfo => ({
  id, kind: 'hero', title: `T-${id}`, campaign, dir: '/d', created: '', exited: false, exitCode: 0, ...over,
})

function store(setup: (s: AppStore) => void = () => {}) {
  const s = new AppStore({
    getState: vi.fn(), getSessions: vi.fn(async () => []), createSession: vi.fn(), deleteSession: vi.fn(),
    addCampaign: vi.fn(), updateCampaign: vi.fn(), removeCampaign: vi.fn(),
    getQuests: vi.fn(), getQuest: vi.fn(), previewQuestLaunch: vi.fn(async () => ({
      content: '# Task briefing\n\nReviewable context.', truncated: false, commentsShown: 1, commentsTotal: 1,
      canStart: true, hero: 'opus', heroLabel: 'Opus', directory: '/p/c1', briefingId: 'preview-1',
      briefingFile: '/state/briefings/c1/3-opus-preview-1.md', opener: 'Read the briefing.',
      systemPrompt: 'Keep to the issue.', command: ['claude', '--model', 'm1'],
    })), getGithub: vi.fn(), setScope: vi.fn(), getScopeOptions: vi.fn(),
    getProjectSuggestions: vi.fn(async () => []), createSampleCampaign: vi.fn(async () => camp('sample', { sample: true, demo: true })),
    pickFolder: vi.fn(async () => ({ cancelled: true })), openCampaignFolder: vi.fn(async () => undefined),
    adoptStarterHeroes: vi.fn(async () => ({ saved: true })),
    getHeroesConfig: vi.fn(async () => ({ defaultHero: 'opus', heroes: [{ id: 'opus', label: 'Opus', args: ['--model', 'm1'], env: ['API_KEY='] }] })),
    saveHeroesConfig: vi.fn(async () => ({ saved: true })),
    setEffortLabels: vi.fn(async (id) => camp(id)), setEffortOverride: vi.fn(async (id) => camp(id)),
    getAppearance: vi.fn(async () => ({ settings: DEFAULT_SETTINGS, stored: false })),
    saveAppearance: vi.fn(async (s) => ({ settings: { ...DEFAULT_SETTINGS, ...s }, stored: true })),
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

  it('shows a centre quest chip for a linked session and opens that issue', async () => {
    const s = store((st) => {
      st.sessions = [sess('linked', 'c1', { quest: 12, title: '#12 Fix the map' })]
      st.activeId = 'linked'
    })
    const select = vi.spyOn(s, 'selectQuest').mockResolvedValue()
    render(Toolbar, { store: s })
    await fireEvent.click(screen.getByRole('button', { name: '#12' }))
    expect(s.questsOpen).toBe(true)
    expect(select).toHaveBeenCalledWith(12)
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

  it('disables launching while busy; shows starter action and errors', () => {
    render(Toolbar, { store: store((s) => { s.busy = true; s.defaultHeroes = true; s.error = 'broke' }) })
    expect(screen.getByRole('button', { name: 'Opus' })).toBeDisabled()
    expect(screen.getByText(/built-in heroes/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Keep these starter heroes' })).toBeDisabled()
    expect(screen.getByRole('alert')).toHaveTextContent('broke')
  })

  it('adopts built-in starter heroes from the in-app action', async () => {
    const s = store((st) => (st.defaultHeroes = true))
    const adopt = vi.spyOn(s, 'adoptStarterHeroes').mockResolvedValue(true)
    render(Toolbar, { store: s })
    await fireEvent.click(screen.getByRole('button', { name: 'Keep these starter heroes' }))
    expect(adopt).toHaveBeenCalledOnce()
  })
})

describe('first-run terminology and appearance', () => {
  it('explains themed words on tap and closes the definition with Escape', async () => {
    render(Term, { word: 'quest', mode: 'hints' })
    const button = screen.getByRole('button', { name: /Quest: A GitHub issue/ })
    expect(button).toHaveTextContent('Quest(issue)')
    await fireEvent.click(button)
    expect(screen.getByRole('status')).toHaveTextContent('A GitHub issue you can send an agent to work on.')
    await fireEvent.keyDown(button, { key: 'Escape' })
    expect(screen.queryByRole('status')).not.toBeInTheDocument()
  })

  it('changes and persists font and independent panel sizes from Settings', async () => {
    const s = store()
    const onclose = vi.fn()
    render(SettingsDialog, { store: s, onclose })
    expect(screen.getByRole('dialog', { name: 'Settings' })).toBeInTheDocument()
    await fireEvent.change(screen.getByLabelText('Reading font'), { target: { value: 'alegreya' } })
    await fireEvent.input(screen.getByLabelText('Campaigns text size'), { target: { value: '18' } })
    await fireEvent.input(screen.getByLabelText('Terminal text size'), { target: { value: '11' } })
    await fireEvent.input(screen.getByLabelText('Quests text size'), { target: { value: '20' } })
    expect(s.appearance).toMatchObject({ font: 'alegreya', sidebarSize: 18, terminalSize: 11, questsSize: 20 })
    await fireEvent.click(screen.getByRole('button', { name: 'Done' }))
    expect(onclose).toHaveBeenCalled()
  })

  it('scrolls only the body, so the header, footer and frame stay still', () => {
    render(SettingsDialog, { store: store(), onclose: vi.fn() })
    const body = screen.getByTestId('settings-body')
    const dialog = body.closest('dialog')!
    expect(body.querySelectorAll('section').length).toBeGreaterThan(1)
    expect(dialog.querySelector('header')!.parentElement).toBe(dialog)
    expect(dialog.querySelector('footer')!.parentElement).toBe(dialog)
    expect(body.contains(dialog.querySelector('header'))).toBe(false)
  })

  it('loads and edits local hero settings without exposing saved environment values', async () => {
    const s = store()
    const save = vi.spyOn(s, 'saveHeroesConfig').mockResolvedValue(true)
    render(SettingsDialog, { store: s, onclose: vi.fn() })
    expect(await screen.findByRole('cell', { name: 'Opus' })).toBeInTheDocument()
    await fireEvent.click(screen.getByRole('button', { name: 'Edit' }))
    expect(screen.getByPlaceholderText('Saved on this computer; leave blank to keep it')).toBeInTheDocument()
    await fireEvent.input(screen.getByLabelText('Model'), { target: { value: 'claude-opus-5-5' } })
    await fireEvent.click(screen.getByRole('button', { name: 'Save hero' }))
    expect(save).toHaveBeenCalledWith(expect.objectContaining({
      defaultHero: 'opus',
      heroes: [expect.objectContaining({ id: 'opus', args: ['--model', 'claude-opus-5-5'], env: ['API_KEY='] })],
    }))
    expect(screen.queryByRole('group', { name: 'Edit Opus' })).not.toBeInTheDocument()
  })
})

describe('Sidebar', () => {
  it('opens the add form without a native dialog, and only picks on Choose folder', async () => {
    const s = store()
    const pick = vi.spyOn(s, 'pickFolder').mockResolvedValue('/work/new-project')
    render(Sidebar, { store: s })
    await fireEvent.click(screen.getByRole('button', { name: 'Add project' }))
    expect(screen.getByPlaceholderText('/absolute/path/to/project')).toBeInTheDocument()
    expect(pick).not.toHaveBeenCalled()
    await fireEvent.click(screen.getByRole('button', { name: /Choose folder/ }))
    expect(pick).toHaveBeenCalledOnce()
    expect(screen.getByPlaceholderText('/absolute/path/to/project')).toHaveValue('/work/new-project')
  })
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

  it('searches campaign names, repositories, and their sessions', async () => {
    const s = store((st) => { st.sessions = [sess('build', 'c2', { title: 'Build goblin room' })] })
    render(Sidebar, { store: s })
    const search = screen.getByRole('searchbox', { name: 'Search projects and sessions' })
    await fireEvent.input(search, { target: { value: 'goblin' } })
    expect(screen.getByRole('button', { name: /^Camp c2/ })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /^Camp c1/ })).not.toBeInTheDocument()
    await fireEvent.input(search, { target: { value: 'acme/c1' } })
    expect(screen.getByRole('button', { name: /^Camp c1/ })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /^Camp c2/ })).not.toBeInTheDocument()
  })

  it('invites you to add a campaign when there are none', () => {
    render(Sidebar, { store: store((s) => { s.campaigns = []; s.campaignId = null }) })
    expect(screen.getByText(/Add one to start/)).toBeInTheDocument()
    expect(screen.queryByPlaceholderText('/absolute/path/to/project')).not.toBeInTheDocument()
  })

  it('closes an idle or finished session at once, and asks before stopping one that is mid-work', async () => {
    const s = store((st) => {
      st.sessions = [
        sess('idle', 'c1', { title: 'Idle one', activity: 'idle' }),
        sess('busy', 'c1', { title: 'Busy one', activity: 'working' }),
        sess('wait', 'c1', { title: 'Waiting one', activity: 'needs-you' }),
      ]
    })
    const close = vi.spyOn(s, 'close').mockResolvedValue()
    render(Sidebar, { store: s })

    await fireEvent.click(screen.getByRole('button', { name: 'Close Idle one' }))
    expect(close).toHaveBeenCalledWith('idle')

    await fireEvent.click(screen.getByRole('button', { name: 'Close Busy one' }))
    expect(close).toHaveBeenCalledTimes(1) // not yet: it asked
    const ask = screen.getByRole('group', { name: 'Confirm stopping Busy one' })
    await fireEvent.click(within(ask).getByRole('button', { name: 'Keep running' }))
    expect(close).toHaveBeenCalledTimes(1)
    expect(screen.queryByRole('group', { name: /Confirm stopping/ })).not.toBeInTheDocument()

    await fireEvent.click(screen.getByRole('button', { name: 'Close Waiting one' }))
    await fireEvent.click(within(screen.getByRole('group', { name: 'Confirm stopping Waiting one' })).getByRole('button', { name: 'Stop and close' }))
    expect(close).toHaveBeenLastCalledWith('wait')
  })

  it('sends the stop request when the close button of a live session is confirmed', async () => {
    const s = store((st) => { st.sessions = [sess('run', 'c1', { title: 'Runner', activity: 'working' })] })
    const del = vi.fn(async () => {})
    ;(s as unknown as { client: { deleteSession: typeof del } }).client.deleteSession = del
    render(Sidebar, { store: s })
    await fireEvent.click(screen.getByRole('button', { name: 'Close Runner' }))
    await fireEvent.click(screen.getByRole('button', { name: 'Stop and close' }))
    await tick()
    expect(del).toHaveBeenCalledWith('run')
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
    expect(within(list).getAllByText('working').length).toBeGreaterThan(0)
    expect(within(list).getByText('exited with code 2')).toBeInTheDocument()
    expect(within(list).getByText('exited')).toBeInTheDocument()
    expect(within(list).getByText('lost (app restarted)')).toBeInTheDocument()
    expect(within(list).getByRole('button', { name: /^T-a/ })).toHaveAttribute('aria-current', 'true')
    expect(within(screen.getByRole('list', { name: 'Expeditions in Camp c2' })).getAllByRole('listitem')).toHaveLength(1)

    await fireEvent.click(screen.getByRole('button', { name: /^T-e/ }))
    expect(s.campaignId).toBe('c2')
    expect(s.activeId).toBe('e')
  })

  it('names a quest session by its issue and hero, and a shell as a scratch terminal', () => {
    const s = store((st) => {
      st.sessions = [
        sess('q', 'c1', { quest: 7, hero: 'sonnet', title: '#7 Signup form' }),
        sess('f', 'c1', { kind: 'shell', title: 'shell' }),
        sess('o', 'c1', { hero: 'opus', title: 'Opus' }),
      ]
    })
    render(Sidebar, { store: s })
    const list = screen.getByRole('list', { name: 'Expeditions in Camp c1' })
    expect(within(list).getByRole('button', { name: /^#7 Signup form \(Sonnet\)/ })).toBeInTheDocument()
    expect(within(list).getByRole('button', { name: /^Scratch terminal/ })).toBeInTheDocument()
    // A free hero session keeps its own title and does not repeat the hero.
    expect(within(list).getByRole('button', { name: /^Opus/ })).not.toHaveTextContent('(Opus)')
    expect(within(list).getByRole('button', { name: 'Close shell' })).toBeInTheDocument()
  })

  it('puts the sample badge beside the project name and the footer actions in two columns', () => {
    const s = store((st) => {
      st.campaigns = [{ id: 'c1', name: 'Demo', dir: '/d', sample: true } as never]
    })
    render(Sidebar, { store: s })
    const line = screen.getByText('Demo').closest('.cline')!
    expect(line).toHaveTextContent('Sample')
    const footer = document.querySelector('.sidebar-footer')!
    expect(within(footer as HTMLElement).getAllByRole('button').map((b) => b.textContent)).toEqual(['＋ Add project', '⚙ Settings', 'Word guide', 'Getting started'])
    expect(screen.getByPlaceholderText('Jump to project or session…')).toBeInTheDocument()
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
    await fireEvent.click(screen.getByRole('button', { name: 'Add project' }))
    const submit = screen.getByRole('button', { name: 'Add campaign' })
    expect(submit).toBeDisabled()
    const dir = screen.getByPlaceholderText('/absolute/path/to/project')
    await fireEvent.input(dir, { target: { value: '  /work/app  ' } })
    await fireEvent.input(screen.getByPlaceholderText(/defaults to the folder/), { target: { value: 'App' } })
    await fireEvent.click(submit)
    expect(add).toHaveBeenCalledWith({ dir: '/work/app', name: 'App' })
    expect(dir).toHaveValue('  /work/app  ') // failure: nothing lost
    await fireEvent.click(submit)
    expect(screen.queryByPlaceholderText('/absolute/path/to/project')).not.toBeInTheDocument()
    await fireEvent.click(screen.getByRole('button', { name: 'Add project' }))
    expect(screen.getByPlaceholderText('/absolute/path/to/project')).toHaveValue('') // success: cleared
  })

  it('offers per-campaign free sessions, folder actions, rename and explained removal', async () => {
    const s = store()
    const launch = vi.spyOn(s, 'launchInCampaign').mockResolvedValue()
    const openFolder = vi.spyOn(s, 'openCampaignFolder').mockResolvedValue()
    render(Sidebar, { store: s })
    await fireEvent.click(screen.getByRole('button', { name: 'Campaign actions for Camp c1' }))
    const menu = screen.getByRole('group', { name: 'Actions for Camp c1' })
    await fireEvent.click(within(menu).getByRole('button', { name: 'Opus: free session' }))
    expect(launch).toHaveBeenCalledWith('c1', { hero: 'opus' })
    await fireEvent.click(screen.getByRole('button', { name: 'Campaign actions for Camp c1' }))
    await fireEvent.click(screen.getByRole('button', { name: 'Open folder' }))
    expect(openFolder).toHaveBeenCalledWith('c1')
    await fireEvent.click(screen.getByRole('button', { name: 'Campaign actions for Camp c1' }))
    await fireEvent.click(screen.getByRole('button', { name: 'Remove project' }))
    expect(screen.getByRole('group', { name: 'Confirm removal of Camp c1' })).toHaveTextContent('Project files are not deleted')
  })

  it('omits an empty name', async () => {
    const s = store()
    const add = vi.spyOn(s, 'addCampaign').mockResolvedValue(true)
    render(Sidebar, { store: s })
    await fireEvent.click(screen.getByRole('button', { name: 'Add project' }))
    await fireEvent.input(screen.getByPlaceholderText('/absolute/path/to/project'), { target: { value: '/x' } })
    await fireEvent.click(screen.getByRole('button', { name: 'Add campaign' }))
    expect(add).toHaveBeenCalledWith({ dir: '/x', name: undefined })
  })

  it('removing a campaign takes two steps and can be cancelled', async () => {
    const s = store()
    const remove = vi.spyOn(s, 'removeCampaign').mockResolvedValue()
    render(Sidebar, { store: s })
    await fireEvent.click(screen.getByRole('button', { name: 'Campaign actions for Camp c1' }))
    await fireEvent.click(screen.getByRole('button', { name: 'Remove project' }))
    expect(remove).not.toHaveBeenCalled()
    await fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(remove).not.toHaveBeenCalled()
    await fireEvent.click(screen.getByRole('button', { name: 'Campaign actions for Camp c1' }))
    await fireEvent.click(screen.getByRole('button', { name: 'Remove project' }))
    await fireEvent.click(within(screen.getByRole('group', { name: 'Confirm removal of Camp c1' })).getByRole('button', { name: 'Remove project' }))
    expect(remove).toHaveBeenCalledWith('c1')
  })
})

const quest = (number: number, over: Partial<Quest> = {}): Quest => ({
  number, title: `Quest ${number}`, state: 'open', closed: false, url: `https://github.com/acme/c1/issues/${number}`,
  labels: [], assignees: [], blockedBy: [], subTotal: 0, subDone: 0, comments: 0, prs: [],
  updated: new Date().toISOString(), ...over,
})

const questResult = (quests: Quest[], over: Partial<QuestResult> = {}): QuestResult => ({
  source: 'github', repo: 'acme/c1', scope: { mode: 'labels', labels: ['tracked'] }, quests,
  totalOpen: quests.filter((q) => !q.closed).length, truncated: false, fetchedAt: new Date().toISOString(), stale: false, ...over,
})

function questStore(result: QuestResult | null, setup: (s: AppStore) => void = () => {}) {
  return store((s) => {
    s.quests = result
    s.questsFor = 'c1'
    // Most of these tests are about the list; the map tests switch to it.
    s.panelView = 'list'
    setup(s)
  })
}

describe('QuestsPanel', () => {
  const sample = [
    quest(1, { title: 'Epic', subTotal: 2, subDone: 1, labels: [{ name: 'epic', color: '5319e7' }] }),
    quest(2, { title: 'Child done', parent: 1, closed: true, state: 'cleared' }),
    quest(3, { title: 'Child locked', parent: 1, state: 'locked', blockedBy: [{ number: 4, closed: false }] }),
    quest(5, { title: 'Mine', state: 'claimed', assignees: ['alice'], prs: [{ number: 9, url: 'u', state: 'open' }] }),
  ]

  it('lists quests with state, effort, relations, labels, progress, assignees and pull requests', () => {
    const s = questStore(questResult([...sample, quest(4, { title: 'Blocker' })]))
    render(QuestsPanel, { store: s })
    expect(screen.getByText('acme/c1')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /#1 Epic/ })).toBeInTheDocument()
    expect(screen.getByText('epic')).toBeInTheDocument()
    expect(screen.getByTitle('Sub-issues done')).toHaveTextContent('1/2')
    expect(screen.getByText('Blocked by #4')).toBeInTheDocument()
    expect(screen.getByText('Blocks #3')).toBeInTheDocument()
    expect(screen.getByText('@alice')).toBeInTheDocument()
    expect(screen.getByText('PR #9 open')).toBeInTheDocument()
    // The state in words at the right of each row, not colour alone.
    const tags = [...document.querySelectorAll('.qlist .statetag')].map((e) => e.textContent)
    expect(tags).toEqual(['Ready', 'Cleared', 'Blocked', 'Assigned', 'Ready'])
    // Unknown effort is said, never guessed.
    expect(screen.getAllByText('Unknown').length).toBe(5)
  })

  it('shows the effort word and creature for a quest with a tier, and a diamond without one', () => {
    const s = questStore(questResult([quest(7, { title: 'Sized', effort: 'moderate', creature: 'goblin-scout' }), quest(8, { title: 'Unsized' })]))
    render(QuestsPanel, { store: s })
    const sized = screen.getByRole('button', { name: /#7 Sized/ })
    expect(sized).toHaveTextContent('Moderate')
    expect(sized.querySelector('.thumb')!.getAttribute('style')).toMatch(/goblin-idle-strip/)
    expect(sized.querySelector('.thumb')).toHaveAttribute('title', 'Goblin scout (Moderate effort)')
    expect(screen.getByRole('button', { name: /#8 Unsized/ }).querySelector('.thumb.unknown')).not.toBeNull()
  })

  it('shows state filters with counts', () => {
    render(QuestsPanel, { store: questStore(questResult(sample)) })
    const filters = screen.getByRole('group', { name: 'Filter by state' })
    expect(within(filters).getByRole('button', { name: /All/ })).toHaveTextContent('All 4')
    expect(within(filters).getByRole('button', { name: /Ready/ })).toHaveTextContent('Ready 1')
    expect(within(filters).getByRole('button', { name: /In progress/ })).toHaveTextContent('In progress 1')
    expect(within(filters).getByRole('button', { name: /Blocked/ })).toHaveTextContent('Blocked 1')
    expect(within(filters).getByRole('button', { name: /Cleared/ })).toHaveTextContent('Cleared 1')
  })

  it('narrows the list by state and by dungeon, flattening the nesting', async () => {
    const quests = [...sample, quest(6, { title: 'Stray' })]
    render(QuestsPanel, { store: questStore(questResult(quests)) })
    await fireEvent.click(screen.getByRole('button', { name: /^Blocked/ }))
    expect(screen.getAllByRole('listitem')).toHaveLength(1)
    expect(screen.getAllByRole('listitem')[0].getAttribute('style')).toMatch(/--depth:\s*0/)
    await fireEvent.click(screen.getByRole('button', { name: /^All/ }))
    const place = screen.getByRole('combobox', { name: 'Dungeon' })
    expect([...place.querySelectorAll('option')].map((o) => o.textContent)).toEqual(['All', '#1 Epic', 'Tavern (no epic)'])
    await fireEvent.change(place, { target: { value: 'tavern' } })
    expect(screen.getAllByRole('listitem').map((li) => li.textContent)).toEqual([expect.stringContaining('Mine'), expect.stringContaining('Stray')])
  })

  it('shows the whole list again when the chosen epic disappears', async () => {
    const quests = [...sample, quest(6, { title: 'Stray' })]
    const s = questStore(questResult(quests))
    render(QuestsPanel, { store: s })
    await fireEvent.change(screen.getByRole('combobox', { name: 'Dungeon' }), { target: { value: '1' } })
    expect(screen.queryByText('Stray')).toBeNull()
    s.quests = questResult([quest(5, { title: 'Mine' }), quest(6, { title: 'Stray' })])
    await tick()
    expect(screen.getAllByRole('listitem').map((li) => li.textContent)).toEqual([expect.stringContaining('Mine'), expect.stringContaining('Stray')])
  })

  it('nests children under their parent', () => {
    render(QuestsPanel, { store: questStore(questResult(sample)) })
    const items = screen.getAllByRole('listitem')
    const depth = (el: HTMLElement) => el.getAttribute('style')
    expect(depth(items[0])).toMatch(/--depth:\s*0/)
    expect(depth(items[1])).toMatch(/--depth:\s*1/)
    expect(items[1]).toHaveTextContent('Child done')
  })

  it('filters by text and can show only cleared quests', async () => {
    render(QuestsPanel, { store: questStore(questResult(sample)) })
    await fireEvent.input(screen.getByLabelText('Filter quests'), { target: { value: 'locked' } })
    expect(screen.getAllByRole('listitem')).toHaveLength(1)
    expect(screen.getByText('Child locked')).toBeInTheDocument()
    await fireEvent.input(screen.getByLabelText('Filter quests'), { target: { value: 'zzz' } })
    expect(screen.getByText('No quests match.')).toBeInTheDocument()
    await fireEvent.input(screen.getByLabelText('Filter quests'), { target: { value: '' } })
    await fireEvent.click(screen.getByRole('button', { name: /^Cleared/ }))
    expect(screen.getAllByRole('listitem')).toHaveLength(1)
    expect(screen.getByText('Child done')).toBeInTheDocument()
  })

  it('opens a quest when clicked', async () => {
    const s = questStore(questResult(sample))
    const select = vi.spyOn(s, 'selectQuest').mockResolvedValue()
    render(QuestsPanel, { store: s })
    await fireEvent.click(screen.getByRole('button', { name: /#5 Mine/ }))
    expect(select).toHaveBeenCalledWith(5)
  })

  it('refreshes on demand and disables the button while loading', async () => {
    const s = questStore(questResult(sample))
    const load = vi.spyOn(s, 'loadQuests').mockResolvedValue()
    const { unmount } = render(QuestsPanel, { store: s })
    await fireEvent.click(screen.getByRole('button', { name: 'Refresh quests' }))
    expect(load).toHaveBeenCalledWith(true)
    unmount()
    render(QuestsPanel, { store: questStore(questResult(sample), (st) => (st.questsLoading = true)) })
    expect(screen.getByRole('button', { name: 'Refresh quests' })).toBeDisabled()
  })

  it('labels sample issue data', async () => {
    const s = questStore(questResult(sample, { source: 'demo', repo: undefined }), (st) => (st.campaigns[0].demo = true))
    render(QuestsPanel, { store: s })
    expect(screen.getByText('sample data')).toBeInTheDocument()
    expect(screen.getByText(/Showing sample issues for this project/)).toBeInTheDocument()
  })

  it('warns when showing stale data, saying what the user sees meanwhile', () => {
    render(QuestsPanel, { store: questStore(questResult(sample, { stale: true, warning: 'GitHub took too long to answer.' })) })
    expect(screen.getByRole('status')).toHaveTextContent('GitHub took too long to answer. Showing saved data.')
  })

  const paused = (st: AppStore) => {
    st.github = {
      rations: [{ resource: 'graphql', limit: 5000, remaining: 4000, used: 1000, resetsAt: '2026-10-08T15:37:00Z', seenAt: '' }],
      camp: { active: true, until: '2026-10-08T15:12:00Z' },
    }
  }

  it('says in plain words that requests are paused and what is shown meanwhile', () => {
    render(QuestsPanel, { store: questStore(questResult(sample), paused) })
    const banner = screen.getByText(/Paused until/)
    expect(banner).toHaveTextContent('GitHub asked this app to slow down. Showing saved data.')
    expect(banner.textContent).not.toMatch(/camp|rations/i)
  })

  it('hides the GitHub budget, pause and stale notices for sample quests', () => {
    render(
      QuestsPanel,
      {
        store: questStore(questResult(sample, { source: 'demo', repo: undefined, stale: true, warning: 'x' }), (st) => {
          st.campaigns[0].demo = true
          paused(st)
        }),
      },
    )
    expect(screen.getByText(/Showing sample issues/)).toBeInTheDocument()
    expect(screen.queryByText(/GitHub requests:/)).not.toBeInTheDocument()
    expect(screen.queryByText(/Paused until/)).not.toBeInTheDocument()
    expect(screen.queryByText(/Showing saved data/)).not.toBeInTheDocument()
  })

  it('offers the separate sample project when the campaign has no repository', async () => {
    const s = questStore(null, (st) => (st.questsError = { code: 'no_repo', message: 'This campaign has no GitHub repository.' }))
    const addSample = vi.spyOn(s, 'addSampleCampaign').mockResolvedValue(true)
    render(QuestsPanel, { store: s })
    expect(screen.getByRole('alert')).toHaveTextContent('no GitHub repository')
    await fireEvent.click(screen.getByRole('button', { name: 'Try the separate sample project' }))
    expect(addSample).toHaveBeenCalled()
    expect(screen.queryByRole('button', { name: 'Try again' })).not.toBeInTheDocument()
  })

  it('offers retry and sample for credential problems, retry only for other failures', async () => {
    const s = questStore(null, (st) => (st.questsError = { code: 'no_token', message: 'No GitHub credentials found.' }))
    const load = vi.spyOn(s, 'loadQuests').mockResolvedValue()
    const { unmount } = render(QuestsPanel, { store: s })
    await fireEvent.click(screen.getByRole('button', { name: 'Try again' }))
    expect(load).toHaveBeenCalledWith(true)
    expect(screen.getByRole('button', { name: 'Try the separate sample project' })).toBeInTheDocument()
    unmount()
    render(QuestsPanel, { store: questStore(null, (st) => (st.questsError = { code: 'rate_limited', message: 'Rate limit.' })) })
    expect(screen.getByRole('button', { name: 'Try again' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Try the separate sample project' })).not.toBeInTheDocument()
  })

  it('has sensible empty and loading states', () => {
    const { unmount } = render(QuestsPanel, { store: questStore(questResult([])) })
    expect(screen.getByText('No issues in this repository yet.')).toBeInTheDocument()
    unmount()
    const { unmount: u2 } = render(QuestsPanel, { store: questStore(null, (st) => (st.questsLoading = true)) })
    expect(screen.getByText('Loading quests…')).toBeInTheDocument()
    u2()
    render(QuestsPanel, { store: store((st) => { st.campaigns = []; st.campaignId = null }) })
    expect(screen.getByText('Select a campaign to see its quests.')).toBeInTheDocument()
  })

  it('keeps the issue list above the detail pane while a quest is open', () => {
    render(QuestsPanel, { store: questStore(questResult(sample), (st) => (st.questNumber = 5)) })
    expect(screen.getByRole('button', { name: /← Quests/ })).toBeInTheDocument()
    expect(screen.getByLabelText('Filter quests')).toBeInTheDocument()
    expect(screen.getByRole('region', { name: 'Selected issue detail' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /#5 Mine/ })).toHaveAttribute('aria-current', 'true')
  })

  it('closes the drawer on narrow screens', async () => {
    const s = questStore(questResult(sample), (st) => (st.questsOpen = true))
    render(QuestsPanel, { store: s })
    expect(screen.getByRole('complementary', { name: 'Quests' })).toHaveClass('open')
    await fireEvent.click(screen.getByRole('button', { name: 'Close quests' }))
    expect(s.questsOpen).toBe(false)
  })
})

describe('QuestDetail', () => {
  const detail = (over: Partial<QuestDetailT> = {}): QuestDetailT => ({
    ...quest(3, { title: 'Build the form', state: 'locked', labels: [{ name: 'feature', color: '0e8a16' }], assignees: ['bob'], milestone: 'v1', parent: 1,
      blockedBy: [{ number: 2, title: 'Design', closed: true }, { number: 9, title: 'Copy', closed: false }],
      prs: [{ number: 21, url: 'u', state: 'merged' }], subTotal: 2, subDone: 1, comments: 2 }),
    author: 'alice', body: '## Steps\n\n1. one\n2. two\n\n<script>window.pwned=1</script>', created: '',
    blocking: [{ number: 4, title: 'Validate', closed: false }], subIssues: [{ number: 8, title: 'Sub', closed: true }],
    parentTitle: 'The epic',
    commentList: [{ author: 'carol', body: 'looks **good**', created: new Date().toISOString() }],
    briefing: { content: '# Task briefing\n\nContext stays plain text.', truncated: false, commentsShown: 1, commentsTotal: 1 },
    ...over,
  })

  function open(d: QuestDetailT | null, setup: (s: AppStore) => void = () => {}) {
    return store((s) => {
      s.questNumber = 3
      s.detail = d
      setup(s)
    })
  }

  it('shows the quest with its metadata, relations, markdown body and comments', () => {
    render(QuestDetail, { store: open(detail()) })
    expect(screen.getByRole('heading', { name: /#3 Build the form/ })).toBeInTheDocument()
    expect(screen.getByText('Locked')).toBeInTheDocument()
    expect(screen.getByText('opened by @alice')).toBeInTheDocument()
    expect(screen.getByText('feature', { selector: '.label' })).toBeInTheDocument()
    expect(screen.getByText('@bob')).toBeInTheDocument()
    expect(screen.getByText('milestone: v1')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '#1 The epic' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /#2 Design/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /#9 Copy/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /#4 Validate/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /#8 Sub/ })).toBeInTheDocument()
    expect(screen.getByText('Sub-issues (1/2)')).toBeInTheDocument()
    expect(screen.getByText('PR #21 merged')).toBeInTheDocument()
    const body = screen.getByLabelText('Description')
    expect(body.querySelector('h2')?.textContent).toBe('Steps')
    expect(body.querySelectorAll('ol li')).toHaveLength(2)
    expect(screen.getByLabelText('Comments').querySelector('.md strong')?.textContent).toBe('good')
  })

  it('keeps the open/closed mark outside the underlined link so no stray underline precedes the number', () => {
    render(QuestDetail, { store: open(detail()) })
    for (const [name, mark] of [[/#8 Sub/, '✓'], [/#4 Validate/, '○']] as const) {
      const link = screen.getByRole('button', { name })
      expect(link.textContent?.startsWith('#')).toBe(true)
      expect(link.querySelector('.mark')).toBeNull()
      expect(link.parentElement?.querySelector('.mark')?.textContent).toBe(mark)
    }
  })

  it('marks open prerequisites in the launch note and the jump list', async () => {
    const d = detail({ state: 'locked', blockedBy: [{ number: 5, title: 'A', closed: false }, { number: 6, title: 'B', closed: false }] })
    const { container } = render(QuestDetail, { store: open(d) })
    const note = container.querySelector('.launch-note')!
    expect([...note.querySelectorAll('.mark')].map((m) => m.textContent)).toEqual(['○', '○'])
    for (const l of note.querySelectorAll('.link')) expect(l.textContent?.startsWith('#')).toBe(true)
    await fireEvent.click(screen.getByRole('button', { name: /Jump to prerequisite/ }))
    const item = within(screen.getByRole('list', { name: 'Prerequisites' })).getByRole('button', { name: /#5 A/ })
    expect(item.textContent?.startsWith('#')).toBe(true)
    expect(item.parentElement?.querySelector('.mark')?.textContent).toBe('○')
  })

  it('never renders script from an issue body', () => {
    const { container } = render(QuestDetail, { store: open(detail()) })
    expect(container.querySelector('script')).toBeNull()
    expect((window as unknown as { pwned?: number }).pwned).toBeUndefined()
  })

  it('reviews the exact launch plan and starts with the same briefing', async () => {
    const s = open(detail({ state: 'open' }))
    const preview = vi.spyOn(s, 'previewQuestLaunch').mockResolvedValue({
      content: '<script>untrusted</script>', truncated: true, commentsShown: 5, commentsTotal: 12,
      canStart: true, hero: 'opus', heroLabel: 'Opus', directory: '/p/c1', briefingId: 'preview-1',
      briefingFile: '/state/briefings/c1/3-opus-preview-1.md', opener: 'Read the briefing.',
      systemPrompt: 'Keep to the issue.', command: ['claude', '--model', 'm1'],
    })
    const start = vi.spyOn(s, 'launchQuest').mockResolvedValue(true)
    render(QuestDetail, { store: s })
    expect(screen.getByLabelText('Hero')).toHaveValue('opus')
    await fireEvent.click(screen.getByRole('button', { name: 'Review launch' }))
    expect(preview).toHaveBeenCalledWith(3, 'opus')
    expect(screen.getByRole('dialog', { name: 'Review launch plan' })).toBeInTheDocument()
    expect(screen.getByText('<script>untrusted</script>')).toBeInTheDocument()
    expect(screen.getByText(/5 of 12 comments · some context was shortened/)).toBeInTheDocument()
    expect(screen.getByText('/state/briefings/c1/3-opus-preview-1.md')).toBeInTheDocument()
    await fireEvent.click(screen.getByRole('button', { name: 'Start expedition' }))
    expect(start).toHaveBeenCalledWith(3, 'opus', 'preview-1')
    expect(screen.queryByRole('dialog', { name: 'Review launch plan' })).not.toBeInTheDocument()
  })

  it('navigates to related quests and back to the list', async () => {
    const s = open(detail())
    const select = vi.spyOn(s, 'selectQuest').mockResolvedValue()
    const close = vi.spyOn(s, 'closeQuest')
    render(QuestDetail, { store: s })
    await fireEvent.click(screen.getByRole('button', { name: '#1 The epic' }))
    await fireEvent.click(screen.getByRole('button', { name: /#9 Copy/ }))
    await fireEvent.click(screen.getByRole('button', { name: /#4 Validate/ }))
    expect(select.mock.calls.map((c) => c[0])).toEqual([1, 9, 4])
    await fireEvent.click(screen.getByRole('button', { name: /← Quests/ }))
    expect(close).toHaveBeenCalled()
  })

  it('links to GitHub only for real https issues, never for demo data', () => {
    const { unmount } = render(QuestDetail, { store: open(detail({ url: 'https://github.com/acme/c1/issues/3' })) })
    const link = screen.getByRole('link', { name: 'Open on GitHub' })
    expect(link).toHaveAttribute('href', 'https://github.com/acme/c1/issues/3')
    expect(link).toHaveAttribute('rel', expect.stringContaining('noopener'))
    unmount()
    const { unmount: u2 } = render(QuestDetail, { store: open(detail(), (s) => (s.campaigns[0].demo = true)) })
    expect(screen.queryByRole('link', { name: 'Open on GitHub' })).not.toBeInTheDocument()
    u2()
    render(QuestDetail, { store: open(detail({ url: 'javascript:alert(1)' })) })
    expect(screen.queryByRole('link', { name: 'Open on GitHub' })).not.toBeInTheDocument()
  })

  it('handles an empty description, no comments and a closed quest with a reason', () => {
    render(QuestDetail, { store: open(detail({ body: '', commentList: [], comments: 0, state: 'cleared', closed: true, reason: 'not planned', blockedBy: [], blocking: [], subIssues: [], subTotal: 0, subDone: 0, prs: [], parent: undefined, parentTitle: undefined })) })
    expect(screen.getByText('No description.')).toBeInTheDocument()
    expect(screen.queryByLabelText('Comments')).not.toBeInTheDocument()
    expect(screen.getByText('Cleared · not planned')).toBeInTheDocument()
    expect(screen.queryByText('Blocked by')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Review launch' })).toBeDisabled()
    expect(screen.getByText('Closed issues cannot be started.')).toBeInTheDocument()
  })

  it('blocks a second launch in a busy working tree unless the project allows several', () => {
    const busy = (s: AppStore) => { s.sessions = [sess('s9', 'c1', { quest: 9, title: '#9 Other' })] }
    const { unmount } = render(QuestDetail, { store: open(detail({ state: 'open', blockedBy: [] }), busy) })
    expect(screen.getByRole('button', { name: 'Review launch' })).toBeDisabled()
    expect(screen.getByText('#9 Other is already running in this working tree.')).toBeInTheDocument()
    unmount()
    render(QuestDetail, { store: open(detail({ state: 'open', blockedBy: [] }), (s) => { busy(s); s.campaigns[0].parallelQuests = true }) })
    expect(screen.getByRole('button', { name: 'Review launch' })).toBeEnabled()
    expect(screen.queryByText(/already running in this working tree/)).not.toBeInTheDocument()
  })

  it('shows loading and error states, with retry', async () => {
    const { unmount } = render(QuestDetail, { store: open(null) })
    expect(screen.getByRole('status')).toHaveTextContent('Loading #3…')
    unmount()
    const s = open(null, (st) => (st.detailError = { code: 'not_found', message: 'That issue was not found.' }))
    const select = vi.spyOn(s, 'selectQuest').mockResolvedValue()
    render(QuestDetail, { store: s })
    expect(screen.getByRole('alert')).toHaveTextContent('That issue was not found.')
    await fireEvent.click(screen.getByRole('button', { name: 'Try again' }))
    expect(select).toHaveBeenCalledWith(3)
  })
})

describe('Toolbar quests toggle', () => {
  it('toggles the quest drawer', async () => {
    const s = store()
    render(Toolbar, { store: s })
    const btn = screen.getByRole('button', { name: 'Quests' })
    expect(btn).toHaveAttribute('aria-pressed', 'false')
    await fireEvent.click(btn)
    expect(s.questsOpen).toBe(true)
    expect(btn).toHaveAttribute('aria-pressed', 'true')
  })
})

describe('Rations', () => {
  const status = (remaining: number, over: Partial<GithubStatus> = {}): GithubStatus => ({
    rations: [
      { resource: 'core', limit: 5000, remaining: 1, used: 4999, resetsAt: new Date().toISOString(), seenAt: '' },
      { resource: 'graphql', limit: 5000, remaining, used: 5000 - remaining, resetsAt: '2026-10-08T15:37:00Z', seenAt: '' },
    ],
    camp: { active: false },
    ...over,
  })

  it('shows the GraphQL budget, not the REST one', () => {
    render(Rations, { status: status(4936) })
    expect(screen.getByText(/GitHub requests: 4,936 of 5,000 left, resets /)).toBeInTheDocument()
  })

  it('explains itself in plain words', () => {
    render(Rations, { status: status(4936) })
    const t = screen.getByText(/GitHub requests: 4,936/).getAttribute('title') ?? ''
    expect(t).toContain('GitHub allows')
    expect(t).toContain('per hour')
    expect(t).toContain('shared with every other tool')
  })

  it('turns to a warning below 20%', () => {
    const { unmount } = render(Rations, { status: status(999) })
    expect(screen.getByText(/GitHub requests: 999 /)).toHaveClass('low')
    unmount()
    render(Rations, { status: status(120) })
    expect(screen.getByText(/GitHub requests: 120 /)).toHaveClass('low')
  })

  it('is not low at 20% and above', () => {
    render(Rations, { status: status(1000) })
    expect(screen.getByText(/GitHub requests: 1,000 /)).not.toHaveClass('low')
    expect(screen.queryByText(/GitHub requests: 4/)).not.toBeInTheDocument()
  })

  it('shows nothing until a budget is known', () => {
    const { container, unmount } = render(Rations, { status: null })
    expect(container.querySelector('.rations')).toBeNull()
    unmount()
    const r = render(Rations, { status: { rations: [], camp: { active: false } } })
    expect(r.container.querySelector('.rations')).toBeNull()
  })
})

describe('QuestsPanel camp and budget', () => {
  it('shows the rations in the header', () => {
    const s = questStore(questResult([quest(1)]), (st) => {
      st.github = {
        rations: [{ resource: 'graphql', limit: 5000, remaining: 4800, used: 200, resetsAt: '2026-10-08T15:37:00Z', seenAt: '' }],
        camp: { active: false },
      }
    })
    render(QuestsPanel, { store: s })
    expect(screen.getByText(/GitHub requests: 4,800 of 5,000 left/)).toBeInTheDocument()
    expect(screen.queryByText(/Paused until/)).not.toBeInTheDocument()
  })

  it('says in plain words that requests are paused and saved data is shown', () => {
    const s = questStore(questResult([quest(1)]), (st) => {
      st.github = { rations: [], camp: { active: true, until: '2026-10-08T15:12:00Z', reason: 'GitHub sent Retry-After' } }
    })
    render(QuestsPanel, { store: s })
    const banner = screen.getByText(/Paused until/)
    expect(banner).toHaveTextContent('GitHub asked this app to slow down')
    expect(banner).toHaveTextContent('Showing saved data.')
  })

  it('does not claim a copy exists when there is none', () => {
    const s = questStore(null, (st) => {
      st.github = { rations: [], camp: { active: true, until: '2026-10-08T15:12:00Z' } }
      st.questsError = { code: 'camp', message: 'The app is not sending requests until 15:12.' }
    })
    render(QuestsPanel, { store: s })
    expect(screen.getByText(/Paused until/)).not.toHaveTextContent('Showing saved data')
    expect(screen.getByRole('alert')).toHaveTextContent('not sending requests until 15:12')
  })
})

describe('ScopePicker', () => {
  const options = (over: Partial<ScopeOptions> = {}): ScopeOptions => ({
    openTotal: 312, assigned: 3, login: 'me',
    labels: [
      { name: 'tracked', open: 14, color: '123456' },
      { name: 'bug', open: 40, color: 'd73a4a' },
      { name: 'wontfix', open: 0 },
      { name: 'a', open: 1 }, { name: 'b', open: 1 }, { name: 'c', open: 1 }, { name: 'd', open: 1 },
    ],
    ...over,
  })

  function picker(opts: ScopeOptions | null, setup: (s: AppStore) => void = () => {}) {
    const s = store((st) => {
      st.campaigns[0].scope = null
      st.scopeOptions = opts
      setup(st)
    })
    vi.spyOn(s, 'loadScopeOptions').mockResolvedValue()
    return s
  }

  it('asks the question with the numbers, preselecting the tracked label when it has issues', async () => {
    const s = picker(options())
    render(ScopePicker, { store: s })
    expect(screen.getByText(/This repository has 312 open issues/)).toBeInTheDocument()
    expect(screen.getByRole('radio', { name: /Issues labeled tracked \(14\)/ })).toBeChecked()
    expect(screen.getByRole('radio', { name: /Assigned to me \(@me\) \(3\)/ })).not.toBeChecked()
    expect(screen.getByRole('radio', { name: /Everything \(312\)/ })).not.toBeChecked()
    const chose = vi.spyOn(s, 'chooseScope').mockResolvedValue(true)
    await fireEvent.click(screen.getByRole('button', { name: 'Show these issues' }))
    expect(chose).toHaveBeenCalledWith({ mode: 'labels', labels: ['tracked'] })
  })

  it('preselects assigned-to-me when nothing is tracked yet, and explains how to start tracking', () => {
    const o = options({ labels: [{ name: 'tracked', open: 0 }, { name: 'bug', open: 40 }] })
    render(ScopePicker, { store: picker(o) })
    expect(screen.getByRole('radio', { name: /Assigned to me/ })).toBeChecked()
    expect(screen.getByText(/None yet. This app never adds the label/)).toBeInTheDocument()
  })

  it('leads with the tracked label only when it has issues; an empty one goes last', () => {
    const order = () => screen.getAllByRole('radio').map((r) => r.closest('label')?.textContent ?? '')
    const { unmount } = render(ScopePicker, { store: picker(options()) })
    expect(order()[0]).toMatch(/Issues labeled tracked/)
    unmount()
    render(ScopePicker, { store: picker(options({ labels: [{ name: 'tracked', open: 0 }, { name: 'bug', open: 4 }] })) })
    const o = order()
    expect(o[0]).toMatch(/Assigned to me/)
    expect(o[o.length - 1]).toMatch(/Issues labeled tracked/)
  })

  it('preselects nothing, and cannot submit, when there is nothing to suggest', async () => {
    const o = options({ assigned: 0, labels: [{ name: 'tracked', open: 0 }] })
    render(ScopePicker, { store: picker(o) })
    for (const r of screen.getAllByRole('radio')) expect(r).not.toBeChecked()
    expect(screen.getByRole('button', { name: 'Show these issues' })).toBeDisabled()
    await fireEvent.click(screen.getByRole('radio', { name: /Everything/ }))
    expect(screen.getByRole('button', { name: 'Show these issues' })).toBeEnabled()
  })

  it('submits each kind of choice', async () => {
    const s = picker(options())
    const chose = vi.spyOn(s, 'chooseScope').mockResolvedValue(true)
    render(ScopePicker, { store: s })
    const submit = () => fireEvent.click(screen.getByRole('button', { name: 'Show these issues' }))
    await fireEvent.click(screen.getByRole('radio', { name: /Assigned to me/ }))
    await submit()
    await fireEvent.click(screen.getByRole('radio', { name: /Everything/ }))
    await submit()
    expect(chose.mock.calls.map((c) => c[0])).toEqual([
      { mode: 'assigned', labels: [] },
      { mode: 'all', labels: [] },
    ])
  })

  it('lets you pick up to five labels, listing only those with issues (and the marker)', async () => {
    const s = picker(options())
    const chose = vi.spyOn(s, 'chooseScope').mockResolvedValue(true)
    render(ScopePicker, { store: s })
    await fireEvent.click(screen.getByRole('radio', { name: /Pick labels/ }))
    const list = screen.getByRole('list', { name: 'Labels' })
    expect(within(list).queryByText('wontfix')).not.toBeInTheDocument() // nothing open: not offered
    expect(screen.getByRole('button', { name: 'Show these issues' })).toBeDisabled() // none picked yet
    const box = (name: string) => within(list).getByRole('checkbox', { name: new RegExp(`^${name} \\(`) })
    for (const name of ['tracked', 'bug', 'a', 'b', 'c']) await fireEvent.click(box(name))
    expect(box('d')).toBeDisabled() // the sixth is refused
    await fireEvent.click(box('c')) // unpicking frees a slot
    expect(box('d')).toBeEnabled()
    await fireEvent.click(screen.getByRole('button', { name: 'Show these issues' }))
    expect(chose).toHaveBeenCalledWith({ mode: 'labels', labels: ['tracked', 'bug', 'a', 'b'] })
  })

  it('when changing an existing choice it starts from that choice and can be cancelled', async () => {
    const s = picker(options(), (st) => {
      st.campaigns[0].scope = { mode: 'labels', labels: ['bug', 'a'] }
      st.pickingScope = true
    })
    render(ScopePicker, { store: s })
    expect(screen.getByText('Show which issues?')).toBeInTheDocument()
    expect(screen.getByRole('radio', { name: /Pick labels/ })).toBeChecked()
    const list = screen.getByRole('list', { name: 'Labels' })
    expect(within(list).getByRole('checkbox', { name: /^bug \(/ })).toBeChecked()
    expect(within(list).getByRole('checkbox', { name: /^a \(/ })).toBeChecked()
    expect(within(list).getByRole('checkbox', { name: /^tracked \(/ })).not.toBeChecked()
    expect(screen.getByRole('button', { name: 'Apply' })).toBeEnabled()
    await fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(s.pickingScope).toBe(false)
  })

  it('starts from the current tracked-only and assigned choices too', () => {
    const { unmount } = render(ScopePicker, { store: picker(options(), (st) => { st.campaigns[0].scope = { mode: 'labels', labels: ['Tracked'] } }) })
    expect(screen.getByRole('radio', { name: /Issues labeled tracked/ })).toBeChecked()
    unmount()
    render(ScopePicker, { store: picker(options(), (st) => { st.campaigns[0].scope = { mode: 'assigned', labels: [] } }) })
    expect(screen.getByRole('radio', { name: /Assigned to me/ })).toBeChecked()
  })

  it('shows progress and failures, and loads the numbers when opened', () => {
    const s = picker(null, (st) => (st.scopeOptionsLoading = true))
    const { unmount } = render(ScopePicker, { store: s })
    expect(screen.getByRole('status')).toHaveTextContent('Counting issues…')
    expect(s.loadScopeOptions).toHaveBeenCalled()
    unmount()
    const bad = picker(null, (st) => (st.scopeOptionsError = { code: 'no_token', message: 'No GitHub credentials found.' }))
    render(ScopePicker, { store: bad })
    expect(screen.getByRole('alert')).toHaveTextContent('No GitHub credentials found.')
  })
})

describe('QuestsPanel scope', () => {
  const issues = [
    quest(1, { labels: [{ name: 'bug', color: 'd73a4a' }, { name: 'tracked' }] }),
    quest(2, { labels: [{ name: 'tracked' }, { name: 'feature' }] }),
    quest(3, { labels: [{ name: 'feature' }] }),
    quest(4, { closed: true, state: 'cleared', labels: [{ name: 'bug' }] }),
  ]

  it('says which issues are shown, and how many of the repository\'s open issues that is', () => {
    render(QuestsPanel, { store: questStore(questResult(issues, { totalOpen: 312 })) })
    expect(screen.getByRole('button', { name: /tracked · 3 of 312 open/ })).toBeInTheDocument()
  })

  it('describes each kind of scope in words', () => {
    const { unmount } = render(QuestsPanel, { store: questStore(questResult(issues, { totalOpen: 9, scope: { mode: 'assigned', labels: [] } })) })
    expect(screen.getByRole('button', { name: /assigned to me · 3 of 9 open/ })).toBeInTheDocument()
    unmount()
    const r = render(QuestsPanel, { store: questStore(questResult(issues, { totalOpen: 3, scope: { mode: 'all', labels: [] } })) })
    expect(screen.getByRole('button', { name: /everything · 3 of 3 open/ })).toBeInTheDocument()
    r.unmount()
    render(QuestsPanel, { store: questStore(questResult(issues, { totalOpen: 50, scope: { mode: 'labels', labels: ['a', 'b'] } })) })
    expect(screen.getByRole('button', { name: /a \+ b · 3 of 50 open/ })).toBeInTheDocument()
  })

  it('opens the picker from the scope line', async () => {
    const s = questStore(questResult(issues))
    render(QuestsPanel, { store: s })
    await fireEvent.click(screen.getByRole('button', { name: /tracked · 3 of/ }))
    expect(s.pickingScope).toBe(true)
  })

  it('shows the picker instead of the list while it is open, or when the campaign has no scope', () => {
    vi.spyOn(AppStore.prototype, 'loadScopeOptions').mockResolvedValue()
    const counts = { openTotal: 312, assigned: 3, login: 'me', labels: [{ name: 'tracked', open: 14 }] }
    const open = questStore(questResult(issues), (st) => { st.pickingScope = true; st.scopeOptions = counts })
    const { unmount } = render(QuestsPanel, { store: open })
    expect(screen.getAllByRole('group').some((g) => g.tagName === 'FIELDSET')).toBe(true)
    expect(screen.queryByLabelText('Filter quests')).not.toBeInTheDocument()
    unmount()
    const first = questStore(null, (st) => {
      st.scopeOptions = counts
      st.questsError = { code: 'scope_needed', message: 'Choose which of this repository\'s issues to show.' }
    })
    render(QuestsPanel, { store: first })
    expect(screen.getAllByRole('group').some((g) => g.tagName === 'FIELDSET')).toBe(true)
    expect(screen.queryByRole('alert')).not.toBeInTheDocument() // not shown as an error
    vi.restoreAllMocks()
  })

  it('warns when the list was cut', () => {
    render(QuestsPanel, { store: questStore(questResult(issues, { truncated: true, totalOpen: 912, scope: { mode: 'all', labels: [] } })) })
    expect(screen.getByRole('status')).toHaveTextContent('Showing the newest 3 of 912 open issues. Narrow the scope')
  })

  it('does not warn when nothing was cut, and has no scope line for demo data', () => {
    const { unmount } = render(QuestsPanel, { store: questStore(questResult(issues)) })
    expect(screen.queryByText(/Showing the newest/)).not.toBeInTheDocument()
    unmount()
    render(QuestsPanel, {
      store: questStore(questResult(issues, { source: 'demo', scope: undefined, repo: undefined }), (st) => (st.campaigns[0].demo = true)),
    })
    expect(screen.queryByRole('button', { name: /open ▾/ })).not.toBeInTheDocument()
  })

  it('offers the most common labels as filters and narrows the list', async () => {
    render(QuestsPanel, { store: questStore(questResult(issues)) })
    const group = screen.getByRole('group', { name: 'Filter by label' })
    const names = within(group).getAllByRole('button').map((b) => b.textContent?.trim())
    expect(names).toEqual(['bug 2', 'feature 2', 'tracked 2']) // by count, ties by name
    await fireEvent.click(within(group).getByRole('button', { name: /^feature/ }))
    expect(within(group).getByRole('button', { name: /^feature/ })).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getAllByRole('listitem').map((li) => li.textContent)).toHaveLength(2)
    expect(screen.queryByText('Quest 1')).not.toBeInTheDocument()
    // Two filters mean "either".
    await fireEvent.click(within(group).getByRole('button', { name: /^bug/ }))
    expect(screen.getAllByRole('listitem')).toHaveLength(4)
    // Toggling again clears.
    await fireEvent.click(within(group).getByRole('button', { name: /^feature/ }))
    await fireEvent.click(within(group).getByRole('button', { name: /^bug/ }))
    expect(screen.getAllByRole('listitem')).toHaveLength(4)
  })

  it('offers no label filters when there is nothing to choose between', () => {
    render(QuestsPanel, { store: questStore(questResult([quest(1, { labels: [{ name: 'only' }] })])) })
    expect(screen.queryByRole('group', { name: 'Filter by label' })).not.toBeInTheDocument()
  })
})

describe('session attention in the cockpit', () => {
  it('marks a waiting session in the list with words and a mark, not colour alone', () => {
    const s = store((st) => {
      st.sessions = [sess('a', 'c1', { activity: 'needs-you' }), sess('b', 'c1', { activity: 'working' }), sess('c', 'c1', { activity: 'idle' })]
      st.activeId = 'b'
    })
    render(Sidebar, { store: s })
    const list = screen.getByRole('list', { name: 'Expeditions in Camp c1' })
    const waiting = within(list).getByRole('button', { name: /^T-a/ })
    expect(waiting).toHaveTextContent('waiting for you')
    expect(waiting.querySelector('.dot.needs')).toHaveTextContent('?')
    expect(within(list).getByRole('button', { name: /^T-b/ })).not.toHaveTextContent('waiting')
    expect(within(list).getByRole('button', { name: /^T-c/ })).toHaveTextContent('idle')
  })

  it('uses plain wording in plain mode', () => {
    const s = store((st) => {
      st.appearance.wordMode = 'plain'
      st.sessions = [sess('a', 'c1', { activity: 'needs-you' })]
    })
    render(Sidebar, { store: s })
    expect(screen.getByRole('button', { name: /^T-a/ })).toHaveTextContent('waiting for your input')
  })

  it('shows the state of the open session as a chip in the toolbar', () => {
    const s = store((st) => {
      st.sessions = [sess('a', 'c1', { activity: 'needs-you' })]
      st.activeId = 'a'
    })
    render(Toolbar, { store: s })
    expect(screen.getByRole('status')).toHaveTextContent('? waiting for you')
  })

  it('shows no chip once the session has ended', () => {
    const s = store((st) => {
      st.sessions = [sess('a', 'c1', { exited: true, activity: 'ended' })]
      st.activeId = 'a'
    })
    render(Toolbar, { store: s })
    expect(screen.queryByRole('status')).toBeNull()
  })
})

describe('desktop notifications switch', () => {
  beforeEach(() => localStorage.clear())
  afterEach(() => { localStorage.clear(); vi.unstubAllGlobals() })
  const fakeNotification = (permission: NotificationPermission, answer: NotificationPermission = permission) => {
    const request = vi.fn(async () => answer)
    vi.stubGlobal('Notification', Object.assign(vi.fn(), { permission, requestPermission: request }))
    return request
  }

  it('does not ask the browser until the user turns it on', async () => {
    const request = fakeNotification('default', 'granted')
    const s = store()
    render(SettingsDialog, { store: s, onclose: () => {} })
    expect(request).not.toHaveBeenCalled()
    expect(s.appearance.notifications).toBe(false)
    await fireEvent.click(screen.getByRole('checkbox', { name: /Desktop notifications/ }))
    await vi.waitFor(() => expect(s.appearance.notifications).toBe(true))
    expect(request).toHaveBeenCalledOnce()
  })

  it('stays off and explains when the browser says no', async () => {
    fakeNotification('default', 'denied')
    const s = store()
    render(SettingsDialog, { store: s, onclose: () => {} })
    const box = screen.getByRole('checkbox', { name: /Desktop notifications/ })
    await fireEvent.click(box)
    expect(await screen.findByRole('alert')).toHaveTextContent(/blocked for this site/)
    expect(s.appearance.notifications).toBe(false)
    expect(box).not.toBeChecked()
  })

  it('turns off without asking again', async () => {
    const request = fakeNotification('granted')
    const s = store((st) => st.updateAppearance({ notifications: true }))
    render(SettingsDialog, { store: s, onclose: () => {} })
    await fireEvent.click(screen.getByRole('checkbox', { name: /Desktop notifications/ }))
    expect(s.appearance.notifications).toBe(false)
    expect(request).not.toHaveBeenCalled()
  })
})

describe('parallel quests switch', () => {
  it('saves the choice for the selected project', async () => {
    const s = store()
    const update = vi.spyOn(s, 'updateCampaign').mockResolvedValue(true)
    render(SettingsDialog, { store: s, onclose: () => {} })
    const box = screen.getByRole('checkbox', { name: /Allow several quests at once/ })
    expect(box).not.toBeChecked()
    await fireEvent.click(box)
    expect(update).toHaveBeenCalledWith('c1', { parallelQuests: true })
  })
})
