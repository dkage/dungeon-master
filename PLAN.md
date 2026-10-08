# Dungeon Master — plan

A desktop cockpit for a human "Dungeon Master" who runs a party of CLI coding agents (Claude in v1; Codex and agy later)
against **GitHub issues**. One button = one agent CLI launched in a PTY with its model, flags and context already set.

Status: plan only. The repo currently holds a GoLand hello-world (`main.go`, `go.mod` = `module dungeon`, go 1.26) — to
be replaced.

## Design principle: invisible to the team

Not everyone in a company will use this app, and the RPG theme is personal. So **Dungeon leaves no trace in the repo or
on GitHub by default**:

- All config is local, under the app's own state dir (`party.toml`, campaign registry, monorepo subproject mapping).
  **No committed `dungeon.json` or `.dungeon/` folder.** Nothing for a teammate to review, merge or be confused by.
- Themed words (campaign, quest, hero, spell…) exist only in this app's UI. Anything written to GitHub or git uses
  neutral wording.
- Anything that writes to the shared world is opt-in and off by default.

## v1 scope

**Claude Code only.** Everything below is written so Codex/agy can be added later (§14) without a rewrite, but nothing
for them is built now: no adapter registry, no typed-prompt fallback, no per-agent detection rules.

---

## 1. Core ideas

- **Agent library**: a local file of named heroes (model, flags, env); the UI generates one launch button per hero.
- **One-button launch**: click spawns a PTY running the agent CLI with its arguments and a one-line opener.
- **Briefing**: the opener points at a file assembled fresh per expedition from the issue.
- **Skills as buttons**: discovered skills become selectable spells (see `PLAN_SKILLS.md`).
- **Terminal**: Go PTY ↔ websocket ↔ xterm.js with scrollback replay and live status.
- **Shell**: a pure-Go server embeds the SPA; an optional cgo webview wraps it in a native window.
- **Release**: GoReleaser for portable binaries plus per-OS native packaging jobs.
- **Work items are GitHub issues**, and the theme is D&D.

---

## 2. Vocabulary (the theme)

| Dungeon term   | Meaning                                                                                                                                                                                                             | Source of truth                                                                            |
|----------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|--------------------------------------------------------------------------------------------|
| **Campaign**   | A working directory registered in the app, usually a git repo or a subfolder of one (monorepo). Issues come from the git root's remote; the agent runs in the registered folder. Optional label filter per campaign | local registry (state dir) + `git rev-parse --show-toplevel` / `git remote` → `owner/repo` |
| **Quest**      | A GitHub issue                                                                                                                                                                                                      | GitHub                                                                                     |
| **Dungeon**    | The visual graph of a campaign's quests (rooms = issues, doors/corridors = "blocked by"/sub-issue links)                                                                                                            | derived                                                                                    |
| **Room state** | Unexplored (open, unclaimed) · Locked (has open blocker) · Occupied (a session holds it) · Cleared (closed / PR merged) · Trapped (session failed / needs input)                                                    | derived from issue state, labels, blockers, linked PR, live sessions                       |
| **Hero**       | A registered agent: adapter + model + flags + env. One button each                                                                                                                                                  | `party.toml` (local, uncommitted)                                                          |
| **Party**      | The set of heroes                                                                                                                                                                                                   | `party.toml`                                                                               |
| **Expedition** | A live PTY session: one hero on one quest (or a free shell)                                                                                                                                                         | runtime                                                                                    |
| **Spell**      | A skill (`SKILL.md`)                                                                                                                                                                                                | skill sources                                                                              |
| **Spellbook**  | A registered skill source (folder or git repo)                                                                                                                                                                      | `books.toml`                                                                               |
| **Briefing**   | The generated file the hero is told to read first (issue body, comments, blockers, chosen spell, repo notes)                                                                                                        | generated per expedition                                                                   |
| **Torch**      | Optional, **off by default**: GitHub-side claim so teammates see an issue is being worked. Neutral wording (label `in-progress`, plain comment), never themed                                                       | GitHub                                                                                     |

Fits the user's role: *you* are the DM; heroes are the agents.

---

## 3. UI layout

