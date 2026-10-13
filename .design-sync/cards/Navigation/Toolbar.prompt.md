Top bar over the terminal: current campaign and session on the left, hero launch buttons on the right; notices below.

```jsx
<div className="main">
  <header className="toolbar">
    <div className="where">
      <strong>dungeon-master</strong><span className="repo">owner/repo</span>
      <span className="session-title"><span className="session-light" />#3 Add the map legend</span>
      <button className="quest-session-chip">#3</button>
      <code className="path">~/Developer/dungeon</code>
    </div>
    <div className="heroes"><button className="hero">Opus</button><button className="shell">Shell</button></div>
  </header>
  <p className="notice warn">…</p>
  <main className="stage"><div className="term-wrap"><div className="term">…terminal…</div></div></main>
</div>
```

`.notice` variants: plain, `warn`, `error`. The terminal (`.term`) is a parchment page framed in brass; fill it with
monospace text in dark ink.
