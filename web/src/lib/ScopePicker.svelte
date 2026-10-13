<script lang="ts">
  import { onMount } from 'svelte'
  import type { Scope } from './api'
  import type { AppStore } from './store.svelte'
  import { labelStyle } from './quests'

  let { store }: { store: AppStore } = $props()

  const MARKER = 'tracked'
  const MAX_LABELS = 5

  type Choice = 'tracked' | 'assigned' | 'labels' | 'all'
  let choice = $state<Choice | null>(null)
  let picked = $state<string[]>([])
  let touched = false

  const opts = $derived(store.scopeOptions)
  const marker = $derived(opts?.labels.find((l) => l.name === MARKER))
  const pickable = $derived(opts ? opts.labels.filter((l) => l.open > 0 || l.name === MARKER) : [])
  // The marker label leads only when it already has issues (or is the current
  // choice); an empty one is an opt-in at the bottom, not the first suggestion.
  const trackedFirst = $derived((marker?.open ?? 0) > 0 || choice === 'tracked')
  const editing = $derived(store.campaign?.scope != null)

  onMount(() => {
    void store.loadScopeOptions()
  })

  // Preselect once the numbers are in: what is chosen now when changing it,
  // otherwise the marker label if it has issues, otherwise assigned-to-me.
  $effect(() => {
    if (!opts || touched) return
    const cur = store.campaign?.scope
    if (cur) {
      if (cur.mode === 'labels') {
        if (cur.labels.length === 1 && cur.labels[0].toLowerCase() === MARKER) choice = 'tracked'
        else {
          choice = 'labels'
          picked = [...cur.labels]
        }
      } else choice = cur.mode
    } else if ((marker?.open ?? 0) > 0) choice = 'tracked'
    else if (opts.assigned > 0) choice = 'assigned'
  })

  function pick(c: Choice) {
    touched = true
    choice = c
  }

  function toggle(name: string) {
    touched = true
    choice = 'labels'
    if (picked.includes(name)) picked = picked.filter((n) => n !== name)
    else if (picked.length < MAX_LABELS) picked = [...picked, name]
  }

  const scope = $derived<Scope | null>(
    choice === 'tracked'
      ? { mode: 'labels', labels: [MARKER] }
      : choice === 'assigned'
        ? { mode: 'assigned', labels: [] }
        : choice === 'all'
          ? { mode: 'all', labels: [] }
          : choice === 'labels' && picked.length > 0
            ? { mode: 'labels', labels: picked }
            : null,
  )

  let saving = $state(false)
  async function save(e: SubmitEvent) {
    e.preventDefault()
    if (!scope) return
    saving = true
    await store.chooseScope(scope)
    saving = false
  }
</script>

{#snippet trackedOpt()}
      <label class="opt">
        <input type="radio" name="scope" checked={choice === 'tracked'} onchange={() => pick('tracked')} />
        <span class="optbody">
          <span>Issues labeled <code>{MARKER}</code> <span class="count">({marker?.open ?? 0})</span></span>
          <span class="help">
            {#if (marker?.open ?? 0) === 0}None yet. This app never adds the label: add it to an issue on GitHub yourself to use this option.{:else}The work driven from here, so a busy repository stays readable.{/if}
          </span>
        </span>
      </label>
{/snippet}

<form class="scope-picker" onsubmit={save}>
  {#if store.scopeOptionsLoading && !opts}
    <p class="hint pad" role="status">Counting issues…</p>
  {:else if store.scopeOptionsError}
    <div class="problem" role="alert">
      <p>{store.scopeOptionsError.message}</p>
      <div class="actions"><button type="button" onclick={() => store.loadScopeOptions()}>Try again</button></div>
    </div>
  {:else if opts}
    <fieldset>
      <legend>
        {#if editing}Show which issues?{:else}This repository has {opts.openTotal.toLocaleString()} open issues. Show:{/if}
      </legend>

      {#if trackedFirst}{@render trackedOpt()}{/if}

      <label class="opt">
        <input type="radio" name="scope" checked={choice === 'assigned'} onchange={() => pick('assigned')} />
        <span class="optbody">
          <span>Assigned to me{opts.login ? ` (@${opts.login})` : ''} <span class="count">({opts.assigned})</span></span>
        </span>
      </label>

      <label class="opt">
        <input type="radio" name="scope" checked={choice === 'labels'} onchange={() => pick('labels')} />
        <span class="optbody">
          <span>Pick labels</span>
          <span class="help">Up to {MAX_LABELS}. Issues with any of them are shown.</span>
        </span>
      </label>
      {#if choice === 'labels'}
        <ul class="labelpick" aria-label="Labels">
          {#each pickable as l (l.name)}
            <li>
              <label>
                <input
                  type="checkbox"
                  checked={picked.includes(l.name)}
                  disabled={!picked.includes(l.name) && picked.length >= MAX_LABELS}
                  onchange={() => toggle(l.name)}
                />
                <span class="label" style={labelStyle(l.color)}><i class="swatch"></i>{l.name}</span>
                <span class="count">({l.open})</span>
              </label>
            </li>
          {/each}
        </ul>
      {/if}

      <label class="opt">
        <input type="radio" name="scope" checked={choice === 'all'} onchange={() => pick('all')} />
        <span class="optbody">
          <span>Everything <span class="count">({opts.openTotal.toLocaleString()})</span></span>
          <span class="help">Large repositories show the newest 300 open issues.</span>
        </span>
      </label>

      {#if !trackedFirst}{@render trackedOpt()}{/if}
    </fieldset>

    <div class="actions">
      <button type="submit" disabled={!scope || saving}>{editing ? 'Apply' : 'Show these issues'}</button>
      {#if editing}
        <button type="button" class="secondary" onclick={() => (store.pickingScope = false)}>Cancel</button>
      {/if}
    </div>
  {/if}
</form>
