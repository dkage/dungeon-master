/**
 * Workspace shortcuts: a registry of semantic actions, binding syntax, validation and routing.
 * Pure, so the rules are testable. A binding only ever dispatches an allowlisted action id;
 * nothing a user types is executed.
 */
export type ActionId =
  | 'expand-projects' | 'expand-terminal' | 'expand-map' | 'expand-detail' | 'restore-split'
  | 'show-map' | 'show-list' | 'open-settings' | 'next-session' | 'previous-session'
  | 'search-terminal' | 'terminal-latest'

export type Action = { id: ActionId; label: string; default: string | null }

/**
 * `Mod` is Cmd on macOS and Ctrl elsewhere. The expansion keys use Ctrl everywhere: Cmd+Shift+3 and 4
 * belong to macOS screenshots.
 */
export const ACTIONS: readonly Action[] = [
  { id: 'expand-projects', label: 'Expand projects', default: 'Ctrl+Shift+1' },
  { id: 'expand-terminal', label: 'Expand terminal', default: 'Ctrl+Shift+2' },
  { id: 'expand-map', label: 'Expand map and list', default: 'Ctrl+Shift+3' },
  { id: 'expand-detail', label: 'Expand issue detail', default: 'Ctrl+Shift+4' },
  { id: 'restore-split', label: 'Restore split', default: 'Ctrl+Shift+0' },
  { id: 'show-map', label: 'Show map', default: null },
  { id: 'show-list', label: 'Show list', default: null },
  { id: 'open-settings', label: 'Open settings', default: 'Mod+,' },
  { id: 'next-session', label: 'Next session', default: 'Ctrl+PageDown' },
  { id: 'previous-session', label: 'Previous session', default: 'Ctrl+PageUp' },
  { id: 'search-terminal', label: 'Search terminal output', default: 'Ctrl+Shift+F' },
  { id: 'terminal-latest', label: 'Jump to latest output', default: 'Ctrl+Shift+End' },
]

export type Platform = 'mac' | 'other'
export const platformOf = (nav: { platform?: string } = typeof navigator === 'undefined' ? {} : navigator): Platform =>
  /Mac|iPhone|iPad/i.test(nav.platform ?? '') ? 'mac' : 'other'

export type Binding = { ctrl: boolean; alt: boolean; shift: boolean; meta: boolean; key: string }

const NAMED = new Set(['PageUp', 'PageDown', 'Home', 'End', 'ArrowUp', 'ArrowDown', 'ArrowLeft', 'ArrowRight', 'Enter', 'Tab', 'Space', 'Backspace', 'Delete', 'Insert', 'Escape'])
const PUNCT = new Set([',', '.', '/', ';', "'", '[', ']', '\\', '-', '=', '`'])

function normaliseKey(key: string): string | null {
  if (key === ' ') return 'Space'
  if (/^[A-Za-z]$/.test(key)) return key.toLowerCase()
  if (/^[0-9]$/.test(key)) return key
  if (/^F([1-9]|1[0-2])$/.test(key)) return key
  if (NAMED.has(key) || PUNCT.has(key)) return key
  return null
}

/** Parses `Ctrl+Shift+1`; `Mod` resolves for the platform. Null for invalid syntax. */
export function parseBinding(text: string, platform: Platform): Binding | null {
  const parts = text.split('+')
  const key = parts.pop()
  if (!key) return null
  const b: Binding = { ctrl: false, alt: false, shift: false, meta: false, key: '' }
  for (const m of parts) {
    const name = m === 'Mod' ? (platform === 'mac' ? 'Meta' : 'Ctrl') : m
    if (name === 'Ctrl') b.ctrl = true
    else if (name === 'Alt') b.alt = true
    else if (name === 'Shift') b.shift = true
    else if (name === 'Meta') b.meta = true
    else return null
  }
  const k = normaliseKey(key)
  if (!k) return null
  b.key = k
  return b
}

export function formatBinding(b: Binding): string {
  return [b.ctrl && 'Ctrl', b.alt && 'Alt', b.shift && 'Shift', b.meta && 'Meta', b.key].filter(Boolean).join('+')
}

/** The label to show: Cmd/Option on macOS. */
export function labelOf(text: string | null, platform: Platform): string {
  if (!text) return 'Not set'
  if (platform !== 'mac') return text
  return text.replace(/\bMeta\b/g, 'Cmd').replace(/\bAlt\b/g, 'Option').replace(/\bCtrl\b/g, 'Control')
}

const CODE_KEY = (code: string): string | null =>
  /^Digit[0-9]$/.test(code) ? code.slice(5) : /^Key[A-Z]$/.test(code) ? code.slice(3).toLowerCase() : null

