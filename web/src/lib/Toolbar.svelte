<script lang="ts">
  import type { AppStore } from './store.svelte'
  import { wordLabel } from './words'
  import { sessionStatus } from './attention'
  import ExpandButton from './ExpandButton.svelte'

  let { store }: { store: AppStore } = $props()
  const campaign = $derived(store.campaign)
  const activeSession = $derived(store.active)
  const activity = $derived(activeSession && !activeSession.exited && !activeSession.lost ? sessionStatus(activeSession, store.appearance.wordMode) : null)
  const canLaunch = $derived(!store.busy && !!campaign)
</script>

<header class="toolbar">
  <div class="where">
    {#if campaign}
      <strong>{campaign.name}</strong>
      <span class="repo" class:none={!campaign.repo}>{campaign.repo ?? 'no GitHub repo'}</span>
      {#if activeSession}<span class="session-title"><span class="session-light" aria-hidden="true"></span>{activeSession.title}</span>{/if}
      {#if activity}<span class="activity-chip {activity.kind}" role="status">{activity.glyph}{activity.glyph ? ' ' : ''}{activity.label}</span>{/if}
      {#if activeSession?.quest}<button type="button" class="quest-session-chip" title={`Select issue #${activeSession.quest}`} onclick={() => { store.questsOpen = true; void store.selectQuest(activeSession.quest!) }}>#{activeSession.quest}</button>{/if}
      <code class="path" title={campaign.dir}>{campaign.dir}</code>
    {:else}
      <span class="hint">No campaign selected</span>
    {/if}
  </div>

  <div class="heroes" role="group" aria-label={wordLabel('hero', store.appearance.wordMode, true)}>
    {#each store.heroes as hero (hero.id)}
      <button
        type="button"
        class="hero"
        disabled={!canLaunch || !store.claude.found}
        title={store.claude.found ? `Start ${hero.label}` : 'Claude Code was not found'}
        onclick={() => store.launch({ hero: hero.id })}
      >
        {hero.label}
      </button>
    {/each}
    <button type="button" class="shell" disabled={!canLaunch} onclick={() => store.launch({ shell: true })}>
      Shell
    </button>
    <ExpandButton {store} region="terminal" />
    <button type="button" class="quests-toggle" aria-pressed={store.questsOpen} onclick={() => (store.questsOpen = !store.questsOpen)}>
      {wordLabel('quest', store.appearance.wordMode, true)}
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
  <p class="notice starter-heroes">
    {store.appearance.wordMode === 'plain' ? 'Using the built-in agent setups.' : 'Using the built-in heroes.'}
    <button type="button" disabled={store.busy} onclick={() => store.adoptStarterHeroes()}>
      {store.appearance.wordMode === 'plain' ? 'Save these agent setups' : 'Keep these starter heroes'}
    </button>
    Settings stay on this computer.
  </p>
{/if}
{#if store.error}
  <p class="notice error" role="alert">{store.error}</p>
{/if}
