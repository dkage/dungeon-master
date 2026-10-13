---
status: planned
milestone: workspace usability
github_issues: [63, 64, 65]
tracking: in-gh-issues
next: review/publish the committed map increment, then implement expansion and configurable shortcuts
verified: 2026-10-13
---

# Workspace focus and shortcuts

The default remains the existing three-panel cockpit and its visual design. All presentations share campaigns,
sessions, selected issue, map level, filters and geometry. Planning, running and reviewing are activities in this
workspace, not separate copies of it. This document specifies future behavior, not implemented features.

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

Implement a small typed workspace state (`expandedRegion`, saved split state), not a general split-pane tree. Buttons,
shortcuts and future native menus call the same allowlisted semantic actions. Keep issue selection and terminal
ownership independent of layout.

## Shortcuts settings

Settings gets a dedicated Shortcuts page reachable in every layout. Each row shows a semantic action, current binding,
platform default and availability. Users can change or disable a binding, reset one row, or restore defaults. Validate
before saving: duplicate/conflicting bindings identify the other action, invalid syntax leaves the last valid choice,
and a disabled action has a reason. Binding a key never changes execution configuration.

Initial actions: expand each of the four regions, restore split, show map, show list, open settings, and next/previous
session. A command palette can use the same action registry later; do not show nonfunctional palette or launch actions.

Default candidates to validate during implementation: Ctrl+Shift+1 through 4 for expansion, Ctrl+Shift+0 to restore,
Ctrl+, for Settings in browser mode, and Ctrl+PageDown/PageUp for session navigation where the host permits them.
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
