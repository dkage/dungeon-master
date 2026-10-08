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

## Acknowledgements

- Dungeon Master is inspired by [chartr](https://github.com/rengwu/chartr) by John Goh, an agent multiplexer with a map
  of your work. It started the idea of mapping registered agents to launch buttons. Dungeon Master is an independent
  project with its own design and code.
- The skills workflow builds on [Matt Pocock's skills](https://github.com/mattpocock/skills) (MIT) specially on the
  `wayfinder` skill, that helped me a lot during initial development of new features in new and legacy project..

## License

To be decided before the first release.
