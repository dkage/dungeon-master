# Dungeon Master

A desktop cockpit for a human "Dungeon Master" who launches CLI coding agents against GitHub issues. One button starts
an agent CLI in a PTY with its model, flags and context already set. Go server with an embedded SPA, optional native
webview shell.

Status: M0-M4.5 done. M3.5 (GitHub budget, scope and the `gh` shim) is done except the SQLite copy and write queue
(issues #41, #42). M3.6 (Stitch v4 cockpit shell, first run and words) and M4 (quest briefing, launch, session linkage)
are done, and so is M4.5 (attention: waiting sessions in the list, tab title and icon, and optional
desktop notifications). Next: M5a map (#28). Everything unbuilt is a GitHub issue; the open list is the
source of truth for what is left, including Windows and release work. Themed words: `GLOSSARY.md`.

Design for work not yet built lives in `docs/` (`architecture.md`, `ideas.md`, `design/`), not in the repo root. Each of
those files starts with front-matter properties (`status`, `milestone`, `github_issues`, `tracking`, `next`,
`verified`); update them when its state changes. When a milestone closes, delete its design doc (git keeps the history).
The history of the project's design references and visual assets lives in
[`design/index.md`](design/index.md). It lists each design folder and the
commit dates when its asset batches were first recorded. The latest and most
up-to-date frontend prototype is the Google Stitch export in
`design/stitch-prototype/`.

Creature map guidance: `docs/design/creature-roster.md` and `docs/design/art-direction.md`; source sprites and the
historical right-panel study remain in `design/quest-encounters/`. Current map assets are indexed in
`design/map-art/README.md` and tracked in `docs/design/art-checklist.md`.

## Scope and principles

- **v1 is Claude Code only.** Keep one `Launcher` seam; do not build an adapter registry, typed-prompt fallback or
  per-agent detection until the later multi-agent plan.
- **Invisible to the team.** All config is local (user state dir). No committed config files and no `.dungeon/` folder
  in user repos. The RPG theme exists only in the app UI; anything written to git or GitHub uses neutral wording. GitHub
  write-back is opt-in, off by default.
- **State files** (`campaigns.json`, `sessions.json`, `party.toml`) live in the user config dir (`DUNGEON_HOME`
  overrides it), written atomically with 0600/0700 permissions (protected user ACLs on Windows). Derived data (git root, owner/repo) is recomputed on
  load, never persisted.
- **Execution config stays local.** Heroes (model, flags, env, binary path) live in the user's `party.toml`, never in
  anything a repo can supply.
- **GitHub credentials** come from `GH_TOKEN`, `GITHUB_TOKEN` or `gh auth token`; they are used for GitHub requests only
  and never stored, logged, put in an error or sent to the browser. `DUNGEON_GITHUB_ENDPOINT` (tests only) accepts
  loopback URLs and nothing else.
- **Never get a user locked out of GitHub.** All of the app's GitHub requests go through the Quartermaster's transport
  (serial, budget read from headers, camp on a limit). Agents run `gh` themselves, so every session the app starts has a
  shim named `gh` first on its PATH (a link to this binary, `internal/gatekeeper`): reads pass straight through, writes
  (`gh issue create`, `gh api -X POST`, GraphQL mutations, ...) wait for a paced slot (3 s apart, 15 a minute, 150 an
  hour) and are refused while GitHub has asked us to slow down. It fails open: an unreachable app or an unrecognised
  command never blocks work. Never loop `gh issue create`; one write at a time.
- **Scope is part of the GitHub query**, never a filter applied to a truncated list. A campaign has no scope until the
  user chooses one (the first-connection question); `tracked` is the neutral marker label the picker suggests.
- **Issue text is untrusted.** Markdown is cut to a bare-tag allowlist, then sanitised; never pass anything but
  `renderMarkdown` output to `{@html}`.
- **Localhost is an attack surface.** Loopback bind only, an access token kept in the config dir (0600) so a signed-in
  browser survives restarts, rotated with `dungeon serve --new-token`; strict Host/Origin checks, JSON-only writes.
  Tests must never touch the real config dir (`cmd/dungeon` has a `TestMain` that redirects `DUNGEON_HOME`). Treat any
  weakening of `internal/server/guard.go` as a release blocker. The CSP deliberately allows inline *styles* (xterm.js
  builds its font, colour and selection rules as `<style>` elements; blocking them breaks the terminal) but never inline
  or foreign scripts; both are covered by tests. Issue text is untrusted input to agents.
- **Own as little as possible for skills.** Use Matt Pocock's skills (MIT) as a pinned external source; fork only on a
  real gap, keeping his notice and credits.
- **Independent design.** Write from the behaviour in the plans; do not copy code, prose or assets from other projects.
  `private/` is gitignored local notes: never commit it, never quote from it in tracked files.

## Vocabulary (UI only)

Campaign = registered working directory (issues come from the git root's remote). Quest = GitHub issue. Hero = a
registered Claude configuration. Spell = skill. Expedition = live session. Briefing = generated context file. Dungeon
= an epic (a top-level issue with sub-issues) on the map; Tavern = quests that belong to no epic. Keep these terms out of GitHub-facing text and out of Go identifiers where a
neutral name is clearer.

## Layout

```
cmd/dungeon/        CLI entry (serve, version)
internal/server/    HTTP, guard (token, Host, Origin, content-type), static assets
internal/session/   PTY sessions (scrollback, subscribers, process-group kill)
internal/party/     party.toml loading and validation
internal/campaign/  registry of working directories; git root and owner/repo resolution
internal/history/   session records that survive restarts
internal/fsutil/    atomic private file writes
internal/quartermaster/ gatekeeper for ALL GitHub requests (an http.RoundTripper): serial, reads the budget from headers, pauses ("camp") when GitHub says slow down. New GitHub code must use its transport
internal/gatekeeper/ the gh shim for agent sessions: classifies gh commands, asks the app for write slots, passes everything else through
internal/quests/    quest model, derived state, cached service; github/ (GraphQL client, token from env or `gh`) and demo/ (sample data) are its sources
internal/launcher/  claude argv, env scrubbing, binary lookup
internal/testutil/  shared fixtures (fake claude)
web/                Svelte 5 + Vite + TypeScript frontend, embedded via go:embed
web/dist/           generated by `make web`; only .gitkeep is tracked
web/e2e/            Playwright browser tests (drive the real binary)
.github/workflows/  CI and release workflows (thin; call scripts in release/)
docs/               architecture.md, ideas.md and design/ (plans for work not yet built; delete a design doc when its
                    milestone closes and its work is in issues)
release/            everything about shipping: plan, scripts, packaging (index: release/README.md)
```

Still to come (see `docs/architecture.md` §4): `spells`.

## Commands

```
make check     # go vet + go test -race + frontend type check and unit tests (what CI runs)
make build     # build the frontend, then bin/dungeon (CGO_ENABLED=0)
make run       # build and serve on 127.0.0.1:7878 (macOS/Linux)
.\run.ps1     # build and serve in Windows PowerShell
make web       # build the frontend into web/dist
make test-linux # Go tests in a Linux container (needs Docker); PTY behaviour differs from macOS
make test-e2e  # browser tests against bin/dungeon (needs `npx playwright install chromium` once)
```

`go build` and `go test` must work on a fresh clone with no frontend toolchain (the server falls back to a built-in
page). Frontend dev: run `dungeon serve`, open its URL once (sets the cookie), then `cd web && npm run dev` and browse
http://127.0.0.1:5173 (the Vite proxy forwards /api and websockets). The future native webview shell needs cgo and is
built separately, never by the portable build.

## Conventions

- Go 1.26, stdlib first. Dependencies must be permissive-licensed and justified. Run `gofmt`; keep `go vet ./...` clean.
- **Test everything, at two levels.** Every change ships with unit tests (pure logic, parsing, validation) and feature
  tests that drive the real behaviour end to end: HTTP + websocket against `httptest`, a real PTY running a stub
  program, the frontend through its public UI. No feature is done without both. Security-relevant behaviour (guard, path
  confinement, argv building, env scrubbing) always has tests, including the hostile cases.
- Tests live beside the code (`*_test.go`); shared fixtures and a fake `claude` stub go in `internal/testutil`. Never
  call the real `claude`, `gh` or the network in automated tests; real-CLI checks are manual probes in the gitignored
  `_probe/`.
- Never build a command line through a shell; exec argv directly.
- **GitHub issues have a shape.** Every issue you create uses the `issue-shape` skill (`.claude/skills/issue-shape`;
  contract in `docs/design/issue-shape.md`): an effort label, sub-issues for epics, blocked-by links in order, neutral
  wording.
- Module path is `github.com/dkage/dungeon-master`. The product name "Dungeon Master" is provisional: it appears in
  prose and UI text only, while the binary, config directory and `DUNGEON_HOME` keep the short `dungeon` name so a later
  rename stays cheap.
- Default branch is `master`.

## Git commits

Conventional Commits. The first commit is the model:

```
docs: add initial project plans

Dungeon Master is a desktop cockpit for launching CLI coding agents against
GitHub issues. Nothing is implemented yet; this commit records the
design so the work can start from a shared reference.

- PLAN.md: v1 scope (Claude Code only), architecture, ...
- PLAN_SKILLS.md: how skills map to quests and launch buttons.
```

- Subject: `type: imperative lowercase summary`, no trailing period, about 50 characters (72 hard limit). Optional
  scope: `feat(server): ...`.
- Types: `feat`, `fix`, `docs`, `chore` (scaffolding, deps), `build` (packaging, Makefile, release), `ci`, `test`,
  `refactor`, `perf`.
- Blank line, then a body wrapped at 72 columns. Say what the change is and why; the diff already says how. Use
  `- file/area: note` bullets for multi-part commits.
- Breaking changes: `!` after the type and a `BREAKING CHANGE:` footer.
- One logical change per commit. Commit only when asked. When asked to commit finished work, commit each step
  separately (for example the package, then the server wiring, then docs), not one commit for everything.
- **Always name the GitHub issue (the card).** Put it in the footer, after the body and a blank line:
  `Closes #N` when the commit finishes the issue, `Refs #N` otherwise (one footer line per issue). Keep issue
  numbers out of the subject line. For big work, add a short summary of what changed to the body (bullets per
  area), still before the footer.
- **Several agent sessions share this working tree**, so other sessions' uncommitted work is usually present. Never
  `git add -A`, `git add .` or `git commit -a`. Stage and commit only the paths you changed (`git add <paths>` then
  `git commit <paths>`), run `git status --short` and `git diff --cached --stat` first, and
  unstage anything that is not yours. If a file you need to commit also holds someone else's edits, stop and ask.
- **No attribution of any kind**: no `Co-Authored-By`, no `Claude-Session:` trailer, no "Generated with" line, in
  commits or PR bodies.
- Do not amend or rewrite commits that have been pushed.
- Releases are automated from these messages (release-please, see `release/PLAN_RELEASE.md` §3): the type decides the
  version bump and the changelog entry. PRs are squash-merged with a Conventional Commit PR title. Never hand-edit
  `CHANGELOG.md`, the release manifest or version numbers.

## Milestones

The list, order, status and epic of every milestone are in `docs/roadmap.md`. Update that file when a milestone closes.

## Verifications

Done:

- A user-invoked skill runs from a positional prompt (`claude "/skill args"`) in the interactive TUI. (M1)
- `--add-dir` and `--append-system-prompt` work as launch flags; the Gatekeeper rule uses the latter. (M1, M3.5)
- GitHub sub-issues, blocked-by edges, assignees and linked PRs come from one GraphQL query on the user's repos. (M3)

Still open:

- Hook settings injection (`--settings`) for session state. (M7)
- Camp and secondary-limit refusal against the real GitHub; only the fake has exercised it. (M3.5)
- Windows runtime fixes (executable extensions, `gh.exe` copies, bounded socket paths, native shells, private state
  ACLs and ConPTY completion) are covered by native Windows tests; CI is configured for all three supported platforms.
  Linux runtime verification via WSL is pending its installation reboot.
