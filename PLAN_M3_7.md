# M3.7: plan reconciliation and review follow-ups

This is a docs-and-decisions milestone. It touches no code and edits no file that another milestone is working in. It
exists so the plans can be brought in line in one pass once `PLAN_M3_5.md` (phases A and B) and `PLAN_EXPERIENCE.md`
(M3.6) have stopped changing them.

Start it when those two have landed. Until then the checklist in part 1 is a to-do list, not an instruction.

Two parts:

1. Stale text that is still in the plans, with the target wording.
2. Review findings on the new M3.5 and M3.6 plans and on `GLOSSARY.md`, each with a recommendation and, where it is a
   real choice, a decision to confirm.

## 1. Reconciliation checklist

Checked against the tree after the M3.5 plan was reshaped. Each row says what is wrong and what it should say. "Done
when" at the end of the part gives a check that can be run.

### `PLAN.md`

| Section                  | Problem                                                                          |
|--------------------------|----------------------------------------------------------------------------------|
| Top status line          | Still says "plan only… GoLand hello-world… `module dungeon`"                     |
| §2 Room state row        | Uses Unexplored, Occupied, Trapped; code uses open, in progress, locked, cleared |
| §3 UI layout             | Superseded by `PLAN_EXPERIENCE.md` §3 (stage tabs, one primary Start button)     |
| §4 Libraries             | Lists `go-github` (REST); the client is hand-written GraphQL                     |
| §6 Auth                  | A pasted token in the OS keychain (`99designs/keyring`); never built             |
| §6 Sync                  | Describes ETag polling and a JSON cache; replaced by `PLAN_M3_5.md`              |
| §6 Relationships         | "Verify, then fall back to parsing the body"; verified, fallback not built       |
| §6 Quest state paragraph | Same old state names as §2                                                       |
| §11 M4 to M7             | Order superseded by `PLAN_EXPERIENCE.md` §4                                      |
| §12 relationships risk   | Resolved in M3                                                                   |
| §13 decisions 1, 2 and 4 | 1 old module name; 2 old layout; 4 old sync design                               |

Target wording:

- Status line: delete it. `CLAUDE.md` owns status.
- Room state: open (unclaimed, unblocked), in progress (assigned), locked (open blocker), cleared (closed). Add
  "occupied" and "needs you" as session-driven states that arrive with M4 and M4.5, and drop "trapped" unless a
  failed-session state is wanted.
- §6 Sync and Auth: replace with one paragraph that points at `PLAN_M3_5.md` and says what is true today (GraphQL, 60 s
  in-memory cache, stale fallback, credentials from `GH_TOKEN`, `GITHUB_TOKEN` or `gh auth token`).
- §11: keep the M-numbers as history, add one line: "Order of upcoming work: `PLAN_EXPERIENCE.md` §4."
- §13: rewrite decisions 1, 2 and 4 to the current ones, or delete the section; it is a list of first guesses.

### Other files

| File                      | Problem                                                                                |
|---------------------------|----------------------------------------------------------------------------------------|
| `CLAUDE.md`               | "Open verifications" still lists M3; M1 and M7 items have no recorded result           |
| `CLAUDE.md`               | Vocabulary line says "Dungeon Master = the room graph"; the map is "dungeon"           |
| `CLAUDE.md`               | Milestones line follows the old order; Layout lacks the new packages                   |
| `PLAN_SKILLS.md`          | "Assigned → Occupied" row; open question 2 is answered; §7 follows the old order       |
| `PLAN_V2.md`              | Refers to `QuestSource`; the interface is `Source`                                     |
| `GLOSSARY.md`             | "Dungeon: the map, M6" (now M5a); "Hero: toolbar" (moves into the quest detail)        |
| `release/PLAN_RELEASE.md` | Check milestone references; SQLite (`modernc.org/sqlite`) must keep the build cgo-free |

Target wording:

