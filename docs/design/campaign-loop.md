---
status: planned
milestone: campaign workflow
github_issues: [61, 67, 68, 71]
tracking: in-gh-issues
---

# Plan, choose, launch, steer and review

A campaign helps the operator clarify work, choose a ready issue, launch a reviewed agent session, answer interruptions
and inspect the result. GitHub owns issue identity and relationships. The app's RPG presentation explains those facts;
it does not introduce a second tracker, automatic scheduler or scores. These additions are planned, not built.

## Choosing work and attention

Reuse the existing map/list view-model for Ready work and Needs input entry points. Explain why a visible issue cannot
start and link its prerequisites; selecting a waiting item opens its existing session. Method, effort, assignment,
readiness and local session occupancy are separate facts. Assigned issues remain readable and may be deliberately
started with the existing assignment notice; no silent assignment or claim is made on GitHub.

A dependency is satisfied by a completed prerequisite, not merely by a stopped agent. Preserve GitHub close reasons:
closed as not planned is a boundary decision, not implemented work. A canceled prerequisite requires reviewing/removing
or replacing that edge explicitly. Missing/inaccessible prerequisites are unknown, not satisfied. Preserve repository
identity for external references; an issue number alone is not a global key. Cycles remain visible and prevent launch
of affected work. These stronger rules need implementation and fixtures before replacing the current open/closed rule.

Launch rechecks issue readiness, tree occupancy and method identity. A changed prerequisite or source invalidates the
reviewed launch with a concrete reason; the app does not silently launch a different plan. Evidence of blocker
completion is useful briefing context, with links and bounded excerpts; missing evidence must be called unknown.

## Planning before issues exist

A campaign can start a free planning session from an operator goal. Choose a method: clarify a human decision,
investigate evidence, explore a disposable prototype, or perform a concrete task. Method guides how work happens,
not what is ready. Draft artifacts live in local user state and are clearly distinguished from GitHub-backed issues.
Planning works without a GitHub connection; publishing does not.

A proposal states goal, constraints, completion criteria, explicit exclusions and remaining decisions, then a set of
bounded work items with native parent/dependency relationships, effort and method. Do not invent answers to unsettled
human decisions or publish implementation issues that pretend unknown requirements are settled.

Publishing is a separate, explicit action using the issue-writing workflow. Preview additions and edits first, keep
existing issue identities, and record each successful write/link so interruption resumes without duplicates. Writes
are paced, one at a time. Build the durable bulk queue only when app-managed bulk publishing actually needs it.

## Outcome handoff

Process exit, issue closure and PR merge are distinct. After a session, show its actual outcome (finished, failed,
killed or lost), its issue and linked PRs, and available next actions: inspect the result, open a PR, launch an available
review method, continue work, or return to ready work. Unavailable methods explain why. A success exit never closes an
issue, posts a comment or asserts that acceptance criteria passed.

Resolution records contain the result/decision, relevant evidence and remaining caveats. Canceled work records the
reason and affected dependencies. A later finding that invalidates a prior decision is explicitly linked and reviewed;
do not reopen or rewrite past decisions automatically. Conversation resume and live PTY reattachment are separate
features; offer only capabilities verified by the session-lifecycle plan.

## Verification

Use unit tests for readiness, references, cycles and outcome actions; HTTP tests for stale launch checks; browser tests
for ready-work selection, waiting-session navigation, local planning and review. A fresh-state real-work walkthrough
must cover the complete loop and record platform, CLI version, result and remaining limitations. Source files and
passing stubs alone do not establish real-CLI compatibility.
