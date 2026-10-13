<script lang="ts">
  import type { AppStore } from './store.svelte'
  import { labelStyle, scopeText, timeAgo, typeChips, visibleRows } from './quests'
  import QuestDetail from './QuestDetail.svelte'
  import QuestMap from './QuestMap.svelte'
  import { CREATURES, effortWord } from './map/art'
  import { STATE_FILTERS, STATE_WORDS, inPlace, inStateFilter, stateCounts, type ListFilter } from './map/model'
  import Rations from './Rations.svelte'
  import ScopePicker from './ScopePicker.svelte'
  import Term from './Term.svelte'
  import { wordLabel } from './words'

  let { store }: { store: AppStore } = $props()

  let text = $state('')
  let types = $state<string[]>([])
  let stateFilter = $state<ListFilter['state']>('all')
  let placeFilter = $state<ListFilter['place']>('all')

  // A different campaign starts with a clean filter.
  $effect(() => {
    void store.campaignId
    types = []
    text = ''
    stateFilter = 'all'
    placeFilter = 'all'
  })

  const result = $derived(store.quests)
  const model = $derived(store.mapModel)
  // A state or epic filter flattens the forest like a search does, so a match
  // never hangs under a parent that is filtered out.
  const narrowed = $derived(stateFilter !== 'all' || placeFilter !== 'all')
  const rows = $derived.by(() => {
    if (!result) return []
    const base = visibleRows(result.quests, { text, hideCleared: false, labels: types })
    const out = base.filter(({ quest }) => {
      const v = model.quests.get(quest.number)
      return !v || (inStateFilter(v, stateFilter) && inPlace(v, placeFilter))
    })
    return narrowed ? out.map((r) => ({ ...r, depth: 0 })) : out
  })
  const tally = $derived(stateCounts([...model.quests.values()].filter((v) => inPlace(v, placeFilter))))
  const chips = $derived(result ? typeChips(result.quests) : [])
  const view = $derived(store.panelView)
  const plain = $derived(store.appearance.wordMode === 'plain')
  const relations = (n: number) => {
    const v = model.quests.get(n)
    if (!v) return []
    const out: string[] = []
    if (v.blocks.length) out.push(`Blocks ${v.blocks.map((b) => `#${b}`).join(', ')}`)
    const open = [...v.openBlockers, ...v.outside.filter((r) => !r.closed).map((r) => r.number)]
    if (open.length) out.push(`Blocked by ${open.map((b) => `#${b}`).join(', ')}`)
    return out
  }

  // The split between the map or list and the detail, as a share of the panel.
  const SPLIT_KEY = 'dungeon.panel.split'
  let split = $state(readSplit())
  let panel = $state<HTMLElement>()
  function readSplit(): number {
    try {
      const n = Number(localStorage.getItem(SPLIT_KEY))
      return Number.isFinite(n) && n >= 20 && n <= 80 ? n : 52
    } catch {
      return 52
    }
  }
  function setSplit(n: number) {
    split = Math.max(20, Math.min(80, Math.round(n)))
    try { localStorage.setItem(SPLIT_KEY, String(split)) } catch { /* not remembered */ }
  }
  let dragging = false
  function splitDown(event: PointerEvent) {
    dragging = true
    ;(event.currentTarget as HTMLElement).setPointerCapture?.(event.pointerId)
  }
  function splitMove(event: PointerEvent) {
    if (!dragging || !panel) return
    const box = panel.getBoundingClientRect()
    if (box.height > 0) setSplit(((event.clientY - box.top) / box.height) * 100)
  }
  function splitKey(event: KeyboardEvent) {
    const step = { ArrowUp: -5, ArrowDown: 5, Home: 20 - split, End: 80 - split }[event.key]
    if (step === undefined) return
    event.preventDefault()
    setSplit(split + step)
  }
  const openShown = $derived(result ? result.quests.filter((q) => !q.closed).length : 0)
  const asking = $derived(store.questsError?.code === 'scope_needed' || store.pickingScope)

  function toggleType(name: string) {
    types = types.includes(name) ? types.filter((t) => t !== name) : [...types, name]
  }
  const demo = $derived(store.campaign?.demo === true)
  const sample = $derived(store.campaign?.sample === true)
</script>

<aside class="quests" class:open={store.questsOpen} aria-label={wordLabel('quest', store.appearance.wordMode, true)} bind:this={panel} style={`--split:${split}%`}>
  <section class="quest-browser" class:mapmode={view === 'map' && !!result && !asking} aria-label="Quest browser">
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
        <div class="viewtoggle" role="group" aria-label="Show as">
          <button type="button" aria-pressed={view === 'map'} onclick={() => store.setPanelView('map')}>Map</button>
          <button type="button" aria-pressed={view === 'list'} onclick={() => store.setPanelView('list')}>List</button>
        </div>
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
    {:else if result && view === 'map'}
      <QuestMap {store} />
    {:else if result}
      <div class="filters">
        <input type="search" placeholder="Filter by title, label or #number" aria-label={`Filter ${wordLabel('quest', store.appearance.wordMode, true).toLowerCase()}`} bind:value={text} />
        {#if model.dungeons.length > 0}
          <label class="placefilter">
            <span>{plain ? 'Epic' : 'Dungeon'}</span>
            <select bind:value={placeFilter}>
              <option value="all">All</option>
              {#each model.dungeons as d (d.id)}<option value={d.id}>#{d.id} {d.title}</option>{/each}
              <option value="tavern">{plain ? 'No epic' : 'Tavern (no epic)'}</option>
            </select>
          </label>
        {/if}
      </div>
      <div class="statefilters" role="group" aria-label="Filter by state">
        {#each STATE_FILTERS as f (f.id)}
          <button type="button" class="statefilter" aria-pressed={stateFilter === f.id} onclick={() => (stateFilter = f.id)}>
            {f.label} <span class="n">{tally[f.id]}</span>
          </button>
        {/each}
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

      {#if result.quests.length === 0}
        <p class="hint pad">No issues in this repository yet.</p>
      {:else if rows.length === 0}
        <p class="hint pad">No {store.appearance.wordMode === 'plain' ? 'issues' : 'quests'} match.</p>
      {:else}
        <ul class="qlist">
          {#each rows as { quest: q, depth } (q.number)}
            {@const v = model.quests.get(q.number)}
            {@const art = v?.creature && !plain ? CREATURES[v.creature] : undefined}
            <li class="qitem {q.state} {v?.state ?? ''}" style={`--depth:${depth}`}>
              <button type="button" class="qbtn" class:selected={store.questNumber === q.number} aria-current={store.questNumber === q.number ? 'true' : undefined} onclick={() => store.selectQuest(q.number)}>
                {#if art}
                  <span class="thumb framed" style={`background-image:url(${art.strip})`} title={`${art.name} (${effortWord(q.effort)} effort)`}></span>
                {:else}
                  <span class="thumb framed unknown" title={`${effortWord(q.effort)} effort`} aria-hidden="true">◇</span>
                {/if}
                <span class="qmain">
                  <span class="qline">
                    <span class="num">#{q.number}</span>
                    <span class="qname">{q.title}</span>
                  </span>
                  <span class="meta">
                    <span class="effort">{effortWord(q.effort)}</span>
                    {#each relations(q.number) as rel (rel)}<span class="chip rel-chip">{rel}</span>{/each}
                    {#each q.labels as l (l.name)}
                      <span class="label" style={labelStyle(l.color)}><i class="swatch"></i>{l.name}</span>
                    {/each}
                    {#if q.subTotal > 0}<span class="chip" title="Sub-issues done">{q.subDone}/{q.subTotal}</span>{/if}
                    {#each q.assignees as a (a)}<span class="chip">@{a}</span>{/each}
                    {#each q.prs as p (p.number)}<span class="chip pr {p.state}">PR #{p.number} {p.state}</span>{/each}
                  </span>
                </span>
                {#if v}<span class="statetag {v.state}">{STATE_WORDS[v.state]}</span>{/if}
              </button>
            </li>
          {/each}
        </ul>
      {/if}
    {/if}
  </section>

  <!-- svelte-ignore a11y_no_noninteractive_tabindex, a11y_no_noninteractive_element_interactions -->
  <div
    class="splitter"
    role="separator"
    aria-orientation="horizontal"
    aria-label="Resize the map and the detail"
    aria-valuemin={20}
    aria-valuemax={80}
    aria-valuenow={split}
    tabindex="0"
    onpointerdown={splitDown}
    onpointermove={splitMove}
    onpointerup={() => (dragging = false)}
    onkeydown={splitKey}
  ></div>

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
