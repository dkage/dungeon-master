---
status: parking-lot
milestone: after v1
github_issues: [79]
tracking: none
next: ideas only; promote an idea to an issue when v1 is stable
verified: 2026-10-13
---

# Dungeon Master — plan v2 (parking lot)

Unpromoted ideas only. Approved source registration/project mapping (#43/#51), workspace expansion/shortcuts (#63),
ready-work/planning (#67/#68), continuity (#69/#70) and outcome review (#71) now live in their design docs and roadmap.
Do not use this parking lot as a second plan or imply these approved additions already ship. v1 remains Claude-only.

## More agents

- `adapter` field in `party.toml`; registry of launchers (claude, codex, generic).
- Prompt delivery modes: argv / flag / typed-into-TUI fallback.
- agy: check its CLI flags and prompt delivery first.
- Also candidates: gemini, opencode.
- First-run PATH scan offering template heroes.
- Per-agent status detection: hooks where they exist, else OSC titles / screen rules via a VT emulator
  (`charmbracelet/x/vt`).
- For another provider, verify source-qualified method delivery and supporting-file access without repository mirrors.

## Quest sources

- Local markdown source (offline repos, no GitHub remote). Only if the quest model proves stable; `Source`
  interface should already allow it.
- Other trackers (Linear, Jira) as further sources.
- Issues/PRs from forks and upstream together.
- Monorepo helper in the UI: auto-detect workspaces (`go.work`, `apps/*`, pnpm/npm workspaces) and offer to create one
  campaign per subproject with a label filter. Stored locally.
- Optional *shared* mapping file (committed) only if a team asks for it; content-only (names, paths, labels), never
  hero/exec config, paths confined to the repo.

## GitHub depth

- GitHub App support: a registered App with installation tokens has its own, larger rate-limit bucket (up to 12,500 per
  hour, 15,000 on Enterprise Cloud) and lets a team attribute actions to one bot. It does not lift the secondary write
  limits, needs setup (create, install, store a private key) and makes issues appear authored by a bot. Only worth it
  for teams or very large organizations. The Quartermaster treats the token as an input, so this is another way to
  supply one.

- Local pins as a scope mode: the user marks issues as "mine to drive" inside the app, stored locally, so a campaign can
  be scoped without writing a label to a repository the user does not own or must not touch. No footprint on GitHub; the
  cost is that the choice is not shared and not visible in GitHub's own filters.
- Projects v2 status columns as dungeon floors.
- Webhook-free live updates via `gh api` polling tuning, or optional relay.
- Auto-comment of expedition summaries (currently user-confirmed only).
- PR review quest type: summon a hero on a PR.
- Create issues from the app (split a quest into sub-quests; hero proposes, you approve).

## Sessions

- Parallel issue sessions in one repository are planned in #79 (working copies #80/#81, session tabs #82).
- Persistent-session technology is decided by #69/#70; do not preselect tmux here.
- Claude transcript parsing: auto titles, token usage and cost per expedition.
- Provider conversation resume after verifying identifiers and recovery semantics; distinct from live reattachment.
- Agents coordinating with or reviewing each other's parallel work; sequential method review is planned in #71.

## Dungeon map

- Fog of war, animated state changes, milestone floors.
- Minimap, search and filter by label/assignee.
- Party view: which hero is where, token burn per room.
- More attention aggregation after #67/#71 validate the basic ready-work and review paths.

## Spells

- Label/type → spell presets (bug → debug spell).
- Skill authoring/editor after registered sources and delivery work (#43/#51/#66).
- Prompt presets catalog per campaign.
- Per-campaign prompt presets after source mapping and the method picker are delivered.

## Release / distribution

- Apple Developer ID signing + notarization.
- Intel / universal macOS build.
- Homebrew cask via GoReleaser tap.
- Linux deb/rpm (nfpm), Arch AUR.
- Windows WebView2 shell, built-in updater.

## Theme packs (initiative)

v1 ships the Dungeon and Neutral packs and the seams for more (`docs/design/ui.md` §6.5). Later:

- New packs, such as a starship pack (sectors, missions, crew, a star-chart map skin, its own icons and palettes).
- Per-pack component variants (framed panels, button shapes) through named variants and tokens, not forked components.
- Richer map skins per pack (art, portraits, animated tokens; `docs/design/map.md` directions E and F).
- User-installable packs: data only (words, tokens, sanitised SVGs), no scripts or arbitrary CSS.
- A pack gallery with previews.

## Polish / ideas

- Dice-roll flavor: random quest names, loot (summary of changes) on clear.
- Additional keybinding presets after configurable Shortcuts (#65).
- Scratch location setting.
- More panes: browser, source control, code review.
