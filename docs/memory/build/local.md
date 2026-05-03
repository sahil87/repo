# Local Build

How v0.0.1 of `repo` is built and installed locally. Cross-platform release pipeline (goreleaser, GitHub Actions, homebrew-tap) is deferred to a follow-up change.

## Justfile

`justfile` at the repo root — one-line recipes only (Constitution Principle V):

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

No `release` recipe in v0.0.1.

## `scripts/build.sh`

```bash
#!/usr/bin/env bash
set -euo pipefail

VERSION="$(git describe --tags --always 2>/dev/null || echo dev)"
mkdir -p bin
cd src
go build -ldflags "-X main.version=${VERSION}" -o ../bin/repo ./cmd/repo
echo "built: bin/repo (version: ${VERSION})"
```

- Output: `./bin/repo` at the repo root. `bin/` is gitignored (`.gitignore` includes `bin/`).
- `VERSION` injected via `-ldflags "-X main.version=${VERSION}"` into the package-level `var version = "dev"` in `src/cmd/repo/main.go`. `repo --version` and `repo -v` print this string (cobra auto-wires both when `rootCmd.Version` is set).
- Possible `VERSION` values:
  - Pre-tag: short SHA from `git describe --always` (e.g., `a08147d`).
  - Tagged: `v0.0.1`.
  - Post-tag commit: `v0.0.1-2-gabc123`.
  - No git history: `dev`.

## `scripts/install.sh`

```bash
#!/usr/bin/env bash
set -euo pipefail

./scripts/build.sh

DEST="${HOME}/.local/bin/repo"
mkdir -p "$(dirname "$DEST")"
cp -f ./bin/repo "$DEST"
echo "installed: $DEST"
```

- Always builds first (no skip-if-exists).
- Copies to `~/.local/bin/repo`. The user is responsible for `~/.local/bin` being on `$PATH`.
- Idempotent — re-running overwrites.

## `repo --version` chain

`scripts/build.sh` → `-ldflags "-X main.version=…"` → `src/cmd/repo/main.go::var version` → `rootCmd.Version = version` (set in `main()`) → cobra wires `--version` and `-v` automatically. The cobra-default `version` subcommand also works; no effort spent suppressing it.

## Cross-platform builds

Verified at apply time by:

```
cd src && GOOS=darwin GOARCH=arm64 go build ./...
cd src && GOOS=linux GOARCH=amd64 go build ./...
```

Both succeed because `internal/platform/` uses build tags (`//go:build darwin`, `//go:build linux`). Runtime tests run on the host platform only.

## Out of scope for v0.0.1

The following are deferred to a follow-up release-pipeline change (`260503-dgq0-release-pipeline`):

- `.goreleaser.yaml` (build matrix: darwin-arm64/amd64, linux-arm64/amd64; no Windows).
- `.github/workflows/release.yml` (tag-push trigger).
- `homebrew-tap` formula publication.
- `HOMEBREW_TAP_TOKEN` provisioning.
- The first `v0.0.1` git tag and GitHub Release.
- A `release` recipe in the justfile.
- Code signing / notarization.

Design intent for the deferred pipeline is captured in `docs/specs/build-and-release.md`.
