// Starts the real dungeon binary for the browser tests, with an isolated
// config directory and a fake claude, and publishes its URL for the tests.
import { execFileSync, spawn } from 'node:child_process'
import { copyFileSync, mkdirSync, mkdtempSync, readFileSync, writeFileSync } from 'node:fs'
import { createServer } from 'node:http'
import { tmpdir } from 'node:os'
import { delimiter, dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const here = dirname(fileURLToPath(import.meta.url))
const windows = process.platform === 'win32'
const suffix = windows ? '.exe' : ''
const bin = resolve(here, '../../bin/dungeon' + suffix)
const port = process.env.E2E_PORT ?? '7899'

const root = mkdtempSync(join(tmpdir(), 'dungeon-e2e-'))
const home = join(root, 'home')
const workdir = join(root, 'work')
const fakeBin = join(root, 'bin')
// The user's own home, so skill discovery never reads the real one.
const userhome = join(root, 'userhome')
for (const d of [home, workdir, fakeBin, userhome]) mkdirSync(d)
// Build portable stub executables for the real PTY and shim.
const fixture = join(fakeBin, 'fixture' + suffix)
execFileSync('go', ['build', '-o', fixture, './internal/testutil/cmd/fixture'], { cwd: resolve(here, '../..') })
const ghLog = join(root, 'gh-calls.log')
for (const [name, options] of [['claude', { Mode: 'e2e-claude' }], ['gh', { Mode: 'e2e-gh', Log: ghLog }]]) {
  const target = join(fakeBin, name + suffix)
  copyFileSync(fixture, target)
  writeFileSync(target + '.json', JSON.stringify(options))
}
const emptyGitConfig = join(root, 'empty.gitconfig')
writeFileSync(emptyGitConfig, '')

// The workdir is a real repository with a GitHub remote, and apps/api is a
// monorepo subfolder, so the tests can check how campaigns resolve them.
const subdir = join(workdir, 'apps', 'api')
mkdirSync(subdir, { recursive: true })
const git = (...args) =>
  execFileSync('git', args, {
    cwd: workdir,
    env: { ...process.env, GIT_CONFIG_GLOBAL: emptyGitConfig, GIT_CONFIG_SYSTEM: emptyGitConfig },
  })
git('init', '-q')
git('remote', 'add', 'origin', 'git@github.com:acme/demo.git')
// One commit, so an issue session can start from it in its own working copy.
git('-c', 'user.email=e2e@example.test', '-c', 'user.name=e2e', 'commit', '-q', '--allow-empty', '-m', 'initial commit')
// copyFileSync keeps the mode of the source (executable).

// A plain directory (no git repository) for the "no GitHub repo" case.
const plaindir = join(root, 'plain')
mkdirSync(plaindir)

const fixtures = resolve(here, '../../internal/quests/github/testdata')

// A fake GitHub. It serves responses recorded from the real API, requires the
// token the server was given, and returns a hostile issue body for #9 so the
// tests can prove the page neutralises it.
// Responses recorded from the real API (see internal/quests/github/testdata), plus
// one hostile issue (#9) so the tests can open it from the list.
const rec = (name) => JSON.parse(readFileSync(join(fixtures, name), 'utf8'))
const hostileNode = {
  number: 9, title: 'Hostile body', state: 'OPEN', stateReason: null, url: 'https://github.com/acme/demo/issues/9',
  updatedAt: new Date().toISOString(), labels: { nodes: [] }, assignees: { nodes: [] }, milestone: null, parent: null,
  blockedBy: { nodes: [] }, subIssuesSummary: { total: 0, completed: 0 }, closedByPullRequestsReferences: { nodes: [] },
  comments: { totalCount: 1 },
}
const withTotal = (page) => {
  page.data.repository.total = { totalCount: 312 } // a busy repository: the panel says "N of 312 open"
  return page
}
const listPages = {
  all: withTotal(rec('list_all_open.json')),
  labels: withTotal(rec('list_labels_open.json')),
  assigned: withTotal(rec('list_assigned_open.json')),
}
listPages.all.data.repository.issues.nodes.push(hostileNode)
const emptyPage = JSON.stringify(withTotal({ data: { repository: { issues: { pageInfo: { hasNextPage: false }, nodes: [] } } } }))
const counts = rec('counts.json')
counts.data.repository.total.totalCount = 312 // the same busy repository
// A label whose pages never end, to see the "narrow the scope" notice.
counts.data.repository.labels.nodes.push({ name: 'huge', color: '000000', issues: { totalCount: 900 } })
const recordedDetail = readFileSync(join(fixtures, 'detail.json'), 'utf8')
const viewer = readFileSync(join(fixtures, 'viewer.json'), 'utf8')
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
// A plain open issue with no blockers, for tests that need a second one to start.
const plainIssue = (number, title) => JSON.stringify({
  data: { repository: { issue: {
    number, title, state: 'OPEN', url: `https://github.com/acme/demo/issues/${number}`,
    updatedAt: new Date().toISOString(), createdAt: new Date().toISOString(),
    body: 'A plain issue.', author: { login: 'octocat' }, labels: { nodes: [] }, assignees: { nodes: [] },
    blockedBy: { nodes: [] }, blocking: { nodes: [] }, subIssues: { nodes: [] },
    comments: { totalCount: 0, nodes: [] },
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
    const { query = '', variables = {} } = JSON.parse(body || '{}')
    if (query.includes('viewer{login}')) return res.end(viewer)
    if (query.includes('assigned: issues')) return res.end(JSON.stringify(counts))
    if (variables.number !== undefined) {
      return res.end(variables.number === 3 ? recordedDetail : variables.number === 9 ? hostile : variables.number === 5 ? plainIssue(5, '[demo] Bug: terminal loses colours') : '{"data":{"repository":{"issue":null}}}')
    }
    if (variables.states?.[0] === 'CLOSED') return res.end(emptyPage)
    const kind = variables.labels ? 'labels' : variables.filter ? 'assigned' : 'all'
    if (variables.labels?.includes('huge')) {
      // Pages that never end: the client stops at its cap and says the list was cut.
      const page = JSON.parse(JSON.stringify(withTotal({ data: { repository: { issues: { pageInfo: {}, nodes: [] } } } })))
      page.data.repository.issues.nodes = variables.after ? [] : listPages.labels.data.repository.issues.nodes
      page.data.repository.issues.pageInfo = { hasNextPage: true, endCursor: `p${(variables.after ?? 'p0').slice(1) * 1 + 1}` }
      return res.end(JSON.stringify(page))
    }
    res.end(JSON.stringify(listPages[kind]))
  })
})
await new Promise((ok) => github.listen(0, '127.0.0.1', ok))
const githubUrl = `http://127.0.0.1:${github.address().port}/graphql`

const child = spawn(bin, ['serve', '--addr', `127.0.0.1:${port}`], {
  env: {
    ...process.env,
    DUNGEON_HOME: home,
    HOME: userhome,
    USERPROFILE: userhome,
    GH_TOKEN: 'e2e-token',
    DUNGEON_GITHUB_ENDPOINT: githubUrl,
    PATH: `${fakeBin}${delimiter}${process.env.PATH}`,
    SHELL: windows ? '' : '/bin/sh',
  },
  stdio: ['ignore', 'pipe', 'inherit'],
})

let buf = ''
child.stdout.on('data', (d) => {
  buf += d
  const m = buf.match(/http:\/\/127\.0\.0\.1:\d+\/\?token=[0-9a-f]+/)
  if (m) {
    writeFileSync(join(here, '.url.json'), JSON.stringify({ url: m[0], workdir, subdir, plaindir, ghLog, userhome }))
    buf = ''
  }
  process.stdout.write(d)
})
child.on('exit', (code) => process.exit(code ?? 0))
for (const sig of ['SIGINT', 'SIGTERM']) process.on(sig, () => { github.close(); child.kill(sig) })
