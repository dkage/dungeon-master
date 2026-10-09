# Parchment font comparison

Open `index.html` in a browser. It compares the v4 Stitch parchment treatment
with Georgia, EB Garamond, Crimson Pro, Cormorant, Alegreya, Cardo and Almendra
at 12.5, 14 and 16 px. Each card uses the same Brazilian Portuguese sample.
Select a card and
use **Focus on selected** to see it at a width closer to the full panel.

The study compares candidates for narrative and quest briefing surfaces. The
current product decision is to offer Almendra, EB Garamond, and Alegreya as
user-selectable serif families. Cinzel headings and Fira Code snippets stay
constant in this comparison. The live PTY terminal keeps its monospaced Nerd
Font stack; the selectable serif choice does not change it.

The parchment background comes from `../stitch-prototype/assets/images/97785ef87771.png`.
All bundled candidates declare Latin Extended coverage in Google Fonts
metadata. Their font character maps include the displayed Portuguese accented
vowels in upper and lower case, as well as `ç` and `Ç`. Cardo also provides
specialist scholarly and medieval glyph coverage.
The font binaries and matching SIL Open Font License texts came from the
[Google Fonts source repository](https://github.com/google/fonts/tree/main/ofl)
on 2026-10-09. Each family has its own `*-OFL.txt` in `fonts/`.

Georgia is a system reference only and is not bundled. This remains a
reference study, not production UI. Copy only the selected font assets and
their matching license texts into the app when implementing the settings.
