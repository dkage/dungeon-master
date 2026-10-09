<script lang="ts">
  import type { AppStore } from './store.svelte'
  import type { QuestRef } from './api'
  import { STATE_LABEL, labelText, timeAgo } from './quests'
  import { renderMarkdown } from './markdown'
  import { wordLabel } from './words'

  let { store }: { store: AppStore } = $props()

  const d = $derived(store.detail)
  const body = $derived(d ? renderMarkdown(d.body) : '')
  const safeUrl = $derived(d && /^https:\/\//i.test(d.url) && !store.campaign?.demo ? d.url : '')
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
  {/if}
</div>
