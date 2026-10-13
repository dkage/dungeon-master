<script lang="ts">
  import { onMount, tick } from 'svelte'
  import { Terminal } from '@xterm/xterm'
  import { FitAddon } from '@xterm/addon-fit'
  import { SearchAddon } from '@xterm/addon-search'
  import '@xterm/xterm/css/xterm.css'
  import './terminal.css'
  import { termUrl, uploadToSession } from './api'
  import { TermSocket } from './termsocket'
  import { connection } from './connection.svelte'
  import { TERM_THEMES, dimStyles, type TermThemeName } from './termTheme'
  import { terminalTypography } from './termOptions'
  import { browserPastes, keyInput } from './termKeys'
  import { DEFAULT_TERMINAL_SIZE, MAX_SIZE, MIN_SIZE, type Zoom } from './termSize'
  import { bufferText, outputFileName, wasCut, type BufferLine } from './termText'
  import { SEARCH_DECORATIONS, TERMINAL_ACTION, matchLabel, scrolledUp } from './termSearch'
  import { fileToBase64, filesOf, pathsText, refusal } from './termUpload'

  let {
    sessionId,
    fontSize = 13,
    fontFamily = terminalTypography().fontFamily,
    lineHeight = terminalTypography().lineHeight,
    theme = 'dungeon',
    onexit,
    onzoom,
  }: {
    sessionId: string
    fontSize?: number
    fontFamily?: string
    lineHeight?: number
    theme?: TermThemeName
    onexit: (code: number) => void
    onzoom?: (step: Zoom) => void
  } = $props()

  const SCROLLBACK = 5000
  let host: HTMLDivElement
  let terminal = $state.raw<Terminal>()
  let status = $state<'connecting' | 'live' | 'reconnecting' | 'exited' | 'disconnected'>('connecting')
  let detail = $state('')
  // A short message about a refused paste or drop, and whether a drag is over the terminal.
  let note = $state('')
  let dropping = $state(false)
  // Scrollback search (#91): the bar, the query and the addon's own count.
  let searching = $state(false)
  let query = $state('')
  let hit = $state({ index: -1, count: 0 })
  let latest = $state(false)
  let searchInput: HTMLInputElement | undefined = $state()
  let search: SearchAddon | undefined
  let noteKind = $state<'info' | 'error'>('error')
  let noteTimer: ReturnType<typeof setTimeout> | undefined

  let refit: (() => void) | undefined
  $effect(() => {
    if (!terminal) return
    terminal.options.fontSize = fontSize
    terminal.options.fontFamily = fontFamily
    terminal.options.lineHeight = lineHeight
    // The cells changed size but the frame did not, so refit: the new
    // columns and rows reach the session and the scrollback stays.
    requestAnimationFrame(() => refit?.())
  })
  $effect(() => {
    if (terminal) terminal.options.theme = TERM_THEMES[theme]
  })

  const options = { decorations: SEARCH_DECORATIONS }
  function find(forward: boolean, incremental = false) {
    if (!search) return
    if (!query) {
      search.clearDecorations()
      hit = { index: -1, count: 0 }
      return
    }
    if (forward) search.findNext(query, { ...options, incremental })
    else search.findPrevious(query, options)
  }
  function openSearch() {
    searching = true
    void tick().then(() => {
      searchInput?.focus()
      searchInput?.select()
    })
  }
  function closeSearch() {
    searching = false
    search?.clearDecorations()
    hit = { index: -1, count: 0 }
    terminal?.focus()
  }
  // The whole scrollback as plain text: what the terminal holds, never what it was sent.
  function outputText(): string {
    const buf = terminal!.buffer.active
    const lines: BufferLine[] = []
    for (let y = 0; y < buf.length; y++) {
      const l = buf.getLine(y)
      if (l) lines.push({ text: l.translateToString(true), wrapped: l.isWrapped })
    }
    return bufferText(lines, wasCut(buf.length, terminal!.rows, SCROLLBACK))
  }
  function inform(msg: string, kind: 'info' | 'error' = 'info') {
    noteKind = kind
    note = msg
    clearTimeout(noteTimer)
    noteTimer = setTimeout(() => (note = ''), 4000)
  }
  async function copyOutput() {
    if (!terminal) return
    const text = outputText()
    try {
      await navigator.clipboard.writeText(text)
      inform(text ? 'Output copied.' : 'There is no output to copy.')
    } catch {
      inform('The browser did not allow copying. Use Save output instead.', 'error')
    }
  }
  function saveOutput() {
    if (!terminal) return
    const url = URL.createObjectURL(new Blob([outputText() + '\n'], { type: 'text/plain;charset=utf-8' }))
    const a = document.createElement('a')
    a.href = url
    a.download = outputFileName(sessionId)
    a.click()
    setTimeout(() => URL.revokeObjectURL(url), 1000)
    inform('Output saved.')
  }
  function toLatest() {
    terminal?.scrollToBottom()
    latest = false
  }
  function searchKey(ev: KeyboardEvent) {
    if (ev.key === 'Escape') {
      ev.preventDefault()
      ev.stopPropagation()
      closeSearch()
    } else if (ev.key === 'Enter') {
      ev.preventDefault()
      find(!ev.shiftKey)
    }
  }

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
      fontFamily,
      lineHeight,
      fontSize,
      scrollback: SCROLLBACK,
      // The Dungeon appearance shows the parchment through the terminal.
      allowTransparency: true,
      // Search highlights are decorations, which xterm still gates behind this flag.
      allowProposedApi: true,
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
      term.options.fontFamily = fontFamily + ' '
      term.options.fontFamily = fontFamily
      // The cell width changed but the frame did not, so the resize observer
      // stays quiet: refit, or the last columns run under the frame.
      try {
        fit.fit()
      } catch {
        /* hidden or zero-sized: fit on the next resize */
      }
    }).catch(() => {})
    const fit = new FitAddon()
    refit = () => {
      try {
        fit.fit()
      } catch {
        /* hidden or zero-sized: fit on the next resize */
      }
    }
    term.loadAddon(fit)
    search = new SearchAddon()
    term.loadAddon(search)
    const found = search.onDidChangeResults((r) => (hit = { index: r.resultIndex, count: r.resultCount }))
    term.open(host)

    // Copy the selection with Cmd+C (macOS) or Ctrl+Shift+C. Plain Ctrl+C is
    // left alone: it must reach the process as an interrupt.
    const mac = /Mac|iPhone|iPad/i.test(navigator.platform ?? '')
    term.attachCustomKeyEventHandler((ev) => {
      // Ctrl+V pastes (Windows, Linux): leave the key to the browser so its paste event fires.
      if (browserPastes(ev, mac)) return false
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
        connection.reportTerminal(sessionId, true)
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
      onReset: () => term.reset(),
      onReconnecting(attempt, kind) {
        status = 'reconnecting'
        detail = attempt > 1 ? ` (attempt ${attempt})` : ''
        connection.reportTerminal(sessionId, kind !== 'unreachable')
      },
      onDisconnect(reason) {
        status = 'disconnected'
        detail = reason
        connection.reportTerminal(sessionId, true) // refused is not unreachable: the app answered
      },
    })

    // Files pasted or dropped become a path in the prompt, inserted at the
    // cursor through xterm's paste so the agent sees one paste and nothing
    // is submitted. Plain text pastes are left to xterm.
    function tell(msg: string) {
      noteKind = 'error'
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

    const sync = () => (latest = scrolledUp(term.buffer.active))
    const scrolled = term.onScroll(sync)
    const parsed = term.onWriteParsed(sync)
    const onAction = (ev: Event) => {
      // Only the terminal the user can see answers.
      if (!host.offsetParent) return
      const id = (ev as CustomEvent<string>).detail
      if (id === 'search-terminal') openSearch()
      else if (id === 'terminal-latest') toLatest()
    }
    window.addEventListener(TERMINAL_ACTION, onAction)

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
      refit = undefined
      window.removeEventListener(TERMINAL_ACTION, onAction)
      scrolled.dispose()
      parsed.dispose()
      found.dispose()
      clearTimeout(noteTimer)
      host.removeEventListener('paste', onPaste, true)
      host.removeEventListener('dragover', onDragOver)
      host.removeEventListener('dragleave', onDragLeave)
      host.removeEventListener('drop', onDrop)
      if (frame) cancelAnimationFrame(frame)
      data.dispose()
      resized.dispose()
      sock.close()
      connection.reportTerminal(sessionId, true)
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
  <div class="term-tools">
    {#if onzoom}
      <button type="button" class="term-tool" onclick={() => onzoom('out')} disabled={fontSize <= MIN_SIZE} aria-label="Smaller text">A−</button>
      <button type="button" class="term-tool" onclick={() => onzoom('in')} disabled={fontSize >= MAX_SIZE} aria-label="Larger text">A+</button>
      {#if fontSize !== DEFAULT_TERMINAL_SIZE}
        <button type="button" class="term-tool" onclick={() => onzoom('reset')} aria-label={`Reset text size (${fontSize}px)`}>{fontSize}px ↺</button>
      {/if}
    {/if}
    {#if latest}
      <button type="button" class="term-tool" onclick={toLatest} aria-label="Jump to latest output">Latest ↓</button>
    {/if}
    <button type="button" class="term-tool" onclick={copyOutput} aria-label="Copy output as text">Copy</button>
    <button type="button" class="term-tool" onclick={saveOutput} aria-label="Save output as a text file">Save</button>
    <button type="button" class="term-tool" onclick={() => (searching ? closeSearch() : openSearch())} aria-label="Search output" aria-pressed={searching}>Search</button>
  </div>
  {#if searching}
    <div class="term-search" role="search">
      <input
        bind:this={searchInput}
        bind:value={query}
        type="search"
        placeholder="Search output"
        aria-label="Search terminal output"
        oninput={() => find(true, true)}
        onkeydown={searchKey}
      />
      <span class="term-count" role="status" aria-live="polite">{matchLabel(query, hit.index, hit.count)}</span>
      <button type="button" class="term-tool" onclick={() => find(false)} aria-label="Previous match" disabled={hit.count === 0}>↑</button>
      <button type="button" class="term-tool" onclick={() => find(true)} aria-label="Next match" disabled={hit.count === 0}>↓</button>
      <button type="button" class="term-tool" onclick={closeSearch} aria-label="Close search">✕</button>
    </div>
  {/if}
  {#if note}
    <div class="term-note" data-kind={noteKind} role="status">{note}</div>
  {/if}
  {#if status !== 'live'}
    <div class="term-status" data-status={status} role="status">
      {status === 'connecting' ? 'Connecting…' : status === 'reconnecting' ? `Reconnecting…${detail}` : status === 'exited' ? `Session ${detail}` : detail}
    </div>
  {/if}
</div>
