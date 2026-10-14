import { formatBinding, fromEvent, labelOf, type Platform } from './shortcuts'

/** How long the test step waits for the keys before saying nothing arrived. */
export const TEST_WAIT_MS = 8000

export type TestState =
  | { kind: 'waiting' }
  | { kind: 'received' }
  | { kind: 'different'; got: string }
  | { kind: 'not-received' }
  | { kind: 'cancelled' }

/** What a key press means for a test of `expected`; null for a bare modifier press, which is not an answer yet. */
export function judge(
  expected: string,
  e: Pick<KeyboardEvent, 'key' | 'code' | 'ctrlKey' | 'altKey' | 'shiftKey' | 'metaKey'>,
  platform: Platform,
): TestState | null {
  if (e.key === 'Escape' && !e.ctrlKey && !e.altKey && !e.metaKey) return { kind: 'cancelled' }
  const b = fromEvent(e)
  if (!b) return null
  const got = formatBinding(b)
  return got === expected ? { kind: 'received' } : { kind: 'different', got: labelOf(got, platform) }
}

/** The line to show for a finished test. It never promises the key is safe from the browser. */
export function testMessage(s: TestState): string {
  switch (s.kind) {
    case 'waiting': return 'Press the shortcut now. Nothing will run. Esc cancels.'
    case 'received': return 'The page received these keys. Another program or a browser update could still take them later.'
    case 'different': return `The page received ${s.got}, not this shortcut. Try again, or pick another key.`
    case 'not-received': return 'Nothing arrived. The browser or system may have taken this key; choose another shortcut.'
    case 'cancelled': return 'Test cancelled.'
  }
}
