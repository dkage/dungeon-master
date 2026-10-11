# World map art

Current raster and vector assets for the world map. The request list and
completion status live in the [art checklist](../../docs/design/art-checklist.md).
Follow the [Dungeon art direction](../../docs/design/art-direction.md) and the
[map plan](../../docs/design/map.md) when adding or changing assets.

## Painted map pieces

Transparent PNG illustrations, 512 × 512:

- `dungeon-1-burrow.png`
- `dungeon-2-crypt.png`
- `dungeon-3-tower.png`
- `dungeon-4-fortress.png`
- `dungeon-5-citadel.png`
- `tavern.png`

The five dungeons form a size ladder from the smallest burrow to the largest
citadel. The tavern is the friendly map destination.

## Tileable floors

Opaque 1024 × 1024 JPEG textures, quality 85:

- `world-floor.jpg`: cool, worn stone for the world map.
- `tavern-floor.jpg`: warm, dark timber planks for the tavern.

Both were reviewed as 2 × 2 repeats for visible seams.

## SVG pieces

All SVGs use paths only, the listed `viewBox`, and `currentColor`; they have no
fixed dimensions, text, embedded raster, gradients, or filters.

| File                  | `viewBox`     | Use                               |
|-----------------------|---------------|-----------------------------------|
| `gate-closed.svg`     | `0 0 24 40`   | Lowered portcullis                |
| `gate-open.svg`       | `0 0 24 40`   | Raised portcullis                 |
| `badge-ready.svg`     | `0 0 24 24`   | Fantasy-style exclamation glyph   |
| `badge-needs-you.svg` | `0 0 24 24`   | Fantasy-style question glyph      |
| `badge-assigned.svg`  | `0 0 24 24`   | Assignment marker                 |
| `badge-locked.svg`    | `0 0 24 24`   | Lock state                        |
| `badge-closed.svg`    | `0 0 24 24`   | Closed state                      |
| `badge-cleared.svg`   | `0 0 24 24`   | Cleared state                     |
| `badge-running.svg`   | `0 0 24 24`   | Active work                       |
| `stub-diamond.svg`    | `0 0 64 64`   | Empty room marker                 |
| `boss-frame.svg`      | `0 0 240 280` | Optional scalable boss-room frame |
| `priority-gem.svg`    | `0 0 16 16`   | Optional priority marker          |

The exclamation and question glyphs use bold, ornamental fantasy punctuation
for the quick readability of World of Warcraft quest marks, with original
shapes. Their actual color comes from the consuming UI through `currentColor`.
