# Art checklist

Every image the map and cockpit use or need: what we have, and what is still to be made. Tick a box when the file is in
the repository and checked at native size next to the existing creatures. The plan it serves is `docs/design/map.md`.

**Style for every new piece:** the same family as the existing creatures. Original gothic dungeon art, crisp dark
silhouette, restrained painterly pixel-art influence, cool blue ambient and rim light with small warm amber
highlights, muted stone, wood and iron. Three-quarter front view. No text, logos, UI, gore or recognisable franchise
designs. Use `quest-encounters/assets/rat-idle-strip.png`, `goblin-idle-strip.png` and `many-eyed-idle-strip.png` as
style references.

**Raster or vector:** painted subjects with light and texture are PNG (JPG when opaque). Anything that must take the
theme's colours, scale crisply or be drawn at 12 to 24 px is SVG. State changes (cleared, locked, running, needs you)
are done in code with filters, glows and overlays, so no extra art variants are needed for them.

## Already in the repository

### Creatures (`design/quest-encounters/assets/`)

Each is one sheet of four idle frames, transparent, 2172 × 724 px (cells 543 × 724). Tiers are in
`quest-encounters/CREATURE_ROSTER.md`.

- [x] Trivial: `rat-idle-strip.png`, `slime-idle-strip.png`, `bat-idle-strip.png`
- [x] Low: `kobold-idle-strip.png`, `beetle-idle-strip.png`, `spider-idle-strip.png`
- [x] Moderate: `goblin-idle-strip.png`, `skeleton-idle-strip.png`, `ghoul-idle-strip.png`
- [x] High: `orc-idle-strip.png`, `troll-idle-strip.png`, `warlock-idle-strip.png`
- [x] Large: `many-eyed-idle-strip.png`, `lich-idle-strip.png`, `dragon-idle-strip.png`

### Textures

- [x] `quest-encounters/assets/stone-floor.png`: dark stone floor, 1254 × 1254, opaque. The dungeon view's backdrop.
- [x] `stitch-prototype/assets/images/stone-floor.png`: the same floor at 512 × 512 (prototype copy).
- [x] `stitch-prototype/assets/images/6a8650faf410.png` and `97785ef87771.png`: one parchment texture, byte-identical,
  288 × 512, opaque. The terminal's page background. AI-generated (confirmed by the author).

### Prototype-only copies

- [x] `stitch-prototype/assets/images/rat-idle-strip.png`, `goblin-idle-strip.png`, `many-eyed-idle-strip.png`:
  512 × 171 downscaled copies of the creature strips. Use the full-size ones from `quest-encounters/`.

### Fonts

- [x] Cinzel, Almendra, EB Garamond, Alegreya, Fira Code, bundled in `web/src/assets/fonts/` (M3.6).

### Removed

- The slate texture (a screenshot from another project) and two screenshots of other projects' fantasy UI kits were
  taken out of the prototype. Its pages now show their dark gradient without the texture layer. A new slate texture is
  on the list below. The app itself never used any of them; it draws its panels with CSS only.

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

- [ ] `dungeon-1-burrow.png`, 512 × 512: smallest dungeon, shown at about 128 to 160 px on the world map.
  > Subject: a small burrow or cave mouth in a mossy rock mound, a few stones around it, a faint warm glow inside. The
  > smallest and least threatening of five dungeon sizes.
- [ ] `dungeon-2-crypt.png`, 512 × 512:
  > Subject: a crypt entrance cut into a hillside: short stone steps down to a sealed iron-banded door under a carved
  > arch, two small unlit braziers. Slightly larger and grimmer than a burrow.
- [ ] `dungeon-3-tower.png`, 512 × 512:
  > Subject: a ruined stone watchtower with a broken top, one lit arrow-slit window, ivy and fallen stones at the base.
- [ ] `dungeon-4-fortress.png`, 512 × 512:
  > Subject: a fortress gatehouse with two squat towers and a short wall, a closed portcullis, torches on the wall and
  > a banner torn at the edges.
- [ ] `dungeon-5-citadel.png`, 512 × 512: the largest, a ladder step above the fortress.
  > Subject: a tall citadel of many spires on a rocky crag, ember-red light in the windows and wisps of smoke, an
  > ominous dragon's-lair feel. The most imposing of the five.
