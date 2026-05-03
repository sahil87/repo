# Spec: Bootstrap Go binary v0.0.1

**Change**: 260503-iu93-bootstrap-go-binary
**Created**: 2026-05-03
**Affected memory**: `docs/memory/cli/`, `docs/memory/config/`, `docs/memory/architecture/`, `docs/memory/build/`

> **Authority note**: The 4 reference specs in `docs/specs/` (`cli-surface.md`, `config-resolution.md`, `architecture.md`, `build-and-release.md`) are the **canonical contracts** for what the binary does. This change spec **does not duplicate** their content — it references them and adds the change-scoped requirements (audit gates, acceptance criteria, scope boundary) needed to drive apply and review. When the reference spec and this spec disagree, the reference spec wins; this spec is wrong and gets corrected.

## Non-Goals

- Release pipeline (`.goreleaser.yaml`, `.github/workflows/release.yml`, `scripts/release.sh`, `homebrew-tap` formula update, first `v0.0.1` git tag) — deferred to follow-up change `260503-dgq0-release-pipeline` because CI runs and `HOMEBREW_TAP_TOKEN` provisioning cannot happen in a single session.
- Windows support — explicitly excluded by Constitution Cross-Platform Behavior (darwin/linux only).
- `internal/git/` package — Architecture spec §"Why `internal/git/` does NOT exist" rules this out for v0.0.1; the entire git surface is one `git clone` plus a filesystem check, which does not justify a package.
- Code signing / notarization — `build-and-release.md` Design Decision 7 defers this until the project warrants the cost.
- Modifying any of the 4 reference specs — they are frozen at session start; if a spec is wrong, fix the spec **before** apply, do not let implementation diverge silently.

## CLI Surface: Subcommand Behavior

### Requirement: Subcommand contract conformance

The binary SHALL expose the 11 subcommands listed in `docs/specs/cli-surface.md` §"Subcommand Inventory (v0.0.1)" with the exact arg shapes, behaviors, exit codes, and stdout/stderr conventions specified there. The implementation MUST satisfy every GIVEN/WHEN/THEN scenario in `docs/specs/cli-surface.md` §"Behavioral Scenarios".

#### Scenario: Reference contract is authoritative
- **GIVEN** a discrepancy is observed between this spec and `docs/specs/cli-surface.md`
- **WHEN** an implementation decision is needed
- **THEN** `docs/specs/cli-surface.md` is followed
- **AND** the discrepancy is reported as a spec correction request before apply continues

#### Scenario: Bare `repo` invocation with no args
- **GIVEN** `repos.yaml` lists at least one repo
- **WHEN** `repo` is invoked with no arguments and `fzf` is on PATH
- **THEN** fzf opens with all repos visible
- **AND** the selected absolute path is printed to stdout
- **AND** exit code is 0 on selection, 130 on Esc

### Requirement: Match resolution algorithm parity

The match resolution path used by `repo`, `repo <name>`, `repo path`, `repo code`, `repo open`, `repo cd`, and `repo clone` MUST implement the algorithm described in `docs/specs/cli-surface.md` §"Match Resolution Algorithm" verbatim — case-insensitive substring match on the **name column only** (not path, not URL); exactly-1-match short-circuits fzf; otherwise invoke fzf with `--query <name> --select-1 --height 40% --reverse --with-nth 1 --delimiter '\t'`.

#### Scenario: Unique substring match short-circuits fzf
- **GIVEN** `repos.yaml` contains exactly one repo whose name contains `repo` (case-insensitive)
- **WHEN** `repo repo` is run
- **THEN** fzf is NOT invoked (verifiable by uninstalling fzf or by integration test that asserts fzf not on PATH does not fail this path)
- **AND** stdout is the absolute path

#### Scenario: Ambiguous match invokes fzf with --query and --select-1
- **GIVEN** `repos.yaml` contains 2+ repos whose names contain `repo`
- **WHEN** `repo repo` is run
- **THEN** fzf is invoked
- **AND** the invocation includes `--query repo --select-1 --height 40% --reverse --with-nth 1 --delimiter '\t'`

