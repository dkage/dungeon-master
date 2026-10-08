# M3.5: GitHub budget and scope

Status: decided, in progress (phases A and B). Comes after M3 (quests list and detail). It runs in parallel with M3.6
(first run and words, in `PLAN_EXPERIENCE.md`) and finishes before M4. Names for the themed concepts are in
`GLOSSARY.md`; Go packages keep plain names.

Two problems, one cause: Dungeon Master reads GitHub as its database, and GitHub is a rate-limited service shared with
every other tool on the same token (the `gh` CLI, scripts, agents).

1. **We must never get a user locked out.** A bulk run of `gh issue create` over 50+ issues locked the owner out of
   `gh` for roughly 30 minutes. That is GitHub's *secondary* rate limit.
2. **We must not drown the user in issues.** A repository with 300 open issues, of which 12 are the work being driven
   here, must open to those 12.

## Decisions

| # | Decision                                                                                                                                                                                                    |
|---|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| 1 | Storage, when a local copy is needed: **SQLite**, pure Go (`modernc.org/sqlite`, no cgo), in the config directory.                                                                                          |
| 2 | Scope marker is a **label named `tracked`** (neutral, configurable), not a Projects board. Kinds (`roadmap`, `bug`) stay ordinary labels.                                                                   |
| 3 | Pacing defaults: **3 s** between writes (plus jitter), **15 per rolling minute**, **150 per rolling hour**, keep **20%** of the hourly read budget for interactive use. About a quarter of GitHub's limits. |
| 4 | Agent guard: **both** a rule in the agent's instructions and a `gh` shim for sessions the app starts.                                                                                                       |
| 5 | Order: see "Phases". Changed after a review (below); **the part after phase B is awaiting confirmation**.                                                                                                   |
| 6 | Token: `GH_TOKEN`, `GITHUB_TOKEN` or `gh auth token`, as now. A fine-grained token limited to issues is documented as the safer option. **No GitHub Apps** for now (kept in `PLAN_V2.md`).                  |
| 7 | Issue reads and writes use **GraphQL**; **REST only for the free change probe**.                                                                                                                            |

### Review of the first order, and what changed

The first version built five phases (budget, scope, SQLite copy, paced write queue, `gh` shim) before M4. A review of
that plan made these points, which are right:

- It is a lot of infrastructure before the core loop (pick a quest, start an agent) works.
- Bulk writes from the app are a v2 idea; `PLAN.md` says write-back is small and opt-in. The app makes **no** writes
  today, so a write queue has nothing to queue yet.
- The one real defect is M3's label filter running after a 300-issue cut. That is phase B and is small.

Where the review is incomplete, and why the guard stays early: the lockout came from **agents running `gh`**, and an
agent can do that in a session right now. A governor inside the app cannot stop `gh` run by an agent; only the
Gatekeeper (shim) can. It is also small and does not need SQLite. The SQLite copy and the write queue are the parts that
can wait.

So the order is **A, B, E** (with the instruction rule), then M3.6 and M4. **C (Chronicle) and D (Caravan) are
deferred** until a feature needs them (offline use, a large roadmap import). For the first roadmap import an idempotent
import command through the Quartermaster is enough; it needs no durable queue.

## 1. Facts (GitHub docs, checked 2026-10-08, and measured on the owner's account)

