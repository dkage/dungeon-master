---
status: current
milestone: M5a, M6
github_issues: [23, 28, 44, 73, 74]
tracking: in-gh-issues
next: review assets in the map as it ships; delete when M6 art work closes
verified: 2026-10-13
---

# Art checklist

Every image the map and cockpit use or need: what we have, and what is still to be made. Tick a box when the file is in
the repository and checked at native size next to the existing creatures. The plan it serves is `docs/design/map.md`.

**Style for every new piece:** the same family as the existing creatures. Original gothic dungeon art, crisp dark
silhouette, restrained painterly pixel-art influence, cool blue ambient and rim light with small warm amber
highlights, muted stone, wood and iron. Three-quarter front view. No text, logos, UI, gore or recognisable franchise
designs. Use `design/quest-encounters/assets/rat-idle-strip.png`, `goblin-idle-strip.png` and `many-eyed-idle-strip.png` as
style references.

**Raster or vector:** painted subjects with light and texture are PNG (JPG when opaque). Anything that must take the
theme's colours, scale crisply or be drawn at 12 to 24 px is SVG. State changes (cleared, locked, running, needs you)
are done in code with filters, glows and overlays, so no extra art variants are needed for them.

## Already in the repository

### Creatures (`design/quest-encounters/assets/`)

Each is one sheet of four idle frames, transparent, 2172 × 724 px (cells 543 × 724). Tiers are in
`docs/design/creature-roster.md`.

- [x] Trivial: `rat-idle-strip.png`, `slime-idle-strip.png`, `bat-idle-strip.png`
- [x] Low: `kobold-idle-strip.png`, `beetle-idle-strip.png`, `spider-idle-strip.png`
- [x] Moderate: `goblin-idle-strip.png`, `skeleton-idle-strip.png`, `ghoul-idle-strip.png`
- [x] High: `orc-idle-strip.png`, `troll-idle-strip.png`, `warlock-idle-strip.png`
- [x] Large: `many-eyed-idle-strip.png`, `lich-idle-strip.png`, `dragon-idle-strip.png`

### Textures

- [x] `design/quest-encounters/assets/stone-floor.png`: dark stone floor, 1254 × 1254, opaque. The dungeon view's backdrop.
- [x] `design/stitch-prototype/assets/images/stone-floor.png`: the same floor at 512 × 512 (prototype copy).
- [x] `design/stitch-prototype/assets/images/slate.png`: dark slate, 1254 × 1254, opaque, generated for this project. Panel
  backgrounds in the prototype.
- [x] `design/stitch-prototype/assets/images/6a8650faf410.png` and `97785ef87771.png`: one parchment texture, byte-identical,
  288 × 512, opaque. The terminal's page background. AI-generated (confirmed by the author). Not yet bundled in the
  app; #73 brings it in.

### Prototype-only copies

- [x] `design/stitch-prototype/assets/images/rat-idle-strip.png`, `goblin-idle-strip.png`, `many-eyed-idle-strip.png`:
  512 × 171 downscaled copies of the creature strips. Use the full-size ones from `design/quest-encounters/`.

### Fonts

- [x] Cinzel, Almendra, EB Garamond, Alegreya, Fira Code, bundled in `web/src/assets/fonts/` (M3.6).

### Removed

- The slate texture (a screenshot from another project) and two screenshots of other projects' fantasy UI kits were
  taken out of the prototype. The generated replacement slate is in (see Textures) and the pages use it again. The app
  itself never used any of them; it draws its panels with CSS only.

## How to use the prompts

