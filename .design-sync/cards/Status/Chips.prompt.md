Chips, pills, state tags, labels and status dots: `<span>`s with the app's classes; modifiers are extra classes.

- `chip` (+ `lock`, `pr open`, `pr merged`): small outlined metadata, inside `<span className="meta">`.
- `label` with `style={{'--gh': '#d73a4a'}}` and a child `<i className="swatch" />`: a GitHub label. In Dungeon it
  renders as a dark tone with a colour swatch.
- `statetag` + `ready | running | needs-you | assigned | locked | cleared`: quest state at the end of a list row.
- `pill` + `open | claimed | locked | effort-chip | source-chip`: detail metadata, inside `<p className="dmeta">`.
- `dot` + `running | idle | failed | done | lost | needs`: session status dot (`needs` holds a `!`).
- `badge` has `margin-left: auto` (it sits at the end of a session row); override it when placed inline.
- `badge`, `activity-chip` (+ `needs`), `rations` (+ `low`): counters and live activity.

These live inside the side panels, which set `font-family: var(--reading-font)`; outside them, set it yourself.
