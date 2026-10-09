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
- A creature suggests **effort** only. The 15-creature roster provides several
  silhouettes per tier; unknown effort uses an empty diamond.
- A separate marker and border carry **state**: `!` ready, `▣` blocked,
  `@` assigned, `✓` cleared. A door marks a blocking edge.
- Quest detail and launch controls stay below the map. The List toggle is the
  readable alternative, with the same selection.
- Reduced-motion mode stops the idle animation.

## Assets

The 15-creature tier map, stable issue assignment guidance, and sprite contract
are in [CREATURE_ROSTER.md](./CREATURE_ROSTER.md). All 15 sheets are in
`assets/`: rat, slime, bat, kobold, beetle, spider, goblin, skeleton, ghoul,
orc, troll, warlock, many-eyed stone watcher, lich, and dragon. The original
rat, goblin, and watcher sheets remain the style anchors for the 12 additions.
Open [roster.html](./roster.html) to compare all strips together by effort tier.

Each strip is 2172 × 724 RGBA, with four 543 px wide cells. The mockup displays
each cell inside a 58 × 70 px window. These are **prototype sprites**, not final
animation masters: inspect the frame edges and baseline at native size before
using them in the M5a runtime renderer. The stone texture has not been verified
as mathematically seamless, so the mockup avoids a visible repeat.

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

Each original strip used its corresponding concept-sheet row as reference and
asked for four equally spaced poses of the **same** character on transparent
background. The 12 new sheets used the rat, goblin, and watcher as style
references and followed the shared strip contract in `CREATURE_ROSTER.md`.
Every prompt asked for restrained idle movement, no scenery or UI, consistent
cool blue rim light, and restrained amber accents. The texture prompt asked
for a quiet top-down blue-black stone floor, uniform lighting and no focal
objects or text.

## What the study suggests

The three silhouettes work at selected-room scale. At a full 300-quest zoom-out,
the renderer should fall back to small room shapes and state marks; reveal the
creatures when zoomed into a hall or selecting a quest. Keep the plain SVG graph
as the underlying renderer, as planned in `PLAN_MAP.md`.
