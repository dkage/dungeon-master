---
status: current
milestone: delivered cockpit and planned campaign workflow
github_issues: [28, 43, 45, 48, 52, 63, 67, 68, 69, 70, 71]
tracking: in-gh-issues
next: review/publish the committed map, then focused workspace views, sources and trustworthy attention
verified: 2026-10-13
---

# Cockpit experience

A local development session should feel like directing a campaign: see the goal, choose useful work, launch with clear
context, answer agents when needed and review the result. The terminal remains the place to steer the agent. The map
makes relationships understandable; the theme represents real work and never substitutes flavor for an actionable error.

## 1. Baseline and evidence

The cockpit shell, first-run flow, issue launch and heuristic attention are delivered through M4.5. The map is published; review fixes are committed locally,
with automated checks passed, and #28 remains open pending #126 and #127. Read `docs/roadmap.md` for the dated status audit. Earlier descriptions of an
empty launcher, a missing map scheduled last, or milestones waiting for a push were historical and are superseded.
The Stitch v4 assets remain the visual baseline; behavior is specified by current design docs rather than generated HTML.

## 2. Principles

- First value before terminology: get to a useful live session without a setup manual.
- One obvious next action, with explicit reasons when blocked. Do not hide work because it is unavailable to launch.
- Keep sample, default and real data separate. Sample creation and shared-world writes are explicit actions.
- The RPG theme carries goal, effort, dependencies, occupancy and attention. Avoid scores, extra plumbing nouns and
  decoration that makes actual work harder to read. Plain mode remains equivalent and usable.
- Own a small product: GitHub and CLI agents already provide tracking and execution. No workflow platform or second tracker.
- Validate by real use. Automated stubs prove behavior boundaries; dated manual probes prove live compatibility.

## 3. Workspace and activities

Default: projects/sessions on the left, selected terminal in the centre, map/list above selected issue detail on the
right. Planning, running and reviewing use the same workspace state. A visible Expand control lets any of those four
regions fill the workspace; Restore split returns with sessions, selections, filters and geometry preserved. Detailed
focus, keyboard and responsive behavior is in `docs/design/workspace.md`; these controls are planned in #63.

The map and list show the same facts and selection. Selecting an issue is reading; only explicit reviewed launch starts
an agent. Double-click enters a parent/Tavern, never launches a room. Selecting an existing session should expose its
terminal; reading a blocker should expose its detail instead of changing process ownership.

## 4. Work order and exit criteria

`docs/roadmap.md` is the sole ordered milestone table. Review/publish the committed map, add workspace focus/shortcuts, implement
registered source-backed skills, verify hooks/attention, add ready-work/planning, settle session continuity and add the
outcome handoff. Packaging consumes the lifecycle decision. Extra map presentation follows usable daily workflow.
Do not duplicate milestone status tables here.

Each increment needs a fresh-state walkthrough and unit/feature coverage. Finish the map only after its actual issues,
list parity, blockers, art fallback, keyboard paths and launch behavior have been verified; local files alone are not
an exit test. Desktop-window reattachment and backend restart survival have separate acceptance criteria.

## 5. First-run experience

### 5.1 Start here

Guide the user through finding Claude, registering a project, choosing existing GitHub scope, reading an issue and
reviewing launch. Keep repository suggestions and typed path alongside the operating-system picker. A folder without
GitHub can still run a free session and, when implemented, a planning session; say why its issue list is empty.

### 5.2 Sample campaign

Sample is a separate, explicitly created campaign with a persistent badge and removable generated workspace. Never offer
to replace real issues with sample data when a GitHub request fails. Sample traffic does not consume GitHub budget.

### 5.3 Starter heroes

Built-in Claude configurations are visibly defaults until the user chooses to save them locally. Settings edits models,
args/env and the default hero. Preserve error messages/last good configuration for malformed files. Installing a skill
source or choosing a method is separate from agent execution settings and never silently changes permission flags.

### 5.4 Empty and unavailable states

State what belongs here, how to create it and why it is useful. Missing credentials/source/method or stale data includes
a concrete reason and recovery action. Never assert that a registered external skill is invocable until verified.

## 6. Words and help

`web/src/lib/words.ts` supplies themed names, plain meanings and definitions; `Term` supplies inline help. Use themed,
themed-with-hints and plain words consistently. Introduce concepts at use, not in a vocabulary tutorial. Error, access,
rate-limit and execution-permission messages stay plain. Avoid adding new public names for internal plumbing.

## 7. Presentation and interactions

### 7.1 Visual baseline

Keep the existing gothic Dungeon and calm Neutral presentations. Terminal content remains real xterm output, never a
structured imitation of a CLI transcript. Prototype focused layouts at ordinary window sizes before elaborating art.
The terminal gets a calm, readable area; conventional controls remain dense and accessible.

### 7.2 Typography and tokens

Use the existing bundled font choices and independent panel sizes specified in `docs/design/ui.md` §6.5. Preserve
terminal monospace, Portuguese coverage, local font licenses, contrast and reduced-motion behavior. New focus controls
use the current tokens rather than introducing a new skin or generic theme framework.

### 7.3 State language

Keep effort, method and state separate. Ready, blocked, assigned elsewhere, locally running, needs input and closed have
text/shape as well as color. Assigned is not proof of a live local agent. Completion and cancellation will become
separate outcomes in #67; current closed-state rendering must not be described as having that distinction already.

### 7.4 Map and workflow

`docs/design/map.md` specifies levels, selection, relations and creature rendering. `docs/design/campaign-loop.md`
specifies ready-work, planning and outcome review. No automatic issue claiming, closure or dispatch follows navigation.

### 7.5 Controls and shortcuts

Quest launch belongs in the selected detail's reviewed footer. Free sessions belong to the project/session controls;
existing toolbar buttons can remain until the prototype parity pass consolidates them. Settings contains configurable
Shortcuts (#65), and buttons/keys use shared semantic actions. A command palette is later work over those actions,
not a second dispatcher. Terminal/text input, IME, browser reservations and modal priority follow the workspace spec.

## 8. Documentation discipline

Plans state behavior and decisions. The roadmap records priority and dated delivery evidence; GitHub owns remaining
work and relationships. Frontmatter tracks the applicable issues. Do not claim pending push without actual ahead commits,
or completion from a successful process exit. When a design's milestone is fully closed, retain its behavior in the
appropriate current contract and remove the obsolete plan per `CLAUDE.md`.

## 9. Real use

Exercise the whole loop on actual development: plan a goal, select ready work, inspect context, run a session, respond
to an interruption, review its result and choose the next action. Record friction and verification boundaries rather
than counting screens or feature parity. Native platforms and real CLI behavior need their own dated evidence.

## 10. Approved direction

Keep the implementation and visual identity; revise priority and scope now. Registered sources mapped to projects,
expandable panels, configurable shortcuts, trustworthy attention, planning and review are explicit future increments.
General plugins, arbitrary nested panes, another tracker and a framework rewrite remain outside this direction.
