<script lang="ts">
  import type { SessionInfo } from './api'

  let {
    sessions,
    activeId,
    onselect,
    onclose,
  }: {
    sessions: SessionInfo[]
    activeId: string | null
    onselect: (id: string) => void
    onclose: (id: string) => void
  } = $props()
</script>

<div class="tabs" role="tablist" aria-label="Sessions">
  {#each sessions as s (s.id)}
    <div class="tab" class:active={s.id === activeId} class:exited={s.exited}>
      <button
        type="button"
        role="tab"
        aria-selected={s.id === activeId}
        title={s.dir}
        onclick={() => onselect(s.id)}
      >
        <span class="name">{s.title}</span>
        {#if s.exited}<span class="badge">exited {s.exitCode}</span>{/if}
      </button>
      <button type="button" class="close" aria-label={`Close ${s.title}`} onclick={() => onclose(s.id)}>×</button>
    </div>
  {/each}
</div>
