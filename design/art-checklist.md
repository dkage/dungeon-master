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
- [x] `stitch-prototype/assets/images/103e1602d0e3.png` and `8c39cd52d48b.png`: one slate texture, byte-identical,
  511 × 512, opaque. Panel backgrounds in the prototype. **Source unknown** (see below).
- [x] `stitch-prototype/assets/images/6a8650faf410.png` and `97785ef87771.png`: one parchment texture, byte-identical,
  288 × 512, opaque. The terminal's page background. **Source unknown** (see below).

### Prototype-only copies

- [x] `stitch-prototype/assets/images/rat-idle-strip.png`, `goblin-idle-strip.png`, `many-eyed-idle-strip.png`:
  512 × 171 downscaled copies of the creature strips. Use the full-size ones from `quest-encounters/`.

### Fonts

- [x] Cinzel, Almendra, EB Garamond, Alegreya, Fira Code, bundled in `web/src/assets/fonts/` (M3.6).

### To check before shipping

- [ ] **Provenance of the slate and parchment textures.** They came out of the Stitch export and look like stock
  photography. Confirm they are Stitch-generated or licensed, or replace them with generated originals (see
  `CLAUDE.md`: independent design, no copied assets). The app does not use any of them yet; it draws its panels with
  CSS only.
- [ ] **`597429b08137.png` (355 × 512) and `993ca6871b69.png` (512 × 432) should leave the repository.** They are
  screenshots of someone else's fantasy UI kits (an info-box kit and a sign-up form). The prototype names them only in
  a CSS comment ("inspired by"), and nothing renders them. Delete them and their manifest entries unless there is a
  licence.

## To make: painted, PNG with transparent background

- [ ] `dungeon-1-burrow.png`, 512 × 512: smallest dungeon, a cave mouth or burrow. Shown at about 128 to 160 px on the
  world map.
- [ ] `dungeon-2-crypt.png`, 512 × 512: a crypt entrance with steps and a sealed door.
- [ ] `dungeon-3-tower.png`, 512 × 512: a ruined watchtower or chapel.
- [ ] `dungeon-4-fortress.png`, 512 × 512: a fortress gate with walls.
- [ ] `dungeon-5-citadel.png`, 512 × 512: the largest, a citadel or castle with a dragon's-lair feel.
- [ ] `tavern.png`, 512 × 512: a tavern with lit windows and a notice board with pinned notes (no readable text).

The tier follows the dungeon's size and danger, so the five read as a ladder from small to grand. Each is one static
frame, centred, small footing only (no ground plane or cast shadow beyond the sprite), the base of the building at the
bottom edge, 16 px of transparent margin. A flicker of torchlight is done in code.

## To make: backgrounds, tileable, opaque

- [ ] `world-floor.jpg`, 1024 × 1024, seamless, JPG quality 85: dark worn stone or parchment, very low contrast, with a
  faint hint of old paths. The world map's backdrop; nodes must stay readable on it.
- [ ] `tavern-floor.jpg`, 1024 × 1024, seamless, JPG quality 85: dark wooden planks, warm and low contrast. Optional;
  the stone floor can be reused.

## To make: SVG, vector

Rules: `viewBox` as given, no fixed width or height, `fill` or `stroke` set to `currentColor`, no embedded raster, no
text, stroke 1.5 or heavier so they stay clear at 12 to 16 px.

- [ ] `gate-closed.svg`, 24 × 40: a small portcullis on a blocked-by line, next to the blocked room.
- [ ] `gate-open.svg`, 24 × 40: the same gate raised, once the blocker is closed.
- [ ] `badge-ready.svg`, 24 × 24: `!` mark.
- [ ] `badge-needs-you.svg`, 24 × 24: `?` mark.
- [ ] `badge-assigned.svg`, 24 × 24: `@` mark.
- [ ] `badge-locked.svg`, 24 × 24: padlock.
- [ ] `badge-closed.svg`, 24 × 24: `×` mark.
- [ ] `badge-cleared.svg`, 24 × 24: check mark.
- [ ] `badge-running.svg`, 24 × 24: a small torch or a hero's helm; a session is working here.
- [ ] `stub-diamond.svg`, 64 × 64: hollow diamond for a quest outside the scope or with unknown effort.
- [ ] `boss-frame.svg`, 240 × 280, designed for 9-slice scaling: an ornate room frame for the last quest of a blocker
  chain. Corners fixed, edges stretch. Optional.
- [ ] `priority-gem.svg`, 16 × 16: a small jewel for urgent priority. Optional.

## Not needed

Hero tokens (generated in code, M6); the progress ring and the lines between rooms (drawn in code); cleared, locked and
running looks (code effects).

## Delivery

Put new files under `design/map-art/` named as above, and tick the box here. The sizes are the largest we expect to
display at 2x density.
