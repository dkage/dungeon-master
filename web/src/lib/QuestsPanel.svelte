<script lang="ts">
  import type { AppStore } from './store.svelte'
  import { STATE_LABEL, counts, labelText, timeAgo, visibleRows } from './quests'
  import QuestDetail from './QuestDetail.svelte'

  let { store }: { store: AppStore } = $props()

  let text = $state('')
  let hideCleared = $state(false)

  const result = $derived(store.quests)
  const rows = $derived(result ? visibleRows(result.quests, { text, hideCleared }) : [])
  const tally = $derived(result ? counts(result.quests) : null)
  const demo = $derived(store.campaign?.demo === true)
</script>

<aside class="quests" class:open={store.questsOpen} aria-label="Quests">
  {#if store.questNumber !== null}
    <QuestDetail {store} />
  {:else}
    <header class="qhead">
      <div class="qtitle">
        <h2>Quests</h2>
        {#if result}
          <span class="source" class:demo={result.source === 'demo'}>
            {result.source === 'demo' ? 'demo data' : result.repo}
          </span>
        {/if}
      </div>
      <div class="qtools">
        {#if result}<span class="age">{timeAgo(result.fetchedAt)}</span>{/if}
        <button
          type="button"
          class="ghost"
          aria-label="Refresh quests"
          disabled={store.questsLoading || !store.campaign}
          onclick={() => store.loadQuests(true)}
        >
          {store.questsLoading ? '…' : '↻'}
        </button>
        <button type="button" class="ghost close-drawer" aria-label="Close quests" onclick={() => (store.questsOpen = false)}>×</button>
      </div>
    </header>

    {#if demo}
      <p class="banner">
        Showing sample quests.
        <button type="button" class="link" onclick={() => store.setDemo(false)}>Use real issues</button>
      </p>
    {/if}
    {#if result?.stale}
      <p class="banner warn" role="status">Showing older data. {result.warning}</p>
    {/if}

    {#if store.questsError}
      {@const err = store.questsError}
      <div class="problem" role="alert">
        <p>{err.message}</p>
        <div class="actions">
          {#if err.code === 'no_repo'}
            <button type="button" onclick={() => store.setDemo(true)}>Load demo quests</button>
          {:else}
            <button type="button" onclick={() => store.loadQuests(true)}>Try again</button>
            {#if err.code === 'no_token' || err.code === 'github_unauthorized' || err.code === 'repo_not_found'}
              <button type="button" class="secondary" onclick={() => store.setDemo(true)}>Load demo quests</button>
            {/if}
          {/if}
        </div>
      </div>
    {:else if !store.campaign}
      <p class="hint pad">Select a campaign to see its quests.</p>
    {:else if !result && store.questsLoading}
      <p class="hint pad" role="status">Loading quests…</p>
    {:else if result}
      <div class="filters">
        <input type="search" placeholder="Filter by title, label or #number" aria-label="Filter quests" bind:value={text} />
        <label class="check"><input type="checkbox" bind:checked={hideCleared} /> Hide cleared</label>
      </div>
      {#if tally}
        <p class="tally" aria-label="Quest counts">
          <span class="s open">{tally.open} open</span>
          <span class="s claimed">{tally.claimed} in progress</span>
          <span class="s locked">{tally.locked} locked</span>
          <span class="s cleared">{tally.cleared} cleared</span>
        </p>
      {/if}

      {#if result.quests.length === 0}
        <p class="hint pad">No issues in this repository yet.</p>
      {:else if rows.length === 0}
        <p class="hint pad">No quests match.</p>
      {:else}
        <ul class="qlist">
          {#each rows as { quest: q, depth } (q.number)}
            <li class="qitem {q.state}" style={`--depth:${depth}`}>
              <button type="button" class="qbtn" onclick={() => store.selectQuest(q.number)}>
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
  {/if}
</aside>
