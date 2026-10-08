import { afterEach, describe, expect, it, vi } from 'vitest'
import { ApiError, createSession, deleteSession, getState, termUrl } from './api'

function mockFetch(status: number, body?: unknown) {
  const fn = vi.fn(async () => ({
    ok: status >= 200 && status < 300,
    status,
    json: async () => {
      if (body === undefined) throw new Error('no body')
      return body
    },
  }))
  vi.stubGlobal('fetch', fn)
  return fn
}

afterEach(() => vi.unstubAllGlobals())

describe('api', () => {
  it('GET sends no content type and includes credentials', async () => {
    const f = mockFetch(200, { version: 'v' })
    await getState()
    const [url, init] = f.mock.calls[0] as unknown as [string, RequestInit]
    expect(url).toBe('/api/state')
    expect(init.method).toBe('GET')
    expect(init.credentials).toBe('same-origin')
    expect(init.headers).toEqual({})
  })

  it('POST sends JSON with the content type the guard requires', async () => {
    const f = mockFetch(201, { id: 's1' })
    await createSession({ hero: 'opus', dir: '/x' })
    const [, init] = f.mock.calls[0] as unknown as [string, RequestInit]
    expect(init.headers).toEqual({ 'Content-Type': 'application/json' })
    expect(JSON.parse(init.body as string)).toEqual({ hero: 'opus', dir: '/x' })
  })

  it('DELETE also sends the JSON content type and escapes the id', async () => {
    const f = mockFetch(204)
    await expect(deleteSession('a/b')).resolves.toBeUndefined()
    const [url, init] = f.mock.calls[0] as unknown as [string, RequestInit]
    expect(url).toBe('/api/sessions/a%2Fb')
    expect(init.headers).toEqual({ 'Content-Type': 'application/json' })
  })

  it('surfaces the server error message', async () => {
    mockFetch(400, { error: 'dir is not an existing directory' })
    await expect(createSession({ hero: 'x', dir: '/nope' })).rejects.toMatchObject({
      status: 400,
      message: 'dir is not an existing directory',
    })
  })

  it('explains 401 when there is no body', async () => {
    mockFetch(401)
    const err = await getState().catch((e) => e)
    expect(err).toBeInstanceOf(ApiError)
    expect(err.message).toContain('dungeon serve')
  })

  it('falls back to a generic message', async () => {
    mockFetch(500)
    await expect(getState()).rejects.toThrow('request failed (500)')
  })

  it('builds websocket urls for http and https', () => {
    expect(termUrl('s1', { protocol: 'http:', host: '127.0.0.1:7878' })).toBe('ws://127.0.0.1:7878/api/sessions/s1/term')
    expect(termUrl('s1', { protocol: 'https:', host: 'x.test' })).toBe('wss://x.test/api/sessions/s1/term')
  })
})
