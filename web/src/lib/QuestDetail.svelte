<script lang="ts">
  import { tick } from 'svelte'
  import type { AppStore } from './store.svelte'
  import type { QuestRef } from './api'
  import type { LaunchPreview } from './api'
  import { STATE_LABEL, labelText, timeAgo } from './quests'
  import { renderMarkdown } from './markdown'
  import { wordLabel } from './words'

  let { store }: { store: AppStore } = $props()

  const d = $derived(store.detail)
  const body = $derived(d ? renderMarkdown(d.body) : '')
  const safeUrl = $derived(d && /^https:\/\//i.test(d.url) && !store.campaign?.demo ? d.url : '')
  const existingQuestSession = $derived(store.campaignId ? store.questSessionForCampaign(store.campaignId) : undefined)
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
            <button type="button" class="link" onclick={() => store.selectQuest(r.number)}>
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
      <h2 class="dtitle"><span class="num">#{d.number}</span> {d.title}</h2>
      <p class="dmeta">
        <span class="pill {d.state}">{STATE_LABEL[d.state]}{d.reason ? ` · ${d.reason}` : ''}</span>
        {#if d.author}<span>opened by @{d.author}</span>{/if}
        <span>updated {timeAgo(d.updated)}</span>
        {#if safeUrl}<a href={safeUrl} target="_blank" rel="noopener noreferrer">Open on GitHub</a>{/if}
      </p>

      {#if d.labels.length > 0 || d.assignees.length > 0 || d.milestone}
        <p class="meta">
          {#each d.labels as l (l.name)}
            <span class="label" style={`background:#${l.color ?? '999999'};color:${labelText(l.color)}`}>{l.name}</span>
          {/each}
          {#each d.assignees as a (a)}<span class="chip">@{a}</span>{/each}
          {#if d.milestone}<span class="chip">milestone: {d.milestone}</span>{/if}
        </p>
      {/if}

      {#if d.parent}
        <section class="rel">
          <h3>Part of</h3>
          <button type="button" class="link" onclick={() => d.parent && store.selectQuest(d.parent)}>
            #{d.parent}{d.parentTitle ? ` ${d.parentTitle}` : ''}
          </button>
        </section>
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
      <label for="quest-hero">Launch with</label>
      <select id="quest-hero" value={selectedHero} onchange={(event) => changeHero(event.currentTarget.value)} disabled={store.heroes.length === 0 || store.busy}>
        {#if store.heroes.length === 0}<option value="">No heroes configured</option>{/if}
        {#each store.heroes as h (h.id)}<option value={h.id}>{h.label}</option>{/each}
      </select>
      <button type="button" class="launch-button" onclick={openBrief} disabled={!d || !selectedHero || previewBusy || store.busy || !!existingQuestSession}>
        {previewBusy ? 'Preparing…' : 'Review launch'}
      </button>
      {#if existingQuestSession}
        <p class="launch-note">{existingQuestSession.title} is already running in this working tree.</p>
        <button type="button" class="go-session" onclick={() => store.goToSession(existingQuestSession.id)}>Go to session</button>
      {:else if d.state === 'locked'}<p class="launch-note">Resolve the blockers before starting this issue.</p>
      {:else if d.state === 'cleared'}<p class="launch-note">Closed issues cannot be started.</p>
      {:else if !store.claude.found}<p class="launch-note">Claude Code is unavailable. Check Settings.</p>{/if}
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
  .launch-footer { flex: 0 0 auto; display: grid; grid-template-columns: auto minmax(0, 1fr) auto; align-items: center; gap: 8px; padding: 10px 12px; border-top: 1px solid var(--line); background: var(--panel); }
  .launch-footer label { color: var(--muted); font-size: 11px; }
  .launch-footer select { min-width: 0; border: 1px solid var(--line); border-radius: 7px; padding: 7px 9px; background: var(--bg); color: var(--text); font: inherit; }
  .launch-button { border: 1px solid #a34c39; border-radius: 7px; padding: 7px 12px; background: linear-gradient(#8b3025, #5d1718); color: #f4dfbd; font: inherit; cursor: pointer; }
  .launch-button:disabled { opacity: .5; cursor: not-allowed; }
  .go-session { border: 1px solid var(--accent); border-radius: 7px; padding: 6px 10px; background: transparent; color: var(--accent); cursor: pointer; }
  .launch-note { grid-column: 1 / -1; margin: 0; color: var(--muted); font-size: 11px; }
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
