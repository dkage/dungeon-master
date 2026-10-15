Left panel: brand header, search, campaign cards with their sessions, footer buttons.

```jsx
<aside className="sidebar">
  <header className="brand-header"><span className="brand-mark">✦</span><strong>Dungeon Master</strong></header>
  <label className="sidebar-search"><span>⌕</span><input type="search" placeholder="Jump to project or session…" /></label>
  <nav>
    <h2 className="sidebar-heading">Campaigns<span className="section-rule" /><button className="heading-add">＋</button></h2>
    <ul className="campaigns">
      <li className="campaign selected">
        <div className="row">
          <button className="pick"><span className="cline"><span className="cname">dungeon-master</span></span><span className="repo">owner/repo</span></button>
          <button className="ghost">⋯</button>
        </div>
        <ul className="sessions">
          <li className="session active">
            <button className="pick"><span className="dot running" /><span className="stitle">#3 Add the map legend</span></button>
            <button className="ghost">×</button>
          </li>
        </ul>
      </li>
    </ul>
  </nav>
  <div className="sidebar-footer"><button className="settings-button">⚙ Settings</button></div>
</aside>
```

A session in its own working copy adds `<span className="badge branch">⎇ 3-map-legend</span>` after `.stitle`.
Selected campaign: `campaign selected`; active session: `session active`. A sample project gets
`<span className="campaign-badge">Sample</span>` inside `.cline`.
