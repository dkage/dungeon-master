<script lang="ts">
  import { onMount } from 'svelte'
  import type { AppStore, ReadingFont, WordMode, ThemePack } from './store.svelte'
  import type { HeroConfig, HeroesConfig } from './api'
  import { wordLabel } from './words'
  import { permission as notifyPermission, requestPermission } from './notify'

  let { store, onclose }: { store: AppStore; onclose: () => void } = $props()
  let dialog: HTMLDialogElement
  let heroConfig = $state<HeroesConfig | null>(null)
  let heroError = $state('')
  let heroBusy = $state(false)
  let draft = $state<HeroConfig | null>(null)
  let draftNew = $state(false)
  let model = $state('')
  let permission = $state('')
  let extraArgs = $state('')
  let envRows = $state<{ key: string; value: string }[]>([])
  let deleteId = $state<string | null>(null)

  onMount(() => {
    if (typeof dialog.showModal === 'function') dialog.showModal()
    else dialog.setAttribute('open', '')
    void loadHeroes()
    const watcher = window.setInterval(() => {
      if (!draft) void loadHeroes()
    }, 1500)
    return () => {
      window.clearInterval(watcher)
      if (dialog.open && typeof dialog.close === 'function') dialog.close()
    }
  })

  function close() {
    if (typeof dialog.close === 'function') dialog.close()
    else {
      dialog.removeAttribute('open')
      onclose()
    }
  }

  function setFont(event: Event) {
    store.updateAppearance({ font: (event.currentTarget as HTMLSelectElement).value as ReadingFont })
  }
  function setWordMode(event: Event) {
    store.updateAppearance({ wordMode: (event.currentTarget as HTMLSelectElement).value as WordMode })
  }
  let notifyMessage = $state('')
  async function toggleNotifications(event: Event) {
    const box = event.currentTarget as HTMLInputElement
    notifyMessage = ''
    if (!box.checked) {
      store.updateAppearance({ notifications: false })
      return
    }
    // The browser is asked only now, because the user just turned this on.
    const answer = notifyPermission() === 'granted' ? 'granted' : await requestPermission()
    if (answer === 'granted') {
      store.updateAppearance({ notifications: true })
    } else {
      box.checked = false
      store.updateAppearance({ notifications: false })
      notifyMessage = answer === 'unsupported'
        ? 'This browser cannot show desktop notifications.'
        : 'Notifications are blocked for this site. Allow them in the browser settings, then try again.'
    }
  }
  function setTheme(event: Event) {
    store.updateAppearance({ theme: (event.currentTarget as HTMLSelectElement).value as ThemePack })
  }

  async function loadHeroes() {
    heroBusy = true
    try {
      heroConfig = await store.loadHeroesConfig()
    } catch (error) {
      heroError = error instanceof Error ? error.message : 'Could not load hero settings.'
    } finally {
      heroBusy = false
    }
  }

  function flagValue(args: string[], flag: string) {
    const index = args.indexOf(flag)
    return index >= 0 ? args[index + 1] ?? '' : ''
  }

  function commandArgs() {
    const args = [...(model.trim() ? ['--model', model.trim()] : [])]
    if (permission) args.push('--permission-mode', permission)
    args.push(...extraArgs.split('\n').map((line) => line.trimEnd()).filter((line) => line.length > 0))
    return args
  }

  function permissionLabel(args: string[]) {
    return ({ acceptEdits: 'Accept edits', plan: 'Plan', bypassPermissions: 'Skip all prompts' } as Record<string, string>)[flagValue(args, '--permission-mode')] ?? 'Default'
  }

  function extraOnly(args: string[]) {
    const out: string[] = []
    for (let i = 0; i < args.length; i++) {
      if (args[i] === '--model' || args[i] === '--permission-mode') { i++; continue }
      out.push(args[i])
    }
    return out.join('\n')
  }

  function beginNew() {
    draftNew = true
    draft = { id: '', label: '', args: [], env: [] }
    model = 'sonnet'
    permission = ''
    extraArgs = ''
    envRows = []
  }

  function beginEdit(hero: HeroConfig) {
    draftNew = false
    draft = { ...hero, args: [...hero.args], env: [...hero.env] }
    model = flagValue(hero.args, '--model')
    permission = flagValue(hero.args, '--permission-mode')
    extraArgs = extraOnly(hero.args)
    envRows = hero.env.map((entry) => {
      const [key, ...rest] = entry.split('=')
      return { key, value: rest.join('=') }
    })
  }

  async function saveHero() {
    if (!heroConfig || !draft) return
    const label = draft.label.trim()
    if (!label) { heroError = 'Enter a name for this hero.'; return }
    let id = draft.id
    if (draftNew) {
      id = label.toLowerCase().replace(/[^a-z0-9_-]+/g, '-').replace(/^-+|-+$/g, '').slice(0, 32) || 'hero'
      const base = id
      let suffix = 2
      while (heroConfig.heroes.some((hero) => hero.id === id)) id = `${base.slice(0, 29)}-${suffix++}`
    }
    const updated: HeroConfig = {
      id, label, args: commandArgs(),
      env: envRows.filter((row) => row.key.trim()).map((row) => `${row.key.trim()}=${row.value}`),
    }
    const heroes = draftNew
      ? [...heroConfig.heroes, updated]
      : heroConfig.heroes.map((hero) => hero.id === draft?.id ? updated : hero)
    const config = { defaultHero: draftNew && heroConfig.heroes.length === 0 ? id : heroConfig.defaultHero, heroes }
    heroError = ''
    if (await store.saveHeroesConfig(config)) {
      heroConfig = config
      draft = null
    } else {
      heroError = store.error ?? 'Could not save hero settings.'
    }
  }

  async function setDefault(id: string) {
    if (!heroConfig || heroConfig.defaultHero === id) return
    const config = { ...heroConfig, defaultHero: id }
    if (await store.saveHeroesConfig(config)) { heroConfig = config; deleteId = null }
    else heroError = store.error ?? 'Could not save the default hero.'
  }

  async function deleteHero(id: string) {
    if (!heroConfig || heroConfig.heroes.length < 2) return
    const heroes = heroConfig.heroes.filter((hero) => hero.id !== id)
    const config = { heroes, defaultHero: heroConfig.defaultHero === id ? heroes[0].id : heroConfig.defaultHero }
    if (await store.saveHeroesConfig(config)) heroConfig = config
    else heroError = store.error ?? 'Could not delete this hero.'
  }
