# Intake: Bootstrap Go binary v0.0.1

**Change**: 260503-iu93-bootstrap-go-binary
**Created**: 2026-05-03
**Status**: Draft

## Origin

> User invoked `/fab-discuss` with: "Lets discuss how to set the initial version of this binary (repo). The current binary resides at ~/code/bootstrap/dotfiles. List down its sub-commands. Lets plan subcommands, folder structure etc in docs/specs. If there's anything to discuss also let me know."

This change is the output of a multi-turn `/fab-discuss` session that locked in 12 decisions about the Go rewrite of the existing bash `repo` script (`~/code/bootstrap/dotfiles/bin/repo`). The discussion progressed through:

1. **Round 1** — Listing existing bash subcommands; proposing initial version, naming, search-order, and spec-folder structure.
2. **Round 2** — User feedback: chose `0.0.1`, asked for constitution corrections, raised onboarding concerns, and clarified that "folder structure" meant the *repository* layout (not the spec folder). User pointed at `~/code/sahil87/fab-kit/src/go/wt` as the reference Go layout.
3. **Round 3** — Locked subcommand naming (`shell-init`), confirmed constitution edits, agreed on search order, agreed on `repo config init` strategy. Raised the "what goes in the starter `repos.yaml`" question.
4. **Round 4** — Resolved layout: `src/cmd/repo/`, cobra, tests adjacent, separate `internal/config/` and `internal/repos/`, `internal/proc/`, `internal/platform/` with build tags, `testdata/` per-package, embedded starter, goreleaser. Picked option (b) for starter content (self-bootstrap with this repo's URL).

**Scope note** (revised mid-session, after initial intake): The change has been refocused. The original intake was scoped to "create 4 spec docs only, defer all Go work to a downstream change." That was reversed: this change now produces the **working v0.0.1 Go binary end-to-end**. The 4 reference spec docs in `docs/specs/` are still produced as part of this change (they are project-level documentation that survives past this change). The **release pipeline** (.goreleaser.yaml, GitHub Actions workflow, homebrew-tap integration, secret provisioning) was carved out into a separate downstream change because it requires CI runs and secret provisioning that can't happen in a single session. The constitution edits agreed during `/fab-discuss` were applied out-of-band before this intake was finalized — `fab/project/constitution.md` already reflects the final state (6 principles, 2 additional constraints, version 1.1.0).

All decisions are encoded as Certain/Confident assumptions in the Assumptions table at the bottom of this intake. The downstream spec/tasks agent has zero conversation history — every value, file path, and design choice that was discussed appears verbatim in this document.

**Interaction mode**: Conversational, multi-turn (4 user turns). `/fab-draft` was invoked at the end to capture the resolved decisions before any implementation.

## Why

### The problem

The existing `repo` tool is a 182-line bash script at `~/code/bootstrap/dotfiles/bin/repo`. It has accumulated three structural problems that motivate a Go rewrite:

1. **No release pipeline** — Distribution is by symlink from a dotfiles repo. There is no versioning, no changelog, no homebrew formula, no install command for users outside Sahil's machine.
2. **No tests** — The bash script has no test harness. Behavior changes are validated by running it against `repos.yaml` and eyeballing output. Edge cases (malformed YAML, missing `fzf`, ambiguous matches) are not exercised systematically.
3. **Cross-platform drift risk** — The script branches on `uname -s` for `open` vs `xdg-open` but has no automated build for both platforms. Future platform-specific code (e.g., Windows? other shells?) has no place to live cleanly.

### The consequences if we don't act

- The tool stays a personal artifact. Other team members or open-source users can't install it through normal channels.
- Subtle defects creep in undetected (e.g., the existing case-insensitive matching is implemented via `awk` and would break if a repo name contained an `awk` regex special character).
- Adding any feature beyond the current 9 subcommands requires writing more bash, which becomes increasingly fragile.

### Why this approach (specs + Go binary in a single change; release pipeline split out)

- **Go for the rewrite**: Sahil already maintains other Go binaries via `fab-kit` (e.g., `wt` at `~/code/sahil87/fab-kit/src/go/wt`). Reusing the same toolchain (cobra, `go test`) means no new infrastructure.
- **Specs alongside code**: The 4 reference specs in `docs/specs/` capture decisions that survive past this change (subcommand contract, config resolution, package layout, build pattern). Writing them as part of this change — not before, not after — keeps spec and implementation aligned and reduces churn from drift between docs and code.
- **Release pipeline carved out**: `.goreleaser.yaml`, `.github/workflows/release.yml`, and `homebrew-tap` integration require CI runs and `HOMEBREW_TAP_TOKEN` provisioning that can't happen in a single conversational session. Those land in a separate downstream change. This change ships a binary that works locally via `just build` + `just install`.

### Rejected alternatives

- **Keep the bash script and just polish it**: Rejected because tests in bash are painful (`bats`, etc.) and the path to homebrew distribution is blocked.
- **Rewrite in Rust/Python**: Rejected. Go matches the existing fab-kit ecosystem; switching languages adds friction with zero offsetting benefit.
- **Specs-only change, Go code in a separate downstream change** (the original framing): Rejected. Splitting specs from code creates a lag where the spec drifts from reality before code is written. Doing both together keeps the spec grounded in implementation and produces a usable binary in one session.
- **Include release pipeline in this change**: Rejected. CI runs and secret provisioning don't fit a single session; carving them out into a separate change keeps this one finishable.

## What Changes

This change ships **a working v0.0.1 Go binary** at `src/cmd/repo/`: feature parity with the bash script (`~/code/bootstrap/dotfiles/bin/repo`) plus 2 new onboarding subcommands (`repo config init`, `repo config path`). Buildable and installable locally via `just build` + `just install`. Tests pass via `just test`. No release pipeline (deferred).

**Status of preconditions** (already in place at session start):
- The 4 reference specs in `docs/specs/` (`cli-surface.md`, `config-resolution.md`, `architecture.md`, `build-and-release.md`) are written and indexed. Section 1 below points at them; the implementation MUST conform.
- `fab/project/constitution.md` is at version 1.1.0 (6 principles, 2 additional constraints) — already updated out-of-band before this intake was finalized.

**Out of scope** (deferred to `260503-dgq0-release-pipeline`):
- `.goreleaser.yaml`
- `.github/workflows/release.yml`
- `HOMEBREW_TAP_TOKEN` secret provisioning
- `scripts/release.sh`
- The first `v0.0.1` git tag and GitHub Release

This change *creates* `justfile` and the local-only `scripts/build.sh` + `scripts/install.sh` because those are needed to validate the binary builds during apply. The release-pipeline change later *adds* a `release` recipe to the justfile and creates `scripts/release.sh`.

---

### 1. Reference specs (`docs/specs/`)

The 4 reference specs already exist on disk and are the authoritative source of truth for this change. The implementation MUST satisfy every requirement, scenario, and design decision in them. Do NOT re-derive design from this intake — the specs are canonical, the intake is a state transfer document that points at them.

| Spec | Path | Purpose |
|---|---|---|
| CLI Surface | `docs/specs/cli-surface.md` | Subcommand contract: 11 subcommands, args, exit codes, stdout/stderr conventions, match resolution algorithm, external tool availability rules, GIVEN/WHEN/THEN scenarios per subcommand |
| Config Resolution | `docs/specs/config-resolution.md` | Search order (`$REPOS_YAML` → `$XDG_CONFIG_HOME/repo/repos.yaml` → `$HOME/.config/repo/repos.yaml`), hard-error semantics, YAML schema, `repo config init` flow, embedded starter content |
| Architecture | `docs/specs/architecture.md` | `src/` layout (full tree), package responsibilities (`cmd/repo`, `internal/{config,repos,fzf,proc,platform}`), wrapper boundaries, security boundary (centralized `internal/proc`), build tags strategy |
| Build & Release | `docs/specs/build-and-release.md` | Local justfile + scripts (in-scope for v0.0.1) + release pipeline intent (deferred to follow-up change `260503-dgq0-release-pipeline`) |

Plus `docs/specs/index.md` is already updated to link these four. No edits to the specs are expected during this change — they're frozen at session start. If the spec is wrong, fix the spec before proceeding (do not let the implementation diverge silently).

### 2. Go binary implementation (`src/`, `justfile`, `scripts/`)

Implement the v0.0.1 binary per `docs/specs/architecture.md`. The implementation MUST satisfy every subcommand contract from `docs/specs/cli-surface.md` and every config-resolution behavior from `docs/specs/config-resolution.md`.

**Files to create** (full list — paths relative to repo root):

```
src/
├── go.mod                                    # module: github.com/sahil87/repo
├── go.sum
├── cmd/
│   └── repo/
│       ├── main.go                           # entrypoint, ~10 lines; sets rootCmd.Version, calls Execute()
│       ├── root.go                           # cobra root command + global flags + version handling
│       ├── path.go                           # `repo path <name>` and bare `repo <name>`
│       ├── path_test.go
│       ├── code.go                           # `repo code [<name>]`
│       ├── code_test.go
│       ├── open.go                           # `repo open [<name>]`
│       ├── open_test.go
│       ├── cd.go                             # `repo cd <name>` (binary form: prints hint, exits 2)
│       ├── cd_test.go
│       ├── clone.go                          # `repo clone [<name>]` and `repo clone --all`
│       ├── clone_test.go
│       ├── ls.go                             # `repo ls`
│       ├── ls_test.go
│       ├── shell_init.go                     # `repo shell-init zsh` (emits zsh integration)
│       ├── shell_init_test.go
│       ├── config.go                         # `repo config init` and `repo config path`
│       ├── config_test.go
│       ├── integration_test.go               # end-to-end tests using built binary
│       └── testutil_test.go                  # shared test helpers
└── internal/
    ├── config/
    │   ├── config.go                         # YAML schema types, Load(path) (*Config, error)
    │   ├── config_test.go
    │   ├── resolve.go                        # Resolve() (string, error) — search order
    │   ├── resolve_test.go
    │   ├── starter.yaml                      # //go:embed for `repo config init`
    │   └── testdata/
    │       ├── valid.yaml
    │       ├── empty.yaml
    │       └── malformed.yaml
    ├── repos/
    │   ├── repos.go                          # type Repo, type Repos, MatchOne/List/etc.
    │   └── repos_test.go
    ├── fzf/
    │   ├── fzf.go                            # Pick(items, query) — wraps fzf via internal/proc
    │   └── fzf_test.go
    ├── proc/
    │   ├── proc.go                           # Run/RunInteractive — centralized exec.CommandContext
    │   └── proc_test.go
    └── platform/
        ├── platform.go                       # OS detection, common types
        ├── open_darwin.go                    # //go:build darwin; uses `open`
        └── open_linux.go                     # //go:build linux; uses `xdg-open`
```

Top-level files in repo root:

```
justfile                        # one-line recipes: build, install, test (no release recipe in this change)
scripts/
├── build.sh                    # go build with -ldflags version injection → ./bin/repo
└── install.sh                  # calls build.sh, copies to ~/.local/bin/repo
```

**Apply-stage acceptance criteria** (every one MUST be met before review):

1. `cd src && go build ./...` succeeds with no errors.
2. `cd src && go test ./...` passes — every package has at least one test file with at least one passing test.
3. `cd src && go vet ./...` passes with no warnings.
4. `just build` produces `./bin/repo`.
5. `just install` puts the binary in `~/.local/bin/repo` (or equivalent local cache).
6. `~/.local/bin/repo --version` and `~/.local/bin/repo -v` both print a non-empty version string (typically `v0.0.1-<n>-g<sha>` from `git describe`).
7. `~/.local/bin/repo ls` against the user's existing `~/code/bootstrap/dotfiles/repos.yaml` (when `REPOS_YAML=~/code/bootstrap/dotfiles/repos.yaml ~/.local/bin/repo ls` is invoked) prints all repos from that file.
8. `~/.local/bin/repo path repo` (against the same yaml) prints `/Users/sahil/code/sahil87/repo`.
9. `~/.local/bin/repo config init` writes a starter file (verify with `repo config path`).
10. `~/.local/bin/repo shell-init zsh` emits valid zsh code (verify by `eval`-ing in a subshell and confirming the function is defined).
11. Audit: `grep -rn "os/exec" src/internal/ src/cmd/` returns hits only in `src/internal/proc/proc.go` (Constitution Principle I — Security First).
12. Audit: `grep -rn "exec.Command\b" src/` returns zero hits (only `exec.CommandContext` is allowed).

**Out of scope for apply** (deferred to release-pipeline change):
- Cross-compilation tests (we only validate the host platform; goreleaser tests Darwin+Linux in CI).
- Homebrew formula generation.
- Tagging or pushing.
- Any GitHub Actions workflow.

**Implementation order** (suggested; tasks.md will formalize):
1. `src/go.mod` (run `go mod init github.com/sahil87/repo`), pin cobra and yaml.v3.
2. `internal/proc/` — foundational; everything else depends on it.
3. `internal/config/` — load + resolve + embed starter.
4. `internal/repos/` — model + matching.
5. `internal/platform/` — Open() with build tags.
6. `internal/fzf/` — depends on proc.
7. `cmd/repo/root.go` — cobra root with version handling.
8. `cmd/repo/<each>.go` — one subcommand at a time, with adjacent tests. Order: `ls`, `path`, `code`, `open`, `cd`, `clone`, `shell-init`, `config`.
9. `cmd/repo/integration_test.go` — end-to-end against test fixtures.
10. `justfile` + `scripts/build.sh` + `scripts/install.sh`.

## Affected Memory

This change creates new memory files documenting the post-implementation behavior. Expected during hydrate (after apply + review pass):

- `cli/subcommands` (new) — what each subcommand actually does, finalized from spec + apply
- `cli/match-resolution` (new) — case-insensitive substring + fzf fallback algorithm
- `config/search-order` (new) — `$REPOS_YAML` → `$XDG_CONFIG_HOME/repo/repos.yaml` → `$HOME/.config/repo/repos.yaml`
- `config/yaml-schema` (new) — directory-keyed map of git URLs
- `config/init-bootstrap` (new) — `repo config init` behavior, embedded starter
- `architecture/package-layout` (new) — `src/cmd/repo/` + `src/internal/<pkg>/` flat layout, cobra
- `architecture/wrapper-boundaries` (new) — `internal/proc` centralized exec, `internal/fzf` wrapper, `internal/platform` build tags
- `build/local` (new) — justfile + scripts/build.sh + scripts/install.sh; cross-platform release deferred

These memory files are created automatically by `/fab-continue` at the hydrate stage based on what was actually built. The hydrate agent reads `apply.md` + the produced source files + `spec.md` and writes memory entries that reflect the realized system, not the planned one.

## Impact

**Files created** (`src/` — Go binary):
- `src/go.mod`, `src/go.sum`
- `src/cmd/repo/{main,root,path,code,open,cd,clone,ls,shell_init,config}.go` + adjacent `_test.go`
- `src/cmd/repo/{integration,testutil}_test.go`
- `src/internal/config/{config,resolve}.go` + adjacent `_test.go`, `src/internal/config/starter.yaml`, `src/internal/config/testdata/{valid,empty,malformed}.yaml`
- `src/internal/repos/{repos.go,repos_test.go}`
- `src/internal/fzf/{fzf.go,fzf_test.go}`
- `src/internal/proc/{proc.go,proc_test.go}`
- `src/internal/platform/{platform.go,open_darwin.go,open_linux.go}`

**Files created** (top-level — local build):
- `justfile`
- `scripts/build.sh`
- `scripts/install.sh`

**Files modified**:
- `.gitignore` — add `bin/` (ignore local build output)
- `README.md` — replace placeholder with install/usage section pointing at `repo --help` and `docs/specs/`

**Preconditions** (already in place — the implementation reads but does NOT modify these):
- `docs/specs/cli-surface.md`, `docs/specs/config-resolution.md`, `docs/specs/architecture.md`, `docs/specs/build-and-release.md` — written before apply
- `docs/specs/index.md` — already updated with the 4 spec rows
- `fab/project/constitution.md` — already at version 1.1.0

**Out of scope** (deferred to a separate release-pipeline change):
- `.goreleaser.yaml`
- `.github/workflows/release.yml`
- `scripts/release.sh`
- Initial `v0.0.1` tag and GitHub Release
- `homebrew-tap` formula update
- `HOMEBREW_TAP_TOKEN` provisioning

**Downstream consumers of this change**:
- The release-pipeline change (separate intake) will read `docs/specs/build-and-release.md` and the produced binary's local build pattern as inputs for the goreleaser config.
- Any future feature change to `repo` will read `docs/specs/cli-surface.md` (canonical contract) and the memory files produced at hydrate.

**Reference paths** (for the executing agent):
- Existing bash script: `/Users/sahil/code/bootstrap/dotfiles/bin/repo` (read-only, source of truth for v1 behavior)
- Reference Go layout: `/Users/sahil/code/sahil87/fab-kit/src/go/wt/` (mirror this structure)
- Existing repos.yaml: `/Users/sahil/code/bootstrap/dotfiles/repos.yaml` (sample data for spec scenarios)

## Open Questions

All in-scope decisions have been resolved (see Assumptions table). One out-of-scope item remains, surfaced for the implementation change's setup checklist:

1. `HOMEBREW_TAP_TOKEN` provisioning — out of scope for this specs change. Must be addressed before the first release tag is pushed.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Initial binary version is 0.0.1 | Discussed — user explicitly chose 0.0.1 over 0.1.0/1.0.0 to signal unproven implementation | S:100 R:90 A:90 D:95 |
| 2 | Certain | Shell integration subcommand is `repo shell-init zsh` (not `init zsh`, `completions zsh`, or `shell-setup zsh`) | Discussed — user picked `shell-init` for consistency with `fab init` and ecosystem convention (`starship init zsh`, `zoxide init zsh`) | S:100 R:80 A:95 D:90 |
| 3 | Certain | Config search order: `$REPOS_YAML` → `$XDG_CONFIG_HOME/repo/repos.yaml` → `$HOME/.config/repo/repos.yaml` | Discussed — user agreed; also explicitly approved dropping `$DOTFILES_DIR` and the hardcoded `$HOME/code/bootstrap/dotfiles/repos.yaml` fallback | S:100 R:65 A:90 D:95 |
| 4 | Certain | Onboarding affordances: `repo config init` (bootstrap a starter file) and `repo config path` (debug aid) | Discussed — user explicitly approved both subcommands as deliberate exceptions to "minimal surface area" | S:95 R:60 A:90 D:90 |
| 5 | Certain | Starter `repos.yaml` content is option (b): self-bootstrap with this repo's URL hardcoded as a copy-pasteable example | Discussed — user picked (b) over (a) empty template and (c) auto-discover, "to have some example for people to readily copy paste and edit" | S:100 R:70 A:90 D:95 |
| 6 | Certain | All Go source under `src/` (matches `fab-kit/src/go/wt` convention); flat `internal/<pkg>/`; tests adjacent | Discussed — user pointed at `~/code/sahil87/fab-kit/src/go/wt` as the reference layout and approved | S:100 R:55 A:95 D:95 |
| 7 | Certain | `src/cmd/repo/` (nested), not flat `src/cmd/` | Discussed — user explicitly chose `src/cmd/repo` in the round-4 answer to question 1 | S:100 R:75 A:95 D:100 |
| 8 | Certain | CLI framework is cobra | Discussed — user explicitly approved; `wt` already uses cobra so dep weight is amortized across binaries | S:100 R:60 A:95 D:95 |
| 9 | Certain | Tests adjacent to source (Go convention) | Discussed — user approved | S:100 R:90 A:100 D:100 |
| 10 | Certain | `internal/config/` and `internal/repos/` are separate packages | Discussed — user approved keeping them separate | S:95 R:75 A:90 D:90 |
| 11 | Certain | `internal/proc/` exists as the centralized `exec.CommandContext` wrapper; all exec calls go through it | Discussed — user approved; aligns with Constitution Principle I (Security First) | S:100 R:60 A:95 D:90 |
| 12 | Certain | `internal/platform/` with build tags (`open_darwin.go`, `open_linux.go`) | Discussed — user approved; matches Constitution Cross-Platform Behavior section | S:100 R:65 A:95 D:95 |
| 13 | Certain | `testdata/` per-package (not centralized at `src/testdata/`) | Discussed — user explicitly clarified after option (a) was presented: "ok - just keep it right next to the text and keeps the relative paths simple" | S:100 R:80 A:100 D:100 |
| 14 | Certain | `internal/config/starter.yaml` embedded via `//go:embed` for `repo config init` | Discussed — user approved | S:95 R:75 A:95 D:90 |
| 15 | Certain | Build/release: justfile (one-liners) → scripts/, goreleaser, GitHub Actions, homebrew-tap | Discussed — user explicitly chose goreleaser over hand-rolled release script | S:100 R:60 A:95 D:90 |
| 16 | Certain | Module path: `github.com/sahil87/repo`, with `go.mod` rooted at `src/` | Discussed — derived from `wt` convention | S:90 R:70 A:95 D:90 |
| 17 | Certain | This change produces both the working v0.0.1 Go binary AND the 4 reference spec files in `docs/specs/`, all in a single session. The release pipeline (`.goreleaser.yaml`, `.github/workflows/`, homebrew-tap, secret provisioning) is split out to a separate downstream change. | Discussed — user explicitly reversed the original specs-only framing mid-session and asked for the binary to be built in this session, with release pipeline carved out into its own intake. Constitution edits applied out-of-band (already done). | S:100 R:55 A:90 D:90 |
| 18 | Confident | Help text mirrors the existing bash `_usage` (lines 6–32 of `~/code/bootstrap/dotfiles/bin/repo`), updated for renamed subcommands | Pattern preservation — bash version is the v1 behavioral contract | S:80 R:75 A:85 D:85 |
| 19 | Confident | Match resolution algorithm preserves bash `_resolve_row` behavior: case-insensitive substring on name, exactly-1-match short-circuit, fzf with `--query` and `--select-1` otherwise | Pattern preservation — explicitly required for behavioral parity with bash | S:85 R:60 A:90 D:85 |
| 20 | Confident | YAML library: `gopkg.in/yaml.v3` (Go ecosystem standard) | Constitution Principle IV says "use a Go YAML library, do not parse by hand"; v3 is the conventional choice | S:80 R:75 A:90 D:80 |
| 21 | Confident | Goreleaser build matrix: darwin-arm64, darwin-amd64, linux-arm64, linux-amd64 (no Windows) | Constitution Cross-Platform Behavior section lists these four targets explicitly | S:90 R:80 A:95 D:95 |
| 22 | Certain | `repo -v` and `repo --version` MUST both work (flag form is the documented version interface). The cobra-default `version` subcommand MAY exist (no effort to suppress). Version string injected at build time via `-ldflags "-X main.version=$(git describe --tags --always)"`. | Discussed — user explicitly required `-v`/`--version` to work and stated the `version` subcommand is optional. Pattern reference: wt currently has no version handling, so this establishes the convention. | S:95 R:85 A:90 D:90 |
| 23 | Certain | `repos.yaml` file mode 0644 on `repo config init` | Discussed — user explicitly chose 0644. File contains repo paths and git URLs, none of which are credentials. | S:100 R:90 A:95 D:95 |
| 24 | Certain | `$REPOS_YAML` set but file missing → hard error (do not fall through). Print: `repo: $REPOS_YAML points to <path>, which does not exist. Set $REPOS_YAML to an existing file or unset it.` Exit 1. | Discussed — user explicitly chose hard error. Setting an env var is a declaration of intent; fall-through would mask config bugs. | S:100 R:80 A:90 D:95 |
| 25 | Certain | `fzf` missing → error with install hint and exit 1, but ONLY when fzf would actually be invoked. Subcommands that resolve without fzf (exact-match name, `repo ls`, `repo shell-init zsh`, `repo config init`, `repo config path`) MUST NOT check for fzf and MUST NOT error if missing. Same pattern applies to `git` (only error during clone), `code` (only error during `repo code`), `open`/`xdg-open` (only error during `repo open`). | Discussed — user chose option 1 (error with install hint). Edge-case refinement applied: scope the dependency check to actual invocation, don't preemptively fail. | S:95 R:75 A:85 D:90 |
| 26 | Certain | No `internal/git/` package for v0.0.1 — git calls happen inline in `cmd/repo/clone.go` via `internal/proc.Run("git", "clone", url, dest)`. Promote to a package only if the git surface grows beyond the single `clone` operation. | Discussed — user explicitly chose inline ("don't optimize prematurely"). Current git surface is exactly one operation plus a filesystem check, which doesn't justify a package. | S:100 R:80 A:90 D:95 |

26 assumptions (22 certain, 4 confident, 0 tentative, 0 unresolved).
