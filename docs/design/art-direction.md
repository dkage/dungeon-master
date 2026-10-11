---
status: current
milestone: M5a, M6
github_issues: [28, 44]
tracking: in-gh-issues
next: use for current theme work; review as later art is added
verified: 2026-10-11
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
surface framed by dark carved wood and aged brass, with a near-black slate
terminal and cool blue-gray stone for the map. Muted burgundy and ember accents
mark selected or active elements. This replaces the first study's black-iron,
oxblood-glass material emphasis. Follow v4 for composition, contrast, density,
and material balance; use the original creature art as a compatible map layer,
not as authority for the whole cockpit.

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

## Review at actual size

Check the map at a 392 px panel width. A person should identify the selected
quest, the blocked chain, the ready rooms, and the creature tier without
zooming the browser. Check keyboard focus, reduced motion, List selection
persistence, and a low-ornament fallback before calling a redesign done.
