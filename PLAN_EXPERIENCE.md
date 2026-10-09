# Dungeon Master: experience plan

How the app should feel from the first launch to the hundredth quest. Extends `PLAN.md` (it changes milestone order and
adds acceptance rules, it does not replace the architecture), `PLAN_SKILLS.md` and `release/PLAN_RELEASE.md`. Status of
the work lives in `CLAUDE.md`, not here.

Written against the tree as of 2026-10-08, updated after M3 (quests list and detail) landed. The first new milestone is
M3.6. (`PLAN_M3_5.md` is a separate plan, about GitHub limits and scope; see §4 for how the two fit.)

## 1. Where we are (verified)

Checked by reading the code and running `go vet ./...` and `go test ./...` (all packages passed before M3 finished;
re-run `make check` for the final tree). The frontend tests were not run by the author of this plan.

What works:

- One Go binary with an embedded Svelte SPA; token, Host and Origin guard; PTY sessions with scrollback and
  process-group kill; session history across restarts.
- Campaigns: registered directories, git root and `owner/repo` derived on load.
- Heroes from `party.toml`; three built-in starter heroes when the file is missing, announced by a notice.
- Quests (M3, done): a `Source` interface with a GitHub GraphQL client and a demo source; derived state (open, in
  progress, locked, cleared); a 60 s cache with throttled refresh and stale fallback; a right-hand list with nesting,
  filters and counts; a detail view with sanitised markdown; error messages that say what to do next. Relationship
  fields (parent, blocked-by, sub-issues) come from GitHub in one query, so the map's data already exists.
- Layout already matches the plan: campaigns and expeditions on the left, terminal in the middle, quests on the right (a
  drawer under 1180 px).

What is missing or weak for someone opening it for the first time:

| Gap                                                                                                                                                                                                     | Evidence in the code                  |
|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|---------------------------------------|
| **No first-run guidance.** An empty app says "Add a campaign on the left to begin." and nothing else.                                                                                                   | `App.svelte`, `Sidebar.svelte`        |
| **Adding a campaign means typing an absolute path.** No suggestions, no "use the folder I launched from".                                                                                               | `Sidebar.svelte` add form             |
| **Themed words are never defined.** Campaign has a hint; hero, quest, expedition, spell, briefing do not. The first button row is "Opus Sonnet Haiku Shell Quests" with no explanation.                 | `Toolbar.svelte`                      |
| **Sample data is a toggle on a real campaign.** "Load demo quests" appears on a real campaign when GitHub fails, so real and sample data share one view (it is labelled, but still mixed).              | `QuestsPanel.svelte`, `store.setDemo` |
| **Starter heroes are a permanent banner**, not an action. The user is told to create a file by hand.                                                                                                    | `Toolbar.svelte` notice               |
| **A quest and a session are not connected yet.** Starting a hero ignores the selected quest (that is M4).                                                                                               | `store.launch`                        |
| **No visual identity.** One generic palette, no tokens, nothing RPG anywhere, including the places that should carry it (empty states, state markers, the map).                                         | `app.css`                             |
| **The map does not exist, and it is scheduled last** (M6, "can slip"). It is the one thing that makes the app different from a plain launcher.                                                          | `PLAN.md` §11                         |
| **No attention signal.** A dot for running or exited. Nothing tells you an agent is waiting for you while you are in another tab.                                                                       | `Sidebar.svelte`                      |
| **Silent limits in the quest list.** At most 300 open issues and the 50 most recently updated closed ones are fetched, and the panel does not say so. A large repository looks complete when it is not. | `internal/quests/github/client.go`    |
| **Docs drift** remains in `PLAN.md` (stale status line, old state names, old sync design); see the docs audit list kept with the milestone notes.                                                       | `PLAN.md` §2, §4, §6, §12, §13        |
| **Dev state hides first-run problems.** Every developer run uses an existing config directory, so empty states are never seen.                                                                          | `make run`                            |

## 2. Lessons we carry into the design

These come from studying how agent-orchestration tools go wrong. They are stated as rules for this project.

1. **The terminal is the product.** Steering a live agent is the value. Anything that moves interaction out of the
   terminal (headless queues, structured result screens) is a different product.
