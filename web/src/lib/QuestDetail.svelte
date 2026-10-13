<script lang="ts">
  import ExpandButton from './ExpandButton.svelte'
  import { tick } from 'svelte'
  import type { AppStore } from './store.svelte'
  import type { QuestRef } from './api'
  import type { LaunchPreview } from './api'
  import type { EffortTier } from './api'
  import { STATE_LABEL, labelStyle, timeAgo } from './quests'
  import { TIERS, effortWord } from './map/art'
  import { renderMarkdown } from './markdown'
  import { wordLabel } from './words'

  let { store }: { store: AppStore } = $props()

  const d = $derived(store.detail)
  const body = $derived(d ? renderMarkdown(d.body) : '')
  const safeUrl = $derived(d && /^https:\/\//i.test(d.url) && !store.campaign?.demo ? d.url : '')
  const existingQuestSession = $derived(store.campaignId ? store.questSessionForCampaign(store.campaignId) : undefined)
  const blockedBySession = $derived(existingQuestSession && !store.campaign?.parallelQuests ? existingQuestSession : undefined)
  const inList = $derived(d ? store.mapModel.quests.get(d.number) : undefined)
  const openBlockers = $derived(d ? d.blockedBy.filter((b) => !b.closed) : [])
  const parentInScope = $derived(d?.parent ? store.mapModel.quests.has(d.parent) : true)
  let jumpOpen = $state(false)
  let newLabel = $state('')
  let newTier = $state<EffortTier>('moderate')
  const mapping = $derived(store.campaign?.effortLabels ?? {})
  const EFFORT_SOURCE = { override: 'set here', label: 'from its label', mapping: 'from a mapped label' }

  function jumpToPrerequisite() {
    if (openBlockers.length === 1) void store.jumpToQuest(openBlockers[0].number)
    else jumpOpen = !jumpOpen
  }

  function changeEffort(value: string) {
    if (!d) return
    void store.setEffortOverride(d.number, value as EffortTier | '')
  }

  function addMapping() {
    if (!newLabel) return
    void store.setEffortLabels({ ...mapping, [newLabel.toLowerCase()]: newTier })
    newLabel = ''
  }

  function removeMapping(label: string) {
    const next = { ...mapping }
    delete next[label]
    void store.setEffortLabels(next)
  }

  let briefDialog = $state<HTMLDialogElement>()
  let launchPreview = $state<LaunchPreview | null>(null)
  let selectedHero = $state('')
  let previewBusy = $state(false)

  $effect(() => {
    const campaign = store.campaignId ?? ''
    let remembered = ''
    try { remembered = localStorage.getItem(`dungeon.hero.${campaign}`) ?? '' } catch { /* storage unavailable */ }
    selectedHero = store.heroes.some((h) => h.id === remembered) ? remembered
      : store.heroes.some((h) => h.id === store.defaultHero) ? store.defaultHero : store.heroes[0]?.id ?? ''
    launchPreview = null
  })

  async function openBrief() {
    if (!d || !selectedHero || previewBusy) return
    previewBusy = true
    launchPreview = null
    const result = await store.previewQuestLaunch(d.number, selectedHero)
    launchPreview = result
    previewBusy = false
    if (!result) return
    await tick()
    if (typeof briefDialog?.showModal === 'function') briefDialog.showModal()
    else briefDialog?.setAttribute('open', '')
  }

  function changeHero(value: string) {
    selectedHero = value
    launchPreview = null
    try { if (store.campaignId) localStorage.setItem(`dungeon.hero.${store.campaignId}`, value) } catch { /* storage unavailable */ }
  }

  async function startQuest() {
    if (!d || !launchPreview?.canStart || !selectedHero || store.busy) return
    if (await store.launchQuest(d.number, selectedHero, launchPreview.briefingId)) closeBrief()
  }

  function closeBrief() {
    if (briefDialog?.open && typeof briefDialog.close === 'function') briefDialog.close()
    else {
      briefDialog?.removeAttribute('open')
    }
  }
</script>

{#snippet refs(title: string, list: QuestRef[])}
  {#if list.length > 0}
    <section class="rel">
      <h3>{title}</h3>
      <ul>
        {#each list as r (r.number)}
          <li>
            <button type="button" class="link" onclick={() => store.jumpToQuest(r.number)}>
              <span class="mark" class:done={r.closed}>{r.closed ? '✓' : '○'}</span>
              #{r.number}{r.title ? ` ${r.title}` : ''}
            </button>
          </li>
        {/each}
      </ul>
    </section>
  {/if}
{/snippet}

<div class="detail">
  <header class="qhead">
    <button type="button" class="back" onclick={() => store.closeQuest()}>← {wordLabel('quest', store.appearance.wordMode, true)}</button>
    <ExpandButton {store} region="detail" />
    <button type="button" class="ghost close-drawer" aria-label="Close quests" onclick={() => (store.questsOpen = false)}>×</button>
  </header>

  {#if store.detailError}
    <div class="problem" role="alert">
      <p>{store.detailError.message}</p>
      <div class="actions">
        <button type="button" onclick={() => store.questNumber !== null && store.selectQuest(store.questNumber)}>Try again</button>
      </div>
    </div>
  {:else if !d}
    <p class="hint pad" role="status">Loading #{store.questNumber}…</p>
  {:else}
    <article>
      <div class="titlecard">
        <h2 class="dtitle"><span class="num">#{d.number}</span> {d.title}</h2>
      </div>
      <p class="dmeta">
        <span class="pill {d.state}">{STATE_LABEL[d.state]}{d.reason ? ` · ${d.reason}` : ''}</span>
        {#if d.effort}<span class="pill effort-chip" title="Effort">{effortWord(d.effort)}</span>{/if}
        {#if d.effort && d.effortSource}<span class="pill source-chip" title="Where the effort comes from">{EFFORT_SOURCE[d.effortSource]}</span>{/if}
        {#if d.author}<span>opened by @{d.author}</span>{/if}
        <span>updated {timeAgo(d.updated)}</span>
        {#if safeUrl}<a href={safeUrl} target="_blank" rel="noopener noreferrer">Open on GitHub</a>{/if}
      </p>

      {#if d.labels.length > 0 || d.assignees.length > 0 || d.milestone}
        <p class="meta">
          {#each d.labels as l (l.name)}
            <span class="label" style={labelStyle(l.color)}><i class="swatch"></i>{l.name}</span>
          {/each}
          {#each d.assignees as a (a)}<span class="chip">@{a}</span>{/each}
          {#if d.milestone}<span class="chip">milestone: {d.milestone}</span>{/if}
        </p>
      {/if}

      {#if !inList && store.quests}
        <p class="scope-note" role="note">This issue is outside the current scope; it is shown because another issue points to it.</p>
      {/if}

      <section class="rel effort-row" aria-label="Effort">
        <h3>Effort</h3>
        <p>
          <strong>{effortWord(d.effort)}</strong>
          {#if d.effort && d.effortSource}<span class="hint">({EFFORT_SOURCE[d.effortSource]})</span>{/if}
          {#if !d.effort && !d.effortConflict?.length}<span class="hint">No effort label, so no creature is chosen.</span>{/if}
        </p>
        {#if d.effortConflict?.length}
          <p class="conflict" role="note">These labels disagree: {d.effortConflict.join(', ')}. Remove one on GitHub, or set the effort here.</p>
        {/if}
        {#if d.priorityConflict?.length}
          <p class="conflict" role="note">These priority labels disagree: {d.priorityConflict.join(', ')}.</p>
        {/if}
        <label class="effort-set">
          <span>Set here</span>
          <select value={store.campaign?.effortOverrides?.[d.number] ?? ''} onchange={(e) => changeEffort(e.currentTarget.value)}>
            <option value="">From labels</option>
            {#each TIERS as tier (tier)}<option value={tier}>{effortWord(tier)}</option>{/each}
          </select>
        </label>
        <details class="effort-map">
          <summary>Effort labels for this project</summary>
          <p class="hint">Labels named <code>effort:trivial</code> to <code>effort:extreme</code> work everywhere. Map your own labels here; this stays on this computer.</p>
          {#if Object.keys(mapping).length > 0}
            <ul>
              {#each Object.entries(mapping) as [label, tier] (label)}
                <li><code>{label}</code> → {effortWord(tier)} <button type="button" class="ghost" aria-label={`Stop mapping ${label}`} onclick={() => removeMapping(label)}>×</button></li>
              {/each}
            </ul>
          {/if}
          {#if d.labels.length > 0}
            <div class="map-add">
              <select aria-label="Label" bind:value={newLabel}>
                <option value="">Choose a label…</option>
                {#each d.labels.filter((l) => !mapping[l.name.toLowerCase()]) as l (l.name)}<option value={l.name}>{l.name}</option>{/each}
              </select>
              <select aria-label="Effort for that label" bind:value={newTier}>
                {#each TIERS as tier (tier)}<option value={tier}>{effortWord(tier)}</option>{/each}
              </select>
              <button type="button" onclick={addMapping} disabled={!newLabel}>Map</button>
            </div>
          {/if}
        </details>
      </section>

      {#if d.parent}
        <section class="rel">
          <h3>Part of</h3>
          <button type="button" class="link" onclick={() => d.parent && store.jumpToQuest(d.parent)}>
            #{d.parent}{d.parentTitle ? ` ${d.parentTitle}` : ''}
          </button>
          {#if !parentInScope}<span class="hint"> (not in scope)</span>{/if}
        </section>
      {/if}
      {#if openBlockers.length > 0}
        <div class="jump">
          <button type="button" class="jump-button" aria-expanded={openBlockers.length > 1 ? jumpOpen : undefined} onclick={jumpToPrerequisite}>
            Jump to prerequisite{openBlockers.length === 1 ? ` #${openBlockers[0].number}` : ''}
          </button>
          {#if jumpOpen && openBlockers.length > 1}
            <ul class="jump-list" aria-label="Prerequisites">
              {#each openBlockers as b (b.number)}
                <li><button type="button" class="link" onclick={() => { jumpOpen = false; void store.jumpToQuest(b.number) }}>#{b.number}{b.title ? ` ${b.title}` : ''}</button></li>
              {/each}
            </ul>
          {/if}
        </div>
      {/if}
      {@render refs('Blocked by', d.blockedBy)}
      {@render refs('Blocking', d.blocking)}
      {@render refs(`Sub-issues (${d.subDone}/${d.subTotal})`, d.subIssues)}

      {#if d.prs.length > 0}
        <section class="rel">
          <h3>Pull requests</h3>
          <ul>
            {#each d.prs as p (p.number)}
              <li><span class="chip pr {p.state}">PR #{p.number} {p.state}</span></li>
            {/each}
          </ul>
        </section>
      {/if}

      <section class="md" aria-label="Description">
        {#if body}
          <!-- Sanitised by renderMarkdown; nothing else may be passed to {@html}. -->
          {@html body}
        {:else}
          <p class="hint">No description.</p>
        {/if}
      </section>

      {#if d.commentList.length > 0}
        <section class="comments" aria-label="Comments">
          <h3>Comments ({d.comments})</h3>
          {#each d.commentList as c, i (i)}
            <div class="comment">
              <p class="who"><strong>@{c.author}</strong> <span>{timeAgo(c.created)}</span></p>
              <div class="md">{@html renderMarkdown(c.body)}</div>
            </div>
          {/each}
        </section>
      {/if}
    </article>
    <footer class="launch-footer">
      <label class="hero-label" for="quest-hero">Hero</label>
      <select id="quest-hero" value={selectedHero} onchange={(event) => changeHero(event.currentTarget.value)} disabled={store.heroes.length === 0 || store.busy}>
        {#if store.heroes.length === 0}<option value="">No heroes configured</option>{/if}
        {#each store.heroes as h (h.id)}<option value={h.id}>{h.label}</option>{/each}
      </select>
      <button type="button" class="launch-button" onclick={openBrief} disabled={!d || d.state === 'cleared' || d.state === 'locked' || !selectedHero || previewBusy || store.busy || !!blockedBySession}>
        {previewBusy ? 'Preparing…' : 'Review launch'}
      </button>
      {#if blockedBySession}
        <p class="launch-note">{blockedBySession.title} is already running in this working tree.</p>
        <button type="button" class="go-session" onclick={() => store.goToSession(blockedBySession.id)}>Go to session</button>
      {:else if d.state === 'locked'}
        <p class="launch-note">
          Blocked by
          {#each openBlockers as b, i (b.number)}{i > 0 ? ', ' : ''}<button type="button" class="link" onclick={() => store.jumpToQuest(b.number)}>#{b.number}</button>{/each}.
          It can start once {openBlockers.length === 1 ? 'that is' : 'those are'} closed.
        </p>
      {:else if d.state === 'cleared'}<p class="launch-note">Closed issues cannot be started.</p>
      {:else if !store.claude.found}<p class="launch-note">Claude Code is unavailable. Check Settings.</p>
      {:else if d.assignees.length > 0}<p class="launch-note">Assigned to {d.assignees.map((a) => `@${a}`).join(', ')}.</p>{/if}
    </footer>
    <dialog bind:this={briefDialog} class="brief-dialog" aria-labelledby="brief-title">
      <header>
        <div><p class="eyebrow">#{d.number} · {launchPreview?.heroLabel ?? ''}</p><h2 id="brief-title">Review launch plan</h2></div>
        <button type="button" class="close" aria-label="Close briefing" onclick={closeBrief}>×</button>
      </header>
      {#if launchPreview}
        <p class="brief-summary">Prepared from {launchPreview.commentsShown} of {launchPreview.commentsTotal} comments{launchPreview.truncated ? ' · some context was shortened' : ''}.</p>
        {#if launchPreview.reason}<p class="launch-note" role="status">{launchPreview.reason}</p>{/if}
        {#if launchPreview.existingSession}<button type="button" class="go-session" onclick={() => store.goToSession(launchPreview!.existingSession!.id)}>Go to {launchPreview.existingSession.title}</button>{/if}
        <dl class="plan-meta">
          {#if launchPreview.directory}<dt>Working directory</dt><dd>{launchPreview.directory}</dd>{/if}
          {#if launchPreview.briefingFile}<dt>Private briefing</dt><dd>{launchPreview.briefingFile}</dd>{/if}
          {#if launchPreview.opener}<dt>Opening instruction</dt><dd>{launchPreview.opener}</dd>{/if}
        </dl>
        <details class="command"><summary>Command and agent instruction</summary><pre>{JSON.stringify(launchPreview.command ?? [], null, 2)}\n\n{launchPreview.systemPrompt ?? ''}</pre></details>
        <h3 class="brief-label">Task briefing sent to the agent</h3>
        <pre class="brief-content">{launchPreview.content}</pre>
        <footer class="dialog-actions">
          <button type="button" class="cancel" onclick={closeBrief}>Cancel</button>
          {#if launchPreview.existingSession}
            <button type="button" class="launch-button" onclick={() => store.goToSession(launchPreview!.existingSession!.id)}>Go to session</button>
          {:else}<button type="button" class="launch-button" onclick={startQuest} disabled={!launchPreview.canStart || store.busy}>{store.busy ? 'Starting…' : 'Start expedition'}</button>{/if}
        </footer>
      {/if}
    </dialog>
  {/if}
</div>

<style>
  .scope-note, .conflict { margin: 8px 0; padding: 6px 8px; border: 1px solid var(--line); border-radius: 6px; color: var(--warn); background: var(--warn-bg); font-size: 12px; }
  .effort-row { margin: 10px 0; }
  .effort-row h3 { margin: 0 0 4px; }
  .effort-row p { margin: 0 0 4px; }
  .effort-row strong { color: var(--text); }
  .effort-row .hint { color: var(--muted); font-size: 12px; }
  .effort-set { display: inline-flex; align-items: center; gap: 6px; font-size: 12px; color: var(--muted); }
  .effort-set select, .map-add select { padding: 3px 6px; border: 1px solid var(--line); border-radius: 6px; background: var(--bg); color: var(--text); font: inherit; font-size: 12px; }
  .effort-map { margin-top: 6px; font-size: 12px; }
  .effort-map summary { cursor: pointer; color: var(--accent); }
  .effort-map ul { margin: 4px 0; padding-left: 16px; }
  .map-add { display: flex; gap: 6px; flex-wrap: wrap; margin-top: 4px; }
  .map-add button { padding: 3px 10px; border: 1px solid var(--line); border-radius: 6px; background: transparent; color: var(--text); }
  .jump { margin: 8px 0 0; }
  .jump-button { padding: 4px 10px; border: 1px solid var(--accent); border-radius: 6px; background: transparent; color: var(--accent); font: inherit; font-size: 12px; cursor: pointer; }
  .jump-list { margin: 4px 0 0; padding-left: 16px; }
  /* Two columns: the hero on the left, a place kept free on the right for the skill selector (#50). */
  .launch-footer { flex: 0 0 auto; display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); grid-template-areas: 'hlabel slabel' 'hero skill' 'note go'; align-items: center; gap: 4px 12px; padding: 10px 12px; border-top: 1px solid var(--line); background: var(--panel); }
  .launch-footer label { grid-area: hlabel; color: var(--muted); font-size: 11px; }
  .launch-footer select { grid-area: hero; min-width: 0; border: 1px solid var(--line); border-radius: 7px; padding: 7px 9px; background: var(--bg); color: var(--text); font: inherit; }
  .launch-button { grid-area: go; justify-self: end; border: 1px solid #a34c39; border-radius: 7px; padding: 7px 12px; background: linear-gradient(#8b3025, #5d1718); color: #f4dfbd; font: inherit; cursor: pointer; }
  .launch-button:disabled { opacity: .5; cursor: not-allowed; }
  .go-session { grid-column: 1 / -1; justify-self: end; border: 1px solid var(--accent); border-radius: 7px; padding: 6px 10px; background: transparent; color: var(--accent); cursor: pointer; }
  .launch-note { grid-area: note; margin: 0; color: var(--muted); font-size: 11px; }
  .brief-dialog { width: min(820px, calc(100vw - 32px)); max-height: min(88vh, 880px); padding: 0; border: 1px solid var(--line); border-radius: 12px; background: var(--panel); color: var(--text); box-shadow: 0 24px 80px #0009; overflow: auto; }
  .brief-dialog::backdrop { background: #0009; backdrop-filter: blur(3px); }
  .brief-dialog > header { position: sticky; top: 0; display: flex; align-items: center; justify-content: space-between; padding: 16px 20px; border-bottom: 1px solid var(--line); background: var(--panel); }
  .brief-dialog .eyebrow { margin: 0 0 4px; color: var(--muted); font-size: 11px; }
  .brief-dialog h2 { margin: 0; font-size: 20px; }
  .brief-dialog .close { width: 32px; height: 32px; border: 1px solid var(--line); border-radius: 8px; background: transparent; color: var(--text); font-size: 22px; }
  .brief-summary { padding: 0 20px; color: var(--muted); font-size: 12px; }
  .plan-meta { display: grid; grid-template-columns: 130px minmax(0,1fr); gap: 5px 12px; margin: 12px 20px; font-size: 12px; }
  .plan-meta dt { color: var(--muted); }
  .plan-meta dd { margin: 0; overflow-wrap: anywhere; }
  .command { margin: 12px 20px; font-size: 12px; }
  .command summary { cursor: pointer; color: var(--accent); }
  .brief-label { margin: 16px 20px 8px; font-size: 14px; }
  .brief-dialog pre { margin: 0 20px 20px; max-height: 40vh; overflow: auto; padding: 14px; border: 1px solid var(--line); border-radius: 8px; background: var(--bg); color: var(--text); white-space: pre-wrap; overflow-wrap: anywhere; font: 12px/1.55 ui-monospace, SFMono-Regular, Menlo, monospace; }
  .dialog-actions { position: sticky; bottom: 0; display: flex; justify-content: flex-end; gap: 8px; padding: 12px 20px; border-top: 1px solid var(--line); background: var(--panel); }
  .dialog-actions .cancel { border: 1px solid var(--line); border-radius: 7px; padding: 7px 12px; background: transparent; color: var(--text); }
</style>
