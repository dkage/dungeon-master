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

## Terminal connection (#136)

Delivered: the server pings each terminal websocket every 20 s and closes a peer that does not answer in 10 s; one log
line per closed connection records the session id, duration, silence and cause (never input, output or tokens). The page
reattaches after an abnormal close with a bounded backoff (0.5 s doubling to 15 s), clears its screen and takes the
replayed scrollback, so output is never shown twice. After a close it asks the server with a plain `GET` of the terminal
URL: 401 means the access token changed, 404 the session is gone (restart), 410 it was lost, a network error means the
app is unreachable (keep retrying), 426 means the session is alive. The first three stop retrying and say what to do.

Decision for #173: the browser reattaches (close code 1013 reattaches at once, without the probe) rather than the server
keeping a slow viewer attached, because the server's bounded subscription buffer protects memory and the replay already
restores everything. A footer note shows when the API or any terminal cannot reach the backend (a 4 s heartbeat detects
a stopped backend). Browser tests simulate drops and a changed token with Playwright websocket and request routing;
restarting the shared test server under an open page is not done.

Reattach by position (#178): the session counts every byte it has produced. Each connection opens with a text frame
`{"type":"attach","start":N,"resume":B}`; the page counts the bytes it receives and reconnects with `?from=<position>`.
When the retained scrollback still reaches back to that position the server replays only the missed bytes and the page
keeps its screen; otherwise (position overwritten, unknown, or an older server that sends no attach frame) everything is
replayed and the page clears first.