- Open verifications: replace with "Verified" and "Still open". Verified: GitHub sub-issue, blocked-by and assignee
  data through GraphQL (M3). For the slash-skill and hook/`--add-dir` questions, record what was found, or keep them
  open with the milestone that settles them (M5, M7). The launcher already passes `--add-dir` and
  `--append-system-prompt`, so at least part of the second is known.
- Vocabulary: "Dungeon = the map of a campaign's quests." The product name is not a vocabulary term.

### Done when

- These searches return nothing in tracked files: `Unexplored`, `Trapped`, `go-github`, `keyring`, `QuestSource`,
  `hello-world`, `module dungeon`, `matches the directory`.
- Every plan names the same milestone order, or points at the one file that does.
- `make docs-check` (below) passes.

## 2. Review findings

### 2.1 The glossary grew faster than the budget

`GLOSSARY.md` now adds Quartermaster, Rations, Camp, Chronicle, Scout, Caravan, Gatekeeper and Tracked. With the
original seven that is fifteen themed words, and six of the new ones name plumbing (a rate limiter, a request wrapper, a
database, a queue). `PLAN_EXPERIENCE.md` caps visible nouns at about eight and says a new word "needs a written reason".

The bigger problem is flavour on a state that needs action. "The party is making camp until 15:12" hides "GitHub asked
this app to slow down; requests are paused until 15:12". A user who does not know the word does not know that anything
is wrong or what to do.

Recommendation:

- Keep Quartermaster, Chronicle, Caravan, Gatekeeper and Scout as internal names in code and docs. The user never sees
  them.
- In the interface, plain text: "GitHub requests: 4,936 of 5,000 left, resets 15:37" and "Paused until 15:12: GitHub
  asked this app to slow down. Showing saved data."
- Rule to add to the glossary: any state the user must act on or wait for is written in plain words, with no flavour
  term as the only explanation.
- Count only words shown in the UI against the budget; mark the others "internal" in the glossary table.

### 2.2 The `tracked` label is a visible write to the team's GitHub