2. **First value before concepts.** A new user should reach a running agent session in under two minutes without reading
   a document. Concepts are introduced at the moment they are needed.
3. **A themed word must teach itself.** Names are fine; unexplained names are not. A newcomer who does not know what a
   word means is lost, so every themed word shows its plain meaning where it appears.
4. **Never mix sample, default and real data.** Each kind is visibly different and removable on its own terms.
5. **One obvious next action per screen.** Counters and lists are not a next action.
6. **Concept budget.** Every new noun the user sees costs attention. Cap it and defend the cap.
7. **Do not build a platform.** No second tracker, no database server, no workflow engine, no registries or versioned
   bundles, no memory store, no scoring. GitHub, Claude Code and the user's own files already do those jobs.
8. **Decide the visual early.** A visual that waits until the end gets cut or bolted on. The map shapes the layout, so
   it is designed with the layout.
9. **Validate by use, not by plan.** Each milestone ends with a fresh-state walkthrough by a person (or a script that
   acts like one), not a checkbox in a plan.
10. **Plans state intent; status lives in one place.** Appending progress to plans makes them unreadable.

## 3. Product shape

Three columns, one centre of gravity:

```
┌──────────────┬──────────────────────────────────┬───────────────────────┐
│ CAMPAIGNS    │                                  │ MAP / LIST            │
│  repo A      │                                  │  filter, state tally  │
│   ● #42 ...  │  selected terminal               │  quest graph or list  │
│   ○ #17 ...  │                                  │ ───── resize ──────── │
│  repo B      │                                  │ QUEST DETAIL          │
│              │                                  │  issue content        │
│ + Add        │                                  │  launch controls M4   │
└──────────────┴──────────────────────────────────┴───────────────────────┘
```

Changes from today:

- **The app keeps the Stitch cockpit layout:** campaigns and sessions on the left, the selected terminal in the centre,
  and the quest workspace on the right. The right panel is split vertically: map/list above, selected quest detail
  below. The quest list remains the precise, searchable alternative to the map.
- **The hero buttons move into the quest detail** as one primary button, "Start with <default hero>", with a menu for
  another hero and an optional spell. The toolbar keeps only "Shell" (free session) and the campaign identity. This
  removes the "which of these buttons do I press?" problem: you start a quest, not a model.
- **A session knows its quest.** Sidebar entries read "#42 Validate email addresses", not "claude".
- **The help layer is built in** (§5), not a separate document.

## 4. Milestones (revised order)

M0 to M4 are done. New or moved work is marked.

| Milestone                                   | Content                                                                                                                                                                      | Exit test (a person does this on a fresh config directory)                                                                                                                                     |
|---------------------------------------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| **M3** (done)                               | Quests list and detail, GitHub source, demo source, sanitised markdown                                                                                                       | Met. Follow-ups carried into M3.6 (the scope part of the truncation fix is Phase B of `PLAN_M3_5.md`): say when the list is truncated, and stop offering sample quests inside a real campaign. |
| **M3.6 Stitch shell, first run and words** (done) | Implement the Stitch v4 cockpit structure and visual foundation in Svelte; fit onboarding, sample campaign, repo suggestions, glossary and `Term`, starter heroes action, empty states, appearance settings, `make run-fresh`, and the first-run browser test into that shell. Reserve the right-panel map/detail split with existing list/detail behavior; do not build the graph yet. | Met from a fresh config; the Stitch shell has a clear place for the future map and quest launch controls. |
| **M4 Launch from a quest** (done)           | Briefing, pinned detail-footer hero/model and skill controls, primary "Start with" button, session titles from quests, quest chip, one-session-per-tree gate; neutral GitHub write-back stays opt-in and off | Met from a fresh config; see GitHub M4 parent issue #19 and `make check` / `make test-e2e`.                                                        |
| **M4.5 Attention** (pulled forward from M7) | "Needs you" signal from PTY output recency (hooks replace it in M7), tab title and favicon badge, optional OS notification                                                   | Start two sessions, switch away, and know which one stopped to ask something without looking.                                                                                                  |
| **M5a Map v1** (pulled forward from M6)     | Fill the reserved right-panel map region with a read-only layered graph in Svelte/SVG: quest rooms use the 15 generated creature sheets selected by known effort tier and a stable per-issue assignment; show blocked-by and parent links, independent state markers, Map/List selection, and click-to-select / double-click-to-start. Add deterministic fallback rooms for unknown tiers or missing art and honor reduced motion. | On a real repository, the dependency picture is readable faster than the list; the same issue keeps its creature after restart, and unknown effort remains clear without guessing. |
| **M5b Spells**                              | Discovery, suggested spell per quest, spell menu in the Start button                                                                                                         | A bug-labelled quest offers the bug skill as the default and one click runs it.                                                                                                                |
| **M6 Map v2**                               | Fog for unspecified work, glyphs by label, milestone floors, live occupancy                                                                                                  | Polish only; M5a already carries the value.                                                                                                                                                    |
| **M7 Hooks status**                         | Hook-driven working, idle, needs-input replaces the output heuristic                                                                                                         | Same exit as M4.5 with exact states.                                                                                                                                                           |
| **M8 Packaging**                            | As in `release/PLAN_RELEASE.md`                                                                                                                                              | Unchanged.                                                                                                                                                                                     |