- [ ] `tavern.png`, 512 × 512:
  > Subject: a cosy, slightly worn timber-and-stone tavern with warm lit windows, a hanging sign (blank, no text), and
  > a notice board beside the door with several pinned paper notes (no readable text). Warmer and friendlier than the
  > dungeons, with the same lighting style.

The five dungeons must read as a ladder from small to grand when placed side by side; generate them in one session so
the scale and style stay consistent. A flicker of torchlight is done in code.

## To make: backgrounds, tileable, opaque

- [ ] `slate-panel.jpg`, 512 × 512, seamless, JPG quality 85: replaces the removed slate texture, behind the side
  panels.
  > Seamless tileable texture, 512 × 512: dark slate rock, near black with a hint of warm brown, fine grain and a few
  > shallow cracks, low contrast, even light with no focal point, no text. Original, not derived from any photograph
  > or game asset. Must tile without visible seams.
- [ ] `world-floor.jpg`, 1024 × 1024, seamless, JPG quality 85:
  > Seamless tileable texture, 1024 × 1024: dark worn stone with a very faint hint of old paths and cracks, low
  > contrast, nearly black-blue, no focal point, no text. Light and detail must be even across the whole image so it
  > tiles without visible seams. Cool blue ambient, a trace of warm tone. Same material family as the attached stone
  > floor.
- [ ] `tavern-floor.jpg`, 1024 × 1024, seamless, JPG quality 85: optional; the stone floor can be reused.
  > Seamless tileable texture, 1024 × 1024: dark wooden floor planks, warm brown, low contrast, a little wear, no
  > focal point, no text, even light so it tiles without seams.

## To make: SVG, vector

Rules: `viewBox` as given, no fixed width or height, `fill` or `stroke` set to `currentColor`, no embedded raster, no
text, stroke 1.5 or heavier so they stay clear at 12 to 16 px. Each prompt follows the **SVG prefix**.

- [ ] `gate-closed.svg`, `viewBox="0 0 24 40"`:
  > A small portcullis seen from the front: two stone posts, a row of vertical iron bars with pointed ends lowered
  > fully, a crossbar. Reads as "blocked".
- [ ] `gate-open.svg`, `viewBox="0 0 24 40"`:
  > The same portcullis and posts as the closed gate, with the bars raised to the top third, leaving the opening
  > empty. Reads as "open".
- [ ] `badge-ready.svg`, `viewBox="0 0 24 24"`: > A bold exclamation mark, centred, thick stroke.
- [ ] `badge-needs-you.svg`, `viewBox="0 0 24 24"`: > A bold question mark, centred, thick stroke.
- [ ] `badge-assigned.svg`, `viewBox="0 0 24 24"`: > An at-sign, centred, thick stroke.
- [ ] `badge-locked.svg`, `viewBox="0 0 24 24"`: > A closed padlock: rounded body, a shackle, a small keyhole.
- [ ] `badge-closed.svg`, `viewBox="0 0 24 24"`: > A bold multiplication cross (×), centred, thick stroke.
- [ ] `badge-cleared.svg`, `viewBox="0 0 24 24"`: > A bold check mark, centred, thick stroke.
- [ ] `badge-running.svg`, `viewBox="0 0 24 24"`:
  > A hand torch: a short handle and a flame with one inner tongue. Reads as "someone is working here".
- [ ] `stub-diamond.svg`, `viewBox="0 0 64 64"`:
  > A hollow diamond (a square rotated 45°) with a thick outline and nothing inside, centred with a small margin.
- [ ] `boss-frame.svg`, `viewBox="0 0 240 280"`, optional, for 9-slice scaling:
  > A rectangular room frame, outline only: an ornate border with a small spike or crest at each corner and a small
  > crown notch at the top centre. The corners are 40 × 40 units and the straight edges between them are plain, so the
  > frame can stretch. The inside is empty.
- [ ] `priority-gem.svg`, `viewBox="0 0 16 16"`, optional:
  > A small faceted gem seen from the front, a flat-top cut with a few facet lines.

## Not needed

Hero tokens (generated in code, M6); the progress ring and the lines between rooms (drawn in code); cleared, locked and
running looks (code effects).

## Delivery

Put new files under `design/map-art/` named as above, and tick the box here. The sizes are the largest we expect to
display at 2x density.
