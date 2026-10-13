Colour tokens of the Dungeon appearance (default) and the Neutral one; use `var(--name)`, never raw hex for UI chrome.

Tokens are defined on `.app[data-theme='dungeon']` (and `.app[data-theme='neutral']`), so they only resolve inside an
element with `class="app" data-theme="dungeon"`.

| Token | Use |
|---|---|
| `--bg` | page / deepest background |
| `--panel`, `--panel-raised`, `--card` | panel surfaces; raised cards use `linear-gradient(180deg, var(--panel-raised), var(--card))` |
| `--line` | borders and dividers |
| `--metal` | card frame border (Dungeon) |
| `--text`, `--muted` | body text, secondary text |
| `--accent`, `--accent-text` | gold accent fill and the text on it |
| `--brass`, `--brass-bright` | brass trim, inset frame lines, highlighted numbers |
| `--warn`/`--warn-bg`, `--error`/`--error-bg` | notices and danger actions |
| `--term-bg` | terminal parchment |
