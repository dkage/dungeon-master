---
status: current
milestone: cockpit and workspace focus
github_issues: [28, 43, 63, 64, 65, 67, 68, 71, 72, 77, 82]
tracking: in-gh-issues
next: finish prototype parity, then expansion/shortcuts; see the roadmap for priority
verified: 2026-10-13
---

# Screens and controls

Every screen of the cockpit, region by region: what is on it, what each control does, and which milestone builds it.
Principles come from `docs/design/experience.md`; the map itself is designed in `docs/design/map.md`; words follow `GLOSSARY.md` and
the plain-words mode in `docs/design/review-findings.md` §9.

This file describes behaviour in our own terms.

The look is set by the prototype in `design/stitch-prototype/` (v4, the Dungeon pack's default theme, §6.5). The
earlier `design/quest-encounters/` prototype is retained as a historical right-panel study and source asset folder.
Current visual guidance is in `docs/design/art-direction.md`. Where a prototype and this file disagree on behaviour,
this file wins.

## 1. Window anatomy

```
┌────────────────────┬──────────────────────────────────┬──────────────────────────────────┐
│ [Search…        ]  │ api-server  owner/api  [📁] [↻]  │ Map  [>_]     [⤢] [×]  [Pane ◧]  │
│ CAMPAIGNS   [>_][+]│──────────────────────────────────│ [← All] Signup flow (hall title) │
│ ┌────────────────┐ │                                  │                                  │
│ │ api-server [+][⋯]│                                  │      ○───○───●                   │
│ │  ● #42 Validate│ │  terminal of the selected        │       \     /                    │
│ │  ○ shell       │ │  expedition (full height)        │        ◌───○                     │
│ └────────────────┘ │                                  │══════════ drag to resize ════════│
│ ┌────────────────┐ │                                  │ 42  Validate email addresses [×] │
│ │ Sample [sample]│ │                                  │ feature · open · ready  [Brief]  │
│ └────────────────┘ │                                  │ (markdown body)                  │
│                    │                                  │                                  │
│ ⚙ Settings        │                                  │ [Opus ▾] [/tdd ▾] [▶ Start]      │
└────────────────────┴──────────────────────────────────┴──────────────────────────────────┘
```

Three regions: the **left panel** (where), the **centre** (the terminal), the **right panel** (what: map and quest).
Settings open as a modal over the whole window (§6). The default split remains the prototype baseline. Planned
Expand/Restore controls on projects, terminal, map/list and issue detail temporarily focus one region; all use the
same workspace state and preserve selections, sessions and split geometry (`docs/design/workspace.md`, #63).

## 2. Left panel

### 2.1 Search

A filter box at the top. Typing filters campaigns and expeditions in place. A future command palette can use the semantic actions
(`docs/design/experience.md` §7.5); command-palette binding and issue jumps are not delivered by the current filter.

### 2.2 Campaigns header

`CAMPAIGNS` with two buttons on the right:

| Button | Does                                                                                        |
|--------|---------------------------------------------------------------------------------------------|
| `>_`   | opens a **scratch shell**: a plain terminal in the home directory, not tied to any campaign |
| `+`    | **adds a campaign** with the operating system's folder picker (see §3)                      |

Milestone: M4 campaign actions. M3.6 keeps the first-run repo suggestions and typed path; these quick actions follow
the quest launch path.

### 2.3 Campaign card

One card per campaign: name, `owner/repo` (or "no GitHub repository"), and two buttons.

- `+` opens a small menu: one entry per hero ("Opus: free session") and "Shell". A free session starts the hero with
  no quest and no spell, in the campaign's folder. This replaces today's hero buttons in the toolbar.
- `⋯` opens: Open folder (in Finder or the file manager), Change which issues to show (the scope picker), Rename,
  Remove (asks once, says files are not touched).
- The sample campaign carries a **Sample** badge and a "Remove sample" entry instead of Remove.

Milestone: M4 campaign actions and free sessions.

### 2.4 Expeditions under a card

One row per session, under its campaign:

- a state dot (working, needs you, idle, exited, lost), with the state in words for screen readers;
- the title: `#42 Validate email addresses` for a quest session, `Opus` for a free session, `Shell` for a shell;
- the spell, when there is one, as a small tag (`/tdd`);
- `×` to close; a running session asks once.

Selecting a row shows its terminal in the centre and selects its quest on the map.

### 2.5 Bottom

`⚙ Settings`, always visible. A small GitHub budget chip can sit next to it (from M3.5) in plain words.

## 3. Adding a campaign

The `+` button opens the **operating system's own folder picker**. The browser cannot do this (it never reveals an
absolute path), so the server, which runs on the same machine, opens it:

| System  | Picker                                                                       |
|---------|------------------------------------------------------------------------------|
| macOS   | `osascript` with `choose folder` (returns a POSIX path)                      |
| Linux   | `zenity --file-selection --directory`, else `kdialog --getexistingdirectory` |
| Windows | PowerShell with the folder browser dialog                                    |
| Webview | the native shell's own dialog, when the shell provides one                   |

Rules:

- `POST /api/pick-folder` behind the existing guard (token, Host, Origin, JSON). It opens one dialog at a time, returns
  `{path}` or `{cancelled: true}`, and has a long timeout because a person is choosing. Commands are run as argv,
  never through a shell, and the prompt text is fixed.
- When no picker is available (a headless Linux box, a missing `zenity`), the dialog falls back to the typed path field
  with suggestions (`docs/design/experience.md` §5.1). The typed field also stays available behind "Type a path instead".
- After a folder is picked, a short confirm step shows what was found: the git root (and that the folder is a subfolder
  of it, if so), `owner/repo`, an editable name, and, for a GitHub repository, the scope question from M3.5 (which
  issues to show). A folder without git or GitHub is allowed: heroes and shells work, the quest panel says why it is
  empty.
- Adding a folder that is already a campaign selects the existing one instead.
- Tests: a fake picker binary on `PATH` in feature tests (picked, cancelled, slow, missing), and the hostile cases of
  the endpoint (wrong Origin, no token, concurrent calls).

Milestone: M4 usability follow-up. M3.6 keeps the typed path and repository suggestions so first run works in browser
and headless environments; the native folder picker and confirmation flow can follow the quest-launch path.

## 4. Centre

### 4.1 Header bar

Campaign name, `owner/repo`, then:

- **Open folder** (reveal the campaign's directory);
- **Refresh** (quests now; throttled as today);
- on the right, **Pane** toggles the right panel, so the terminal can take the whole width.

When a quest session is selected, the header also shows its quest chip (`#42`, click selects it on the map), its hero
and its spell.

### 4.2 Terminal

The selected expedition's terminal at full height. Sessions stay mounted while hidden, as today. Tabs above the
terminal list the live sessions with their attention state, so parallel work on several issues is one click apart
(#82, under #79). This replaces the earlier no-tabs rule: the left panel still lists sessions by project, and the
tabs follow the same selection.

## 5. Right panel

Layout and map behaviour are in `docs/design/map.md` §2 and §3. The controls around them:

### 5.1 Map header

- Title `Map`, with **Map | List** toggle, **Expand / Restore split** (fills the application workspace; #64) and
  **Close** (hides the panel, same as the Pane toggle).
- **Shell here** (`>_`): opens a new shell session in the campaign's folder, listed under the campaign in the left
  panel like any other session. It is per folder, not per quest, so it sits in the panel header rather than the quest
  footer. Same action as "Shell" in the campaign card's `+` menu; the right panel just keeps it within reach while
  reading quests.
- In the Dungeon pack the title reads **Quest encounters** (as in the v4 prototype); the Neutral pack and plain-words
  mode keep `Map`. A `?` button opens the plain-language help for the panel's words (the help layer from M3.6).
- **List view** has state filters with counts, as in the prototype: All, Ready, In progress, Blocked, Cleared. Each row
  shows a small creature or diamond, the title, the effort word and its relations ("Blocks #4", "Blocked by #3"), and
  the state tag at the right. Closed quests are struck through. A dungeon filter (default: all, with the Tavern as an
  entry) narrows the list to one epic. The list is a separate view, not a level of the map.
- **Levels and breadcrumb**: the map opens on the world level (dungeons and the Tavern). Double-click or Enter on a
  node opens it; the breadcrumb (`World › Onboarding flow · 1 of 4 cleared`) and Escape go back up. Clicking the
  breadcrumb title shows the epic itself in the detail (its plan, notes, decisions). Rules in `docs/design/map.md` §3,
  Levels. This replaces the scope chip and "All N quests" button of the v4 prototype.

### 5.2 Quest detail

- Header: number, title, **Expand / Restore split** (#64), `×` (deselects).
- Chips: the quest's kind (from labels), its state in words, and **ready** when it can be started now.
- **Brief** button: a preview of exactly what will be sent if the quest is started now: the hero's command line, the
  spell, the opener, the appended instructions and the briefing file. It builds trust, and it is the first place to
  look when an agent misunderstands a quest. From M4, when the briefing exists.
- Body: the sanitised markdown, relations (part of, blocked by, blocking, sub-issues) and comments, as in M3.

### 5.3 Detail footer (pinned)

- **Hero / model combo**: choose a configured hero, whose configuration determines the Claude Code model and launch
  flags. Show its crest or glyph and model name. Default to the campaign's last-used hero.
- **Spell / skill combo**: choose one available skill for this quest. Default to the recommended skill when one exists;
  include **None — open without a skill** as an explicit option. Group other skills Plan / Do / Check and show their
  source and availability. When no recommendation exists, default to None. The user can override either default.
- **Start**: one deliberate action launches the selected hero on the selected quest. If a skill is selected, invoke it
  automatically as the opener when the hero starts, with the quest context; no second manual skill command is needed.
  If None is selected, start a plain quest session with its briefing and safety instructions, and an opener that only
  says to read the brief. Choosing a value in either combo does not itself launch anything.
- Keep both combos and Start visible while the detail body scrolls. At a 392 px panel width, wrap the controls into
  two compact rows if needed rather than truncating skill names. The **Brief** preview reflects the current hero,
  skill or None, opener, appended instructions and briefing file.
- A hero with no quest at all (a free session) is not started from here; that is the campaign card's `+` menu (§2.3).

| Footer action    | Hero | Quest context | Skill invoked | Where                    |
|------------------|------|---------------|---------------|--------------------------|
| Start with skill | yes  | yes           | yes           | detail footer            |
| Start with None  | yes  | yes           | no            | detail footer            |
| Free session     | yes  | no            | no            | campaign card `+` menu   |
| Shell here       | no   | no            | no            | right panel header, card |

- Disabled states say why next to Start, not only in a tooltip. A locked or closed quest keeps both combos and Start
  visible, but Start is disabled; "Blocked by #41, #43" links select the blockers (rules in `docs/design/map.md` §3).
  The v4 prototype also puts a **Jump to prerequisite #N** button inside the blocked-by card and shows a lock on the
  disabled combos and Start ("Blocked · prerequisite required", "Resolve #3 to unlock"); build that. A quest
  with an active session offers **Go to session** instead of starting a duplicate.

## 6. Settings

Decided: a **modal**. A large dialog centred over the whole window (about 80% of its width and height), with the
section list on its left and the selected section on its right. Sessions keep running behind it.

- Opens with ⌘, (Ctrl+, elsewhere), the app menu, or `⚙ Settings` in the left panel. Closes with Escape, ⌘W or `×`.
- Changes apply as they are made; there is no Save button, except inside dialogs that create something (New hero).
- The last section viewed is remembered.
- A dialog opened from inside Settings (New hero, Add source) stacks above it; Escape closes only the top one.

Sections in this order, each with a one-line description of what it controls.

### 6.1 Heroes

A table: name, model, permission mode, and edit and delete actions, with **New hero** at the top.

The edit dialog:

| Field           | Notes                                                                                 |
|-----------------|---------------------------------------------------------------------------------------|
| Name            | shown on buttons and in the session list                                              |
| Model           | a combo of known model ids, free text allowed                                         |
| Permission mode | default, accept edits, plan, or skip all prompts; the last one shows a plain warning  |
| Extra arguments | a list where **each entry is one argument**; no shell, no splitting, no quoting rules |
| Environment     | `KEY=VALUE` pairs; values are masked after saving                                     |
| Default hero    | a star; the default for new campaigns                                                 |
| Command preview | the exact command line that will run, updated as the fields change                    |

Where heroes are stored: a local file, `party.toml`, in the app's config directory (on macOS
`~/Library/Application Support/dungeon/`). It is a plain-text settings file in TOML format, one block per hero. Before
this screen existed it was the only way to define heroes; from now on **users never need to know it exists**.

Decided: **the app owns the file.** Settings is the way to edit heroes. The app rewrites the whole file atomically
(0600) on every change, with a header line saying it is managed by the app. Hand edits still work and are picked up (the
TOML library does not keep comments, so they are lost on the next save), and a file with errors shows the error
and its line while the last good set of heroes keeps working. "Open in editor" stays in Local files (§6.3) for people
who want it.

To consider during M3.6: the file name carries a theme word. A plain name (`agents.toml`), with a one-time rename of
an existing `party.toml`, would match the rule that anything outside the UI uses neutral wording.

Milestone: M3.6 for the table and dialog, because the starter heroes become editable there.

### 6.2 Spells and buttons

Three parts, matching `docs/design/skills.md`:

1. **Sources**: discovered sources (the project's skills folder, the user's, installed plugins) listed read-only with a
   skill count; registered sources (a folder or a git URL pinned to a commit) with enable, order (drag), refresh,
   remove, and **Add source**. Order is precedence when two sources have a skill with the same name.
2. **Recommendation rules**: which spell is preselected in the quest footer for which kind of quest. A table of rows
   such as `label bug → /diagnosing-bugs`, `label research → /research`, `anything else → /implement`, each a combo
   of the discovered spells, with a link that reveals the skill on disk.
3. **Skill presets**: the user binds a skill to a named preset, optionally with fixed extra text after the skill
   (`/research` + "focus on security"). Presets appear as entries in the skill combo. This is how new skills become
   launch choices without code.

Milestone: M5b (spells). Registered folder/Git sources with per-project mappings are the primary source model.
Project/user/plugin discovery supplements it. Source pins, availability and verified invocation follow
`docs/design/skills.md`; fetching a repository never implies native skill registration.

### 6.3 Local files

Every local config file in one place, each with its path, its state (not created, fine, or the error with its line),
and **Create from defaults**, **Open in editor** and **Reveal**:

- `party.toml` (heroes), and later the terminal look and notification settings if they are files.

Milestone: M4.5 usability follow-up. Hero editing remains in M3.6 Settings; this section is for direct file status and
file actions.

The same pattern as the starter-heroes action in `docs/design/experience.md` §5.3, applied to every file.

### 6.4 Other sections

| Section            | Contents                                                                  | Milestone |
|--------------------|---------------------------------------------------------------------------|-----------|
| Appearance & words | Dungeon/Neutral theme and word mode, selectable fonts, independent panel sizes | M3.6 |
| GitHub             | account and token source, budget in plain words, write-back toggles (off) | M3.5      |
| Terminal           | scrollback length (colours follow the palette); independent font size is in Appearance & words | M4.5 |
| Notifications      | when to notify (needs you, finished, failed), system notifications        | M4.5      |
| Session titles     | automatic titles for free sessions                                        | later     |

### 6.5 Appearance: theme picker (theme packs)

A **Theme** combo at the top of Appearance & words. A theme is not just colours: it is a **theme pack** that can change
everything about how the app looks and what things are called, while nothing about what the app does changes.

**What a theme pack contains**

| Part     | Example: Dungeon (v1 default)                | Example: a later Starship pack          |
|----------|----------------------------------------------|-----------------------------------------|
| Words    | campaign, quest, hero, spell, expedition     | sector, mission, crew, protocol, sortie |
| Map skin | torchlight rooms, doors, `!` and `?` markers | star chart, jump lanes, beacons         |
| Icons    | crest, door, padlock, torch                  | ship, airlock, beacon                   |
| Palettes | one or more dark palettes and one light one  | its own dark and light palettes         |
| Fonts    | interface and display fonts                  | its own fonts                           |
| Flavour  | empty-state lines, the hero token style      | its own lines and token style           |

**What a theme pack never changes**

- data, behaviour, layout, keyboard shortcuts, URLs, settings, and anything written to GitHub or git;
- Go identifiers, API fields and file names, which stay neutral;
- safety and error text, which is plain in every pack;
- accessibility: every pack passes the same contrast, focus and reduced-motion rules.

**The Neutral pack is the plain mode.** The plain-words mode (`docs/design/review-findings.md` §9) becomes simply the Neutral pack:
standard IT words (project, issue, agent, skill, session), the plain graph map (direction A in `docs/design/map.md`), standard
icons and calm palettes. "Themed with hints" stays a separate switch that works for every themed pack: on first sight a
themed word shows its plain name, and the hover popover always gives the plain meaning.

**Palette and pack**: each pack ships its palettes; inside a pack the user can pick among them or follow the system
(light or dark). So the Settings section has two combos: **Theme** (the pack) and **Palette** (within the pack, default
"System").

**Dungeon typography choice (settled 2026-10-09):** Appearance & words offers **Almendra**, **EB Garamond**, and
**Alegreya** as the selectable serif family for the left campaign panel and right quest panel. Almendra is the preferred
default. Each of the three visible panels has its own font-size control: campaigns/sidebar, terminal, and quests. The
terminal size changes xterm's size only; its monospaced Nerd Font stack remains fixed so terminal glyphs keep working.
The selected serif never changes the terminal font. Keep Cinzel for display headings and Fira Code for code samples as
shown in the visual baseline. Bundle the selectable font files with their matching SIL Open Font License texts; the
font study in `design/font-study/` documents the candidates and Portuguese coverage (`ç`, `ã`, `á`, and other accented
letters).

### What v1 builds now (cheap seams)

Only the Dungeon pack and the Neutral pack ship in v1. The point now is to keep the seams in place, so a new pack later
is mostly data, not a rewrite:

1. **Words** come from one table keyed by neutral keys (`entity.campaign`, `state.locked`, ...). Each pack provides a
   value for every key; the Neutral pack is the plain meaning. A unit test fails when a pack misses a key, so packs
   stay complete. Sentences use templates over keys, with singular and plural.
2. **Map skins** implement the one renderer interface in `docs/design/map.md` §5. A pack names its skin; the layout model is
   shared by all skins.
3. **Icons** are referenced by meaning (`icon.quest`, `icon.locked`, `icon.hero`), never by file. Each pack maps
   meanings to its own SVGs; the Neutral pack uses a standard icon set.
4. **Design tokens** carry every colour, font, radius and shadow. Components use only tokens. A pack's palettes are
   token files; switching sets `data-pack` and `data-palette` on the root element.
5. **Flavour text** (empty-state lines, hints) is part of the words table, so a pack can replace it or leave it out.
6. **No per-pack components in v1.** Packs change words, tokens, icons and the map skin. Component variants per pack (a
   different button shape, framed panels) are a later step, and only through tokens or a small set of named
   variants, never by forking components.

### Rules for packs

- **Packs are data plus a skin that ships with the app.** A pack is a words file, token files, icon SVGs and the name
  of a map skin built into the binary. No pack can carry scripts or arbitrary CSS, so a pack downloaded later cannot
  run code in a page that controls terminals. User-installable packs (if ever) load only this data, with SVGs
  sanitised.
- **Each pack has a concept budget too.** Same count of visible nouns as the Neutral pack; a pack translates the words,
  it does not add new concepts.
- **Contrast is tested per palette**: body text and button text at 4.5:1 or better, the danger colour readable on every
  surface it is used on. A unit test computes the ratios from the token values.
- **The terminal follows the palette** (its 16 colours, background, selection), but keeps the Nerd Font stack so its
  icons work in every pack.
- **Fonts ship inside the binary**, never from a font service, with their licence texts (for example the SIL Open Font
  License).
- A preview strip next to the combos shows the pack's words, a few map rooms, the state markers and a line of terminal
  text, so a choice can be made without closing Settings.
- A starting dark palette exists from earlier design work (a dark, green-tinted palette with measured contrast and an
  IBM Plex Sans/Mono plus Libre Baskerville pairing). It is input to the design round, not a decision.
- **The Dungeon pack's visual baseline is the v4 prototype** in `design/stitch-prototype/` (gothic, Diablo-like:
  framed dark-red panels, a parchment terminal, Cinzel and Fira Code). Its colours, fonts and textures become the
  pack's tokens and assets; its generated HTML is a reference, never copied into `web/`.

Milestones: M3.6 applies the Stitch v4 visual foundation to the Svelte cockpit shell, records the Dungeon/Neutral theme
choice, ships the glossary and font controls, and establishes the right-panel split. Full theme-pack data seams,
multiple palettes, pack completeness and contrast tests are deferred until a second pack is approved; do not build a
generic pack framework ahead of that need. The reference's remaining textures and frames (parchment terminal page,
slate side panels, framed detail, list rows and headers) are Dungeon baseline work in #72, done with theme tokens and
plain CSS rather than per-pack component variants. The reference's Clear and Reload buttons are intentionally omitted
(decision #77): the terminal's own commands cover clearing, and reattaching to a running session covers
reconnecting. M5a fills the reserved map area with the sprite-backed interactive graph. New packs, such
as a starship pack, are a later initiative in `docs/ideas.md`.

## 7. Keyboard and platform conventions

### 7.1 Shortcuts settings

Settings gains a dedicated Shortcuts page (#65): named actions, editable/disabled bindings, platform defaults, conflict
feedback, per-action reset and restore defaults. It remains accessible while a panel is expanded. The exact action,
focus, input-context and persistence contract is `docs/design/workspace.md`. Shortcut dispatch and visible buttons use
one semantic registry; a key never launches an agent implicitly.

### 7.2 Browser and native shell

Browser keys owned by the host remain host actions, with working button alternatives and documented bindings. Do not
intercept close/quit/reload or ordinary terminal/editing keys. Native menus can bind the same semantic actions when
shell work lands; keep system Edit menu behavior on macOS for clipboard support and platform-correct Settings access.
Native menu integration is #36/#57 work, not a claim that the current browser UI implements it. Operating-system
fullscreen is distinct from expanding an application panel. Avoid arbitrary-script menu bridges and double dispatch.

## 8. Milestone summary

- **M3.6 (shipped)**: Stitch v4 cockpit shell and right-panel list/detail split; onboarding and sample campaign;
  glossary, starter heroes and hero settings; Dungeon/Neutral appearance choice, three bundled reading fonts, and
  independent panel font sizes. This is the scope in `docs/design/experience.md` §4 and the M3.6 acceptance issue.
- **M4**: detail footer with hero combo, None skill choice and Start; Brief preview; quest chip in the centre header;
  quest session titles; campaign action menu and free-session/shell actions. The operating-system folder picker is a
  follow-up after the typed path and repository suggestions already available in first run.
- **M4.5**: "needs you" state dots, Dock or taskbar badge, Notifications settings; Settings navigation and Local
  files section; browser keyboard shortcuts and shortcut help; session terminal settings. Keep native app menus and
  window state in R1.
- **M5a**: the sprite-backed graph in the map area established in M3.6; stable per-issue creature assignment from
  known effort tiers, separate state markers, Map / List behavior, expand, and the three levels (world map of
  dungeons, dungeon view, the Tavern) with the breadcrumb (`docs/design/map.md`).
- **M5b**: Spells settings (sources, recommendation rules, skill presets) and the populated skill combo.
- **R1** (release): native menus with Settings on ⌘, and the standard Edit menu (§7.1), window state.
- **Later**: automatic titles for free sessions, art for hero crests.
- **Later visual-system work**: add full pack/palette abstractions and contrast/completeness checks only when a second
  theme pack is approved; M3.6's Dungeon/Neutral choice and font settings remain the v1 baseline.

## 9. Decisions and open questions

Decided (2026-10-08; updated 2026-10-09):

1. Settings is a **modal** over the whole window (§6).
2. The app **owns the heroes file**; users edit heroes in Settings and never need the file (§6.1).
3. The **scratch shell** stays, as `>_` next to the campaigns header (§2.2).
4. Skill presets are **global** for now. Per-campaign presets are a later idea (`docs/ideas.md`).
5. A locked quest **cannot be started**: no "start anyway" override (`docs/design/map.md` §3).
6. The terminal stays in the centre while the right panel holds the map/list and quest detail
   (`docs/design/experience.md` §3, `docs/design/map.md` §2). Updated 2026-10-13: session tabs above the terminal are
   wanted for parallel work (#82), replacing the earlier "no centre stage tabs" rule.

Still open:

1. Rename `party.toml` to a plain name such as `agents.toml`?
