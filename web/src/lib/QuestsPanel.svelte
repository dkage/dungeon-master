<script lang="ts">
  import type { AppStore } from './store.svelte'
  import { STATE_LABEL, counts, labelText, scopeText, timeAgo, typeChips, visibleRows } from './quests'
  import QuestDetail from './QuestDetail.svelte'
  import Rations from './Rations.svelte'
  import ScopePicker from './ScopePicker.svelte'
  import Term from './Term.svelte'
  import { wordLabel } from './words'

  let { store }: { store: AppStore } = $props()

  let text = $state('')
  let hideCleared = $state(false)
  let types = $state<string[]>([])

  // A different campaign starts with a clean filter.
  $effect(() => {
    void store.campaignId
    types = []
    text = ''
  })

  const result = $derived(store.quests)
  const rows = $derived(result ? visibleRows(result.quests, { text, hideCleared, labels: types }) : [])
  const tally = $derived(result ? counts(result.quests) : null)
  const chips = $derived(result ? typeChips(result.quests) : [])
  const openShown = $derived(result ? result.quests.filter((q) => !q.closed).length : 0)
  const asking = $derived(store.questsError?.code === 'scope_needed' || store.pickingScope)

  function toggleType(name: string) {
    types = types.includes(name) ? types.filter((t) => t !== name) : [...types, name]
  }
  const demo = $derived(store.campaign?.demo === true)
  const sample = $derived(store.campaign?.sample === true)
</script>

