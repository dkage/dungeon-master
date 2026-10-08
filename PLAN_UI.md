# Screens and controls

Every screen of the cockpit, region by region: what is on it, what each control does, and which milestone builds it.
Principles come from `PLAN_EXPERIENCE.md`; the map itself is designed in `PLAN_MAP.md`; words follow `GLOSSARY.md` and
the plain-words mode in `PLAN_M3_7.md` §2.9.

This file describes behaviour in our own terms.

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
│ ⚙ Settings         │                                  │ [Opus ▾] [Open] [▶ /tdd] [More▾] │
└────────────────────┴──────────────────────────────────┴──────────────────────────────────┘
```

Three regions: the **left panel** (where), the **centre** (the terminal), the **right panel** (what: map and quest).
Settings open as a modal over the whole window (§6).

## 2. Left panel

### 2.1 Search

A filter box at the top. Typing filters campaigns and expeditions in place. Ctrl/Cmd+K opens the command palette
(`PLAN_EXPERIENCE.md` §7.5) for jumping to a quest by number or title, which the filter box does not do.

### 2.2 Campaigns header

`CAMPAIGNS` with two buttons on the right:

| Button | Does                                                                                        |
|--------|---------------------------------------------------------------------------------------------|
| `>_`   | opens a **scratch shell**: a plain terminal in the home directory, not tied to any campaign |
| `+`    | **adds a campaign** with the operating system's folder picker (see §3)                      |

### 2.3 Campaign card

One card per campaign: name, `owner/repo` (or "no GitHub repository"), and two buttons.

- `+` opens a small menu: one entry per hero ("Opus: free session") and "Shell". A free session starts the hero with
  no quest and no spell, in the campaign's folder. This replaces today's hero buttons in the toolbar.
- `⋯` opens: Open folder (in Finder or the file manager), Change which issues to show (the scope picker), Rename,
  Remove (asks once, says files are not touched).
- The sample campaign carries a **Sample** badge and a "Remove sample" entry instead of Remove.

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
  with suggestions (`PLAN_EXPERIENCE.md` §5.1). The typed field also stays available behind "Type a path instead".
- After a folder is picked, a short confirm step shows what was found: the git root (and that the folder is a subfolder
  of it, if so), `owner/repo`, an editable name, and, for a GitHub repository, the scope question from M3.5 (which
  issues to show). A folder without git or GitHub is allowed: heroes and shells work, the quest panel says why it is
  empty.
- Adding a folder that is already a campaign selects the existing one instead.
- Tests: a fake picker binary on `PATH` in feature tests (picked, cancelled, slow, missing), and the hostile cases of
  the endpoint (wrong Origin, no token, concurrent calls).

Milestone: M3.6 (first run), since adding a project is step 2 of the Start-here checklist.

## 4. Centre

### 4.1 Header bar

Campaign name, `owner/repo`, then:

- **Open folder** (reveal the campaign's directory);
- **Refresh** (quests now; throttled as today);
- on the right, **Pane** toggles the right panel, so the terminal can take the whole width.

When a quest session is selected, the header also shows its quest chip (`#42`, click selects it on the map), its hero
and its spell.

### 4.2 Terminal

The selected expedition's terminal at full height. Sessions stay mounted while hidden, as today. No tabs in the centre:
the left panel is the session list, so tabs would be a second copy of it.

## 5. Right panel

Layout and map behaviour are in `PLAN_MAP.md` §2 and §3. The controls around them:

### 5.1 Map header

- Title `Map`, with **Map | List** toggle, **Expand** (the map takes the centre until Escape) and **Close** (hides the
  panel, same as the Pane toggle).
- **Shell here** (`>_`): opens a new shell session in the campaign's folder, listed under the campaign in the left
  panel like any other session. It is per folder, not per quest, so it sits in the panel header rather than the quest
  footer. Same action as "Shell" in the campaign card's `+` menu; the right panel just keeps it within reach while
  reading quests.
