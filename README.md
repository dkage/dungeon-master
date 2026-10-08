# Dungeon

A desktop cockpit for running CLI coding agents against your GitHub issues. Pick an issue, pick a skill, click a button, and an agent starts in a terminal with the right model, flags and context already set.

> Status: planning. Nothing is built yet. See [PLAN.md](PLAN.md) for the v1 plan, [PLAN_SKILLS.md](PLAN_SKILLS.md) for how skills fit in, and [PLAN_V2.md](PLAN_V2.md) for ideas that are not in v1.

- v1 targets Claude Code only; other agents come later.
- Everything is local: no accounts, no telemetry, and nothing is added to your repositories by default.
- The theme (campaigns, quests, heroes, spells) is only in the app's UI. Anything written to git or GitHub uses neutral wording.

## Acknowledgements

- Dungeon is inspired by [chartr](https://github.com/rengwu/chartr) by John Goh, an agent multiplexer with a map of your work. It started the idea of mapping registered agents to launch buttons. Dungeon is an independent project with its own design and code.
- The skills workflow builds on [Matt Pocock's skills](https://github.com/mattpocock/skills) (MIT).

## License

To be decided before the first release.
