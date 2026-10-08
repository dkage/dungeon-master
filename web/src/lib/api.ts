// Typed client for the Dungeon server. Every write sends application/json,
// which the server's guard requires.

export interface Hero {
  id: string
  label: string
}

export interface SessionInfo {
  id: string
  kind: 'hero' | 'shell'
  hero?: string
  title: string
  dir: string
  created: string
  exited: boolean
  exitCode: number
}

export interface AppState {
  version: string
  claude: { found: boolean; path?: string; error?: string }
  defaultHeroes: boolean
  heroes: Hero[]
  sessions: SessionInfo[]
}

export interface CreateRequest {
  hero?: string
  shell?: boolean
  dir: string
  opener?: string
  cols?: number
  rows?: number
}

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message)
    this.name = 'ApiError'
  }
}

async function call<T>(method: string, path: string, body?: unknown): Promise<T | undefined> {
  const headers: Record<string, string> = {}
  if (method !== 'GET') headers['Content-Type'] = 'application/json'
  const res = await fetch(path, {
    method,
    headers,
    credentials: 'same-origin',
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  if (res.status === 204) return undefined
  let data: unknown
  try {
    data = await res.json()
  } catch {
    data = undefined
  }
  if (!res.ok) {
    const msg =
      data && typeof data === 'object' && 'error' in data
        ? String((data as { error: unknown }).error)
        : res.status === 401
          ? 'not authorised: open the URL printed by `dungeon serve` once'
          : `request failed (${res.status})`
    throw new ApiError(res.status, msg)
  }
  return data as T
}

export const getState = () => call<AppState>('GET', '/api/state') as Promise<AppState>
export const createSession = (req: CreateRequest) =>
  call<SessionInfo>('POST', '/api/sessions', req) as Promise<SessionInfo>
export const deleteSession = (id: string) =>
  call<void>('DELETE', `/api/sessions/${encodeURIComponent(id)}`) as Promise<void>

export function termUrl(id: string, loc: Pick<Location, 'protocol' | 'host'> = location): string {
  const scheme = loc.protocol === 'https:' ? 'wss' : 'ws'
  return `${scheme}://${loc.host}/api/sessions/${encodeURIComponent(id)}/term`
}
