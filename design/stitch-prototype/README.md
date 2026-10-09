# Stitch prototype: Gothic Codex (v4)

The most recent full-window prototype of the cockpit, generated in Google Stitch (project "Dungeon Master Desktop
Cockpit"). It is the visual baseline for the Dungeon pack, the default theme (`PLAN_UI.md` §6.5): left panel, terminal
and right panel with the quest map and detail, in the gothic, Diablo-like style.

It supersedes the first prototype, `design/quest-encounters/` (a right-panel study); that folder's `ART_DIRECTION.md`
still describes the material and hierarchy rules.

Open any `index.html` in a browser. The pages work offline: images and fonts live in `assets/`.

| Folder       | Stitch screen | State shown                                                        |
|--------------|---------------|--------------------------------------------------------------------|
| `v4/`        | v4            | map view, ready quest #3 selected, its expedition running          |
| `v4-list/`   | v4-list       | list view with state filters, ready quest #3 selected              |
| `v4-list-2/` | v4-list-2     | list view, blocked quest #4 selected: Start locked, blocker linked |

Each folder holds the exported `index.html` and Stitch's own render (`screenshot.png` or `.jpg`, 2560×2048) to compare
against.

## How to use it

- **Copy the look, not the code.** The HTML is generated, with inline styles and scripts. The frontend rebuilds it in
  Svelte with design tokens, icons by meaning and the words table (`PLAN_UI.md` §6.5); no component is copied as is.
- **Behaviour comes from the plans.** Where the prototype differs from `PLAN_UI.md`, `PLAN_MAP.md` or `GLOSSARY.md`
  (labels, controls, which button does what), the plans win.
- **The content is sample data.** Quest titles follow the demo campaign; the terminal log, effort tiers and model
  names are illustrative.

## Files

- `assets/images/`: sprite strips, avatars and textures, named by a hash of their Stitch URL.
- `assets/fonts/`: Cinzel, MedievalSharp and Fira Code (SIL Open Font License), with `fonts.css`. Licence texts are
  not included, and the parchment text falls back to Georgia, which cannot ship; see `PLAN_UI.md` §9, open question 3.
- `assets/manifest.json`: each local file and the URL it was downloaded from.

Exported on 2026-10-09. The Stitch project keeps evolving; re-export into a new folder rather than editing these files.
