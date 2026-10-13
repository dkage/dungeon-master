/**
 * Which workspace region fills the application, if any. Layout only: sessions, the selected
 * issue and the map level live elsewhere and never depend on this. Buttons and, later,
 * shortcuts call the same semantic actions.
 */
export type Region = 'projects' | 'terminal' | 'map' | 'detail'

export const REGIONS: readonly Region[] = ['projects', 'terminal', 'map', 'detail']

export const REGION_NAMES: Record<Region, string> = {
  projects: 'projects',
  terminal: 'terminal',
  map: 'map and list',
  detail: 'issue detail',
}

export type Workspace = { expanded: Region | null }

export const SPLIT: Workspace = { expanded: null }

/** Expand a region; expanding another changes the target. */
export const expand = (_: Workspace, region: Region): Workspace => ({ expanded: region })

export const restore = (): Workspace => SPLIT

/** The control for a region: expands it, or restores when it is already the expanded one. */
export const toggle = (w: Workspace, region: Region): Workspace => (w.expanded === region ? restore() : expand(w, region))

export const expandLabel = (region: Region, expanded: boolean) =>
  expanded ? 'Restore split' : `Expand ${REGION_NAMES[region]}`

/** The right panel hosts two regions; it must show when either one is expanded. */
export const inRightPanel = (region: Region | null) => region === 'map' || region === 'detail'

/**
 * Escape restores an expanded region, unless something else owns the key: an open dialog, text
 * editing, or the terminal (where Escape is terminal input).
 */
export function escapeRestores(w: Workspace, target: EventTarget | null, dialogOpen: boolean): boolean {
  if (w.expanded === null || dialogOpen) return false
  const el = target as Element | null
  if (el?.closest?.('input, textarea, select, [contenteditable=""], [contenteditable="true"], .xterm')) return false
  return true
}
