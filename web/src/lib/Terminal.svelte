<script lang="ts">
  import { onMount } from 'svelte'
  import { Terminal } from '@xterm/xterm'
  import { FitAddon } from '@xterm/addon-fit'
  import '@xterm/xterm/css/xterm.css'
  import './terminal.css'
  import { termUrl } from './api'
  import { TermSocket } from './termsocket'
  import { TERM_THEMES, dimStyles } from './termTheme'
  import { terminalTypography } from './termOptions'

  let {
    sessionId,
    fontSize = 13,
    theme = 'dungeon',
    onexit,
  }: { sessionId: string; fontSize?: number; theme?: 'dungeon' | 'neutral'; onexit: (code: number) => void } = $props()

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
      ...terminalTypography(),
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
    // xterm measures cells with whatever font is ready; remeasure once the
    // bundled Fira Code and symbols font have loaded, so columns do not shift.
    void Promise.all([
      document.fonts?.load('13px "Fira Code"'),
      document.fonts?.load('13px "Dungeon Symbols"', '\ue0b0\uf015'),
    ]).then(() => {
      term.options.fontFamily = terminalTypography().fontFamily + ' '
      term.options.fontFamily = terminalTypography().fontFamily
      // The cell width changed but the frame did not, so the resize observer
      // stays quiet: refit, or the last columns run under the frame.
      try {
        fit.fit()
      } catch {
        /* hidden or zero-sized: fit on the next resize */
      }
    }).catch(() => {})
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
    // One fit per frame, so dragging a column border does not flood the session with resizes.
    let frame = 0
    const ro = new ResizeObserver(() => {
      if (frame) return
      frame = requestAnimationFrame(() => {
        frame = 0
        try {
          fit.fit()
        } catch {
          /* hidden or zero-sized: fit on the next resize */
        }
      })
    })
    ro.observe(host)

    return () => {
      ro.disconnect()
      if (frame) cancelAnimationFrame(frame)
      data.dispose()
      resized.dispose()
      sock.close()
      term.dispose()
    }
  })
</script>

<div class="term-wrap">
  <!-- The addon measures its parent's width and ignores that parent's padding, so xterm
       lives in an unpadded host inside the padded frame. -->
  <div class="term" data-testid="terminal" data-term-theme={theme}>
    <div class="term-host" bind:this={host}></div>
  </div>
  {#if status !== 'live'}
    <div class="term-status" data-status={status} role="status">
      {status === 'connecting' ? 'Connecting…' : status === 'exited' ? `Session ${detail}` : `Disconnected: ${detail}`}
    </div>
  {/if}
</div>
