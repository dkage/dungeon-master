# Third-party notices

Dungeon Master is released under the [MIT License](LICENSE). It includes or builds on the work below, each under its
own licence. The licence texts of bundled fonts sit beside the font files.

## Original art

The creature sprites, textures, map pieces and other art in `design/` and `web/src/assets/` were made by the author
with image-generation tools from the author's own prompts, and are released under the MIT License with the code.

`design/stitch-prototype/` is the exported output of a design tool, kept as a visual reference only. It is not part of
the application, and the MIT grant does not cover its images or generated pages.

## Fonts

All are bundled unmodified.

| Font | Licence | Licence text |
|---|---|---|
| Alegreya | SIL OFL 1.1 | `web/src/assets/fonts/alegreya-OFL.txt` |
| Almendra | SIL OFL 1.1 | `web/src/assets/fonts/almendra-OFL.txt` |
| Cinzel | SIL OFL 1.1 | `web/src/assets/fonts/cinzel-OFL.txt` |
| EB Garamond | SIL OFL 1.1 | `web/src/assets/fonts/ebgaramond-OFL.txt` |
| Fira Code | SIL OFL 1.1 | `web/src/assets/fonts/firacode-OFL.txt` |
| Symbols Nerd Font Mono | MIT (a collection of glyphs from several icon sets; see the Nerd Fonts project) | `web/src/assets/fonts/symbolsnerdfont-MIT.txt` |

The reference folders `design/font-study/fonts/` and `design/stitch-prototype/assets/fonts/` hold more OFL fonts
(Cardo, Cormorant, Crimson Pro, MedievalSharp) with their licence texts.

## Frontend libraries (bundled in the built app)

| Package | Licence |
|---|---|
| Svelte | MIT |
| xterm.js (`@xterm/xterm`, `@xterm/addon-fit`) | MIT |
| marked | MIT |
| DOMPurify | MPL-2.0 or Apache-2.0 |

Development tools (Vite, Vitest, Playwright and others) are not shipped. Their licences are in `web/package-lock.json`.

## Go modules (compiled into the binary)

| Module | Licence |
|---|---|
| `github.com/BurntSushi/toml` | MIT |
| `github.com/aymanbagabas/go-pty` | MIT |
| `github.com/coder/websocket` | ISC |
| `github.com/creack/pty` | MIT |
| `github.com/u-root/u-root` | BSD-3-Clause |
| `golang.org/x/crypto`, `golang.org/x/sys` | BSD-3-Clause |

## Inspiration

Dungeon Master is an independent project. It is inspired by [chartr](https://github.com/rengwu/chartr) and builds on
[Matt Pocock's skills](https://github.com/mattpocock/skills) (MIT); no code or prose from either is included.
