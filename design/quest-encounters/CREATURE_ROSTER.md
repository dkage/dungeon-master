# Quest creature roster

This is a 15-creature design roster for encounter-map studies. It is an art
mapping, not a claim that the current GitHub API supplies effort estimates.
The three existing sheets are the style anchors and count toward the total.

## Effort tiers

| Tier | Suggested signal | Creature ID → sprite sheet |
|---|---|---|
| Trivial | Tiny, well-scoped change; roughly 1 point | `dungeon-rat` → `rat-idle-strip.png`; `cave-slime` → `slime-idle-strip.png`; `cave-bat` → `bat-idle-strip.png` |
| Low | Small, bounded change; roughly 2–3 points | `kobold-scout` → `kobold-idle-strip.png`; `cave-beetle` → `beetle-idle-strip.png`; `cave-spider` → `spider-idle-strip.png` |
| Moderate | Several steps or a few related files; roughly 3–5 points | `goblin-scout` → `goblin-idle-strip.png`; `skeleton-guard` → `skeleton-idle-strip.png`; `crypt-ghoul` → `ghoul-idle-strip.png` |
| High | Broad change, substantial uncertainty, or many dependent tasks | `orc-raider` → `orc-idle-strip.png`; `stone-troll` → `troll-idle-strip.png`; `dungeon-warlock` → `warlock-idle-strip.png` |
| Epic | Large initiative or boss-sized parent with a substantial child quest chain | `many-eyed-stone-watcher` → `many-eyed-idle-strip.png`; `crypt-lich` → `lich-idle-strip.png`; `ember-dragon` → `dragon-idle-strip.png` |

Point ranges are illustrative and should be replaced by the project's chosen
estimate source. Child count can increase the parent encounter's scale, but
should not silently replace a task's own effort estimate. Priority is a
separate signal: use it for a small jewel, frame, or urgency marker, not to
change the creature's effort tier. Keep status markers separate from both.

## Stable issue assignment

When a task has an explicit local creature override, use it. Otherwise resolve
its effort tier from an explicit estimate or configured label mapping, then
choose from that tier's roster using the first 64 bits (big-endian) of
`SHA-256("creature-assignment-v1|owner/repo#123")` modulo the sorted creature
IDs in that tier. Use the canonical repository identity and issue number; never
use a random choice, title text, current time, or login session as the seed.

Persist the chosen creature ID with the project's local issue metadata when
first assigned. This makes the choice survive issue edits and roster changes.
Treat each roster revision as additive: existing persisted IDs remain valid,
and adding creatures affects only issues first assigned under the new roster.
Keep a stable ordering and a roster revision ID for reproducible assignments
that have not yet been persisted. Hashing is a fallback; stored assignment is
the long-term source of truth.

If effort is unknown, use the neutral room mark rather than guessing from an
issue title. A parent issue with children may use a larger room frame while its
creature still reflects the parent's own effort tier.

## Sprite sheet contract

- One creature per sheet, four idle frames in a single horizontal strip.
- Transparent background; equal-width frames, same character scale and
  baseline in every frame.
- Preserve the supplied prototype contract: 2172 × 724 px, four 543 × 724 px
  cells, with the creature centered and readable at about 58 × 70 px.
- Match the rat, goblin, and stone watcher: original gothic dungeon-creature
  art; crisp dark silhouette; restrained painterly pixel-art influence; cool
  blue ambient/rim light; small warm amber highlights; muted stone, leather,
  bone, or earthy colors; clear forms at small size.
- Match their three-quarter side view, outline weight, light direction,
  shadow density, visual detail, and idle animation restraint. No scenery,
  ground plane, cast shadow outside the sprite, UI, text, logos, gore, or
  recognizable franchise designs.

Existing art: `assets/rat-idle-strip.png`, `assets/goblin-idle-strip.png`, and
`assets/many-eyed-idle-strip.png`. New generated sheets should use all three as
style references, then be inspected together at native size before acceptance.
All 15 roster sheets are currently present in `assets/`; the original three
were retained and 12 additions were generated against those three style anchors.
