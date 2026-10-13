/**
 * Vector marks for the map, from design/map-art/ (original art for this project).
 * Each is drawn inside the map SVG with the theme colour, so it needs no image
 * request and follows light and dark themes.
 */

export interface IconPart {
  d: string
  /** Stroke width; absent means a filled shape. */
  stroke?: number
}

export interface Icon {
  viewBox: string
  parts: IconPart[]
}

const DATA = {
  ready: { viewBox: '0 0 24 24', parts: [{ d: 'M10 2h4l-1 13h-2L10 2Zm2 16 3 3-3 3-3-3 3-3Z' }] },
  needsYou: { viewBox: '0 0 24 24', parts: [{ d: 'M7 7c0-3.2 2.2-5 5.4-5 3.5 0 5.6 2 5.6 5 0 2.2-1 3.4-3 5l-2.3 1.8-.3 2.2h-3l-.2-3.2 3.2-2.8c1.2-1 2-1.7 2-3 0-1.5-.9-2.4-2.2-2.4-1.5 0-2.3.9-2.4 2.4H7Zm5 11 3 3-3 3-3-3 3-3Z' }] },
  assigned: { viewBox: '0 0 24 24', parts: [{ d: 'M16.5 16.5v-7a4.5 4.5 0 1 0-1.2 3.1c.2 2.1 1.4 3.4 3.2 3.4 2.3 0 3.5-2 3.5-5.2 0-5-3.8-8.3-9-8.3A9.5 9.5 0 0 0 3.5 12a9.5 9.5 0 0 0 9.6 9.5c1.5 0 2.8-.3 4-.8', stroke: 2.2 }] },
  locked: { viewBox: '0 0 24 24', parts: [{ d: 'M7 10V7a5 5 0 0 1 10 0v3M5 10h14v11H5z', stroke: 2.2 }, { d: 'M12 14v3m0-5.5a1.5 1.5 0 1 0 0 3 1.5 1.5 0 0 0 0-3Z' }] },
  closed: { viewBox: '0 0 24 24', parts: [{ d: 'm5 3 7 7 7-7 2 2-7 7 7 7-2 2-7-7-7 7-2-2 7-7-7-7 2-2Z' }] },
  cleared: { viewBox: '0 0 24 24', parts: [{ d: 'm3 12 2.3-2.4 4.4 4.2L18.8 4l2.4 2.3L9.8 18 3 12Z' }] },
  running: { viewBox: '0 0 24 24', parts: [{ d: 'M11 13 8 21h8l-3-8m-2 0h4', stroke: 2.2 }, { d: 'M12 2c.6 2.2 3.5 3.7 3.5 6.6A3.5 3.5 0 0 1 12 12a3.5 3.5 0 0 1-3.5-3.4C8.5 6 11 4.6 12 2Zm0 4.3c-.2 1.2-1.2 1.8-1.2 2.7a1.2 1.2 0 0 0 2.4 0c0-.9-.8-1.5-1.2-2.7Z' }] },
  gateClosed: { viewBox: '0 0 24 40', parts: [{ d: 'M2.5 38V15.5a9.5 9.5 0 0 1 19 0V38M5.5 38V15.5a6.5 6.5 0 0 1 13 0V38', stroke: 2.5 }, { d: 'M3 18h2.5M3 28h2.5M18.5 18H21M18.5 28H21', stroke: 1.2 }, { d: 'M6 13h12M6 24h12M7.5 13v19l-1.2 3 1.2-.8 1.2.8-1.2-3M10.5 13v19l-1.2 3 1.2-.8 1.2.8-1.2-3M13.5 13v19l-1.2 3 1.2-.8 1.2.8-1.2-3M16.5 13v19l-1.2 3 1.2-.8 1.2.8-1.2-3', stroke: 1.8 }] },
  gateOpen: { viewBox: '0 0 24 40', parts: [{ d: 'M2.5 38V15.5a9.5 9.5 0 0 1 19 0V38M5.5 38V15.5a6.5 6.5 0 0 1 13 0V38', stroke: 2.5 }, { d: 'M3 18h2.5M3 28h2.5M18.5 18H21M18.5 28H21', stroke: 1.2 }, { d: 'M6 8h12M7.5 8v5l-1 2 1-.6 1 .6-1-2M10.5 8v5l-1 2 1-.6 1 .6-1-2M13.5 8v5l-1 2 1-.6 1 .6-1-2M16.5 8v5l-1 2 1-.6 1 .6-1-2M6 14h12', stroke: 1.8 }] },
  stubDiamond: { viewBox: '0 0 64 64', parts: [{ d: 'M32 5 59 32 32 59 5 32 32 5Z', stroke: 4 }] },
  priorityGem: { viewBox: '0 0 16 16', parts: [{ d: 'M3 3h10l2 4-7 8-7-8 2-4Z', stroke: 1.5 }, { d: 'M3 3 5 7l3 8 3-8 2-4M1 7h14M5 7h6', stroke: 1.5 }] },
  bossFrame: { viewBox: '0 0 240 280', parts: [{ d: 'M40 4H96l8 10 8-10h24l8 10 8-10h48l-4 16 14-8v28l-10 8 10 8v168l-10 8 10 8v28l-14-8 4 16h-48l-8-10-8 10h-24l-8-10-8 10H40l4-16-14 8v-28l10-8-10-8V66l10-8-10-8V22l14 8L40 4Zm0 42h160M40 234h160', stroke: 3 }] },
} satisfies Record<string, Icon>

export type IconName = keyof typeof DATA

export const ICONS: Record<IconName, Icon> = DATA
