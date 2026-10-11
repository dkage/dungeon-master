# Dungeon Master

A desktop cockpit for running CLI coding agents against your GitHub issues. The idea is to have a cockpit for using
agents like `claude code`, that would map to chats using subscriptions, instead of API Tokens (support for token-based
AI providers, are also in the roadmap).

Pick an issue, pick a skill, click a button, and an agent starts in a terminal with the right model, flags and context
already set.

> Status: early development. Heroes, terminals, campaigns and the GitHub quest list and detail work; starting a hero
> from a quest is next. See [PLAN.md](PLAN.md) for the v1 plan, [PLAN_SKILLS.md](PLAN_SKILLS.md) for how skills fit in,
> [release/](release/README.md) for the release plan, and [PLAN_V2.md](PLAN_V2.md) for ideas that are not in v1.

- v1 targets Claude Code only; other agents come later.
- Everything is local: no accounts, no telemetry, and nothing is added to your repositories by default.
- The theme (campaigns, quests, heroes, spells) is only in the app's UI. Anything written to git or GitHub uses neutral
  wording.

## Run it

### Requirements

To build from source and run the app, install these tools and make them available on your `PATH`:

| Tool | Version / purpose |
|------|-------------------|
| Go | 1.26 or newer, as required by `go.mod` |
| Node.js | Use 24.x, matching the project's CI configuration |
| npm | Use the version bundled with Node.js; installs and builds the frontend |
| Git | Resolves campaign repositories and their GitHub remotes |
| [Claude Code](https://claude.com/claude-code) CLI | Required to launch heroes; authenticate with Claude Code before launching sessions |
| Make and a Unix shell | Required for the macOS/Linux `make` commands below; Windows uses PowerShell instead |
| GitHub CLI (`gh`, optional) | Run `gh auth login` to fetch GitHub issues, or supply `GH_TOKEN` / `GITHUB_TOKEN` instead |

Check the required CLI tools from your terminal:

```text
go version
node --version
npm --version
git --version
claude --version
```

Svelte, Vite and the other frontend dependencies are installed locally by `npm ci`; no global installation is needed.
The build below disables cgo, so it does not require a C compiler. Node.js/npm are needed when building the frontend;
the built executable serves the embedded frontend without a separate Node.js server. A browser is needed to use the UI.

### macOS

From the repository root, with `make` and a Unix shell available:

```sh
make run        # builds the frontend and the binary, then serves on 127.0.0.1:7878
```

### Linux

From the repository root, with GNU Make and a Unix shell available:

```sh
make run
```

On both macOS and Linux, `make run` installs frontend dependencies with `npm ci`, builds Svelte with `npm run build`,
builds the Go binary, then starts the server. `make build` performs the same build steps without starting the server;
after building, use `./bin/dungeon serve` to start it again.

### Windows (PowerShell)

The Makefile uses Unix shell syntax and commands such as `touch`, so it does not run directly in PowerShell.
From the repository root, run the equivalent steps:

```powershell
# Install frontend dependencies and build Svelte
Set-Location web
npm ci
npm run build
Set-Location ..

# Build the Go executable and start the server
$env:CGO_ENABLED = "0"
go build -trimpath -o bin/dungeon.exe ./cmd/dungeon
.\bin\dungeon.exe serve
```

Stop if any build command fails and resolve the reported error before continuing. After building, subsequent starts
only need `.\bin\dungeon.exe serve`; rebuild after changing the code.

Windows support still needs verification on a real machine, including the PTY and Gatekeeper socket. The Windows
`gh.exe` shim copy is not implemented yet: the current installer attempts a symlink named `gh`. If installation fails,
the app continues with a warning that GitHub writes from agent sessions will not be paced.

### Open the app

The Go executable embeds the built Svelte frontend and serves it itself; these commands start one server. Keep the
terminal open while using the app and press Ctrl+C to stop it.

Open the URL it prints (it carries an access token that is exchanged for a cookie). The token is stored in the config
directory, so a browser that is already signed in stays signed in across restarts; `dungeon serve --new-token` replaces
it. Add a campaign (a project directory), then click a hero to start a session in it. The right-hand panel lists the
project's GitHub issues; it needs credentials from `gh auth login` (or `GH_TOKEN`), or you can load sample quests.
Heroes come from `party.toml` in the config directory; without one, built-in Opus, Sonnet and Haiku heroes are offered.

`make check` runs the Go and frontend tests; `make test-e2e` runs the browser tests (once:
`cd web && npx playwright install chromium`).

## Supported platforms

Dungeon Master is released for macOS, Linux and Windows, and every feature must work on all three (a requirement for
each milestone, not a later port).

| Platform | Minimum                        |
|----------|--------------------------------|
| macOS    | current and previous release   |
| Linux    | glibc or musl                  |
| Windows  | Windows 10 version 1803 (2018) |

Windows 10 1803 is the first version with Unix domain sockets and ConPTY, which the Gatekeeper and the terminals need.
Builds are for x86-64 and ARM; Linux is also tested in CI and with `make test-linux`.

Where the systems differ, the code takes the plain route on each: the Gatekeeper talks to the shim over a Unix domain
socket in a private per-user directory (file modes on macOS and Linux, the user's own temporary directory on Windows),
and the `gh` shim is a symlink on macOS and Linux and will be a copy of the program on Windows (symlinks need extra
rights there; not done yet). See
[release/](release/README.md) for how each platform is built and tested.

## GitHub safety

Agents started here get a `gh` that paces its writes (about one every 3 s, at most 15 a minute and 150 an hour) and
stops while GitHub has asked for a pause, so a loop cannot get you locked out. It guards the common path only: it
cannot stop a command that calls `gh` by absolute path, another client such as `curl`, or `gh api`. For stronger
protection, give agents a [fine-grained token](https://github.com/settings/personal-access-tokens) limited to issues,
and start heroes without `--dangerously-skip-permissions`. Sessions inherit `GH_TOKEN` from the environment the app
started in; the Gatekeeper rule and its limits are described in [PLAN_M3_5.md](PLAN_M3_5.md).

## Acknowledgements

- Dungeon Master is inspired by [chartr](https://github.com/rengwu/chartr) by John Goh, an agent multiplexer with a map
  of your work. It started the idea of mapping registered agents to launch buttons. Dungeon Master is an independent
  project with its own design and code.
- The skills workflow builds on [Matt Pocock's skills](https://github.com/mattpocock/skills) (MIT) specially on the
  `wayfinder` skill, that helped me a lot during initial development of new features in new and legacy project..

## License

To be decided before the first release.
