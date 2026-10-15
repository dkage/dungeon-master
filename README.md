# Dungeon Master

A desktop cockpit for running CLI coding agents against your GitHub issues. The idea is to have a cockpit for using
agents like `claude code`, that would map to chats using subscriptions, instead of API Tokens (support for token-based
AI providers, are also in the roadmap).

Pick an issue, pick a skill, click a button, and an agent starts in a terminal with the right model, flags and context
already set.

> Status: early development. The Stitch cockpit, first-run sample flow, glossary, appearance controls, local hero
> settings, and quest-to-session launch flow (M4) are complete. M4.5 adds session attention signals; the dependency
> graph with creature sprites follows in M5a. The 15-creature roster and prototype sheets in `design/quest-encounters/` are
> the planned art inputs for that first renderer.
> See [docs/design/experience.md](docs/design/experience.md) for the current experience and milestone order,
> [release/](release/README.md) for the release plan, and [docs/ideas.md](docs/ideas.md) for ideas
> that are not in v1.

- v1 targets Claude Code only; other agents come later.
- Everything is local: no accounts, no telemetry, and nothing is added to your repositories by default.
- The theme (campaigns, quests, heroes, spells) is only in the app's UI. Anything written to git or GitHub uses neutral
  wording.

## Run it

### Requirements

To build from source and run the app, install these tools and make them available on your `PATH`:

| Tool                                              | Version / purpose                                                                         |
|---------------------------------------------------|-------------------------------------------------------------------------------------------|
| Go                                                | 1.26 or newer, as required by `go.mod`                                                    |
| Node.js                                           | Use 24.x, matching the project's CI configuration                                         |
| npm                                               | Use the version bundled with Node.js; installs and builds the frontend                    |
| Git                                               | Resolves campaign repositories and their GitHub remotes                                   |
| [Claude Code](https://claude.com/claude-code) CLI | Required to launch heroes; authenticate with Claude Code before launching sessions        |
| Make and a Unix shell                             | Required for the macOS/Linux `make` commands below; Windows uses PowerShell instead       |
| GitHub CLI (`gh`, optional)                       | Run `gh auth login` to fetch GitHub issues, or supply `GH_TOKEN` / `GITHUB_TOKEN` instead |

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

From the repository root:

```powershell
.\run.ps1
```

This installs frontend dependencies, builds Svelte and the Go executable, then starts the server. Stop any running
copy before rebuilding. To build without starting, use `.\run.ps1 -BuildOnly`. To start an existing build without
rebuilding, use `.\bin\dungeon.exe serve`.

If PowerShell blocks local scripts, use a process-scoped policy for this invocation:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\run.ps1
```

The script leaves your saved execution policy unchanged. Windows uses ConPTY for terminals and a `gh.exe` copy for
the GitHub pacing shim, so symlink privileges are not required. CLI lookup honors Windows executable extensions.

### Open the app

The Go executable embeds the built Svelte frontend and serves it itself; these commands start one server. Keep the
terminal open while using the app and press Ctrl+C to stop it.

Open the URL it prints (it carries an access token that is exchanged for a cookie). The token is stored in the config
directory, so a browser that is already signed in stays signed in across restarts; `dungeon serve --new-token` replaces
it. Add a campaign (a project directory), then click a hero to start a session in it. The right-hand panel lists the
project's GitHub issues; it needs credentials from `gh auth login` (or `GH_TOKEN`), or you can load sample quests.
Heroes come from `party.toml` in the config directory; without one, built-in Opus, Sonnet and Haiku heroes are offered. The built-in heroes pass model aliases (`--model opus`), which Claude resolves to the latest model of each family, so they stay current; put a full model name in a hero to pin it.

`make check` runs the Go and frontend tests; `make test-e2e` runs the browser tests (once:
`cd web && npx playwright install chromium`).

### Development with automatic updates

Stop the existing `make run` server, then run:

```sh
make dev
```

Open the backend's printed token URL once to sign in, then use **http://127.0.0.1:5173** for development.
Vite updates Svelte, TypeScript, CSS and asset changes in the browser. The backend builds once and stays running,
so edits do not terminate active agent sessions. Go changes take effect when you stop and restart development.
Ctrl+C stops both servers and ends their sessions.

For backend work when no active sessions need to survive, use `make dev-watch`. Go source or module changes then
rebuild and restart the backend automatically. Each restart terminates active agent sessions, including agents
editing this project from inside Dungeon. Use `make dev` for those sessions.

The optional watch mode uses the MIT-licensed [wgo](https://github.com/bokwoon95/wgo) watcher, pinned to v0.7.1 and
downloaded by Go on first use. It does not add a runtime dependency or reinstall frontend dependencies on every start.
To use other ports: `make dev DEV_ADDR=127.0.0.1:7880 DEV_PORT=5174`.
`make run` remains the way to check the complete embedded production build.

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
and the `gh` shim is a symlink on macOS and Linux and a copy of the program on Windows. State files use private Unix
modes on macOS/Linux and protected Windows ACLs. See
[release/](release/README.md) for how each platform is built and tested.

## GitHub safety

Agents started here get a `gh` that paces its writes (about one every 3 s, at most 15 a minute and 150 an hour) and
stops while GitHub has asked for a pause, so a loop cannot get you locked out. It guards the common path only: it
cannot stop a command that calls `gh` by absolute path, another client such as `curl`, or `gh api`. For stronger
protection, give agents a [fine-grained token](https://github.com/settings/personal-access-tokens) limited to issues,
and start heroes without `--dangerously-skip-permissions`. Sessions inherit `GH_TOKEN` from the environment the app
started in; the Gatekeeper rule and its limits are described in [docs/design/github-budget.md](docs/design/github-budget.md).

## Acknowledgements

- Dungeon Master is inspired by [chartr](https://github.com/rengwu/chartr) by John Goh, an agent multiplexer with a map
  of your work. It started the idea of mapping registered agents to launch buttons. Dungeon Master is an independent
  project with its own design and code.
- The skills workflow builds on [Matt Pocock's skills](https://github.com/mattpocock/skills) (MIT) specially on the
  `wayfinder` skill, that helped me a lot during initial development of new features in new and legacy project..

## License

[MIT](LICENSE). Bundled fonts, libraries and the art notice are in [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).
