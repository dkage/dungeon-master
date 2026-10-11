---
name: issue-shape
description: Create or change GitHub issues for this repository in the shape the Dungeon Master map reads - an epic with sub-issues, blocked-by links in order, an effort label on every issue, optional priority, plain neutral wording. Use whenever creating a GitHub issue, epic, sub-issue or blocker link, or when asked to plan work as issues.
---

# Issue shape

Every issue this repository gets must be readable by the map (`docs/design/map.md`). The contract is
`docs/design/issue-shape.md`; read its "What the app reads" table before writing. This skill is the procedure.

## Before creating anything

1. **Is it an epic or a quest?** An epic groups several quests and gets sub-issues. A quest is one piece of work.
   A single stray task (a bug fix, a chore) has no parent and no children, and that is fine: it lands in the Tavern.
2. **Pick the effort tier** for every issue, with no exceptions. Judge the work, never the title:

   | Tier | Use when |
   |---|---|
   | `effort:trivial` | tiny, well-scoped change, one place |
   | `effort:low` | small, bounded change |
   | `effort:moderate` | several steps or a few related files |
   | `effort:high` | broad change, real uncertainty, or many dependent tasks |
   | `effort:extreme` | a large initiative or a boss-sized piece with a long chain |

   If you cannot judge it, say so and ask; do not leave the label off silently (an unlabelled issue shows as unknown).
3. **Priority** only when it is not normal: `priority:high` or `priority:urgent`. Otherwise add no label.
4. **Order of work:** list which existing issues block this one. Create blockers first so the links can point at
   them.
5. **Check the labels exist**: `gh label list --limit 100`. Create any that are missing, one at a time, for example
   `gh label create effort:moderate --color 6f42c1 --description "Effort tier: moderate"`. Use the same colour for the
   five effort labels and one for the two priority labels.

## Writing the issue

- Title: plain, imperative, no issue numbers, no themed words (no quest, dungeon, hero, spell and so on). Anything
  written to GitHub uses neutral wording.
- Body, in this order: `## Goal` (one or two sentences), `## Scope` (bullets), `## Done when` (checkable bullets), and
  a last line `Plan: <path to the doc>` using a real path under `docs/` (check it exists).
- Labels: `enhancement` or `bug`, plus exactly one `effort:*`, plus `priority:*` if not normal, plus the scope label
  (`tracked`) only if this repository already has it. Add `epic` (or `initiative` for an epic of epics) to an issue
  that is planned as an epic, even before its sub-issues exist.
- Put the body in a file and use `--body-file`; never build the command through a shell string.

```
gh issue create --title "..." --label enhancement --label effort:moderate --body-file /path/body.md
```

## Linking (REST needs the issue id, not the number)

```
id=$(gh api repos/OWNER/REPO/issues/N --jq .id)
# make N a sub-issue of epic E
gh api -X POST repos/OWNER/REPO/issues/E/sub_issues -F sub_issue_id=$id
# make N blocked by B
gh api -X POST repos/OWNER/REPO/issues/N/dependencies/blocked_by -F issue_id=$(gh api repos/OWNER/REPO/issues/B --jq .id)
```

Get OWNER/REPO from `gh repo view --json nameWithOwner --jq .nameWithOwner`. Epic first, then its children, then the
blocked-by links, children in dependency order.

## Writes are paced

All `gh` writes go through the app's gatekeeper: **one write at a time, never a loop.** Create an issue, wait for it to
return, then link it, then the next. If a command is refused or exits 75 (GitHub asked us to slow down), stop, report
it, and do not retry in a loop. For a large batch (more than about ten issues), show the plan first and ask.

## After creating

Verify and report, in a table: number, title, effort, blocked by, parent. Check the links with
`gh api repos/OWNER/REPO/issues/E/sub_issues --jq '.[].number'` and
`gh api repos/OWNER/REPO/issues/N/dependencies/blocked_by --jq '.[].number'`. Say plainly if any link or label failed.

## Do not

- Do not mark an issue as an epic by label alone: sub-issue links decide what an epic is.
- Do not edit or relabel existing issues in bulk without being asked.
- Do not put themed words, effort jokes or internal plan names in titles or bodies.
