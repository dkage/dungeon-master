# Design resources

This directory holds visual references and standalone prototypes for the
Dungeon UI. These files document design decisions and help generate or review
future UI work; they are not part of the app's runtime bundle.

## Folders

| Folder                                            | Purpose                                                                                                               | Current role                                                                                                                                                                        |
|---------------------------------------------------|-----------------------------------------------------------------------------------------------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [`stitch-prototype/`](stitch-prototype/README.md) | Full cockpit screens exported from Google Stitch: map, list, and blocked-quest states.                                | Current visual baseline for the Dungeon theme. Re-export into a new version folder when the Stitch design changes.                                                                  |
| [`quest-encounters/`](quest-encounters/README.md) | Original standalone study of the right quest panel, 15-creature effort roster, stable assignment guidance, sample quest data, and art direction. | Sprite and creature mapping reference; the three original sheets anchor 12 matching additions. `stitch-prototype/` remains the overall layout reference. |
| [`font-study/`](font-study/README.md)             | Offline comparison page and licensed font files for serif candidates, shown with Portuguese sample text.              | Typography reference. Almendra, EB Garamond, and Alegreya are the chosen selectable font candidates; the study remains useful for revisiting typography or exploring future themes. |

## Using these materials

- Start with `stitch-prototype/` for the current overall composition and
  `docs/design/ui.md`, `docs/design/map.md`, and `GLOSSARY.md` for product behavior and
  terminology.
- Use `font-study/` when evaluating the three serif choices, including their
  Portuguese glyph coverage. Its font binaries are study assets; copy only the
  selected production assets and matching license texts into the app when
  implementing them.
- Use `quest-encounters/` for sprite and creature mapping. `CREATURE_ROSTER.md`
  lists the 15 effort-tier creatures and stable assignment rule; its README
  identifies which quest fields remain illustrative.
- Treat exported HTML and images as references. Rebuild production UI in the
  app rather than copying generated markup directly.
