# repo Constitution

## Core Principles

### I. Security First
All process execution MUST use `exec.CommandContext` with explicit argument slices — never shell strings or `exec.Command` without a context/timeout. User-provided input (repo names, paths, queries) SHALL be validated before passing to any subprocess (`git`, `code`, `open`, `xdg-open`, `fzf`). Shell injection is a show-stopper. This is non-negotiable.

### II. No Database
State MUST be derived from `repos.yaml` and the filesystem at request time. repo SHALL NOT introduce a database, cache file, or persistent state store. Every invocation re-reads the YAML and re-checks disk. If you can't derive it from these sources, you don't need it.

### III. Convention Over Configuration
repo SHOULD derive values from conventions rather than requiring explicit configuration. Repo names from `repos.yaml` URL basenames (with `.git` stripped). Paths from `<dir>/<name>` joins. The only required config is `repos.yaml` itself. New flags and env vars SHALL be added only when convention genuinely cannot suffice.

### IV. Wrap, Don't Reinvent
External tools (`git`, `fzf`, `yq` equivalents in Go, `code`, `open`, `xdg-open`) MUST be wrapped via internal packages, not reimplemented. For YAML, use a Go YAML library — do not parse by hand. For fzf, shell out — do not embed a fuzzy matcher. When a battle-tested tool does what you need, call it.

### V. Thin Justfile, Fab-Kit Build Pattern
The build system MUST mirror fab-kit's structure: `justfile` recipes are one-liners that delegate to `scripts/`. Logic, loops, and conditionals belong in shell scripts — the justfile is an index, not an implementation. Releases SHALL be cut by tagging `v*`, with a GitHub Actions workflow that builds cross-platform binaries, publishes a GitHub Release, and updates `homebrew-tap`. Local development uses `just build` and `just install` to populate a local cache.

### VI. Minimal Surface Area
repo is a locator. New top-level subcommands require explicit justification in the change's intake — "could this be a flag on an existing subcommand, or a separate tool?" must be answered "no" before adding one.

## Additional Constraints

### Test Integrity
Tests MUST conform to the implementation spec — never the other way around. When tests fail, the fix SHALL either (a) update the tests to match the spec, or (b) update the implementation to match the spec. Modifying implementation code solely to accommodate test fixtures or test infrastructure is prohibited. Specs are the source of truth; tests verify conformance to specs.

### Cross-Platform Behavior
Platform-specific code (e.g., `open` on Darwin vs `xdg-open` on Linux) MUST be isolated behind a small abstraction. The binary SHALL build and run on darwin-arm64, darwin-amd64, linux-arm64, and linux-amd64. Windows is not supported.

## Governance

**Version**: 1.1.0 | **Ratified**: 2026-05-03 | **Last Amended**: 2026-05-03
