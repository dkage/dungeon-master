Launch review dialog: what will run, where, with which method, and the exact briefing, before a session starts.

```jsx
<dialog open className="brief-dialog">
  <header>
    <div><p className="eyebrow">#3 · Opus</p><h2>Review launch plan</h2></div>
    <button className="close">×</button>
  </header>
  <p className="brief-summary">Prepared from 1 of 1 comments.</p>
  <dl className="plan-meta"><dt>Working directory</dt><dd>…</dd><dt>Method</dt><dd>tdd · source @ rev</dd></dl>
  <fieldset className="where">
    <legend>Where the session runs</legend>
    <label><input type="radio" name="where" defaultChecked /><span>Its own working copy</span></label>
    <label><input type="radio" name="where" /><span>The project's main working tree</span></label>
    <p className="where-note">…</p>
    <h3 className="brief-label">Bring from the main working tree</h3>
    <ul className="carry"><li><code>.env</code><span className="hint">ignored by Git</span><select>…</select></li></ul>
  </fieldset>
  <details className="command"><summary>Command and agent instruction</summary><pre>…</pre></details>
  <h3 className="brief-label">Task briefing sent to the agent</h3>
  <pre className="brief-content">…</pre>
  <footer className="dialog-actions"><button className="cancel">Cancel</button><button className="launch-button">Start expedition</button></footer>
</dialog>
```

`.plan-meta` is a two-column `dt`/`dd` grid. The dialog lives inside `.detail`; its header and `.dialog-actions` stick to
the top and bottom while the middle scrolls.