```
┌──────────────┬───────────────────────────────┬───────────────────────────┐
│ CAMPAIGNS    │  EXPEDITION (terminal)        │  DUNGEON MAP   │ QUEST    │
│  repo A      │  xterm.js, tabs per session   │  rooms+corridors│ detail  │
│  repo B      │                               │                 │ (md)    │
│──────────────│                               │─────────────────┴─────────│
│ EXPEDITIONS  │                               │  SPELLS (skill selector)  │
│  ● #42 claude│                               │  [ grill ][ research ]…   │
│  ○ #17 sonnet│                               │  HEROES: [Opus][Sonnet]   │
│  ✋ #9 opus  │                               │          [▶ Summon]       │
└──────────────┴───────────────────────────────┴───────────────────────────┘
```

- **Left**: campaigns, and under the selected campaign its expeditions (status dot: working / idle / waiting for you).
  Selecting one attaches the centre terminal.
- **Centre**: terminal for the selected expedition (the "chat").
- **Right-top**: dungeon map and a quest detail pane (rendered issue markdown, comments, labels, blockers, linked PR).
  Clicking a room opens the detail.
- **Right-bottom**: spell selector + hero buttons. Pick quest → pick spell (optional) → click a hero → expedition
  starts.

Note: your message described the visual representation as both "left panel" and "right panel". The plan uses left =
lists, right = map/detail/skills. It is a layout choice, easy to flip.

Dungeon map ideas (still open): top-down room graph, auto-laid-out (layered DAG via `dagre`/`elkjs`, drawn on canvas or
SVG). Room glyph by label (`bug` = trap room, `feature` = treasure room, `research` = library, `epic`/parent = boss
room). Fog of war over rooms that are Locked. Milestones = floors. Keep the renderer behind a small interface (`mount`,
`setModel`, `onSelect`) so it can be swapped.

---

## 4. Architecture

Single Go module, one process, one server, one embedded SPA, one optional native shell.

```
cmd/dungeon/        pure-Go CLI: serves the embedded SPA on localhost
cmd/shell/          cgo webview wrapper (build tag `webview`), native window
internal/
  party/            hero registry, party.toml load/validate, PATH detection
  adapter/          launch spec: hero + opener → argv / typed-in text
  pty/              PTY spawn (unix + ConPTY), process groups, resize
  session/          expedition manager, scrollback ring buffer, activity detector
  github/           issues client, sync, cache, write-back (claim, comment)
  quests/           issue → quest model, room-state derivation, graph
  spells/           skill discovery, SKILL.md frontmatter parse, mirror
  briefing/         assemble briefing file
  server/           HTTP, control websocket (JSON snapshots), terminal websocket (binary)
  config/           paths, toml, settings
web/                Svelte + Vite + TS SPA (embedded via go:embed)
```

Key runtime pieces:

1. **Control socket** (JSON, server-authoritative): whole-state snapshot pushed on every change, resent on reconnect.
   Simple and robust; avoid client-side state merging.
