# Art direction for the quest cockpit

This document preserves the visual decisions behind the encounter-map study.
The interactive baseline is [index.html](./index.html); the true-width visual
check is [preview-panel.png](./preview-panel.png). This is original project
direction, not an asset sheet.

## Current theme

The first art direction is a **gothic dungeon crawler**: black iron, worn
stone, oxblood red, dark crimson glass or jewel accents, and restrained aged
metal. It can evoke the atmosphere of classic dark fantasy game interfaces
without copying any game's artwork, component shapes, icons, or layout. The
current prototype fixes the cockpit layout and encounter scale; later visual
passes should deepen this material language while preserving readability.

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
- Future low-tier creatures should vary in body shape and motion so common
  quests do not all look alike. Keep camera angle, lighting, frame size, and
  animation baseline consistent across the roster.
- A priority marker and a parent's hall size are independent of the creature
  species. An issue retains its assigned creature across sessions and later
  roster additions.
- At repository-wide zoom, simplify to room shapes and state marks. Show
  full creatures when a hall is opened or a quest is selected.

## Reference order for later design work

1. The interactive prototype and its screenshots define the current layout
   and visual balance.
2. This note defines the material and creature rules that should survive a
   redesign.
3. External mood images may guide atmosphere and material choices. Create
   original borders, icons, and components for this app; do not import,
   trace, or extract pieces from reference images.

## Review at actual size

Check the map at a 392 px panel width. A person should identify the selected
quest, the blocked chain, the ready rooms, and the creature tier without
zooming the browser. Check keyboard focus, reduced motion, List selection
persistence, and a low-ornament fallback before calling a redesign done.
