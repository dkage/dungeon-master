<script lang="ts">
  import * as realApi from './api'
  import type { ProjectSkillSource, SkillSource, SkillSourceReport, SkillSourcesState, SkillSuggestion } from './api'
  import { wordLabel } from './words'

  type Api = Pick<typeof realApi, 'getSkillSources' | 'addSkillSource' | 'updateSkillSource' | 'refreshSkillSource' | 'removeSkillSource'
    | 'reorderSkillSources' | 'getProjectSkillSources' | 'setProjectSkillSources' | 'pickFolder'>
  let { campaignId = '', campaignName = '', wordMode = 'hints', api = realApi, now = () => Date.now(), onchange = () => {} }: {
    campaignId?: string
    campaignName?: string
    wordMode?: 'hints' | 'themed' | 'plain'
    api?: Api
    now?: () => number
    /** Called after anything that changes which skills a project can use. */
    onchange?: () => void
  } = $props()

  let sourcesState = $state<SkillSourcesState | null>(null)
  let project = $state<ProjectSkillSource[]>([])
  let error = $state('')
  let loadError = $state('')
  let busy = $state(false)
  let removeId = $state<string | null>(null)

  // The registration dialog's draft; editing is the same form with the source's own values.
  let draft = $state<{ id: string; name: string; kind: 'folder' | 'git'; location: string; pin: string; suggestion: string; notice: string } | null>(null)

  // The form takes focus when it opens, so it can be filled in from the keyboard.
  let nameInput = $state<HTMLInputElement>()
  $effect(() => { if (draft && nameInput) nameInput.focus() })

  const title = $derived(`${wordLabel('spell', wordMode)} sources`)
  const reportOf = (id: string): SkillSourceReport | undefined => sourcesState?.reports.find((r) => r.sourceId === id)
  const registered = (s: SkillSuggestion) => !!sourcesState?.sources.some((x) => x.location === s.location)

  // Reloads can overlap (a change, a rescan, a project switch); only the newest one may write its answer.
  let loadSeq = 0
  async function load() {
    const seq = ++loadSeq
    loadError = ''
    try {
      const book = await api.getSkillSources()
      const mine = campaignId ? ((await api.getProjectSkillSources(campaignId)).sources ?? []) : []
      if (seq !== loadSeq) return
      sourcesState = { ...book, sources: book.sources ?? [], suggestions: book.suggestions ?? [], reports: book.reports ?? [] }
      project = mine
    } catch (e) {
      if (seq === loadSeq) loadError = message(e, 'Could not load the sources.')
    }
  }
  $effect(() => { void campaignId; void load() })

  const message = (e: unknown, fallback: string) => (e instanceof Error && e.message ? e.message : fallback)

  async function run(work: () => Promise<void>, fallback: string) {
    busy = true
    error = ''
    try { await work(); onchange() } catch (e) { error = message(e, fallback); onchange() } finally { busy = false }
  }

  function beginNew(kind: 'folder' | 'git' = 'folder', from?: SkillSuggestion) {
    error = ''
    draft = from
      ? { id: '', name: from.name, kind: 'git', location: from.location, pin: '', suggestion: from.key, notice: from.notice }
      : { id: '', name: '', kind, location: '', pin: '', suggestion: '', notice: '' }
  }
  function beginEdit(s: SkillSource) {
    error = ''
    draft = { id: s.id, name: s.name, kind: s.kind, location: s.location, pin: s.pin ?? '', suggestion: '', notice: s.notice ?? '' }
  }
  async function pick() {
    const r = await api.pickFolder()
    if (draft && r.path) draft.location = r.path
  }
  async function save() {
    const d = draft
    if (!d) return
    if (!d.name.trim()) { error = 'Give the source a name.'; return }
    await run(async () => {
      if (d.id) {
        const s = sourcesState?.sources.find((x) => x.id === d.id)
        const patch: realApi.SkillSourcePatch = { name: d.name.trim() }
        if (s && d.location !== s.location) patch.location = d.location.trim()
        if (s && d.kind === 'git' && d.pin.trim() !== (s.pin ?? '')) patch.pin = d.pin.trim()
        await api.updateSkillSource(d.id, patch)
      } else {
        await api.addSkillSource(d.suggestion
          ? { name: d.name.trim(), suggestion: d.suggestion, pin: d.pin.trim() }
          : { name: d.name.trim(), kind: d.kind, location: d.location.trim(), pin: d.kind === 'git' ? d.pin.trim() : undefined })
      }
      draft = null
      await load()
    }, 'Could not save the source.')
  }
  const toggle = (s: SkillSource, enabled: boolean) => run(async () => { await api.updateSkillSource(s.id, { enabled }); await load() }, 'Could not change the source.')
  const refresh = (s: SkillSource) => run(async () => { await api.refreshSkillSource(s.id); await load() }, 'Could not refresh the source.')
  const remove = (s: SkillSource) => run(async () => { await api.removeSkillSource(s.id); removeId = null; await load() }, 'Could not remove the source.')
  function move(id: string, by: -1 | 1) {
    const ids = sourcesState?.sources.map((s) => s.id) ?? []
    const i = ids.indexOf(id)
    const j = i + by
    if (i < 0 || j < 0 || j >= ids.length) return
    ;[ids[i], ids[j]] = [ids[j], ids[i]]
    return run(async () => { sourcesState = await api.reorderSkillSources(ids); await load() }, 'Could not change the order.')
  }

  // A project lists its sources itself once the operator changes anything; "Use defaults" clears that.
  const saveProject = (list: ProjectSkillSource[]) =>
    run(async () => { project = (await api.setProjectSkillSources(campaignId, list.map((s) => ({ sourceId: s.id, enabled: s.enabled })))).sources ?? [] }, 'Could not save the project\'s sources.')
  const setProjectEnabled = (id: string, enabled: boolean) => saveProject(project.map((s) => (s.id === id ? { ...s, enabled } : s)))
  function moveProject(id: string, by: -1 | 1) {
    const list = [...project]
    const i = list.findIndex((s) => s.id === id)
    const j = i + by
    if (i < 0 || j < 0 || j >= list.length) return
    ;[list[i], list[j]] = [list[j], list[i]]
    return saveProject(list)
  }
  const useDefaults = () => run(async () => { project = (await api.setProjectSkillSources(campaignId, [])).sources ?? [] }, 'Could not reset the project\'s sources.')

  function age(iso: string): string {
    const mins = Math.max(0, Math.round((now() - Date.parse(iso)) / 60000))
    if (mins < 1) return 'just now'
    if (mins < 60) return `${mins} min ago`
    if (mins < 2880) return `${Math.round(mins / 60)} h ago`
    return `${Math.round(mins / 1440)} days ago`
  }
  const where = (s: SkillSource) => s.kind === 'folder' ? s.location : `${s.location}`
  const pinned = (s: SkillSource) => s.kind === 'git' ? `${s.pin || 'default branch'}${s.verified ? ` · ${s.verified.revision.slice(0, 12)}` : ''}` : ''