</script>

<dialog bind:this={dialog} class="settings" aria-labelledby="settings-title" onclose={onclose}>
  <header>
    <div><p class="eyebrow">Dungeon Master</p><h1 id="settings-title">Settings</h1></div>
    <button type="button" class="close" aria-label="Close settings" onclick={close}>×</button>
  </header>

  <section aria-labelledby="appearance-title">
    <h2 id="appearance-title">Appearance & words</h2>
    <label class="field">
      <span>Theme</span>
      <select value={store.appearance.theme} onchange={setTheme}>
        <option value="dungeon">Dungeon</option>
        <option value="neutral">Neutral</option>
      </select>
    </label>
    <label class="field">
      <span>Reading font</span>
      <select value={store.appearance.font} onchange={setFont}>
        <option value="almendra">Almendra</option>
        <option value="eb-garamond">EB Garamond</option>
        <option value="alegreya">Alegreya</option>
      </select>
    </label>
    <label class="field">
      <span>Words</span>
      <select value={store.appearance.wordMode} onchange={setWordMode}>
        <option value="hints">Themed with plain hints</option>
        <option value="themed">Themed</option>
        <option value="plain">Plain</option>
      </select>
    </label>
    <div class="field">
      <label class="check"><input type="checkbox" checked={store.appearance.notifications} onchange={toggleNotifications} /> <span>Desktop notifications</span></label>
      <small class="hint">Tell me when a session is waiting for me and this window is not in front. Off unless you turn it on.</small>
      {#if notifyMessage}<p class="notice warn" role="alert">{notifyMessage}</p>{/if}
    </div>
    {#if store.campaign}
      <div class="field">
        <label class="check"><input type="checkbox" checked={store.campaign.parallelQuests === true} onchange={(e) => void store.setParallelQuests(e.currentTarget.checked)} /> <span>Allow several quests at once in {store.campaign.name}</span></label>
        <small class="hint">By default one quest runs per Git working tree, because agents would edit the same files. Turn this on to start more, for example when each works on separate files. Off unless you turn it on.</small>
      </div>
    {/if}
  </section>

  <section class="hero-section" aria-labelledby="heroes-title">
    <div class="section-heading"><div><h2 id="heroes-title">Heroes</h2><p>Claude setups used to start agents.</p></div><button type="button" onclick={beginNew}>New hero</button></div>
    {#if heroBusy}<p>Loading local hero settings…</p>
    {:else if heroConfig}
      <table>
      <thead><tr><th>Name</th><th>Model</th><th>Permission mode</th><th>Default</th><th>Actions</th></tr></thead>
        <tbody>
          {#each heroConfig.heroes as hero (hero.id)}
            <tr>
              <td>{hero.label}</td><td>{flagValue(hero.args, '--model') || 'CLI default'}</td><td>{permissionLabel(hero.args)}</td>
              <td><button type="button" aria-label={`Set ${hero.label} as default hero`} aria-pressed={heroConfig.defaultHero === hero.id} onclick={() => setDefault(hero.id)}>{heroConfig.defaultHero === hero.id ? '★' : '☆'}</button></td>
              <td><button type="button" onclick={() => beginEdit(hero)}>Edit</button> <button type="button" disabled={heroConfig.heroes.length < 2} onclick={() => (deleteId = hero.id)}>Delete</button></td>
            </tr>
            {#if deleteId === hero.id}
              <tr><td colspan="5"><div class="delete-confirm"><span>Delete {hero.label}? Your Claude install and project files will not be touched.</span><button type="button" onclick={() => (deleteId = null)}>Keep setup</button><button type="button" onclick={() => deleteHero(hero.id)}>Delete hero</button></div></td></tr>
            {/if}
          {/each}
        </tbody>
      </table>
      {#if heroConfig.heroes.length === 0}<p>No hero setups yet. Add one to start an agent.</p>{/if}
    {/if}
    {#if store.partyError}<p class="hero-error" role="alert">The saved hero file has an error. The last working settings are still active. Fix or save these settings to replace the file. {store.partyError}</p>{/if}
    {#if heroError}<p class="hero-error" role="alert">{heroError}</p>{/if}
    {#if draft}
      <div class="editor" role="group" aria-labelledby="hero-editor-title">
        <h3 id="hero-editor-title">{draftNew ? 'New hero' : `Edit ${draft.label}`}</h3>
        <label class="field"><span>Name</span><input bind:value={draft.label} /></label>
        <label class="field"><span>Model</span><input bind:value={model} placeholder="Claude model id" /></label>
        <label class="field"><span>Permission mode</span><select bind:value={permission}><option value="">Default</option><option value="acceptEdits">Accept edits</option><option value="plan">Plan</option><option value="bypassPermissions">Skip all prompts</option></select></label>
        {#if permission === 'bypassPermissions'}<p class="permission-warning" role="alert">This mode skips Claude's permission prompts and allows actions without asking.</p>{/if}
        <label class="field"><span>Extra arguments (one per line)</span><textarea bind:value={extraArgs} rows="3"></textarea></label>
        <fieldset><legend>Environment</legend>
          {#each envRows as row, i (i)}
            <div class="env-row"><input aria-label="Environment key" bind:value={row.key} placeholder="KEY" /><input type="password" aria-label="Environment value" bind:value={row.value} placeholder={row.value === '' && draft?.env.some((entry) => entry.startsWith(`${row.key}=`)) ? 'Saved on this computer; leave blank to keep it' : 'Value (masked)'} /><button type="button" aria-label="Remove environment variable" onclick={() => envRows = envRows.filter((_, index) => index !== i)}>×</button></div>
          {/each}
          <button type="button" onclick={() => envRows = [...envRows, { key: '', value: '' }]}>Add environment variable</button>
        </fieldset>
        <p class="preview">Command preview (argv): <code>claude {JSON.stringify(commandArgs())}</code></p>
        <div class="editor-actions"><button type="button" onclick={() => (draft = null)}>Cancel</button><button type="button" onclick={saveHero}>Save hero</button></div>
      </div>
    {/if}
  </section>

  <section aria-labelledby="sizes-title">
    <h2 id="sizes-title">Panel text size</h2>
    {#each [
      { key: 'sidebarSize', label: wordLabel('campaign', store.appearance.wordMode, true), size: store.appearance.sidebarSize },
      { key: 'terminalSize', label: 'Terminal', size: store.appearance.terminalSize },
      { key: 'questsSize', label: wordLabel('quest', store.appearance.wordMode, true), size: store.appearance.questsSize },
    ] as panel (panel.key)}
      <label class="range-field">
        <span>{panel.label}<output>{panel.size}px</output></span>
        <input
          type="range"
          min="10"
          max="24"
          step="1"
          value={panel.size}
          aria-label={`${panel.label} text size`}
          oninput={(event) => store.updateAppearance({ [panel.key]: Number((event.currentTarget as HTMLInputElement).value) })}
        />
      </label>
    {/each}
  </section>

  <footer><p>Changes apply immediately and stay on this computer.</p><button type="button" onclick={close}>Done</button></footer>
</dialog>

<style>
  .settings { width: min(620px, calc(100vw - 32px)); max-height: min(88vh, 800px); padding: 0; border: 1px solid var(--line); border-radius: 12px; background: var(--panel); color: var(--text); box-shadow: 0 24px 80px #0009; }
  .settings::backdrop { background: #0009; backdrop-filter: blur(3px); }
  header, footer { display: flex; align-items: center; justify-content: space-between; gap: 20px; padding: 18px 22px; }
  header { border-bottom: 1px solid var(--line); }
  h1, h2, p { margin: 0; }
  h1 { font-size: 24px; }
  h2 { margin-bottom: 14px; color: var(--muted); font-size: 12px; letter-spacing: .08em; text-transform: uppercase; }
  .eyebrow { color: var(--muted); font-size: 11px; letter-spacing: .12em; text-transform: uppercase; }
  section { display: grid; grid-template-columns: 1fr 1fr; gap: 12px 20px; padding: 20px 22px; border-bottom: 1px solid var(--line); }
  section h2 { grid-column: 1 / -1; }
  .field { display: grid; gap: 6px; color: var(--muted); font-size: 13px; }
  select { width: 100%; min-height: 36px; padding: 6px 9px; border: 1px solid var(--line); border-radius: 6px; background: var(--bg); color: var(--text); font: inherit; }
  .range-field { display: grid; gap: 8px; font-size: 13px; }
  .range-field span { display: flex; justify-content: space-between; gap: 8px; }
  output { color: var(--muted); font-variant-numeric: tabular-nums; }
  input[type='range'] { width: 100%; accent-color: var(--accent); }
  .hero-section { grid-template-columns: 1fr; }
  .section-heading, .editor-actions { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
  .section-heading p, .preview { color: var(--muted); font-size: 12px; }
  table { width: 100%; border-collapse: collapse; font-size: 13px; }
  th, td { padding: 7px 5px; border-bottom: 1px solid var(--line); text-align: left; }
  .editor { display: grid; gap: 12px; padding: 14px; border: 1px solid var(--line); border-radius: 8px; background: var(--bg); }
  .editor h3 { margin: 0; }
  .editor input, .editor textarea, .editor select { width: 100%; box-sizing: border-box; min-height: 34px; padding: 6px 8px; border: 1px solid var(--line); border-radius: 5px; background: var(--panel); color: var(--text); font: inherit; }
  .editor fieldset { display: grid; gap: 8px; border: 1px solid var(--line); border-radius: 6px; }
  .env-row { display: grid; grid-template-columns: 1fr 2fr auto; gap: 6px; }
  .hero-error { color: var(--error); }
  .delete-confirm { display: flex; align-items: center; justify-content: flex-end; flex-wrap: wrap; gap: 8px; color: var(--muted); }
  .permission-warning { margin: 0; color: var(--warn); }
  .close { width: 34px; height: 34px; border: 1px solid var(--line); border-radius: 7px; background: transparent; color: inherit; font-size: 21px; }
  footer { color: var(--muted); font-size: 13px; }
  footer button { padding: 7px 16px; border: 1px solid var(--accent); border-radius: 6px; background: var(--accent); color: var(--accent-text); }
  @media (max-width: 540px) { section { grid-template-columns: 1fr; } }
</style>
