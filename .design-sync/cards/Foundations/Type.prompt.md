Typography: Cinzel for headings and labels, a reading serif for panels, Fira Code for paths and code.

- Headings, section labels, buttons in Dungeon: `font-family: 'Cinzel', var(--reading-font, serif)`, often uppercase with
  `letter-spacing: .06em`–`.13em`.
- Panel text: `font-family: var(--reading-font)`. `data-font` on `.app` picks it: `almendra` (default), `eb-garamond`,
  `alegreya`.
- Paths, repo names, code: `'Fira Code', ui-monospace, monospace` (classes `.path`, `.repo`, `.source` already use it).
- Icon glyphs: `'Dungeon Symbols'` (Nerd Font symbols).

```jsx
<h2 className="sidebar-heading">Campaigns<span className="section-rule" /></h2>
<h2 className="dtitle"><span className="num">#3</span> Quest title</h2>
```
