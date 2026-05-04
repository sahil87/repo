# Build and Release

> Build system, release pipeline, and distribution for the `repo` binary.
>
> **Scope note**: The release pipeline mirrors `~/code/sahil87/run-kit`'s hand-rolled GitHub Actions workflow shape (cross-compile loop, `softprops/action-gh-release`, formula template + `sed` for the Homebrew tap update). It diverges from run-kit on one point: the **git tag is the version source of truth** (no `VERSION` file), because `repo` is a single-binary repo. Local builds use `git describe --tags --always`; release builds read the pushed tag.

## Justfile

Per Constitution Principle V ("Thin Justfile, Fab-Kit Build Pattern") — recipes are one-liners; logic lives in `scripts/`.

```just
default:
    @just --list

build:
    ./scripts/build.sh

install:
    ./scripts/install.sh

test:
    cd src && go test ./...

release bump="patch":
    ./scripts/release.sh {{bump}}
```

## Local Build Scripts

### `scripts/build.sh`

```bash
#!/usr/bin/env bash
set -euo pipefail

VERSION="$(git describe --tags --always 2>/dev/null || echo dev)"
mkdir -p bin
cd src
go build -ldflags "-X main.version=${VERSION}" -o ../bin/repo ./cmd/repo
echo "built: bin/repo (version: ${VERSION})"
```

- Injects the version via `-ldflags "-X main.version=..."`. Pre-tag, `git describe --tags --always` returns a short SHA (e.g., `a08147d`); on a tagged commit, it returns `v0.0.1`; post-tag, `v0.0.1-2-g<sha>` for commits past the tag.
- Output: `./bin/repo` at the repo root. `bin/` is gitignored.
- Used for local development.

### `scripts/install.sh`

```bash
#!/usr/bin/env bash
set -euo pipefail

./scripts/build.sh

DEST="${HOME}/.local/bin/repo"
mkdir -p "$(dirname "$DEST")"
cp -f ./bin/repo "$DEST"
echo "installed: $DEST"
```

- Calls `build.sh` first.
- Copies to `~/.local/bin/repo`. The user is responsible for ensuring `~/.local/bin` is on `$PATH`.
- Idempotent — re-running overwrites the previous install.

### `scripts/release.sh`

Tag-driven: computes the next semantic version from `git describe --tags --abbrev=0` (with a `v0.0.0` fallback for tagless repos), creates the tag locally, and pushes it to `origin`. The script does NOT modify any tracked files — there is no `VERSION` file to bump and no commit to make.

- Accepts exactly one argument: `patch`, `minor`, or `major`.
- Pre-flight: clean working tree (`git status --porcelain` empty) and on a branch (not detached HEAD). Exits 1 if either check fails.
- Bump arithmetic: parses the current `major.minor.patch` (after stripping any leading `v`) and increments per the bump type.
- Bare invocation (no args) prints usage and exits 0 (informational). Unknown args or multiple bump-type args print an error and exit 1.

Usage: `just release patch` (or `just release` — `bump` defaults to `patch`).

## Version Reporting

| Form | Behavior |
|---|---|
| `repo --version` | Prints version string, exit 0 |
| `repo -v` | Same as `--version` |

Only `repo --version` and `repo -v` are public version-printing surfaces. The cobra-default `version` subcommand is shadowed by the parent change's positional `repo <name>` handler — invoking `repo version` triggers an fzf lookup for a repo named "version", not a version print.

The version string format depends on build context:
- Tagged release: `v0.0.1`
- Post-tag commit: `v0.0.1-2-gabc123`
- Pre-tag dev build: `<short-sha>`
- No git history (e.g., source tarball): `dev`
- Built without ldflags (e.g., `go install ...`): `dev` (the `var version = "dev"` default in `src/cmd/repo/main.go`)

## Cross-Platform Release Pipeline

The pipeline is hand-rolled. It mirrors `~/code/sahil87/run-kit`'s release workflow shape and adapts where the two repos differ (single binary, tag-driven version source). See Design Decision #1 for the rationale and the alternative considered.

### Architecture

```
just release patch  →  scripts/release.sh
                    →  git tag v0.0.1 + git push origin v0.0.1
                    →  GitHub Actions (.github/workflows/release.yml)
                    →  cross-compile loop (4 GOOS/GOARCH targets)
                    →  GitHub Release (4 tar.gz archives)
                    +  homebrew-tap update (Formula/repo.rb via sed-substituted formula-template.rb)
```

Single trigger (tag push), single source of truth (the git tag), one mental model.

### Workflow shape (`.github/workflows/release.yml`)

Hand-rolled steps (mirrors `~/code/sahil87/run-kit/.github/workflows/release.yml`, minus the frontend toolchain and tmux-config steps that are run-kit-specific):

