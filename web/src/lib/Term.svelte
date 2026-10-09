<script lang="ts">
  import { WORDS, type WordKey } from './words'
  import type { WordMode } from './store.svelte'

  let { word, mode = 'hints' }: { word: WordKey; mode?: WordMode } = $props()
  let open = $state(false)
  const info = $derived(WORDS[word])
  const label = $derived(mode === 'plain' ? info.plain : info.themed)

  function keydown(event: KeyboardEvent) {
    if (event.key === 'Escape') open = false
  }
</script>

<span class="term-wrap">
  <button
    type="button"
    class="term-word"
    aria-expanded={open}
    aria-label={`${label}: ${info.meaning}`}
    onclick={() => (open = !open)}
    onkeydown={keydown}
  >
    {label}{#if mode === 'hints'} <span class="term-plain">({info.plain.toLowerCase()})</span>{/if}
  </button>
  {#if open}<span class="term-popover" role="status">{info.meaning}</span>{/if}
</span>

<style>
  .term-wrap { position: relative; display: inline; }
  .term-word { display: inline; padding: 0; border: 0; border-bottom: 1px dotted currentColor; color: inherit; background: none; font: inherit; cursor: help; }
  .term-plain { color: var(--muted); font-size: .9em; font-weight: 400; }
  .term-popover { position: absolute; z-index: 50; left: 0; top: calc(100% + 5px); width: max-content; max-width: min(280px, 80vw); padding: 8px 10px; border: 1px solid var(--line); border-radius: 6px; background: var(--panel); color: var(--text); box-shadow: 0 6px 20px #0005; font-size: 13px; font-weight: 400; letter-spacing: normal; text-transform: none; }
</style>
