# Local Build

How `hop` is built and installed locally. The cross-platform release pipeline (GitHub Actions, homebrew-tap) lives in [release-pipeline](release-pipeline.md).

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
go build -ldflags "-X main.version=${VERSION}" -o ../bin/hop ./cmd/hop
echo "built: bin/hop (version: ${VERSION})"
```

- Output: `./bin/hop` at the repo root. `bin/` is gitignored (`.gitignore` includes `bin/`).
- `VERSION` injected via `-ldflags "-X main.version=${VERSION}"` into the package-level `var version = "dev"` in `src/cmd/hop/main.go`. `hop --version` and `hop -v` print this string (cobra auto-wires both when `rootCmd.Version` is set).
- Possible `VERSION` values:
  - Pre-tag: short SHA from `git describe --always` (e.g., `9b6b2a4`).
  - Tagged: `v0.1.0`.
  - Post-tag commit: `v0.1.0-2-gabc123`.
  - No git history: `dev`.

## `scripts/install.sh`

```bash
#!/usr/bin/env bash
set -euo pipefail

./scripts/build.sh

DEST="${HOME}/.local/bin/hop"
mkdir -p "$(dirname "$DEST")"
cp -f ./bin/hop "$DEST"
echo "installed: $DEST"
```

- Always builds first (no skip-if-exists).
- Copies to `~/.local/bin/hop`. The user is responsible for `~/.local/bin` being on `$PATH`.
- Idempotent — re-running overwrites.

## `hop --version` chain

`scripts/build.sh` → `-ldflags "-X main.version=…"` → `src/cmd/hop/main.go::var version` → `rootCmd.Version = version` (set in `main()`) → cobra wires `--version` and `-v` automatically. The cobra-default `version` subcommand also works; no effort spent suppressing it.

## Cross-platform builds

Verified at apply time by:

```
cd src && GOOS=darwin GOARCH=arm64 go build ./...
cd src && GOOS=linux GOARCH=amd64 go build ./...
```

Both succeed because `internal/platform/` uses build tags (`//go:build darwin`, `//go:build linux`). Runtime tests run on the host platform only.

## Cross-references

The cross-platform release pipeline (tag-driven workflow, formula template, `release.sh`, homebrew-tap update) is documented in [release-pipeline](release-pipeline.md). Pre-implementation design intent lives in `docs/specs/build-and-release.md`.
