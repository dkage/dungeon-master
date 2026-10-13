<script lang="ts">
  import { onMount } from 'svelte'
  import { Terminal } from '@xterm/xterm'
  import { FitAddon } from '@xterm/addon-fit'
  import '@xterm/xterm/css/xterm.css'
  import './terminal.css'
  import { termUrl } from './api'
  import { TermSocket } from './termsocket'
  import { TERM_THEMES, dimStyles } from './termTheme'

  let {
    sessionId,
    fontSize = 13,
    theme = 'dungeon',
    onexit,
  }: { sessionId: string; fontSize?: number; theme?: 'dungeon' | 'neutral'; onexit: (code: number) => void } = $props()

  // Nerd Font names come first so prompt icons (Starship, Powerline) render
  // when one is installed; the rest is the usual monospace fallback.
  const FONT_STACK = [
    '"JetBrainsMono Nerd Font"', '"JetBrainsMono NF"', '"MesloLGS NF"', '"MesloLGS Nerd Font"',
    '"FiraCode Nerd Font"', '"Hack Nerd Font"', '"Symbols Nerd Font Mono"',
    'ui-monospace', 'SFMono-Regular', 'Menlo', 'Consolas', 'monospace',
  ].join(', ')

  let host: HTMLDivElement
  let terminal = $state.raw<Terminal>()
  let status = $state<'connecting' | 'live' | 'exited' | 'disconnected'>('connecting')
  let detail = $state('')

  $effect(() => {
    if (terminal) terminal.options.fontSize = fontSize
  })
  $effect(() => {
    if (terminal) terminal.options.theme = TERM_THEMES[theme]
  })

  onMount(() => {
    // One shared sheet of readable dim colours for every terminal.
    if (!document.getElementById('term-dim-colours')) {
      const style = document.createElement('style')
      style.id = 'term-dim-colours'
      style.textContent = dimStyles()
      document.head.append(style)
    }
    const term = new Terminal({
      cursorBlink: true,
      fontFamily: FONT_STACK,
      fontSize,
      scrollback: 5000,
      // The Dungeon appearance shows the parchment through the terminal.
      allowTransparency: true,
      // Lift any text colour the agent picks (its grays) that would read below
      // WCAG AA against the theme background, the parchment tone in Dungeon.
      // xterm halves the target for dim text, so dimStyles() covers dim.
      minimumContrastRatio: 4.5,
      // Agents such as Claude turn on mouse reporting, which stops plain
      // drags from selecting. Shift-drag always selects; on macOS so does
      // Option-drag.
      macOptionClickForcesSelection: true,
      theme: TERM_THEMES[theme],
    })
    terminal = term
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
  <div class="term" bind:this={host} data-testid="terminal" data-term-theme={theme}></div>
  {#if status !== 'live'}
    <div class="term-status" data-status={status} role="status">
      {status === 'connecting' ? 'Connecting…' : status === 'exited' ? `Session ${detail}` : `Disconnected: ${detail}`}
    </div>
  {/if}
</div>
