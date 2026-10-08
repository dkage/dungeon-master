<script lang="ts">
  import type { AppStore } from './store.svelte'
  import type { SessionInfo } from './api'

  let { store }: { store: AppStore } = $props()

  let dir = $state('')
  let name = $state('')
  let confirming = $state<string | null>(null)

  async function add(e: SubmitEvent) {
    e.preventDefault()
    if (!dir.trim()) return
    if (await store.addCampaign({ dir: dir.trim(), name: name.trim() || undefined })) {
      dir = ''
      name = ''
    }
  }

  function status(s: SessionInfo): { label: string; kind: string } {
    if (s.lost) return { label: 'lost (Dungeon restarted)', kind: 'lost' }
    if (!s.exited) return { label: 'running', kind: 'running' }
    return s.exitCode === 0
      ? { label: 'exited', kind: 'done' }
      : { label: `exited with code ${s.exitCode}`, kind: 'failed' }
  }
</script>

<aside class="sidebar">
  <nav aria-label="Campaigns">
    <h2>Campaigns</h2>
    {#if store.campaigns.length === 0}
      <p class="hint">A campaign is a project directory. Add one to start.</p>
    {/if}
    <ul class="campaigns">
      {#each store.campaigns as c (c.id)}
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
            </button>
            {#if confirming === c.id}
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
            <ul class="sessions" aria-label={`Expeditions in ${c.name}`}>
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
  </nav>

  <form class="add" onsubmit={add}>
    <h2>Add campaign</h2>
    <label>
      <span>Directory</span>
      <input type="text" bind:value={dir} placeholder="/absolute/path/to/project" spellcheck="false" autocomplete="off" />
    </label>
    <label>
      <span>Name (optional)</span>
      <input type="text" bind:value={name} placeholder="defaults to the folder name" autocomplete="off" />
    </label>
    <button type="submit" disabled={store.busy || !dir.trim()}>Add campaign</button>
  </form>
</aside>
