<script lang="ts">
  import type { AppStore } from './store.svelte'
  import Term from './Term.svelte'
  let { store, openSettings }: { store: AppStore; openSettings: () => void } = $props()
  const repoCampaign = $derived(!!store.campaign?.repo && !store.campaign?.sample)
  const connected = $derived(store.campaign?.sample || (repoCampaign && !!store.quests && !store.questsError))
  const issueReady = $derived(store.questNumber !== null)
  const started = $derived(store.campaignId ? store.sessionsOf(store.campaignId).length > 0 : false)
  const mode = $derived(store.appearance.wordMode)

  const steps = $derived([
    { done: store.claude.found, title: 'Claude Code is available', help: store.claude.found ? 'Ready to start an agent.' : 'Install Claude Code or set its path in party.toml.' },
    { done: !!store.campaign, title: 'Add a project', help: 'Choose a suggested repository, browse to a folder, or try the sample.' },
    { done: !!connected, title: store.campaign?.sample ? 'Sample project ready' : 'Connect GitHub and choose issues', help: store.campaign?.sample ? 'Sample issues load locally; no GitHub connection is needed.' : 'The issue scope appears in the right panel after GitHub connects.' },
    { done: !!issueReady, title: store.appearance.wordMode === 'plain' ? 'Choose an issue' : 'Choose a quest', help: 'Select an issue in the right panel.' },
    { done: !!started, title: 'Start an agent', help: 'Choose a model at the top of the terminal.' },
  ])

  async function useSuggestion(suggestion: (typeof store.suggestions)[number]) {
    await store.addSuggestedProject(suggestion)
  }
</script>

<section class="start-here" aria-labelledby="start-title">
  <header>
    <div><p class="kicker">First visit</p><h1 id="start-title">Start here</h1></div>
    <button type="button" class="quiet" aria-label="Hide getting started" onclick={() => store.dismissFirstRun()}>×</button>
  </header>
  <p class="intro">Add a project, choose a <Term word="quest" mode={mode} />, then start an agent.</p>

  <ol class="steps">
    {#each steps as step, i}
      <li class:done={step.done} class:current={!step.done && !steps.slice(0, i).some((s) => !s.done)}>
        <span class="mark" aria-hidden="true">{step.done ? '✓' : i + 1}</span>
        <div><strong>{step.title}</strong><p>{step.help}</p></div>
      </li>
    {/each}
  </ol>

  {#if !store.campaign && store.suggestions.length > 0}
    <div class="suggestions" aria-label="Suggested projects">
      <h2>Suggested projects</h2>
      {#each store.suggestions as suggestion (suggestion.dir)}
        <button type="button" class="suggestion" disabled={store.busy} onclick={() => useSuggestion(suggestion)}>
          <span><strong>{suggestion.name}</strong><small>{suggestion.repo ?? suggestion.dir}</small></span><span aria-hidden="true">＋</span>
        </button>
      {/each}
    </div>
  {:else if !store.campaign && store.suggestionsLoading}
    <p role="status" class="muted">Looking for Git projects in common development folders…</p>
  {/if}

  {#if store.campaign?.sample}
    <button type="button" class="sample" disabled={store.busy} onclick={() => store.addSampleCampaign()}>Open the sample project</button>
  {:else}
    <button type="button" class="sample" disabled={store.busy} onclick={() => store.addSampleCampaign()}>
      {store.campaign ? 'Switch to the sample project' : 'Try the sample first'}
    </button>
  {/if}

  {#if store.questsError?.code === 'no_token' || store.questsError?.code === 'github_unauthorized'}
    <p class="credential" role="status">To connect GitHub, run <code>gh auth login</code> or set <code>GH_TOKEN</code> and reload.</p>
  {/if}

  <footer>
    <button type="button" class="quiet" onclick={openSettings}>Settings</button>
    <button type="button" class="quiet" onclick={() => store.loadSuggestions()}>Find projects again</button>
  </footer>
</section>

<style>
  .start-here { width: min(760px, calc(100% - 48px)); max-height: calc(100% - 32px); overflow-y: auto; margin: auto; padding: 22px; border: 1px solid var(--metal, var(--line)); border-radius: 7px; background: linear-gradient(155deg, var(--panel-raised, var(--panel)), var(--panel)); color: var(--text); box-shadow: inset 0 1px #e7bf6e1a, 0 14px 38px #0008; }
  header { display: flex; align-items: start; justify-content: space-between; }
  h1 { margin: 0; font: 700 27px/1.15 'Cinzel', var(--reading-font, Georgia, serif); }
  h2 { margin: 18px 0 8px; font-size: 12px; color: var(--muted); letter-spacing: .08em; text-transform: uppercase; }
  p { margin: 0; }
  .kicker { color: var(--accent); font-size: 11px; letter-spacing: .15em; text-transform: uppercase; }
  .intro { margin: 7px 0 14px; color: var(--muted); font-size: 15px; }
  .quiet { border: 0; padding: 5px 8px; background: transparent; color: var(--muted); font: inherit; cursor: pointer; }
  .steps { display: grid; gap: 0; margin: 0; padding: 0; list-style: none; }
  .steps li { display: grid; grid-template-columns: 28px minmax(0, 1fr); gap: 8px; padding: 7px 0; color: var(--muted); }
  .steps li.current { color: var(--text); }
  .steps li.done { color: var(--text); }
  .mark { display: grid; place-items: center; width: 24px; height: 24px; border: 1px solid var(--line); border-radius: 50%; font-size: 12px; }
  .done .mark { border-color: #47986a; color: #62ca8f; }
  .steps p { margin-top: 3px; font-size: 13px; color: var(--muted); }
  .suggestions { margin-top: 14px; }
  .suggestion { width: 100%; display: flex; justify-content: space-between; align-items: center; gap: 10px; padding: 10px 12px; border: 1px solid var(--line); border-bottom: 0; background: var(--bg); color: var(--text); text-align: left; }
  .suggestion:first-of-type { border-radius: 7px 7px 0 0; }
  .suggestion:last-child { border-bottom: 1px solid var(--line); border-radius: 0 0 7px 7px; }
  .suggestion span:first-child { display: grid; gap: 2px; min-width: 0; }
  .suggestion small { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--muted); }
  .sample { margin-top: 16px; padding: 9px 13px; border: 1px solid var(--accent); border-radius: 7px; background: var(--accent); color: var(--accent-text); font: inherit; cursor: pointer; }
  .credential { margin-top: 14px; padding: 10px 12px; background: var(--warn-bg); color: var(--warn); border-radius: 7px; }
  footer { display: flex; gap: 8px; margin-top: 16px; border-top: 1px solid var(--line); padding-top: 10px; }
  .muted { color: var(--muted); }
  @media (max-width: 760px) { .start-here { width: calc(100% - 24px); padding: 20px; } }
</style>
