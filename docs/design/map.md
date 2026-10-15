---
status: current
milestone: M5a, M6
github_issues: [28, 44, 64, 67]
tracking: in-gh-issues
delete_when: #44 closes
---

# Map and right panel: design

Design for the map (M5a and M6 in `docs/design/experience.md` §4) and the right-hand panel around it. The right-column
composition is decided and its shell belongs in M3.6; M5a implements the graph and sprite-backed rooms inside the
reserved map region.

The Stitch v4 prototype is the visual reference for the shell. This document specifies map behavior; `docs/design/ui.md`
specifies the surrounding controls. Where the generated prototype differs from those behavior plans, the plans win.

## 1. What we want

The parts of the experience we want:

- The map has three levels (§3, Levels). The **world map** shows every top-level epic as a **dungeon** and the
  **Tavern** for quests that belong to no epic; lines between dungeons keep the order of work. Opening a dungeon shows
  its quests as rooms (the Stitch v4 screen). The **List** is a separate flat view.
- In a dungeon, every quest is a room. A known effort tier selects an original creature sprite from
  `docs/design/creature-roster.md`; creatures being worked in a terminal look different (brighter, warmer).
  Blocked-by and parent links are lines between rooms.
- Clicking a point highlights it (brighter, its links emphasised) and shows the quest in the lower part of the right
  panel. The split between map and detail can be resized.
- The bottom of the detail has pinned launch controls: a hero/model picker, a skill picker defaulted to the
  recommendation (with None available), and Start. The selected skill runs as the session opener.

Constraints:

- **No game engine.** M5a uses Svelte, SVG, CSS, a layout library, and the generated creature sheets in the Dungeon
  theme. Keep a plain SVG fallback for unknown effort, missing assets, Neutral mode, and reduced motion.
- Creatures communicate **effort**; light, shape, and glyphs communicate **state**. Never make the creature species
  carry blocked, active, assigned, or cleared state.
- Readable first: a user must understand the state of a repository at a glance, in both theme modes.
- Works with the plain-words mode (`docs/design/review-findings.md` §9): the map has a plain rendering too.
- Fast enough for about 300 quests, accessible by keyboard and screen reader, testable without a browser for the
  layout logic.

## 2. Layout: map on the right, terminal untouched

```
┌──────────────┬──────────────────────────────────┬──────────────────────────────┐
│ CAMPAIGNS    │ [ #42 Validate emails ][ #17 ]   │ MAP            [Map | List]  │
│  repo A      │                                  │   ○───○───●                  │
│   ● #42      │  terminal (full height, always   │    \     /                   │
│   ○ #17      │  visible)                        │     ○───◌ (locked)           │
│  repo B      │                                  │                              │
│              │                                  │ ════════ drag to resize ════ │
│              │                                  │ #42 Validate email addresses │
│              │                                  │ open · feature · blocked by… │
│              │                                  │ (markdown body, scrolls)     │
│              │                                  │                              │
│ + Add        │                                  │ [Opus ▾] [/tdd ▾] [▶ Start]  │
└──────────────┴──────────────────────────────────┴──────────────────────────────┘
```

The controls around the map (header, detail, footer, settings, shortcuts) are specified in `docs/design/ui.md`; this section
keeps only the layout decision.

This keeps the selected terminal visible while the user reads the map and quest detail, matching the Stitch cockpit
and keeping the three regions in stable roles.

- **Right column**: map on top, quest detail below, a draggable splitter between them. Both sizes are remembered per
  window. The column itself is resizable (about half the window on a wide desktop, minimum 360 px, while keeping the
  centre terminal usable).
