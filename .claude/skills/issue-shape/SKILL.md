---
name: issue-shape
description: Create, revise or audit this repository's GitHub issues with bounded scope, completion evidence, effort labels and native parent/dependency links. Use when planning work as issues or changing their relationships.
---

# Issue shape

Read `CLAUDE.md` and `docs/design/issue-shape.md`. GitHub owns identity, status, assignees, parents and dependencies;
issue prose describes intent and evidence. This skill does not authorize publishing an unrequested plan.

## Prepare the change

- Inspect existing open and relevant closed issues before creating anything. Reuse issue identities; preserve useful
  comments, parent links and unrelated scope. A title match alone is not proof of a duplicate.
- Decide whether each item is a bounded task, research question, disposable prototype, human decision or parent epic.
  An epic needs native sub-issues; a label alone does not establish membership. Separate unsettled decisions from
  implementation that depends on them. Research should resolve a precise question, not demand an implementation.
- Draft Goal, Scope and Done when in that order. Add Context, Out of scope or Open questions only when they materially
  clarify the work. For decisions, state the choice the human must settle and the evidence needed. Do not answer it
  yourself. For prototypes, state what is disposable and how feedback answers the question.
- Pick one `effort:*` tier based on scope and uncertainty: trivial, low, moderate, high or extreme. Pick one `type:*`
  method for new/revised executable items: task, research, prototype or decision. These labels are independent of
  feature/bug category and priority. Normal priority needs no label. Scope labels use existing repository conventions;
  do not create `tracked` merely to make this skill work.
- Native blockers mean a prerequisite is required, not merely related or scheduled earlier. Check identity, repository,
  closure reason and transitive edges; reject self-links and cycles. Do not link milestones sequentially just to show
  roadmap order. Use the roadmap for prioritization and blockers for actual dependencies.
- Check available labels once. Create only the missing labels needed for the authorized change. Preview the operation
  list for a substantial reorganization; an explicit request to reorganize the backlog already authorizes those edits.

## Publish carefully

Write each exact body to a file, then pass `--body-file`. Use neutral wording in titles, bodies, labels and comments.
The final Plan line points to an existing repository-owned design/release file. No private-source names or quotations.

All writes go through the application's live `gh` gatekeeper: one command at a time, no mutation loops or parallel
writes. Inspect quota before a substantial change and avoid repetitive read polling. If the gatekeeper is unavailable,
prepare local bodies and report the publishing limitation; do not bypass it. Exit 75 or a GitHub rate-limit response
means stop and report, with no automatic retry. Do not add a second approval request when this session already
explicitly authorized the operation.

```sh
gh issue create --title "Record the session recovery decision" \
  --label enhancement --label type:research --label effort:moderate --body-file /path/body.md
gh issue edit NUMBER --title "..." --body-file /path/body.md
```

Create a parent before its children. Capture each returned URL/number and REST numeric ID immediately; keep a local
operation journal for multi-step work. Attach children in useful order, then add dependencies only after both endpoints
exist. GraphQL node IDs are not REST numeric IDs.

```sh
gh api repos/OWNER/REPO/issues/NUMBER --jq .id
gh api -X POST repos/OWNER/REPO/issues/PARENT/sub_issues -F sub_issue_id=CHILD_REST_ID
gh api -X POST repos/OWNER/REPO/issues/NUMBER/dependencies/blocked_by -F issue_id=BLOCKER_REST_ID
```

If a write's outcome is uncertain, read that specific issue/link before deciding whether another write is necessary.
Resume from recorded successes, not from the start. A failed link leaves a partial plan to report and repair; never
silently create replacement issues. Read/review again before overwriting an issue that changed during this task.

## Verify and report

Use one bounded graph read where possible to verify changed parent/dependency links, method and effort labels, duplicate
work and cycles. For REST-only checks, inspect the affected parent's `sub_issues` and issue's `dependencies/blocked_by`.
Report number, title, effort, method, parent, blockers and any incomplete operation.

GitHub issue status does not establish implementation status. Distinguish planned, local implementation, checks passed,
committed, published and closed. Do not close an issue just because a file exists or a process exited. Record completion
or cancellation with evidence and caveats before using the repository's normal closure procedure. Canceled work may
invalidate downstream dependencies; revise those edges explicitly rather than treating cancellation as fulfillment.
