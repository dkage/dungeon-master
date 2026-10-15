# Claude Design sync notes

- The frontend is Svelte, so the converter (React only) does not apply. This is a hand-built **styles-only** bundle:
  `node .design-sync/build.mjs <out-dir>` from the repo root, then upload `<out-dir>` (fonts in their own call).
- `_ds_bundle.css` is the `<style>` blocks of QuestDetail.svelte (launch bar, briefing dialog) and QuestMap.svelte,
  followed by `web/src/app.css` with font URLs rewritten to `./fonts/`. QuestMap's rules reuse names the app also uses
  (`.badge`, `.num`, `.link`, `.gem`, `.empty`), so `build.mjs` prefixes each of its selectors with `.map`; match the
  root as a whole class (`.maptools` and `.plain-dot` are not the root). If other component-scoped styles become part
  of the visual baseline, add their files to the list in `build.mjs`, scoped the same way when they reuse names.
  SessionTabs (`.tabs`), SkillChips (`.chips`) and StopConfirm (`.stop-confirm`) are scoped that way too. QuestDetail
  stays unscoped (app.css theme rules target its classes), but the names it shares with the app get a parent through
  `QUALIFY` in `build.mjs` (`.where` -> `.brief-dialog .where`, `.opt` -> `.launch-footer .opt`); add to it when a new
  QuestDetail class collides.
- Scoping only adds rules; app-wide rules still reach component markup in the real app. Two visible cases the cards
  reproduce faithfully: `.app[data-theme='dungeon'] .label` (GitHub labels) restyles the session tab buttons
  (`.tabs .label`), and the toolbar's global `.where { display: flex }` lays out the launch dialog's "Where the session
  runs" fieldset. Fix them in the app, not in the cards.
- A `url()` inside a custom property (the map's `--floor`) resolves against the stylesheet that uses it, so captured
  `--x: url(ASSETS/...)` is rewritten to `assets/...` (stylesheet-relative), not `../../../assets/...`.
- The map cards are not hand-written: `capture/run.sh` renders the real `QuestMap` against fixtures in jsdom (copied
  into a temporary `web/src/ds-capture-tmp/` because vitest needs `web/node_modules`, removed on exit) and the build
  inserts the HTML at `<!-- capture:map-dungeon -->` / `<!-- capture:map-world -->`. Image URLs come out as
  `/src/assets/...` and are rewritten to the bundle's `assets/`.
- `assets/` ships the map art (`web/src/assets/map/`) and the three Stitch prototype renders, downscaled to 1600px
  JPEGs with macOS `sips`, for the Reference cards. The prototype is frozen; the element cards win where they differ.
- Cards are hand-authored fragments in `.design-sync/cards/<Group>/<Name>.html` + `.prompt.md`, wrapped by the build
  in `<div class="app" data-theme="dungeon" data-font="almendra">`. Markup copies the real Svelte templates; keep them
  in step when those templates change.
- Below 1180px `.quests` is a hidden drawer (`position: fixed`, translated out); cards render it with
  `class="quests open"` and inline `position:static;transform:none`.
- `.actions button` repaints every child as the accent primary, so `danger`/`ghost` must sit outside `.actions`.
- No `_ds_sync.json` anchor (off-script layout): every re-sync re-verifies and re-uploads everything, which is cheap.
- Verification: render each card in Chromium (fonts loaded, no failed requests) and look at the screenshots; check
  every class/token named in `conventions.md` exists in the built `_ds_bundle.css`.

## Changing a design

The sync is one-way (repo -> Claude Design); the app is the source of truth. Never edit the design-system project
itself: the next sync re-uploads everything and overwrites it.

1. Explore in a separate Claude Design project that uses the Dungeon Master design system.
2. Bring the result back to the code: read that project's files (or a screenshot plus its CSS), then change the real
   rules in `web/src/app.css` or the component `<style>` blocks, mapping hard-coded colours to the existing tokens
   (`--accent`, `--brass`, ...). Update the Svelte markup if the structure changed, with tests (`make check`), and the
   matching card in `.design-sync/cards/` if its markup changed.
3. Run `/design-sync` so the design system catches up with the app, then commit when asked.

Exploration projects keep their own copy of the markup. Shared CSS changes reach them when they re-render; markup
changes only when Claude Design redoes them. Small tweaks (radius, a hover colour) can skip Claude Design: change the
code directly and re-sync.