### Requirement: `repo cd` binary form prints hint and exits 2

The compiled binary's `cd` subcommand SHALL print the exact stderr message defined in `docs/specs/cli-surface.md` §"Behavioral Scenarios" → "`repo cd` binary form" and exit with code 2. The shell-function form (post-`eval`) is delivered by `repo shell-init zsh`'s emitted text, which is verified separately.

#### Scenario: Direct `repo cd` invocation
- **WHEN** the user runs `./bin/repo cd somerepo` directly (no shell integration)
- **THEN** stderr contains `repo: 'cd' is shell-only. Add 'eval "$(repo shell-init zsh)"' to your zshrc, or use: cd "$(repo path "<name>")"`
- **AND** exit code is 2

### Requirement: `-v` and `--version` flag forms work; `version` subcommand tolerated

The binary SHALL print the version string when invoked with `-v` or `--version` and exit 0. The cobra-default `version` subcommand MAY also work; no effort is spent suppressing it. The version string is supplied at build time via `-ldflags "-X main.version=..."`; the package-level default is `dev`.

#### Scenario: `--version` prints non-empty string
- **GIVEN** `bin/repo` was built via `just build`
- **WHEN** `bin/repo --version` is run
- **THEN** stdout is a single non-empty line
- **AND** exit code is 0

#### Scenario: `-v` is identical to `--version`
- **WHEN** `bin/repo -v` is run
- **THEN** stdout matches the output of `bin/repo --version`
- **AND** exit code is 0

### Requirement: Lazy external-tool dependency checks

External tools (`fzf`, `git`, `code`, `open`/`xdg-open`) MUST be checked **only at the point of invocation**, never preemptively. Subcommands that resolve without an external tool (exact-match name, `repo ls`, `repo shell-init zsh`, `repo config init`, `repo config path`) MUST NOT error if those tools are missing. The exact stderr message and exit code per missing tool are specified in `docs/specs/cli-surface.md` §"External Tool Availability".

#### Scenario: `repo ls` works without fzf
- **GIVEN** `fzf` is not on PATH
- **AND** `repos.yaml` is loadable
- **WHEN** `repo ls` is run
- **THEN** stdout shows the table of repos
- **AND** exit code is 0

#### Scenario: Ambiguous match without fzf produces install hint
- **GIVEN** `fzf` is not on PATH
- **AND** the resolution would otherwise need fzf (0 or 2+ matches)
- **WHEN** `repo <ambiguous>` is run
- **THEN** stderr is exactly `repo: fzf is not installed. Install it: brew install fzf (macOS) or apt install fzf (Debian).`
- **AND** exit code is 1

### Requirement: `repo shell-init zsh` emits a working zsh integration

The output of `repo shell-init zsh` SHALL, when `eval`-ed in a zsh shell, define `repo` as a function that intercepts the `cd` subcommand and otherwise delegates to the binary, AND register completion via `compdef _repo repo`. Unsupported shells (no arg, `bash`, etc.) SHALL print the exact stderr messages in `docs/specs/cli-surface.md` §"Behavioral Scenarios" → "`repo shell-init zsh`" and exit 2.

#### Scenario: Zsh function defined after eval
- **GIVEN** zsh is available
- **WHEN** `eval "$(repo shell-init zsh)"` runs in a zsh subshell
- **THEN** `whence -w repo` reports `repo: function`

## Config Resolution: Search and Loading

### Requirement: Search order conformance

Config resolution SHALL implement the search order in `docs/specs/config-resolution.md` §"Search Order" exactly: (1) `$REPOS_YAML` if set; (2) `$XDG_CONFIG_HOME/repo/repos.yaml` if `$XDG_CONFIG_HOME` is set; (3) `$HOME/.config/repo/repos.yaml`. Resolution MUST NOT fall through on existence — if `$REPOS_YAML` is set, candidates 2 and 3 are not consulted. The bash-script paths `$DOTFILES_DIR/repos.yaml` and `$HOME/code/bootstrap/dotfiles/repos.yaml` MUST be absent from the search order.

