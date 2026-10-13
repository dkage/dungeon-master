import assert from 'node:assert/strict'
import { execFileSync, spawn } from 'node:child_process'
import { cp, mkdir, mkdtemp, readFile, symlink, writeFile } from 'node:fs/promises'
import { createServer } from 'node:net'
import { get } from 'node:http'
import { tmpdir } from 'node:os'
import { delimiter, resolve } from 'node:path'
import { setTimeout as delay } from 'node:timers/promises'
import { test } from 'node:test'
import { backendCommand } from './run.mjs'

const repo = resolve(import.meta.dirname, '..')

test('default development launches the backend directly, with no restart watcher', () => {
  assert.deepEqual(backendCommand(false, '127.0.0.1:7878'), ['./bin/dungeon-dev', 'serve', '--addr', '127.0.0.1:7878'])
  assert.ok(backendCommand(true, '127.0.0.1:7878').includes('github.com/bokwoon95/wgo@v0.7.1'))
})

function request(url, headers = {}) {
  return new Promise((done, reject) => {
    const req = get(url, { headers, agent: false }, (response) => {
      let body = ''
      response.on('data', (data) => { body += data })
      response.on('end', () => done({ status: response.statusCode, headers: response.headers, body }))
      response.on('error', reject)
    })
    req.on('error', reject)
  })
}

async function freePort() {
  const server = createServer()
  await new Promise((done) => server.listen(0, '127.0.0.1', done))
  const port = server.address().port
  await new Promise((done) => server.close(done))
  return port
}

async function until(check) {
  const end = Date.now() + 30_000
  while (Date.now() < end) {
    try { if (await check()) return } catch { /* wait for startup/rebuild */ }
    await delay(100)
  }
  assert.fail('development servers did not reach the expected state')
}

// Exercise the actual Make target in a copy, with isolated state and a fake gh.
// No tracked files, real agent, user configuration or GitHub service is touched.
for (const target of ['dev', 'dev-watch']) {
  test(`${target}: frontend server and backend restart policy`, { timeout: 90_000 }, async () => {
    const root = await mkdtemp(resolve(tmpdir(), 'dungeon-dev-test-'))
    for (const path of ['Makefile', 'go.mod', 'go.sum', 'cmd', 'internal', 'dev']) {
      await cp(resolve(repo, path), resolve(root, path), { recursive: true })
    }
    await mkdir(resolve(root, 'web/dist'), { recursive: true })
    await writeFile(resolve(root, 'web/dist/.gitkeep'), '')
    for (const path of ['embed.go', 'src', 'index.html', 'package.json', 'vite.config.ts', 'tsconfig.json']) {
      await cp(resolve(repo, 'web', path), resolve(root, 'web', path), { recursive: true })
    }
    await symlink(resolve(repo, 'web/node_modules'), resolve(root, 'web/node_modules'), 'dir')
    await mkdir(resolve(root, 'fake-bin'))
    const fakeGH = resolve(root, 'fake-bin/gh')
    execFileSync('go', ['build', '-o', fakeGH, './internal/testutil/cmd/fixture'], { cwd: repo })
    await writeFile(fakeGH + '.json', JSON.stringify({ Exit: 1 }))
    const backendPort = await freePort()
    const frontendPort = await freePort()
    const backend = `http://127.0.0.1:${backendPort}`
    const frontend = `http://127.0.0.1:${frontendPort}`
    const home = await mkdtemp(resolve(tmpdir(), 'dungeon-dev-home-'))
    const child = spawn('make', [target, `DEV_ADDR=127.0.0.1:${backendPort}`, `DEV_PORT=${frontendPort}`], {
      cwd: root, detached: true,
      env: { ...process.env, DUNGEON_HOME: home, GH_TOKEN: '', GITHUB_TOKEN: '',
        SHELL: '', PATH: `${resolve(root, 'fake-bin')}${delimiter}${process.env.PATH}` },
      stdio: ['ignore', 'pipe', 'pipe'],
    })
    let logs = ''
    child.stdout.on('data', (data) => { logs += data })
    child.stderr.on('data', (data) => { logs += data })
    const starts = () => (logs.match(/listening on http:/g) ?? []).length
    try {
      await until(async () => starts() === 1 && (await request(frontend)).status === 200)
      assert.match((await request(frontend)).body, /\/@vite\/client/)
      const token = /\?token=([a-f0-9]+)/.exec(logs)[1]
      const signIn = await request(`${backend}/?token=${token}`)
      const cookie = signIn.headers['set-cookie'][0].split(';')[0]
      assert.equal((await request(`${frontend}/api/state`, { cookie })).status, 200)

      // Frontend edits must not restart the backend in either mode.
      const css = resolve(root, 'web/src/app.css')
      await writeFile(css, await readFile(css, 'utf8') + '\n/* development probe */\n')
      await delay(1000)
      assert.equal(starts(), 1)

      await writeFile(resolve(root, 'cmd/dungeon/dev_probe.go'), 'package main\n')
      if (target === 'dev-watch') await until(() => starts() === 2)
      else { await delay(2000); assert.equal(starts(), 1) }
      await until(async () => (await request(`${frontend}/api/state`, { cookie })).status === 200)
    } catch (error) {
      error.message += '\n' + logs.replace(/token=[a-f0-9]+/g, 'token=REDACTED')
      throw error
    } finally {
      process.kill(-child.pid, 'SIGTERM')
      await until(async () => {
        try { await request(frontend); return false } catch { return true }
      })
      await until(async () => {
        try { await request(backend); return false } catch { return true }
      })
    }
  })
}
