# M3.5: GitHub budget, local mirror and scope

Status: plan, nothing built. Comes after M3 (quests list and detail) and before the roadmap goes into issues and before M4.

Two problems, one cause: Dungeon Master is about to treat GitHub as its database, and GitHub is a rate-limited service shared with every other tool on the same token (the `gh` CLI, scripts, agents).

1. **We must never get a user locked out.** A bulk run of `gh issue create` on 50+ issues locked the owner out of `gh` for roughly 30 minutes. That is GitHub's *secondary* rate limit. Anyone who maps a whole backlog through this app, or through an agent running a loop, can hit it.
2. **We must not drown the user in issues.** A repository with 300 open issues, of which 12 are the roadmap, must open to those 12, not to 300.

## 1. Facts (GitHub docs, checked 2026-10-08)

Sources: [REST rate limits](https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api), [GraphQL rate and node limits](https://docs.github.com/en/graphql/overview/rate-limits-and-node-limits-for-the-graphql-api), [REST best practices](https://docs.github.com/en/rest/using-the-rest-api/best-practices-for-using-the-rest-api).

| Limit | Value |
|---|---|
| Primary, REST | 5,000 requests per hour per user token |
| Primary, GraphQL | 5,000 points per hour per user; a query costs `ceil(connection requests / 100)`, minimum 1 |
| Secondary: concurrency | no more than 100 concurrent requests (REST + GraphQL together) |
| Secondary: points per minute | 900 per REST endpoint, 2,000 for GraphQL. GET/query = 1 point, POST/PATCH/PUT/DELETE or a mutation = 5 |
| Secondary: CPU | 90 s per 60 s real time (60 s for GraphQL) |
| Secondary: content creation | about **80 per minute and 500 per hour** (creating issues, comments, ...). Some endpoints lower. Web UI actions count too |
| Undisclosed | "too many compute-heavy requests" is limited with no published number; limits can change without notice |
| Pacing advice | **make requests serially, not concurrently; wait at least 1 s between mutating requests** |
| On a limit | `retry-after` header: wait that long. `x-ratelimit-remaining: 0`: wait until `x-ratelimit-reset`. Otherwise wait at least 1 minute, then back off exponentially. Retrying while limited "may result in the banning of your integration" |
| Conditional requests | an ETag request answered `304` does not count against the primary limit (REST only; GraphQL has no ETags) |
| Webhooks | recommended over polling; not available to a local app |

## 2. Can we see our usage? Partly.

| What | Visible? | How |
|---|---|---|
| Primary budget (REST `core` and `graphql` are separate buckets) | **Yes, free and authoritative** | `x-ratelimit-limit / remaining / used / reset / resource` headers on *every* response; `GET /rate_limit`; the GraphQL field `rateLimit { cost remaining resetAt }` |
| Usage by other tools on the same token | **Yes**, same counters | the headers show what is left for the whole account, so `gh` and scripts are already included |
| Secondary limits (points/min, content creation per minute/hour, CPU) | **No counters are exposed** | we must track our own writes, and react to `403/429` with `retry-after` |

Verified against the owner's account: `GET /rate_limit` returned identical numbers on two consecutive calls (it is free); a GraphQL response carried `X-Ratelimit-Limit: 5000`, `Remaining`, `Reset`, `Resource: graphql`, and `rateLimit { cost: 1 }`.

So: **read the primary budget from the response headers of every request (no extra calls, no own counting), and keep our own persisted counters only for what GitHub does not show: our writes per rolling minute and hour.** The self-limit must be far below GitHub's, because other tools share the budget and the content-creation numbers are approximate.

## 2b. What the `gh` CLI actually uses (measured)

Traced with `GH_DEBUG=api` against the owner's repository on 2026-10-08.

| Command | Requests it makes | API |
|---|---|---|
| `gh issue list` | 1 query (`IssueList`) | GraphQL |
| `gh issue view` | `IssueByNumber` (+ a second query for details) | GraphQL |
| `gh issue create` | `IssueRepositoryInfo` query, then the `IssueCreate` mutation; each `--label`, `--assignee`, `--milestone` or `--project` adds more lookup queries | GraphQL |
| `gh issue edit --add-label` | `IssueByNumber`, `RepositoryLabelList`, then the `LabelAdd` mutation (3 requests) | GraphQL |
| `gh issue comment` / `close` | `IssueByNumber` query, then the mutation (2 requests) | GraphQL |
| `gh api repos/...` | 1 request | REST (a separate `core` budget) |

So `gh issue ...` is **GraphQL, not REST**, and counts against the **GraphQL** budget (5,000 points per hour, 2,000 points per minute secondary, mutations 5 points, plus the content-creation caps). It does no pacing and no lookup caching: a loop creating 50 issues with labels and assignees sends roughly 150 to 200 requests, 50 of them content-creating mutations, in whatever time the loop takes, and each created issue also notifies people. That is the shape of what locked the account.

GraphQL has every mutation we need: `createIssue`, `updateIssue`, `closeIssue`, `reopenIssue`, `addComment`, `createLabel`, `addLabelsToLabelable`, `addSubIssue`, `addBlockedBy`, `removeBlockedBy` (confirmed by introspection). **Dungeon Master will use GraphQL for everything**, so reads and writes share one budget and one accounting model with `gh issue`, and the lookups (repository id, label ids, user ids) are cached in the mirror so a create costs **one** request instead of two to four.

## 3. Design

### 3.1 One governor in front of every GitHub request

All GitHub traffic (reads, mutations, the outbox worker) goes through one component. Nothing else may talk to GitHub.

- **Serial by default**: one request in flight. No concurrency, as the docs advise.
- **Budget awareness**: after every response, record `remaining/limit/reset/resource`. Reads are refused (served from the mirror instead) below a floor (default: keep 20% of the hourly budget; hard stop at 500 remaining). Writes need a higher floor.
- **Write pacing, self-imposed**, defaults deliberately about a quarter of GitHub's limits:
  - at least **3 s** between mutations, with jitter (docs: 1 s minimum);
  - at most **15 mutations per rolling minute** and **150 per rolling hour** (GitHub: ~80 and ~500). Counters persisted so a restart cannot reset them.
  - all mutations counted as content-generating, to be safe.
- **Circuit breaker**: on `403/429` with `retry-after`, `x-ratelimit-remaining: 0`, or a "secondary rate limit" message: stop every write immediately, pause for `max(retry-after, 60 s)`, double on repeats (cap 1 h), persist the pause across restarts, and tell the user. Reads continue from the mirror.
- **Never retry blindly** on a limit signal; no retry storm. Transient 5xx and network errors back off separately.
- **Visibility**: expose `Status{remaining, limit, resetsAt, pausedUntil, pendingWrites, lastSync}` to the UI.

### 3.2 Local mirror (SQLite) and incremental sync

GitHub stays the source of truth. The mirror is a cache plus the outbox, never edited directly.

**Storage decision: SQLite, pure Go (`modernc.org/sqlite`, no cgo), in the config directory, 0600.** Reasons:
- the outbox needs a durable queue with atomic state changes and crash recovery;
- the quest list is relational (parent, blockers, labels, filters, counts);
- incremental sync is upserts by issue number;
- it keeps the release plan intact (the portable binary stays cgo-free); cost is a few MB of binary.
JSON files would be fine for a read-only cache but turn into hand-rolled transactions the moment a write queue exists. Schema versioned with `PRAGMA user_version`.

Tables (sketch): `repos`, `issues` (number, node id, title, state, labels, assignees, parent, milestone, updated_at, fetched_at, in_scope), `edges` (blocked-by), `comments` (lazy), `outbox`, `limiter` (counters, pause).

**Sync, cheapest first:**
- **Steady state**: GraphQL `issues(filterBy: {since: <last sync>})` ordered by `UPDATED_AT`. One request returns only what changed, cost 1. Poll about every 60 s while a window is looking at the campaign, every few minutes otherwise, never when no client has asked recently. That is roughly 60 points per hour (about 1% of the budget) per active campaign.
- **Reconcile**: a lightweight full pass (numbers and `updatedAt` only, 100 per page, a few points) on first connect, when the scope changes, on demand, and when the last full pass is over 24 h old. This catches deletions, transfers and anything `since` cannot see.
- **Details and comments**: fetched when an issue is opened, cached, invalidated when the issue's `updatedAt` moves.
- **Offline / limited**: the app keeps working from the mirror and says how old it is.

**Divergence between our copy and GitHub:** reads are never in conflict (GitHub wins on every sync). Writes carry the `updatedAt` they were based on; if the issue changed on GitHub since, the operation stops in a `conflict` state for the user to decide. Creates cannot conflict.

### 3.3 Outbox and paced worker for writes

For bulk work (importing a roadmap, an agent splitting a task into 30 issues):
- the user (or a skill) produces a **plan**: a list of operations (create issue, edit, comment, close, add sub-issue, add blocked-by, create label), referring to not-yet-created issues by local ids that resolve to numbers once created;
- the UI shows the plan with an **estimate** ("52 issues + 71 links, about 11 minutes"), then runs it with **pause, resume and cancel**;
- one worker drains it through the governor, strictly serial, with the pacing above;
- every operation has a durable state (`pending`, `running`, `done`, `failed`, `conflict`), so a crash or restart resumes where it stopped;
- **no duplicates after an uncertain failure** (timeout, 5xx after the request left): before retrying a create, list recently created issues by the user and match title and body hash, instead of marking issues with hidden text;
- dry-run first: nothing is sent until the user confirms.

### 3.4 Agents can still bypass the governor

Skills such as `to-tickets` and `wayfinder` tell an agent to publish many issues with `gh`. Those calls share the token but not our pacing. Options, from least to most protective:

1. A rule in the injected preamble: create issues one at a time, wait between them. Cheap, not enforceable.
2. **A `gh` shim on the `PATH` of sessions we start**: it forwards to the real `gh`, but mutating commands (`issue create`, `issue edit`, `api -X POST/PATCH/...`) wait on the same persisted pacing and honour the pause. Agents then cannot flood GitHub by accident, from any skill. Reads pass straight through.
3. Both.

Recommendation: do 1 now and 2 as the last phase of M3.5, after the governor exists, since the shim is just another client of it.

## 4. Scope: what appears in the panel

### The defect to fix first
M3 fetches up to 300 open issues (the most recently updated) and applies the campaign's label filter **afterwards, in the browser-side service**. In a large repository the roadmap issues that fall outside those 300 never show up. **The filter must be part of the GitHub query** (and of the mirror's `in_scope` column), not applied to a truncated list.