Why the map moves ahead of spells: spells add convenience to something that already works, the map is the reason to pick
this app over a plain launcher. A crude map made early also forces the layout and data questions while they are cheap.
This ordering is a recommendation; it is the one decision worth confirming (see §10).

## 5. First-run experience (M3.6)

### 5.1 The Start-here panel

Shown in the stage whenever there is no expedition, and reopenable from a help item. It is a short checklist whose steps
tick themselves:

1. **Claude Code found.** If not: the exact message and the `party.toml` line to set the path. Detection already exists.
2. **Add a project.** Suggest repositories instead of asking for a path: the directory `dungeon serve` was launched from
   if it is a git repo, plus recently modified git repos under common development folders (`~/Developer`, `~/code`,
   `~/src`, `~/dev`, `~/projects`, depth-limited, read-only scan, nothing stored). One click adds one; a text field
   remains as the fallback.
3. **Connect GitHub.** Show the detected source of credentials (`gh auth token`, `GH_TOKEN`) or the single command to
   run. Skip silently when the campaign has no GitHub remote.
4. **Pick a quest.** Text: "A quest is a GitHub issue." with a shortcut to the first open one.
5. **Start.** The big button.

Each step shows done, current or blocked, and blocked steps say why in one sentence. A "Try the sample first" link at
the top jumps to the sample campaign.

### 5.2 Sample campaign

- A **separate campaign**, never a mode of a real one: name "Sample (not a real project)", a persistent "Sample" badge
  in the sidebar and a banner in its quest panel, working directory in a scratch location under the state directory.
- Contents: the existing demo quests, so the list, detail, blockers and sub-issues can be explored. The agent can still
  be started in it (a throwaway folder), so the whole loop is learnable with no repository.
- **Removable with one click**, and removing it never touches real data. It reappears only from the Start-here panel.
- Remove the current "Load demo quests" button from the error state of a real campaign. A failed GitHub fetch shows its
  error and the fix; it does not offer to swap in fake data.
- Rule: sample data is created only by an explicit user action, and everything derived from it carries the badge.

### 5.3 Starter heroes

- When `party.toml` is missing, built-in heroes carry a small "starter" tag and the notice offers **Keep these starter
  heroes**. This explicitly writes the local file (0600, atomically); the heroes then appear in Settings for editing and
  the starter notice disappears. The user does not need to open or edit the file directly; Settings owns it as described
  in `PLAN_UI.md` §6.1.
- A malformed `party.toml` shows its error with the line and keeps the last good party running.
- File changes reload without a restart.
- The default hero is a stored setting (first hero until changed) and is what "Start with" uses.

### 5.4 Empty states

Every empty list says three things: what goes here, what creates one, why it is worth it. Examples: no campaigns, no
issues in this repository, no expeditions, no matching quests, no GitHub remote. No celebratory messages, toasts, or
flavour text before the first real launch.

## 6. Words (glossary and inline help)

