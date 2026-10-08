# Dummy plan: export quests to CSV

Status: draft, nothing built. This file only exists to check the markdown formatter, so the content is made up on
purpose, but the shape is what an agent usually writes: long single-line paragraphs, crude tables and nested lists.

## 1. Goal

Let the Dungeon Master export the quests of a campaign as a CSV file so that a human can open it in a spreadsheet, sort
it, and share it with someone who does not use the app. The export must never include tokens, local paths or anything
from `party.toml`, and it must respect the current scope filter (`tracked`).

## 2. Columns

| Column     | Type      | Notes                                                             |
|------------|-----------|-------------------------------------------------------------------|
| number     | int       | GitHub issue number                                               |
| title      | string    | Plain text, markdown stripped, quotes escaped as `""`             |
| state      | enum      | open, in progress, locked, cleared (derived, see `GLOSSARY.md`)   |
| labels     | string    | Joined with `;`                                                   |
| updated    | timestamp | RFC 3339, UTC                                                     |
| blocked by | string    | Issue numbers joined with `;`, empty when the quest is not locked |

## 3. Steps

1. Add `internal/quests/export.go` with `WriteCSV(w io.Writer, quests []Quest) error`, which streams rows so a
   repository with thousands of issues does not need to be held in memory twice, and which guards against spreadsheet
   formula injection by prefixing cells that start with `=`, `+`, `-` or `@` with a single quote.
2. Add `GET /api/campaigns/{id}/quests.csv` behind the existing guard (token, Host and Origin checks), with
   `Content-Disposition: attachment` and a neutral file name that does not contain the theme words.
   - Unit tests for quoting, newlines inside titles, and the formula-injection guard.
   - Feature test with `httptest` that calls the endpoint with and without the access token and checks the 401.
3. Add an "Export" button to the quests panel header; it only downloads, it never writes to GitHub.
10. Document the endpoint in `PLAN.md` section 8 after M4 lands, not before, because the quest model may still change
    while the briefing work is under way.

## 4. Risks

> Issue text is untrusted. A title such as `=HYPERLINK("http://evil","click")` must come out as plain text in the
> spreadsheet, which is why the guard in step 1 is a requirement and not a nicety, and why it has its own test.

- Large repositories: the export streams, but the quest list itself comes from the cache, so an export during a
  rate-limit pause reflects the stale copy and should say so in the file name or a header comment row.
