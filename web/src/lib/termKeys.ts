// Keys the terminal translates before they reach the session.

// What the Claude Code prompt reads as "new line" instead of "submit"
// (probe on #89: ESC then CR, the same bytes Alt+Enter sends).
export const NEW_LINE = '\x1b\r'

type KeyLike = Pick<KeyboardEvent, 'type' | 'key' | 'shiftKey' | 'ctrlKey' | 'altKey' | 'metaKey' | 'isComposing'>

// The bytes to send for a key press the terminal handles itself, or null to
// leave it to xterm. Only a bare Shift+Enter is translated; plain Enter keeps
// sending a carriage return, and chords with other modifiers are untouched.
export function keyInput(ev: KeyLike): string | null {
  if (ev.type !== 'keydown' || ev.isComposing) return null
  if (ev.key === 'Enter' && ev.shiftKey && !ev.ctrlKey && !ev.altKey && !ev.metaKey) return NEW_LINE
  return null
}
