<script lang="ts">
  import { onMount, tick } from 'svelte'
  import { AppStore } from './lib/store.svelte'
  import Sidebar from './lib/Sidebar.svelte'
  import Toolbar from './lib/Toolbar.svelte'
  import Terminal from './lib/Terminal.svelte'
  import QuestsPanel from './lib/QuestsPanel.svelte'
  import StartHere from './lib/StartHere.svelte'
  import SettingsDialog from './lib/SettingsDialog.svelte'
  import WordsDialog from './lib/WordsDialog.svelte'
  import { attentionTitle, faviconHref } from './lib/attention'
  import { route as routeShortcut } from './lib/shortcuts'
  import { PLACE_KEY, parse as parsePlace, type Place } from './lib/place'
  import { keyWidth, DEFAULTS, limits, type Edge } from './lib/columns'
  import { escapeRestores } from './lib/workspace'
  import { notificationText, permission } from './lib/notify'

  let { store = new AppStore() }: { store?: AppStore } = $props()
  let settingsOpen = $state(false)
  let wordsOpen = $state(false)

  // While the page is visible: keep the GitHub budget fresh, and re-read the
  // quests now and then as low-priority background work. The server answers
  // most of these from its cache, so they cost GitHub nothing.
  const BUDGET_EVERY = 30_000
  const QUESTS_EVERY = 60_000
  // Sessions are local and cheap to read, so their activity is read often.
  const SESSIONS_EVERY = 2_000
  const BASE_TITLE = 'Dungeon Master'

  // Is the user really looking at this window? A visible tab in a background window is not.
  let viewing = $state(document.visibilityState === 'visible' && document.hasFocus())

  // Where the user was survives a reload: the address-bar hash first, then the copy in storage.
  let restoring = $state(true)
  async function restore(p: Place) {
    restoring = true
    try {
      await store.applyPlace(p)
    } finally {
      restoring = false
    }
  }
  const stored = () => {
    try {
      return localStorage.getItem(PLACE_KEY)
    } catch {
      return null
    }
  }
  $effect(() => {
    if (restoring || store.unauthorised) return
    const hash = store.placeHash()
    const now = location.hash.replace(/^#/, '')
    if (hash === now) return
    const was = parsePlace(now)
    const is = store.place
    const moved = was.campaign !== is.campaign || was.quest !== is.quest || was.session !== is.session
    const url = location.pathname + location.search + (hash ? `#${hash}` : '')
    try {
      if (moved && now) history.pushState(null, '', url)
      else history.replaceState(null, '', url)
    } catch {
      /* the address bar could not be updated; storage still has it */
    }
  })

  onMount(() => {
    void store.refresh().then(() => restore(location.hash.length > 1 ? parsePlace(location.hash) : parsePlace(stored())))
    const onPop = () => void restore(parsePlace(location.hash))
    window.addEventListener('popstate', onPop)
    const onEscape = (event: KeyboardEvent) => {
      if (event.key !== 'Escape' || event.defaultPrevented || event.isComposing) return
      if (!escapeRestores(store.workspace, event.target, settingsOpen || wordsOpen)) return
      event.preventDefault()
      const region = store.workspace.expanded
      store.restoreSplit()
      void tick().then(() => document.querySelector<HTMLElement>(`.expand-button[data-region="${region}"]`)?.focus())
    }
    window.addEventListener('keydown', onEscape)
    const onWindow = () => (store.windowWidth = window.innerWidth)
    window.addEventListener('resize', onWindow)
    // Capture phase, so a binding is handled before the terminal or a field sees the key.
    const onShortcut = (event: KeyboardEvent) => {
      const action = routeShortcut(event, store.shortcuts, store.platform, settingsOpen || wordsOpen)
      if (!action) return
      event.preventDefault()
      event.stopPropagation()
      store.runAction(action, () => (settingsOpen = true))
    }
    window.addEventListener('keydown', onShortcut, true)
    const visible = () => document.visibilityState === 'visible'
    const look = () => (viewing = document.visibilityState === 'visible' && document.hasFocus())
    window.addEventListener('focus', look)
    window.addEventListener('blur', look)
    document.addEventListener('visibilitychange', look)
    const sessions = setInterval(() => {
      if (!store.unauthorised && store.sessions.length > 0) void store.refreshSessions()
    }, SESSIONS_EVERY)
    const budget = setInterval(() => visible() && store.loadGithub(), BUDGET_EVERY)
    const quests = setInterval(() => {
      if (visible() && store.campaignId && !store.unauthorised) void store.loadQuests(false, true)
    }, QUESTS_EVERY)
    const onFocus = () => {
      if (visible() && store.campaignId) {
        void store.loadQuests(false, true)
        void store.loadGithub()
      }
    }
    document.addEventListener('visibilitychange', onFocus)
    return () => {
      clearInterval(budget)
      clearInterval(sessions)
      window.removeEventListener('focus', look)
      window.removeEventListener('blur', look)
      document.removeEventListener('visibilitychange', look)
      clearInterval(quests)
      document.removeEventListener('visibilitychange', onFocus)
      window.removeEventListener('keydown', onShortcut, true)
      window.removeEventListener('keydown', onEscape)
      window.removeEventListener('resize', onWindow)
      window.removeEventListener('popstate', onPop)
    }
  })

  // Dragging a column border: the pointer position is the new width.
  let dragging = $state<Edge | null>(null)
  function dragMove(edge: Edge, event: PointerEvent) {
    if (dragging !== edge) return
    store.resizeColumn(edge, edge === 'sidebar' ? event.clientX : window.innerWidth - event.clientX)
  }
  function borderKey(edge: Edge, event: KeyboardEvent) {
    const next = keyWidth(store.columnWidths, edge, event.key, store.windowWidth)
    if (!next) return
    event.preventDefault()
    store.setColumns(next)
  }
  const bounds = (edge: Edge) =>
    limits(edge, edge === 'sidebar' ? store.columnWidths.quests : store.columnWidths.sidebar, store.windowWidth)

  // Sessions of every campaign stay mounted (hidden) so switching is instant
  // and their terminals keep their state; lost sessions have no terminal.
  const live = $derived(store.sessions.filter((s) => !s.lost))

  // Looking at a session that waits counts as having seen it.
  $effect(() => {
    store.attend(viewing)
  })

  // A desktop notification for a session that starts waiting while this window
  // is not in front: once per wait, only if the user turned it on.
  $effect(() => {
    const fire = store.takeNotifications(viewing)
    if (fire.length === 0 || permission() !== 'granted') return
    for (const s of fire) {
      const text = notificationText(s, store.campaigns.find((c) => c.id === s.campaign)?.name ?? 'a project')
      try {
        const n = new Notification(text.title, { body: text.body, tag: s.id })
        n.onclick = () => {
          window.focus()
          void store.selectSession(s.id)
          n.close()
        }
      } catch {
        /* the browser refused; the title and icon still show it */
      }
    }
  })

  // The tab title and icon say how many sessions are waiting, so it shows from another tab.
  const waiting = $derived(store.attention.length)
  $effect(() => {
    document.title = attentionTitle(BASE_TITLE, waiting)
    let link = document.querySelector<HTMLLinkElement>('link[rel="icon"]')
    if (!link) {
      link = document.createElement('link')
      link.rel = 'icon'
      document.head.appendChild(link)
    }
    link.type = 'image/svg+xml'
    link.href = faviconHref(waiting)
  })
</script>

{#if store.unauthorised}
  <main class="signin" aria-labelledby="signin-title">
    <h1 id="signin-title">This tab is not signed in</h1>
    <p>
      Dungeon Master did not accept this browser's access token. Your campaigns and sessions are safe: they are stored on
      this computer.
    </p>
    <p>
      Open the link printed in the terminal where you ran <code>make run</code> or <code>dungeon serve</code>. It ends in
      <code>?token=…</code> and signs this browser in. The token is kept between restarts, so you only need to do this once.
    </p>
    <button type="button" onclick={() => store.refresh()}>Try again</button>
  </main>
{:else}
<div
  class="app"
  data-expanded={store.workspace.expanded ?? undefined}
  data-theme={store.appearance.theme}
  data-font={store.appearance.font}
  style={`--col-sidebar:${store.columnWidths.sidebar}px;--col-quests:${store.columnWidths.quests}px;--sidebar-size:${store.appearance.sidebarSize}px;--sidebar-font-scale:${store.appearance.sidebarSize / 14};--terminal-size:${store.appearance.terminalSize}px;--quests-size:${store.appearance.questsSize}px;--quests-font-scale:${store.appearance.questsSize / 14}`}
>
  <Sidebar {store} onsettings={() => (settingsOpen = true)} onwords={() => (wordsOpen = true)} />
  <div class="main">
    <Toolbar {store} />
    <main class="stage">
      {#each live as s (s.id)}
        <div class="pane" hidden={s.id !== store.activeId || store.guideOpen}>
          <Terminal sessionId={s.id} fontSize={store.appearance.terminalSize} theme={store.appearance.theme} onexit={(code) => store.markExited(s.id, code)} />
        </div>
      {/each}
      {#if store.guideShown}
        <StartHere {store} openSettings={() => (settingsOpen = true)} />
      {:else if store.active?.lost}
        <p class="empty">This expedition was running when Dungeon Master last stopped, so its terminal is gone. Close it to dismiss.</p>
      {:else if !store.active}
        <p class="empty">
          {store.campaign ? 'Pick a hero above to start an expedition.' : 'Add a project on the left to begin.'}
        </p>
      {/if}
    </main>
  </div>
  <QuestsPanel {store} />
  {#each ['sidebar', 'quests'] as const as edge (edge)}
    <!-- svelte-ignore a11y_no_noninteractive_tabindex, a11y_no_noninteractive_element_interactions -->
    <div
      class="col-border"
      class:dragging={dragging === edge}
      data-edge={edge}
      role="separator"
      aria-orientation="vertical"
      aria-label={edge === 'sidebar' ? 'Resize projects panel' : 'Resize issues panel'}
      aria-valuenow={store.columnWidths[edge]}
      aria-valuemin={bounds(edge).min}
      aria-valuemax={bounds(edge).max}
      tabindex="0"
      style={edge === 'sidebar' ? `left:${store.columnWidths.sidebar - 3}px` : `right:${store.columnWidths.quests - 3}px`}
      onpointerdown={(e) => { dragging = edge; e.currentTarget.setPointerCapture(e.pointerId) }}
      onpointermove={(e) => dragMove(edge, e)}
      onpointerup={() => (dragging = null)}
      onpointercancel={() => (dragging = null)}
      ondblclick={() => store.resizeColumn(edge, DEFAULTS[edge])}
      onkeydown={(e) => borderKey(edge, e)}
    ></div>
  {/each}
  {#if settingsOpen}<SettingsDialog {store} onclose={() => (settingsOpen = false)} />{/if}
  {#if wordsOpen}<WordsDialog onclose={() => (wordsOpen = false)} />{/if}
</div>
{/if}
