import type { Quest } from '../api'

/** A quest with plain defaults, for tests. */
export function quest(number: number, over: Partial<Quest> = {}): Quest {
  return {
    number, title: `Quest ${number}`, state: 'open', closed: false, url: '', labels: [], assignees: [],
    blockedBy: [], subTotal: 0, subDone: 0, comments: 0, prs: [], updated: '2026-10-01T00:00:00Z', ...over,
  }
}