Sources: [REST rate limits](https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api),
[GraphQL rate and node
limits](https://docs.github.com/en/graphql/overview/rate-limits-and-node-limits-for-the-graphql-api),
[REST best practices](https://docs.github.com/en/rest/using-the-rest-api/best-practices-for-using-the-rest-api).

| Limit                            | Value                                                                                                                                                                                                  |
|----------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Primary, REST (`core`) / GraphQL | 5,000 per hour each, **separate buckets**. A GraphQL query costs `ceil(connection requests / 100)`, minimum 1                                                                                          |
| Secondary: concurrency           | no more than 100 concurrent requests                                                                                                                                                                   |
| Secondary: points per minute     | 900 per REST endpoint, 2,000 for GraphQL. Query = 1 point; mutation / POST / PATCH / PUT / DELETE = 5                                                                                                  |
| Secondary: content creation      | about **80 per minute and 500 per hour** (issues, comments, ...), the same on REST and GraphQL                                                                                                         |
| Pacing advice                    | **serial requests; at least 1 s between mutating requests**                                                                                                                                            |
| On a limit                       | `retry-after`: wait that long. `x-ratelimit-remaining: 0`: wait until `x-ratelimit-reset`. Otherwise at least 1 minute, then exponential backoff. Retrying while limited can get an integration banned |
| Conditional requests             | `304` does not count against the primary limit (REST). Measured: `core used` did not move                                                                                                              |
| Change latency                   | measured: after a comment on an in-scope issue, the conditional probe flipped from 304 to 200 within about 10 s (`Cache-Control: max-age=60`)                                                          |

**What `gh` uses** (traced with `GH_DEBUG=api`): every `gh issue` command is GraphQL (`POST /graphql`), not REST, and
multiplies requests with lookups. `create` is an info query plus the `createIssue` mutation (more lookups per `--label`
or `--assignee`); `edit --add-label` is 3 requests; `comment` and `close` are 2. Only `gh api repos/...` is REST. It
paces nothing and caches nothing, so a loop of 50 issues sends 150 to 200 requests, 50 of them content-creating, as fast
as it runs. GraphQL has every mutation we need (`createIssue`, `updateIssue`, `closeIssue`, `reopenIssue`, `addComment`,
`createLabel`, `addLabelsToLabelable`, `addSubIssue`, `addBlockedBy`, `removeBlockedBy`).

**Visibility.** The primary budget is visible and free: `x-ratelimit-*` headers on every response (they include what
`gh` and other tools used on the same token), `GET /rate_limit`, and the GraphQL `rateLimit` field. The **secondary
limits are not exposed**, so we count our own writes and react to `retry-after`.

## 2. Quartermaster: one gatekeeper in front of all GitHub traffic (phase A)

An `http.RoundTripper` that every GitHub request of the app goes through, so no caller can bypass it.

- **Serial**: one request in flight.
- **Rations**: records `limit / remaining / reset / resource` from every response. Floors by request class: background
  reads stop below 20% remaining, interactive reads below 5%, writes below 10%.
- **Camp (circuit breaker)**: on `403/429` with `retry-after`, `x-ratelimit-remaining: 0`, or a secondary-limit message,
  stop **all** requests, wait `max(retry-after, reset, 60 s)`, double on repeats (cap 1 h), persist across restarts,
  tell
  the user. No retry storm.
- **Visibility**: `GET /api/github` returns the rations and the camp, for a small chip in the quest panel.
- **Reads only for now.** The app makes no writes today, so the write pacing described in decision 3 is built with the
  Gatekeeper (phase E), the only thing that writes. The Quartermaster keeps a `Reserve` hook for that.
- State (the camp) lives in a small private JSON file; it moves into SQLite if and when phase C happens.

## 3. Scope (phase B)

### The defect fixed first

M3 fetches the 300 most recently updated open issues and applies the campaign's label filter afterwards. In a large
repository, tracked issues outside those 300 never appear. **Scope becomes part of the GitHub query.**

### Model

Per campaign, stored locally:

| Mode                         | Meaning                                          | Query                                                                     |
|------------------------------|--------------------------------------------------|---------------------------------------------------------------------------|
| `labels` (default `tracked`) | issues carrying any of the chosen labels         | `issues(labels: [...])`, one query per label, merged (GitHub ANDs a list) |
| `assigned`                   | open issues assigned to the token's user         | `filterBy: {assignee}`                                                    |
| `all`                        | everything, paged and capped, with the cap shown | paged                                                                     |

Kind is separate: the panel shows **type chips** (`roadmap`, `bug`, ...) built from the labels of the in-scope issues.
A bug found mid-roadmap is still `tracked`.

### First connection

A campaign with no scope yet shows one question, with counts that cost a few points: "312 open issues. Show: tracked
(0) · assigned to me (3) · pick labels (list with counts) · everything (312)". If the marker label has items it is
preselected. The header always reads **`tracked · 14 of 312 open`**, with one click to widen to everything.

### Creating issues

Issues the app creates later get the marker label automatically (created once, with confirmation, if missing). Adopting
an old issue means adding the label, in GitHub or from the panel. Several repos in one workspace later is a set of
repos sharing the marker.

## 4. Gatekeeper: agents cannot bypass the Quartermaster (phase E)

Skills such as `to-tickets` and `wayfinder` tell an agent to publish many issues with `gh`. That bypasses the app but
shares the token, and it is possible today.

- **Rule** in the instructions of every hero session (cheap, not enforceable): create issues one at a time; never loop
  `gh issue create`; a delayed command is intentional.
- **`gh` shim** first on the `PATH` of sessions the app starts. Read-only commands pass straight through. Commands that
  create or change things first ask the app for a **slot** (same pacing, same camp), then run the real `gh`.
  **Fail-open**: an unrecognised command, or an unreachable app, never blocks work. A per-session token limits what the
  shim may call.
- **Write pacing lives here** (decision 3): at least 3 s between writes (plus 0 to 1 s jitter), at most 15 per rolling
  minute and 150 per rolling hour, every write counted as content-creating, counters persisted so a restart cannot reset
  them, and a write is refused while the Quartermaster is in camp.

## 5. Deferred: Chronicle (phase C) and Caravan (phase D)

Kept here so the design is not lost; built only when a feature needs them.

**Chronicle**, a SQLite copy of the tracked issues (tables for repos, issues, labels, edges, comments, sync state).
Sync per repo: a free conditional `GET /repos/{o}/{r}/issues?labels=tracked&sort=updated&per_page=1` with the last ETag
(304 means nothing changed), then a GraphQL query for issues updated since a cursor when it did change, plus a cheap
reconcile pass (numbers and `updatedAt`) on first connect, scope change and about daily. Triggers: an interval (60 s
when looked at, 5 min idle), window focus, a finished session, starting a hero on a quest, the refresh button. Needed
for offline use or many campaigns; today the in-memory cache costs about 60 points per hour per active campaign.

**Caravan**, a durable queue of paced writes for bulk work: a plan of operations (create, edit, comment, close, link)
with local refs, previewed with an estimate and run with pause, resume and cancel; one worker through the Quartermaster;
durable states so a crash resumes; a duplicate guard after uncertain failures; cached ids so a create is one request.
Needed when the app itself creates issues in bulk.

## 6. Tests

- A **fake GitHub** that simulates primary headers counting down, `403` with `retry-after`, the secondary-limit
  message, content-creation caps, `304`, and slow and failing responses; a **fake clock**.
- Property test: for any stream of writes the Quartermaster never exceeds its caps in any rolling minute or hour, never
  has two requests in flight, and always stops on a limit signal.
- Scope tests: a tracked issue in a repository with more than 300 newer issues is still found; the picker counts.
- Shim tests: command classification, pass-through of exit code, output and stdin, fail-open.
- Browser tests: the first-connection picker, the scope header, the camp banner and rations chip.
- `make test-linux` for everything that touches processes.

## 7. Phases and status

| Phase            | Contents                                                                                                   | Status               |
|------------------|------------------------------------------------------------------------------------------------------------|----------------------|
| A. Quartermaster | RoundTripper, rations from headers, floors, serial, camp, persistence, `/api/github`, UI chip (reads only) | in progress          |
| B. Scope         | server-side scope, counts, first-connection picker, scope header, type chips, 300-cut fix                  | planned              |
| E. Gatekeeper    | instruction rule, `gh` shim with a per-session token, and the write pacing                                 | planned, **confirm** |
| C. Chronicle     | SQLite copy and sync                                                                                       | deferred             |
| D. Caravan       | paced bulk writes                                                                                          | deferred             |

The first roadmap import goes through the Quartermaster (an idempotent import command), after phase B.
