# Intake: Cross-platform release pipeline

**Change**: 260503-dgq0-release-pipeline
**Created**: 2026-05-03
**Status**: Draft

## Origin

> Carved out from the parent change `260503-iu93-bootstrap-go-binary` (which builds the v0.0.1 Go binary in-session). The release pipeline was split out because it requires CI runs and `HOMEBREW_TAP_TOKEN` provisioning that can't happen inside a single conversational session — those need to happen between sessions, with the user provisioning the secret in GitHub repo settings.

This intake is **draft, not active**. It exists as the explicit follow-up to the binary-bootstrap change. The user explicitly asked for it ("you create another intake for the pipeline files").

## Why

### The problem

The bootstrap change ships a binary that compiles and runs locally, but it does not yet:
- Produce signed/checksummed cross-platform release artifacts
- Publish those artifacts to GitHub Releases on tag push
- Update `homebrew-tap` (the `sahil87/homebrew-tap` repo) so users can `brew install sahil87/tap/repo`

Without these, distribution is "build from source" only. That's fine for Sahil but blocks any onboarding flow for other users (which was a stated goal during the `/fab-discuss` session).

### The consequences if we don't act

- Every user must clone the source and run `just install` themselves.
- Version pinning is informal — there are no published artifacts at named versions.
- The `repos.yaml` starter that ships embedded in the binary references this repo (`git@github.com:sahil87/repo.git`), making "install this tool" a chicken-and-egg problem (need the tool to install the tool).

### Why this approach (goreleaser + GitHub Actions + homebrew-tap)

- **goreleaser** is the de-facto standard for Go release pipelines. ~30 lines of YAML versus ~200 lines of bash for hand-rolled releases.
- **GitHub Actions** runs on tag push, no manual steps. Single source of truth (the git tag) drives the release.
- **homebrew-tap** integration ships with goreleaser — one config block, no separate workflow needed.
- The constitution's `Cross-Platform Behavior` section (carried over from the parent change) lists the four target platforms (darwin-arm64, darwin-amd64, linux-arm64, linux-amd64). Goreleaser's matrix matches this exactly.

### Rejected alternatives

- **Hand-rolled `scripts/release.sh`**: Rejected. ~200 lines of bash to do what goreleaser does in ~30. No advantage.
- **Manual releases (build locally, upload to GitHub by hand)**: Rejected. Doesn't scale; introduces drift between what's tagged and what's published.
- **No homebrew-tap, just GitHub Releases**: Rejected. `brew install` is the canonical Mac install path; without the tap, Mac onboarding is "download a tar.gz and unpack it manually."

## What Changes

This change implements the release pipeline already designed in `docs/specs/build-and-release.md` (the "Cross-Platform Release Pipeline (deferred to follow-up change)" section and below). The spec is the authoritative source of truth for `.goreleaser.yaml` config, the GitHub Actions workflow, the homebrew-tap integration, and the setup checklist. This intake adds the implementation-specific scaffolding around it.

Outcome: a working release pipeline that publishes a v0.0.1 GitHub Release with 4 binaries (darwin-arm64, darwin-amd64, linux-arm64, linux-amd64) and updates `sahil87/homebrew-tap` so `brew install sahil87/tap/repo` succeeds.

### Files to create / modify

| File | Action | Source of truth |
|---|---|---|
| `.goreleaser.yaml` | Create | `docs/specs/build-and-release.md` § `.goreleaser.yaml (intent)` |
| `.github/workflows/release.yml` | Create | `docs/specs/build-and-release.md` § `.github/workflows/release.yml (intent)` |
| `scripts/release.sh` | Create | See "Release script behavior" below (not fully detailed in spec) |
| `justfile` | Modify — add `release` recipe | See "Justfile recipe" below |
| `README.md` | Modify — replace install section | `docs/specs/build-and-release.md` § Distribution Channels |

### Release script behavior (`scripts/release.sh`)

The spec mentions this script exists; the contract:

