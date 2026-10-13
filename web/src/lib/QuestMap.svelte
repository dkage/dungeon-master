<script lang="ts">
  import { tick, untrack } from 'svelte'
  import type { AppStore } from './store.svelte'
  import { ICONS, type IconName } from './map/icons'
  import { CREATURES, DUNGEON_ART, DUNGEON_NAMES, FLOORS, TAVERN_ART, effortWord } from './map/art'
  import { STATE_WORDS, type MapState, type Place, type QuestView } from './map/model'
  import { layoutDungeon, layoutTavern, layoutWorld, ROOM, type Box, type Layout, type StubBox, type WorldLayout } from './map/layout'

  let { store }: { store: AppStore } = $props()

  const model = $derived(store.mapModel)
  // A level that no longer exists (after a refresh or a scope change) falls back to the world.
  const level = $derived.by(() => {
    const l = store.mapLevel
    if (l.kind === 'dungeon' && !model.dungeons.some((d) => d.id === l.id)) return { kind: 'world' as const }
    return l
  })
  const place = $derived<Place | undefined>(
    level.kind === 'dungeon' ? model.dungeons.find((d) => d.id === level.id) : level.kind === 'tavern' ? model.tavern : undefined,
  )
  const world = $derived<WorldLayout | null>(level.kind === 'world' ? layoutWorld(model) : null)
  const rooms = $derived<Layout | null>(
    level.kind === 'dungeon' ? layoutDungeon(model, level.id) : level.kind === 'tavern' ? layoutTavern(model) : null,
  )
  const size = $derived(world ?? rooms ?? { width: 400, height: 300 })
  // Plain mode and the Neutral theme draw the plain graph: no art, plain words.
  const plain = $derived(store.appearance.wordMode === 'plain' || store.appearance.theme === 'neutral')
  const selected = $derived(store.questNumber)

  let viewport = $state<HTMLDivElement>()
  let viewWidth = $state(0)
  let zoom = $state(1)
  const far = $derived(zoom < 0.6)

  const levelKey = $derived(level.kind === 'dungeon' ? `d${level.id}` : level.kind)
  const measured = $derived(viewWidth > 0)
  // A number, so a rebuilt layout of the same width (every session refresh
  // rebuilds it) does not count as a change.
  const contentWidth = $derived(size.width)
  // Each level opens fitted to the panel's width; resizing the panel or a
  // background refresh keeps the zoom the user chose.
  $effect(() => {
    void levelKey
    void contentWidth
    if (measured) untrack(fit)
  })

  function fit() {
    if (!viewWidth) return
    zoom = clamp(Math.min(1.1, (viewWidth - 8) / size.width))
  }
  const clamp = (z: number) => Math.max(0.35, Math.min(2, Math.round(z * 100) / 100))
  const zoomBy = (f: number) => (zoom = clamp(zoom * f))

  function onWheel(event: WheelEvent) {
    if (!event.ctrlKey && !event.metaKey) return
    event.preventDefault()
    zoomBy(event.deltaY < 0 ? 1.1 : 1 / 1.1)
  }

  // Dragging the floor pans the view; clicks on rooms are left alone.
  let drag: { x: number; y: number; left: number; top: number } | null = null
  function onPointerDown(event: PointerEvent) {
    if (!viewport || (event.target as Element).closest('[data-hit]')) return
    drag = { x: event.clientX, y: event.clientY, left: viewport.scrollLeft, top: viewport.scrollTop }
  }
  function onPointerMove(event: PointerEvent) {
    if (!drag || !viewport) return
    viewport.scrollLeft = drag.left - (event.clientX - drag.x)
    viewport.scrollTop = drag.top - (event.clientY - drag.y)
  }
  const endDrag = () => (drag = null)

  function onMapKey(event: KeyboardEvent) {
    if (event.key === 'Escape' && level.kind !== 'world') {
      event.preventDefault()
      store.leavePlace()
      void focusFirst()
    }
  }

  async function focusFirst() {
    await tick()
    viewport?.querySelector<SVGElement>('[data-hit]')?.focus()
  }

  // Arrow keys move to the nearest room or node in that direction.
  function arrow(event: KeyboardEvent, from: Box) {
    const dir = { ArrowRight: [1, 0], ArrowLeft: [-1, 0], ArrowDown: [0, 1], ArrowUp: [0, -1] }[event.key]
    if (!dir || !viewport) return
    event.preventDefault()
    const cx = from.x + from.w / 2
    const cy = from.y + from.h / 2
    let best: Element | null = null
    let bestScore = Infinity
    for (const el of viewport.querySelectorAll('[data-hit]')) {
      const x = Number(el.getAttribute('data-cx'))
      const y = Number(el.getAttribute('data-cy'))
      const dx = x - cx
      const dy = y - cy
      const along = dx * dir[0] + dy * dir[1]
      if (along <= 1) continue
      const across = Math.abs(dx * dir[1]) + Math.abs(dy * dir[0])
      const score = along + across * 2
      if (score < bestScore) {
        bestScore = score
        best = el
      }
    }
    ;(best as SVGElement | null)?.focus()
  }

  function activate(event: KeyboardEvent, box: Box, run: () => void) {
    if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault()
      run()
    } else arrow(event, box)
  }

  async function enter(id: number | 'tavern') {
    store.enterPlace(id)
    await focusFirst()
  }

  const view = (n: number) => model.quests.get(n)!
  /** The larger side of an icon's viewBox, so every icon scales to the size asked for. */
  const iconSize = (viewBox: string) => {
    const [, , w, h] = viewBox.split(' ').map(Number)
    return Math.max(w, h)
  }
  /** Gate art is drawn at 1.0x of its 24x40 box: about 24 by 40 px on the map. */
  const GATE_SCALE = 1
  const short = (s: string, n: number) => (s.length > n ? `${s.slice(0, n - 1).trimEnd()}…` : s)

  const BADGE: Record<MapState, IconName> = {
    ready: 'ready', running: 'running', 'needs-you': 'needsYou', assigned: 'assigned', locked: 'locked', cleared: 'cleared',
  }

  function roomLabel(v: QuestView, boss: boolean): string {
    const parts = [`#${v.number} ${v.quest.title}`, STATE_WORDS[v.state], `${effortWord(v.effort)} effort`]
    if (v.openBlockers.length || v.outside.some((r) => !r.closed)) {
      const open = [...v.openBlockers, ...v.outside.filter((r) => !r.closed).map((r) => r.number)]
      parts.push(`blocked by ${open.map((n) => `#${n}`).join(', ')}`)
    }
    if (v.quest.assignees.length && !v.session) parts.push(`assigned to ${v.quest.assignees.map((a) => `@${a}`).join(', ')}`)
    if (v.priority === 'high' || v.priority === 'urgent') parts.push(`${v.priority} priority`)
    if (boss) parts.push('end of the longest chain')
    return parts.join(', ')
  }

  function placeLabel(p: Place): string {
    const name = p.id === 'tavern' ? (plain ? 'No epic' : 'Tavern') : `${plain ? 'Epic' : 'Dungeon'} #${p.id} ${p.title}`
    const done = p.total > 0 && p.cleared === p.total
    const counts = done ? 'all cleared' : `${p.cleared} of ${p.total} cleared, ${p.ready} ready, ${p.running} in progress, ${p.blocked} blocked`
    return `${name}, ${counts}${p.needsYou ? ', a session is waiting for you' : ''}. Press Enter to open.`
  }

  function stubLabel(s: StubBox): string {
    const where = s.where === 'outside' ? 'outside this scope' : s.where === 'tavern' ? 'in the Tavern' : `in ${model.dungeons.find((d) => d.id === s.where)?.title ?? 'another epic'}`
    return `#${s.n} ${s.title}, ${s.closed ? 'closed' : 'open'}, ${where}`
  }

  // Links that touch the selection light up; the rooms at their other end too.
  const near = $derived.by(() => {
    const out = new Set<number>()
    if (selected === null || !rooms) return out
    for (const l of rooms.links) {
      if (l.from === selected) out.add(l.to)
      if (l.to === selected) out.add(l.from)
    }
    return out
  })
  const crumbPlace = $derived(place && place.id !== 'tavern' ? place : undefined)
</script>

{#snippet gateIcon(open: boolean, x: number, y: number)}
  <!-- The gate is 24x40 in its own box; centre the doorway (about y=26) on the line so the line runs through it. -->
  <circle cx={x} cy={y} r="14" class="gate-backing" />
  {@render icon(open ? 'gateOpen' : 'gateClosed', x - 0.5 * 24 * GATE_SCALE, y - 26 * GATE_SCALE, 40 * GATE_SCALE)}
{/snippet}

{#snippet icon(name: IconName, x: number, y: number, s: number)}
  {@const ic = ICONS[name]}
  <g transform={`translate(${x},${y}) scale(${s / iconSize(ic.viewBox)})`} class="icon">
    {#each ic.parts as part, i (i)}
      {#if part.stroke}
        <path d={part.d} fill="none" stroke="currentColor" stroke-width={part.stroke} stroke-linecap="round" stroke-linejoin="round" />
      {:else}
        <path d={part.d} fill="currentColor" />
      {/if}
    {/each}
  </g>
{/snippet}

{#snippet badge(state: MapState, x: number, y: number)}
  <g class="badge {state}">
    <circle cx={x} cy={y} r="11" />
    {@render icon(BADGE[state], x - 7, y - 7, 14)}
  </g>
{/snippet}

<div class="map" class:plain class:far data-level={level.kind}>
  <div class="maptools">
    <nav class="crumbs" aria-label="Map level">
      {#if level.kind === 'world'}
        <span class="crumb current" aria-current="location">{plain ? 'All epics' : 'World'}</span>
      {:else}
        <button type="button" class="crumb" onclick={() => store.leavePlace()}>{plain ? 'All epics' : 'World'}</button>
        <span class="sep" aria-hidden="true">›</span>
        {#if crumbPlace}
          <button type="button" class="crumb current" aria-current="location" title="Show the epic itself" onclick={() => store.selectQuest(crumbPlace.id as number)}>
            {crumbPlace.title}
          </button>
        {:else}
          <span class="crumb current" aria-current="location">{plain ? 'No epic' : 'Tavern'}</span>
        {/if}
        {#if place}<span class="progress">· {place.cleared} of {place.total} cleared</span>{/if}
      {/if}
    </nav>
    <div class="zoom" role="group" aria-label="Zoom">
      <button type="button" aria-label="Zoom out" onclick={() => zoomBy(1 / 1.25)}>−</button>
      <button type="button" aria-label="Zoom in" onclick={() => zoomBy(1.25)}>+</button>
      <button type="button" aria-label="Fit the map to the panel" onclick={fit}>Fit</button>
    </div>
  </div>

  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div
    class="viewport"
    bind:this={viewport}
    bind:clientWidth={viewWidth}
    style={plain ? '' : `--floor:url(${level.kind === 'world' ? FLOORS.world : level.kind === 'tavern' ? FLOORS.tavern : FLOORS.dungeon})`}
    onwheel={onWheel}
    onpointerdown={onPointerDown}
    onpointermove={onPointerMove}
    onpointerup={endDrag}
    onpointerleave={endDrag}
    onkeydown={onMapKey}
  >
    {#if model.quests.size === 0}
      <p class="empty">No quests to map yet.</p>
    {:else}
      <svg
        role="group"
        aria-label={level.kind === 'world' ? 'World map' : `Map of ${place?.title ?? ''}`}
        viewBox={`0 0 ${size.width} ${size.height}`}
        width={Math.round(size.width * zoom)}
        height={Math.round(size.height * zoom)}
        class:has-selection={selected !== null}
      >
        <defs>
          <radialGradient id="light-ready"><stop offset="0" stop-color="#7fb8ff" stop-opacity=".45" /><stop offset="1" stop-color="#7fb8ff" stop-opacity="0" /></radialGradient>
          <radialGradient id="light-running"><stop offset="0" stop-color="#ffab3d" stop-opacity=".6" /><stop offset="1" stop-color="#ffab3d" stop-opacity="0" /></radialGradient>
          <radialGradient id="light-needs-you"><stop offset="0" stop-color="#ffd75e" stop-opacity=".6" /><stop offset="1" stop-color="#ffd75e" stop-opacity="0" /></radialGradient>
          <radialGradient id="light-assigned"><stop offset="0" stop-color="#c9b48f" stop-opacity=".25" /><stop offset="1" stop-color="#c9b48f" stop-opacity="0" /></radialGradient>
        </defs>

        {#if world}
          {#each world.edges as e (`${e.from}>${e.to}`)}
            <g class="wedge" class:open={e.open}>
              <path d={e.d} />
              <text x={e.label.x} y={e.label.y} text-anchor="middle">{e.count}</text>
              <g class="gate">{@render gateIcon(e.open, e.gate.x, e.gate.y)}</g>
            </g>
          {/each}
          {#each world.nodes as node (node.id)}
            {@const p = node.id === 'tavern' ? model.tavern : model.dungeons.find((d) => d.id === node.id)!}
            {@const done = p.total > 0 && p.cleared === p.total}
            {@const frac = p.total ? p.cleared / p.total : 0}
            <g
              class="node"
              class:tavern={node.id === 'tavern'}
              class:done
              class:selected={node.id === selected}
              data-hit
              data-cx={node.x + node.w / 2}
              data-cy={node.y + node.h / 2}
              role="button"
              tabindex="0"
              aria-label={placeLabel(p)}
              aria-pressed={node.id === selected}
              transform={`translate(${node.x},${node.y})`}
              onclick={() => node.id !== 'tavern' && store.selectQuest(node.id)}
              ondblclick={() => enter(node.id)}
              onkeydown={(ev) => activate(ev, node, () => enter(node.id))}
            >
              <title>{p.id === 'tavern' ? 'Tavern: quests with no epic' : `#${p.id} ${p.title}`}</title>
              <rect class="plate" width={node.w} height={node.h} rx="14" />
              {#if !plain}
                <image
                  href={node.id === 'tavern' ? TAVERN_ART : DUNGEON_ART[p.tier - 1]}
                  x={(node.w - 104) / 2}
                  y="6"
                  width="104"
                  height="104"
                  aria-hidden="true"
                >
                  <title>{node.id === 'tavern' ? 'Tavern' : DUNGEON_NAMES[p.tier - 1]}</title>
                </image>
              {:else}
                <rect class="shape" x={(node.w - 64) / 2} y="22" width="64" height="64" rx={node.id === 'tavern' ? 32 : 8} />
              {/if}
              <circle class="ring-bg" cx={node.w - 22} cy="22" r="14" />
              <circle class="ring" cx={node.w - 22} cy="22" r="14" stroke-dasharray={`${frac * 88} 88`} transform={`rotate(-90 ${node.w - 22} 22)`} />
              {#if done}
                <g class="badge cleared">{@render icon('cleared', node.w - 29, 15, 14)}</g>
              {:else if p.needsYou}
                {@render badge('needs-you', node.w - 22, 22)}
              {/if}
              <text class="title" x={node.w / 2} y="128" text-anchor="middle">
                {node.id === 'tavern' ? (plain ? 'No epic' : 'Tavern') : short(p.title, 22)}
              </text>
              <text class="counts" x={node.w / 2} y="146" text-anchor="middle">
                {#if node.id !== 'tavern'}#{p.id} · {/if}{p.cleared}/{p.total} cleared
              </text>
              <text class="counts" x={node.w / 2} y="162" text-anchor="middle">
                {p.ready} ready · {p.running} active · {p.blocked} blocked
              </text>
            </g>
          {/each}
        {/if}

        {#if rooms}
          {#each rooms.halls as h (h.n)}
            {@const hv = view(h.n)}
            <g
              class="hall"
              class:selected={h.n === selected}
              data-hit
              data-cx={h.x + 40}
              data-cy={h.y + 14}
              role="button"
              tabindex="0"
              aria-label={`Group #${h.n} ${hv.quest.title}, ${hv.quest.subDone} of ${hv.quest.subTotal} done, ${effortWord(hv.effort)} effort`}
              aria-pressed={h.n === selected}
              onclick={() => store.selectQuest(h.n)}
              onkeydown={(ev) => activate(ev, { x: h.x, y: h.y, w: 80, h: 28 }, () => store.selectQuest(h.n))}
            >
              <rect x={h.x} y={h.y} width={h.w} height={h.h} rx="16" />
              <text x={h.x + 14} y={h.y + 20}>#{h.n} {short(hv.quest.title, 30)} · {hv.quest.subDone}/{hv.quest.subTotal} · {effortWord(hv.effort)}</text>
            </g>
          {/each}

          {#if rooms.outskirtsY !== null}
            <g class="outskirts" aria-hidden="true">
              <line x1="16" x2={rooms.width - 16} y1={rooms.outskirtsY + 8} y2={rooms.outskirtsY + 8} />
              <text x="20" y={rooms.outskirtsY + 26}>{plain ? 'Other epics and issues outside this scope' : 'Outskirts · external quests'}</text>
            </g>
          {/if}

          {#each rooms.links as l (`${l.kind}${l.from}>${l.to}`)}
            <g class="link {l.kind}" class:open={l.open} class:cycle={l.cycle} class:lit={selected !== null && (l.from === selected || l.to === selected)}>
              <path d={l.d} />
              <g class="gate">{@render gateIcon(l.open, l.gate.x, l.gate.y)}</g>
            </g>
          {/each}

          {#each rooms.stubs as s (s.n)}
            <g
              class="stub"
              class:closed={s.closed}
              class:selected={s.n === selected}
              data-hit
              data-cx={s.x + s.w / 2}
              data-cy={s.y + s.h / 2}
              role="button"
              tabindex="0"
              aria-label={stubLabel(s)}
              aria-pressed={s.n === selected}
              transform={`translate(${s.x},${s.y})`}
              onclick={() => store.selectQuest(s.n)}
              ondblclick={() => s.where !== 'outside' && store.jumpToQuest(s.n)}
              onkeydown={(ev) => activate(ev, s, () => store.selectQuest(s.n))}
            >
              <title>{stubLabel(s)}</title>
              <rect width={s.w} height={s.h} rx="8" />
              {@render icon('stubDiamond', 8, 10, 24)}
              <text class="num" x="40" y="19">#{s.n}</text>
              <text class="stitle" x="40" y="34">{short(s.title, 15)}</text>
            </g>
          {/each}

          {#each rooms.rooms as r (r.n)}
            {@const v = view(r.n)}
            {@const art = !plain && !far && v.creature ? CREATURES[v.creature] : undefined}
            <g
              class="room {v.state}"
              class:selected={r.n === selected}
              class:near={near.has(r.n)}
              class:boss={r.boss}
              data-hit
              data-room={r.n}
              data-cx={r.x + r.w / 2}
              data-cy={r.y + r.h / 2}
              role="button"
              tabindex="0"
              aria-label={roomLabel(v, r.boss)}
              aria-pressed={r.n === selected}
              transform={`translate(${r.x},${r.y})`}
              onclick={() => store.selectQuest(r.n)}
              ondblclick={() => store.selectQuest(r.n)}
              onkeydown={(ev) => activate(ev, r, () => store.selectQuest(r.n))}
            >
              <title>#{v.number} {v.quest.title} · {STATE_WORDS[v.state]}</title>
              {#if r.boss}
                <g class="boss-frame">{@render icon('bossFrame', -10, -10, ROOM.h + 20)}</g>
              {/if}
              <rect class="floor" width={r.w} height={r.h} rx="10" />
              {#if !plain && (v.state === 'ready' || v.state === 'running' || v.state === 'needs-you' || v.state === 'assigned')}
                <ellipse class="light" cx={r.w / 2} cy="48" rx="52" ry="46" fill={`url(#light-${v.state})`} />
              {/if}
              {#if v.state === 'needs-you'}<rect class="pulse" x="-3" y="-3" width={r.w + 6} height={r.h + 6} rx="12" />{/if}
              {#if far}
                <text class="big" x={r.w / 2} y="62" text-anchor="middle">#{v.number}</text>
              {:else if art}
                <svg class="creature" x={(r.w - 60) / 2} y="6" width="60" height="80" viewBox="0 0 60 80">
                  <image class="strip" href={art.strip} width="240" height="80" preserveAspectRatio="none">
                    <title>{art.name}</title>
                  </image>
                </svg>
              {:else if !plain}
                <g class="unknown">{@render icon('stubDiamond', r.w / 2 - 20, 22, 40)}</g>
              {:else}
                <circle class="plain-dot" cx={r.w / 2} cy="44" r="20" />
              {/if}
              {#if !far}
                <text class="num" x={r.w / 2} y="100" text-anchor="middle">#{v.number} · {effortWord(v.effort)}</text>
                <text class="rtitle" x={r.w / 2} y="115" text-anchor="middle">{short(v.quest.title, 17)}</text>
              {/if}
              {#if v.priority === 'high' || v.priority === 'urgent'}
                <g class="gem {v.priority}">{@render icon('priorityGem', 6, 6, 14)}</g>
              {/if}
              {@render badge(v.state, r.w - 4, 4)}
            </g>
          {/each}
          {#if rooms.rooms.length === 0}
            <text class="none" x="28" y="48">{level.kind === 'tavern' ? 'Every quest here belongs to an epic.' : 'This epic has no quests in scope.'}</text>
          {/if}
        {/if}
      </svg>
    {/if}
  </div>
  {#if level.kind === 'world' && model.quests.size > 0}
    <p class="maphint">Double-click or press Enter on a {plain ? 'node' : 'dungeon'} to open it. Escape goes back.</p>
  {/if}
</div>

<style>
  .map {
    --ready: #8fc3ff; --running: #ffae45; --needs: #ffd75e; --assigned: #c6b28c; --locked: #8b7a70; --cleared: #6cc785;
    --room: #17110f; --room-line: #5a4436; --ink: #eadfcb; --ink-muted: #b4a48f; --link: #a08770; --stub: #9c8f84;
    display: flex; flex-direction: column; min-height: 0; flex: 1 1 auto;
  }
  .map.plain {
    --ready: #2f6fb3; --running: #b35f00; --needs: #9a6a00; --assigned: #7b6a4d; --locked: #6d6d68; --cleared: #2e8b4b;
    --room: var(--panel); --room-line: var(--line); --ink: var(--text); --ink-muted: var(--muted); --link: var(--muted); --stub: var(--muted);
  }
  .maptools { display: flex; align-items: center; justify-content: space-between; gap: 8px; padding: 6px 10px; border-bottom: 1px solid var(--line); }
  .crumbs { display: flex; align-items: baseline; gap: 6px; min-width: 0; font-size: 12px; overflow: hidden; white-space: nowrap; }
  .crumb { padding: 2px 4px; border: 0; background: none; color: var(--muted); font: inherit; cursor: pointer; overflow: hidden; text-overflow: ellipsis; }
  .crumb.current { color: var(--text); font-weight: 600; }
  span.crumb { cursor: default; }
  .sep, .progress { color: var(--muted); }
  .zoom { display: flex; gap: 4px; flex: 0 0 auto; }
  .zoom button { min-width: 26px; padding: 2px 6px; border: 1px solid var(--line); border-radius: 6px; background: transparent; color: var(--text); font: inherit; font-size: 12px; cursor: pointer; }
  .viewport {
    flex: 1 1 auto; min-height: 120px; overflow: auto; position: relative;
    background: linear-gradient(#0008, #000a), var(--floor, none) 0 0 / 256px repeat, #0d0a09;
    cursor: grab; touch-action: none;
  }
  .plain .viewport { background: var(--bg); }
  .viewport:active { cursor: grabbing; }
  .viewport svg { display: block; }
  .empty, .maphint { margin: 0; padding: 12px; color: var(--muted); font-size: 12px; }
  .maphint { padding: 4px 10px; border-top: 1px solid var(--line); font-size: 11px; }

  [data-hit] { cursor: pointer; outline: none; }
  [data-hit]:focus-visible .floor, [data-hit]:focus-visible .plate, [data-hit]:focus-visible > rect { stroke: var(--needs); stroke-width: 3; }
  text { fill: var(--ink); font-family: var(--reading-font, Georgia, serif); }
  .icon { pointer-events: none; }

  /* World */
  .node .plate { fill: color-mix(in srgb, var(--room) 85%, transparent); stroke: var(--room-line); stroke-width: 1.5; }
  .node.selected .plate { stroke: var(--needs); stroke-width: 3; }
  .node .shape { fill: none; stroke: var(--ink-muted); stroke-width: 2; }
  .node .title { font-size: 13px; font-weight: 700; }
  .node .counts { font-size: 10px; fill: var(--ink-muted); }
  .node.done image, .node.done .shape { opacity: .45; filter: grayscale(1); }
  .ring-bg { fill: color-mix(in srgb, var(--room) 90%, transparent); stroke: var(--room-line); stroke-width: 3; }
  .ring { fill: none; stroke: var(--cleared); stroke-width: 3; }
  .wedge path { fill: none; stroke: var(--link); stroke-width: 2.5; }
  .wedge.open path { stroke-dasharray: 2 6; opacity: .6; }
  .wedge text { font-size: 12px; font-weight: 700; fill: var(--ink); paint-order: stroke; stroke: #0009; stroke-width: 3; }
  .gate { color: var(--locked); }
  .gate-backing { fill: var(--bg, #14110d); opacity: 0.85; stroke: none; }
  .wedge.open .gate, .link.open .gate { color: var(--cleared); }

  /* Dungeon and Tavern */
  .hall rect { fill: color-mix(in srgb, var(--room) 40%, transparent); stroke: var(--room-line); stroke-width: 1.5; stroke-dasharray: 6 4; }
  .hall text { font-size: 11px; font-weight: 700; fill: var(--ink-muted); }
  .hall.selected rect { stroke: var(--needs); stroke-dasharray: none; }
  .outskirts line { stroke: var(--room-line); stroke-dasharray: 4 4; }
  .outskirts text { font-size: 11px; fill: var(--ink-muted); letter-spacing: .04em; }
  .link path { fill: none; stroke: var(--link); stroke-width: 2; opacity: .85; }
  .link.stub path { stroke-dasharray: 5 5; opacity: .6; }
  .link.open path { opacity: .35; }
  .link.cycle path { stroke: var(--needs); }
  .link.lit path { stroke: var(--needs); opacity: 1; stroke-width: 3; }
  .stub rect { fill: color-mix(in srgb, var(--room) 70%, transparent); stroke: var(--stub); stroke-dasharray: 4 3; }
  .stub .icon { color: var(--stub); }
  .stub .num { font-size: 12px; font-weight: 700; }
  .stub .stitle { font-size: 10px; fill: var(--ink-muted); }
  .stub.closed text { text-decoration: line-through; opacity: .7; }
  .stub.selected rect { stroke: var(--needs); stroke-width: 2.5; stroke-dasharray: none; }

  .room .floor { fill: var(--room); stroke: var(--room-line); stroke-width: 1.5; }
  .room .num { font-size: 11px; font-weight: 700; }
  .room .rtitle { font-size: 10px; fill: var(--ink-muted); }
  .room .big { font-size: 26px; font-weight: 700; }
  .room.ready .floor { stroke: var(--ready); }
  .room.running .floor { stroke: var(--running); stroke-width: 2.5; }
  .room.needs-you .floor { stroke: var(--needs); stroke-width: 2.5; }
  .room.assigned .floor { stroke: var(--assigned); }
  .room.locked .floor { fill: color-mix(in srgb, var(--room) 70%, #000); }
  .room.locked .creature, .room.locked .unknown { opacity: .7; filter: brightness(.75) saturate(.7); }
  .room.cleared .creature, .room.cleared .unknown, .room.cleared .plain-dot { opacity: .35; filter: grayscale(1); }
  .room.cleared text { opacity: .6; }
  .room.selected .floor { stroke: var(--needs); stroke-width: 3.5; }
  .has-selection .room:not(.selected):not(.near) { opacity: .78; }
  .unknown { color: var(--stub); }
  .plain-dot { fill: none; stroke: var(--ink-muted); stroke-width: 2; }
  .pulse { fill: none; stroke: var(--needs); stroke-width: 2; animation: pulse 1.6s ease-in-out infinite; }
  .room.running .light { animation: flicker 2.3s infinite; }
  .boss-frame { color: var(--running); opacity: .9; }
  .gem.high { color: #f2a623; }
  .gem.urgent { color: #e5463b; }

  .badge circle { fill: #120d0b; stroke: currentColor; stroke-width: 1.5; }
  .plain .badge circle { fill: var(--panel); }
  .badge.ready { color: var(--ready); }
  .badge.running { color: var(--running); }
  .badge.needs-you { color: var(--needs); }
  .badge.assigned { color: var(--assigned); }
  .badge.locked { color: var(--locked); }
  .badge.cleared { color: var(--cleared); }

  .strip { animation: idle 1.2s steps(4) infinite; }
  @keyframes idle { to { transform: translateX(-240px); } }
  @keyframes flicker { 0%, 100% { opacity: 1; } 18% { opacity: .82; } 22% { opacity: .95; } 51% { opacity: .78; } 70% { opacity: .92; } }
  @keyframes pulse { 0%, 100% { opacity: .35; } 50% { opacity: 1; } }
  @media (prefers-reduced-motion: reduce) {
    .strip, .room.running .light, .pulse { animation: none; }
  }
</style>