### Model
Each campaign has a **scope**, stored locally (never in the repository):

| Scope | Meaning | Query |
|---|---|---|
| Labels | issues with any of these labels (default `roadmap`) | `issues(labels: [...])` |
| Assigned to me | open issues assigned to the token's user | `filterBy: {assignee}` |
| Milestone | one milestone | `filterBy: {milestone}` |
| Under an issue | the sub-issue tree of a chosen epic | sub-issues |
| Project board | items of a GitHub Project (needs the `project` scope, later) | `projectV2.items` |
| Everything | all issues | paged, on demand |

### First connection (the "300 issues" case)
Never open to nothing and never to everything. On first connection to a repository the panel asks one question, with counts that cost a few points in total:

> This repository has 312 open issues. Show:
> - issues labeled **roadmap** (14)  ← preselected when such a label exists
> - issues assigned to me (3)
> - pick a label (list with counts)
> - everything (312)

If no label looks like a roadmap, preselect "assigned to me" when it has items, otherwise ask without a default. The choice is stored per campaign.

The panel header always shows the scope and the total: **`roadmap · 14 of 312 open`** with one click to widen to "All" (paged, filter text sent to GitHub rather than loading everything) and back.

### The pattern for creating issues
Issues created by Dungeon Master (through the outbox) carry the scope label automatically, creating the label once if it is missing (one mutation, with the user's confirmation). Skills that label their own issues (for example `wayfinder:*`) are covered by letting a scope list several labels. The label name is neutral (`roadmap`), because the theme must not leak into GitHub.

## 5. Phases

| Phase | Contents | Why this order |
|---|---|---|
| **A. Governor** | single request path, header-based budget tracking, floors, serial execution, `retry-after` handling, circuit breaker, UI chip (remaining, reset time, paused banner) | protects every other phase and the owner's account immediately |
| **B. Scope** | server-side filtering, counts, first-connection picker, "All" toggle, header total; fix the 300-issue truncation | fixes a real defect, and the roadmap needs it |
| **C. Mirror** | SQLite, schema and migrations, incremental sync, reconcile, stale/offline behaviour, sync age in the UI | makes polling nearly free and survives limits |
| **D. Outbox** | operations, plan preview with estimate, paced worker, pause/resume/cancel, crash recovery, duplicate protection | the bulk-write path; the roadmap import is its first real user |
| **E. Agent guard** | `gh` shim for sessions (decision below) | closes the bypass |

Roadmap issues go in **after D**, created through the outbox, so the first real use of the app on itself is also the first test of the safe bulk path.

## 6. Tests

- A **fake GitHub** that simulates: primary headers counting down, `403` with `retry-after`, the secondary-limit message, content-creation caps per minute and hour, `304` on conditional requests, slow and failing responses.
- A **fake clock**, so an hour of simulated traffic runs in milliseconds.
- Property test: for any queue of writes, the governor never exceeds its own caps in any rolling minute or hour, and always stops on a limit signal.
- Crash tests: kill the worker between "request sent" and "result stored"; on restart there are no duplicates and no lost operations.
- Sync tests: a deleted or transferred issue disappears after a reconcile; a changed one updates; an unchanged poll costs one point.
- Scope tests: a roadmap issue in a repository with more than 300 newer issues is still found.
- Browser test: the first-connection picker, the "All" toggle, the paused banner, plan preview and progress.
- `make test-linux` covers SQLite on Linux.

## 7. Decisions needed

1. **SQLite** for mirror and outbox (recommended), or JSON files and no outbox for now?
2. **Scope default**: the first-connection picker with a `roadmap` label as the suggested pattern (recommended), or a different default label or mechanism? Projects v2 stays a later option.
3. **Pacing defaults**: 3 s between writes, 15 per minute, 150 per hour, keep 20% of the hourly read budget. Too slow, too fast?
4. **Agent guard**: preamble rule now and the `gh` shim in phase E (recommended), or the rule only?
5. **Order**: A, B, C, D, E as above, or move B (scope) first because it fixes a defect?
