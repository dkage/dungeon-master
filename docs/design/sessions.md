---
status: investigation
milestone: session continuity
github_issues: [69, 70, 79, 80, 81]
tracking: in-gh-issues
next: record the ownership decision before implementing recovery or wiring desktop-window shutdown
verified: 2026-10-14
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
§4.2). The answer and its probe evidence are recorded below ("Working copies for parallel issue sessions").

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
`web/e2e/restart.spec.ts` also restarts a real backend (its own, on a spare port, with its own state directory) under an open page: a crash and restart shows the session is gone, and a restart with `--new-token` shows the sign-in message.

Reattach by position (#178): the session counts every byte it has produced. Each connection opens with a text frame
`{"type":"attach","start":N,"resume":B}`; the page counts the bytes it receives and reconnects with `?from=<position>`.
When the retained scrollback still reaches back to that position the server replays only the missed bytes and the page
keeps its screen; otherwise (position overwritten, unknown, or an older server that sends no attach frame) everything is
replayed and the page clears first.

## Working copies for parallel issue sessions (#80)

Status: researched on macOS on 2026-10-14; Linux and Windows checks are open (listed at the end).

### Answer

Each issue session gets its own `git worktree`, created and removed by the app with argv-only `git` calls. A second
clone is the fallback only for a repository where worktrees are unusable (see "Why not a clone"). Planning-only
sessions may share the main working tree, by the user's choice in the launch review, never silently.

| Question | Answer |
|---|---|
| Where the copies live | `<state dir>/worktrees/<campaign id>/<session id>/`, in the user's config directory, never inside the repository or any folder the team sees. Created 0700. |
| What Git itself writes in the repository | `.git/worktrees/<name>/` metadata and the new branch ref. Both live inside `.git`; `git status` of the main tree shows nothing. Only the branch is visible to `git branch`. |
| Starting point | The tip of the branch the project is on (`HEAD` of the main tree) at launch, shown in the launch review. A copy never contains the main tree's uncommitted or untracked files (probed: a modified `a.txt` was the committed version in the copy). The review says so. |
| Branch naming | `dungeon/issue-<number>` (ASCII digits only, built from the issue number, never from its title). If it exists and is checked out in another copy, Git refuses (`already used by worktree`); the app treats that as "this issue already has a session" and offers the existing one. If it exists unchecked-out, it is reused. |
| Hostile names | Branch and directory names use only `[a-z0-9-]` plus the numeric issue and the session id; `git check-ref-format` accepts `;`, `é` and rejects `..`, spaces, a leading `-` and `.lock`, so the app does not rely on it. Every call is argv (`git worktree add -b <branch> <path> <ref>`), never through a shell. |
| Monorepo campaigns | A campaign inside a subfolder (`apps/api`) runs in the same subfolder of the copy (probed: `apps/api/f` exists in the copy). The git root is still the unit of conflict. |
| Untracked and ignored files | Not copied by default. Probed: `.env` and `node_modules/` (ignored) and an untracked note are absent from the copy. The launch review lists the ignored/untracked top-level entries of the main tree (`git ls-files --others --ignored --exclude-standard --directory`) and offers an explicit per-entry choice to copy or symlink a small file such as `.env`; dependency folders are never copied automatically (the agent installs them). Nothing is chosen for the user. |
| Shared configuration | Hooks, `.git/config` and user identity are shared with the main tree (probed: `user.name` and the common git dir resolve to the main repository). Local agent settings kept as untracked files (for example a project's local Claude settings) are not carried over; say so in the review. |
| Cleanup when a session ends or is closed | Closing the session stops the agent first (#117), then removes the copy only when it is clean (`git worktree remove` without `--force` refuses a copy with modified or untracked files, probed). A clean copy is removed; the branch is kept unless it has no commits beyond its start point, when `git branch -d` removes it. |
| Uncommitted work | Never discarded silently. A dirty copy is kept, the session row says "working copy kept: uncommitted changes", and the user can open it, or remove it explicitly with a confirmation naming the files. Commits on the branch are never deleted. |
| A copy deleted by hand | `git worktree list` marks it `prunable`; the app runs `git worktree prune` for that campaign when it starts a session or lists copies, then recreates it if the session is still wanted (probed). |
| Conflict rules (for #81) | One active session per issue, and one session per working copy. A free terminal or a planning session in the main tree counts as occupying the main tree only. Today's check (`quest_session_active`, `camp.ParallelQuests` in `internal/server/api.go`) is replaced by these. |

### Planning-only sessions

A planning session (the #43 method, #68) may use the main working tree while an implementing session runs in a copy,
because it should not change files and the copy isolates the implementer's changes from it. Nothing enforces "should
not change files": an agent can still edit. So the launch review states "this session works in your main tree" and
two sessions are never put in the main tree together unless both are planning sessions. The user can always choose a
copy instead.

### Why not a clone

A local clone (`git clone --local`) took 0.04 s and 144 KB for the probe repository and hardlinks objects, but it
carries its own configuration and remotes: `origin` points at the local path, so pushes go to the main tree's
repository, not the real remote, and every copy needs its remote rewritten and its branches fetched back. A worktree
shares the repository, its remotes, hooks and objects, so a commit in a copy is visible to the main repository at once
without fetching. The clone is therefore a fallback only for repositories where `git worktree add` fails (bare layouts,
some submodule setups).

### Probe evidence (macOS, Darwin 25.5, git 2.50.1, 2026-10-14, throwaway repository)

- `git worktree add -b dungeon/issue-12 <state>/wt/issue-12 HEAD` created the copy outside the repository; the main
  tree's `git status` was identical before and after (` M a.txt`, `?? untracked.txt`), and `ls -a` of the repository
  showed no new entries. `.git/worktrees/issue-12/gitdir` and the copy's `.git` file hold absolute paths: a copy moved
  by hand breaks.
- A second copy of the same branch failed: `fatal: 'dungeon/issue-12' is already used by worktree at <path>`. A second
  issue on its own branch succeeded, and a file written in the first copy did not appear in the second or the main tree.
- `git worktree remove <dirty copy>` failed with "contains modified or untracked files, use --force to delete it"; a
  clean copy was removed and its branch stayed. After deleting a copy's directory by hand, `git worktree list` showed
  `prunable` and `git worktree prune` cleared it.
- `core.hooksPath` unset, `user.name` and `--git-common-dir` resolved to the main repository from inside a copy.
- `git ls-files --others --ignored --exclude-standard --directory` listed `.env` and `node_modules/` (ignored); neither
  existed in the copy.

### Open platform checks

- Windows: path length (copy path under `%LOCALAPPDATA%` plus a repository's deepest path against the 260-character
  limit; whether `core.longpaths` is needed), file locking when removing a copy whose agent was just killed (retry with
  a delay), `git worktree` with CRLF settings, and whether the shim `gh` and the `git` in the agent's PATH resolve in a
  copy. Needs a native Windows run.
- Linux: the same probes (expected identical); a case-sensitivity difference for branch names; a copy on a different
  filesystem from the repository (works, since only metadata paths are absolute).
- All platforms: a repository with submodules (not initialised in a copy; decide whether the review offers
  `git submodule update --init`), Git LFS, and a repository whose main tree is itself a worktree or a bare layout.
- Free disk: a copy is a full checkout; the review shows its size estimate (`git ls-files` count) for large repositories.

### What #81 must cover (scope changes to the issue)

- The launch review shows: copy location, branch and start point, the list of ignored/untracked entries with the
  copy/symlink choice, and the "main tree uncommitted changes are not in the copy" note.
- Create/reuse/prune/cleanup exactly as in the table; clean copies are removed on close, dirty ones are kept and shown.
- Conflict rules replace `quest_session_active`; planning-only sessions may share the main tree as described.
- Tests: branch/path sanitisation with hostile names, a dirty copy kept on close, a prunable copy recreated, a
  second session for the same issue refused, two stub issue sessions writing in separate copies, all against a
  temporary repository and never the real `git` config (`GIT_CONFIG_GLOBAL`/`SYSTEM` set empty).
