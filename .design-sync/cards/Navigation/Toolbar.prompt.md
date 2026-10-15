Top bar over the terminal: current campaign and session on the left, hero launch buttons on the right; notices below.

```jsx
<div className="main">
  <header className="toolbar">
    <div className="where">
      <strong>dungeon-master</strong><span className="repo">owner/repo</span>
      <span className="session-title"><span className="session-light" />#3 Add the map legend</span>
      <button className="quest-session-chip">#3</button><span className="branch-chip">⎇ dungeon/3-map-legend</span>
      <code className="path">~/Developer/dungeon</code>
    </div>
    <div className="heroes"><button className="hero">Opus</button><button className="shell">Shell</button></div>
  </header>
  <div className="tabs" role="tablist">
    <div className="tab active"><button role="tab" className="label"><span className="dot running" /><span className="text">#3 Add the map legend (Opus)</span><span className="branch">⎇ 3-map-legend</span></button><button className="x">×</button></div>
    <div className="tab needs"><button role="tab" className="label"><span className="dot needs">!</span><span className="text">#7 Fix scope picker</span></button><button className="x">×</button></div>
  </div>
  <div className="stop-confirm"><p>… is still working. Closing it stops the agent …</p><button className="danger">Stop and close</button><button className="secondary">Keep running</button></div>
  <p className="notice warn">…</p>
  <main className="stage"><div className="term-wrap"><div className="term">…terminal…</div></div></main>
</div>
```

`.notice` variants: plain, `warn`, `error`. Session tabs sit under the toolbar (`.tab.active`, `.tab.needs`; dots
`running`, `idle`, `failed`, `lost`, `needs`); `.stop-confirm` appears there when closing a working session. The terminal (`.term`) is a parchment page framed in brass; fill it with
monospace text in dark ink.