1. **Checkout** with `fetch-depth: 0` (needed for the previous-tag-base computation).
2. **Set up Go** with `go-version-file: src/go.mod`.
3. **Extract version from tag**: `tag="${GITHUB_REF#refs/tags/}"` (with `v` prefix; used for ldflags) and `version="${tag#v}"` (without `v`; used for sed-substituting the formula template).
4. **Cross-compile loop** — for each of `darwin/arm64`, `darwin/amd64`, `linux/arm64`, `linux/amd64`: `CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build -ldflags "-X main.version=$tag" -o dist/repo-$os-$arch/repo ./src/cmd/repo`, then `tar -czf dist/repo-$os-$arch.tar.gz -C dist/repo-$os-$arch repo`.
5. **Determine release notes base tag** (run-kit's logic, verbatim): if the current tag's patch component is `0` (a minor bump), find the earliest tag matching `v{major}.{minor-1}.*` via `git tag -l "${prev_prefix}*" --sort=version:refname | head -1`. Otherwise, leave the base tag empty (default GitHub behavior: compare to immediate previous tag).
6. **Create GitHub Release** via `softprops/action-gh-release@v2` with `files: dist/*.tar.gz`, `generate_release_notes: true`, `previous_tag: ${{ steps.release-base.outputs.base_tag }}`.
7. **Update Homebrew tap**: compute the four SHA256s via `sha256sum`, clone `sahil87/homebrew-tap` using `HOMEBREW_TAP_TOKEN`, `sed` `.github/formula-template.rb` to substitute `VERSION_PLACEHOLDER`, `SHA_DARWIN_ARM64`, `SHA_DARWIN_AMD64`, `SHA_LINUX_ARM64`, `SHA_LINUX_AMD64`, write to `Formula/repo.rb`, commit as `github-actions[bot]` with message `repo v<version>`, push to the tap's default branch.

All third-party action references SHALL be pinned to commit SHAs (full 40-character SHAs) with `# v<N>` comments. The SHAs match those used in run-kit's workflow at apply time.

### Formula template (`.github/formula-template.rb`)

A Ruby formula skeleton with placeholders that the workflow substitutes at release time. Structure mirrors run-kit's template:

- `class Repo < Formula` opener.
- `desc "Locate, open, list, and clone repos from repos.yaml"`.
- `homepage "https://github.com/sahil87/repo"`.
- `version "VERSION_PLACEHOLDER"` — substituted with the bare version (no `v` prefix). The URLs re-add `v` via `releases/download/v#{version}/...`.
- `license "MIT"` (informational only; brew does not enforce).
- `on_macos` block with `on_arm`/`on_intel` sub-blocks; each declares a `url` (`releases/download/v#{version}/repo-darwin-{arm64,amd64}.tar.gz`) and a `sha256` (placeholder `SHA_DARWIN_{ARM64,AMD64}`).
- `on_linux` block with the same structure for `linux-{arm64,amd64}`.
- `install` block: `bin.install "repo"`.
- `test` block: `assert_match version.to_s, shell_output("#{bin}/repo --version")`.

### Workflow trigger

```yaml
on:
  push:
    tags:
      - "v*"

permissions:
  contents: write
```

No `workflow_dispatch`, no `pull_request`, no schedule, no branch pushes. Tag pushes only.

### Setup Checklist (manual, pre-first-release)

These are out-of-band steps the user MUST complete before pushing the first `v*` tag:

1. **Provision `HOMEBREW_TAP_TOKEN`**:
   - Create a fine-grained GitHub PAT with `Contents: write` on `sahil87/homebrew-tap`.
   - Add as a repo secret in `sahil87/repo` settings → Secrets and variables → Actions.
2. **Verify `sahil87/homebrew-tap`** exists with a `Formula/` directory. (Already exists for `Formula/rk.rb`; should be fine.)
3. **First release smoke test**: `just release patch`, watch the workflow, verify the GitHub Release has 4 tar.gz binaries, verify `Formula/repo.rb` lands in the tap, verify `brew install sahil87/tap/repo` succeeds in a clean shell.

### Build Matrix

| OS | Arch | Status |
|---|---|---|
| darwin | arm64 | Supported |
| darwin | amd64 | Supported |
| linux | arm64 | Supported |
| linux | amd64 | Supported |
| windows | * | NOT supported (Constitution Cross-Platform Behavior) |

### Initial Release: v0.0.1

- The first tag is `v0.0.1`.
- On a tagless repo, `git describe --tags --abbrev=0` exits non-zero; `scripts/release.sh` falls back to `v0.0.0` as the synthetic baseline, so `just release patch` produces `v0.0.1`.
- Reasoning: the Go binary is unproven on day one. `0.x.y` signals pre-stability; reserve `1.0.0` for "has run in production for ~2 weeks without friction."
- Once the binary has replaced the bash script in daily use without issues, cut `1.0.0`.

## Distribution Channels

After the release pipeline is live:

| Channel | Install command |
|---|---|
| Homebrew (macOS, Linux) | `brew install sahil87/tap/repo` |
| GitHub Release tarball | Download `repo-{os}-{arch}.tar.gz` from `https://github.com/sahil87/repo/releases/latest`, extract, place on `$PATH` |
| From source | `git clone …; cd repo; just install` |

## Behavioral Scenarios (GIVEN/WHEN/THEN)

### Local build

> **GIVEN** a clean checkout
> **WHEN** I run `just build`
> **THEN** `./bin/repo` exists
> **AND** `./bin/repo --version` prints a non-empty string

> **GIVEN** a clean checkout
> **WHEN** I run `just install`
> **THEN** `~/.local/bin/repo` exists
> **AND** is executable
> **AND** is byte-identical to `./bin/repo`

> **GIVEN** a clean checkout
> **WHEN** I run `just test`
> **THEN** all tests in `src/...` pass
> **AND** exit code is 0

### Release pipeline

> **GIVEN** all setup-checklist items complete and a clean working tree at commit `<sha>`
> **WHEN** I run `just release patch`
> **THEN** `scripts/release.sh` creates tag `v0.0.1` and pushes it to `origin`
> **AND** the `Release` workflow runs to completion
> **AND** `https://github.com/sahil87/repo/releases/tag/v0.0.1` shows 4 tar.gz archives
> **AND** `sahil87/homebrew-tap` has a new commit adding/updating `Formula/repo.rb`

> **GIVEN** the v0.0.1 release is published and `Formula/repo.rb` is in the tap
> **WHEN** a fresh user runs `brew install sahil87/tap/repo`
> **THEN** the install succeeds
> **AND** `repo --version` prints `repo version v0.0.1`

## Design Decisions

1. **Hand-rolled workflow mirroring run-kit, with tag-driven version source.** Mirror `~/code/sahil87/run-kit`'s release workflow shape (cross-compile loop, `softprops/action-gh-release`, hand-rolled tap update via formula template + `sed`). Diverge on one point: the git tag is the version source of truth (no `VERSION` file), because `repo` is single-binary and run-kit's `VERSION`-file rationale (multi-binary monorepo: frontend + backend disambiguation) doesn't apply here. *Rejected*: goreleaser. Smaller config (~30 LOC vs. ~80) and free Homebrew tap update via `brews:` are real advantages, but the **minor-aware base-tag logic for release notes is awkward in goreleaser** (requires disabling its changelog and using post-hoc `gh release edit` via `gh api`). The hand-rolled pattern handles it natively via `softprops/action-gh-release`'s `previous_tag` parameter. Combined with the consistency win of mirroring run-kit (same author, same tap, same target platforms), goreleaser's leverage doesn't compound for a single-binary CLI. Goreleaser pays back if/when this repo grows multiple binaries or wants signing/Docker/Snap; switching is a one-evening rewrite then.
2. **Tag push is the single trigger.** No manual workflow dispatch, no GUI clicks. `git push origin <tag>` (via `just release patch`) is the entire release-day action.
3. **Version injection via ldflags, with two source paths.** Local builds use `git describe --tags --always` (already in `scripts/build.sh`); release builds use `${GITHUB_REF#refs/tags/}` from the pushed tag. Both inject as `-ldflags "-X main.version=<string>"` retaining the leading `v` prefix. In the released case, both paths produce the same string (e.g., `v0.0.1`), so `repo --version` output is consistent across local-tagged and CI-released binaries.
4. **`v0.0.1` initial version.** Pre-stability signal. Bumps to 1.0 once daily-driven without friction.
5. **`scripts/release.sh` does not commit.** It only creates and pushes a tag. Tags are stable git refs; no commit churn means the release script is idempotent on the file tree. If the tag push fails, nothing local needs reverting.
6. **No Windows support.** Cross-platform code uses build tags for darwin/linux only. Adding Windows would require a `platform/open_windows.go` and a different shell integration story; out of scope for now.
7. **No code signing / notarization — not in scope.** Binaries ship unsigned. macOS users see a Gatekeeper warning on first run; brew installs typically don't trip this as hard as direct downloads. Apple Developer accounts cost $99/year — the marginal UX win doesn't justify recurring cost for a personal-tooling CLI. Not deferred — explicitly out of scope until a third party demands it.
8. **No prerelease tag support.** `release.sh` accepts only `patch|minor|major`. The workflow does not handle `-rc.N` tags specially.
9. **Action SHAs pinned, mirroring run-kit's exact SHAs.** Supply-chain hardening; also keeps both repos updateable via a single-source diff if action versions ever bump.
