<script lang="ts">
  import { onMount } from 'svelte'
  import { AppStore } from './lib/store.svelte'
  import Sidebar from './lib/Sidebar.svelte'
  import Toolbar from './lib/Toolbar.svelte'
  import Terminal from './lib/Terminal.svelte'
  import QuestsPanel from './lib/QuestsPanel.svelte'

  let { store = new AppStore() }: { store?: AppStore } = $props()

  // While the page is visible: keep the GitHub budget fresh, and re-read the
  // quests now and then as low-priority background work. The server answers
  // most of these from its cache, so they cost GitHub nothing.
  const BUDGET_EVERY = 30_000
  const QUESTS_EVERY = 60_000

  onMount(() => {
    store.refresh()
    const visible = () => document.visibilityState === 'visible'
    const budget = setInterval(() => visible() && store.loadGithub(), BUDGET_EVERY)
    const quests = setInterval(() => {
      if (visible() && store.campaignId && !store.unauthorised) void store.loadQuests(false, true)
    }, QUESTS_EVERY)
    const onFocus = () => {
      if (visible() && store.campaignId) {
        void store.loadQuests(false, true)
        void store.loadGithub()
      }
    }
    document.addEventListener('visibilitychange', onFocus)
    return () => {
      clearInterval(budget)
      clearInterval(quests)
      document.removeEventListener('visibilitychange', onFocus)
    }
  })

  // Sessions of every campaign stay mounted (hidden) so switching is instant
  // and their terminals keep their state; lost sessions have no terminal.
  const live = $derived(store.sessions.filter((s) => !s.lost))
</script>

{#if store.unauthorised}
  <main class="signin" aria-labelledby="signin-title">
    <h1 id="signin-title">This tab is not signed in</h1>
    <p>
      Dungeon Master did not accept this browser's access token. Your campaigns and sessions are safe: they are stored on
      this computer.
    </p>
    <p>
      Open the link printed in the terminal where you ran <code>make run</code> or <code>dungeon serve</code>. It ends in
      <code>?token=…</code> and signs this browser in. The token is kept between restarts, so you only need to do this once.
    </p>
    <button type="button" onclick={() => store.refresh()}>Try again</button>
  </main>
{:else}
<div class="app">
  <Sidebar {store} />
  <div class="main">
    <Toolbar {store} />
    <main class="stage">
      {#each live as s (s.id)}
        <div class="pane" hidden={s.id !== store.activeId}>
          <Terminal sessionId={s.id} onexit={(code) => store.markExited(s.id, code)} />
        </div>
      {/each}
      {#if store.active?.lost}
        <p class="empty">This expedition was running when Dungeon Master last stopped, so its terminal is gone. Close it to dismiss.</p>
      {:else if !store.active}
        <p class="empty">
          {store.campaign ? 'Pick a hero above to start an expedition.' : 'Add a campaign on the left to begin.'}
        </p>
      {/if}
    </main>
  </div>
  <QuestsPanel {store} />
</div>
{/if}
