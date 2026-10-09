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
> See [PLAN_EXPERIENCE.md](PLAN_EXPERIENCE.md) for the current experience and milestone order, [PLAN_SKILLS.md](PLAN_SKILLS.md)
> for how skills fit in, [release/](release/README.md) for the release plan, and [PLAN_V2.md](PLAN_V2.md) for ideas
> that are not in v1.

- v1 targets Claude Code only; other agents come later.
- Everything is local: no accounts, no telemetry, and nothing is added to your repositories by default.
- The theme (campaigns, quests, heroes, spells) is only in the app's UI. Anything written to git or GitHub uses neutral
  wording.

## Run it

Requires Go, Node and the [Claude Code](https://claude.com/claude-code) CLI.

```sh
make run        # builds the frontend and the binary, then serves on 127.0.0.1:7878
```

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
