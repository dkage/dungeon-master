Quest detail: framed title card, metadata pills, effort, relation lists, markdown body, comments and the launch bar.

```jsx
<div className="detail">
  <header className="qhead"><button className="back">← Quests</button></header>
  <article>
    <div className="titlecard"><h2 className="dtitle"><span className="num">#3</span> Add the map legend</h2></div>
    <p className="dmeta"><span className="pill open">Open</span><span className="pill effort-chip">Medium</span><span>updated 2h ago</span></p>
    <section className="rel effort-row"><h3>Effort</h3><p><strong>Medium</strong></p></section>
    <section className="rel"><h3>Blocked by</h3><ul><li><button className="link"><span className="mark done">✓</span>#1 Scaffold</button></li></ul></section>
    <section className="md"><p>Issue body…</p></section>
    <section className="comments"><h3>Comments (1)</h3><div className="comment"><p className="who"><strong>@user</strong> 1h ago</p><div className="md">…</div></div></section>
  </article>
  <footer className="launch-footer">
    <label className="dir-label">Additional direction <span className="opt">Optional</span></label>
    <textarea className="dir" rows={2} placeholder="Add context for the agent…" />
    <label className="hero-label">Hero</label>
    <select className="hero-select"><option>Opus</option></select>
    <label className="skill-label">Method</label>
    <select className="skill-select"><option>Automatic</option></select>
    <span className="settings-links"><button className="link">Heroes…</button><button className="link">Spell sources…</button></span>
    <button className="launch-button">Review launch</button>
    <p className="launch-note">Assigned to @user.</p>
  </footer>
</div>
```

`.launch-footer` is a named-area grid: direction across the top, hero and method side by side, Settings links and the
launch button, then the note. "Review launch" opens the launch review dialog (see Quests/LaunchReview). An empty
selection uses `<section className="detail-empty"><p className="kicker">Quest detail</p><h2>Select an issue</h2><p>…</p></section>`.
