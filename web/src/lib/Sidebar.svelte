<script lang="ts">
  import type { AppStore } from './store.svelte'
  import type { SessionInfo } from './api'
  import Term from './Term.svelte'
  import { wordLabel } from './words'

  let { store, onsettings = () => {}, onwords = () => {} }: { store: AppStore; onsettings?: () => void; onwords?: () => void } = $props()

  let dir = $state('')
  let name = $state('')
  let search = $state('')
  let adding = $state(false)
  let confirming = $state<string | null>(null)
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

  function status(s: SessionInfo): { label: string; kind: string } {
    if (s.lost) return { label: 'lost (app restarted)', kind: 'lost' }
    if (!s.exited) return { label: 'running', kind: 'running' }
    return s.exitCode === 0
      ? { label: 'exited', kind: 'done' }
      : { label: `exited with code ${s.exitCode}`, kind: 'failed' }
  }
</script>

<aside class="sidebar">
  <header class="brand-header">
    <span class="brand-mark" aria-hidden="true">✦</span>
    <strong>Dungeon Master</strong>
  </header>
  <label class="sidebar-search">
    <span class="sr">Search projects and sessions</span>
    <span aria-hidden="true">⌕</span>
    <input type="search" bind:value={search} placeholder="Search projects and sessions…" />
  </label>
  <nav aria-label={wordLabel('campaign', store.appearance.wordMode, true)}>
    <h2 class="sidebar-heading"><Term word="campaign" mode={store.appearance.wordMode} /><span class="section-rule" aria-hidden="true"></span><button type="button" class="heading-add" aria-label="Add project" onclick={() => (adding = true)}>＋</button></h2>
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
              <span class="cname">{c.name}</span>
              <span class="repo" class:none={!c.repo}>{c.repo ?? 'no GitHub repo'}</span>
              {#if c.sample}<span class="campaign-badge">Sample</span>{/if}
            </button>
            {#if c.sample}
              <button type="button" class="ghost" aria-label="Remove sample project" onclick={() => store.removeCampaign(c.id)}>×</button>
            {:else if confirming === c.id}
              <button type="button" class="danger" onclick={() => { confirming = null; store.removeCampaign(c.id) }}>
                Remove?
              </button>
              <button type="button" class="ghost" aria-label="Cancel removal" onclick={() => (confirming = null)}>×</button>
            {:else}
              <button type="button" class="ghost" aria-label={`Remove campaign ${c.name}`} onclick={() => (confirming = c.id)}>
                ⋯
              </button>
            {/if}
          </div>

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
                    <span class="dot {st.kind}" aria-hidden="true"></span>
                    <span class="stitle">{s.title}</span>
                    <span class="sr">{st.label}</span>
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
        <input type="text" bind:value={dir} placeholder="/absolute/path/to/project" spellcheck="false" autocomplete="off" />
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
    <button type="button" class="settings-button" onclick={onwords}>Word guide</button>
    <button type="button" class="settings-button" onclick={() => { store.openFirstRun(); void store.loadSuggestions() }}>Getting started</button>
  </div>
</aside>
