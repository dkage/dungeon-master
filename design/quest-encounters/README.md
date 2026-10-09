# Quest encounters: right-panel study

This is the first prototype (v1). The current visual baseline is `design/stitch-prototype/`.

Open `index.html` in a browser. This is a standalone, interactive design study at a
392 px right-panel width, alongside a quiet terminal and the sample campaign.
It does not change the app or its quest data.

`ART_DIRECTION.md` records the visual hierarchy and material rules for future
UI work. External mood images are separate references; this directory contains
the original project prototype and generated assets.

The map uses the real sample quest titles and relationships from
`internal/quests/demo/demo.go`. Effort tiers are **local mock data** for the
study: #2 is trivial, #3 and #4 are moderate, #5 is large, and #6 and #7 are
unknown. The current quest API does not provide an effort estimate. A real
implementation needs an explicit local mapping from labels or another chosen
source; it should never infer effort from an issue title.

`sample-quests.json` lists all 13 sample quests in a compact format for UI
generators. It mirrors the demo source's titles and relationships; its
`effortTier` and `creatureId` fields are design fixtures, not API fields.

## Visual rule

- The room, number, title, and dependency edges carry the task information.
- A creature suggests **effort** only. Rat = trivial, goblin = moderate,
  original many-eyed creature = large, empty diamond = unknown.
- A separate marker and border carry **state**: `!` ready, `▣` blocked,
  `@` assigned, `✓` cleared. A door marks a blocking edge.
- Quest detail and launch controls stay below the map. The List toggle is the
  readable alternative, with the same selection.
- Reduced-motion mode stops the idle animation.

## Assets

| File                              | Purpose                                                 |
|-----------------------------------|---------------------------------------------------------|
| `assets/rat-idle-strip.png`       | Four transparent rat poses                              |
| `assets/goblin-idle-strip.png`    | Four transparent goblin poses                           |
| `assets/many-eyed-idle-strip.png` | Four transparent poses of an original floating creature |
| `assets/stone-floor.png`          | Dark stone texture under a heavy CSS shade              |

The three strips are 2172 × 724 RGBA, with four 543 px wide cells. The mockup
displays each cell inside a 58 × 70 px window. The first frame of every creature
is recognizable at that size in `preview-panel.png`. These are **prototype
sprites**, not final animation masters: the generated frames need consistent
baseline and edge cleanup before shipping. The stone texture has not been
verified as mathematically seamless, so the mockup avoids a visible repeat.

`preview-panel.png` is the actual-width map and detail. `preview.png` shows the
whole cockpit. `preview-list.png` shows the list alternative after selecting a
blocked quest. The HTML can be used to try room selection and the Map/List
toggle; its Start button intentionally shows a prototype notice.

## Generation prompts

The images were made with the built-in imagegen tool. The concept sheet used:

> Explore three original dungeon encounter sprites for explicit effort tiers in
> a narrow quest map: a tiny rat, a mischievous goblin, and an original
> formidable floating many-eyed creature. Show idle poses and a dark stone
> dungeon treatment. Crisp, readable 2D game art with restrained painterly
> pixel-art influence; cool blue ambient light and warm amber accents; no UI
> text, logos, gore, or franchise character likeness.

Each strip then used its corresponding concept-sheet row as reference and asked
for four equally spaced poses of the **same** character on a transparent
background: rat (crouch, sniff, rise, crouch), goblin (crouch, breathing shift,
glance, crouch), and many-eyed creature (hover, tilt, narrowed eye, hover).
The texture prompt asked for a quiet top-down blue-black stone floor, uniform
lighting and no focal objects or text.

## What the study suggests

The three silhouettes work at selected-room scale. At a full 300-quest zoom-out,
the renderer should fall back to small room shapes and state marks; reveal the
creatures when zoomed into a hall or selecting a quest. Keep the plain SVG graph
as the underlying renderer, as planned in `PLAN_MAP.md`.