- **Drilling into a hall**: clicking a parent issue's region zooms into it and shows `← All` plus the parent's title.
  Clicking the title shows the parent issue itself in the detail (its plan, notes, decisions). This is how an epic with
  many children is read without collapsing the whole graph.

### 5.2 Quest detail

- Header: number, title, `×` (deselects).
- Chips: the quest's kind (from labels), its state in words, and **ready** when it can be started now.
- **Brief** button: a preview of exactly what will be sent if the quest is started now: the hero's command line, the
  spell, the opener, the appended instructions and the briefing file. It builds trust, and it is the first place to
  look when an agent misunderstands a quest. From M4, when the briefing exists.
- Body: the sanitised markdown, relations (part of, blocked by, blocking, sub-issues) and comments, as in M3.

### 5.3 Detail footer (pinned)

- Left: **hero combo** with the hero's crest or glyph. Defaults to the campaign's last-used hero.
- Right: the **recommended spell** as the white primary button, labelled with the spell (`▶ /tdd`), then **More**:
  other spells grouped Plan / Do / Check and any custom buttons (§6.2).
- Between them, a plain **Open** button (secondary style): starts the selected hero on this quest **without a
  spell**. The agent gets the quest's briefing and the safety instructions, and the opener only says "read the brief";
  no skill is invoked, so the user leads the conversation. It is a visible button, not buried in More, because talking
  the quest through before choosing an approach is a normal way to start.
- When a quest has no recommended spell (no rule matches, or no spells are installed), **Open** becomes the primary
  button and the spell buttons collapse into More.
- A hero with no quest at all (a free session) is not started from here; that is the campaign card's `+` menu (§2.3).

| Footer action    | Hero | Quest briefing | Skill invoked | Where                    |
|------------------|------|----------------|---------------|--------------------------|
| `▶ /tdd` (spell) | yes  | yes            | yes           | footer, primary          |
| Open             | yes  | yes            | no            | footer, secondary        |
| Free session     | yes  | no             | no            | campaign card `+` menu   |
| Shell here       | no   | no             | no            | right panel header, card |

- Disabled states say why next to the buttons, not only in a tooltip. A locked quest keeps its spell buttons
  visible but greyed out, with "Blocked by #41, #43" as links that select the blockers (rules in `PLAN_MAP.md` §3).

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
(0600) on every change, with a header line saying it is managed by the app. Hand edits still work and are picked up
(the TOML library does not keep comments, so they are lost on the next save), and a file with errors shows the error
and its line while the last good set of heroes keeps working. "Open in editor" stays in Local files (§6.3) for people
who want it.

To consider during M3.6: the file name carries a theme word. A plain name (`agents.toml`), with a one-time rename of
an existing `party.toml`, would match the rule that anything outside the UI uses neutral wording.

Milestone: M3.6 for the table and dialog, because the starter heroes become editable there.

### 6.2 Spells and buttons

Three parts, matching `PLAN_SKILLS.md`:

1. **Sources**: discovered sources (the project's skills folder, the user's, installed plugins) listed read-only with a
   skill count; registered sources (a folder or a git URL pinned to a commit) with enable, order (drag), refresh,
   remove, and **Add source**. Order is precedence when two sources have a skill with the same name.
2. **Recommendation rules**: which spell becomes the main button for which kind of quest. A table of rows such as
   `label bug → /diagnosing-bugs`, `label research → /research`, `anything else → /implement`, each a combo of the
   discovered spells, with a link that reveals the skill on disk.
3. **Custom buttons**: the user binds any spell to a named button, optionally with fixed extra text after the spell
   (`/research` + "focus on security"). Custom buttons appear in **More**, and can be pinned so they also show next to
   the main button. This is how new skills become new buttons without code.

Milestone: M5b (spells).

### 6.3 Local files

Every local config file in one place, each with its path, its state (not created, fine, or the error with its line),
and **Create from defaults**, **Open in editor** and **Reveal**:

- `party.toml` (heroes), and later the terminal look and notification settings if they are files.

The same pattern as the starter-heroes action in `PLAN_EXPERIENCE.md` §5.3, applied to every file.

### 6.4 Other sections

| Section            | Contents                                                                  | Milestone |
|--------------------|---------------------------------------------------------------------------|-----------|
| Appearance & words | theme pack and palette (§6.5), hints, reduced motion                      | M3.6      |
| GitHub             | account and token source, budget in plain words, write-back toggles (off) | M3.5      |
| Terminal           | font size, scrollback length (colours follow the palette)                 | M4        |
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

**The Neutral pack is the plain mode.** The plain-words mode (`PLAN_M3_7.md` §2.9) becomes simply the Neutral pack:
standard IT words (project, issue, agent, skill, session), the plain graph map (direction A in `PLAN_MAP.md`), standard
icons and calm palettes. "Themed with hints" stays a separate switch that works for every themed pack: on first sight a
themed word shows its plain name, and the hover popover always gives the plain meaning.

**Palette and pack**: each pack ships its palettes; inside a pack the user can pick among them or follow the system
(light or dark). So the Settings section has two combos: **Theme** (the pack) and **Palette** (within the pack, default
"System").

### What v1 builds now (cheap seams)

Only the Dungeon pack and the Neutral pack ship in v1. The point now is to keep the seams in place, so a new pack later
is mostly data, not a rewrite:

1. **Words** come from one table keyed by neutral keys (`entity.campaign`, `state.locked`, ...). Each pack provides a
   value for every key; the Neutral pack is the plain meaning. A unit test fails when a pack misses a key, so packs
   stay complete. Sentences use templates over keys, with singular and plural.
2. **Map skins** implement the one renderer interface in `PLAN_MAP.md` §5. A pack names its skin; the layout model is
   shared by all skins.
3. **Icons** are referenced by meaning (`icon.quest`, `icon.locked`, `icon.hero`), never by file. Each pack maps
   meanings to its own SVGs; the Neutral pack uses a standard icon set.
4. **Design tokens** carry every colour, font, radius and shadow. Components use only tokens. A pack's palettes are
   token files; switching sets `data-pack` and `data-palette` on the root element.
5. **Flavour text** (empty-state lines, hints) is part of the words table, so a pack can replace it or leave it out.
6. **No per-pack components in v1.** Packs change words, tokens, icons and the map skin. Component variants per pack
   (a different button shape, framed panels) are a later step, and only through tokens or a small set of named
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

Milestones: M3.6 ships the seams (words table with keys and the Neutral pack, icons by meaning, tokens), the Theme and
Palette combos, the Dungeon and Neutral packs, and the completeness and contrast tests. The map skin choice per pack
lands with the map (M5a). New packs, such as a starship pack, are a later initiative in `PLAN_V2.md`.

## 7. Keyboard and platform conventions

The app is planned to ship as a native desktop application (the webview shell, `release/PLAN_RELEASE.md`), so it must
behave like one. A Mac app that does not open its settings with Cmd+, feels broken.

### 7.1 Native shell (the packaged app)

The shell installs a real application menu, so shortcuts work through the system and show up where users look for
them. On macOS:

| Menu                      | Item and shortcut                                                           |
|---------------------------|-----------------------------------------------------------------------------|
| Dungeon Master (app menu) | About · **Settings… ⌘,** · Hide ⌘H · Hide Others ⌥⌘H · Quit ⌘Q              |
| File                      | Add Campaign… ⇧⌘O · New Session ⌘T · Close Session ⌘W (asks if running)     |
| Edit                      | Undo, Redo, Cut, Copy, Paste, Select All (standard items)                   |
| View                      | Show Map ⌘1 · Show List ⌘2 · Toggle Pane ⌥⌘P · Expand Map ⇧⌘M · Zoom In/Out |
| Go                        | Command Palette ⌘K · Find in panel ⌘F · Next/Previous Session ⌃Tab/⌃⇧Tab    |
| Window                    | Minimize ⌘M · Zoom · Enter Full Screen ⌃⌘F                                  |
| Help                      | Words (the glossary) · Keyboard Shortcuts                                   |