Decision 2 of `PLAN_M3_5.md` makes scope a label that the user adds to issues ("adopting an old issue means adding the
label"), and issues the app creates carry it. That is a change every teammate sees, and it contradicts "invisible to the
team" and "GitHub write-back is opt-in, off by default". It also makes the first-connection picker lead with "tracked
(0)" in nearly every repository, which means nothing to a new user.

Options:

| Option                       | Footprint on GitHub | Notes                                                 |
|------------------------------|---------------------|-------------------------------------------------------|
| Local pins                   | none                | A list of issue numbers per campaign, stored locally  |
| Existing labels and assignee | none                | Pick from labels already used; "assigned to me"       |
| `tracked` label              | visible to everyone | Shared across machines, costs a write and a team norm |

Recommendation: default to existing labels, "assigned to me" and local pins, none of which write anything. Offer the
`tracked` label as an explicit opt-in ("Also mark these on GitHub, so other machines see the same list"). Lead the
picker with what exists (counts of real labels and of assigned issues), not with a marker that does not exist yet. This
changes decision 2 and §3 of `PLAN_M3_5.md`; confirm before phase B ships the picker.

### 2.3 The Gatekeeper needs a security design before it is built

Phase E adds a `gh` shim on the `PATH` of sessions that asks the app for a "slot" using "a per-session token". That is a
new authenticated entry into the app, and `CLAUDE.md` calls any weakening of `internal/server/guard.go` a release
blocker. The plan does not say how the shim authenticates without that.

Recommendation:

- Use a per-session Unix domain socket under the config directory (directory 0700, socket 0600), or a second loopback
  listener that serves one route and nothing else. Do not add a token path to the browser-facing guard.
- The endpoint does one thing (grant or refuse a slot) and accepts no input beyond the command class.
- Tests include the hostile cases: another local user, a web page, a replayed or stale session token, a very large
  request.
- Say plainly what the shim cannot stop: an absolute path to `gh`, `gh api`, `curl`, or any other client. It reduces
  accidents; it is not a boundary.
- Name the cheaper first layer. In the interactive terminal, Claude asks before running `gh issue create` unless the
  hero skips permissions, so every call is a natural pause. The lockout most likely came from a session that did not
  ask.
  Starter hero templates should not skip permissions, and the docs should say a hero that does so can flood GitHub.
- Sessions inherit the full environment, including `GH_TOKEN`, because `launcher.Environ` only removes parent-session
  markers. Decide whether agent sessions should get the token at all (`gh` has its own credential store) and document
  the fine-grained, issues-only token as the recommended one.

### 2.4 Status is inside the plans again

`PLAN_M3_5.md` carries "Status: decided, in progress", a status column and "awaiting confirmation" in the decisions
table. `PLAN_EXPERIENCE.md` §8 says status belongs in the `CLAUDE.md` line only. Plans that hold status go stale the
first time a phase finishes.

Recommendation: keep decisions and rationale in the plan; move "in progress", "planned", "deferred" and "confirm" to the
`CLAUDE.md` status line, and mark deferred work with its trigger ("build when a feature needs offline use") instead of
"deferred".

### 2.5 One first-run path, not two

M3.6 adds a Start-here panel with "add a project, connect GitHub, pick a quest". M3.5 phase B adds a first-connection
scope picker. Built separately they become two first-run panels, and the picker would appear before the checklist is
aware of it.

Recommendation: the scope picker is step 3 of the Start-here checklist ("choose which issues to show") and the same
component is reused later from the panel header. No scope yet means the step is open; a chosen scope ticks it. Agree the
component and the state flag (`scope` absent means first connection) before either side builds its UI.

### 2.6 The map must not hide blockers outside the scope

Scope filters the issue list. A scoped issue can be blocked by an issue outside the scope. If the map draws only scoped
issues, a locked room has a door to nowhere and the picture is wrong. Quest state already comes from the blocker's own
state, so it stays correct; the drawing must too.

Recommendation: the map shows an out-of-scope blocker or parent as a muted stub with its number, title and state, not
as a full room, and the list says "blocked by #88 (outside the current scope)". Add this to M5a's exit test.

### 2.7 The sample campaign and the Quartermaster

The sample source makes no GitHub calls. Say so in the plan and test it: no rations chip and no camp state for a sample
campaign, and a camp for a real campaign never blocks the sample.

### 2.8 One wording for "old data"

The panel already shows "Showing older data" when a refresh fails. The camp banner, the stale banner and offline mode
are three messages for one situation. Use one pattern: what happened, until when, what the user sees meanwhile.

## 3. A docs check

A small `make docs-check` keeps this from drifting again. It stays tiny and reads only tracked files.

- `python3 ~/.claude/skills/formatting-markdown/scripts/mdfmt.py --check` over `*.md` and `release/*.md`, or a plain
  line-length check if that script is not on the machine.
- A list of banned phrases (the ones in "Done when") that fails on a match.
- Every relative link in a plan points at a file that exists.
- Every word in `GLOSSARY.md` marked "UI" appears in `web/src/lib/words.ts` once that file exists, and the reverse.

Skip it if it grows beyond a short script; the reconciliation pass is the valuable part.

## 4. Decisions to confirm

1. Are the six plumbing terms (Quartermaster, Chronicle, Caravan, Gatekeeper, Scout, Camp) internal only, with plain
   interface text?
2. Is scope without a GitHub write the default, with the `tracked` label as an opt-in?
3. Does the Gatekeeper talk to the app over a Unix socket, and should sessions receive `GH_TOKEN` at all?
4. Are plan statuses moved out of the plans into the `CLAUDE.md` line?
5. Should the scope picker live inside the Start-here checklist?
6. Does M3.7 run as one docs commit after M3.5 A/B and M3.6, or in pieces as each lands?