- **Map | List toggle** at the top of the map. The list (M3's panel) is the precise, searchable view of the same data,
  and the accessible alternative to the map. Selection is shared: picking in one picks in the other.
- **Detail footer**, pinned, never scrolls away:
    - **hero/model picker** defaults to the campaign's last-used hero, then the global default;
    - **skill picker** defaults to the recommendation, groups other skills Plan / Do / Check, and offers None for a
      plain quest session (see `docs/design/ui.md` §5.3);
    - **Start** launches the chosen hero and invokes the chosen skill as the opener. Disabled states say why ("claude
      not found", "another session is working in this folder").
- **Expand the map/list**: its header button fills the application workspace; Restore split returns. Projects,
  terminal and issue detail have equivalent controls. Bindings and input-aware Escape follow
  `docs/design/workspace.md` (#64/#65); terminal sessions remain mounted and running.
- **Narrow screens**: the right column becomes a drawer (as today), with map and detail stacked.

## 3. What the map shows

### Levels

Words: a **quest** is any GitHub issue. An **epic** is a quest that has sub-issues; this is worked out from the
sub-issue links, never from a label. A **dungeon** is a top-level epic (one with no parent). A **hall** is a sub-epic
inside a dungeon (drawing term only). The **Tavern** is the place for quests that belong to no epic: a quest with no
parent and no sub-issues, including an issue labelled "epic" that has no sub-issues.

| Level | Shows | Opens with | Back |
|---|---|---|---|
| **World** | one node per dungeon, plus the Tavern; lines between dungeons | it is the Map's top | none |
| **Dungeon** | the epic's quests as rooms, halls for sub-epics, external stubs | double-click or Enter on a node | Escape, or the breadcrumb |
| **Tavern** | the dungeon view for stray quests: a plain grid, no halls, lines only for real blockers | double-click or Enter on the Tavern node | Escape, or the breadcrumb |

- **Breadcrumb** above the map: `World › Onboarding flow`, with its progress ("1 of 4 cleared"). It replaces the scope
  chip and the "All N quests" button of the v4 prototype: the breadcrumb root is the way back to the whole campaign.
- **World node**: the epic's number and title, a progress ring, counts (ready, running, blocked), and the state markers
  of §3 (a dungeon with a waiting session inside shows `?`). Dungeon art starts as a plain shape; real art comes later.
  A dungeon whose quests are all closed is greyed out with a green check, not hidden.
- **World lines**: dungeon A leads to dungeon B when any open or closed quest in A blocks a quest in B. The line carries
  the count of such links and a gate at B. Walking into B shows those blockers as external stubs (below).
- **A cleared quest or dungeon** is shown greyed with a green check, at every level. An optional "hide cleared" switch
  (off by default) is for large repositories.
- **Selection works at every level.** Selecting a dungeon shows the epic's own issue in the detail, with its progress
  and its quests; selecting a quest shows that quest. Blocked dungeons and quests can always be opened and read.

### States

The state language from `docs/design/experience.md` §7.3, made visual. Colour is never the only signal; each state has a
shape or glyph too.

| Quest state          | Light                    | Glyph    | Meaning                                   |
|----------------------|--------------------------|----------|-------------------------------------------|
| Open                 | lit, cool (blue-white)   | `!`      | ready to start                            |
| In progress (agent)  | bright, warm, flickering | hero     | a session here is running                 |
| Needs you            | bright, pulsing ring     | `?`      | the agent is waiting for an answer        |
| Assigned (elsewhere) | lit, muted               | `@`      | someone else holds it                     |
| Locked               | dark, behind a door      | `!` grey | blocked by an open issue                  |
| Cleared              | faint, desaturated       | `✓` green | closed; always shown greyed, "hide cleared" is optional |
| Outside scope        | outline only, small      | none     | a blocker or parent not in the scope      |

The `!` and `?` borrow the familiar MMO convention: a mark over something means "there is something for you here", a
grey mark means "not yet". It costs nothing to draw and reads instantly. `?` is used for "needs you" (an agent waiting,
or later a pull request ready for review), so it means "come back and answer".

### Links

- **Blocked by**: a line with a door marker at the blocked end. Closed blockers turn the door open (or the line faint).
- **Parent / sub-issue**: not a line but containment. A top-level parent is a dungeon (see Levels). A parent that is
  itself a child becomes a **hall** inside its dungeon: a rounded region that holds its children, with the parent's
  title on it. Halls can be collapsed to one room with a count.
- **Outside the scope**: a blocker or parent that the scope leaves out is drawn as a small stub (a hollow diamond, effort
  "unknown", its number, kind and short title) in a strip under the graph labelled **Outskirts · external quests**,
  joined to the room it blocks by a dashed line. The v4 prototype shows exactly this. Closed stubs are struck through.
- **Quests that belong to no epic** are not scattered on the graph: they live in the Tavern (see Levels). Inside a
  dungeon every quest is a child of the epic, so there is no strip of unlinked quests; siblings without blockers sit
  in the first column.

### Selection: the map is how you pick the next quest

**Every room is clickable, whatever its state.** Clicking is reading; starting is a separate, deliberate action in the
detail footer. So a locked quest can always be opened and read, it just cannot be started yet.

- Click (or Enter on a focused room): the room brightens, its direct links light up, the rest dims slightly, and the
  quest opens in the detail below. Hovering shows a small card (number, title, state in words).
- What the footer allows depends on the state:

| Quest state          | Can read | Can start | Footer shows                                                  |
|----------------------|----------|-----------|---------------------------------------------------------------|
| Open                 | yes      | yes       | hero/model combo, skill combo, Start                           |
| Needs you            | yes      | n/a       | "Go to session" instead of Start (the session already exists) |
| In progress (agent)  | yes      | n/a       | "Go to session"                                               |
| Assigned (elsewhere) | yes      | yes       | Start, with a note: "assigned to @alice"                      |
| Locked               | yes      | **no**    | Start disabled, with "Blocked by #41, #43" as links           |
| Cleared              | yes      | no        | Start disabled, "Closed"                                      |
| Outside scope (stub) | yes      | no        | a short summary and "Show in scope" / "Open on GitHub"        |

- Disabled launch controls stay visible, so the user sees what would be offered once the blockers clear. The reason is
  next to Start, not only in a tooltip, and each blocker is a link that selects it on the map, so the user can walk the
  chain to the quest that is actually startable.
- **Jump to prerequisite**: a button in the blocked-by card. One blocker: selects it. Several: a short list to pick
  from. A blocker in another dungeon: the map moves to that dungeon first, then selects it.
- Starting is only the footer's Start button. Double-click never starts a session: on a dungeon or the Tavern it
  enters; on a room it only selects. A launch costs tokens and writes to the repository, so it is never one slip away.
- Decided: **a locked quest cannot be started**, not even through an override. The same holds for a dungeon's epic.
  Reading stays open on purpose: someone building the foundation may still want to read the future work to understand
  the whole roadmap, so locked items are never hidden or made unreadable. GitHub's blocked-by link is the source
  of truth: if the link is wrong, fix it on GitHub ("Open on GitHub" is in the detail) and the quest unlocks on the
  next refresh. To explore a locked quest anyway, start a free session from the campaign card and talk it through.

### Room anatomy (from the v4 prototype)

- Top to bottom: creature (or the hollow diamond when effort is unknown), then `#N` with the effort word (Trivial, Low,
  Moderate, High, Extreme, Unknown), then the short title.
- A round **state badge** sits on the top-right corner: `✓` cleared, `!` open, a small lock or box for locked, `@`
  assigned elsewhere, `×` closed, `?` needs you. The prototype also puts `!` on the quest that has a running session;
  that is a prototype shortcut. A running quest shows the hero token and warm light from §3 States, so state and
  effort stay separate.
- A **gate** marker sits on a blocked-by line, about 36 by 60 px, 36 px from the blocked room so the room never clips
  it; it opens (or the line fades) when the blocker closes. A line that skips a column keeps its gate beside its blocker,
  and lines meeting one side of a room are spread over its edge. Lines are not routed around intermediate rooms; that is
  left for the v2 rework (#44).
- The selected room gets a bright outline; the rest dims only a little so the map stays readable.

## 4. Visual directions

Ordered from cheapest to most expensive. They are not exclusive; the recommendation in §6 combines the first three.

### A. Plain graph (the fallback and the plain mode)

Dots or rounded boxes and lines, state colours, labels. Close to any dependency viewer.

- Effort: smallest. It is the first thing built in any case, because every other direction is a skin over it.
- Use: the rendering for plain-words mode, and the fallback if a skin fails.
- Theme: none.

### B. Torchlight dungeon (recommended skin for the MVP)

The same graph drawn as a dark floor plan where **light carries the state**:

- Background: near-black stone texture made with a CSS gradient and a little noise (an SVG `feTurbulence` filter), no
  image files.
- Rooms: rounded rectangles with a thin stone border; a readable creature sprite sits in the room above or alongside
  the issue number. The room's light is a radial gradient behind it. Open rooms glow cool, active rooms glow warm and
  **flicker** (a slow, slightly random CSS opacity animation), locked rooms are dark.
- Corridors: the links, drawn as soft lines with a faint glow where both ends are lit.
- Fog: everything outside the selected hall or the startable set can dim into darkness (a mask), toggled by "focus".
- `prefers-reduced-motion` turns the flicker into a steady glow.

Effort: medium, using SVG and CSS around the sprite sheets. It gives the "dark dungeon crawler" feeling while keeping
the graph legible; remove the filters and sprites and it degrades to direction A.

### C. Effort-creature sprites (M5a)

M5a uses the 15 generated sprite sheets as the creature layer inside SVG quest rooms. They are part of the first map
renderer, not a later portrait pass.

- Resolve an explicit local creature override first, then an explicit estimate or a configured mapping from existing
  issue labels to the five roster tiers. For a parent without an estimate, child count may raise the parent's tier or
  room scale. Priority remains a separate urgency marker and never changes effort tier.
- If no reliable tier is available, use the neutral room fallback; do not guess from the title. Unknown effort must
  remain obvious in the room and detail.
- Choose a creature deterministically from the selected tier using the stable issue key described in
  `docs/design/creature-roster.md`. The assignment is computed, not stored, so reading an issue never writes state;
  a roster change ships as a new revision. Never write it to the repository or GitHub.
- At readable zoom, show the four-frame idle strip in the creature window. With reduced motion, show a fixed frame.
  Below the readable zoom threshold, simplify rooms to their state mark and number rather than drawing tiny art.
- In List mode, keep the same selected issue and may show its small creature marker beside the effort label; the list
  remains fully usable if images fail to load.

The tier definitions, sheet dimensions, style anchors and local assignment rule live in
`docs/design/creature-roster.md`. The sheets in `design/quest-encounters/assets/` are original design assets and are
never fetched at runtime.

### D. Quest markers

`!`, `?` and grey `!` above rooms (see §3), drawn as text in a bold display face with a small glow. Cheap, strongly
themed, and readable even at small sizes. Works on top of A or B.

### E. Hero tokens

Each running session places its hero's **token** on its room: a small circle with a glyph, ringed in the hero's
colour. Tokens are generated, not drawn:

- a geometric face or crest built from the hero id (like an identicon): a few shapes, symmetrical, two colours;
- or a class glyph chosen by the user (sword, staff, bow, book), from a small icon set we draw ourselves as SVG paths.

When an expedition starts, the token appears at the room; when it ends, it fades. Several tokens can share a room.
This makes "which agent is where" visible at a glance and is the closest to "characters" without art.

### F. Portraits and dark digital art (later)

Hand-made or commissioned portraits per hero class, a stone-and-ember UI frame, room illustrations by label. Extend the
current direction in `docs/design/art-direction.md` and use licensed or original assets (our own, or CC0 packs with
their notices). It fits behind the same renderer interface and the same tokens, so it can come later without redesign.

### G. Floors, isometric or sprite maps (later)

Milestones as stacked floors (isometric), tile-based rooms, and animated creatures walking corridors. This is distinct
from the M5a sprite-backed graph: here layout becomes spatial design, not a graph. Only after the M5a map proves useful
on real repositories, and only behind the same renderer interface.

### Comparison

| Direction        | Effort       | Theme strength | Readability | When       |
|------------------|--------------|----------------|-------------|------------|
| A. Plain graph   | small        | none           | high        | MVP (base) |
| B. Torchlight    | small–medium | strong         | high        | MVP        |
| C. Creature sprites | medium    | very strong    | high        | M5a        |
| D. Quest markers    | tiny      | strong         | high        | M5a        |
| E. Hero tokens      | small     | medium         | high        | M6         |
| F. Portraits        | large     | very strong    | medium      | later      |
| G. Isometric         | very large| very strong    | medium–low  | later      |

## 5. How it is built

- **One renderer interface**: `mount(el)`, `setModel(model)`, `onSelect(fn)`, `focus(id)`, `destroy()`. The model is
  plain data (rooms, halls, links, tokens, states, positions). Skins A to F are implementations or style layers of it.
- **Layout is a pure function** `layout(quests, sessions, scope) → model`, unit-tested without a browser: same input,
  same positions. It handles the three levels (world, dungeon, Tavern), halls, out-of-scope stubs, derived lines
  between dungeons and collapsed halls. A view-model per quest (state, effort tier, blockers, parent, epic) is built
  once and shared by the map and the list, so the two cannot disagree.
- **Graph layout**: in-house, in `web/src/lib/map/layout.ts`, with no dependency. Rooms sit in columns by their
  longest blocked-by path (cycles are cut and marked), each hall is its own horizontal lane so its frame never
  overlaps other rooms, and stubs go in an outskirts strip below. `@dagrejs/dagre` (MIT) stays the fallback if
  real repositories need crossing reduction; `elkjs` is EPL-2.0, which is not on our permissive list.
- **SVG in Svelte**, no canvas for the MVP. Pan and zoom with a small in-house handler (wheel, drag, pinch, fit to view)
  or one small permissive library. Canvas only if a real repository proves SVG too slow.
- **Stable positions**: when a quest is added or closed, the rest should not jump. Seed the layout with previous
  positions where possible and animate small moves (disabled under reduced motion).
- **Live state**: session start, exit and "needs you" update tokens and lights without re-layout.

## 6. Recommended MVP

**A + B + C + D**, with the right-column layout from §2:

1. Right column: map on top, detail below, resizable splitter, Map | List toggle, pinned footer with hero/model combo,
   skill combo (recommended by default or None), and Start.
2. The world map and Tavern, the dungeon view with halls for sub-epics, door markers on blocked-by links,
   out-of-scope stubs, and a readable fallback for unknown effort or missing sprites.
3. Torchlight skin: dark floor, creature sprites in readable rooms, lit and dark states, warm flicker for active sessions.
4. `!`, grey `!` and `?` markers; creature identity remains independent of status.
5. Plain-words and Neutral modes show skin A with plain state names and no reliance on creature art.

Then M6: hero tokens (E), fog and focus, collapse and expand halls, stable-position animation, cleared-room toggle,
milestones as a filter (not floors yet).

Later, with an art direction: F, and only then G.

### Exit test for the MVP

On a real repository with at least 30 open issues and some blocked-by links: a person can say which quests are
startable, which are blocked and by what, and which one an agent is working on, within ten seconds and without reading
the list. Selecting a room and pressing Start launches the selected hero with the selected skill or None on that quest.

## 7. Accessibility and tests

- The map is an SVG with `role="application"` only where it must be; rooms are focusable with an accessible name ("#42
  Validate email addresses, open, blocked by #41"). Arrow keys move along links; Enter selects; the List view is
  always one key away.
- State is never colour alone (glyphs and shapes per §3), and contrast is checked for each state on the dark floor.
- Unit tests: the layout function (halls, outskirts, stubs, stable positions), state-to-visual mapping.
- Browser tests: click a room → detail shows it; footer starts a session with the chosen hero and spell; splitter size
  persists; Map | List keeps the selection; reduced motion disables flicker.
- A screenshot per milestone in `_probe/` of the map on the sample campaign and on a real repository.

## 8. Sample campaign as the showcase

The sample quests (`internal/quests/demo`) should be shaped to show the map well: two epics, the second blocked by
a quest of the first (a line on the world map and an external stub inside), one epic with a hall, a blocked chain of
three, two stray quests (the Tavern), one closed, one assigned elsewhere. The first-run experience
then shows a meaningful map within seconds, before any GitHub setup.

## 9. Plan alignment

- `docs/design/experience.md` §3 and §4: implement the Stitch cockpit shell in M3.6 and reserve the right-column map/detail
  split there; M5a implements the graph. The terminal remains in the full-height centre.
- `docs/design/experience.md` §3 and §7.5: launch controls move into the detail footer with a hero/model combo, a skill combo
  defaulted to the recommendation or None, and Start. None gives a plain quest session without a skill.
- `docs/design/experience.md` §7.4: point to this file for the map design.
- The spell strip became the footer's skill combo, not a separate strip (delivered with #50).
- `GLOSSARY.md`: "dungeon" now means an epic and "Tavern" the place for quests with no epic (plain: "No epic").
  "hall" and "outskirts" are drawing terms, not new vocabulary; if shown at all, they appear as plain "group" and
  "other epics" in the popover.

## 10. Decisions and remaining questions

Decided: cleared quests and dungeons are shown greyed with a green check; "hide cleared" is optional (§3, Levels).

Decided (the author delegated these; revisit any of them after seeing the map on a real repository):

1. **Hero tokens (M6):** generated geometric faces from the hero id, with no art to commission. A user-chosen class
   glyph can come later as an option.
2. **Halls:** parents drawn as regions that hold their children, as in the v4 prototype. Lines are only for blocked-by.
3. **A quest in scope whose parent epic is not:** it goes in the Tavern, and its detail says "part of #N, not in scope".
   A dashed stub dungeon is not built in v1.
4. **Boss:** the last quest of a dungeon's longest blocker chain, when that chain has three or more quests, gets the
   boss frame. The creature still follows effort; the frame carries position. At the world level, a dungeon's art tier
   follows its size and its longest chain.
5. **A very large world map:** the first version lays out every node; grouping by milestone is for later.
6. **A hall's own creature:** not drawn. The hall's header shows its title, progress and effort word. The creature
   belongs to quests that are rooms.
7. **Priority:** a `priority:low`, `priority:normal`, `priority:high` or `priority:urgent` label, mapped per campaign
   like effort. Only high (amber) and urgent (red) draw the small jewel on the room. It never changes the effort tier.
8. **Epic hint labels:** `epic` and `initiative` are hints only; the sub-issue links decide what an epic is
   (`docs/design/issue-shape.md`).

Decided: the epic itself can be started like any quest (what a session on an epic opens with is part of the briefing
work). A dungeon with many quests scrolls vertically inside the map. The map reads from the local SQLite copy once it
exists (#41); until then it uses the cached quest service, so it does not wait on #41. Keyboard: as in §7, extended
across the levels (arrows move between neighbours, Enter opens or selects, Escape goes up one level, one key
switches to the List). The art still to be made is listed in `docs/design/art-checklist.md`.

Resolved: M5a includes the torchlight skin, state markers, and creature sprites. The 15-sheet roster and current theme
guidance are in `docs/design/creature-roster.md` and `docs/design/art-direction.md`; source artwork remains in
`design/quest-encounters/assets/`. Only walking sprites and isometric floors remain later work.

## 11. Planned workflow extensions

Basic expansion/restoration belongs to #63, not deferred map decoration. #44 is limited to additional map-specific
focus/grouping and milestone presentation after the full campaign loop is useful; it must not rebuild M5a's halls,
outside-scope stubs or existing world navigation. #67 strengthens completion/cancellation, missing-blocker, cycle and
qualified-reference handling. Until that implementation lands, the current map uses the existing open/closed facts.
Changing method never changes readiness; the list and map consume the same model and remain behaviorally equivalent.
