# repo Constitution

## Core Principles

### I. Security First
All process execution MUST use `exec.CommandContext` with explicit argument slices — never shell strings or `exec.Command` without a context/timeout. User-provided input (repo names, paths, queries) SHALL be validated before passing to any subprocess (`git`, `code`, `open`, `xdg-open`, `fzf`). Shell injection is a show-stopper. This is non-negotiable.

### II. No Database
State MUST be derived from `repos.yaml` and the filesystem at request time. repo SHALL NOT introduce a database, cache file, or persistent state store. Every invocation re-reads the YAML and re-checks disk. If you can't derive it from these sources, you don't need it.

### III. Backward-Compatible Migration
The Go binary MUST be a drop-in replacement for the existing shell script (`~/code/bootstrap/dotfiles/bin/repo`). Every documented subcommand (`path`, `code`, `open`, `cd`, `clone`, `clone --all`, `ls`, `init zsh`, the bare fzf form) SHALL behave identically — same arguments, same exit codes, same stdout/stderr conventions, same `$REPOS_YAML`/`$DOTFILES_DIR` env var resolution. Behavioral changes require a version bump and explicit changelog entry.

### IV. Convention Over Configuration
repo SHOULD derive values from conventions rather than requiring explicit configuration. Repo names from `repos.yaml` URL basenames (with `.git` stripped). Paths from `<dir>/<name>` joins. The only required config is `repos.yaml` itself. New flags and env vars SHALL be added only when convention genuinely cannot suffice.

### V. Wrap, Don't Reinvent
External tools (`git`, `fzf`, `yq` equivalents in Go, `code`, `open`, `xdg-open`) MUST be wrapped via internal packages, not reimplemented. For YAML, use a Go YAML library — do not parse by hand. For fzf, shell out — do not embed a fuzzy matcher. When a battle-tested tool does what you need, call it.

### VI. Thin Justfile, Fab-Kit Build Pattern
The build system MUST mirror fab-kit's structure: `justfile` recipes are one-liners that delegate to `scripts/`. Logic, loops, and conditionals belong in shell scripts — the justfile is an index, not an implementation. Releases SHALL be cut by tagging `v*`, with a GitHub Actions workflow that builds cross-platform binaries, publishes a GitHub Release, and updates `homebrew-tap`. Local development uses `just build` and `just install` to populate a local cache.

### VII. Minimal Surface Area
The CLI MUST stay small. New subcommands SHOULD only be added when an existing one genuinely cannot accommodate the functionality. Resist feature creep — repo is a locator, not a project manager. Anything beyond locate/open/list/clone belongs in a different tool.

## Additional Constraints

### Test Integrity
Tests MUST conform to the implementation spec — never the other way around. When tests fail, the fix SHALL either (a) update the tests to match the spec, or (b) update the implementation to match the spec. Modifying implementation code solely to accommodate test fixtures or test infrastructure is prohibited. Specs are the source of truth; tests verify conformance to specs.

### Process Execution
All `exec.CommandContext` calls MUST use a context with timeout. Defaults: 5s for read-only commands (`yq`, `fzf` query, file checks), 30s for `git clone`, no timeout only when explicitly waiting on user input (interactive `fzf`). Zombie processes from hung subcommands MUST NOT block the binary.

### Shell Integration
The `repo init zsh` output MUST remain stable. Users `eval` it in their `zshrc`; breaking the contract breaks every shell. Changes to the emitted shell function or completion SHALL be treated as a breaking change requiring a MAJOR bump.

### Cross-Platform Behavior
Platform-specific code (e.g., `open` on Darwin vs `xdg-open` on Linux) MUST be isolated behind a small abstraction. The binary SHALL build and run on darwin-arm64, darwin-amd64, linux-arm64, and linux-amd64. Windows is not supported.

## Governance

**Version**: 1.0.0 | **Ratified**: 2026-05-03 | **Last Amended**: 2026-05-03