- Accept exactly one arg: the tag (e.g., `v0.0.1`).
- Validate format: `v[0-9]+\.[0-9]+\.[0-9]+` (or with optional `-rc\.[0-9]+` suffix if prerelease support is enabled — see assumption #14).
- Confirm working tree is clean: `git status --porcelain` returns nothing. Exit 1 with helpful message if not.
- Confirm we're on `main` (or warn if not). Soft check — allow `--force` to bypass.
- `git tag -a "$TAG" -m "Release $TAG"`.
- `git push origin "$TAG"`.
- Print: `Pushed $TAG. Watch the workflow at https://github.com/sahil87/repo/actions`.

### Justfile recipe

Append to the existing `justfile` (created in the parent change):

```just
release tag:
    ./scripts/release.sh {{tag}}
```

Existing recipes (`build`, `install`, `test`) stay unchanged.

### Apply-stage acceptance criteria

1. `cd src && go build ./...` still passes (no regression from parent change).
2. `goreleaser check` (run locally with `--config .goreleaser.yaml`) passes — config is well-formed.
3. `act -j release` (or equivalent local GitHub Actions runner) can dry-run the workflow at least up to the goreleaser step (it will fail at homebrew tap push without `HOMEBREW_TAP_TOKEN`, which is fine — that proves we got that far).
4. `scripts/release.sh` is executable and rejects malformed tags (`./scripts/release.sh foo` exits 1).
5. README's install section shows the brew install command first, "from source" second.

### Release-day runbook (out of code; user-driven)

After apply + review pass:

1. **Provision `HOMEBREW_TAP_TOKEN`** (per spec setup checklist).
2. **Verify `sahil87/homebrew-tap`** has a `Formula/` directory.
3. `just release v0.0.1` — pushes the tag.
4. Watch the workflow at `https://github.com/sahil87/repo/actions`.
5. Verify `https://github.com/sahil87/repo/releases/tag/v0.0.1` has 4 binaries + `checksums.txt`.
6. Verify `sahil87/homebrew-tap` got a new commit adding `Formula/repo.rb`.
7. Smoke test in a clean shell: `brew install sahil87/tap/repo && repo --version` prints `v0.0.1`.

Steps 1–2 cannot be done by the apply agent (they require GitHub UI and external repo state). Steps 3–7 happen *after* this change is merged — they are not part of apply.

## Affected Memory

- `build/release-pipeline` (new) — goreleaser config, GitHub Actions trigger, homebrew-tap integration

## Impact

**Files created**:
- `.goreleaser.yaml`
- `.github/workflows/release.yml`
- `scripts/release.sh`

**Files modified**:
- `justfile` — add `release` recipe
- `README.md` — install section update

**Out-of-repo state changes**:
- GitHub repo secret `HOMEBREW_TAP_TOKEN` provisioned by user
- First `v0.0.1` tag pushed, triggering first GitHub Release
- `sahil87/homebrew-tap` updated by goreleaser with `Formula/repo.rb`

**Dependencies on parent change** (`260503-iu93-bootstrap-go-binary`):
- The `src/` layout must exist (the goreleaser `main` field points at `./src/cmd/repo`).
- The `justfile` must already exist (this change *adds* to it, doesn't create).
- The `scripts/` directory must already exist.
- The README must already have placeholder content this change replaces.

This change MUST NOT be applied until the parent change is merged.

## Open Questions

All open items are tracked in the Assumptions table below as Confident or Tentative rows. No questions blocking this intake.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Release tooling is goreleaser (not hand-rolled bash) | Inherited from parent change's discussion — user explicitly chose goreleaser | S:100 R:80 A:95 D:95 |
| 2 | Certain | Build matrix: darwin-arm64, darwin-amd64, linux-arm64, linux-amd64 (no Windows) | Constitution Cross-Platform Behavior section explicitly lists these four | S:100 R:90 A:100 D:100 |
| 3 | Certain | Homebrew tap target: `sahil87/homebrew-tap` | Already in user's repos.yaml (`~/code/bootstrap/dotfiles/repos.yaml` line 11) | S:100 R:75 A:95 D:100 |
| 4 | Certain | First release version: v0.0.1 | Inherited from parent change — user explicitly chose 0.0.1 | S:100 R:90 A:95 D:100 |
| 5 | Certain | CI trigger is git tag push matching `v*` | Standard goreleaser pattern, matches Constitution Principle V (release via tag) | S:95 R:80 A:95 D:95 |
| 6 | Certain | `HOMEBREW_TAP_TOKEN` is a manual setup step, not automated | GitHub PATs cannot be provisioned via code | S:100 R:60 A:100 D:100 |
| 7 | Confident | Archive format: tar.gz with LICENSE + README included | Goreleaser default, matches typical Go binary releases | S:80 R:80 A:90 D:80 |
| 8 | Confident | Checksum format: SHA256, single `checksums.txt` | Goreleaser default | S:80 R:85 A:90 D:80 |
| 9 | Confident | `goreleaser-action` is pinned to `v6` (current stable major as of 2026-05) | Stability over auto-update; matches typical CI pinning practice | S:75 R:75 A:80 D:75 |
| 10 | Confident | Workflow uses `actions/setup-go@v5` with `go-version-file: src/go.mod` | Standard pattern; reads version from go.mod so it's always in sync | S:85 R:85 A:90 D:85 |
| 11 | Confident | Local `scripts/release.sh` validates tag format and clean working tree before pushing | Standard release-script pattern; prevents accidental dirty releases | S:80 R:80 A:90 D:80 |
| 12 | Tentative | macOS code signing / notarization is deferred (Gatekeeper warning is acceptable for v0.0.1) | User did not explicitly resolve; signing certs cost money and require Apple Developer account | S:50 R:65 A:60 D:55 |
| 13 | Tentative | Changelog for v0.0.1 is hand-written (override goreleaser default) because there's no prior tag and commit history is noisy | User did not explicitly resolve; default would produce a long ugly changelog | S:55 R:85 A:70 D:60 |
| 14 | Tentative | No prerelease support (no `v0.0.1-rc.1` testing) for the first release | User did not explicitly resolve; adds complexity for marginal benefit | S:50 R:75 A:65 D:55 |
| 15 | Tentative | Linux release artifacts are tar.gz only (no .deb/.rpm) | User did not explicitly resolve; tar.gz suffices for brew-on-Linux and direct download | S:50 R:80 A:75 D:60 |

15 assumptions (6 certain, 5 confident, 4 tentative, 0 unresolved).