#### Scenario: `$REPOS_YAML` set but file missing → hard error
- **GIVEN** `$REPOS_YAML=/nonexistent/path.yaml`
- **WHEN** any subcommand needing config runs
- **THEN** stderr is exactly `repo: $REPOS_YAML points to /nonexistent/path.yaml, which does not exist. Set $REPOS_YAML to an existing file or unset it.`
- **AND** exit code is 1
- **AND** candidates 2 and 3 are NOT consulted

#### Scenario: All three candidates resolve to nothing
- **GIVEN** `$REPOS_YAML` unset, `$XDG_CONFIG_HOME` unset, `~/.config/repo/repos.yaml` does not exist
- **WHEN** any subcommand needing config runs
- **THEN** stderr is exactly `repo: no repos.yaml found. Set $REPOS_YAML to a tracked file (e.g., a Dropbox path or a git-tracked dotfile), or run 'repo config init' to bootstrap one at $XDG_CONFIG_HOME/repo/repos.yaml.`
- **AND** exit code is 1

### Requirement: YAML schema and derived fields

`repos.yaml` SHALL be parsed using `gopkg.in/yaml.v3` as a top-level map of `directory → list of git URLs`. Directory keys whose literal first character is `~` SHALL be expanded to `$HOME` at load time; other directory strings are used verbatim. The repo **name** SHALL be derived as the last `/`-separated component of the URL with a trailing `.git` stripped. The repo **path** SHALL be `<expanded-dir> + "/" + <name>`.

#### Scenario: Name derivation handles SSH and HTTPS URLs
- **GIVEN** `git@github.com:sahil87/repo.git` and `https://github.com/wvrdz/loom.git`
- **WHEN** the repos are loaded
- **THEN** the derived names are `repo` and `loom` respectively

#### Scenario: Empty file loads as zero repos
- **GIVEN** `repos.yaml` is empty
- **WHEN** any loader runs
- **THEN** loading succeeds with an empty repo list
- **AND** `repo ls` prints nothing and exits 0

#### Scenario: Malformed YAML produces parse error with file context
- **GIVEN** `repos.yaml` has invalid YAML structure
- **WHEN** any loader runs
- **THEN** stderr contains the file path and a line number
- **AND** exit code is 1

### Requirement: `repo config init` writes starter at mode 0644

`repo config init` SHALL write the embedded `starter.yaml` to the resolved write target (`$REPOS_YAML` if set, else the XDG-default path). The parent directory SHALL be created with `os.MkdirAll` mode 0755 if absent. The file SHALL be created with mode 0644. If the target file already exists, the command SHALL refuse (no overwrite), print the exact "already exists" message in `config-resolution.md` §"`repo config init`", and exit 1. The embedded starter content SHALL be byte-identical to the block in `config-resolution.md` §"Embedded starter content" (self-bootstrapping with `git@github.com:sahil87/repo.git` under `~/code/sahil87`).

#### Scenario: Init creates starter at XDG path
- **GIVEN** `$REPOS_YAML` unset and `~/.config/repo/repos.yaml` does not exist
- **WHEN** `repo config init` runs
- **THEN** the file is created at `~/.config/repo/repos.yaml` with mode 0644
- **AND** stdout contains `Created /path/to/file`
- **AND** exit code is 0

#### Scenario: Init refuses to overwrite
- **GIVEN** `~/.config/repo/repos.yaml` already exists with arbitrary content
- **WHEN** `repo config init` runs
- **THEN** stderr contains the "already exists" message
- **AND** the existing file is byte-identical to its pre-invocation content
- **AND** exit code is 1

### Requirement: `repo config path` is non-load and never errors on missing file

