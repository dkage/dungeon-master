---
status: current
milestone: M5a, M6
github_issues: [28, 44, 72, 73, 74, 75, 76, 77, 93, 96, 97, 98, 99]
tracking: in-gh-issues
next: review as later art is added
verified: 2026-10-13
---

# Dungeon theme: art direction

This is the current visual direction for the Dungeon theme. The full cockpit
baseline is the Google Stitch v4 export in
[`design/stitch-prototype/`](../../design/stitch-prototype/). The earlier
quest encounter study and its previews remain in
[`design/quest-encounters/`](../../design/quest-encounters/) as historical
references and an asset library. This document describes the direction to
carry into product work; it is not an asset sheet.

## Current theme

The current direction is the Stitch v4 **Gothic Codex**: a warm parchment
terminal page framed by dark carved wood and aged brass, near-black slate
behind the side panels and cool blue-gray stone for the map. Muted burgundy
and ember accents mark selected or active elements. This replaces the first
study's black-iron, oxblood-glass material emphasis. Follow v4 for
composition, contrast, density, and material balance; use the original
creature art as a compatible map layer, not as authority for the whole
cockpit.

The theme selector is a later feature. Build this one coherent Dungeon look
first, with reusable color and material tokens so another theme can be added
without rebuilding the layout.

## Visual hierarchy

1. The terminal is the calm, largest surface. Its text and cursor remain the
   clearest and most familiar part of the window.
2. The quest map carries the fantasy atmosphere. Rooms, corridors, lights, and
   creature silhouettes make relationships readable at a glance.
3. Quest detail, settings, and forms stay compact and conventional. Ornament
   may frame a section or a primary action, but never reduce text contrast or
   crowd a control.
4. In a narrow window, preserve a usable terminal and move the right panel
   into a drawer.

## Materials and light

- Use blue-black or charcoal stone and quiet grain behind the map. Place a
  dark shade over the stone texture so nodes, lines, and titles dominate.
- Use black iron and aged brass for framing. Oxblood red belongs in recesses,
  selected frames, and a few important controls; small red jewels can anchor
  a hall title, modal corner, or primary action. Jewels should look embedded
  in the interface, not scattered across every row.
- Keep a separate state language: cool light for ready quests, warm ember for
  a running agent, gold attention for needs-input, and dim stone for blocked
  and cleared rooms. Decorative red does not replace state markers.
- Carved corners and bevels should be strongest on the map frame, a hall, or
  a modal. Dense lists, form fields, and the terminal need simpler edges.
- Treat blocked doors as part of the graph. They should connect visibly to
  the issue that blocks the selected quest.
- Use color together with a glyph, shape, and text. Warm, cool, and muted
  states must still make sense without hue.

## Creatures

- A creature is a compact effort cue inside a numbered quest room. The room
  and its graph links remain useful if the image fails to load.
- The supplied rat, goblin, and many-eyed stone watcher set the first style:
  dark outline, cool edge light, controlled warm highlights, and a silhouette
  readable in roughly 58 × 70 px. The surrounding room can carry red jewel
  detail without making every creature red.
- Match those references closely for every new sprite: one three-quarter side
  view, consistent dark outline weight, cool blue ambient and rim light from
  the same direction, restrained amber highlights, painterly pixel-art
  influence, muted materials, and the same 2172 × 724 four-frame strip with
  transparent background and shared baseline. Keep the character centered in
  each 543 × 724 cell; use only the silhouette, anatomy, and idle motion to
  distinguish species. Do not add a ground plane, scene lighting changes,
  decorative particles, UI, text, or franchise-specific features.
- Future low-tier creatures should vary in body shape and motion so common
  quests do not all look alike. Keep camera angle, lighting, frame size, and
  animation baseline consistent across the roster.
- A priority marker and a parent's hall size are independent of the creature
  species. An issue retains its assigned creature across sessions and later
  roster additions.
- At repository-wide zoom, simplify to room shapes and state marks. Show
  full creatures when a hall is opened or a quest is selected.

## Reference order for later design work

1. `design/stitch-prototype/` v4 exports define the current cockpit layout and
   visual balance. Its map, list, and blocked-quest views are companion states.
2. This note defines the material and creature rules to carry into product UI.
3. External mood images may guide atmosphere and material choices. Create
   original borders, icons, and components for this app; do not import,
   trace, or extract pieces from reference images.

## Terminal typography

Mapped against the Stitch v4 full-window reference (#96). The reference sets
the terminal body in a serif with HTML pills and a prompt box; the real
terminal draws a cell grid, so only family, size, line height, weight and
padding can map. Options live in `web/src/lib/termOptions.ts`.

| Trait | Reference | Terminal | Why |
| --- | --- | --- | --- |
| Family | Georgia serif body; Fira Code for stamps | Fixed monospace stack: Nerd Font names, bundled Fira Code, then system monospace | Column alignment and glyph coverage (box drawing, prompt icons) need a monospace. The stack is not user-selectable. |
| Interface serif | n/a | Almendra, EB Garamond and Alegreya never reach the terminal, in either appearance | The reading font only sets `--reading-font`; a browser test checks the computed family. |
| Line height | 1.7 | 1.3 | 1.7 costs about a third of the rows a TUI needs; 1.3 adds air and box drawing stays joined. |
| Weight | Bold dark ink | Regular, with bold left to the agent | Bold everywhere removes the agent's own emphasis; contrast comes from the colour rules (#93). |
| Size | 12.5px | 13px default | The user's size is #92. |
| Padding | 22px 28px | 22px 28px in Dungeon, 8px in Neutral | Matches the parchment frame; Neutral stays compact. |
| Fira Code | Timestamps and pills | In the stack ahead of system monospace | Already bundled, so the look is the same on every machine. |

Planned differences (cannot map to a cell grid): the serif body and log lines,
the bold code pills, and the prompt input box. #72's side-by-side comparison
cites this section for each.

## Accepted differences from the v4 reference

The #72 comparison (2026-10-13) set the running cockpit beside the three reference screens. Textures, frames, panel
headers, the left panel grouping and the launch bar match. These differences remain on purpose; the plans win where
they disagree with the reference.

| Reference | Cockpit | Reason |
|---|---|---|
| Clear, Reload and Expand buttons in the terminal header | Omitted | #77 (Clear, Reload); panel expansion is #64. |
| "Expedition: #3 …" chip, hero buttons in the right panel | Project name, state chip and hero buttons in the terminal header | The header is the workspace's only place to start a free session; the plans keep it. |
| Skill selector and the spell shortcut bar | Slot kept free in the launch bar; no spell bar | Skill choice and delivery are #50, #43 and #51. |
| Quest rows with title, effort line and state pill only | Rows also show labels, relations and progress | The list must show scope labels, blockers and progress (`docs/design/ui.md`). |
| Quest detail with three badges and a boxed blocker | Effort source and setting, parent, blocked-by and blocking lists, prerequisite jump | Effort is settable locally and relations are navigable (`docs/design/map.md`). |
| An opened dungeon laid out in a grid inside a hall frame | One row per chain, outskirts below, no hall frame | Layout follows `docs/design/map.md`; richer grouping is #44. |
| No starter-hero banner | Banner while the built-in heroes are in use | It is state the user must see until they keep or edit them. |

## Review at actual size

Check the map at a 392 px panel width. A person should identify the selected
quest, the blocked chain, the ready rooms, and the creature tier without
zooming the browser. Check keyboard focus, reduced motion, List selection
persistence, and a low-ornament fallback before calling a redesign done.
