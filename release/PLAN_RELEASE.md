---
status: partial
milestone: M8 (R0 to R6)
github_issues: [29, 30, 31, 35, 36, 37, 38, 39]
tracking: in-gh-issues
next: phases R0 to R6 are issues; delete when the first release ships
verified: 2026-10-11
---

# Release plan

How Dungeon Master is versioned, built, packaged, verified and distributed. Everything release-related lives in this
directory; the only exceptions are files GitHub forces elsewhere (`.github/workflows/`) and thin `Makefile` entry points
that delegate here.

Aligns with milestone M8 in `../docs/architecture.md`, but the first pieces land much earlier
(see §10).

## 1. Goals

1. Anyone can download one file for their OS and run Dungeon Master.
2. A release is reproducible from a tag, by CI, with no manual steps on a laptop.
3. A broken release step is found by a dry run, not by the real tag.
4. Trust is added in order of payoff: integrity first, then signing, then convenience.

Non-goals for now: `go install` (the embedded frontend makes it unusable), auto-updating, Windows native shell, Intel
macOS.

## 2. Artifacts

| Artifact                                                                                        | Built by                     | Where               | Needs cgo   | Phase |
|-------------------------------------------------------------------------------------------------|------------------------------|---------------------|-------------|-------|
| Portable binary (`dungeon serve`, browser mode), darwin/linux/windows × amd64/arm64, tar.gz/zip | GoReleaser, one Linux job    | ubuntu              | no          | R0    |
| `checksums.txt`                                                                                 | GoReleaser                   | ubuntu              | no          | R0    |
| macOS app: `Dungeon Master.app` in a `.dmg` (arm64) + `.sha256`                                 | script in `release/macos/`   | macos arm64 runner  | yes         | R1    |
| Linux AppImage (amd64, arm64)                                                                   | script in `release/linux/`   | ubuntu + ubuntu-arm | yes         | R2    |
| `.deb` / `.rpm`                                                                                 | nfpm                         | ubuntu              | yes (shell) | later |
| Windows shell (WebView2)                                                                        | script in `release/windows/` | windows             | yes         | later |
| Build provenance attestations for all of the above                                              | GitHub Actions               | any                 | no          | R3    |
| Homebrew cask                                                                                   | GoReleaser                   | ubuntu              | no          | R4    |

The browser-mode portable binary is always the fallback: a broken native shell must never block a release.

## 3. Versioning, changelog, tags (automated)

Tool: **release-please** (manifest mode, `release-type: go`). Config lives in `release/release-please-config.json` and
`release/.release-please-manifest.json`; the workflow points at them with `config-file` / `manifest-file`.

How it works:
1. Commits land on `master` as Conventional Commits.
2. On every push, release-please opens or updates one **release PR**: it computes the next version from the commits,
   rewrites `CHANGELOG.md`, and bumps the manifest.
3. A human reviews and merges that PR. This is the release gate.
4. On merge, release-please creates the tag `vX.Y.Z` and a **draft** GitHub release whose notes are the changelog entry.
5. The build jobs (§4) run in the same workflow run, upload assets to the draft, and the final job publishes it.

Version rules (pre-1.0, `bump-minor-pre-major: true`):

| Commit                                    | Bump                                                |
|-------------------------------------------|-----------------------------------------------------|
| `fix`, `perf`                             | patch                                               |
| `feat`                                    | minor                                               |
| `feat!` / `BREAKING CHANGE:`              | minor while `0.x`, major after `1.0.0`              |
| `docs`, `chore`, `ci`, `test`, `refactor` | no release by themselves; hidden from the changelog |
| `build`                                   | shown in the changelog under "Build and packaging"  |

- The manifest starts at `0.0.0`, so the first `feat` proposes `0.1.0`. To force a version (for example `1.0.0` later),
  put a `Release-As: 1.0.0` footer in a commit.
- Never edit `CHANGELOG.md`, the manifest or a version by hand; the release PR owns them. Fix a bad changelog line by
  editing the commit message or the release PR before merge.
- Version, commit and date are stamped into both binaries with `-ldflags -X` from the tag.
- Pipeline testing uses the dry-run workflow (§10). release-please has prerelease options (`prerelease`,
  `prerelease-type`); verify them before relying on an `-rc.N` flow.

Commit hygiene this depends on:
- Merge PRs with **squash**, using the PR title as the commit subject, so the title must be a Conventional Commit.
  Enforce it with a PR-title check in CI.
- One logical change per PR keeps changelog entries meaningful.

### Why not the alternatives

| Option                             | Verdict                                                                                                                                           |
|------------------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------|
| **release-please**                 | Chosen. No Node toolchain, the release PR is a human gate, writes `CHANGELOG.md`, supports draft releases and Go.                                 |
| semantic-release                   | Node-based plugin chain; releases straight from a push with no review gate. Works, but heavier and less control for a Go repo.                    |
| Manual tags + GoReleaser changelog | Simplest: you pick the version and push the tag, GoReleaser writes the notes from commits. No release PR and no changelog file. A valid fallback. |
| `svu` + GoReleaser                 | Same, but the next tag is computed from commits. Good fallback if release-please becomes a burden.                                                |
| git-cliff                          | Changelog generator only; still needs something to decide versions and tag.                                                                       |