2. **Terminal socket** (binary, per session): raw PTY bytes down, keystrokes up, scrollback replay on attach.
3. **Security** (do this from day one, it's a localhost server controlling shells): bind 127.0.0.1 only, random
   per-launch token, strict `Origin`/`Host` checks on HTTP + websockets, require `Content-Type: application/json` on
   writes, owner-only (0600/0700) state files. A webpage in your browser must never be able to drive your terminals.
4. **Webview shell** wraps the same server in a native window; the server also works in a plain browser
   (`dungeon serve`), which is the dev loop.

### Libraries (all permissive)

- Server: `net/http`, `github.com/coder/websocket`
- PTY: `github.com/creack/pty` (unix) or `github.com/aymanbagabas/go-pty` (unix + Windows ConPTY in one API)
- Terminal state/detection: `github.com/charmbracelet/x/vt` (headless VT emulator to read the screen for "waiting for
  input" detection)
- Config: `github.com/BurntSushi/toml`; file watching: `github.com/fsnotify/fsnotify`
- GitHub: `github.com/google/go-github` (REST) + a tiny GraphQL call for Projects v2 (or `github.com/shurcooL/githubv4`)
- Native window: `github.com/webview/webview_go` (cgo)
- Frontend: Svelte 5 + Vite + TypeScript, `@xterm/xterm` (+ fit/webgl addons), `marked` + `dompurify` for issue markdown
  (issue bodies are untrusted — always sanitize), optionally shadcn-svelte/Tailwind

---

## 5. Hero registry → auto-generated buttons (Claude only in v1)

v1 supports **Claude Code only**. Codex, agy and others get their own plan later (see §14). A hero is a named Claude
configuration; each one is a button.

`~/.config/dungeon/party.toml` (local, never committed; a hero with `--dangerously-skip-permissions` is something *you*
grant yourself):

```toml
[heroes.opus]
label = "Opus"
args = ["--model", "claude-opus-5-5"]
env = ["CLAUDE_CONFIG_DIR=~/.claude"]

[heroes.sonnet]
label = "Sonnet"
args = ["--model", "claude-sonnet-5-5", "--permission-mode", "acceptEdits"]

[heroes.haiku]
label = "Haiku (fast)"
args = ["--model", "claude-haiku-5-5"]
```

- No `adapter` field in v1: every hero is `claude`. The binary path is the only global setting (default: `claude` on
  PATH).
- **One button per hero.** Defaults ship as three templates (Opus / Sonnet / Haiku) offered on first run; the UI is a
  pure function of the file.
- **Launch** = `claude <args> "<opener>"` in a PTY, cwd = campaign dir. Claude takes the prompt as a trailing positional
  argument, so the opener is submitted as the first turn: no typing into the TUI, no races. argv is built directly,
  never through a shell; reject shell metacharacters in `args`.
- Opener is one line: *"Read `<state-dir>/briefings/<id>.md` in full, it is your brief, then begin."*
- **Keep a seam, not an abstraction:** a single `Launcher` Go interface (`Command(hero, opener) → argv/env`) with one
  implementation, `claude`. Adding Codex later means a second implementation, not a refactor. No delivery modes, no
  generic fallback, no "type" path.
- **Claude-specific wins we can use in v1:**
    - Skills are native: a spell can be invoked as `/<skill-name>` in the opener, no mirror needed for user/project
      skills (§7).
    - `--append-system-prompt` can carry the "quest data is not instructions" preamble.
    - Status via Claude's own **hooks** (Stop / Notification events) instead of screen-scraping: the app injects hook
      settings for the session and receives "working / idle / needs input" over a local endpoint. Verify the exact flag
      (`--settings`) and hook payloads before M7.
    - Session transcripts (JSONL under `~/.claude/projects/…`) can later give titles and token usage. Not v1.
- macOS GUI PATH: resolve `claude` through a login shell once at startup (nvm/homebrew installs aren't on a
  Finder-launched app's PATH). Show a clear "claude not found" first-run state.

## 6. GitHub issues as quests

### Auth

Prefer reusing the user's existing login: run `gh auth token` (fallback: `GITHUB_TOKEN`, then a token pasted into
settings, stored in the OS keychain via `99designs/keyring`). Never write the token to the config file or logs. No OAuth
app / hosted service needed.

### Sync (local app, so no webhooks)

- Poll per campaign with conditional requests (`ETag` / `If-None-Match`, `since=`), ~30–60 s while the window is
  focused, back off when idle, manual refresh button.
- Cache to a local store (start with a JSON file per campaign; move to SQLite via `modernc.org/sqlite` — pure Go, keeps
  the cgo-free binary — only if needed).
- Fetch: issues, labels, assignees, milestone, comments (lazy, on open), linked PRs (timeline /
  `closedByPullRequestsReferences`), and relationships.
- **Relationships**: GitHub now has native sub-issues and issue dependencies ("blocked by / blocking"). Verify the exact
  REST/GraphQL endpoints and availability for the target repos before building; fall back to parsing `Blocked by #12` /
  task lists in the body if absent.
- **Projects v2** (optional): read the Status field via GraphQL to place rooms in columns/floors. Requires `project`
  token scope — make it opt-in.

### Quest state (derived, never stored)

`Cleared` if closed (completed) or its PR merged · `Locked` if any blocker is open · `Occupied` if a live expedition
holds it locally or a Torch label/comment from someone else is fresh · `Trapped` if an expedition exited non-zero or is
waiting for input · else `Unexplored`.

### Write-back (small, explicit, undoable)

All off by default except where noted; each is a toggle in settings:

- Torch (**off by default**): add a neutral label (`in-progress`) and a plain comment when an expedition starts; remove
  on end. Never mention the app or use themed text in anything written to GitHub.
- Post the agent's final summary as an issue comment — **always a user-confirmed button**, never automatic.
- Open PR / link PR: leave to the agent (it has `gh`), just detect the result on next sync.
- Never close issues from the app; closing follows PR merge.

Errors to design for: rate limits (show remaining quota), expired token, repos with no remote, forks (issues live on
upstream), private repos, SSO-protected orgs.

---

## 7. Skills → spells → buttons

> Detailed skills plan: `PLAN_SKILLS.md` (supersedes the M5 outline in §11).

Discovery (read-only scan, re-scanned on file change):

1. `<campaign>/.claude/skills/*/SKILL.md`, `<campaign>/.agents/skills/*`
2. `~/.claude/skills/*`, installed plugin skill dirs
3. Extra spellbooks registered by the user: local folder or git URL (cloned/pinned under
   `~/.local/state/dungeon/books/`, refreshed on demand)

Each `SKILL.md` frontmatter (`name`, `description`) becomes a **spell** card. Resolution order = precedence: campaign →
user → books; `book/spell` addresses one exactly.

Using a spell:

- In the right-bottom panel, selecting a spell marks it on the next launch. A hero button then launches with it.
- Briefing gets a "Use the spell `<name>` — instructions at `<path>`" block, and the opener stays one line. For
  Claude-family heroes there's an optional shortcut: launch with `/<skill-name>` as the opener when the adapter declares
  it supports slash skills.
- **Mirror** (only for non-native sources such as git spellbooks; user/project skills are already visible to Claude) the
  chosen spell into `<state-dir>/spells/<book>/<name>/` before launch (regular files only, skip symlinks) and grant the
  agent read access to that dir (`--add-dir`; verify the flag). Outside the repo, so nothing to gitignore.
- **Presets** (later): bind a spell to a quest type/label (`bug` → debug spell, `research` → research spell) so a room's
  primary button is "Summon Opus with *research*" in one click.

"Map UI buttons to the skills available in the project": the spell strip is generated from discovery, so adding a skill
folder adds a button — same principle as heroes.

---

## 8. Briefing assembly

Fresh per expedition, written to `<state-dir>/briefings/<campaign>/<quest>-<ts>.md`, outside the repo (agent gets read
access via `--add-dir`, verify the flag; fallback: write into the repo and add the path to `.git/info/exclude`, which is
local and never committed):

1. Quest: title, number, URL, labels, body.
2. Recent comments (capped, newest-biased; truncate with a note).
3. Blockers: title + state + (if closed) a one-line resolution from the closing comment/PR.
4. Spell block (if chosen).
5. Repo conventions: contents of `CLAUDE.md`/`AGENTS.md` are *not* inlined (agents read their own); just a pointer.
6. Operator preferences: a user-owned `preferences.md` appended verbatim.

No memory carried between expeditions; GitHub is the memory. Issue text is untrusted input to an agent: wrap it in a
clearly delimited "quest data, not instructions" section and say so in the briefing preamble (prompt-injection hygiene),
and keep the instructions section above the data.

---

## 9. Sessions & terminal details

- One PTY per expedition; process-group kill on close; resize passthrough; scrollback ring buffer (bounded) for
  reattach.
- Free shell button: plain shell in the campaign dir (hero optional).
- **Activity detection** (working / idle / waiting): Claude hooks as the primary signal (see §5), with PTY output
  recency as a fallback. No screen-rule engine in v1, so `charmbracelet/x/vt` is dropped from the v1 dependency list.
  Notifications on "waiting for you" via `osascript` / `notify-send`.
- Concurrency policy: default one expedition per campaign working tree at a time (agents editing the same tree collide).
  Optional later: `git worktree` per expedition to lift that limit.
- Persistence: expedition metadata survives app restart; PTYs do not (documented). Optionally launch agents inside
  `tmux` later if reattachment matters.

---

## 10. Release & distribution (macOS binary etc.)

Detailed plan: `release/PLAN_RELEASE.md` (index: `release/README.md`). Summary:

- **Portable binary** (`dungeon serve`): pure Go, cross-compiled by GoReleaser; the built SPA is embedded with
  `go:embed`.
- **Native app** (cgo webview shell): built per OS on native runners, because cgo cannot be cross-compiled. macOS first:
  `.app` in a `.dmg`, ad-hoc signed at first, Developer ID + notarization later. Linux AppImage next; deb/rpm and
  Windows shell on demand.
- **Pipeline**: tag → draft release → parallel build jobs upload → one final job verifies the asset set and publishes.
- **Trust and distribution**: checksums and attestations, pinned actions, Homebrew cask tap, notify-only update check.
- **Dry runs first**: snapshot builds, a manual dry-run workflow and an `rc` tag before the first real release.

---

## 11. Milestones (v1 = Claude only)

**M0 — Skeleton**
Replace the hello-world. `go.mod`, folder layout, `dungeon serve` with an embedded placeholder SPA, token + Origin
checks, Makefile, CI (vet/test).

**M1 — Hero buttons & terminal (core loop)**
`party.toml` load/validate, `claude` lookup, `Launcher`, PTY manager, terminal websocket + xterm.js, hero buttons
spawning in a chosen directory, free shell, scrollback, resize, close. *Exit: click Opus and Sonnet, get two live
independent Claude sessions.*

**M2 — Campaigns & left panel**
(Local registry only. Resolve git root vs working dir; per-campaign optional label filter stored locally, which also
covers monorepo subfolders.)
Register a repo (path → `owner/repo`), campaign list, expedition list with status dots, attach/detach, persisted
metadata.

**M3 — GitHub quests** (done)
Built: a `Source` interface with a GitHub GraphQL client (one request per page returns labels, assignees, parent,
blockers and linked PRs; comments only when an issue is opened) and a demo source for exploring without GitHub;
credentials from `GH_TOKEN`, `GITHUB_TOKEN` or `gh auth token`; a 60s cache with throttled refresh, shared in-flight
fetches and a stale fallback; derived state (open, in progress, locked, cleared); a right-hand panel with list, nesting,
filter, counts and a detail view with sanitised markdown. Differences from the first sketch: GraphQL instead of REST +
ETags, and the demo source is a per-campaign toggle persisted locally. Not built: Projects v2 status, the optional
in-progress label, write-back.

**M4 — Briefing + launch from a quest**
Briefing assembler, "Summon hero on quest", one-expedition-per-tree gate, optional Torch label/comment (off by default).

**M5 — Spells**
Discovery from `.claude/skills` + `~/.claude/skills` (+ plugin dirs), frontmatter parse, spell strip, `/skill` opener,
label→spell presets. Git spellbooks and mirroring only if needed.

**M6 — Dungeon map**
Graph layout + renderer, glyphs by label, fog for locked rooms, click-to-select, live state, milestone floors.

**M7 — Hooks status & notifications**
Claude hooks → working/idle/needs-input, native notifications.

**M8 — Packaging & release**
Webview shell, macOS .app/.dmg, GoReleaser, draft→publish release workflow, Linux AppImage, first alpha tag.

M1 → M4 is the first usable version; M6 can slip.

---

## 12. Risks / things to verify early

- **Claude CLI flags change.** Keep hero `args` opaque and user-editable; ship templates as docs. Verify `--model` IDs,
  `--permission-mode` and hook settings against the installed version.
- **GitHub relationships API** (sub-issues / dependencies): confirm endpoints and availability on your plan/repos in M3
  before designing the map around them.
- **GUI PATH on macOS**: launched from Finder, the app lacks your shell PATH → resolve via a login shell
  (`$SHELL -lic 'echo $PATH'`) once at startup, or agents installed via nvm/asdf/homebrew won't be found.
- **Localhost control surface** is a real attack surface (see §4.3) — treat as a release blocker, not polish.
- **Untrusted issue content → agent prompt**: mitigated in the briefing format, but remember agents often run with
  permissions skipped.
- **cgo shell** can't be cross-built; every OS needs its own runner. Keep the browser mode a first-class path so
  releases aren't blocked by shell bugs.
- **Scope**: keep it small: no transcript parsers, no prompts catalog, no auto-titling in v1.

---

## 13. Decisions I made that you may want to change

1. Name/module: `dungeon` (matches the directory).
2. Layout: lists left, terminal centre, map + quest detail + spells/heroes right.
3. Auth via `gh auth token` — no OAuth app.
4. Polling, not webhooks; JSON cache first, SQLite later.
5. v1 is Claude-only; hero = a Claude config (model + flags), no adapter field yet.
6. Spells are discovered from standard skill dirs automatically (no manual registration needed for the common case).
7. macOS: unsigned + ad-hoc, arm64 only, for the first alpha.
8. Write-back to GitHub is off by default; the optional Torch label/comment and the user-confirmed summary comment use
   neutral wording.
9. Everything local; nothing committed to the repo (see design principle).

---

## 14. Later plan (not in v1): more agents

Broader parking lot: see `PLAN_V2.md`.

Separate plan once v1 is stable. Known work, so v1 doesn't paint us into a corner:

- Add `adapter` to `party.toml` and a registry of `Launcher`s (claude, codex, generic).
- Prompt delivery modes: argv / flag / typed-into-TUI fallback (needed for agy until its CLI is checked).
- Per-agent status detection (hooks where they exist, otherwise OSC titles / screen rules via a VT emulator).
- Skill delivery for agents without native skills: mirror into the repo + path in the briefing.
- First-run PATH scan for `codex`, `agy`, `gemini`, `opencode`.
