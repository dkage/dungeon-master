---
status: current
milestone: all
github_issues: [45, 48, 52, 61, 67, 68, 69, 70, 71, 35, 36, 37, 38, 57, 58, 151, 44]
tracking: in-gh-issues
---

# Roadmap

GitHub owns status: which issues are open, closed or blocked, and the native parent and blocker links. This file owns
only what GitHub cannot say well: the order of the increments, why work is parked, and what is out of scope. Order is
not a blocker link; use native blockers for true prerequisites. Dated evidence belongs in the closing comment of the
issue, not here. Keep existing issue numbers through replanning.

## Delivery order

Delivered so far (see the closed issues): M0 to M4.5, the map (#28), the visual baseline (#72), workspace focus
(#63), parallel issue sessions with working copies and tabs (#79), and registered skill sources with a picker and
launch with a skill (#43).

| Order | Increment | Issues | Design |
|---|---|---|---|
| 1 | M7: trustworthy session attention; probe before implementation | #48 → #45; #52 provides real-CLI fixtures | `docs/design/experience.md` |
| 2 | Ready work and planning before issues | #67, #68; #61 publishes shaped issues | `docs/design/campaign-loop.md` |
| 3 | Session ownership and continuity | #69 → #70 | `docs/design/sessions.md` |
| 4 | Outcome review and continuation | #71; #47 verifies killed outcomes on Windows | `docs/design/campaign-loop.md` |
| 5 | M8: verified portable release and desktop packaging; the lifecycle decision precedes shell wiring | #35–#38, #36/#57 | `release/PLAN_RELEASE.md` |
| Later | M6 / v2: rework the map view as a whole, once the full workflow is usable | #151 (#44) | `docs/design/map.md` |

#69 can run alongside the attention work: decide ownership before implementing desktop-window shutdown.
#83 (refreshing quest state after pushes) has high priority and is not tied to an increment.

## Conditional and platform work

| Work | Issues | Trigger or prerequisite |
|---|---|---|
| Native desktop notifications | #58 (#53–#57) | shell focus/click-through and lifecycle decision; consume hooks when verified |
| Real Windows CLI and killed-session outcome | #32, #47 | native Windows environment with Claude |
| Windows and Linux working-copy checks (path length, file locking, submodules, LFS) | listed in `docs/design/sessions.md` | a native Windows or Linux machine |
| Rate-limit/shim behavior against real services | #33, #34 | small controlled probes; never provoke a secondary limit |
| Local issue database | #41 | demonstrated offline or multi-campaign need |
| Durable paced write queue | #42 | app-managed bulk publishing requires resumable operations |
| Local issue pins | #40 | needed scope without changing GitHub labels |
| Documentation/contract checks | #46 | a short bounded checker; includes unchanged legacy label/body migration |
| Further distribution | #39 | actual users/platform demand after initial release |
| Another agent provider | `docs/ideas.md` | validate the core loop first; integrate one provider when demand justifies it |

## Out of scope

No generic pane manager, plugin platform, alternate tracker, native-framework rewrite or provider registry is required.
Keep one workspace model, the existing art and the RPG/Neutral presentations.

## Updating

Read `CLAUDE.md`, this file, the current issue and its design doc. Change this file only when the order, a trigger or
the scope boundary changes. A design spec or an issue label is not evidence that something is implemented; close issues
only after their exit criteria and the repository delivery procedure are met.
