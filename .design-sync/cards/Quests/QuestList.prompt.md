Quest browser: framed header with gem, scope chip, filters and a list of quest rows with state tags.

```jsx
<aside className="quests">
  <section className="quest-browser">
    <header className="qhead">
      <div className="qtitle"><span className="gem" /><h2>Quests</h2><span className="source">owner/repo</span></div>
      <div className="qtools"><div className="viewtoggle"><button aria-pressed="false">Map</button><button aria-pressed="true">List</button></div></div>
    </header>
    <ul className="qlist">
      <li className="qitem open ready">
        <button className="qbtn selected">
          <span className="thumb framed unknown">◇</span>
          <span className="qmain">
            <span className="qline"><span className="num">#3</span><span className="qname">Add the map legend</span></span>
            <span className="meta"><span className="effort">Medium</span><span className="chip">1/3</span></span>
          </span>
          <span className="statetag ready">Ready</span>
        </button>
      </li>
    </ul>
  </section>
</aside>
```

The row state goes on both `.qitem` (`cleared` strikes the title, `locked` mutes it) and the `.statetag`. Nest rows
with `style={{'--depth': 1}}` on `.qitem`. The selected row is `qbtn selected`. In the app `.quests` is a two-row grid
(browser, `.splitter`, detail); use `style={{display:'block'}}` when showing the list alone.

Below 1180px of viewport width `.quests` becomes an off-screen drawer (`position: fixed`, shifted out). Always render
it as `<aside className="quests open" style={{position:'static', transform:'none', boxShadow:'none'}}>` unless the
design is a full window wider than 1180px.
