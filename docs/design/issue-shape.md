---
status: current
milestone: M5a
github_issues: [28, 24, 27, 61, 62]
tracking: in-gh-issues
next: keep until the map is built; then fold what remains into the issue-writing skill
verified: 2026-10-11
---

# Issue shape: what the map reads from GitHub

The map (`docs/design/map.md`) places and draws a quest from a few facts. This is the contract: what an issue must
carry so the app can show it correctly. It uses plain GitHub features and neutral wording; nothing here is themed and
nothing needs a file in the repository.

## What the app reads

| Fact | Where it lives in GitHub | What the map does with it |
|---|---|---|
| Epic and its quests | Sub-issues: the epic is the parent, the quests are its sub-issues | A top-level parent is a dungeon; its sub-issues are the rooms; a sub-issue that is itself a parent is a hall |
| Order of work | "Blocked by" links between issues | Lines and gates; lines between dungeons are derived from them |
| No epic | An issue with no parent and no sub-issues | The Tavern |
| Effort tier | A label `effort:trivial`, `effort:low`, `effort:moderate`, `effort:high` or `effort:extreme` | Picks the creature (3 per tier in `design/quest-encounters/CREATURE_ROSTER.md`) |
| In the app's scope | The scope label (`tracked` by default) | Whether the issue is shown or is an out-of-scope stub |
| Who holds it | Assignees | The assigned-elsewhere marker |
| Closed | Issue state | Cleared, greyed with a green check |

An issue with no effort label has unknown effort: the room shows the hollow diamond, and the app never guesses from the
title. An epic's own `effort:*` label sets the creature of its parent room; the dungeon's art tier on the world map is
derived from its size and longest blocker chain.

## Effort tiers

`trivial`, `low`, `moderate`, `high`, `extreme`. The words are neutral on purpose: they appear on GitHub, where the
team sees them.

- The label names are a default. Each campaign can map its own labels (for example `size/S`) to the five tiers in local
  state, so a team that already estimates does not have to relabel anything.
- The user can set a tier on one quest in the app without touching GitHub. That override stays on their machine.
- Priority is a separate signal with its own source (not decided yet); it never changes the tier.

## What an issue-writing skill would do

Later, once the map works against this contract, a thin skill can create issues in this shape: an epic with sub-issues,
blocked-by links in the right order, an effort label on each, and the scope label. It writes through the `gh` shim, one
issue at a time, never in a loop. It starts from Matt Pocock's skills; only the gap (this shape) is ours. Tracked in
GitHub issue #61.