`repo config path` SHALL run the same search-order resolution as a normal load and print the resolved path to stdout, **regardless of whether the file exists**. It MUST NOT trigger the `$REPOS_YAML`-set-but-missing hard error. Exit 0 unless the path itself is unresolvable (no env vars and no `$HOME`).

#### Scenario: Path printed even when file missing
- **GIVEN** `$REPOS_YAML=/tmp/nonexistent.yaml`
- **WHEN** `repo config path` runs
- **THEN** stdout is `/tmp/nonexistent.yaml`
- **AND** exit code is 0

## Architecture: Package Layout and Boundaries

### Requirement: Source tree matches reference layout

The Go source tree SHALL match the layout in `docs/specs/architecture.md` §"Top-Level Repository Layout" exactly: `go.mod` rooted at `src/`, module path `github.com/sahil87/repo`, `cmd/repo/<verb>.go` (one file per subcommand), flat `internal/<pkg>/` (config, repos, fzf, proc, platform), tests adjacent to source, `testdata/` per-package.

#### Scenario: Module declaration matches
- **GIVEN** the source tree exists
- **WHEN** `head -1 src/go.mod` is read
- **THEN** the line is `module github.com/sahil87/repo`

### Requirement: Centralized exec via `internal/proc`

All subprocess invocations in production code MUST go through `internal/proc`. No package outside `internal/proc/` MAY import `os/exec` directly. This is enforced by Constitution Principle I (Security First) and verified at apply time.

#### Scenario: os/exec import audit
- **WHEN** `grep -rn "os/exec" src/internal/ src/cmd/` is run from the repo root
- **THEN** the only matching files are under `src/internal/proc/`

#### Scenario: exec.Command (without Context) is forbidden
- **WHEN** `grep -rn "exec\.Command\b" src/` is run from the repo root
- **THEN** zero matches are returned (only `exec.CommandContext` is permitted)

> **Test-file exception**: Test files (`*_test.go`) MAY use `os/exec` and `exec.Command` for spawning the built binary in integration tests. The audits above scope to non-test code by virtue of the directory layout (production code lives in non-test files). Apply MUST verify this by also running `grep --include='*.go' --exclude='*_test.go' -rn "exec\.Command\b" src/` and getting zero results.

### Requirement: Cross-platform isolation via build tags

Platform-specific code SHALL be confined to `internal/platform/`. `open_darwin.go` SHALL begin with `//go:build darwin` and call `open <path>` via `internal/proc`. `open_linux.go` SHALL begin with `//go:build linux` and call `xdg-open <path>` via `internal/proc`. The rest of the codebase SHALL be platform-agnostic and SHALL build for both targets without modification.

#### Scenario: Darwin build succeeds with only open_darwin.go
- **GIVEN** the source tree
- **WHEN** `cd src && GOOS=darwin GOARCH=arm64 go build ./...` is run
- **THEN** the build succeeds with no errors

#### Scenario: Linux build succeeds with only open_linux.go
- **GIVEN** the source tree
- **WHEN** `cd src && GOOS=linux GOARCH=amd64 go build ./...` is run
- **THEN** the build succeeds with no errors

### Requirement: Cobra wiring conventions

Each subcommand SHALL be defined by a `func newXxxCmd() *cobra.Command` factory in its own file under `src/cmd/repo/`. The root command SHALL set `SilenceUsage = true` and `SilenceErrors = true`. The bare-form behavior (`repo` with no subcommand, or `repo <single-arg>`) SHALL be implemented via the root command's `RunE` checking `len(args)` and dispatching to the same handler used by `repo path`. `rootCmd.Version` SHALL be assigned from a `var version string` whose default value is `dev`.

#### Scenario: Bare-form delegates to path handler
- **GIVEN** `repos.yaml` has a single repo named `foo`
- **WHEN** `repo foo` is run
- **THEN** stdout is the same absolute path that `repo path foo` would produce
- **AND** exit code is the same

## Build & Local Install: Justfile and Scripts

