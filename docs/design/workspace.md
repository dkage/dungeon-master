---
status: planned
milestone: workspace usability
github_issues: [63, 64, 65, 78, 84, 87, 88, 160, 163]
tracking: in-gh-issues
---

# Workspace focus and shortcuts

The default remains the existing three-panel cockpit and its visual design. All presentations share campaigns,
sessions, selected issue, map level, filters and geometry. Planning, running and reviewing are activities in this
workspace, not separate copies of it. Panel expansion (#64) column resizing (#78), reload restore (#84, #87) configurable shortcuts (#65) and the terminal comfort work (#88, below) are implemented; the rest of this document specifies future behavior.

## Panel expansion

Provide one visible Expand button in each region's header: projects/sessions, terminal, map/list, and selected issue
detail. A click expands that region to fill the application workspace; its button becomes Restore split. Another
click returns to the previous layout. This is application layout, not operating-system or browser fullscreen.

- Only one region is expanded. Expanding another changes the target without replacing the saved split geometry.
- Keep non-visible regions mounted and inaccessible to pointer and keyboard interaction. Live terminals keep their
  sessions and scrollback; expanding or restoring never spawns, reconnects or terminates an agent as a side effect.
- Preserve map/list choice, map level, selected issue, list filters, scroll positions, selected session, drawer state
  and split sizes. Resize xterm and the map when their container changes, including after restoration.
- Expanded headers retain project/session or issue identity, Settings access and a visible Restore split button.
  Labels and tooltips name the target, e.g. Expand terminal or Restore split. Controls expose their pressed state.
- Restore keyboard focus to the invoking control if it still exists. Opening a selected session or issue makes the
  corresponding region visible; it must not leave focus inside a hidden region.
- Escape closes the active dialog first. Outside text editing and the terminal, it restores an expanded region before
  following map-level navigation. Escape inside a terminal remains terminal input; use Restore split or its binding.
- On narrow screens, expansion uses the same action and restores the previous responsive layout/drawer. It must not
  leave an invisible overlay blocking input. Start each fresh page in split mode so a remembered hidden region does
  not obscure onboarding; remember split dimensions independently when that geometry is implemented.

Decision (#164, 2026-10-14): **every fresh page starts in split mode.** The reload restore (#84) keeps the project,
issue, session and map view; the expanded panel is not remembered and is not part of a shared link. Probe: expanding the
terminal, the projects panel and the map, then reloading, always returned to the split with the same session selected
(`web/e2e/workspace.spec.ts`). Rejected: *remember the expanded panel* hides the other regions from a returning user who
may not recall why they are gone, and a link to an issue would open into a half-hidden window; *remember it only for the
terminal* has the same problem for the projects list and the issue detail the link points at, and the terminal is one
shortcut press away (Ctrl+Shift+2) while the session and issue it needs are already restored. No follow-up code is
needed. Reopen the question if users report re-expanding on every visit.

Narrow layouts (#165): browser tests expand and restore all four regions at a drawer width (900 px) and at phone width
(390 px); each restore returns the quests drawer to the state it had and leaves the terminal clickable and typeable.
An open drawer covers the toolbar's Quests button, so it closes with its own × button.

Implement a small typed workspace state (`expandedRegion`, saved split state), not a general split-pane tree. Buttons,
shortcuts and future native menus call the same allowlisted semantic actions. Keep issue selection and terminal
ownership independent of layout.

## Column resizing

The three columns (projects/sessions, terminal, right panel) are resized by dragging the two borders between them
(#78), the same way the map/list and detail split already works inside the right panel.

- Each border is a focusable separator: arrow keys step its width, Home/End go to the limits, and a double-click or a
  key returns to the default split. Hover and focus are visible in both appearances.
- Minimum widths keep every panel usable; the terminal keeps the largest share by default and never drops below a
  working size.
- The terminal refits its columns and rows while a border moves and resizes the running session, with updates
  coalesced during a drag. Scrollback and the session survive every resize.
- The other panels reflow: text wraps or truncates with a tooltip, controls wrap instead of overflowing, and the map
  refits to its area.
- Column widths are remembered per browser next to the map/detail split. Saved widths that no longer fit the window
  fall back to defaults; below the drawer breakpoint the narrow layout applies and saved widths never hide a panel.
- The saved column widths are the split geometry that Restore split (§Panel expansion) returns to.

## Remembering place across reloads

Decision (#87, chosen by the project owner): keep "where was I" in **both the address-bar hash and browser
storage**.

- The hash carries identifiers only: `#c=<campaign>&s=<session>&q=<issue number>&v=map|list&l=<map level>`
  (`l` is `t` for the Tavern or `d<epic number>`; the world level is omitted). It never carries the access token, issue
  text or scrollback. Reload, back/forward and a link copied to another window all work.
- `localStorage` (`dungeon.place`) holds the same string, used only when the URL has no hash, so a bare URL returns
  to the last place. The hash wins when both exist.
- Campaign, session and issue changes push a history entry; view and map-level changes replace the current one.
- Fallback: each field is validated on its own. An unknown campaign, an ended or lost session, or an issue that no
  longer loads is dropped silently and the default applies; malformed or unreadable data is ignored. Restoring never
  starts, restarts or ends a session; it reconnects the terminal of a session that is still live.
- Rejected: storage only (no deep links, back/forward or sharing a link) and address bar only (a bare URL always lands
  on the default view).

## Shortcuts settings

Settings gets a dedicated Shortcuts page reachable in every layout. Each row shows a semantic action, current binding,
platform default and availability. Users can change or disable a binding, reset one row, or restore defaults. Validate
before saving: duplicate/conflicting bindings identify the other action, invalid syntax leaves the last valid choice,
and a disabled action has a reason. Binding a key never changes execution configuration.

Initial actions: expand each of the four regions, restore split, show map, show list, open settings, and next/previous
session. A command palette can use the same action registry later; do not show nonfunctional palette or launch actions.

Implemented defaults: Ctrl+Shift+1 through 4 for expansion (Control, not Cmd, on macOS, where Cmd+Shift+3/4 are
screenshots), Ctrl+Shift+0 to restore, Mod+, (Cmd on macOS) for Settings, and Ctrl+PageDown/PageUp for session navigation where the host permits them. Terminal output search is Ctrl+Shift+F and jumping to the latest output is Ctrl+Shift+End (#91); plain Ctrl+F and Ctrl+End stay with the agent.
Use platform labels and record native-shell alternatives separately; desktop/browser conflicts need practical checks.
Settings must explain when the browser or operating system owns a proposed key and keep the button alternative.
Do not intercept browser close/quit/reload keys, unmodified typing or standard clipboard shortcuts.

- Store validated overrides in local UI settings. Missing/malformed preferences fall back per action and remain
  recoverable. Surface persistence failure in Settings instead of silently claiming the changes were saved.
- Ignore IME composition and unintended repeat. Text inputs, contenteditable controls and the terminal own ordinary
  editing keys; application shortcuts in those contexts require explicit modifier bindings tested for conflicts.
- Dialogs have priority; Escape must never both close a dialog and navigate the map. A settings binding editor captures
  keys for its own input instead of triggering actions. Plain single-letter global shortcuts are excluded.
- Native menus and browser handlers share action identifiers; native events must not dispatch twice. Native menu
  wiring remains desktop-shell work and never executes arbitrary JavaScript from a user binding.

## Acceptance and prototype work

First finish and verify the existing map/list implementation. Then prototype expanded terminal, map/list and issue
reading at ordinary desktop and narrow sizes using the existing assets. Assess real CLI legibility, not the prototype's
illustrated transcript. Keep the RPG theme focused on meaningful work states and retain the neutral rendering.

Unit tests cover state transitions, restoration, binding validation and context routing. Browser tests expand and
restore every region, switch targets, change and reload a shortcut, and show that a live stub session keeps its identity
and output. Check keyboard focus, hidden-region accessibility, modal ordering and responsive restoration.

## Terminal comfort (#88)

All implemented; the evidence is each child issue's tests. Decisions that are not obvious from the code:

- Shift+Enter sends ESC CR (`\x1b\r`), which the real Claude prompt reads as a new line; plain Enter stays CR (#85,
  probe #89). The kitty, modifyOtherKeys, backslash-Enter and LF forms also worked; ESC CR is the plainest.
- Pasted images and dropped files are stored by the server under `<state dir>/uploads/<session id>/` (10 MB, images,
  PDF and plain text sniffed from the bytes, sanitised names, removed with the session) and their quoted path is
  inserted with xterm's paste, which the Claude prompt turns into `[Image #N]`. Typing the path does not attach it, and
  Ctrl+V is not forwarded because the CLI would read the clipboard of the machine, not the browser's (#86, #90).
- Appearance settings live in `appearance.json` in the config dir and are served by `/api/appearance`; the browser
  keeps a cache for the first paint and hands its old choices over once (#149). The terminal font, line height and
  colour scheme are stored there too (#150, #153, #154); a typed font that is not installed is reported (#158).
- Search and jump-to-latest use Ctrl+Shift+F and Ctrl+Shift+End; the zoom actions have no default binding because
  the browser owns Ctrl and Cmd with plus, minus and zero (#91, #92). Copy and Save take the scrollback as text (#142).
