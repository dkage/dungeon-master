Quest detail: framed title card, metadata pills, relation lists, markdown body, comments and the launch bar.

```jsx
<div className="detail">
  <header className="qhead"><button className="back">← Quests</button></header>
  <article>
    <div className="titlecard"><h2 className="dtitle"><span className="num">#3</span> Add the map legend</h2></div>
    <p className="dmeta"><span className="pill open">Open</span><span className="pill effort-chip">Medium</span><span>updated 2h ago</span></p>
    <section className="rel"><h3>Blocked by</h3><ul><li><button className="link"><span className="mark done">✓</span>#1 Scaffold</button></li></ul></section>
    <section className="md"><p>Issue body…</p></section>
    <section className="comments"><h3>Comments (1)</h3><div className="comment"><p className="who"><strong>@user</strong> 1h ago</p><div className="md">…</div></div></section>
  </article>
  <footer className="launch-footer">
    <label className="hero-label">Hero</label>
    <select><option>Opus</option></select>
    <button className="launch-button">Review launch</button>
    <p className="launch-note">Assigned to @user.</p>
  </footer>
</div>
```

`.launch-footer` is a two-column grid: label, hero select, then note and the launch button. An empty selection uses
`<section className="detail-empty"><p className="kicker">Quest detail</p><h2>Select an issue</h2><p>…</p></section>`.
