// Taking the appearance settings to another machine as a small JSON file (#156).
// The file holds appearance values only: it is built from the settings'
// own keys, so nothing else can ride along, and an import reads only those keys.
import { DEFAULT_SETTINGS, parseSettings, type UISettings } from './appearance'

export const FILE_KIND = 'dungeon-appearance'
export const FILE_NAME = 'dungeon-appearance.json'
/** Larger than any real file; refuses something that is clearly not one. */
export const MAX_FILE_BYTES = 16 * 1024

const KEYS = Object.keys(DEFAULT_SETTINGS) as (keyof UISettings)[]

/** The text to save: a version tag and the appearance values. */
export function exportText(settings: UISettings): string {
  const values: Record<string, unknown> = {}
  for (const key of KEYS) values[key] = settings[key]
  return JSON.stringify({ kind: FILE_KIND, version: 1, settings: values }, null, 2) + '\n'
}

export interface Change {
  key: keyof UISettings
  from: UISettings[keyof UISettings]
  to: UISettings[keyof UISettings]
}

export type ImportPlan =
  | { ok: false; error: string }
  | {
      ok: true
      /** What would change, in settings order. Empty when the file matches the current look. */
      changes: Change[]
      /** Fields in the file whose value is not allowed; they keep their current value. */
      invalid: (keyof UISettings)[]
      /** The settings after applying, every value valid. */
      next: UISettings
    }

/**
 * Reads an import file without applying anything: unknown fields are ignored,
 * every known value is checked against its allowed set and range, and the
 * result lists what would change so the user can confirm.
 */
export function planImport(text: string, current: UISettings): ImportPlan {
  if (text.length > MAX_FILE_BYTES) return { ok: false, error: 'That file is too large to be an appearance file.' }
  let data: unknown
  try {
    data = JSON.parse(text)
  } catch {
    return { ok: false, error: 'That file is not valid JSON.' }
  }
  const root = data && typeof data === 'object' && !Array.isArray(data) ? (data as Record<string, unknown>) : null
  const values = root?.kind === FILE_KIND && root.settings && typeof root.settings === 'object' && !Array.isArray(root.settings)
    ? (root.settings as Record<string, unknown>)
    : null
  if (!values) return { ok: false, error: 'That is not a Dungeon Master appearance file.' }

  const given: Record<string, unknown> = {}
  for (const key of KEYS) if (key in values) given[key] = values[key]
  // A value that does not survive validation keeps what the user has now, not the default.
  const checked = parseSettings({ ...current, ...given })
  const invalid = KEYS.filter((k) => k in given && checked[k] !== given[k])
  const merged: Record<string, unknown> = { ...current, ...given }
  for (const k of invalid) merged[k] = current[k]
  const next = parseSettings(merged)
  const changes: Change[] = KEYS.filter((k) => next[k] !== current[k]).map((k) => ({ key: k, from: current[k], to: next[k] }))
  return { ok: true, changes, invalid, next }
}

const LABELS: Record<keyof UISettings, string> = {
  font: 'Reading font', sidebarSize: 'Campaigns text size', terminalSize: 'Terminal text size', questsSize: 'Quests text size',
  wordMode: 'Words', theme: 'Theme', notifications: 'Desktop notifications', terminalFont: 'Terminal font',
  terminalFontName: 'Installed font name', terminalLineHeight: 'Line height', terminalColors: 'Terminal colours',
}
export const labelOf = (key: keyof UISettings) => LABELS[key]

export const show = (v: unknown) => (v === '' ? '(none)' : typeof v === 'boolean' ? (v ? 'on' : 'off') : String(v))
