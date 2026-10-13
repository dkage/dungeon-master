---
status: current
milestone: delivered foundations and planned extensions
github_issues: [28, 43, 63, 67, 68, 69, 70, 71, 79]
tracking: in-gh-issues
next: review/publish the committed map; use the roadmap for order and design docs for extensions
verified: 2026-10-13
---

# Dungeon Master architecture

A local cockpit for steering Claude Code against GitHub issues, with an RPG presentation and a neutral alternative.
The operator plans, chooses ready work, reviews launch context, steers a live terminal and inspects results. Current
scope and rules are in `CLAUDE.md`; priority and evidence of delivery are in `docs/roadmap.md`.

## 1. Product boundary

GitHub owns issue identities, assignments, parent/dependency links and closure. The app owns local campaign registration,
execution settings, sessions and presentation. No mandatory project config, file tracker, automatic queue or scoring.
The theme stays in the UI; shared artifacts use neutral language. GitHub writes are explicit and paced.

## 2. Local state and vocabulary

Campaign = registered directory; quest = GitHub issue; hero = Claude configuration; spell = selected skill/method;
expedition = session; briefing = private launch context. Dungeon = top-level parent, Tavern = issues outside parents.
Definitions and UI wording come from `GLOSSARY.md` and `web/src/lib/words.ts`.

`party.toml`, `campaigns.json`, `sessions.json`, token and briefings live in the user config/state directory, with atomic
private writes. Derived git roots/remotes are recalculated. Browser UI preferences currently live in localStorage;
validated shortcut overrides and per-project skill-source mappings are planned additions, not current state files.

## 3. Workspace

The current shell has projects/sessions on the left, the selected terminal in the centre, and map/list plus issue detail
on the right. The local M5a implementation shares a view-model between list and map. Panel expansion and shortcuts are
specified in `docs/design/workspace.md`: one state model, one expanded region, restoration without terminal remounts.
Layout never owns or changes process identity. Existing prototypes supply appearance; design docs specify behavior.

## 4. Runtime packages

| Package | Responsibility |
|---|---|
| `cmd/dungeon` | Serve/version entry, configuration wiring, gh-shim dispatch |
| `internal/server` | Authenticated HTTP/terminal websocket, APIs and embedded frontend |
| `internal/session` | PTYs, scrollback/subscribers, activity heuristic and process cleanup |
| `internal/history` | Persistent session metadata; lost records after backend restart |
| `internal/party`, `internal/launcher` | Validated local execution definitions and direct argv/environment |
| `internal/campaign`, `internal/fsutil` | Directory registry, local settings and atomic private writes |
| `internal/quests` | Source/service cache, GitHub/demo data and derived issue facts |
| `internal/briefing` | Bounded issue context and reviewed private launch artifacts |
| `internal/quartermaster`, `internal/gatekeeper` | GitHub transport budget and agent gh-write pacing |
| `internal/platform` | Cross-platform filesystem/path/process conventions |
| `web` | Svelte 5/TypeScript, xterm.js, shared issue views and local UI state |

Planned skill discovery/source registration belongs behind a small `spells` seam. The optional `cmd/shell` native
webview is release work; persistent-process ownership is an investigation before changing session architecture.

## 5. Agent launch

v1 is Claude Code only. A named hero sets local args/env; Launcher builds direct argv, never an interpolated shell
command. GitHub issue prose cannot supply executable settings. A quest launch prepares a private reviewed briefing,
checks issue availability and serializes the active-issue-session-per-working-tree check/spawn. Free sessions remain
available; the quest gate is not a sandbox against unrelated processes editing that directory.

## 6. GitHub and derived state

Credentials come from environment or gh's credential store and never reach browser/logs. Requests use the Quartermaster.
The source queries scope server-side, returns native relations, and caches with stale fallback; comments load on detail.
Local SQLite (#41), write queue (#42), local pins (#40) and Projects v2 are conditional extensions.

Current state derivation: closed issue → cleared; open blocker → locked; assignee → claimed; otherwise open. Local
session activity adds running/needs-input presentation independently. #67 will distinguish completed/canceled and
unknown prerequisites, preserve repository-qualified identity, validate cycles, and recheck launch readiness. Do not
infer issue completion from session exit or PR presence. Method and effort are separate from state.

## 7. Skills

`docs/design/skills.md` is the contract. Planned sources are registered local folders or pinned Git repositories,
mapped/enabled per campaign. Optional project/user/plugin discovery supplements them. Source-qualified selection and
explicit precedence are independent of verified Claude invocation; external caching does not establish slash-command
registration. Skill launches retain the briefing and supporting-file context. No repository mirrors or setup files.

## 8. Briefings and review

Generated context stays outside the working repository. Include selected issue identity/body, bounded comments and
relations, pointers to repository instructions, and selected method/source plus operator direction when implemented.
Issue content is untrusted data. Preview states shortening and unavailable facts; launch validates the chosen source
and work facts rather than silently changing the reviewed plan. Source/readiness freshness improvements are planned
in #66/#67; the current preview stores the issue briefing and launch uses that artifact.

## 9. Sessions

The server currently owns PTYs; browser detach leaves them running, server Shutdown kills them, and metadata survives
without a live terminal. #69 settles ownership and #70 implements continuity. Reattachment, restart recovery and
provider conversation resume are separate contracts. #48 probes hook settings before #45 replaces heuristic attention.

## 10. Security and verification

Retain loopback-only bind, persistent private token, strict Host/Origin checks and JSON-only writes. Sanitize issue
Markdown before HTML rendering. Keep local execution settings out of repository-supplied content; bound skill scanning.
All behavioral changes require unit and real-behavior feature tests. Automated checks use stubs/fakes, never live Claude,
gh or network. Record dated manual probes separately; a test file is not evidence that its check ran.

## 11. Delivery order

`docs/roadmap.md` owns order and current status. Existing M-numbers identify history; new workspace/workflow increments
do not renumber existing issues. Validate local map work before declaring the map milestone delivered.

## 12. Open verification boundaries

Hook injection, real limit recovery/refusal, real agent shim behavior and Windows/Linux runtime probes remain tracked.
Session recovery, native menus, skill delivery and shortcuts require platform-specific checks. Packaging follows
`release/PLAN_RELEASE.md`; the browser path remains supported while native shells are built.

## 13. Stable decisions

Keep Go/Svelte, the existing visual assets, Claude-only launch seam, local configuration and GitHub work model. Add
workspace focus and source-backed methods without a generic plugin platform or arbitrary pane tree. The product name
remains provisional; binary/config names remain `dungeon`.

## 14. Later capabilities

`docs/ideas.md` holds unpromoted ideas. Another provider, alternative trackers, richer pane layouts, transcript/cost
parsing and theme galleries require demonstrated demand and separate bounded plans. Separate working copies for
parallel issue sessions were promoted to #79.
