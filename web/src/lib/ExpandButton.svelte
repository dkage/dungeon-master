<script lang="ts">
  import { tick } from 'svelte'
  import type { AppStore } from './store.svelte'
  import { expandLabel, type Region } from './workspace'

  let { store, region }: { store: AppStore; region: Region } = $props()
  const expanded = $derived(store.workspace.expanded === region)
  const label = $derived(expandLabel(region, expanded))
</script>

<button
  type="button"
  class="ghost expand-button"
  data-region={region}
  aria-label={label}
  title={label}
  aria-pressed={expanded}
  onclick={(e) => {
    const button = e.currentTarget
    store.toggleRegion(region)
    void tick().then(() => button.isConnected && button.focus())
  }}
>{expanded ? '⤡' : '⤢'}</button>
