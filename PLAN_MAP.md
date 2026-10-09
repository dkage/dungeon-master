# Map and right panel: brainstorm

Brainstorm for the map (M5a and M6 in `PLAN_EXPERIENCE.md` §4) and for the right-hand panel around it. Nothing here
is decided yet. It ends with a recommended MVP and the questions to settle before building.

It does not edit the plans in motion. Where it changes them (the layout in `PLAN_EXPERIENCE.md` §3 and §7.4), the
change is listed in §9 so it can go into the M3.7 reconciliation pass.

## 1. What we want

The parts of the experience we want:

- Every open quest is a point on a map. Quests being worked in a terminal right now look different (brighter, warmer).
  Blocked-by and parent links are lines between points.
- Clicking a point highlights it (brighter, its links emphasised) and shows the quest in the lower part of the right
  panel. The split between map and detail can be resized.
- The bottom of the detail has pinned launch controls: a hero/model picker, a skill picker defaulted to the
  recommendation (with None available), and Start. The selected skill runs as the session opener.

Constraints:

- **No game engine and no sprite pipeline for the MVP.** SVG, CSS and a layout library. Art can come later behind the
  same interface, and it is not ruled out.
- The theme comes from **light, shape and a few glyphs**, not from illustrations.
- Readable first: a user must understand the state of a repository at a glance, in both theme modes.
- Works with the plain-words mode (`PLAN_M3_7.md` §2.9): the map has a plain rendering too.
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

The controls around the map (header, detail, footer, settings, shortcuts) are specified in `PLAN_UI.md`; this section
keeps only the layout decision.

This is better than the stage tabs proposed in `PLAN_EXPERIENCE.md` §3, where the map and the terminals took turns in
the centre. With the map on the right you watch the agents and the map together, which is the point of a cockpit.

- **Right column**: map on top, quest detail below, a draggable splitter between them. Both sizes are remembered per
  window. The column itself is resizable (about half the window on a wide desktop, minimum 360 px, while keeping the
  centre terminal usable).
