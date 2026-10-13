# Stitch prototype: Gothic Codex (v4)

A frozen, one-off full-window prototype of the cockpit, generated in Google Stitch (project "Dungeon Master Desktop
Cockpit"). It is the visual baseline for the Dungeon pack, the default theme (`docs/design/ui.md` §6.5): left panel, terminal
and right panel with the quest map and detail, in the gothic, Diablo-like style.

It supersedes the first prototype, `design/quest-encounters/` (a right-panel study). The material and hierarchy rules
are in `docs/design/art-direction.md`.

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
  Svelte with design tokens, icons by meaning and the words table (`docs/design/ui.md` §6.5); no component is copied as is.
- **Behaviour comes from the plans.** Where the prototype differs from `docs/design/ui.md`, `docs/design/map.md` or `GLOSSARY.md`
  (labels, controls, which button does what), the plans win.
- **The content is sample data.** Quest titles follow the demo campaign; the terminal log, effort tiers and model
  names are illustrative.

## Files

- `assets/images/`: the creature strips, the stone floor, the slate texture (generated for this project) and the parchment texture (which keeps the hash of its Stitch URL).
- `assets/fonts/`: Cinzel, MedievalSharp and Fira Code (SIL Open Font License), with `fonts.css` and each font's
  licence text (`*-OFL.txt`). The parchment text falls back to Georgia, which cannot ship; see `docs/design/ui.md` §9, open question 3.
- `assets/manifest.json`: each local file and the URL it was downloaded from.

Exported on 2026-10-09. Stitch is no longer used, so there will be no further exports; do not edit these files. New
design work goes through the Claude Design project kept in sync by `.design-sync/`.
