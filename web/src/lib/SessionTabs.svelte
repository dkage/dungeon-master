<script lang="ts">
  import { tick } from 'svelte'
  import type { AppStore } from './store.svelte'
  import { sessionStatus } from './attention'
  import { sessionTabs, tabLabel, tabTarget } from './sessionTabs'

  let { store }: { store: AppStore } = $props()
  const tabs = $derived(sessionTabs(store.sessions, store.campaignId))
  const heroName = (hero?: string) => store.heroes.find((h) => h.id === hero)?.label ?? ''
  let strip: HTMLDivElement | undefined = $state()

  // The active tab stays in view when there are more tabs than room.
  $effect(() => {
    const id = store.activeId
    if (!strip || !id) return
    void tick().then(() => strip?.querySelector<HTMLElement>(`[data-tab="${CSS.escape(id)}"]`)?.scrollIntoView?.({ inline: 'nearest', block: 'nearest' }))
  })

  function key(ev: KeyboardEvent, index: number) {
    const to = tabTarget(ev.key, index, tabs.length)
    if (to === null) return
    ev.preventDefault()
    const target = tabs[to]
    void store.selectSession(target.id)
    void tick().then(() => strip?.querySelector<HTMLElement>(`[data-tab="${CSS.escape(target.id)}"]`)?.focus())
  }
</script>

{#if tabs.length > 0}
  <div class="tabs" role="tablist" aria-label="Sessions in this project" bind:this={strip}>
    {#each tabs as s, i (s.id)}
      {@const st = sessionStatus(s, store.appearance.wordMode)}
      <div class="tab" class:active={s.id === store.activeId} class:needs={st.kind === 'needs'}>
        <button
          type="button"
          role="tab"
          class="label"
          data-tab={s.id}
          aria-selected={s.id === store.activeId}
          tabindex={s.id === store.activeId || (!tabs.some((t) => t.id === store.activeId) && i === 0) ? 0 : -1}
          onclick={() => store.selectSession(s.id)}
          onkeydown={(ev) => key(ev, i)}
        >
          <span class="dot {st.kind}" aria-hidden="true">{st.glyph}</span>
          <span class="text">{tabLabel(s, heroName(s.hero))}</span>
          {#if s.branch}<span class="branch" aria-hidden="true">⎇ {s.branch.replace(/^dungeon\//, '')}</span>{/if}
          <span class="sr">{st.label}</span>
        </button>
        <button type="button" class="x" aria-label={`${s.title}: close session`} tabindex="-1" onclick={() => store.requestClose(s.id)}>×</button>
      </div>
    {/each}
  </div>
{/if}

<style>
  .tabs { display: flex; flex-wrap: nowrap; gap: 2px; overflow-x: auto; overflow-y: hidden; padding: 4px 8px 0; border-bottom: 1px solid var(--line); background: var(--panel); scrollbar-width: thin; }
  .tab { display: inline-flex; align-items: center; flex: none; max-width: 30ch; border: 1px solid var(--line); border-bottom: 0; border-radius: 6px 6px 0 0; background: transparent; color: var(--muted); }
  .tab.active { background: var(--bg); color: var(--text); border-color: var(--accent); }
  .tab.needs:not(.active) { border-color: #e5a50a; }
  .label { display: inline-flex; align-items: center; gap: 6px; min-width: 0; padding: 4px 4px 4px 10px; border: 0; background: transparent; color: inherit; font: inherit; font-size: 12px; cursor: pointer; }
  .text { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .branch { color: var(--muted); font-size: 11px; white-space: nowrap; }
  .x { border: 0; background: transparent; color: var(--muted); padding: 2px 8px; font: inherit; cursor: pointer; }
  .x:hover { color: var(--text); }
  .label:focus-visible, .x:focus-visible { outline: 2px solid var(--accent); outline-offset: -2px; }
  .dot { width: 9px; height: 9px; border-radius: 50%; flex: none; background: var(--muted); }
  .dot.running { background: #2fb36a; }
  .dot.failed { background: #d64545; }
  .dot.lost { background: transparent; border: 1.5px solid var(--muted); }
  .dot.idle { background: transparent; border: 1.5px solid #2fb36a; }
  .dot.needs { width: 15px; height: 15px; background: #e5a50a; color: #1b1405; font-size: 11px; font-weight: 700; line-height: 15px; text-align: center; }
  .sr { position: absolute; width: 1px; height: 1px; overflow: hidden; clip: rect(0 0 0 0); white-space: nowrap; }
</style>
