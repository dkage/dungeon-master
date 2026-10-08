<script lang="ts">
  // Stands in for the xterm-backed Terminal in jsdom tests.
  let { sessionId, onexit }: { sessionId: string; onexit: (code: number) => void } = $props()
</script>

<div data-testid="term-stub">{sessionId}</div>
<button type="button" onclick={() => onexit(5)}>fake-exit-{sessionId}</button>
