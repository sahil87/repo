# Build and Release

> Build system, release pipeline, and distribution for the `repo` binary.
>
> **Scope note**: For v0.0.1, only the local build path (`just build`, `just install`) is implemented. The cross-platform release pipeline (`.goreleaser.yaml`, GitHub Actions, homebrew-tap) is captured here as the design intent and implemented in a follow-up change (`260503-dgq0-release-pipeline`). This spec is the authoritative input for that change.

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
```

The `release` recipe is added by the follow-up release-pipeline change. It is **not** part of v0.0.1.

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

- Injects the version via `-ldflags "-X main.version=..."`. Pre-tag, `git describe --tags --always` returns a short SHA (e.g., `a08147d`); post-tag, it returns `v0.0.1` or `v0.0.1-2-g<sha>` for commits past the tag.
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

## Version Reporting

| Form | Behavior |
|---|---|
| `repo --version` | Prints version string, exit 0 |
| `repo -v` | Same as `--version` |
| `repo version` | Cobra-default subcommand. May exist (no effort to suppress). Same output as the flag. |

The version string format depends on build context:
- Tagged release: `v0.0.1`
- Post-tag commit: `v0.0.1-2-gabc123`
- Pre-tag dev build: `<short-sha>`
- No git history (e.g., source tarball): `dev`

## Cross-Platform Release Pipeline (deferred to follow-up change)

The pipeline below is the **design intent**. Implementation lands in `260503-dgq0-release-pipeline`.

### Architecture

```
git tag v* push  →  GitHub Actions (.github/workflows/release.yml)
                 →  goreleaser
                 →  GitHub Release (4 binaries + checksums)
                 +  homebrew-tap update (Formula/repo.rb)
```

Single trigger (tag push), single tool (goreleaser), single source of truth (the git tag).

### `.goreleaser.yaml` (intent)

```yaml
version: 2

project_name: repo

before:
  hooks:
    - cd src && go mod tidy

builds:
  - id: repo
    main: ./src/cmd/repo
    binary: repo
    env:
      - CGO_ENABLED=0
    ldflags:
      - -s -w -X main.version={{.Version}}
    goos: [darwin, linux]
    goarch: [amd64, arm64]

archives:
  - format: tar.gz
    name_template: "repo_{{.Version}}_{{.Os}}_{{.Arch}}"
    files:
      - LICENSE
      - README.md

checksum:
  name_template: checksums.txt

brews:
  - repository:
      owner: sahil87
      name: homebrew-tap
      token: "{{ .Env.HOMEBREW_TAP_TOKEN }}"
    folder: Formula
    description: "CLI for quick repo operations — locate, open, list, and clone repos from repos.yaml"
    homepage: "https://github.com/sahil87/repo"
    test: |
      system "#{bin}/repo", "--version"
    install: |
      bin.install "repo"

release:
  github:
    owner: sahil87
    name: repo
```

The `before.hooks` step runs `go mod tidy` from `src/` so goreleaser builds against a clean module. The `main: ./src/cmd/repo` field accommodates the `src/` rooted layout.

### `.github/workflows/release.yml` (intent)

```yaml
name: release
on:
  push:
    tags: ['v*']

permissions:
  contents: write

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - uses: actions/setup-go@v5
        with:
          go-version-file: src/go.mod
      - uses: goreleaser/goreleaser-action@v6
        with:
          version: latest
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          HOMEBREW_TAP_TOKEN: ${{ secrets.HOMEBREW_TAP_TOKEN }}
```

`fetch-depth: 0` is required for goreleaser to read git history for changelog generation. `go-version-file: src/go.mod` keeps the CI Go version in sync with the module.

### Setup Checklist (manual, pre-first-release)

These are out-of-band steps the user MUST complete before pushing the first `v*` tag:

1. **Provision `HOMEBREW_TAP_TOKEN`**:
   - Create a GitHub PAT with `contents:write` on `sahil87/homebrew-tap`.
   - Add as a repo secret in `sahil87/repo` settings → Secrets and variables → Actions.
2. **Verify `sahil87/homebrew-tap`** exists with a `Formula/` directory.
3. **First release smoke test**: tag, push, watch the workflow, verify the GitHub Release has 4 binaries + `checksums.txt`, verify `Formula/repo.rb` lands in the tap.

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
- Reasoning: the Go binary is unproven on day one. `0.x.y` signals pre-stability; reserve `1.0.0` for "has run in production for ~2 weeks without friction."
- Once the binary has replaced the bash script in daily use without issues, cut `1.0.0`.

## Distribution Channels

After the release pipeline is live:

| Channel | Install command |
|---|---|
| Homebrew (macOS, Linux) | `brew install sahil87/tap/repo` |
| GitHub Release tarball | Download from `https://github.com/sahil87/repo/releases`, extract, place on PATH |
| From source | `git clone …; cd repo; just install` |

For v0.0.1 (this change), only the "from source" channel works. The other two land with the follow-up change.

## Behavioral Scenarios (GIVEN/WHEN/THEN)

### Local build (in scope for v0.0.1)

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

### Release pipeline (deferred to follow-up change)

> **GIVEN** all setup-checklist items complete and a clean working tree at commit `<sha>`
> **WHEN** I run `git tag -a v0.0.1 -m "Initial release" && git push origin v0.0.1`
> **THEN** the `release` workflow runs to completion
> **AND** `https://github.com/sahil87/repo/releases/tag/v0.0.1` shows 4 binary archives + `checksums.txt`
> **AND** `sahil87/homebrew-tap` has a new commit adding/updating `Formula/repo.rb`

> **GIVEN** the v0.0.1 release is published and `Formula/repo.rb` is in the tap
> **WHEN** a fresh user runs `brew install sahil87/tap/repo`
> **THEN** the install succeeds
> **AND** `repo --version` prints `v0.0.1`

## Design Decisions

1. **goreleaser over hand-rolled bash.** ~30 lines of YAML versus ~200 lines of bash. Industry standard. Free integration with homebrew-tap.
2. **Tag push is the single trigger.** No manual workflow dispatch, no GUI clicks. Tag → release.
3. **Version injection via ldflags.** Same string used for `--version` output, GitHub Release name, and homebrew formula version. Single source of truth: `git describe`.
4. **`v0.0.1` initial version.** Pre-stability signal. Bumps to 1.0 once daily-driven without friction.
5. **Local build path lives in this change; release pipeline split out.** Local builds need no secrets and finish in seconds; the release pipeline needs CI runs and `HOMEBREW_TAP_TOKEN` provisioning that can't happen in a single session.
6. **No Windows support.** Cross-platform code uses build tags for darwin/linux only. Adding Windows would require a `platform/open_windows.go` and a different shell integration story; out of scope for now.
7. **No code signing / notarization for v0.0.1.** Gatekeeper warns on first run; users right-click → Open. Real signing requires an Apple Developer account ($99/year) and is deferred until the project has users that warrant the cost.