Notes:

- **The Edit menu is not optional.** In a macOS webview, copy and paste (including in the terminal and in text fields)
  only work when the standard Edit menu items exist. Missing it is a classic bug in webview apps.
- Settings is reached from the menu and from ⌘, and also from the `⚙ Settings` button in the left panel; all three
  open the same modal. Escape or ⌘W closes it.
- Menu items call into the web UI through one small, fixed bridge (an allowlist of action names), not arbitrary script.
- On Linux and Windows the same actions use Ctrl instead of Cmd, and Settings is Ctrl+, (the convention of most
  editors on those systems), plus the platform's usual menu placement.
- The `webview_go` binding does not build menus itself, so the shell needs a small platform layer for them (Cocoa on
  macOS first). This belongs to release phase R1 and is part of its smoke test.

### 7.2 Browser mode (`dungeon serve`)

The same shortcuts are bound in the page, but a browser keeps some for itself (⌘, opens the browser's own settings in
Chrome and Safari, ⌘W closes the tab, ⌘T opens a tab). In the browser:

- shortcuts the browser reserves get a second binding that works (`Ctrl+,` or `g` then `s`), shown in the Keyboard
  Shortcuts help;
- nothing in the page tries to override ⌘W or ⌘Q;
- the help says which shortcuts are native-app only.

### 7.3 Terminal focus

When the terminal has focus, every key goes to the agent except the app's menu shortcuts in the native shell and a
small reserved set in the browser (⌘K, ⌘,). Ctrl+C, Escape and the arrow keys always belong to the terminal. Leaving
the terminal by keyboard is one documented shortcut (for example ⌃⇧Tab or ⌘\`), so the app is usable without a mouse.

### 7.4 Other desktop expectations

- The window remembers its size, position and the panel splits.
- Quitting with running sessions asks once and says how many will be stopped.
- A Dock badge (macOS) or taskbar badge shows how many sessions need you (M4.5).
- Settings changes apply immediately; there is no Save button except in dialogs that create something.
- Light and dark follow the system unless the user picks one.

Tests: a unit test that the shortcut table has no duplicate bindings per platform; browser tests for the in-page
bindings; the R1 smoke test opens Settings with ⌘, and pastes into a text field in the packaged app.

## 8. Milestone summary

- **M3.6**: in-page shortcuts (§7.2), campaign `+` with the folder picker, campaign card menus, scratch shell, the
  Settings page with Heroes, Local files and Appearance & words, including the theme and
  palette pickers (§6.5).
- **M4**: detail footer with hero combo and main button, Brief preview, quest chip in the centre header, Terminal
  settings.
- **M4.5**: "needs you" state dots, Dock or taskbar badge, Notifications settings.
- **M5a**: the right-panel map, Map / List, expand, drilling into halls (`PLAN_MAP.md`).
- **M5b**: Spells settings (sources, recommendation rules, custom buttons) and the More menu contents.
- **R1** (release): native menus with Settings on ⌘, and the standard Edit menu (§7.1), window state.
- **Later**: session titles, art for hero crests.

## 9. Decisions and open questions

Decided (2026-10-08):

1. Settings is a **modal** over the whole window (§6).
2. The app **owns the heroes file**; users edit heroes in Settings and never need the file (§6.1).
3. The **scratch shell** stays, as `>_` next to the campaigns header (§2.2).
4. Custom buttons are **global** for now. Per-campaign buttons are a later idea (`PLAN_V2.md`).
5. A locked quest **cannot be started**: no "start anyway" override (`PLAN_MAP.md` §3).

Still open:

1. No tabs in the centre (the left panel is the session list): agreed?
2. Rename `party.toml` to a plain name such as `agents.toml`?