Paste the **painted prefix** or the **SVG prefix** first, then the item's prompt. Attach the three style-reference
creatures (`rat`, `goblin`, `many-eyed` strips) to every painted request, and check the result next to them at native
size. Most image models cannot output a true transparent background: ask for a flat pure-magenta (#FF00FF) background
and remove it afterwards, or use a model that exports transparency. For SVG, ask a text model to write the code
directly; no image model is needed.

**Painted prefix:**
> Original gothic dungeon-crawler illustration for a desktop app, matching the attached reference creatures: crisp dark
> silhouette, restrained painterly pixel-art influence, cool blue ambient and rim light from the upper left, small warm
> amber highlights, muted stone, wood and iron. Three-quarter front view, one subject, centred, on a flat pure-magenta
> background. No text, letters, logos, UI, people, gore, cast shadow or ground plane beyond a small footing. Not
> derived from any existing game or franchise. Square, 512 × 512.

**SVG prefix:**
> Write one standalone SVG file. Use the given `viewBox`, no `width` or `height`, no embedded raster, no text, no
> gradients or filters. Draw with paths only, using `fill="currentColor"` or `stroke="currentColor"`, stroke width 1.5
> or more, round joins, simple enough to read at 14 px. Return only the SVG code.

## To make: painted, PNG with transparent background

- [x] `dungeon-1-burrow.png`, 512 × 512: smallest dungeon, shown at about 128 to 160 px on the world map.
  > Subject: a small burrow or cave mouth in a mossy rock mound, a few stones around it, a faint warm glow inside. The
  > smallest and least threatening of five dungeon sizes.
- [x] `dungeon-2-crypt.png`, 512 × 512:
  > Subject: a crypt entrance cut into a hillside: short stone steps down to a sealed iron-banded door under a carved
  > arch, two small unlit braziers. Slightly larger and grimmer than a burrow.
- [x] `dungeon-3-tower.png`, 512 × 512:
  > Subject: a ruined stone watchtower with a broken top, one lit arrow-slit window, ivy and fallen stones at the base.
- [x] `dungeon-4-fortress.png`, 512 × 512:
  > Subject: a fortress gatehouse with two squat towers and a short wall, a closed portcullis, torches on the wall and
  > a banner torn at the edges.
- [x] `dungeon-5-citadel.png`, 512 × 512: the largest, a ladder step above the fortress.
  > Subject: a tall citadel of many spires on a rocky crag, ember-red light in the windows and wisps of smoke, an
  > ominous dragon's-lair feel. The most imposing of the five.
- [x] `tavern.png`, 512 × 512:
  > Subject: a cosy, slightly worn timber-and-stone tavern with warm lit windows, a hanging sign (blank, no text), and
  > a notice board beside the door with several pinned paper notes (no readable text). Warmer and friendlier than the
  > dungeons, with the same lighting style.

The five dungeons read as a ladder from small to grand when placed side by side. The five dungeon assets and tavern were
generated in one session; all six were checked together at 512 × 512. A flicker of torchlight is done in code.

## To make: backgrounds, tileable, opaque

- [x] `design/stitch-prototype/assets/images/slate.png`, done: the slate that replaces the removed texture, behind the side
  panels. It arrived as a 1254 × 1254 PNG of 3.5 MB. The app copy is `web/src/assets/slate.jpg`: 512 × 512 JPG, quality
  85, 41 KB, shown at 256 px under a dark shade in Dungeon only (#74). The source did not tile (mean edge step 13 to 15
  against 7 inside the image), so the copy is the source folded into a mirrored 2 × 2 and scaled back to 512; checked as
  a 2 × 2 repeat, the edge step is 0 to 0.15 against 4 inside, so no seam shows.
  > Seamless tileable texture, 512 × 512: dark slate rock, near black with a hint of warm brown, fine grain and a few
  > shallow cracks, low contrast, even light with no focal point, no text. Match the tone and lighting of the attached
  > stone floor (our own `stone-floor.png`, the only reference to attach). Original, not derived from any photograph
  > or game asset. Must tile without visible seams.
- [x] `world-floor.jpg`, 1024 × 1024, seamless, JPG quality 85:
  > Seamless tileable texture, 1024 × 1024: dark worn stone with a very faint hint of old paths and cracks, low
  > contrast, nearly black-blue, no focal point, no text. Light and detail must be even across the whole image so it
  > tiles without visible seams. Cool blue ambient, a trace of warm tone. Same material family as the attached stone
  > floor.
- [x] `tavern-floor.jpg`, 1024 × 1024, seamless, JPG quality 85 (optional; the stone floor can be reused):
  > Seamless tileable texture, 1024 × 1024: dark wooden floor planks, warm brown, low contrast, a little wear, no
  > focal point, no text, even light so it tiles without seams.

## To make: SVG, vector

Rules: `viewBox` as given, no fixed width or height, `fill` or `stroke` set to `currentColor`, no embedded raster, no
text, stroke 1.5 or heavier so they stay clear at 12 to 16 px. Each prompt follows the **SVG prefix**.

- [x] `gate-closed.svg`, `viewBox="0 0 24 40"`:
  > A small portcullis seen from the front: two stone posts, a row of vertical iron bars with pointed ends lowered
  > fully, a crossbar. Reads as "blocked".
- [x] `gate-open.svg`, `viewBox="0 0 24 40"`:
  > The same portcullis and posts as the closed gate, with the bars raised to the top third, leaving the opening
  > empty. Reads as "open".
- [x] `badge-ready.svg`, `viewBox="0 0 24 24"`:
  > A bold fantasy-style exclamation mark, centred, with an angular shaft and faceted diamond dot. Use the chunky,
  > ornamental readability of WoW's punctuation marks as inspiration while drawing an original shape.
- [x] `badge-needs-you.svg`, `viewBox="0 0 24 24"`:
  > A bold fantasy-style question mark, centred, with an angular curve and faceted diamond dot. Use the chunky,
  > ornamental readability of WoW's punctuation marks as inspiration while drawing an original shape.
- [x] `badge-assigned.svg`, `viewBox="0 0 24 24"`: > An at-sign, centred, thick stroke.
- [x] `badge-locked.svg`, `viewBox="0 0 24 24"`: > A closed padlock: rounded body, a shackle, a small keyhole.
- [x] `badge-closed.svg`, `viewBox="0 0 24 24"`: > A bold multiplication cross (×), centred, thick stroke.
- [x] `badge-cleared.svg`, `viewBox="0 0 24 24"`: > A bold check mark, centred, thick stroke.
- [x] `badge-running.svg`, `viewBox="0 0 24 24"`:
  > A hand torch: a short handle and a flame with one inner tongue. Reads as "someone is working here".
- [x] `stub-diamond.svg`, `viewBox="0 0 64 64"`:
  > A hollow diamond (a square rotated 45°) with a thick outline and nothing inside, centred with a small margin.
- [x] `boss-frame.svg`, `viewBox="0 0 240 280"` (optional), for 9-slice scaling:
  > A rectangular room frame, outline only: an ornate border with a small spike or crest at each corner and a small
  > crown notch at the top centre. The corners are 40 × 40 units and the straight edges between them are plain, so the
  > frame can stretch. The inside is empty.
- [x] `priority-gem.svg`, `viewBox="0 0 16 16"` (optional):
  > A small faceted gem seen from the front, a flat-top cut with a few facet lines.

## Not needed

Hero tokens (generated in code, M6); the progress ring and the lines between rooms (drawn in code); cleared, locked and
running looks (code effects).

## Delivery

Put new files under `design/map-art/` named as above, and tick the box here. The sizes are the largest we expect to
display at 2x density.