- One typed module, `web/src/lib/words.ts`: for each themed term the themed word, the plain meaning, and a one-line
  definition. It is the only place those words are written in the UI.
- A `Term` component renders a themed word with a dotted underline and a popover: "Quest: a GitHub issue you can send an
  agent to." Used in headings, buttons, tooltips and empty states.
- **First sight shows both**: "Quests (issues)", "Hero (a Claude setup)". After the user dismisses the intro, or after
  the term has been seen a few times, only the themed word shows. The popover always remains.
- A help page generated from the same module lists every term.
- Terms never explain themselves with other themed terms. Definitions use plain words first.
- Safety information is never themed away: anything that runs without sandbox or with skipped permissions says so in
  plain text next to the themed name.
- Terms stay out of GitHub-facing text and Go identifiers, as in `CLAUDE.md`. A test fails if a themed word appears in a
  string sent to GitHub.
- A second glossary (a plain-words toggle) is possible later because every word is already in one table; it is not built
  now.

### Concept budget

The user sees at most **eight** nouns: campaign, quest, hero, spell, expedition, briefing, map (the room graph), and
settings. Today's extra words are folded in:

- Torch becomes a plain setting: "Mark issues in progress on GitHub (off)".
- Spellbook becomes a plain "Skill sources" list in settings.
- Dungeon is the map; the word is used only in the map's own help.

Adding a ninth noun needs a written reason: what screen needs it, why an existing word cannot carry it, and what the
popover says.

## 7. UI and visual design

### 7.1 Principles

- The terminal gets the largest area and the calmest surroundings. Chrome stays quiet around it.
- The theme is concentrated where it carries meaning: the wordmark, state markers, the map, empty-state illustrations.
  Controls, forms and text stay conventional and dense.
- Colour never carries meaning alone (the existing screen-reader text for states stays; add shapes).
- Everything reachable by keyboard; nothing requires hovering to discover.

### 7.2 Design tokens

Replace ad hoc values in `app.css` with a small token set before more screens exist: surfaces (3 levels), text (2
levels), one accent, four state colours (open, claimed, locked, cleared) with a shape each, radius scale, spacing scale,
type scale, and one monospace stack shared with the terminal theme. Dark is the default look; light follows the system.
Tokens live in one file; components reference tokens only.

**First step of the UI work: implement the settled font choice.** The comparison is in
`design/font-study/`; `PLAN_UI.md` §6.5 records the decision. Appearance & words offers Almendra (default), EB Garamond,
and Alegreya for the campaign and quest panels, plus an independent size control for each panel including the terminal.
The terminal keeps its monospaced Nerd Font stack. Keep Cinzel for display headings and Fira Code for code samples.
Bundle the font files used by the app with their matching SIL Open Font License texts, and retain Portuguese accents in
the typography specimen and UI checks.

### 7.3 State language

One set of markers used everywhere (list, map, tabs, sidebar):

| State               | Meaning                     | Marker                      | Map                    |
|---------------------|-----------------------------|-----------------------------|------------------------|
| Open                | open, unblocked, unassigned | hollow circle               | lit room               |
| Claimed             | assigned to someone         | half-filled circle          | occupied room          |
| Locked              | has an open blocker         | padlock                     | dim room behind a door |
| Cleared             | closed                      | check                       | explored room          |
| Needs you (session) | agent is waiting            | pulsing ring plus tab badge | glowing room           |

### 7.4 The map

- Read-only. GitHub owns relationships; the map never edits them, so there is no cycle handling, conflict handling or
  write path.
- Layered left-to-right layout from blocked-by and parent links (dagre or elk, behind a small interface `mount`,
  `setModel`, `onSelect` so the renderer can change).
- SVG first (accessible, testable, themeable). Canvas only if a real repository proves SVG too slow.
- Rooms show number, title (truncated), state marker and label glyph; edges are doors. Selecting a room selects the
  quest everywhere; double-click or Enter starts it.
- Large graphs: collapse a parent's children, zoom and pan, a "focus on frontier" toggle that dims everything not
  startable. A text alternative (the list) always exists.
- Fog, milestone floors and animation come in M6, after the plain map has been used on real repositories.