<aside class="quests" class:open={store.questsOpen} aria-label={wordLabel('quest', store.appearance.wordMode, true)}>
  <section class="quest-browser" aria-label="Quest browser">
    <header class="qhead">
      <div class="qtitle">
        <h2><Term word="quest" mode={store.appearance.wordMode} /></h2>
        {#if result}
          <span class="source" class:demo={result.source === 'demo'}>
            {sample ? 'sample project' : result.source === 'demo' ? 'sample data' : result.repo}
          </span>
        {/if}
      </div>
      <div class="qtools">
        {#if result}<span class="age">{timeAgo(result.fetchedAt)}</span>{/if}
        <button
          type="button"
          class="ghost"
          aria-label={`Refresh ${wordLabel('quest', store.appearance.wordMode, true).toLowerCase()}`}
          disabled={store.questsLoading || !store.campaign}
          onclick={() => store.loadQuests(true)}
        >
          {store.questsLoading ? '…' : '↻'}
        </button>
        <button type="button" class="ghost close-drawer" aria-label="Close quests" onclick={() => (store.questsOpen = false)}>×</button>
      </div>
    </header>

    {#if !demo && store.github && store.github.rations.length > 0}
      <div class="budgetline"><Rations status={store.github} /></div>
    {/if}
    {#if !demo && store.github?.camp.active}
      <p class="banner warn" role="status">
        Paused until {new Date(store.github.camp.until ?? '').toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}:
        GitHub asked this app to slow down. {result ? 'Showing saved data.' : ''}
      </p>
    {/if}
    {#if result?.scope && !demo && !store.pickingScope}
      <div class="scopeline">
        <button type="button" class="scopechip" title="Change which issues are shown" onclick={() => (store.pickingScope = true)}>
          {scopeText(result.scope)} · {openShown} of {result.totalOpen.toLocaleString()} open ▾
        </button>
      </div>
    {/if}
    {#if result?.truncated && !store.pickingScope}
      <p class="banner warn" role="status">
        Showing the newest {openShown} of {result.totalOpen.toLocaleString()} open issues. Narrow the scope to see the rest.
      </p>
    {/if}
    {#if sample}
      <p class="banner">This is a separate sample project. Its issues are local examples and do not come from GitHub.</p>
    {:else if demo}
      <p class="banner">Showing sample issues for this project.</p>
    {/if}
    {#if !demo && result?.stale}
      <p class="banner warn" role="status">{result.warning ? `${result.warning} ` : ''}Showing saved data.</p>
    {/if}

    {#if asking}
      <ScopePicker {store} />
    {:else if store.questsError}
      {@const err = store.questsError}
      <div class="problem" role="alert">
        <p>{err.message}</p>
        <div class="actions">
          {#if err.code === 'no_repo'}
            <button type="button" onclick={() => store.addSampleCampaign()}>Try the separate sample project</button>
          {:else}
            <button type="button" onclick={() => store.loadQuests(true)}>Try again</button>
            {#if err.code === 'no_token' || err.code === 'github_unauthorized' || err.code === 'repo_not_found'}
              <button type="button" class="secondary" onclick={() => store.addSampleCampaign()}>Try the separate sample project</button>
            {/if}
          {/if}
        </div>
      </div>
    {:else if !store.campaign}
      <p class="hint pad">Select a campaign to see its quests.</p>
    {:else if !result && store.questsLoading}
      <p class="hint pad" role="status">Loading {wordLabel('quest', store.appearance.wordMode, true).toLowerCase()}…</p>
    {:else if result}
      <div class="filters">
        <input type="search" placeholder="Filter by title, label or #number" aria-label={`Filter ${wordLabel('quest', store.appearance.wordMode, true).toLowerCase()}`} bind:value={text} />
        <label class="check"><input type="checkbox" bind:checked={hideCleared} /> Hide cleared</label>
      </div>
      {#if chips.length > 1}
        <div class="typechips" role="group" aria-label="Filter by label">
          {#each chips as c (c.name)}
            <button
              type="button"
              class="typechip"
              aria-pressed={types.includes(c.name)}
              style={`--chip:#${c.color ?? '999999'}`}
              onclick={() => toggleType(c.name)}
            >
              {c.name} <span class="n">{c.count}</span>
            </button>
          {/each}
        </div>
      {/if}
      {#if tally}
        <p class="tally" aria-label={`${wordLabel('quest', store.appearance.wordMode, true)} counts`}>
          <span class="s open">{tally.open} open</span>
          <span class="s claimed">{tally.claimed} in progress</span>
          <span class="s locked">{tally.locked} locked</span>
          <span class="s cleared">{tally.cleared} cleared</span>
        </p>
      {/if}

      {#if result.quests.length === 0}
        <p class="hint pad">No issues in this repository yet.</p>
      {:else if rows.length === 0}
        <p class="hint pad">No {store.appearance.wordMode === 'plain' ? 'issues' : 'quests'} match.</p>
      {:else}
        <ul class="qlist">
          {#each rows as { quest: q, depth } (q.number)}
            <li class="qitem {q.state}" style={`--depth:${depth}`}>
              <button type="button" class="qbtn" class:selected={store.questNumber === q.number} aria-current={store.questNumber === q.number ? 'true' : undefined} onclick={() => store.selectQuest(q.number)}>
                <span class="state {q.state}" title={STATE_LABEL[q.state]}>
                  <span class="sr">{STATE_LABEL[q.state]}:</span>
                </span>
                <span class="qmain">
                  <span class="qline">
                    <span class="num">#{q.number}</span>
                    <span class="qname">{q.title}</span>
                  </span>
                  <span class="meta">
                    {#each q.labels as l (l.name)}
                      <span class="label" style={`background:#${l.color ?? '999999'};color:${labelText(l.color)}`}>{l.name}</span>
                    {/each}
                    {#if q.subTotal > 0}<span class="chip" title="Sub-issues done">{q.subDone}/{q.subTotal}</span>{/if}
                    {#if q.state === 'locked'}
                      <span class="chip lock" title="Blocked by open issues">
                        blocked by {q.blockedBy.filter((b) => !b.closed).map((b) => `#${b.number}`).join(', ')}
                      </span>
                    {/if}
                    {#each q.assignees as a (a)}<span class="chip">@{a}</span>{/each}
                    {#each q.prs as p (p.number)}<span class="chip pr {p.state}">PR #{p.number} {p.state}</span>{/each}
                  </span>
                </span>
              </button>
            </li>
          {/each}
        </ul>
      {/if}
    {/if}
  </section>

  {#if store.questNumber !== null}
    <section class="quest-detail-slot" aria-label="Selected issue detail">
      <QuestDetail {store} />
    </section>
  {:else}
    <section class="quest-detail-slot detail-empty" aria-label="Issue details">
      <p class="kicker">Quest detail</p>
      <h2>Select an issue</h2>
      <p>Choose an issue above to read its description, blockers, and related work.</p>
    </section>
  {/if}
</aside>