/** The binding a key event represents. Digits and letters use the physical key, so Shift+1 stays "1". */
export function fromEvent(e: Pick<KeyboardEvent, 'key' | 'code' | 'ctrlKey' | 'altKey' | 'shiftKey' | 'metaKey'>): Binding | null {
  if (['Control', 'Alt', 'Shift', 'Meta', 'Dead', 'Process'].includes(e.key)) return null
  const key = CODE_KEY(e.code) ?? normaliseKey(e.key)
  if (!key) return null
  return { ctrl: e.ctrlKey, alt: e.altKey, shift: e.shiftKey, meta: e.metaKey, key }
}

/** Keys the browser or operating system owns; they are never bound. */
function reservedReason(b: Binding, platform: Platform): string | null {
  const cmd = platform === 'mac' ? b.meta : b.ctrl
  if (cmd && !b.alt && !b.shift && 'cvxazywtnqrlfpsdkb'.includes(b.key) && b.key.length === 1) {
    return 'The browser or system owns this key (clipboard, tabs, window, reload, find).'
  }
  if (cmd && /^F\d+$/.test(b.key)) return 'The browser owns this key.'
  if (b.key === 'F5' || b.key === 'F11' || b.key === 'F12') return 'The browser owns this key.'
  if (b.ctrl && b.key === 'Tab') return 'The browser owns tab switching.'
  if (platform === 'mac' && b.meta && b.shift && !b.ctrl && !b.alt && ['3', '4', '5'].includes(b.key)) {
    return 'macOS owns this key (screenshots).'
  }
  return null
}

/** Why this binding cannot be used, or null when it can. */
export function validate(b: Binding, platform: Platform): string | null {
  if (!b.ctrl && !b.alt && !b.meta) return 'Add Ctrl, Alt or Cmd: unmodified keys belong to typing.'
  if (b.key === 'Escape') return 'Escape is used to close dialogs and restore the split.'
  return reservedReason(b, platform)
}

export type Overrides = Partial<Record<ActionId, string | null>>

/** The effective canonical binding for an action: override, disabled (null) or the platform default. */
export function effective(id: ActionId, overrides: Overrides, platform: Platform): string | null {
  if (id in overrides) return overrides[id] ?? null
  const def = ACTIONS.find((a) => a.id === id)?.default
  if (!def) return null
  const b = parseBinding(def, platform)
  return b ? formatBinding(b) : null
}

/** Checks a proposed canonical binding for an action: syntax, reserved keys and duplicates. */
export function check(id: ActionId, text: string, overrides: Overrides, platform: Platform): { ok: true; binding: string } | { ok: false; reason: string } {
  const b = parseBinding(text, platform)
  if (!b) return { ok: false, reason: 'That is not a valid key combination.' }
  const bad = validate(b, platform)
  if (bad) return { ok: false, reason: bad }
  const canonical = formatBinding(b)
  const other = ACTIONS.find((a) => a.id !== id && effective(a.id, overrides, platform) === canonical)
  if (other) return { ok: false, reason: `Already used by "${other.label}".` }
  return { ok: true, binding: canonical }
}

/** Where a key event happened: ordinary editing contexts own unmodified keys. */
export function inEditing(target: EventTarget | null): boolean {
  const el = target as Element | null
  return !!el?.closest?.('input, textarea, select, [contenteditable=""], [contenteditable="true"], .xterm')
}

/**
 * The action a key event triggers, or null. IME composition and auto-repeat never trigger, and
 * nothing triggers while a dialog is open (its own keys come first).
 */
export function route(
  e: KeyboardEvent,
  overrides: Overrides,
  platform: Platform,
  dialogOpen: boolean,
): ActionId | null {
  if (dialogOpen || e.isComposing || e.repeat || e.keyCode === 229) return null
  const b = fromEvent(e)
  if (!b || validate(b, platform)) return null
  const text = formatBinding(b)
  return ACTIONS.find((a) => effective(a.id, overrides, platform) === text)?.id ?? null
}

export const KEY = 'dungeon.shortcuts'

export function parseOverrides(raw: string | null, platform: Platform): Overrides {
  if (!raw) return {}
  let v: unknown
  try {
    v = JSON.parse(raw)
  } catch {
    return {}
  }
  if (!v || typeof v !== 'object') return {}
  const out: Overrides = {}
  for (const a of ACTIONS) {
    const val = (v as Record<string, unknown>)[a.id]
    if (val === null) out[a.id] = null
    else if (typeof val === 'string') {
      const b = parseBinding(val, platform)
      if (b && !validate(b, platform)) out[a.id] = formatBinding(b)
    }
  }
  // A stored duplicate (edited by hand) keeps the first claim and drops the rest.
  const seen = new Set<string>()
  for (const a of ACTIONS) {
    const t = effective(a.id, out, platform)
    if (!t) continue
    if (seen.has(t)) delete out[a.id]
    else seen.add(t)
  }
  return out
}
