# Local Build

How `repo` is built and installed locally. The cross-platform release pipeline (GitHub Actions, homebrew-tap) lives in [release-pipeline](release-pipeline.md).

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

release bump="patch":
    ./scripts/release.sh {{bump}}
```

The `release` recipe delegates to `scripts/release.sh` and is documented in [release-pipeline](release-pipeline.md).

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

## Cross-references

The cross-platform release pipeline (tag-driven workflow, formula template, `release.sh`, homebrew-tap update) is documented in [release-pipeline](release-pipeline.md). Pre-implementation design intent lives in `docs/specs/build-and-release.md`.
