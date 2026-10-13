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

  onMount(() => {
    store.refresh()
    const onEscape = (event: KeyboardEvent) => {
      if (event.key !== 'Escape' || event.defaultPrevented || event.isComposing) return
      if (!escapeRestores(store.workspace, event.target, settingsOpen || wordsOpen)) return
      event.preventDefault()
      const region = store.workspace.expanded
      store.restoreSplit()
      void tick().then(() => document.querySelector<HTMLElement>(`.expand-button[data-region="${region}"]`)?.focus())
    }
    window.addEventListener('keydown', onEscape)
    const onShortcut = (event: KeyboardEvent) => {
      if (event.ctrlKey && !event.metaKey && event.key === ',') {
        event.preventDefault()
        settingsOpen = true
      }
    }
    window.addEventListener('keydown', onShortcut)
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
      window.removeEventListener('keydown', onShortcut)
      window.removeEventListener('keydown', onEscape)
    }
  })

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
  style={`--sidebar-size:${store.appearance.sidebarSize}px;--sidebar-font-scale:${store.appearance.sidebarSize / 14};--terminal-size:${store.appearance.terminalSize}px;--quests-size:${store.appearance.questsSize}px;--quests-font-scale:${store.appearance.questsSize / 14}`}
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
  {#if settingsOpen}<SettingsDialog {store} onclose={() => (settingsOpen = false)} />{/if}
  {#if wordsOpen}<WordsDialog onclose={() => (wordsOpen = false)} />{/if}
</div>
{/if}
