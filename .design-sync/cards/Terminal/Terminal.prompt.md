Terminal stage: the agent's PTY on a parchment page, its tool buttons and search, the status line, the skill chips
under it and the backend-down strip.

```jsx
<div className="main">
  <main className="stage">
    <div className="pane"><div className="term-wrap">
      <div className="term" data-term-theme="dungeon"><div className="term-host">…xterm…</div></div>
      <div className="term-tools"><button className="term-tool">A−</button><button className="term-tool">A+</button><button className="term-tool">Copy</button><button className="term-tool" aria-pressed="true">Search</button></div>
      <div className="term-search"><input type="search" placeholder="Search output" /><span className="term-count">1 of 2</span><button className="term-tool">↓</button></div>
      <div className="term-status" data-status="reconnecting">Reconnecting…</div>
    </div></div>
  </main>
  <div className="chips"><button className="chip on" aria-pressed="true">tdd</button><button className="chip">code-review</button></div>
  <footer className="backend-down">Cannot reach the app. What you see may be out of date.</footer>
</div>
```

`.term[data-term-theme]` is `dungeon`, `neutral`, `light` or `high-contrast`; the last two replace the parchment with a
flat page. `.term-note` (with `data-kind="info"` for non-errors) is a floating message above the status line. A chip
only chooses the open issue's method (`.chip.on`); it never starts a session.
