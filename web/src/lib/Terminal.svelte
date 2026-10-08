<script lang="ts">
  import { onMount } from 'svelte'
  import { Terminal } from '@xterm/xterm'
  import { FitAddon } from '@xterm/addon-fit'
  import '@xterm/xterm/css/xterm.css'
  import { termUrl } from './api'
  import { TermSocket } from './termsocket'

  let {
    sessionId,
    onexit,
  }: { sessionId: string; onexit: (code: number) => void } = $props()

  let host: HTMLDivElement
  let status = $state<'connecting' | 'live' | 'exited' | 'disconnected'>('connecting')
  let detail = $state('')

  onMount(() => {
    const term = new Terminal({
      cursorBlink: true,
      fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace',
      fontSize: 13,
      scrollback: 5000,
      allowProposedApi: false,
    })
    const fit = new FitAddon()
    term.loadAddon(fit)
    term.open(host)

    const sock = new TermSocket(termUrl(sessionId), {
      onOpen() {
        status = 'live'
        fit.fit()
        sock.resize(term.cols, term.rows)
        term.focus()
      },
      onOutput: (data) => term.write(data),
      onExit(code) {
        status = 'exited'
        detail = `exited with code ${code}`
        onexit(code)
      },
      onDisconnect(reason) {
        status = 'disconnected'
        detail = reason
      },
    })

    const data = term.onData((d) => sock.sendInput(d))
    const resized = term.onResize(({ cols, rows }) => sock.resize(cols, rows))
    const ro = new ResizeObserver(() => {
      try {
        fit.fit()
      } catch {
        /* hidden or zero-sized: fit on the next resize */
      }
    })
    ro.observe(host)

    return () => {
      ro.disconnect()
      data.dispose()
      resized.dispose()
      sock.close()
      term.dispose()
    }
  })
</script>

<div class="term-wrap">
  <div class="term" bind:this={host} data-testid="terminal"></div>
  {#if status !== 'live'}
    <div class="term-status" data-status={status} role="status">
      {status === 'connecting' ? 'Connecting…' : status === 'exited' ? `Session ${detail}` : `Disconnected: ${detail}`}
    </div>
  {/if}
</div>
