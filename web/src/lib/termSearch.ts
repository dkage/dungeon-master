// Pure helpers for the terminal's scrollback search (#91).

/** What the match counter says: nothing before a search, "No matches", or "3 of 12". */
export function matchLabel(query: string, index: number, count: number): string {
  if (!query) return ''
  if (count === 0) return 'No matches'
  // xterm reports -1 when there are more matches than it counts; show the count alone then.
  return index < 0 ? `${count}+ matches` : `${index + 1} of ${count}`
}

/** True when the view shows older output than the newest line, so "Latest" is worth offering. */
export function scrolledUp(buffer: { viewportY: number; baseY: number }): boolean {
  return buffer.viewportY < buffer.baseY
}

// Highlight colours must be #RRGGBB for the search addon.
export const SEARCH_DECORATIONS = {
  matchBackground: '#6b5a1f',
  matchBorder: '#d6b24a',
  matchOverviewRuler: '#d6b24a',
  activeMatchBackground: '#c9831f',
  activeMatchBorder: '#ffd37a',
  activeMatchColorOverviewRuler: '#ffd37a',
} as const

/** The window event the shortcut model uses to reach the visible terminal. */
export const TERMINAL_ACTION = 'dungeon:terminal-action'
