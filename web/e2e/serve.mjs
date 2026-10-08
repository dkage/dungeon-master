// Starts the real dungeon binary for the browser tests, with an isolated
// config directory and a fake claude, and publishes its URL for the tests.
import { spawn } from 'node:child_process'
import { copyFileSync, mkdirSync, mkdtempSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const here = dirname(fileURLToPath(import.meta.url))
const bin = resolve(here, '../../bin/dungeon')
const port = process.env.E2E_PORT ?? '7899'

const root = mkdtempSync(join(tmpdir(), 'dungeon-e2e-'))
const home = join(root, 'home')
const workdir = join(root, 'work')
const fakeBin = join(root, 'bin')
for (const d of [home, workdir, fakeBin]) mkdirSync(d)
copyFileSync(join(here, 'fake-claude.sh'), join(fakeBin, 'claude'))
// copyFileSync keeps the mode of the source (executable).

const child = spawn(bin, ['serve', '--addr', `127.0.0.1:${port}`], {
  env: {
    ...process.env,
    DUNGEON_HOME: home,
    PATH: `${fakeBin}:${process.env.PATH}`,
    SHELL: '/bin/sh',
  },
  stdio: ['ignore', 'pipe', 'inherit'],
})

let buf = ''
child.stdout.on('data', (d) => {
  buf += d
  const m = buf.match(/http:\/\/127\.0\.0\.1:\d+\/\?token=[0-9a-f]+/)
  if (m) {
    writeFileSync(join(here, '.url.json'), JSON.stringify({ url: m[0], workdir: workdir }))
    buf = ''
  }
  process.stdout.write(d)
})
child.on('exit', (code) => process.exit(code ?? 0))
for (const sig of ['SIGINT', 'SIGTERM']) process.on(sig, () => child.kill(sig))