### Requirement: Justfile is one-line recipes

`justfile` SHALL contain only one-line recipes that delegate to scripts (per Constitution Principle V). The v0.0.1 recipes are `default` (lists recipes), `build` (calls `./scripts/build.sh`), `install` (calls `./scripts/install.sh`), and `test` (`cd src && go test ./...`). No `release` recipe is included in this change.

#### Scenario: Justfile has the required recipes
- **WHEN** `just --list` is run from the repo root
- **THEN** the output lists `default`, `build`, `install`, `test`
- **AND** does NOT list `release`

### Requirement: build.sh injects version from git describe

`scripts/build.sh` SHALL be byte-identical (modulo trailing newline) to the script in `docs/specs/build-and-release.md` §"`scripts/build.sh`": `set -euo pipefail`, derive `VERSION` from `git describe --tags --always 2>/dev/null || echo dev`, build with `-ldflags "-X main.version=${VERSION}"`, output to `../bin/repo` (relative to `src/`), echo a success line.

#### Scenario: Build produces ./bin/repo
- **GIVEN** a clean checkout
- **WHEN** `just build` is run from the repo root
- **THEN** `./bin/repo` exists and is executable
- **AND** `./bin/repo --version` prints a non-empty line and exits 0

### Requirement: install.sh copies to ~/.local/bin/repo

`scripts/install.sh` SHALL invoke `./scripts/build.sh` first, then copy `./bin/repo` to `${HOME}/.local/bin/repo`, creating the parent directory if needed. The script content SHALL be byte-identical (modulo trailing newline) to the script in `docs/specs/build-and-release.md` §"`scripts/install.sh`".

#### Scenario: Install copies binary to ~/.local/bin
- **GIVEN** a clean checkout
- **WHEN** `just install` is run
- **THEN** `~/.local/bin/repo` exists, is executable
- **AND** `~/.local/bin/repo` is byte-identical to `./bin/repo` (after the build step)

### Requirement: `bin/` is gitignored

`.gitignore` SHALL include a `bin/` rule so that the local build output never appears in `git status`. If a `.gitignore` already exists, the rule is appended; otherwise, the file is created with this single rule.

#### Scenario: bin/ ignored after build
- **GIVEN** the repo at HEAD
- **WHEN** `just build` runs and then `git status --porcelain` is invoked
- **THEN** no line mentioning `bin/` appears

## Apply-Stage Acceptance Criteria

Apply MUST NOT mark itself complete until **every** criterion below passes. Each is a binary check; review re-runs them.

### Requirement: All audits and smoke tests pass

#### Scenario: Build, vet, and tests pass
- **WHEN** the agent runs `cd src && go build ./...`, then `cd src && go vet ./...`, then `cd src && go test ./...`
- **THEN** all three exit 0 with no warnings or errors
- **AND** every package under `src/cmd/repo/` and `src/internal/<pkg>/` has at least one `_test.go` file with at least one passing test

#### Scenario: just build and just install succeed
- **WHEN** the agent runs `just build`
- **THEN** `./bin/repo` exists, exit 0
- **WHEN** the agent runs `just install`
- **THEN** `~/.local/bin/repo` exists, exit 0

#### Scenario: Built binary version reporting works
- **WHEN** the agent runs `~/.local/bin/repo --version` and `~/.local/bin/repo -v`
- **THEN** both print a non-empty version string and exit 0

#### Scenario: Smoke test against existing dotfiles repos.yaml
- **GIVEN** `~/code/bootstrap/dotfiles/repos.yaml` exists
- **WHEN** the agent runs `REPOS_YAML=~/code/bootstrap/dotfiles/repos.yaml ~/.local/bin/repo ls`
- **THEN** stdout is non-empty (lists repos from that file) and exit code is 0
- **WHEN** the agent runs `REPOS_YAML=~/code/bootstrap/dotfiles/repos.yaml ~/.local/bin/repo path repo`
- **THEN** stdout is `/Users/sahil/code/sahil87/repo` (or the host-equivalent if `$HOME` differs) and exit code is 0
- **NOTE**: If the dotfiles file is absent on the apply host, the agent MUST construct an equivalent fixture file with the same `~/code/sahil87/repo.git` entry and run the smoke test against that, recording the substitution in apply notes.

