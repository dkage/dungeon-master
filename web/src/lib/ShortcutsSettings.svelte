<script lang="ts">
  import type { AppStore } from './store.svelte'
  import { ACTIONS, effective, formatBinding, fromEvent, labelOf, parseBinding, type ActionId } from './shortcuts'
  import { judge, testMessage, TEST_WAIT_MS, type TestState } from './shortcutTest'

  let { store }: { store: AppStore } = $props()
  let capturing = $state<ActionId | null>(null)
  let errors = $state<Partial<Record<ActionId, string>>>({})
  // The "test it" step: asks for the keys again and says whether the page received them. It runs nothing.
  let testing = $state<{ id: ActionId; state: TestState } | null>(null)
  let testTimer: ReturnType<typeof setTimeout> | undefined
  function stopTest(state?: TestState) {
    clearTimeout(testTimer)
    if (testing && state) testing = { id: testing.id, state }
    else if (!state) testing = null
  }
  function beginTest(id: ActionId) {
    clearTimeout(testTimer)
    capturing = null
    testing = { id, state: { kind: 'waiting' } }
    testTimer = setTimeout(() => stopTest({ kind: 'not-received' }), TEST_WAIT_MS)
  }
  function testKey(event: KeyboardEvent, id: ActionId) {
    if (event.key === 'Tab') return
    event.preventDefault()
    event.stopPropagation()
    const expected = current(id)
    if (!expected) return
    const result = judge(expected, event, platform)
    if (result) stopTest(result)
  }

  const platform = $derived(store.platform)
  const defaultOf = (id: ActionId) => {
    const text = ACTIONS.find((a) => a.id === id)?.default
    const b = text ? parseBinding(text, platform) : null
    return b ? formatBinding(b) : null
  }
  const current = (id: ActionId) => effective(id, store.shortcuts, platform)
  const isCustom = (id: ActionId) => id in store.shortcuts
  /** Why an action cannot run right now, or '' when it can. */
  function availability(id: ActionId): string {
    if (id in store.shortcuts && store.shortcuts[id] === null) return 'Disabled'
    if (id === 'expand-detail' && store.questNumber === null) return 'Needs an open issue'
    if (id === 'next-session' || id === 'previous-session') return store.sessions.filter((s) => !s.lost).length > 1 ? '' : 'Needs two or more sessions'
    return ''
  }

  /** A freshly inserted input is not focused by `autofocus`; do it explicitly. */
  function focusNow(node: HTMLElement) {
    node.focus()
  }
  function begin(id: ActionId) {
    errors = { ...errors, [id]: undefined }
    testing = null
    capturing = id
  }
  function capture(event: KeyboardEvent, id: ActionId) {
    // The editor owns every key while it is open; nothing here triggers an action.
    if (event.key === 'Tab') return
    event.preventDefault()
    event.stopPropagation()
    if (event.key === 'Escape' && !event.ctrlKey && !event.altKey && !event.metaKey) {
      capturing = null
      return
    }
    const b = fromEvent(event)
    if (!b) return
    const reason = store.setShortcut(id, formatBinding(b))
    if (reason) errors = { ...errors, [id]: reason }
    else {
      errors = { ...errors, [id]: undefined }
      capturing = null
    }
  }
  function reset(id: ActionId) {
    errors = { ...errors, [id]: undefined }
    capturing = null
    store.resetShortcut(id)
  }
</script>

<section aria-labelledby="shortcuts-title" class="shortcuts">
  <div class="shead">
    <div><h2 id="shortcuts-title">Keyboard shortcuts</h2><p>Each shortcut runs one workspace action. The buttons do the same.</p></div>
    <button type="button" onclick={() => { errors = {}; capturing = null; store.resetShortcuts() }}>Restore defaults</button>
  </div>
  {#if store.shortcutError}<p role="alert" class="serror">{store.shortcutError}</p>{/if}
  <table>
    <thead><tr><th>Action</th><th>Shortcut</th><th>Default</th><th>Status</th><th><span class="sr">Edit</span></th></tr></thead>
    <tbody>
      {#each ACTIONS as a (a.id)}
        {@const why = availability(a.id)}
        <tr>
          <th scope="row">{a.label}</th>
          <td>
            {#if capturing === a.id}
              <input readonly use:focusNow aria-label={`Press the new shortcut for ${a.label}`} placeholder="Press keys… (Esc cancels)" onkeydown={(e) => capture(e, a.id)} onblur={() => (capturing = null)} />
            {:else}
              <kbd>{labelOf(current(a.id), platform)}</kbd>
            {/if}
            {#if errors[a.id]}<p role="alert" class="serror">{errors[a.id]}</p>{/if}
            {#if testing?.id === a.id}
              {#if testing.state.kind === 'waiting'}
                <input readonly use:focusNow aria-label={`Press the shortcut for ${a.label} to test it`} placeholder="Press the shortcut…" onkeydown={(e) => testKey(e, a.id)} onblur={() => testing?.state.kind === 'waiting' && stopTest({ kind: 'cancelled' })} />
              {/if}
              <p role="status" class="stest" data-kind={testing.state.kind}>{testMessage(testing.state)}</p>
            {/if}
          </td>
          <td>{labelOf(defaultOf(a.id), platform)}</td>
          <td>{why || 'Ready'}</td>
          <td class="sactions">
            <button type="button" aria-label={`Change shortcut for ${a.label}`} onclick={() => begin(a.id)}>Change</button>
            <button type="button" aria-label={`Test shortcut for ${a.label}`} disabled={current(a.id) === null} onclick={() => beginTest(a.id)}>Test</button>
            <button type="button" aria-label={`Disable shortcut for ${a.label}`} disabled={current(a.id) === null} onclick={() => store.setShortcut(a.id, null)}>Disable</button>
            <button type="button" aria-label={`Reset shortcut for ${a.label}`} disabled={!isCustom(a.id)} onclick={() => reset(a.id)}>Reset</button>
          </td>
        </tr>
      {/each}
    </tbody>
  </table>
  <p class="shint">Browser and system keys (copy, paste, close, reload, screenshots) cannot be used, and keys typed in a field or the terminal keep their usual meaning. Every action also has a button.</p>
</section>

<style>
  .shead { display: flex; justify-content: space-between; align-items: flex-start; gap: 12px; }
  table { width: 100%; border-collapse: collapse; margin-top: 8px; font-size: 13px; }
  th, td { text-align: left; padding: 6px 6px; border-bottom: 1px solid var(--line); vertical-align: top; }
  thead th { color: var(--muted); font-weight: 600; }
  tbody th { font-weight: 500; }
  kbd { font: inherit; padding: 1px 6px; border: 1px solid var(--line); border-radius: 4px; background: var(--bg); white-space: nowrap; }
  input { width: 100%; box-sizing: border-box; }
  .sactions { white-space: nowrap; }
  .sactions button { padding: 2px 8px; margin-right: 4px; }
  .serror { color: var(--error); margin: 4px 0 0; }
  .stest { margin: 4px 0 0; font-size: 12px; color: var(--muted); }
  .stest[data-kind='not-received'], .stest[data-kind='different'] { color: var(--warn); }
  .shint { color: var(--muted); margin-top: 8px; font-size: 12px; }
  .sr { position: absolute; width: 1px; height: 1px; overflow: hidden; clip: rect(0 0 0 0); }
</style>