- **Map | List toggle** at the top of the map. The list (M3's panel) is the precise, searchable view of the same data,
  and the accessible alternative to the map. Selection is shared: picking in one picks in the other.
- **Detail footer**, pinned, never scrolls away:
    - **hero/model picker** defaults to the campaign's last-used hero, then the global default;
    - **skill picker** defaults to the recommendation, groups other skills Plan / Do / Check, and offers None for a
      plain quest session (see `PLAN_UI.md` §5.3);
    - **Start** launches the chosen hero and invokes the chosen skill as the opener. Disabled states say why ("claude
      not found", "another session is working in this folder").
- **Expand the map**: one key (M) or a button makes the map fill the centre temporarily, for large repositories. Escape
  returns. The terminal keeps running.
- **Narrow screens**: the right column becomes a drawer (as today), with map and detail stacked.

## 3. What the map shows

### States

The state language from `PLAN_EXPERIENCE.md` §7.3, made visual. Colour is never the only signal; each state has a
shape or glyph too.

| Quest state          | Light                    | Glyph    | Meaning                                   |
|----------------------|--------------------------|----------|-------------------------------------------|
| Open                 | lit, cool (blue-white)   | `!`      | ready to start                            |
| In progress (agent)  | bright, warm, flickering | hero     | a session here is running                 |
| Needs you            | bright, pulsing ring     | `?`      | the agent is waiting for an answer        |
| Assigned (elsewhere) | lit, muted               | `@`      | someone else holds it                     |
| Locked               | dark, behind a door      | `!` grey | blocked by an open issue                  |
| Cleared              | faint, desaturated       | `✓`     | closed; hidden by default, toggle to show |
| Outside scope        | outline only, small      | none     | a blocker or parent not in the scope      |

The `!` and `?` borrow the familiar MMO convention: a mark over something means "there is something for you here", a
grey mark means "not yet". It costs nothing to draw and reads instantly. `?` is used for "needs you" (an agent waiting,
or later a pull request ready for review), so it means "come back and answer".

### Links

- **Blocked by**: a line with a door marker at the blocked end. Closed blockers turn the door open (or the line faint).
- **Parent / sub-issue**: not a line but containment. A parent with children becomes a **hall**: a rounded region that
  holds its children, with the parent's title on it. Halls can be collapsed to one room with a count.
- **Quests with no links at all**: grouped in an **outskirts** strip at the edge (a loose grid), so they do not scatter
  the graph. This is the "loose tasks are a town" idea, kept simple.

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
- Double-click on a startable room starts it with the footer's current hero and spell; on any other room it only
  selects.
- Decided: **a locked quest cannot be started**, not even through an override. GitHub's blocked-by link is the source
  of truth: if the link is wrong, fix it on GitHub ("Open on GitHub" is in the detail) and the quest unlocks on the
  next refresh. To explore a locked quest anyway, start a free session from the campaign card and talk it through.

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
- Rooms: rounded rectangles with a thin stone border; the room's light is a radial gradient behind it. Open rooms glow
  cool, active rooms glow warm and **flicker** (a slow, slightly random CSS opacity animation), locked rooms are dark.
- Corridors: the links, drawn as soft lines with a faint glow where both ends are lit.
- Fog: everything outside the selected hall or the startable set can dim into darkness (a mask), toggled by "focus".
- `prefers-reduced-motion` turns the flicker into a steady glow.

Effort: small to medium, all SVG and CSS. It gives the "dark dungeon crawler" feeling without a single sprite, and it
degrades well: remove the filters and it is direction A.

### C. Quest markers

`!`, `?` and grey `!` above rooms (see §3), drawn as text in a bold display face with a small glow. Cheap, strongly
themed, and readable even at small sizes. Works on top of A or B.

### D. Hero tokens

Each running session places its hero's **token** on its room: a small circle with a glyph, ringed in the hero's
colour. Tokens are generated, not drawn:

- a geometric face or crest built from the hero id (like an identicon): a few shapes, symmetrical, two colours;
- or a class glyph chosen by the user (sword, staff, bow, book), from a small icon set we draw ourselves as SVG paths.

When an expedition starts, the token appears at the room; when it ends, it fades. Several tokens can share a room.
This makes "which agent is where" visible at a glance and is the closest to "characters" without art.

### E. Portraits and dark digital art (later)

Hand-made or commissioned portraits per hero class, a stone-and-ember UI frame, room illustrations by label. This is
where a "Diablo-like" look would come from. It needs an art direction, a style guide and licensed or original assets
(our own, or CC0 packs with their notices). It fits behind the same renderer interface and the same tokens, so it can
come later without redesign.

### F. Floors, isometric or sprite maps (later)

Milestones as stacked floors (isometric), tile-based rooms, animated sprites walking corridors. The most game-like and
the most expensive: layout becomes spatial design, not a graph. Only after the plain map has proved useful on real
repositories, and only behind the same interface.

### Comparison

| Direction        | Effort       | Theme strength | Readability | When       |
|------------------|--------------|----------------|-------------|------------|
| A. Plain graph   | small        | none           | high        | MVP (base) |
| B. Torchlight    | small–medium | strong         | high        | MVP        |
| C. Quest markers | tiny         | strong         | high        | MVP        |
| D. Hero tokens   | small        | medium         | high        | M6         |
| E. Portraits     | large        | very strong    | medium      | later      |
| F. Isometric     | very large   | very strong    | medium–low  | later      |

## 5. How it is built

- **One renderer interface**: `mount(el)`, `setModel(model)`, `onSelect(fn)`, `focus(id)`, `destroy()`. The model is
  plain data (rooms, halls, links, tokens, states, positions). Skins A to F are implementations or style layers of it.
- **Layout is a pure function** `layout(quests, sessions, scope) → model`, unit-tested without a browser: same input,
  same positions. It handles halls, outskirts, out-of-scope stubs and collapsed halls.
- **Graph layout library**: `@dagrejs/dagre` (MIT) for a layered left-to-right layout. Check licences before adding
  anything else; `elkjs` is EPL-2.0, which is not on our permissive list.
- **SVG in Svelte**, no canvas for the MVP. Pan and zoom with a small in-house handler (wheel, drag, pinch, fit to view)
  or one small permissive library. Canvas only if a real repository proves SVG too slow.
- **Stable positions**: when a quest is added or closed, the rest should not jump. Seed the layout with previous
  positions where possible and animate small moves (disabled under reduced motion).
- **Live state**: session start, exit and "needs you" update tokens and lights without re-layout.

## 6. Recommended MVP

**A + B + C**, with the right-column layout from §2:

1. Right column: map on top, detail below, resizable splitter, Map | List toggle, pinned footer with hero/model combo,
   skill combo (recommended by default or None), and Start.
2. Plain graph with halls for parents, outskirts for unlinked quests, door markers on blocked-by links, out-of-scope
   stubs.
3. Torchlight skin: dark floor, lit and dark rooms, warm flicker for active sessions.
4. `!`, grey `!` and `?` markers.
5. Plain-words mode shows skin A with plain state names.

Then M6: hero tokens (D), fog and focus, collapse and expand halls, stable-position animation, cleared-room toggle,
milestones as a filter (not floors yet).

Later, with an art direction: E, and only then F.

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

The sample quests (`internal/quests/demo`) should be shaped to show the map well: one epic with five children (a hall),
a blocked chain of three, two loose quests (outskirts), one closed, one assigned elsewhere. The first-run experience
then shows a meaningful map within seconds, before any GitHub setup.

## 9. Changes to plans in motion (for the M3.7 pass)

- `PLAN_EXPERIENCE.md` §3: replace "stage tabs: map and one tab per expedition" with the right-column layout in §2 of
  this file; the terminal keeps the full centre.
- `PLAN_EXPERIENCE.md` §3 and §7.5: launch controls move into the detail footer with a hero/model combo, a skill combo
  defaulted to the recommendation or None, and Start. None gives a plain quest session without a skill.
- `PLAN_EXPERIENCE.md` §7.4: point to this file for the map design.
- `PLAN_SKILLS.md` §6: the spell strip becomes the footer's skill combo, not a separate strip.
- `GLOSSARY.md`: "hall" and "outskirts" are drawing terms, not new vocabulary; if shown at all, they appear as plain
  "group" and "unlinked" in the popover.

## 10. Questions to settle

1. Right-column layout (this file) instead of stage tabs: agreed?
2. MVP skin: torchlight (B) with markers (C), or start with plain (A) only and add the skin in M6?
3. Should cleared quests show by default (faint) or be hidden until toggled?
4. Hero tokens: generated geometric faces, or user-chosen class glyphs?
5. Do halls (parents as regions) read better than parent links drawn as lines? Worth a quick prototype of both on the
   sample data.
6. Is a later art direction (E) a goal for v1, or a v2 item for `PLAN_V2.md`?
