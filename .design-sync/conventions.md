# Dungeon Master: how to build with this design system

This is a **styles-only** system. The product UI is written in Svelte, so there are no React components to import.
`window.DungeonMaster` is empty. Build every element as plain JSX with the app's own CSS classes, which are listed
in each card's `.prompt.md`. The styles come from the real app stylesheet, so markup that uses those classes looks
exactly like the app.

## Wrapping: required

Every design must sit inside the theme root, or nothing is themed (the tokens are scoped to it):

```jsx
<div className="app" data-theme="dungeon" data-font="almendra">…</div>
```

- `data-theme`: `dungeon` (default, gothic crimson and brass) or `neutral` (plain system palette).
- `data-font`: `almendra` (default), `eb-garamond` or `alegreya`; it sets `--reading-font` for the panels.
- `.app` is the full-window three-column grid (sidebar 246px | terminal | quest panel 410–520px). For a full screen,
  put `aside.sidebar`, `div.main` and `aside.quests` inside it. For a fragment, keep the
  wrapper but add `style={{display: 'block', height: 'auto'}}`.
- Below 1180px of width, `aside.quests` turns into a hidden drawer. Give it `className="quests open"` and
  `style={{position: 'static', transform: 'none'}}` unless the design is a full window wider than 1180px.

## Styling idiom

- Colours: `var(--bg)`, `--panel`, `--panel-raised`, `--card`, `--line`, `--metal`, `--text`, `--muted`, `--accent`,
  `--accent-text`, `--brass`, `--brass-bright`, `--warn`/`--warn-bg`, `--error`/`--error-bg`, `--term-bg`.
- The framed card look: `border: 1px solid var(--metal); border-radius: 4px;
  background: linear-gradient(180deg, var(--panel-raised), var(--card));
  box-shadow: inset 0 0 0 2px #0006, inset 0 0 0 3px var(--brass)` (as `.titlecard`).
- Type: headings `'Cinzel'` (uppercase and letter-spaced for labels), panel text `var(--reading-font)`, paths and code
  `'Fira Code'`.
- States use attributes, not classes: `aria-pressed="true"` for toggles and filters, `disabled` for unavailable.
- For your own layout glue, use inline styles with these tokens. Don't invent new class names.

## Class families (see the cards)

| Area | Classes |
|---|---|
| Shell | `app`, `sidebar`, `main`, `toolbar`, `stage`, `term-wrap`, `term`, `quests`, `splitter` |
| Sidebar | `brand-header`, `brand-mark`, `sidebar-search`, `sidebar-heading`, `section-rule`, `heading-add`, `campaigns`, `campaign`(`selected`), `row`, `pick`, `cline`, `cname`, `repo`, `campaign-badge`, `sessions`, `session`(`active`), `stitle`, `hero-tag`, `sidebar-footer`, `settings-button`(`minor`) |
| Toolbar | `where`, `session-title`, `session-light`, `quest-session-chip`, `path`, `heroes`, `hero`, `shell`, `notice`(`warn`/`error`) |
| Quests | `quest-browser`, `qhead`, `qtitle`, `gem`, `qtools`, `viewtoggle`, `source`, `age`, `scopeline`, `scopechip`, `filters`, `statefilters`, `statefilter`, `qlist`, `qitem`, `qbtn`(`selected`), `thumb framed`, `qmain`, `qline`, `num`, `qname`, `meta`, `effort` |
| Detail | `detail`, `back`, `titlecard`, `dtitle`, `dmeta`, `pill`, `rel`, `link`, `mark`, `md`, `comments`, `comment`, `who`, `launch-footer`, `hero-label`, `launch-button`, `launch-note`, `detail-empty`, `kicker` |
| Status | `chip`, `label` + `swatch`, `statetag`, `dot`, `badge`, `activity-chip`, `rations` |
| Generic | `actions`, `secondary`, `danger`, `ghost`, `hint`, `banner`, `empty` |

## Where the truth lives

`_ds/<folder>/styles.css` imports `_ds_bundle.css`, which is the app's real stylesheet. Read it before styling
anything new. Each card's `components/<Group>/<Name>/<Name>.prompt.md` gives that element's exact markup.

## Example

```jsx
<div className="app" data-theme="dungeon" data-font="almendra" style={{display: 'block', height: 'auto', padding: 16}}>
  <div className="detail" style={{background: 'var(--panel)'}}>
    <article>
      <div className="titlecard"><h2 className="dtitle"><span className="num">#3</span> Add the map legend</h2></div>
      <p className="dmeta"><span className="pill open">Open</span><span className="pill effort-chip">Medium</span></p>
    </article>
    <footer className="launch-footer">
      <label className="hero-label">Hero</label>
      <select><option>Opus</option></select>
      <button className="launch-button">Review launch</button>
    </footer>
  </div>
</div>
```

Vocabulary in UI text: campaign = project, quest = GitHub issue, hero = agent setup, expedition = running session.
