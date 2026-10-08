// Starts the real dungeon binary for the browser tests, with an isolated
// config directory and a fake claude, and publishes its URL for the tests.
import { execFileSync, spawn } from 'node:child_process'
import { copyFileSync, mkdirSync, mkdtempSync, readFileSync, writeFileSync } from 'node:fs'
import { createServer } from 'node:http'
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

// The workdir is a real repository with a GitHub remote, and apps/api is a
// monorepo subfolder, so the tests can check how campaigns resolve them.
const subdir = join(workdir, 'apps', 'api')
mkdirSync(subdir, { recursive: true })
const git = (...args) =>
  execFileSync('git', args, {
    cwd: workdir,
    env: { ...process.env, GIT_CONFIG_GLOBAL: '/dev/null', GIT_CONFIG_SYSTEM: '/dev/null' },
  })
git('init', '-q')
git('remote', 'add', 'origin', 'git@github.com:acme/demo.git')
// copyFileSync keeps the mode of the source (executable).

// A plain directory (no git repository) for the "no GitHub repo" case.
const plaindir = join(root, 'plain')
mkdirSync(plaindir)

// A fake GitHub. It serves responses recorded from the real API, requires the
// token the server was given, and returns a hostile issue body for #9 so the
// tests can prove the page neutralises it.
const fixtures = resolve(here, '../../internal/quests/github/testdata')
// The recorded list plus one hostile issue (#9), so the tests can open it from the list.
const listJson = JSON.parse(readFileSync(join(fixtures, 'list.json'), 'utf8'))
listJson.data.repository.issues.nodes.push({
  number: 9, title: 'Hostile body', state: 'OPEN', stateReason: null, url: 'https://github.com/acme/demo/issues/9',
  updatedAt: new Date().toISOString(), labels: { nodes: [] }, assignees: { nodes: [] }, milestone: null, parent: null,
  blockedBy: { nodes: [] }, subIssuesSummary: { total: 0, completed: 0 }, closedByPullRequestsReferences: { nodes: [] },
  comments: { totalCount: 1 },
})
const recordedList = JSON.stringify(listJson)
const recordedDetail = readFileSync(join(fixtures, 'detail.json'), 'utf8')
const emptyPage = JSON.stringify({ data: { repository: { issues: { pageInfo: { hasNextPage: false }, nodes: [] } } } })
const hostile = JSON.stringify({
  data: { repository: { issue: {
    number: 9, title: 'Hostile body', state: 'OPEN', url: 'https://github.com/acme/demo/issues/9',
    updatedAt: new Date().toISOString(), createdAt: new Date().toISOString(),
    body: 'Hello <script>window.__pwned = 1</script><img src=x onerror="window.__pwned=1"> [x](javascript:window.__pwned=1) **still bold**',
    author: { login: 'mallory' }, labels: { nodes: [] }, assignees: { nodes: [] },
    blockedBy: { nodes: [] }, blocking: { nodes: [] }, subIssues: { nodes: [] },
    comments: { totalCount: 1, nodes: [{ author: { login: 'mallory' }, body: '<svg onload="window.__pwned=1">', createdAt: new Date().toISOString(), url: '' }] },
  } } },
})
const github = createServer((req, res) => {
  let body = ''
  req.on('data', (d) => (body += d))
  req.on('end', () => {
    res.setHeader('Content-Type', 'application/json')
    // The budget headers the real GitHub sends; the Quartermaster reads them.
    res.setHeader('X-Ratelimit-Limit', '5000')
    res.setHeader('X-Ratelimit-Remaining', '4936')
    res.setHeader('X-Ratelimit-Used', '64')
    res.setHeader('X-Ratelimit-Reset', String(Math.floor(Date.now() / 1000) + 1800))
    res.setHeader('X-Ratelimit-Resource', 'graphql')
    if (req.headers.authorization !== 'Bearer e2e-token') {
      res.statusCode = 401
      return res.end('{}')
    }
    const { variables = {} } = JSON.parse(body || '{}')
    if (variables.number !== undefined) {
      return res.end(variables.number === 3 ? recordedDetail : variables.number === 9 ? hostile : '{"data":{"repository":{"issue":null}}}')
    }
    res.end(variables.states?.[0] === 'CLOSED' ? emptyPage : recordedList)
  })
})
await new Promise((ok) => github.listen(0, '127.0.0.1', ok))
const githubUrl = `http://127.0.0.1:${github.address().port}/graphql`

const child = spawn(bin, ['serve', '--addr', `127.0.0.1:${port}`], {
  env: {
    ...process.env,
    DUNGEON_HOME: home,
    GH_TOKEN: 'e2e-token',
    DUNGEON_GITHUB_ENDPOINT: githubUrl,
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
    writeFileSync(join(here, '.url.json'), JSON.stringify({ url: m[0], workdir, subdir, plaindir }))
    buf = ''
  }
  process.stdout.write(d)
})
child.on('exit', (code) => process.exit(code ?? 0))
for (const sig of ['SIGINT', 'SIGTERM']) process.on(sig, () => { github.close(); child.kill(sig) })
