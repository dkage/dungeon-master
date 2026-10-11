<script lang="ts">
  import { onMount } from 'svelte'
  import { WORDS } from './words'

  let { onclose }: { onclose: () => void } = $props()
  let dialog: HTMLDialogElement

  onMount(() => {
    if (typeof dialog.showModal === 'function') dialog.showModal()
    else dialog.setAttribute('open', '')
    return () => { if (dialog.open && typeof dialog.close === 'function') dialog.close() }
  })

  function close() {
    if (typeof dialog.close === 'function') dialog.close()
    else { dialog.removeAttribute('open'); onclose() }
  }
</script>

<dialog bind:this={dialog} class="words-dialog" aria-labelledby="words-title" onclose={onclose}>
  <header><div><p>Dungeon Master</p><h1 id="words-title">Word guide</h1></div><button type="button" aria-label="Close word guide" onclick={close}>×</button></header>
  <p class="intro">These labels have a plain meaning in the app. Choose a style in Settings; the popovers remain available in every mode.</p>
  <div class="table-wrap">
    <table>
      <thead><tr><th scope="col">Themed</th><th scope="col">Plain</th><th scope="col">Meaning</th></tr></thead>
      <tbody>
        {#each Object.values(WORDS) as word (word.themed)}
          <tr><th scope="row">{word.themed}</th><td>{word.plain}</td><td>{word.meaning}</td></tr>
        {/each}
      </tbody>
    </table>
  </div>
  <footer><button type="button" onclick={close}>Done</button></footer>
</dialog>

<style>
  .words-dialog { width: min(760px, calc(100vw - 32px)); max-height: min(88vh, 800px); padding: 0; border: 1px solid var(--line); border-radius: 12px; background: var(--panel); color: var(--text); box-shadow: 0 24px 80px #0009; }
  .words-dialog::backdrop { background: #0009; backdrop-filter: blur(3px); }
  header, footer { display: flex; align-items: center; justify-content: space-between; padding: 16px 20px; border-bottom: 1px solid var(--line); }
  header p { margin: 0; color: var(--muted); font-size: 11px; letter-spacing: .12em; text-transform: uppercase; }
  h1 { margin: 2px 0 0; font-size: 24px; }
  header button { width: 34px; height: 34px; border: 1px solid var(--line); border-radius: 7px; background: transparent; color: inherit; font-size: 21px; }
  .intro { margin: 0; padding: 14px 20px; color: var(--muted); }
  .table-wrap { overflow: auto; padding: 0 20px 16px; }
  table { width: 100%; border-collapse: collapse; text-align: left; }
  th, td { padding: 9px 10px; border-bottom: 1px solid var(--line); vertical-align: top; }
  thead th { color: var(--muted); font-size: 11px; letter-spacing: .08em; text-transform: uppercase; }
  tbody th { white-space: nowrap; }
  footer { justify-content: flex-end; border-top: 1px solid var(--line); border-bottom: 0; }
  footer button { padding: 7px 16px; border: 1px solid var(--accent); border-radius: 6px; background: var(--accent); color: var(--accent-text); }
</style>
