<script lang="ts">
  import { onMount } from 'svelte'
  import { Terminal } from '@xterm/xterm'
  import { FitAddon } from '@xterm/addon-fit'
  import '@xterm/xterm/css/xterm.css'
  import './terminal.css'
  import { termUrl, uploadToSession } from './api'
  import { TermSocket } from './termsocket'
  import { TERM_THEMES, dimStyles } from './termTheme'
  import { terminalTypography } from './termOptions'
  import { keyInput } from './termKeys'
  import { fileToBase64, filesOf, pathsText, refusal } from './termUpload'

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
  // A short message about a refused paste or drop, and whether a drag is over the terminal.
  let note = $state('')
  let dropping = $state(false)
  let noteTimer: ReturnType<typeof setTimeout> | undefined

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
      // Shift+Enter inserts a line break in the agent's prompt without submitting.
      const input = keyInput(ev)
      if (input !== null) {
        ev.preventDefault()
        sock.sendInput(input)
        return false
      }
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

    // Files pasted or dropped become a path in the prompt, inserted at the
    // cursor through xterm's paste so the agent sees one paste and nothing
    // is submitted. Plain text pastes are left to xterm.
    function tell(msg: string) {
      note = msg
      clearTimeout(noteTimer)
      noteTimer = setTimeout(() => (note = ''), 6000)
    }
    async function sendFiles(files: File[]) {
      if (status !== 'live') return tell('The session is not running, so the file was not added.')
      const paths: string[] = []
      for (const file of files) {
        const why = refusal(file)
        if (why) {
          tell(why)
          continue
        }
        try {
          paths.push((await uploadToSession(sessionId, file.name, await fileToBase64(file))).path)
        } catch (e) {
          tell(`${file.name || 'The file'} was not added: ${e instanceof Error ? e.message : 'upload failed'}`)
        }
      }
      if (paths.length) {
        note = ''
        term.paste(pathsText(paths))
        term.focus()
      }
    }
    const onPaste = (ev: ClipboardEvent) => {
      const files = filesOf(ev.clipboardData)
      if (!files.length) return
      ev.preventDefault()
      ev.stopPropagation()
      void sendFiles(files)
    }
    const hasFiles = (ev: DragEvent) => Array.from(ev.dataTransfer?.types ?? []).includes('Files')
    const onDragOver = (ev: DragEvent) => {
      if (!hasFiles(ev)) return
      ev.preventDefault()
      dropping = true
    }
    const onDragLeave = () => (dropping = false)
    const onDrop = (ev: DragEvent) => {
      dropping = false
      const files = filesOf(ev.dataTransfer)
      if (!files.length) return
      ev.preventDefault()
      void sendFiles(files)
    }
    host.addEventListener('paste', onPaste, true)
    host.addEventListener('dragover', onDragOver)
    host.addEventListener('dragleave', onDragLeave)
    host.addEventListener('drop', onDrop)

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
      clearTimeout(noteTimer)
      host.removeEventListener('paste', onPaste, true)
      host.removeEventListener('dragover', onDragOver)
      host.removeEventListener('dragleave', onDragLeave)
      host.removeEventListener('drop', onDrop)
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
  <div class="term" data-testid="terminal" data-term-theme={theme} data-dropping={dropping}>
    <div class="term-host" bind:this={host}></div>
  </div>
  {#if note}
    <div class="term-note" role="status">{note}</div>
  {/if}
  {#if status !== 'live'}
    <div class="term-status" data-status={status} role="status">
      {status === 'connecting' ? 'Connecting…' : status === 'exited' ? `Session ${detail}` : `Disconnected: ${detail}`}
    </div>
  {/if}
</div>