#### Scenario: config init smoke test
- **WHEN** the agent runs `REPOS_YAML=$(mktemp -u --suffix=.yaml) ~/.local/bin/repo config init`
- **THEN** the file is created with mode 0644 containing the embedded starter content and exit code is 0
- **WHEN** the agent runs `REPOS_YAML=<that-path> ~/.local/bin/repo config path`
- **THEN** stdout is `<that-path>` and exit code is 0

#### Scenario: shell-init zsh smoke test
- **GIVEN** `zsh` is available on the apply host
- **WHEN** the agent runs `zsh -c 'eval "$(~/.local/bin/repo shell-init zsh)" && whence -w repo'`
- **THEN** stdout is `repo: function` and exit code is 0
- **NOTE**: If zsh is unavailable on the apply host, the agent MAY skip the runtime eval and instead grep the emitted text for the `compdef _repo repo` line and the `repo()` function definition.

#### Scenario: os/exec audit
- **WHEN** the agent runs `grep --include='*.go' --exclude='*_test.go' -rn '"os/exec"' src/internal/ src/cmd/`
- **THEN** matches are restricted to files under `src/internal/proc/`
- **WHEN** the agent runs `grep --include='*.go' --exclude='*_test.go' -rn 'exec\.Command\b' src/`
- **THEN** zero matches are returned

## Design Decisions

1. **This change spec is a thin pointer over the 4 reference specs.**
   - *Why*: The 4 reference specs are already written, indexed, and authoritative. Re-stating their content here would create two sources of truth and a guaranteed drift point. Reference + supplement avoids the drift; the only content unique to this spec is what is unique to *this change* (audit gates, acceptance criteria, scope boundary).
   - *Rejected*: Inlining all reference-spec content into this file. Risks: doubles the maintenance surface; review needs to compare two specs that say "the same thing" but in different words; if either diverges, ambiguity over which wins.

2. **`docs/specs/*` are the canonical contracts; this spec defers to them on conflict.**
   - *Why*: The reference specs are project-level documentation that survives past this change and is read by future feature changes. They MUST be the source of truth.
   - *Rejected*: Letting this spec override on conflict. Would create a precedent that change specs supersede project specs, defeating the project spec's purpose.

3. **`fab score` reads only this spec's Assumptions table; intake table is state-transfer.**
   - *Why*: Per `_generation.md` Spec Generation Procedure step 6 — confirm/upgrade/override intake assumptions, add new spec-discovered ones. The spec is the authoritative scoring source.
   - *Rejected*: Reading both. Double-counting would inflate confidence artificially.

4. **Test-file exception to the os/exec audit is explicit.**
   - *Why*: Integration tests must spawn the built binary; doing that without `os/exec` would require building a custom test harness, which is over-engineering. The Constitution Principle I bans direct `os/exec` in production code; integration tests are not production code.
   - *Rejected*: Routing tests through `internal/proc` as well. Adds ceremony, no security benefit (test code does not run on user machines).

5. **Smoke tests against the user's actual `~/code/bootstrap/dotfiles/repos.yaml` are part of apply.**
   - *Why*: The intake explicitly lists this as an apply-stage acceptance criterion. The smoke test exercises the real loader against real data, catching schema mismatches that fixtures might miss.
   - *Rejected*: Fixture-only smoke tests. Risk: a quirk of the user's actual yaml (a custom directory key, an unusual URL form) goes uncaught until first real use.

