# Open work

A snapshot of what is missing or undecided, taken on 2026-10-08 after M3.5 phases A, B and E. It is meant to be picked
up in a later session and then turned into GitHub issues. This file is a worklist, not a design: the designs live in the
plans it points at. When an item becomes an issue, delete it here; when the file is empty, delete the file.

Built so far: M0 to M3 (skeleton, heroes and terminal, campaigns, GitHub quests), the Quartermaster, scope in the query
and its picker, the `gh` shim over a private socket, and plain-words notices. Details are in `CLAUDE.md`.

## 1. Decide first

These block or shape the work below. Each has a recommendation; none has an answer yet.

| # | Question                                                                           | Recommendation                                                        |
|---|------------------------------------------------------------------------------------|-----------------------------------------------------------------------|
| 1 | Roadmap import: build an idempotent command before M4, or go straight to M4?       | Straight to M4; import once the Quartermaster has a real write caller |
| 2 | Should sessions receive `GH_TOKEN` and `GITHUB_TOKEN` from the app's own env?      | Keep inheriting; document the issues-only token (done in README)      |
| 3 | Does the scope picker live inside the Start-here checklist (M3.6)?                 | Yes, one first-run path (`PLAN_M3_7.md` 2.5)                          |
| 4 | Does the plain-words switch ship with the hover help in M3.6 or right after?       | With it; both are about the same words (`PLAN_M3_7.md` 2.9)           |
| 5 | Is the milestone order from `PLAN_EXPERIENCE.md` §4 confirmed (map before spells)? | Yes, it is the reason to pick this app over a plain launcher          |
| 6 | Plain-words rules: are the six plumbing names internal only?                       | Yes; applied to notices, still to check the rest of the UI            |

## 2. Next milestones, in order

The content of each is in `PLAN_EXPERIENCE.md` §4; this is only what is not started.

- **M3.6 Stitch shell, first run and words** (`PLAN_EXPERIENCE.md` §4–6): implement the Stitch v4 cockpit structure
  and visual foundation first, including the right-panel map/detail split; then fit onboarding, sample campaign, repo
  suggestions, `words.ts` with `Term` popovers, starter heroes as an action, empty states, appearance settings,
  `make run-fresh`, and a first-run browser test into that shell. The graph remains M5a.
- **M4 Launch from a quest**: briefing file, primary "Start with" button, session titles from quests, quest chip,
  one-session-per-tree gate. GitHub write-back stays opt-in and off.
- **M4.5 Attention**: "needs you" from PTY output recency, tab title and favicon badge, optional OS notification.
- **M5a Map v1** (`PLAN_MAP.md`, `PLAN_UI.md`): implement the read-only layered graph inside the map region reserved
  in M3.6; blocked-by and parent edges, click selects, double-click starts. It must not hide blockers outside the scope
  (`PLAN_M3_7.md` 2.6).
- **M5b Spells** (`PLAN_SKILLS.md`), **M6 Map v2**, **M7 Hooks status**, **M8 Packaging**.

## 3. Left over from M3.5

- **Chronicle** (SQLite copy of tracked issues) and **Caravan** (paced bulk writes): build only when a feature needs
  offline use or a large import. Design is kept in `PLAN_M3_5.md` §5.
- **Camp against the real GitHub**: refusal and recovery have only run against the fake. Needs a manual probe that is
  safe to run (read-only calls with a tiny budget), recorded in the gitignored `_probe/`.
- **Pins as a scope mode**: local, no footprint on GitHub. Listed in `PLAN_V2.md`; build when someone cannot add a
  label.
- **Gatekeeper limits to measure**: how an agent behaves on exit 75 (does it stop, as the rule asks, or retry?). Try it
  with a real Claude session by hand.
- **Rate-limit chip placement**: it is a line in the quest panel; `PLAN_UI.md` may move it. Follow that plan.

## 4. Windows and release

Windows 10 1803 is the minimum (README). Nothing has run on Windows yet.

- The `gh` shim is a symlink; Windows needs a copy of the binary or a `.cmd`. Decide, build and test.
- Run the Gatekeeper socket and the PTY on a real Windows machine; add a `windows-latest` job with a ConPTY round trip
  (`release/PLAN_RELEASE.md` §7).
- Path and line-ending handling in `internal/campaign` and the launcher on Windows (drive letters, `\`).
- GoReleaser, release-please and the rest of R0 to R6 are planned, not built (`release/PLAN_RELEASE.md`).

## 5. Verification debt

- Hook settings injection (`--settings`) for session state, needed by M7.
- Behaviour of `claude` on Windows (flags, PTY, login-shell lookup).
- Linux CI covers `go test`; the browser tests run on Ubuntu only. macOS and Windows e2e are not run.

## 6. Documentation to reconcile

`PLAN_M3_7.md` §1 has the checklist; none of it is done. In short:

- `PLAN.md` still has an old status line, old room-state names, `go-github`, a keychain auth plan and the old sync
  design; §11 and §13 are out of date.
- `PLAN_SKILLS.md`, `PLAN_V2.md` (`QuestSource`) and `release/PLAN_RELEASE.md` have stale references.
- Move status words ("in progress", "deferred") out of the plans into the `CLAUDE.md` line, and give each plan a header
  saying whether it is current, built or superseded, so no agent acts on a stale one.
- Optional: a small `make docs-check` (line length, banned phrases, links, glossary against `words.ts`).
- Keep these plans until their content is in GitHub issues, then delete them; git keeps the history.

## 7. Housekeeping

- `DUMMY_PLAN.md` is a formatter test file that keeps coming back untracked. Add it to `.gitignore` or move it under the
  formatter's test directory.
- The demo issues #1 to #6 in `dkage/dungeon-master` are all marked "[demo]"; close them when no longer needed.
- `.claude/skills/formatting-markdown/` has uncommitted changes from another session; commit or drop them there.
- Wide table rows in `GLOSSARY.md` and `PLAN_M3_5.md` cannot be wrapped; shorten the cells.
- `CLAUDE.md` still says "Commit only when asked"; the standing rule in this project is to commit each step and push to
  `master`. Update the line so a new session does not hesitate.
