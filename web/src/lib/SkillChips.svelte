<script lang="ts">
  import type { AppStore } from './store.svelte'
  import type { SkillCatalog } from './api'
  import { wordLabel } from './words'

  let { store }: { store: AppStore } = $props()

  let catalog = $state<SkillCatalog | null>(null)
  $effect(() => {
    void store.skillsVersion
    if (!store.campaignId) { catalog = null; return }
    let current = true
    void store.loadProjectSkills().then((c) => { if (current) catalog = c })
    return () => { current = false }
  })

  // The same skills as the Method select, with the same availability: a chip only chooses the method of the open
  // issue. It never starts a session and never types into one; launching stays with "Review launch".
  const entries = $derived(catalog?.entries ?? [])
  const number = $derived(store.questNumber)
  const chosen = $derived(store.choiceFor(number).skill)
  const reason = $derived(number === null ? 'Open an issue to choose its method.' : '')
  const label = (e: SkillCatalog['entries'][number]) => (e.shadowed ? `${e.name} (${e.sourceName})` : e.name)

  function pick(id: string) {
    if (number === null) return
    store.setChoice(number, { skill: chosen === id ? '' : id })
  }
</script>

{#if entries.length > 0}
  <div class="chips" role="group" aria-label={`${wordLabel('spell', store.appearance.wordMode, true)} for the open issue`}>
    {#each entries as e (e.id)}
      <button type="button" class="chip" class:on={chosen === e.id} aria-pressed={chosen === e.id} disabled={number === null}
        title={number === null ? reason : `${e.sourceName}${e.description ? `: ${e.description}` : ''}${e.notes?.length ? ` (${e.notes.length} note${e.notes.length === 1 ? '' : 's'})` : ''}`} onclick={() => pick(e.id)}>{label(e)}</button>
    {/each}
    {#if reason}<span class="hint">{reason}</span>{/if}
  </div>
{/if}

<style>
  .chips { flex: 0 0 auto; display: flex; align-items: center; gap: 6px; padding: 6px 12px; overflow-x: auto; border-top: 1px solid var(--line); background: var(--panel); }
  .chip { flex: 0 0 auto; padding: 3px 10px; border: 1px solid var(--line); border-radius: 999px; background: var(--bg); color: var(--text); font: inherit; font-size: 12px; cursor: pointer; white-space: nowrap; }
  .chip.on { border-color: var(--accent); color: var(--accent); }
  .chip:disabled { opacity: .55; cursor: not-allowed; }
  .hint { flex: 0 0 auto; color: var(--muted); font-size: 11px; white-space: nowrap; }
</style>