6. **Per-subcommand cobra factories instead of a single `init()` registry.**
   - *Why*: Mirrors the `wt` reference convention (see `~/code/sahil87/fab-kit/src/go/wt/cmd/main.go`) and makes each subcommand independently testable (the test imports the factory, builds a fresh `*cobra.Command`, executes against a buffer).
   - *Rejected*: `init()` side-effect registration. Couples subcommand wiring to package-level state, complicates testing, doesn't match the reference.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Initial binary version is 0.0.1 | Confirmed from intake #1 — user explicitly chose 0.0.1 to signal pre-stability; reinforced by `build-and-release.md` Design Decision 4 | S:100 R:90 A:90 D:95 |
| 2 | Certain | Shell integration subcommand is `repo shell-init zsh` | Confirmed from intake #2 — locked in `cli-surface.md` §"Subcommand Inventory"; ecosystem convention (`starship init zsh`, `zoxide init zsh`) | S:100 R:80 A:95 D:90 |
| 3 | Certain | Config search order: $REPOS_YAML → $XDG_CONFIG_HOME/repo/repos.yaml → $HOME/.config/repo/repos.yaml; no $DOTFILES_DIR or hardcoded dotfiles fallback | Confirmed from intake #3 — codified verbatim in `config-resolution.md` §"Search Order" | S:100 R:65 A:90 D:95 |
| 4 | Certain | Onboarding affordances: `repo config init` and `repo config path` | Confirmed from intake #4 — locked in `cli-surface.md` and `config-resolution.md` | S:95 R:60 A:90 D:90 |
| 5 | Certain | Starter `repos.yaml` content is self-bootstrap with this repo's URL hardcoded | Confirmed from intake #5 — exact byte content in `config-resolution.md` §"Embedded starter content" | S:100 R:70 A:90 D:95 |
| 6 | Certain | All Go source under `src/`, flat `internal/<pkg>/`, tests adjacent | Confirmed from intake #6 — codified in `architecture.md` §"Top-Level Repository Layout" | S:100 R:55 A:95 D:95 |
| 7 | Certain | `src/cmd/repo/` (nested), not flat `src/cmd/` | Confirmed from intake #7 — verified against `architecture.md` §"Conventions" | S:100 R:75 A:95 D:100 |
| 8 | Certain | CLI framework is cobra | Confirmed from intake #8 — `architecture.md` §"Conventions"; matches wt | S:100 R:60 A:95 D:95 |
| 9 | Certain | Tests adjacent to source | Confirmed from intake #9 — Go convention; codified in `architecture.md` §"Conventions" | S:100 R:90 A:100 D:100 |
| 10 | Certain | `internal/config/` and `internal/repos/` are separate packages | Confirmed from intake #10 — `architecture.md` §"Package Responsibilities" defines distinct surfaces | S:95 R:75 A:90 D:90 |
| 11 | Certain | All exec routes through `internal/proc/` (production code); audit via grep | Confirmed from intake #11; tightened with the test-file exception in this spec's Architecture section | S:100 R:60 A:95 D:90 |
| 12 | Certain | `internal/platform/` with build tags (`open_darwin.go`, `open_linux.go`) | Confirmed from intake #12; matches `architecture.md` §"Cross-Platform Strategy" | S:100 R:65 A:95 D:95 |
| 13 | Certain | `testdata/` per-package | Confirmed from intake #13 — codified in `architecture.md` §"Conventions" | S:100 R:80 A:100 D:100 |
| 14 | Certain | `internal/config/starter.yaml` embedded via `//go:embed` | Confirmed from intake #14 — `architecture.md` §"`internal/config`" defines the embed | S:95 R:75 A:95 D:90 |
| 15 | Certain | Build/release: justfile + scripts/, goreleaser/Actions/homebrew deferred | Confirmed from intake #15; this change implements the local path only, defers release pipeline per `build-and-release.md` §"Cross-Platform Release Pipeline" | S:100 R:60 A:95 D:90 |
| 16 | Certain | Module path: `github.com/sahil87/repo`, go.mod rooted at `src/` | Confirmed from intake #16 — `architecture.md` §"Conventions" | S:90 R:70 A:95 D:90 |
| 17 | Certain | This change ships v0.0.1 binary AND the 4 reference specs in one session; release pipeline split out | Confirmed from intake #17 — scope already locked by the existing reference specs and constitution v1.1.0 | S:100 R:55 A:90 D:90 |
| 18 | Certain | Help text mirrors bash `_usage` with renamed subcommands | Upgraded from intake Confident #18 — `cli-surface.md` §"Help Text" specifies the exact text verbatim, eliminating ambiguity | S:95 R:75 A:95 D:90 |
| 19 | Certain | Match resolution algorithm: case-insensitive substring on name, exactly-1 short-circuit, fzf with --query/--select-1 | Upgraded from intake Confident #19 — `cli-surface.md` §"Match Resolution Algorithm" specifies the exact flag set | S:95 R:60 A:95 D:90 |
| 20 | Certain | YAML library: gopkg.in/yaml.v3 | Upgraded from intake Confident #20 — `architecture.md` §"Conventions" locks this in | S:95 R:75 A:95 D:90 |
| 21 | Confident | Build matrix: darwin-arm64/amd64, linux-arm64/amd64 (no Windows) | Confirmed from intake #21 — locked by Constitution Cross-Platform Behavior, but only the host platform is exercised in this change (cross-compilation is goreleaser's job in the deferred pipeline) | S:90 R:80 A:95 D:95 |
| 22 | Certain | `repo -v` and `repo --version` MUST work; cobra `version` subcommand tolerated; version via -ldflags | Confirmed from intake #22 — `cli-surface.md` §"Cobra Wiring" and `build-and-release.md` §"Version Reporting" lock the contract | S:95 R:85 A:90 D:90 |
| 23 | Certain | `repos.yaml` file mode 0644 on `repo config init` | Confirmed from intake #23 — `config-resolution.md` Design Decision 3 documents the rationale | S:100 R:90 A:95 D:95 |
| 24 | Certain | `$REPOS_YAML` set but file missing → hard error with exact message | Confirmed from intake #24 — `config-resolution.md` §"Hard error" specifies the exact message | S:100 R:80 A:90 D:95 |
| 25 | Certain | External tools checked lazily, only when invoked; specific tool requirements per subcommand | Confirmed from intake #25 — `cli-surface.md` §"External Tool Availability" specifies exact messages and trigger points | S:100 R:75 A:90 D:95 |
| 26 | Certain | No `internal/git/` package for v0.0.1; inline `proc.Run("git", ...)` in `clone.go` | Confirmed from intake #26 — `architecture.md` §"Why `internal/git/` does NOT exist" | S:100 R:80 A:90 D:95 |
| 27 | Certain | Smoke tests against `~/code/bootstrap/dotfiles/repos.yaml` are part of apply, with fixture-substitution allowed if absent on host | New (spec) — intake §"Apply-stage acceptance criteria" #7 and #8 require this; the substitution clause handles ephemeral worktree environments | S:90 R:80 A:90 D:90 |
| 28 | Certain | Test files (`*_test.go`) MAY use `os/exec` and `exec.Command` for spawning the built binary in integration tests | New (spec) — needed to make the os/exec audit precise; integration tests must spawn the binary | S:95 R:90 A:90 D:95 |
| 29 | Certain | If a reference spec is wrong, fix the spec before apply continues; do not let the implementation diverge silently | New (spec) — Constitution Test Integrity section directly mandates this for tests; this spec extends the same posture to specs themselves | S:95 R:80 A:95 D:90 |
| 30 | Certain | Per-subcommand `func newXxxCmd() *cobra.Command` factories (mirrors wt convention); root sets `SilenceUsage = true` and `SilenceErrors = true` | New (spec) — codifies the cobra wiring pattern from `architecture.md` §"`cmd/repo`" so apply has a precise factory shape to follow | S:95 R:80 A:95 D:95 |

30 assumptions (28 certain, 2 confident, 0 tentative, 0 unresolved).
