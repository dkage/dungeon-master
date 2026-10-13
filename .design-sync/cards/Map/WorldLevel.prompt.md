World map: one node per epic ("dungeon", art by size tier) plus the Tavern for issues with no epic, with progress rings.

```jsx
<div className="map" data-level="world">
  <div className="maptools"><nav className="crumbs"><span className="crumb current">World</span></nav></div>
  <div className="viewport" style={{'--floor': 'url(<assets>/map/world-floor.jpg)'}}>
    <svg viewBox="0 0 488 460" width="488" height="460">
      <g className="node" transform="translate(24,24)">
        <rect className="plate" width="168" height="176" rx="14" />
        <image href="<assets>/map/dungeon-2-crypt.png" x="32" y="6" width="104" height="104" />
        <circle className="ring-bg" cx="146" cy="22" r="14" />
        <circle className="ring" cx="146" cy="22" r="14" strokeDasharray="35 88" transform="rotate(-90 146 22)" />
        <text className="title" x="84" y="128" textAnchor="middle">Onboarding flow</text>
        <text className="counts" x="84" y="146" textAnchor="middle">#1 · 2/5 cleared</text>
      </g>
    </svg>
  </div>
  <p className="maphint">Double-click or press Enter on a dungeon to open it. Escape goes back.</p>
</div>
```

Dungeon art by tier: `dungeon-1-burrow.png`, `dungeon-2-crypt.png`, `dungeon-3-tower.png`, `dungeon-4-fortress.png`,
`dungeon-5-citadel.png`; the Tavern is `tavern.png`. The ring's dash is `cleared/total × 88`. Add `done` to a node when
every quest is cleared, `selected` when chosen. Edges between nodes: `<g className="wedge"><path d="…" /><text>2</text></g>`.