### Known sharp edges (checked against the release-please docs)

- With `draft: true`, GitHub does not create the tag until the release is published, which makes the next run lose track
  of the previous release and produce a changelog of the whole history. `force-tag-creation: true` (set in our config)
  creates the tag immediately and avoids this.
- Anything release-please creates with the default `GITHUB_TOKEN` (the PR, the tag, the release) does **not** trigger
  other workflows. Consequences: (1) our build jobs must live in the same workflow as release-please and run off its
  outputs (`release_created`, `tag_name`), not on a `tag`/`release` trigger; (2) CI will not run on the release PR
  unless the action uses a PAT or GitHub App token (`RELEASE_PLEASE_TOKEN`). Start with `GITHUB_TOKEN`; switch only if
  branch protection requires checks on the release PR.
- GoReleaser must not also create the release or write its own changelog: use it to build archives and checksums only
  (`--skip=publish`, `changelog.disable: true`), and upload to the existing draft with `gh release upload`.
- Config paths: `packages` paths and `changelog-path` are relative to the repo root even though the config file sits in
  `release/`. Confirm on the first dry run.

## 4. Pipeline

```
push to master
  └─ job: release-please   maintains the release PR; when that PR is merged:
       tag vX.Y.Z + DRAFT release (notes from CHANGELOG.md), outputs release_created/tag_name
       ├─ job: portable   (ubuntu)   GoReleaser (build only) → archives + checksums.txt → gh release upload
       ├─ job: macos      (macos)    web → shell → .app → ad-hoc/Developer-ID sign → .dmg → smoke
       ├─ job: linux      (matrix)   web → shell → AppImage → smoke
       └─ job: windows    (windows)  ConPTY smoke (and shell later)
            │ all required jobs upload to the draft
            ▼
       job: attest + publish   verify the asset set → attest provenance → publish draft
```

Rules:
- All jobs live in one workflow (`release.yml`) and run only when `release_created` is true; see §3 for why they cannot
  trigger off the tag.
- Jobs upload to the **draft**; only the final job publishes, and only if every required job succeeded.
- Each job rebuilds the frontend (`make web`) from the same commit; no cross-job build artifacts besides uploads.
- Best-effort jobs (Windows shell) may fail without blocking; required ones may not.
- The final job checks the expected asset names exist before publishing, so a silently skipped job cannot ship a partial
  release.

## 5. Directory layout (target)

```
release/
  README.md              index of this directory
  PLAN_RELEASE.md        this file
  release-please-config.json     versioning/changelog rules (present)
  .release-please-manifest.json  current released version (present, owned by release-please)
  goreleaser.yaml        portable binaries, archives, checksums, (cask); no changelog, no publish
  macos/
    build-app.sh         shell binary → Dungeon Master.app (icon, Info.plist, signing)
    Info.plist.tmpl
    make-dmg.sh          app + /Applications link + README → .dmg + .sha256
    dmg-README.txt       first-launch (Gatekeeper) instructions
    icons/               source artwork, iconset script
  linux/
    build-appimage.sh
    nfpm.yaml
    dungeon.desktop
    dungeon.metainfo.xml
  windows/               (later)
  scripts/
    stamp-version.sh     shared version/commit/date resolution
    verify-assets.sh     asset-set check used by the publish job
    smoke-*.sh           launch the built artifact, hit /api/health
```

Not here, by necessity: `.github/workflows/release.yml` and `ci.yml`. They stay thin: set up toolchains, then call
scripts in `release/` or `make` targets, so a release step can be run locally.

The root `Makefile` keeps only entry points (`make snapshot`, `make dmg`, `make appimage`) that delegate to `release/`.

## 6. macOS

Build steps (script, on a macOS arm64 runner):
1. Build the shell: `CGO_ENABLED=1 go build -tags webview -trimpath -ldflags "..."`.
2. Assemble `Dungeon Master.app/Contents/{MacOS,Resources,Info.plist}`; the binary is `MacOS/dungeon`.
3. Icon: render the source PNG to the required sizes with `sips`, bundle with `iconutil -c icns`.
4. `Info.plist`: bundle id, display name, short version (strip pre-release suffix) and full version, minimum macOS read
   from the binary (`otool -l` → `minos`), `NSHighResolutionCapable`, category.
5. Sign (see ladder below) and `codesign --verify --strict`.
6. DMG: stage the app, an `/Applications` symlink and a plain-text README, `hdiutil create -format UDZO`,
   `hdiutil verify`, write `.sha256`. No styled window.
