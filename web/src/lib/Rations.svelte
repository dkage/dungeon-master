<script lang="ts">
  import type { GithubStatus } from './api'

  let { status }: { status: GithubStatus | null } = $props()

  const graphql = $derived(status?.rations.find((r) => r.resource === 'graphql'))
  const low = $derived(graphql ? graphql.remaining < graphql.limit * 0.2 : false)
  const fmt = (n: number) => n.toLocaleString()
  const at = (iso: string) => new Date(iso).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
</script>

{#if graphql}
  <span
    class="rations"
    class:low
    title="GitHub allows a fixed number of requests per hour. This count is shared with every other tool that uses your GitHub login."
  >
    GitHub requests: {fmt(graphql.remaining)} of {fmt(graphql.limit)} left, resets {at(graphql.resetsAt)}
  </span>
{/if}
