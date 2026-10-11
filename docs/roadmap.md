---
status: current
milestone: all
github_issues: [23, 28, 43, 44, 45, 58, 35, 36, 37, 38, 39]
tracking: in-gh-issues
next: update the table when a milestone closes; keep one row per milestone
verified: 2026-10-11
---

# Roadmap

Where the project is and what comes next. This is a map, not a plan: each row points to the GitHub epic that holds the
work and to the design doc that holds the detail. The open issue list is the source of truth; if this file disagrees
with it, fix this file. History lives in git and the closed issues, not here.

## Milestones

| Milestone | What it delivers | Status | Epic | Detail |
|---|---|---|---|---|
| M0 Skeleton | Go server, embedded SPA, guard | done | | `docs/architecture.md` |
| M1 Heroes and terminal | `party.toml`, PTY sessions, terminal in the browser | done | | `docs/architecture.md` |
| M2 Campaigns | Registered working directories, git root, owner/repo | done | | `docs/architecture.md` |
| M3 GitHub quests | Quest list and detail, GitHub and demo sources | done | | `docs/design/experience.md` §4 |
| M3.5 GitHub budget and scope | Quartermaster transport, `gh` shim, scope as part of the query | done except #41, #42 | | `docs/design/github-budget.md` |
| M3.6 Cockpit shell and first run | Stitch v4 shell, first run, words | done | #7 | `docs/design/experience.md` §5 to §7 |
| M4 Launch from a quest | Briefing, launch button, session linkage | done | #19 | `docs/design/experience.md` §4 |
| M4.5 Attention | Waiting sessions in the list, tab title, favicon, optional browser notification | built, closes when pushed | #23 | `docs/design/experience.md` §4 |
| M5a Map v1 | Read-only layered graph of a campaign's quests, creatures, select and start | next | #28 | `docs/design/map.md` |
| M5b Spells | Skill discovery, a suggested spell per quest, a spell menu in the Start button | planned | #43 | `docs/design/skills.md` |
| M6 Map v2 | Focus, halls, outskirts, milestone floors | planned | #44 | `docs/design/map.md` |
| M7 Hooks status | Hook-driven session state replaces the output heuristic | planned | #45 | `docs/design/experience.md` §4 |
| M8 Packaging | Release pipeline, macOS app, Linux AppImage, checksums | planned | #35 to #39 | `release/PLAN_RELEASE.md` |

## Cross-cutting work

Not tied to one milestone; scheduled when its dependency lands.

| Work | Epic or issues | Depends on |
|---|---|---|
| Native desktop notifications on Windows, macOS and Linux | #58 (#53 to #57) | the desktop shell (#57, which needs #36); fires from #45 hooks when they exist |
| Windows verification of the real `claude` CLI | #32, #47 | a Windows machine with `claude` installed |
| Rate-limit and shim probes against the real services | #33, #34 | network access and a throwaway repository |
| Local SQLite copy and paced write queue | #41, #42 | none |
| Pins as a scope mode | #40 | none |
| Real-claude smoke check and captured output fixtures | #52 | replaces the guessed "needs you" patterns in the activity classifier |
| Docs check in `make check` | #46 | none |

## Reading order for a new session

1. `CLAUDE.md`: rules and conventions.
2. This file: where things stand.
3. The epic for the milestone you are working on, then its design doc.
