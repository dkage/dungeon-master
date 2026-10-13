---
status: current
milestone: all
github_issues: [28, 72, 63, 78, 43, 45, 67, 68, 61, 69, 70, 71, 44, 58, 35, 36, 37, 38, 39]
tracking: in-gh-issues
next: review and publish the committed map increment, then workspace focus and source-backed skill launch
verified: 2026-10-13
---

# Roadmap

GitHub's open issues own remaining scope and native relationships. This file owns priority/order; order alone is not
a blocker link. Design docs specify behavior, not completion. Status must distinguish local implementation, passing
checks, committed/published code and a closed milestone. Keep existing issue numbers through replanning.

## Current-state audit (2026-10-13)

- M0 through M4.5 are delivered; #7, #19 and #23 and their implementation children are closed. M4.5's old
  "closes when pushed" status was stale.
- M5a (#28) is open. Its implementation, assets, shared model/layout, effort APIs and tests are committed locally
  across five commits: a1676ae, 4f5b6f9, 7a25fbd, 01a461a and 808573b. There are no remaining implementation changes
  in the working tree; the roadmap/design/skill reset is maintained separately from those implementation commits.
- At the implementation audit, after fetching origin and before committing the planning reset, `master` was five
  commits ahead of `origin/master` (0 behind). The implementation is committed
  but unpublished. This supersedes the earlier same-day observation that the branches matched before these commits.
- `make check` passed on macOS against the local tree: Go vet/race tests, frontend check (zero diagnostics), and
  322 frontend tests. All 33 Playwright browser tests passed, including seven map scenarios. These are stub/fake-based
  checks, not proof of real-CLI, real-GitHub or other-platform behavior.
- Current session records survive restart; PTYs do not. Source registration, configurable shortcuts and panel
  expansion are planned, not shipped. Do not interpret a design spec or new issue label as implemented behavior.

## Delivered foundations

| Milestone | Delivered foundation | Tracking |
|---|---|---|
| M0–M2 | Local server/guard, Claude PTYs, hero configuration, registered campaigns and session history | `CLAUDE.md` |
| M3 | GitHub/demo issues, detail, sanitization and derived state | `docs/architecture.md` |
| M3.5 | Request budgeting, gh pacing and query scope | #41/#42 remain conditional extensions |
| M3.6 | Cockpit shell, first run, words and appearance | closed #7 |
| M4 | Reviewed briefing, issue/session linkage, one active issue session per tree | closed #19 |
| M4.5 | Waiting heuristic, list/title/favicon attention, optional browser notifications | closed #23 |

## Delivery order

| Order | Increment | Status | Issues | Design |
|---|---|---|---|---|
| 1 | M5a: review and publish the map/list implementation | five local commits; checks passed; milestone open | #28 (#24–#27, #59, #60, #62) | `docs/design/map.md` |
| 2 | Dungeon visual baseline: parchment terminal, slate panels, framed detail/list/headers | planned | #72 (#73–#77) | `docs/design/art-direction.md` |
| 3 | Workspace focus: resizable columns, expand/restore panels and configurable shortcuts | planned | #63 (#78, #64, #65) | `docs/design/workspace.md` |
| 4 | M5b: registered skill sources mapped to projects, verified method delivery and picker | planned | #43 (#51, #49, #66, #50) | `docs/design/skills.md` |
| 5 | M7: trustworthy session attention | probe before implementation | #48 → #45; #52 provides real-CLI fixtures | `docs/design/experience.md` |
| 6 | Ready work and planning before issues | planned | #67, #68; #61 publishes shaped issues | `docs/design/campaign-loop.md` |
| 7 | Session ownership and continuity | investigation, then implementation | #69 → #70 | `docs/design/sessions.md` |
| 8 | Outcome review and continuation | planned | #71; #47 verifies killed outcomes on Windows | `docs/design/campaign-loop.md` |
| 9 | M8: verified portable release and desktop packaging | planned; lifecycle decision precedes shell wiring | #35–#38, #36/#57 | `release/PLAN_RELEASE.md` |
| Later | M6: additional map presentation | deferred until the full workflow is usable | #44 | `docs/design/map.md` |

#69 can run alongside the source/attention work: decide ownership before implementing desktop-window shutdown.
#72's children have no prerequisites; #76 restyles the panel headers that #64 later extends, so keep room for its
controls. #77 settles the reference's Clear/Reload buttons before anyone builds them.
Finish the existing visual baseline (#72); new prototypes test expanded planning, terminal and reading at ordinary
window sizes. Keep one workspace model, existing art and the RPG/Neutral presentations. No generic pane manager, plugin
platform, alternate tracker, native-framework rewrite or provider registry is required by this reset.

## Conditional and platform work

| Work | Issues | Trigger or prerequisite |
|---|---|---|
| Native desktop notifications | #58 (#53–#57) | shell focus/click-through and lifecycle decision; consume hooks when verified |
| Real Windows CLI and killed-session outcome | #32, #47 | native Windows environment with Claude |
| Rate-limit/shim behavior against real services | #33, #34 | small controlled probes; never provoke a secondary limit |
| Local issue database | #41 | demonstrated offline or multi-campaign need |
| Durable paced write queue | #42 | app-managed bulk publishing requires resumable operations |
| Local issue pins | #40 | needed scope without changing GitHub labels |
| Documentation/contract checks | #46 | a short bounded checker; includes unchanged legacy label/body migration |
| Further distribution | #39 | actual users/platform demand after initial release |
| Another agent provider | `docs/ideas.md` | validate the core loop first; integrate one provider when demand justifies it |

## Reading and updating

Read `CLAUDE.md`, this file, the current issue and its design doc. Update status with dated evidence when checks run
or work is committed/published. Close issues only after their exit criteria and repository delivery procedure are met.
Do not write "pending push" unless verified commits actually exist ahead of the intended remote.

## New work: verified issue relationships

Snapshot verified 2026-10-13. These are planned issues, not delivered functionality.

| Issue | Work | Effort | Method | Parent | Blocked by |
|---|---|---|---|---|---|
| [#63](https://github.com/dkage/dungeon-master/issues/63) | Make workspace panels expandable and shortcuts configurable | high | task | — | — |
| [#64](https://github.com/dkage/dungeon-master/issues/64) | Expand and restore individual workspace panels | moderate | task | #63 | — |
| [#65](https://github.com/dkage/dungeon-master/issues/65) | Configure workspace shortcuts in Settings | moderate | task | #63 | #64 |
| [#78](https://github.com/dkage/dungeon-master/issues/78) | Resize the three workspace panels by dragging their borders | moderate | task | #63 | — |
| [#66](https://github.com/dkage/dungeon-master/issues/66) | Verify and preserve the selected skill invocation contract | high | task | #43 | #49 |
| [#67](https://github.com/dkage/dungeon-master/issues/67) | Show ready work and waiting sessions with trustworthy dependency reasons | high | task | — | — |
| [#68](https://github.com/dkage/dungeon-master/issues/68) | Start a planning session before publishing issues | moderate | task | — | #61, #50 |
| [#69](https://github.com/dkage/dungeon-master/issues/69) | Investigate session ownership, detach and recovery | moderate | research | — | — |
| [#70](https://github.com/dkage/dungeon-master/issues/70) | Preserve sessions across window closure and support bounded recovery | high | task | — | #69 |
| [#71](https://github.com/dkage/dungeon-master/issues/71) | Review session outcomes and continue issue work | moderate | task | — | #50 |
| [#72](https://github.com/dkage/dungeon-master/issues/72) | Finish the Dungeon visual baseline from the reference | high | task | — | — |
| [#73](https://github.com/dkage/dungeon-master/issues/73) | Show the parchment texture behind the terminal | moderate | task | #72 | — |
| [#74](https://github.com/dkage/dungeon-master/issues/74) | Use the slate texture behind the side panels | low | task | #72 | — |
| [#75](https://github.com/dkage/dungeon-master/issues/75) | Frame the issue detail and list rows like the reference | moderate | task | #72 | — |
| [#76](https://github.com/dkage/dungeon-master/issues/76) | Frame the panel headers and project panel actions | low | task | #72 | — |
| [#77](https://github.com/dkage/dungeon-master/issues/77) | Decide on Clear and Reload actions for the terminal | low | decision | #72 | — |

The reset revised 16 existing issues, added nine bounded issues, and verified 26 native relationship repairs/additions.
Affected issues have effort/method labels and concrete scope/evidence. Unchanged legacy items still need label/body
normalization when taken up or audited in #46; avoid cosmetic bulk writes during roadmap changes.
