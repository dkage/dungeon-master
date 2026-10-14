<script lang="ts">
  import type { AppStore } from './store.svelte'
  let { store }: { store: AppStore } = $props()
  const session = $derived(store.stopping ? store.sessions.find((s) => s.id === store.stopping) : undefined)
</script>

{#if session}
  <div class="stop-confirm" role="group" aria-label={`Confirm stopping ${session.title}`}>
    <p>{session.title} is still working. Closing it stops the agent and anything it started, and unfinished work in the session is lost.</p>
    <button type="button" class="danger" onclick={() => store.confirmStop()}>Stop and close</button>
    <button type="button" class="secondary" onclick={() => store.cancelStop()}>Keep running</button>
  </div>
{/if}

<style>
  .stop-confirm { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; padding: 8px 16px; border-bottom: 1px solid var(--line); background: var(--warn-bg); color: var(--warn); }
  .stop-confirm p { margin: 0; flex: 1 1 24ch; font-size: 12px; }
  .stop-confirm button { border: 1px solid var(--line); border-radius: 5px; padding: 4px 10px; background: transparent; color: var(--text); font: inherit; font-size: 12px; cursor: pointer; }
  .stop-confirm .danger { border-color: var(--error); color: var(--error); }
</style>
