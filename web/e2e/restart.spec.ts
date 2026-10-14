// Restarts a real backend under an open terminal. The shared test server is
// left alone: each test starts its own binary on a spare port with its own
// state directory, so nothing here touches the real config directory.
import { spawn, type ChildProcess } from 'node:child_process'
import { mkdirSync, mkdtempSync } from 'node:fs'
import { createServer } from 'node:net'
import { tmpdir } from 'node:os'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { expect } from '@playwright/test'
import { test } from './fixtures'

const here = dirname(fileURLToPath(import.meta.url))
const windows = process.platform === 'win32'
const bin = resolve(here, '../../bin/dungeon' + (windows ? '.exe' : ''))

const freePort = () =>
  new Promise<number>((ok, fail) => {
    const s = createServer()
    s.once('error', fail)
    s.listen(0, '127.0.0.1', () => {
      const { port } = s.address() as { port: number }
      s.close(() => ok(port))
    })
  })

class Backend {
  child?: ChildProcess
  url = ''
  constructor(
    readonly home: string,
    readonly port: number,
  ) {}

  /** Starts the binary and resolves to its sign-in URL. */
  start(...flags: string[]): Promise<string> {
    return new Promise((ok, fail) => {
      const child = spawn(bin, ['serve', '--addr', `127.0.0.1:${this.port}`, ...flags], {
        env: {
          ...process.env,
          DUNGEON_HOME: this.home,
          // Never the real GitHub: a token that nothing accepts and an endpoint nothing listens on.
          GH_TOKEN: 'restart-test',
          DUNGEON_GITHUB_ENDPOINT: 'http://127.0.0.1:1/graphql',
          SHELL: windows ? '' : '/bin/sh',
        },
        stdio: ['ignore', 'pipe', 'ignore'],
      })
      this.child = child
      let buf = ''
      child.stdout!.on('data', (d) => {
        buf += d
        const m = buf.match(/http:\/\/127\.0\.0\.1:\d+\/\?token=[0-9a-f]+/)
        if (m) {
          this.url = m[0]
          ok(m[0])
        }
      })
      child.once('error', fail)
      child.once('exit', () => fail(new Error('the backend exited before it was ready')))
    })
  }

  async stop() {
    const child = this.child
    if (!child || child.exitCode !== null) return
    child.removeAllListeners('exit') // stopping on purpose is not a failure to start
    const gone = new Promise((r) => child.once('exit', r))
    child.kill('SIGKILL') // a crash: a graceful stop ends the session with an exit frame instead
    await gone
  }
}

async function withOpenTerminal(page: import('@playwright/test').Page, flagsAfter: string[]) {
  const root = mkdtempSync(join(tmpdir(), 'dungeon-restart-'))
  const home = join(root, 'home')
  const work = join(root, 'work')
  mkdirSync(home)
  mkdirSync(work)
  const backend = new Backend(home, await freePort())
  const url = await backend.start()
  const token = new URL(url).searchParams.get('token')!
  const api = (method: string, path: string, data?: unknown) =>
    page.request.fetch(`http://127.0.0.1:${backend.port}${path}`, {
      method,
      data,
      headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
    })
  const camp = await (await api('POST', '/api/campaigns', { dir: work, name: 'restart' })).json()
  expect((await api('POST', '/api/sessions', { shell: true, campaign: camp.id })).status()).toBe(201)

  await page.goto(url)
  const screen = page.locator('.pane:not([hidden]) .xterm-rows')
  await expect(screen).toContainText(windows ? '>' : '$', { timeout: 25_000 })
  await page.locator('.pane:not([hidden]) .xterm').click()
  await page.keyboard.type('echo before-restart')
  await page.keyboard.press('Enter')
  await expect(screen).toContainText('before-restart')

  await backend.stop()
  const down = page.getByTestId('backend-down')
  await expect(down).toBeVisible({ timeout: 15_000 })
  await expect(down).toContainText('out of date')
  await backend.start(...flagsAfter)
  return { backend, status: page.locator('.pane:not([hidden]) .term-status') }
}

test('after a restart the page says the session is gone and stops retrying', async ({ page }) => {
  test.setTimeout(100_000)
  const { backend, status } = await withOpenTerminal(page, [])
  try {
    await expect(status).toContainText('Start a new one', { timeout: 40_000 })
    await expect(page.getByTestId('backend-down')).toHaveCount(0, { timeout: 40_000 }) // the app answers again
  } finally {
    await backend.stop()
  }
})

test('after a restart with a new token the page shows the sign-in message', async ({ page }) => {
  test.setTimeout(100_000)
  const { backend, status } = await withOpenTerminal(page, ['--new-token'])
  try {
    await expect(status).toContainText('access token changed', { timeout: 40_000 })
    expect(await status.innerText()).not.toMatch(/[0-9a-f]{64}/) // never the token itself
  } finally {
    await backend.stop()
  }
})