### 7.5 Interaction

- Command palette (Ctrl or Cmd K): jump to a campaign, a quest by number or title, an expedition, or "start quest". It
  replaces most navigation.
- List keyboard model: arrows or j/k move, Enter opens detail, s starts with the default hero, / filters.
- One primary button per context, always the same place: "Start with <hero>" in quest detail. Secondary actions live in
  its menu.
- Closing or removing anything destructive asks once, inline, with the consequence ("Removes the campaign from this app.
  Your files are not touched.").

### 7.6 Responsiveness and text

- Keep the current drawer behaviour for the quest column; the stage never goes below a usable terminal width.
- User-visible strings in sentence case, no jargon without a `Term`, no emoji, error text that ends in a next step (the
  quest errors already do this; it becomes the rule everywhere).

## 8. Guardrails against sprawl

Written as rules; each is cheap to check in review.

- **Surface cap.** Three columns with the right-panel map/detail workspace, a settings sheet, and a help page. A new top-level surface needs a
  written case against putting it in an existing one.
- **Scope fences.** Not in v1 and not to be built "just in case": a task store of our own, a database server, a workflow
  or pipeline engine, registries or versioned bundles of hero setups, containers or sandboxes, a memory or knowledge
  store, autonomy beyond "you press the button", scoring, achievements or levels, headless runs with result screens.
- **Own as little as possible.** Skills come from a pinned external source (as in `PLAN_SKILLS.md`); issue relationships
  come from GitHub; status comes from Claude's hooks.
- **Data honesty.** Three kinds of data (yours, sample, starter) each have their own look and removal path. Nothing
  system-owned appears in a list of user-owned things without a tag.
- **No reward before value.** No toasts, flavour text or badges before a real expedition has run.
- **Plan hygiene.** Plans hold intent and acceptance tests. Status is the `CLAUDE.md` line. Update it and the README
  status in the same commit that finishes a milestone.

## 9. Process changes

1. **Fresh-state runs.** `make run-fresh` serves with a throwaway `DUNGEON_HOME`. Every milestone's exit test runs
   there, because a developer's own state hides empty states and first-run problems.
2. **First-run browser test.** Playwright against the real binary with an empty config directory and the fake `claude`
   stub: Start-here appears, the sample campaign loads, a quest can be selected and a session can be started inside the
   sample workspace, and the sample badge is visible. Quest-bound session titles belong to M4. This test is part of
   `make test-e2e`.
3. **Screenshot walkthrough per milestone.** Capture the key screens (empty, first run, quest selected, session running,
   map) and look at them before calling the milestone done. Store them in the gitignored `_probe/`.
4. **Unexplained-word check.** A unit test walks `words.ts` and fails if a term lacks a plain meaning or if its
   definition uses another themed term as the only explanation.
5. **Themed words never leak.** A test over everything written to GitHub or git (comment bodies, labels) asserts no
   themed word appears.
6. **Concept-budget check in review.** The PR template for UI changes asks: does this add a noun, a top-level surface or
   a setting?
7. **Every change keeps the two-level tests rule** from `CLAUDE.md` (unit plus end-to-end).

## 10. Decisions to confirm

1. **Map before spells** (this plan) or spells first (the old order)?
2. **Repo suggestions**: is a bounded, read-only scan of common development folders acceptable, or should it be
   launch-directory only plus manual paths?
3. **Sample campaign working directory**: a scratch directory under the state directory, with a real agent session
   allowed, or sessions disabled for the sample?
4. **Windows in v1?** It decides the PTY approach and the release matrix before M8.
5. **Does an unattended mode** (non-interactive `claude -p` for research tickets) belong in v1, v2, or never? It is the
   one feature that conflicts with "the terminal is the product", so it stays in `PLAN_V2.md` until decided.

## 11. Definition of done for the experience

- Fresh install to a live agent session on a sample or real quest in two minutes or less, no document opened.
- Show the app to someone for five seconds: they can say what it is for and what to click.
- No word on screen is unexplained.
- Nothing on screen is of unknown origin (yours, sample, or starter).
- The dependency picture of a real repository is visible without reading the list.
- You know which of several running agents needs you without opening each.
