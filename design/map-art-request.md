# Map art still to be made

What the three-level map (`docs/design/map.md`) needs that the repository does not have yet. The 15 creature sheets
and `stone-floor.png` already exist (`design/quest-encounters/`) and are not part of this list.

**Style for every piece:** the same family as the existing creatures. Original gothic dungeon art, crisp dark
silhouette, restrained painterly pixel-art influence, cool blue ambient and rim light with small warm amber
highlights, muted stone, wood and iron. Three-quarter front view. No text, logos, UI, gore or recognisable franchise
designs. Use `quest-encounters/assets/rat-idle-strip.png`, `goblin-idle-strip.png` and `many-eyed-idle-strip.png` as
style references, and judge new pieces next to them at native size.

**Why raster or vector:** painted subjects with light and texture are PNG (or JPG when opaque). Anything that must
take the theme's colours, scale crisply or be drawn at 12 to 24 px is SVG. State changes (cleared, locked, running,
needs you) are done in code with filters, glows and overlays, so no extra art variants are needed for them.

## A. Painted, PNG with transparent background

| # | File | Size | Notes |
|---|---|---|---|
| 1 | `dungeon-1-burrow.png` | 512 × 512 | Smallest dungeon: a cave mouth or burrow. Shown at about 128 to 160 px on the world map. |
| 2 | `dungeon-2-crypt.png` | 512 × 512 | A crypt entrance with steps and a sealed door. |
| 3 | `dungeon-3-tower.png` | 512 × 512 | A ruined watchtower or chapel. |
| 4 | `dungeon-4-fortress.png` | 512 × 512 | A fortress gate with walls. |
| 5 | `dungeon-5-citadel.png` | 512 × 512 | The largest: a citadel or castle with a dragon's-lair feel. |
| 6 | `tavern.png` | 512 × 512 | A tavern with lit windows and a notice board with pinned notes (no readable text). |

The tier follows the dungeon's size and danger, so the five should read as a clear ladder from small to grand. Each:
one static frame, centred, small footing only (no ground plane or cast shadow beyond the sprite), the base of the
building at the bottom edge, 16 px of transparent margin. A flicker of torchlight is done in code.

## B. Backgrounds, tileable, opaque

| # | File | Size | Format | Notes |
|---|---|---|---|---|
| 7 | `world-floor.jpg` | 1024 × 1024, seamless | JPG quality 85 | Dark worn stone or parchment, very low contrast, with a faint hint of old paths. The world map's backdrop; nodes must stay readable on it. |
| 8 | `tavern-floor.jpg` | 1024 × 1024, seamless | JPG quality 85 | Dark wooden planks, warm and low contrast. Optional; the stone floor can be reused. |

## C. SVG, vector

Common rules: `viewBox` as given, no fixed width or height, `fill` or `stroke` set to `currentColor` so the theme
colours them, no embedded raster, no text, and stroke 1.5 or heavier so they stay clear at 12 to 16 px.

| # | File | viewBox | Use |
|---|---|---|---|
| 9 | `gate-closed.svg` | 24 × 40 | A small portcullis on a blocked-by line, next to the blocked room. |
| 10 | `gate-open.svg` | 24 × 40 | The same gate raised, once the blocker is closed. |
| 11 | `badge-ready.svg` | 24 × 24 | `!` mark. |
| 12 | `badge-needs-you.svg` | 24 × 24 | `?` mark. |
| 13 | `badge-assigned.svg` | 24 × 24 | `@` mark. |
| 14 | `badge-locked.svg` | 24 × 24 | Padlock. |
| 15 | `badge-closed.svg` | 24 × 24 | `×` mark. |
| 16 | `badge-cleared.svg` | 24 × 24 | Check mark. |
| 17 | `badge-running.svg` | 24 × 24 | A small torch or a hero's helm: a session is working here. |
| 18 | `stub-diamond.svg` | 64 × 64 | Hollow diamond for a quest outside the scope or with unknown effort. |
| 19 | `boss-frame.svg` | 240 × 280, designed for 9-slice scaling | An ornate room frame for the last quest of a blocker chain. Corners fixed, edges stretch. Optional. |
| 20 | `priority-gem.svg` | 16 × 16 | A small jewel for urgent priority. Optional. |

## Not needed

The 15 creature sheets (done); hero tokens (generated in code, M6); the progress ring and the lines between rooms
(drawn in code); cleared, locked and running looks (code effects); a font (bundled already).

## Delivery

Put the files under `design/map-art/` named as above. Check each at native size next to the existing creatures. The
sizes are the largest we expect to display at 2x density, so smaller is fine to generate only if the style holds.
