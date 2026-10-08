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

  // Nerd Font names come first so prompt icons (Starship, Powerline) render
  // when one is installed; the rest is the usual monospace fallback.
  const FONT_STACK = [
    '"JetBrainsMono Nerd Font"', '"JetBrainsMono NF"', '"MesloLGS NF"', '"MesloLGS Nerd Font"',
    '"FiraCode Nerd Font"', '"Hack Nerd Font"', '"Symbols Nerd Font Mono"',
    'ui-monospace', 'SFMono-Regular', 'Menlo', 'Consolas', 'monospace',
  ].join(', ')

  // The terminal is always dark, in both light and dark page themes.
  const TERM_THEME = {
    background: '#14141a',
    foreground: '#e8e8e4',
    cursor: '#e8e8e4',
    cursorAccent: '#14141a',
    selectionBackground: 'rgba(143, 120, 255, 0.45)',
    selectionInactiveBackground: 'rgba(143, 120, 255, 0.25)',
  }

  let host: HTMLDivElement
  let status = $state<'connecting' | 'live' | 'exited' | 'disconnected'>('connecting')
  let detail = $state('')

  onMount(() => {
    const term = new Terminal({
      cursorBlink: true,
      fontFamily: FONT_STACK,
      fontSize: 13,
      scrollback: 5000,
      // Agents such as Claude turn on mouse reporting, which stops plain
      // drags from selecting. Shift-drag always selects; on macOS so does
      // Option-drag.
      macOptionClickForcesSelection: true,
      theme: TERM_THEME,
    })
    const fit = new FitAddon()
    term.loadAddon(fit)
    term.open(host)

    // Copy the selection with Cmd+C (macOS) or Ctrl+Shift+C. Plain Ctrl+C is
    // left alone: it must reach the process as an interrupt.
    term.attachCustomKeyEventHandler((ev) => {
      const copy = ev.type === 'keydown' && ev.code === 'KeyC' && (ev.metaKey || (ev.ctrlKey && ev.shiftKey))
      if (copy && term.hasSelection()) {
        void navigator.clipboard?.writeText(term.getSelection())
        return false
      }
      return true
    })

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
