<script lang="ts">
  import type { AppStore } from './store.svelte'
  import type { SessionInfo } from './api'
  import Term from './Term.svelte'
  import { wordLabel } from './words'
  import { sessionStatus } from './attention'

  let { store, onsettings = () => {}, onwords = () => {} }: { store: AppStore; onsettings?: () => void; onwords?: () => void } = $props()

  let dir = $state('')
  let name = $state('')
  let search = $state('')
  let adding = $state(false)
  let confirming = $state<string | null>(null)
  let menuFor = $state<string | null>(null)
  let renameFor = $state<string | null>(null)
  let renameName = $state('')
  const visibleCampaigns = $derived(store.campaigns.filter((campaign) => {
    const query = search.trim().toLowerCase()
    if (!query) return true
    return [campaign.name, campaign.repo ?? '', campaign.dir, ...store.sessionsOf(campaign.id).map((session) => session.title)]
      .some((value) => value.toLowerCase().includes(query))
  }))

  async function add(e: SubmitEvent) {
    e.preventDefault()
    if (!dir.trim()) return
    if (await store.addCampaign({ dir: dir.trim(), name: name.trim() || undefined })) {
      dir = ''
      name = ''
      adding = false
    }
  }

  async function rename(e: SubmitEvent, id: string) {
    e.preventDefault()
    if (!renameName.trim()) return
    if (await store.updateCampaign(id, { name: renameName.trim() })) renameFor = null
  }

  /** The hero behind a session, shown beside its quest or scratch terminal. */
  const heroName = (s: SessionInfo) => (s.hero ? (store.heroes.find((h) => h.id === s.hero)?.label ?? s.hero) : '')

  const status = (s: SessionInfo) => sessionStatus(s, store.appearance.wordMode)
</script>

