# Claude Design sync notes

- The frontend is Svelte, so the converter (React only) does not apply. This is a hand-built **styles-only** bundle:
  `node .design-sync/build.mjs <out-dir>` from the repo root, then upload `<out-dir>` (fonts in their own call).
- `_ds_bundle.css` is QuestDetail.svelte's `<style>` block (launch bar, briefing dialog) followed by `web/src/app.css`
  with font URLs rewritten to `./fonts/`. If other component-scoped styles become part of the visual baseline, add
  their files to the list in `build.mjs`.
- Cards are hand-authored fragments in `.design-sync/cards/<Group>/<Name>.html` + `.prompt.md`, wrapped by the build
  in `<div class="app" data-theme="dungeon" data-font="almendra">`. Markup copies the real Svelte templates; keep them
  in step when those templates change.
- Below 1180px `.quests` is a hidden drawer (`position: fixed`, translated out); cards render it with
  `class="quests open"` and inline `position:static;transform:none`.
- `.actions button` repaints every child as the accent primary, so `danger`/`ghost` must sit outside `.actions`.
- No `_ds_sync.json` anchor (off-script layout): every re-sync re-verifies and re-uploads everything, which is cheap.
- Verification: render each card in Chromium (fonts loaded, no failed requests) and look at the screenshots; check
  every class/token named in `conventions.md` exists in the built `_ds_bundle.css`.
