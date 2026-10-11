# Glossary

This file is the development reference (plain meaning, code name). The in-app source of truth is planned as
`web/src/lib/words.ts` with the `Term` popovers (M3.6, `PLAN_EXPERIENCE.md`); when it exists this table is generated
from or checked against it, and the terms below (Quartermaster, Rations, Camp, Chronicle, Caravan, Gatekeeper) are added
there.

The themed words are for the user interface. Each one always shows its plain meaning where it appears, and Go
identifiers use plain names. Nothing themed is ever written to GitHub or to a repository.

| Word              | Plain meaning                                                                                                    | Where the user sees it                          | Code name              |
|-------------------|------------------------------------------------------------------------------------------------------------------|-------------------------------------------------|------------------------|
| **Campaign**      | a registered project directory (usually a git repository or a subfolder of one)                                  | sidebar                                         | `campaign`             |
| **Quest**         | a GitHub issue                                                                                                   | right-hand panel                                | `quests`               |
| **Hero**          | a named Claude configuration (model, flags): one launch button                                                   | quest detail                                    | `party`                |
| **Expedition**    | a running session of a hero (or a shell) in a campaign                                                           | sidebar, under its campaign                     | `session`, `history`   |
| **Dungeon**       | the map of a campaign's quests (rooms, locked doors)                                                             | the map, M5a                                    | `quests` (graph)       |
| **Spell**         | a skill                                                                                                          | spell strip, M5                                 | `spells`               |
| **Briefing**      | the generated context file a hero reads first                                                                    | M4                                              | `briefing`             |
| **Quartermaster** | the gatekeeper for all GitHub traffic: one request at a time, paced writes, stops when GitHub says slow down     | "GitHub budget" panel                           | `quartermaster`        |
| **Rations**       | the remaining GitHub request budget for the hour                                                                 | "GitHub requests: 4,936 of 5,000 left"          | `quartermaster.Budget` |
| **Camp**          | the app has paused its GitHub requests because GitHub asked it to slow down; it keeps working from the Chronicle | "Paused until 15:12: GitHub asked to slow down" | `quartermaster.Camp`   |
| **Chronicle**     | the local SQLite copy of the tracked issues, kept in sync with GitHub                                            | "Chronicle updated 12 s ago"                    | `chronicle`            |
| **Scout**         | the free check that asks GitHub whether anything changed                                                         | internal                                        | `chronicle` (probe)    |
| **Caravan**       | a paced batch of GitHub writes (importing a roadmap, for example), with preview, pause and resume                | "Caravan: 52 issues, about 9 min"               | `caravan`              |
| **Gatekeeper**    | the wrapper placed in front of `gh` inside agent sessions so agents cannot flood GitHub                          | settings, logs                                  | `gatekeeper`           |
| **Tracked**       | the label that marks an issue as part of the work done here. It is a plain GitHub label, neutral on purpose      | scope header: `tracked · 14 of 312 open`        | `scope`                |
| **Scope**         | which of a repository's issues the panel shows                                                                   | scope header and first-connection picker        | `scope`                |

**Internal words.** Quartermaster, Rations, Camp, Chronicle, Scout, Caravan and Gatekeeper are names for developers:
they appear in code, logs and these docs, never in the main interface. What the user sees is the plain sentence in the
"Where the user sees it" column. Every notice that stops or slows something follows one pattern: what happened, until
when, and what the user sees meanwhile. Sample (demo) campaigns never show GitHub notices at all.

Rules for adding a word: it must teach itself in the interface (plain meaning beside it), it must not leak into GitHub,
and every new word costs the user attention, so prefer none.
