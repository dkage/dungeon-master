import { describe, expect, it, vi } from 'vitest'
import { fireEvent, render, screen, within } from '@testing-library/svelte'
import Toolbar from './Toolbar.svelte'
import Sidebar from './Sidebar.svelte'
import { AppStore } from './store.svelte'
import type { Campaign, Quest, QuestDetail as QuestDetailT, QuestResult, SessionInfo } from './api'
import QuestsPanel from './QuestsPanel.svelte'
import QuestDetail from './QuestDetail.svelte'
import Rations from './Rations.svelte'
import type { GithubStatus, ScopeOptions } from './api'
import ScopePicker from './ScopePicker.svelte'

const camp = (id: string, over: Partial<Campaign> = {}): Campaign => ({
  id, name: `Camp ${id}`, dir: `/p/${id}`, remote: 'origin', scope: { mode: 'labels', labels: ['tracked'] }, demo: false, repo: `acme/${id}`, ...over,
})
const sess = (id: string, campaign: string, over: Partial<SessionInfo> = {}): SessionInfo => ({
  id, kind: 'hero', title: `T-${id}`, campaign, dir: '/d', created: '', exited: false, exitCode: 0, ...over,
})

function store(setup: (s: AppStore) => void = () => {}) {
  const s = new AppStore({
    getState: vi.fn(), createSession: vi.fn(), deleteSession: vi.fn(),
    addCampaign: vi.fn(), updateCampaign: vi.fn(), removeCampaign: vi.fn(),
    getQuests: vi.fn(), getQuest: vi.fn(), getGithub: vi.fn(), setScope: vi.fn(), getScopeOptions: vi.fn(),
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

  it('lists quests with state, labels, progress, blockers, assignees and pull requests', () => {
    render(QuestsPanel, { store: questStore(questResult(sample)) })
    expect(screen.getByText('acme/c1')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /#1 Epic/ })).toBeInTheDocument()
    expect(screen.getByText('epic')).toBeInTheDocument()
    expect(screen.getByTitle('Sub-issues done')).toHaveTextContent('1/2')
    expect(screen.getByText('blocked by #4')).toBeInTheDocument()
    expect(screen.getByText('@alice')).toBeInTheDocument()
    expect(screen.getByText('PR #9 open')).toBeInTheDocument()
    // Accessible state text for each row.
    expect(screen.getAllByText(/^(Open|Locked|In progress|Cleared):$/).map((e) => e.textContent)).toEqual([
      'Open:', 'Cleared:', 'Locked:', 'In progress:',
    ])
  })

  it('shows state counts', () => {
    render(QuestsPanel, { store: questStore(questResult(sample)) })
    const tally = screen.getByLabelText('Quest counts')
    expect(tally).toHaveTextContent('1 open')
    expect(tally).toHaveTextContent('1 in progress')
    expect(tally).toHaveTextContent('1 locked')
    expect(tally).toHaveTextContent('1 cleared')
  })

  it('nests children under their parent', () => {
    render(QuestsPanel, { store: questStore(questResult(sample)) })
    const items = screen.getAllByRole('listitem')
    const depth = (el: HTMLElement) => el.getAttribute('style')
    expect(depth(items[0])).toMatch(/--depth:\s*0/)
    expect(depth(items[1])).toMatch(/--depth:\s*1/)
    expect(items[1]).toHaveTextContent('Child done')
  })

  it('filters by text and can hide cleared quests', async () => {
    render(QuestsPanel, { store: questStore(questResult(sample)) })
    await fireEvent.input(screen.getByLabelText('Filter quests'), { target: { value: 'locked' } })
    expect(screen.getAllByRole('listitem')).toHaveLength(1)
    expect(screen.getByText('Child locked')).toBeInTheDocument()
    await fireEvent.input(screen.getByLabelText('Filter quests'), { target: { value: 'zzz' } })
    expect(screen.getByText('No quests match.')).toBeInTheDocument()
    await fireEvent.input(screen.getByLabelText('Filter quests'), { target: { value: '' } })
    await fireEvent.click(screen.getByLabelText('Hide cleared'))
    expect(screen.queryByText('Child done')).not.toBeInTheDocument()
    expect(screen.getAllByRole('listitem')).toHaveLength(3)
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

  it('labels demo data and lets you switch back to real issues', async () => {
    const s = questStore(questResult(sample, { source: 'demo', repo: undefined }), (st) => (st.campaigns[0].demo = true))
    const setDemo = vi.spyOn(s, 'setDemo').mockResolvedValue(true)
    render(QuestsPanel, { store: s })
    expect(screen.getByText('demo data')).toBeInTheDocument()
    expect(screen.getByText(/Showing sample quests/)).toBeInTheDocument()
    await fireEvent.click(screen.getByRole('button', { name: 'Use real issues' }))
    expect(setDemo).toHaveBeenCalledWith(false)
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
    expect(screen.getByText(/Showing sample quests/)).toBeInTheDocument()
    expect(screen.queryByText(/GitHub requests:/)).not.toBeInTheDocument()
    expect(screen.queryByText(/Paused until/)).not.toBeInTheDocument()
    expect(screen.queryByText(/Showing saved data/)).not.toBeInTheDocument()
  })

  it('offers demo quests when the campaign has no repository', async () => {
    const s = questStore(null, (st) => (st.questsError = { code: 'no_repo', message: 'This campaign has no GitHub repository.' }))
    const setDemo = vi.spyOn(s, 'setDemo').mockResolvedValue(true)
    render(QuestsPanel, { store: s })
    expect(screen.getByRole('alert')).toHaveTextContent('no GitHub repository')
    await fireEvent.click(screen.getByRole('button', { name: 'Load demo quests' }))
    expect(setDemo).toHaveBeenCalledWith(true)
    expect(screen.queryByRole('button', { name: 'Try again' })).not.toBeInTheDocument()
  })

  it('offers retry (and demo) for credential problems, retry only for other failures', async () => {
    const s = questStore(null, (st) => (st.questsError = { code: 'no_token', message: 'No GitHub credentials found.' }))
    const load = vi.spyOn(s, 'loadQuests').mockResolvedValue()
    const { unmount } = render(QuestsPanel, { store: s })
    await fireEvent.click(screen.getByRole('button', { name: 'Try again' }))
    expect(load).toHaveBeenCalledWith(true)
    expect(screen.getByRole('button', { name: 'Load demo quests' })).toBeInTheDocument()
    unmount()
    render(QuestsPanel, { store: questStore(null, (st) => (st.questsError = { code: 'rate_limited', message: 'Rate limit.' })) })
    expect(screen.getByRole('button', { name: 'Try again' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Load demo quests' })).not.toBeInTheDocument()
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

  it('shows the detail view in place of the list while a quest is open', () => {
    render(QuestsPanel, { store: questStore(questResult(sample), (st) => (st.questNumber = 5)) })
    expect(screen.getByRole('button', { name: /← Quests/ })).toBeInTheDocument()
    expect(screen.queryByLabelText('Filter quests')).not.toBeInTheDocument()
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
    expect(screen.getByText('feature')).toBeInTheDocument()
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

  it('never renders script from an issue body', () => {
    const { container } = render(QuestDetail, { store: open(detail()) })
    expect(container.querySelector('script')).toBeNull()
    expect((window as unknown as { pwned?: number }).pwned).toBeUndefined()
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
    expect(screen.getByText(/None yet. Add the label to an issue on GitHub/)).toBeInTheDocument()
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
    expect(screen.getByRole('group')).toBeInTheDocument() // the fieldset
    expect(screen.queryByLabelText('Filter quests')).not.toBeInTheDocument()
    unmount()
    const first = questStore(null, (st) => {
      st.scopeOptions = counts
      st.questsError = { code: 'scope_needed', message: 'Choose which of this repository\'s issues to show.' }
    })
    render(QuestsPanel, { store: first })
    expect(screen.getByRole('group')).toBeInTheDocument()
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
