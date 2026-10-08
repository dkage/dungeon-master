# Dungeon Master — plan v2 (parking lot)

Brainstorm only. Nothing here is in v1 (see `PLAN.md`: Claude only, GitHub issues read-mostly, one expedition per
working tree). Add ideas freely; promote to a real plan when v1 is stable.

## More agents

- `adapter` field in `party.toml`; registry of launchers (claude, codex, generic).
- Prompt delivery modes: argv / flag / typed-into-TUI fallback.
- agy: check its CLI flags and prompt delivery first.
- Also candidates: gemini, opencode.
- First-run PATH scan offering template heroes.
- Per-agent status detection: hooks where they exist, else OSC titles / screen rules via a VT emulator
  (`charmbracelet/x/vt`).
- Skills for agents without native skill support: mirror into repo + path in briefing.

## Quest sources

- Local markdown source (offline repos, no GitHub remote). Only if the quest model proves stable; `QuestSource`
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

- Projects v2 status columns as dungeon floors.
- Webhook-free live updates via `gh api` polling tuning, or optional relay.
- Auto-comment of expedition summaries (currently user-confirmed only).
- PR review quest type: summon a hero on a PR.
- Create issues from the app (split a quest into sub-quests; hero proposes, you approve).

## Sessions

- `git worktree` per expedition to allow parallel heroes in one repo.
- Run agents inside tmux for reattach after app restart.
- Claude transcript parsing: auto titles, token usage and cost per expedition.
- Resume (`--resume`) a previous expedition from its quest.
- Multi-hero quests: one hero implements, another reviews.

## Dungeon map

- Fog of war, animated state changes, milestone floors.
- Minimap, search and filter by label/assignee.
- Party view: which hero is where, token burn per room.
- Alternate "inbox" view: quests needing my attention (waiting-for-input, PR ready).

## Spells

- Label/type → spell presets (bug → debug spell).
- Git spellbooks with pinning; spell editor.
- Prompt presets catalog per campaign.
- Per-campaign custom spell buttons (v1 has global custom buttons only, `PLAN_UI.md` §6.2).

## Release / distribution

- Apple Developer ID signing + notarization.
- Intel / universal macOS build.
- Homebrew cask via GoReleaser tap.
- Linux deb/rpm (nfpm), Arch AUR.
- Windows WebView2 shell, built-in updater.

## Theme packs (initiative)

v1 ships the Dungeon and Neutral packs and the seams for more (`PLAN_UI.md` §6.5). Later:

- New packs, such as a starship pack (sectors, missions, crew, a star-chart map skin, its own icons and palettes).
- Per-pack component variants (framed panels, button shapes) through named variants and tokens, not forked components.
- Richer map skins per pack (art, portraits, animated tokens; `PLAN_MAP.md` directions E and F).
- User-installable packs: data only (words, tokens, sanitised SVGs), no scripts or arbitrary CSS.
- A pack gallery with previews.

## Polish / ideas

- Dice-roll flavor: random quest names, loot (summary of changes) on clear.
- Keybinding presets (vim/emacs).
- Scratch location setting.
- More panes: browser, source control, code review.
