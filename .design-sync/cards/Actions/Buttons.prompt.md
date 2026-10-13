Buttons: every variant is a plain `<button>` with one of the app's classes; no button component exists.

| Markup | Look |
|---|---|
| `<button className="hero">Opus</button>` | red-gradient launch button for a hero (inside `.heroes`) |
| `<button className="shell">Shell</button>` | dark framed secondary button (also `.quests-toggle`) |
| `<button className="launch-button">Review launch</button>` | large Cinzel uppercase primary call to action |
| `<div className="actions"><button>Try again</button><button className="secondary">Cancel</button></div>` | accent primary + outlined secondary |
| `<button className="danger">Remove</button>` | destructive |
| `<button className="ghost">⋯</button>` | icon-only, no frame |
| `<button className="heading-add">＋</button>` | small framed add button inside `.sidebar-heading` |
| `<div className="viewtoggle"><button aria-pressed="true">Map</button><button aria-pressed="false">List</button></div>` | segmented toggle |
| `<button className="statefilter" aria-pressed="true">Ready <span className="n">4</span></button>` | filter pill (inside `.statefilters`) |
| `<div className="sidebar-footer"><button className="settings-button">⚙ Settings</button><button className="settings-button minor">Word guide</button></div>` | sidebar footer buttons |

`danger` and `ghost` must not sit inside `.actions` (its rule repaints every child button as the accent primary).
Selected/pressed state is `aria-pressed="true"`, not a class. Disabled is the `disabled` attribute.