<aside class="sidebar">
  <header class="brand-header">
    <span class="brand-mark" aria-hidden="true">✦</span>
    <strong>Dungeon Master</strong>
  </header>
  <label class="sidebar-search">
    <span class="sr">Search projects and sessions</span>
    <span aria-hidden="true">⌕</span>
    <input type="search" bind:value={search} placeholder="Jump to project or session…" />
  </label>
  <nav aria-label={wordLabel('campaign', store.appearance.wordMode, true)}>
    <h2 class="sidebar-heading"><Term word="campaign" mode={store.appearance.wordMode} /><span class="section-rule" aria-hidden="true"></span><button type="button" class="heading-add" aria-label="Add project" onclick={() => { adding = true }}>＋</button></h2>
    {#if store.campaigns.length === 0}
      <p class="hint">A campaign is a project directory. Add one to start.</p>
    {/if}
    <ul class="campaigns">
      {#each visibleCampaigns as c (c.id)}
        {@const selected = c.id === store.campaignId}
        <li class="campaign" class:selected>
          <div class="row">
            <button
              type="button"
              class="pick"
              aria-current={selected ? 'true' : undefined}
              title={c.dir}
              onclick={() => store.selectCampaign(c.id)}
            >
              <span class="cline"><span class="cname">{c.name}</span>{#if c.sample}<span class="campaign-badge">Sample</span>{/if}</span>
              <span class="repo" class:none={!c.repo}>{c.repo ?? 'no GitHub repo'}</span>
            </button>
            {#if c.sample}
              <button type="button" class="ghost" aria-label="Remove sample project" onclick={() => (confirming = c.id)}>×</button>
            {:else}
              <button type="button" class="ghost" aria-label={`Campaign actions for ${c.name}`} aria-expanded={menuFor === c.id} onclick={() => (menuFor = menuFor === c.id ? null : c.id)}>⋯</button>
            {/if}
          </div>

          {#if menuFor === c.id}
            <div class="campaign-menu" role="group" aria-label={`Actions for ${c.name}`}>
              <strong>Free session</strong>
              {#each store.heroes as h (h.id)}<button type="button" disabled={store.busy || !store.claude.found} onclick={() => { menuFor = null; void store.launchInCampaign(c.id, { hero: h.id }) }}>{h.label}: free session</button>{/each}
              <button type="button" disabled={store.busy} onclick={() => { menuFor = null; void store.launchInCampaign(c.id, { shell: true }) }}>Shell</button>
              <hr />
              <button type="button" onclick={() => { menuFor = null; void store.openCampaignFolder(c.id) }}>Open folder</button>
              <button type="button" onclick={() => { menuFor = null; store.changeCampaignScope(c.id) }}>Change which issues to show</button>
              <button type="button" onclick={() => { menuFor = null; renameFor = c.id; renameName = c.name }}>Rename</button>
              <button type="button" class="danger" onclick={() => { menuFor = null; confirming = c.id }}>Remove project</button>
            </div>
          {/if}

          {#if renameFor === c.id}
            <form class="campaign-inline-form" onsubmit={(e) => rename(e, c.id)}>
              <label><span>Project name</span><input aria-label={`New name for ${c.name}`} bind:value={renameName} /></label>
              <button type="submit" disabled={!renameName.trim()}>Save name</button>
              <button type="button" class="secondary" onclick={() => (renameFor = null)}>Cancel</button>
            </form>
          {/if}

          {#if confirming === c.id}
            <div class="remove-confirm" role="group" aria-label={`Confirm removal of ${c.name}`}>
              <p>{c.sample ? 'Remove the sample project? Its generated workspace will be deleted.' : `Remove ${c.name} from this app? Project files are not deleted.`}</p>
              <button type="button" class="danger" onclick={() => { confirming = null; void store.removeCampaign(c.id) }}>{c.sample ? 'Remove sample' : 'Remove project'}</button>
              <button type="button" class="secondary" onclick={() => (confirming = null)}>Cancel</button>
            </div>
          {/if}

          {#if store.sessionsOf(c.id).length > 0}
            <ul class="sessions" aria-label={`${store.appearance.wordMode === 'plain' ? 'Sessions' : 'Expeditions'} in ${c.name}`}>
              {#each store.sessionsOf(c.id) as s (s.id)}
                {@const st = status(s)}
                <li class="session" class:active={s.id === store.activeId}>
                  <button
                    type="button"
                    class="pick"
                    aria-current={s.id === store.activeId ? 'true' : undefined}
                    onclick={() => store.selectSession(s.id)}
                  >
                    <span class="dot {st.kind}" aria-hidden="true">{st.glyph}</span>
                    <span class="stitle">{s.kind === 'shell' && !s.quest ? 'Scratch terminal' : s.title}{#if s.quest && heroName(s)}{' '}<span class="hero-tag">({heroName(s)})</span>{/if}</span>
                    <span class="sr">{st.label}</span>
                    {#if st.kind === 'needs'}<span class="badge needs" aria-hidden="true">{st.label}</span>{/if}
                    {#if s.exited && !s.lost}<span class="badge">exit {s.exitCode}</span>{/if}
                    {#if s.lost}<span class="badge">lost</span>{/if}
                  </button>
                  <button type="button" class="ghost" aria-label={`Close ${s.title}`} onclick={() => store.close(s.id)}>×</button>
                </li>
              {/each}
            </ul>
          {/if}
        </li>
      {/each}
    </ul>
    {#if search && visibleCampaigns.length === 0}
      <p class="hint">No projects or sessions match.</p>
    {/if}
  </nav>

  {#if adding}
    <form class="add" onsubmit={add}>
      <div class="add-heading"><h2>Add {store.appearance.wordMode === 'plain' ? 'project' : 'campaign'}</h2><button type="button" class="ghost" aria-label="Cancel adding project" onclick={() => (adding = false)}>×</button></div>
      <label>
        <span>Directory</span>
        <button type="button" class="secondary" aria-label="Choose folder" disabled={store.busy} onclick={() => void store.pickFolder().then((path) => { if (path) { dir = path; if (!name.trim()) name = path.split(/[\\/]/).filter(Boolean).at(-1) ?? '' } })}>Choose folder…</button>
        <input type="text" bind:value={dir} placeholder="/absolute/path/to/project" spellcheck="false" autocomplete="off" />
        <small class="hint">Type a path if the native folder picker is unavailable.</small>
      </label>
      <label>
        <span>Name (optional)</span>
        <input type="text" bind:value={name} placeholder="defaults to the folder name" autocomplete="off" />
      </label>
      <button type="submit" disabled={store.busy || !dir.trim()}>Add {store.appearance.wordMode === 'plain' ? 'project' : 'campaign'}</button>
    </form>
  {/if}
  <div class="sidebar-footer">
    <button type="button" class="settings-button add-project" onclick={() => (adding = !adding)}>{adding ? '× Cancel' : '＋ Add project'}</button>
    <button type="button" class="settings-button" onclick={onsettings}>⚙ Settings</button>
    <button type="button" class="settings-button minor" onclick={onwords}>Word guide</button>
    <button type="button" class="settings-button minor" onclick={() => { store.openFirstRun(); void store.loadSuggestions() }}>Getting started</button>
  </div>
</aside>
