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

Sub-issues, blocked-by links, assignees and linked pull requests all come from the one GraphQL query the app already
makes (verified in M3, see `CLAUDE.md` Verifications). Effort is the only new fact, and it comes from labels, which the
same query returns.

## What the app reads

| Fact | Where it lives in GitHub | What the map does with it |
|---|---|---|
| Epic and its quests | Sub-issues: the epic is the parent, the quests are its sub-issues | A top-level parent is a dungeon; its sub-issues are the rooms; a sub-issue that is itself a parent is a hall |
| Order of work | "Blocked by" links between issues | Lines and gates; lines between dungeons are derived from them |
| No epic | An issue with no parent and no sub-issues | The Tavern |
| Effort tier | A label `effort:trivial`, `effort:low`, `effort:moderate`, `effort:high` or `effort:extreme` | Picks the creature (3 per tier in `design/quest-encounters/CREATURE_ROSTER.md`) |
| In the app's scope | The scope label (`tracked` by default) | Whether the issue is shown or is an out-of-scope stub |
| Who holds it | Assignees | The assigned-elsewhere marker |
| Priority | A label `priority:low`, `priority:normal`, `priority:high` or `priority:urgent` | High (amber) and urgent (red) draw a small jewel on the room |
| Intent to be an epic | A label `epic` or `initiative` (a hint only) | A hint when it has no sub-issues (see Epic hint labels) |
| Closed | Issue state | Cleared, greyed with a green check |

## Worked example

Three kinds of issue in one repository (numbers are examples):

```
#10  Ship the onboarding flow          epic: has sub-issues #11 to #14        labels: tracked
 ├─ #11  Design the welcome screen     closed                                 labels: tracked, effort:trivial
 ├─ #12  Build the signup form         blocked by #11                         labels: tracked, effort:moderate
 ├─ #13  Validate email addresses      blocked by #12                         labels: tracked, effort:moderate
 └─ #14  Send the welcome email        blocked by #13, #30                    labels: tracked, effort:high
#30  Pick an email provider            in another epic (#25), still open      labels: tracked, effort:low
#40  Fix the Safari login bug          no parent, no sub-issues               labels: tracked, effort:low
#41  Update the licence year           no parent, no sub-issues               labels: tracked            (no effort)
```

What the map shows:

- **World map:** a dungeon for #10, one for #25, and the Tavern. A line leads from #25 to #10, because #30 (in #25)
  blocks #14 (in #10); the line carries the count 1 and a gate at #10.
- **Inside #10:** four rooms in a chain, left to right or top to bottom: #11 (a rat, greyed, green check), then #12 and
  #13 (moderate-tier creatures), then #14 (a high-tier creature; it ends a chain of four, so it gets the boss frame).
  #30 shows as an external stub under the graph, joined to #14 by a dashed line. #12 is ready; #13 and #14 are locked
  but can be read.
- **Tavern:** #40 with a low-tier creature, and #41 with the hollow diamond, because it has no effort label.

## Effort tiers

`trivial`, `low`, `moderate`, `high`, `extreme`. The words are neutral on purpose: they appear on GitHub, where the
team sees them.

How a quest's tier is decided, first match wins:

1. The user's **local override** for that quest (stays on their machine; never written to GitHub).
2. The campaign's **label mapping**: the user maps their own labels (for example `size/S`, `estimate:3`) to the five
   tiers, kept in local state, so a team that already estimates does not have to relabel anything.
3. The **default labels** `effort:<tier>`.
4. Otherwise **unknown**: the hollow diamond. The app never guesses from the title.

Details:

- Label matching ignores case and surrounding spaces.
- The prefix `effort:` can be changed per campaign, for a team that already uses it for something else.
- An issue with **two different** effort labels is treated as a conflict: the room shows unknown effort and the detail
  says which labels clash. The app does not pick one silently.
- An epic's own effort label is shown as its effort word in the detail and the list. The dungeon's art tier on the
  world map is derived from its size (number of quests) and its longest blocker chain, never from a label. A hall's
  header shows its title, progress and effort word; it has no creature of its own.
- **Priority** is a separate signal (see below). It never changes the tier.

## Priority

`priority:low`, `priority:normal`, `priority:high`, `priority:urgent`, mapped per campaign and with a changeable
prefix, exactly like effort. Only high (amber) and urgent (red) draw the small jewel on the room, so a normal repository
is not covered in markers. No label means normal. Two different priority labels are a conflict, shown as in effort.

## Epic hint labels

What an epic is comes from the sub-issue links alone: an issue with sub-issues is an epic, and it stops being one when
its last child is removed. A label never decides that, because a label that disagrees with the links would force the
app to pick a winner.

The labels `epic` and `initiative` are optional hints for a person planning ahead:

- `epic` (or `initiative`) on an issue **with** sub-issues: nothing to say, the shape is right.
- On an issue **without** sub-issues: the issue goes in the Tavern as a stray (the rule above), and the shape hint
  says "marked as an epic, but it has no sub-issues yet". This is how a planned epic is told apart from a plain stray.
- Sub-issues **without** the label: fine, nothing is shown.
- Both names map per campaign like the others (`type: epic`, `theme`, and so on), and neither changes the map.
- An **initiative** that contains epics is just a larger parent: the top-level parent is the dungeon, and the epics
  inside it are halls. There is no extra level on the world map for now.

## Edge cases

| Case | What the map does |
|---|---|
| Blocker cycle (A blocks B blocks A) | Draw both lines, mark the quests as a loop in the detail, and lay them out without failing; none of them can be started |
| A blocker that is already closed | The gate is open (or the line fades); the quest is not locked by it |
| An epic with one sub-issue | Still a dungeon, with one room |
| Nesting deeper than two levels | A top-level epic is the dungeon; every deeper parent is a hall inside it |
| A sub-issue in another repository | Shown as an external stub, as an out-of-scope quest |
| A closed epic with open sub-issues | The dungeon is not cleared: it shows the open count, and the detail notes the mismatch |
| An epic with no sub-issues | A stray: it goes in the Tavern, even if it carries an epic-like label |
| A quest in scope whose parent epic is not | It goes in the Tavern, and its detail says "part of #N, not in scope" (`docs/design/map.md` §10) |
| Labelled `epic` or `initiative` but no sub-issues | Tavern, with the shape hint |

## When the shape is wrong

The map still draws every quest; it never hides one because it is badly labelled. A missing or conflicting effort label
gives the hollow diamond. A small hint above the map counts the problems ("7 quests have no effort label", "2 marked as
epics have no sub-issues"), and clicking it filters the list to them. This is also the list that the issue-writing
skill fixes.

## The issue-writing skill

The skill `issue-shape` (`.claude/skills/issue-shape/SKILL.md`) creates issues in this shape: an epic with sub-issues,
blocked-by links in the right order, an effort label on each, an optional priority, and the scope label when the
repository uses one. It writes through the `gh` shim, one issue at a time, never in a loop. It is the first version,
for this repository's own issues; shipping it to users as a spell comes after the map works (GitHub issue #61). The
worked example above doubles as the sample campaign's fixture and as the skill's template, so there is one source for
all three.
