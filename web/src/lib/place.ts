import type { MapLevel } from './store.svelte'

/**
 * Where the user was: identifiers only, never issue text or the access token. It lives in the
 * address-bar hash (reload, back/forward and copied links) with a copy in localStorage for a
 * bare URL (docs/design/workspace.md, "Remembering place").
 */
export type Place = {
  campaign?: string
  quest?: number
  session?: string
  view?: 'map' | 'list'
  level?: MapLevel
}

export const PLACE_KEY = 'dungeon.place'
const ID = /^[A-Za-z0-9_-]{1,64}$/

const levelText = (l: MapLevel) => (l.kind === 'world' ? 'w' : l.kind === 'tavern' ? 't' : `d${l.id}`)

function levelOf(text: string): MapLevel | undefined {
  if (text === 'w') return { kind: 'world' }
  if (text === 't') return { kind: 'tavern' }
  const m = /^d(\d{1,9})$/.exec(text)
  return m ? { kind: 'dungeon', id: Number(m[1]) } : undefined
}

/** The hash, without the leading '#'. Empty when there is nothing to remember. */
export function format(p: Place): string {
  const q = new URLSearchParams()
  if (p.campaign) q.set('c', p.campaign)
  if (p.session) q.set('s', p.session)
  if (p.quest !== undefined) q.set('q', String(p.quest))
  if (p.view) q.set('v', p.view)
  if (p.level && p.level.kind !== 'world') q.set('l', levelText(p.level))
  return q.toString()
}

/** Unknown, malformed or hostile input is dropped field by field. */
export function parse(hash: string | null | undefined): Place {
  const q = new URLSearchParams((hash ?? '').replace(/^#/, ''))
  const p: Place = {}
  const c = q.get('c')
  if (c && ID.test(c)) p.campaign = c
  const s = q.get('s')
  if (s && ID.test(s)) p.session = s
  const n = q.get('q')
  if (n && /^\d{1,9}$/.test(n) && Number(n) > 0) p.quest = Number(n)
  const v = q.get('v')
  if (v === 'map' || v === 'list') p.view = v
  const l = q.get('l')
  if (l) p.level = levelOf(l)
  return p
}
