<script lang="ts">
  import { tick } from 'svelte'
  import type { AppStore } from './store.svelte'
  import type { QuestRef } from './api'
  import { STATE_LABEL, labelText, timeAgo } from './quests'
  import { renderMarkdown } from './markdown'
  import { wordLabel } from './words'

  let { store }: { store: AppStore } = $props()

  const d = $derived(store.detail)
  const body = $derived(d ? renderMarkdown(d.body) : '')
  const safeUrl = $derived(d && /^https:\/\//i.test(d.url) && !store.campaign?.demo ? d.url : '')
  let briefDialog = $state<HTMLDialogElement>()

  async function openBrief() {
    await tick()
    if (typeof briefDialog?.showModal === 'function') briefDialog.showModal()
    else briefDialog?.setAttribute('open', '')
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
        {#if d.briefing}<button type="button" class="brief-button" onclick={openBrief}>Brief</button>{/if}
      </p>

      {#if d.briefing}
        <dialog bind:this={briefDialog} class="brief-dialog" aria-labelledby="brief-title">
          <header>
            <div><p class="eyebrow">#{d.number}</p><h2 id="brief-title">Task briefing</h2></div>
            <button type="button" class="close" aria-label="Close briefing" onclick={closeBrief}>×</button>
          </header>
          <p class="brief-summary">Prepared from {d.briefing.commentsShown} of {d.briefing.commentsTotal} comments{d.briefing.truncated ? ' · some context was shortened' : ''}.</p>
          <pre>{d.briefing.content}</pre>
        </dialog>
      {/if}

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
  {/if}
</div>

<style>
  .brief-button { border: 1px solid var(--line); border-radius: 12px; padding: 1px 8px; background: var(--panel); color: var(--accent); font: inherit; cursor: pointer; }
  .brief-button:hover { border-color: var(--accent); }
  .brief-dialog { width: min(820px, calc(100vw - 32px)); max-height: min(88vh, 880px); padding: 0; border: 1px solid var(--line); border-radius: 12px; background: var(--panel); color: var(--text); box-shadow: 0 24px 80px #0009; }
  .brief-dialog::backdrop { background: #0009; backdrop-filter: blur(3px); }
  .brief-dialog > header { position: sticky; top: 0; display: flex; align-items: center; justify-content: space-between; padding: 16px 20px; border-bottom: 1px solid var(--line); background: var(--panel); }
  .brief-dialog .eyebrow { margin: 0 0 4px; color: var(--muted); font-size: 11px; }
  .brief-dialog h2 { margin: 0; font-size: 20px; }
  .brief-dialog .close { width: 32px; height: 32px; border: 1px solid var(--line); border-radius: 8px; background: transparent; color: var(--text); font-size: 22px; }
  .brief-summary { padding: 0 20px; color: var(--muted); font-size: 12px; }
  .brief-dialog pre { margin: 0 20px 20px; max-height: 65vh; overflow: auto; padding: 14px; border: 1px solid var(--line); border-radius: 8px; background: var(--bg); color: var(--text); white-space: pre-wrap; overflow-wrap: anywhere; font: 12px/1.55 ui-monospace, SFMono-Regular, Menlo, monospace; }
</style>
