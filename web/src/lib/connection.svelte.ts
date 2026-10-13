// Whether the page can reach the app's backend. Both the API calls and the
// terminal connections report here, so one indicator covers them.

let api = $state(true)
let terminals = $state<string[]>([])

export const connection = {
  /** True while an API request or a terminal connection is failing to reach the backend. */
  get unreachable(): boolean {
    return !api || terminals.length > 0
  },
  /** An API request got an answer (any status) or failed to reach the backend. */
  reportApi(reached: boolean) {
    api = reached
  },
  /** A terminal's connection is waiting out an unreachable backend, or has recovered. */
  reportTerminal(id: string, reachable: boolean) {
    const has = terminals.includes(id)
    if (!reachable && !has) terminals = [...terminals, id]
    else if (reachable && has) terminals = terminals.filter((t) => t !== id)
  },
  reset() {
    api = true
    terminals = []
  },
}

export const UNREACHABLE_NOTE = 'Cannot reach the app. What you see may be out of date.'
