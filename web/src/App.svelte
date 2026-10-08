<script lang="ts">
  import { onMount } from 'svelte'
  import { AppStore } from './lib/store.svelte'
  import Toolbar from './lib/Toolbar.svelte'
  import Tabs from './lib/Tabs.svelte'
  import Terminal from './lib/Terminal.svelte'

  let { store = new AppStore() }: { store?: AppStore } = $props()

  onMount(() => {
    store.refresh()
  })
</script>

<div class="app">
  <Toolbar {store} />
  <Tabs sessions={store.sessions} activeId={store.activeId} onselect={(id) => store.select(id)} onclose={(id) => store.close(id)} />
  <main class="stage">
    {#each store.sessions as s (s.id)}
      <div class="pane" hidden={s.id !== store.activeId}>
        <Terminal sessionId={s.id} onexit={(code) => store.markExited(s.id, code)} />
      </div>
    {:else}
      <p class="empty">Pick a hero above to start a session.</p>
    {/each}
  </main>
</div>
