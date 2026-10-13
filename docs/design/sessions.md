---
status: investigation
milestone: session continuity
github_issues: [69, 70, 79, 80, 81]
tracking: in-gh-issues
next: record the ownership decision before implementing recovery or wiring desktop-window shutdown
verified: 2026-10-13
---

# Session ownership and continuity

Current behavior: the Go server owns PTYs. Browser disconnect does not stop a session, server shutdown kills sessions,
and restart retains metadata rather than live terminals. History marks unfinished records as lost. The lifecycle
investigation and continuity implementation are future work; historical persistence is not live-session recovery.

## Decisions to settle

| Action | Intended contract to validate |
|---|---|
| Close/reopen browser UI | Detach/reattach to the running backend; preserve process identity |
| Close desktop window | Detach by default; backend ownership must be decided before shell wiring |
| Close an active session | Explicitly terminate that session, with running-session confirmation |
| Stop backend intentionally | State the consequence and separate stop-all from detach |
| Crash/restart backend | Recover only where a verified process boundary supports it; otherwise report lost |
| Remove project | Never delete project files or silently terminate its sessions |
| Resume a conversation | New agent process using verified provider identity; not reconnecting to a PTY |

Evaluate the smallest viable independent backend ownership and persistent-session options. Do not preselect tmux,
a daemon, a database or a framework rewrite. Record Windows, macOS and Linux limitations and practical verification
needs. Keep portable browser operation and the existing loopback authentication/Host/Origin boundaries.

## Implementation boundaries

Maintain stable session IDs, campaign ownership, local tree occupancy, terminal dimensions, bounded scrollback and
attention state across detach/reattach. A recovered session must not create duplicate issue work. Stale attachment
handles fail clearly. Shutdown and child-process cleanup must be deliberate and tested, including killed outcomes.
No restart flow may claim an old PTY is still alive because only its historical record exists.

Desktop packaging and focus notifications consume the chosen lifecycle; they do not decide it implicitly. True backend
restart survival is a separate acceptance boundary from UI closure. Conversation resume remains a later capability
until its provider identifiers and behavior have a dedicated verification plan.

## Parallel issue sessions

Today the server refuses a second issue session in the same Git working tree (`quest_session_active`), so two issues
of one project cannot run at once; free sessions and shells are not limited. #79 lifts that limit. #80 decides how
each issue session gets its own working copy (a `git worktree` per session is the first candidate; a separate clone
or letting planning-only sessions share the tree are the alternatives), including location in local state, branch
naming, untracked files a new copy needs, cleanup and uncommitted work, with platform checks. #81 implements the
recorded answer: one active session per issue and one per working copy, shown in the launch review. Copies never
appear as committed files or folders the team sees. Tabs for moving between the sessions are #82 (`docs/design/ui.md`
§4.2). Record #80's answer and its dated probe evidence here.

## Evidence required

Use real PTYs running stubs for disconnect, reconnect, process termination and failure handling; browser tests for
reopening and scrollback; native desktop checks for window closure. Record the decision and platform results here.
Implementation can start only after the investigation identifies the ownership boundary and testable failure modes.
