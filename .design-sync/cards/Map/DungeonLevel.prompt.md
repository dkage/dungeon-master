Quest map inside one epic ("dungeon"): each issue is a room with its creature, state light and badge; lines are blockers.

Captured from the real `QuestMap` component. The map is an SVG the app lays out itself, so for a design reuse the
structure and classes rather than inventing a new drawing:

```jsx
<div className="map" data-level="dungeon">
  <div className="maptools">
    <nav className="crumbs"><button className="crumb">World</button><span className="sep">›</span>
      <span className="crumb current">Onboarding flow</span><span className="progress">· 1 of 5 cleared</span></nav>
    <div className="zoom"><button>−</button><button>+</button><button>Fit</button></div>
  </div>
  <div className="viewport" style={{'--floor': 'url(<assets>/map/stone-floor.jpg)'}}>
    <svg viewBox="0 0 520 480" width="520" height="480">
      {/* one room: 112 × 124, translated into place */}
      <g className="room running" transform="translate(28,178)">
        <rect className="floor" rx="10" width="112" height="124" />
        <svg className="creature" x="26" y="6" width="60" height="80" viewBox="0 0 60 80">
          <image className="strip" width="240" height="80" preserveAspectRatio="none" href="<assets>/map/creatures/ghoul-idle-strip.png" />
        </svg>
        <text className="num" x="56" y="100" textAnchor="middle">#5 · High</text>
        <text className="rtitle" x="56" y="115" textAnchor="middle">Password reset</text>
      </g>
    </svg>
  </div>
</div>
```

- Room states (class after `room`): `ready`, `running`, `needs-you`, `assigned`, `locked`, `cleared`; add `selected`,
  and `boss` for the end of the longest chain. The state colours are `.map` variables: `--ready`, `--running`,
  `--needs`, `--assigned`, `--locked`, `--cleared`.
- Blocker lines: `<g className="link block"><path d="…" /></g>` (`open` when the blocker is closed, `lit` when it
  touches the selected room).
- Creature strips are 4 frames side by side (240 × 80); the `.strip` class animates them.
- `<assets>` is the `assets/` folder next to this design system's `styles.css`.