7. Smoke: mount or unpack, launch, wait for the health endpoint, quit.

Trust ladder:

| Stage | Signing                                                        | User experience                                                | When |
|-------|----------------------------------------------------------------|----------------------------------------------------------------|------|
| 1     | ad-hoc (`codesign --sign -`)                                   | Gatekeeper blocks first launch; Open Anyway in System Settings | R1   |
| 2     | Developer ID Application cert, hardened runtime, `--timestamp` | still needs notarization to be quiet                           | R5   |
| 3     | + `notarytool submit --wait` + `stapler staple`                | opens normally, works offline                                  | R5   |

Architecture: arm64 only at first. The architecture is in the file name. Universal or Intel needs two cgo builds joined
with `lipo`; decide later.

## 7. Linux and Windows

- **AppImage** (bundles WebKitGTK): build on the oldest reasonable Ubuntu LTS runner for glibc compatibility; arm64 on
  an arm runner. Smoke on a bare container with no WebKit installed.
- **deb/rpm** via nfpm depend on the distro's WebKitGTK; smoke on Ubuntu LTS and Fedora containers.
- **Windows minimum: Windows 10 1803.** Unix domain sockets (the Gatekeeper) and ConPTY (terminals) both arrive in that
  version; every feature must work there, so each milestone that touches processes, sockets or paths adds a Windows
  check. Open items: the `gh` shim as a copy of the binary or a `.cmd` (symlinks need Developer Mode), the socket
  directory under the user's temp directory, and no file modes.
- **Windows:** ship the portable binary first. A ConPTY round-trip smoke test runs on `windows-latest` from the start,
  because PTY behaviour differs most there. The WebView2 shell comes later.

## 8. Supply chain and integrity

From R0: `checksums.txt`; workflows declare least-privilege `permissions` per job; `govulncheck` in CI.
From R3: GitHub artifact attestations (build provenance) for every released asset; third-party actions pinned to commit
SHAs (kept updated by Dependabot); optional SBOM.
Never: telemetry, hosted services, or user accounts; the README promise stays true.

Secrets (added only when needed): `RELEASE_PLEASE_TOKEN` (PAT or GitHub App token, only if CI must run on the release
PR), Developer ID certificate (base64) + password, notarization credentials (API key), tap repo token. None are needed
before R5/R4.

## 9. Distribution

1. GitHub Releases (always).
2. Homebrew cask in a personal tap, generated by GoReleaser (R4): `brew install --cask`.
3. In-app "new version available" notice reading the GitHub releases API, notify-only. Auto-install only after Developer
   ID signing exists.
4. Later, on demand: AUR, winget, Linux repos.

## 10. Dry runs and testing

- `goreleaser release --snapshot --clean --skip=publish` locally and in CI on every PR that touches `release/` or
  workflows.
- `release.yml` has `workflow_dispatch` with a `dry_run` input: runs all build jobs, uploads artifacts to the run,
  creates no release.
- The first real release goes through at least one dry run on a branch or fork.
- Every artifact has a smoke test that runs the built file and checks `/api/health` with the token flow.
- `verify-assets.sh` runs in the publish job and in dry runs.
- Walking skeleton: build the pipeline against the current placeholder so every later milestone ships through a pipeline
  that already works.

## 11. Phases

| Phase | Contents                                                                                                                                                                                                                  | Land alongside          |
|-------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|-------------------------|
| R0    | `release.yml` with the release-please job, PR-title check, `release/goreleaser.yaml` (build only), snapshot in CI, portable archives + checksums uploaded to the draft, `govulncheck`, version stamping, dry-run workflow | M2                      |
| R1    | shell binary (`cmd/shell`, build tag `webview`), macOS `.app`/`.dmg` scripts, ad-hoc signing, smoke, draft→publish job                                                                                                    | M2–M3                   |
| R2    | Linux AppImage + smoke, Windows ConPTY smoke                                                                                                                                                                              | M4                      |
| R3    | attestations, SHA-pinned actions, Dependabot                                                                                                                                                                              | before first public tag |
| R4    | Homebrew cask tap                                                                                                                                                                                                         | after first public tag  |
| R5    | Developer ID signing + notarization, then optional update notice                                                                                                                                                          | when others use the app |
| R6    | deb/rpm, Windows shell, Intel/universal macOS                                                                                                                                                                             | on demand               |

First public release (`v0.1.0`, via the first release PR) needs R0–R3 and a clean dry run.

## 12. Open questions

1. Bundle identifier and final product/app name (affects the plist, desktop file and tap name).
2. GitHub owner for the repo and the tap, and the module path.
3. Do we have, or want, an Apple Developer account (and when)?
4. Is Intel macOS needed by anyone who will actually use it?
5. App icon source art (needs creating; avoid reusing anyone else's assets).
6. Exact flags and tooling versions to confirm when R0 starts (GoReleaser config version, notarytool options,
   `actions/attest-build-provenance` inputs).