</script>

<section class="skills" aria-labelledby="skill-sources-title" id="skill-sources">
  <div class="section-heading">
    <div><h2 id="skill-sources-title">{title}</h2><p>Folders and Git repositories holding skills. An earlier enabled source wins when two skills share a name. Nothing is written into your projects.</p></div>
    <div class="actions"><button type="button" onclick={load} disabled={busy}>Rescan</button><button type="button" onclick={() => beginNew()} disabled={busy}>New source</button></div>
  </div>

  {#if sourcesState}
    {#if sourcesState.sources.length === 0}<p class="muted">No sources yet. Register a folder or a repository to offer its skills.</p>{:else}
      <ul class="list" aria-label="Registered sources">
        {#each sourcesState.sources as s, i (s.id)}
          {@const r = reportOf(s.id)}
          <li class:off={!s.enabled}>
            <input type="checkbox" checked={s.enabled} aria-label={`Use ${s.name}`} onchange={(e) => toggle(s, e.currentTarget.checked)} disabled={busy} />
            <div class="what">
              <strong>{s.name}</strong>
              <span class="where">{where(s)}</span>
              {#if s.kind === 'git'}<span class="pin">{pinned(s)}</span>{/if}
              {#if s.license}<span class="pin">{s.license} licence</span>{/if}
              {#if s.notice}<span class="notice">{s.notice}</span>{/if}
              {#if s.error}<span class="warn" role="status">Last refresh failed {age(s.error.at)}: {s.error.message}.{s.verified ? ` Showing the version from ${age(s.verified.at)}.` : ''}</span>{/if}
              {#if r?.unavailable}<span class="warn">{r.unavailable}</span>{/if}
              {#each r?.warnings ?? [] as w (w)}<span class="warn">{w}</span>{/each}
            </div>
            <span class="count" aria-label={`${r?.count ?? 0} skills`}>{r?.count ?? 0}</span>
            <div class="row-actions">
              <button type="button" aria-label={`Move ${s.name} up`} onclick={() => move(s.id, -1)} disabled={busy || i === 0}>↑</button>
              <button type="button" aria-label={`Move ${s.name} down`} onclick={() => move(s.id, 1)} disabled={busy || i === sourcesState.sources.length - 1}>↓</button>
              <button type="button" onclick={() => beginEdit(s)} disabled={busy}>Edit</button>
              {#if s.kind === 'git'}<button type="button" onclick={() => refresh(s)} disabled={busy}>Refresh</button>{/if}
              <button type="button" onclick={() => (removeId = s.id)} disabled={busy}>Remove</button>
            </div>
            {#if removeId === s.id}
              <div class="confirm"><span>Remove {s.name}? {s.kind === 'folder' ? 'Your folder is not touched.' : 'Only the copy kept by this app is deleted.'}</span>
                <button type="button" onclick={() => (removeId = null)}>Keep it</button><button type="button" onclick={() => remove(s)}>Remove source</button></div>
            {/if}
          </li>
        {/each}
      </ul>
    {/if}
    {#each sourcesState.suggestions.filter((x) => !registered(x)) as sg (sg.key)}
      <div class="suggest"><span>{sg.name} ({sg.license}), from {sg.location}. Fetched when you add it; nothing is bundled.</span><button type="button" onclick={() => beginNew('git', sg)} disabled={busy}>Add {sg.name}…</button></div>
    {/each}
  {:else}<p class="muted">Loading sources…</p>{/if}

  {#if loadError}<p class="error" role="status">{loadError}</p>{/if}
  {#if error}<p class="error" role="alert">{error}</p>{/if}

  {#if draft}
    <div class="editor" role="group" aria-labelledby="source-editor-title">
      <h3 id="source-editor-title">{draft.id ? `Edit ${draft.name}` : 'Register a source'}</h3>
      <label class="field"><span>Name</span><input bind:this={nameInput} bind:value={draft.name} placeholder="my skills" maxlength="80" /></label>
      {#if !draft.id && !draft.suggestion}
        <label class="field"><span>Kind</span>
          <select bind:value={draft.kind}><option value="folder">Folder on this machine</option><option value="git">Git repository</option></select></label>
      {/if}
      {#if draft.kind === 'folder'}
        <label class="field"><span>Path</span>
          <span class="path"><input bind:value={draft.location} placeholder="/Users/you/skills" autocomplete="off" spellcheck="false" /><button type="button" onclick={pick}>Select folder</button></span></label>
        <small class="hint">Your folder, edited by you. This app only reads it. Use an absolute path.</small>
      {:else}
        <label class="field"><span>URL</span><input bind:value={draft.location} placeholder="https://github.com/someone/skills.git" readonly={!!draft.suggestion} autocomplete="off" spellcheck="false" /></label>
        <label class="field"><span>Ref</span><input bind:value={draft.pin} placeholder="The default branch" autocomplete="off" spellcheck="false" /></label>
        <small class="hint">Optional branch, tag or commit; blank follows the default branch. The exact commit is shown and kept until you refresh. Register repositories you trust: their files are read, never run.</small>
        {#if draft.notice}<small class="hint">{draft.notice}</small>{/if}
      {/if}
      <div class="editor-actions"><button type="button" onclick={() => (draft = null)}>Cancel</button><button type="button" onclick={save} disabled={busy}>{draft.id ? 'Save' : 'Register'}</button></div>
    </div>
  {/if}

  {#if campaignId && project.length > 0}
    <div class="project" role="group" aria-label={`Sources for ${campaignName}`}>
      <div class="section-heading"><div><h3>For {campaignName}</h3><p>{project.some((s) => s.override) ? 'This project has its own list.' : 'Using the defaults above. Change anything to give this project its own list.'}</p></div>
        <button type="button" onclick={useDefaults} disabled={busy || !project.some((s) => s.override)}>Use defaults</button></div>
      <ul class="list">
        {#each project as s, i (s.id)}
          <li class:off={!s.enabled}>
            <input type="checkbox" checked={s.enabled} aria-label={`Use ${s.name} in ${campaignName}`} onchange={(e) => setProjectEnabled(s.id, e.currentTarget.checked)} disabled={busy} />
            <div class="what"><strong>{s.name}</strong>{#if !s.override}<span class="pin">default</span>{/if}</div>
            <span></span>
            <div class="row-actions">
              <button type="button" aria-label={`Move ${s.name} up in ${campaignName}`} onclick={() => moveProject(s.id, -1)} disabled={busy || i === 0}>↑</button>
              <button type="button" aria-label={`Move ${s.name} down in ${campaignName}`} onclick={() => moveProject(s.id, 1)} disabled={busy || i === project.length - 1}>↓</button>
            </div>
          </li>
        {/each}
      </ul>
    </div>
  {/if}
</section>

<style>
  .skills { display: grid; grid-template-columns: 1fr; gap: 12px; padding: 20px 22px; border-bottom: 1px solid var(--line); }
  h2 { margin: 0 0 6px; color: var(--muted); font-size: 12px; letter-spacing: .08em; text-transform: uppercase; }
  h3 { margin: 0; font-size: 14px; }
  p { margin: 0; }
  .section-heading { display: flex; align-items: flex-start; justify-content: space-between; gap: 12px; }
  .section-heading p, .muted, .hint { color: var(--muted); font-size: 12px; }
  .actions, .row-actions, .editor-actions { display: flex; gap: 6px; flex-wrap: wrap; }
  .editor-actions { justify-content: flex-end; }
  .list { list-style: none; margin: 0; padding: 0; display: grid; gap: 0; border-top: 1px solid var(--line); }
  .list li { display: grid; grid-template-columns: auto minmax(0, 1fr) auto auto; align-items: start; gap: 4px 10px; padding: 9px 4px; border-bottom: 1px solid var(--line); font-size: 13px; }
  .list li.off .what { opacity: .6; }
  .what { display: grid; gap: 2px; min-width: 0; }
  .where, .pin, .notice { color: var(--muted); font-size: 12px; overflow-wrap: anywhere; }
  .warn { color: var(--warn); font-size: 12px; }
  .count { min-width: 2ch; text-align: right; font-variant-numeric: tabular-nums; }
  .confirm { grid-column: 1 / -1; display: flex; align-items: center; justify-content: flex-end; flex-wrap: wrap; gap: 8px; color: var(--muted); }
  .suggest { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 8px 10px; border: 1px dashed var(--line); border-radius: 8px; color: var(--muted); font-size: 12px; }
  .error { color: var(--error); }
  .editor, .project { display: grid; gap: 12px; padding: 14px; border: 1px solid var(--line); border-radius: 8px; background: var(--bg); }
  .field { display: grid; gap: 6px; color: var(--muted); font-size: 13px; }
  .path { display: grid; grid-template-columns: 1fr auto; gap: 6px; }
  input:not([type='checkbox']), select { width: 100%; box-sizing: border-box; min-height: 34px; padding: 6px 8px; border: 1px solid var(--line); border-radius: 5px; background: var(--panel); color: var(--text); font: inherit; }
  @media (max-width: 540px) { .list li { grid-template-columns: auto minmax(0, 1fr) auto; } .row-actions { grid-column: 1 / -1; } .section-heading { flex-direction: column; } }
</style>
