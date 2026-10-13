import { spawn } from 'node:child_process'
import { pathToFileURL } from 'node:url'

export function backendCommand(watch, addr) {
  if (!watch) return ['./bin/dungeon-dev', 'serve', '--addr', addr]
  return ['go', 'run', 'github.com/bokwoon95/wgo@v0.7.1',
    '-file', '\\.go$', '-file', '^go\\.(mod|sum)$', '-xfile', '_test\\.go$',
    '-xdir', '^(web|design|private|_probe|bin|build|dev)(/|$)',
    'go', 'build', '-o', 'bin/dungeon-dev', './cmd/dungeon',
    '::', './bin/dungeon-dev', 'serve', '--addr', addr]
}

async function main() {
  const addr = process.env.DEV_ADDR ?? '127.0.0.1:7878'
  const port = process.env.DEV_PORT ?? '5173'
  const env = { ...process.env, CGO_ENABLED: '0', DUNGEON_URL: `http://${addr}` }
  const children = new Set()
  let stopping = false
  function stop(code = 0) {
    if (stopping) return
    stopping = true
    process.exitCode = code
    for (const child of children) {
      try { process.kill(-child.pid, 'SIGTERM') } catch { /* already exited */ }
    }
  }
  process.on('SIGINT', () => stop())
  process.on('SIGTERM', () => stop())
  function start([cmd, ...args]) {
    const child = spawn(cmd, args, { env, stdio: 'inherit', detached: true })
    children.add(child)
    child.on('error', (error) => { console.error(error.message); stop(1) })
    child.on('exit', (code) => {
      children.delete(child)
      if (!stopping && code !== 0) stop(code ?? 1)
    })
    return child
  }
  if (env.DEV_WATCH !== '1') {
    const build = start(['go', 'build', '-o', 'bin/dungeon-dev', './cmd/dungeon'])
    await new Promise((done) => build.once('close', done))
    if (stopping) return
  } else {
    console.log('Backend watch enabled: each restart terminates active agent sessions.')
  }
  const backend = start(backendCommand(env.DEV_WATCH === '1', addr))
  const frontend = start(['npm', '--prefix', 'web', 'run', 'dev', '--', '--port', port, '--strictPort'])
  for (const child of [backend, frontend]) child.on('exit', () => stop())
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) await main()
