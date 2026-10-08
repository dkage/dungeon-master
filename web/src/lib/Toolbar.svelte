<script lang="ts">
  import type { AppStore } from './store.svelte'

  let { store }: { store: AppStore } = $props()
  const campaign = $derived(store.campaign)
  const canLaunch = $derived(!store.busy && !!campaign)
</script>

<header class="toolbar">
  <div class="where">
    {#if campaign}
      <strong>{campaign.name}</strong>
      <span class="repo" class:none={!campaign.repo}>{campaign.repo ?? 'no GitHub repo'}</span>
      <code class="path" title={campaign.dir}>{campaign.dir}</code>
    {:else}
      <span class="hint">No campaign selected</span>
    {/if}
  </div>

  <div class="heroes" role="group" aria-label="Heroes">
    {#each store.heroes as hero (hero.id)}
      <button
        type="button"
        class="hero"
        disabled={!canLaunch || !store.claude.found}
        title={store.claude.found ? `Start ${hero.label}` : 'claude was not found'}
        onclick={() => store.launch({ hero: hero.id })}
      >
        {hero.label}
      </button>
    {/each}
    <button type="button" class="shell" disabled={!canLaunch} onclick={() => store.launch({ shell: true })}>
      Shell
    </button>
  </div>
</header>

{#if !store.claude.found}
  <p class="notice warn" role="alert">
    Claude Code was not found{store.claude.error ? `: ${store.claude.error}` : ''}. Install it, or set
    <code>claude = "/path/to/claude"</code> in party.toml.
  </p>
{/if}
{#if store.defaultHeroes}
  <p class="notice">
    Using the built-in heroes. Create <code>party.toml</code> in the Dungeon config directory to define your own.
  </p>
{/if}
{#if store.error}
  <p class="notice error" role="alert">{store.error}</p>
{/if}
