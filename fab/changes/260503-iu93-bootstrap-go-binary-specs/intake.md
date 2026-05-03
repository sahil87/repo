# Intake: Bootstrap Go binary specs

**Change**: 260503-iu93-bootstrap-go-binary-specs
**Created**: 2026-05-03
**Status**: Draft

## Origin

> User invoked `/fab-discuss` with: "Lets discuss how to set the initial version of this binary (repo). The current binary resides at ~/code/bootstrap/dotfiles. List down its sub-commands. Lets plan subcommands, folder structure etc in docs/specs. If there's anything to discuss also let me know."

This change is the output of a multi-turn `/fab-discuss` session that locked in 12 decisions about the Go rewrite of the existing bash `repo` script (`~/code/bootstrap/dotfiles/bin/repo`). The discussion progressed through:

1. **Round 1** — Listing existing bash subcommands; proposing initial version, naming, search-order, and spec-folder structure.
2. **Round 2** — User feedback: chose `0.0.1`, asked for constitution corrections, raised onboarding concerns, and clarified that "folder structure" meant the *repository* layout (not the spec folder). User pointed at `~/code/sahil87/fab-kit/src/go/wt` as the reference Go layout.
3. **Round 3** — Locked subcommand naming (`shell-init`), confirmed constitution edits, agreed on search order, agreed on `repo config init` strategy. Raised the "what goes in the starter `repos.yaml`" question.
4. **Round 4** — Resolved layout: `src/cmd/repo/`, cobra, tests adjacent, separate `internal/config/` and `internal/repos/`, `internal/proc/`, `internal/platform/` with build tags, `testdata/` per-package, embedded starter, goreleaser. Picked option (b) for starter content (self-bootstrap with this repo's URL).

**Scope note** (post-draft): The constitution edits agreed during the discussion were applied directly *before* this intake was finalized — they are no longer pending work in this change. `fab/project/constitution.md` already reflects the final state (6 principles, 2 additional constraints, version 1.1.0). This intake's scope is now **specs-only**.

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

### Why this approach (specs-first workflow before any Go code)

- **Go for the rewrite**: Sahil already maintains other Go binaries via `fab-kit` (e.g., `wt` at `~/code/sahil87/fab-kit/src/go/wt`). Reusing the same toolchain (cobra, `go test`, goreleaser, homebrew-tap) means no new infrastructure.
- **Specs-first**: Per Sahil's `fab` workflow, the canonical path is intake → spec → tasks → apply. By writing 4 specs *before* a single line of Go, downstream `/fab-continue` runs have a precise contract to implement against. This change produces those specs only.

### Rejected alternatives

- **Keep the bash script and just polish it**: Rejected because no homebrew-tap/release pipeline is possible without restructuring, and tests in bash are painful (`bats`, etc.).
- **Rewrite in Rust/Python**: Rejected. Go matches the existing fab-kit ecosystem; switching languages adds friction with zero offsetting benefit.
- **Skip the specs and start coding directly**: Rejected. Four distinct concerns (CLI surface, config resolution, repo architecture, build/release) each have non-obvious decisions that benefit from being written down before code. Without specs, the Go change becomes one giant intake with no clear acceptance criteria.

## What Changes

This change has **one deliverable**: create 4 spec files in `docs/specs/` and update `docs/specs/index.md` to link them.

No Go code is written in this change. No constitution edits remain (already applied). The Go implementation is a separate, downstream change that consumes these specs.

---

### 1. Specs to create

Four new files under `docs/specs/`. Each is sized to be quickly scannable (~50–150 lines). The specs index (`docs/specs/index.md`) MUST be updated to add a row for each new spec.

#### 1a. `docs/specs/cli-surface.md`

The canonical contract for what the binary exposes. Must contain:

**Subcommand inventory**: Exactly 11 subcommands for v0.0.1. The first 9 mirror the bash script's surface (feature parity is the change's stated purpose). The two new ones (`config init`, `config path`) are the onboarding affordances.

| Subcommand | Args | Behavior | Exit codes |
|---|---|---|---|
| `repo` | (none) | fzf picker, print selected repo's absolute path on stdout | 0 on selection, non-zero on cancel |
| `repo <name>` | `<name>` | Print abs path of matching repo. Case-insensitive substring match. If exactly 1 match, no fzf. If 0 or >1 matches, fzf opens with `--query <name>` prefilled. | 0 on print, non-zero on cancel/no-match |
| `repo path <name>` | `<name>` | Same as `repo <name>`, explicit form | 0 on print, non-zero on cancel |
| `repo code [<name>]` | optional `<name>` | Resolve via match-or-fzf, then `code <path>` | 0 on success |
| `repo open [<name>]` | optional `<name>` | Resolve via match-or-fzf, then `open <path>` (Darwin) or `xdg-open <path>` (Linux) | 0 on success, 1 if unsupported OS |
| `repo cd <name>` | `<name>` | Binary form: prints hint to stderr explaining `eval "$(repo shell-init zsh)"`, exits 2. Shell function form (after `eval`) intercepts and changes parent shell's cwd. | Binary: 2. Shell function: 0 on success, non-zero on no-match |
| `repo clone [<name>]` | optional `<name>` or `--all` | If `--all`: clone every repo in `repos.yaml` not already on disk; print summary `cloned=N skipped=N failed=N`; exit 0 only if `failed == 0`. Otherwise: resolve via match-or-fzf and `git clone <url> <dest>` if not already cloned. | 0 on success, 1 on conflict (path exists, not git), non-zero on git failure |
| `repo ls` | (none) | List all repos with name and path columns. Use `column -t -s $'\t'` style alignment. | 0 |
| `repo shell-init zsh` | `zsh` (required) | Emit zsh shell integration to stdout: function wrapper that intercepts `repo cd`, plus zsh completion via `compdef`. User runs `eval "$(repo shell-init zsh)"` in their `zshrc`. | 0 on success, 2 if shell missing/unsupported |
| `repo config init` | (none) | Bootstrap a starter `repos.yaml` at the resolved location (see config-resolution.md for path logic). If `$REPOS_YAML` is set, write there. Otherwise write to `$XDG_CONFIG_HOME/repo/repos.yaml` (creating dirs as needed). Refuse to overwrite an existing file. Print path written to stdout. | 0 on success, 1 if file already exists, 2 on write error |
| `repo config path` | (none) | Print the resolved config path to stdout (whether or not the file exists). Useful for debugging which file `repo` is reading. | 0 |
| `repo -h \| --help \| help` | (none) | Print help text and exit | 0 |

**Help text content**: Mirror the structure of the existing bash script's `_usage` function (lines 6–32 of `~/code/bootstrap/dotfiles/bin/repo`), updating subcommand names where they differ:
- Replace `repo init zsh` with `repo shell-init zsh`
- Add `repo config init` and `repo config path` rows
- Update the "Notes" section to mention the new config search order: `$REPOS_YAML`, then `$XDG_CONFIG_HOME/repo/repos.yaml`

**Stdout/stderr conventions** (from the bash script):
- Resolved paths and selections → stdout
- Status messages (`clone: <url> → <path>`, `skip: <reason>`) → stderr
- Errors → stderr
- `repo ls` output → stdout

**Match resolution algorithm** (Steps 1–3 of `_resolve_row` in the bash script, lines 92–106):
1. If query is non-empty: case-insensitive substring match on the *name* column (not path, not URL).
2. If exactly 1 match: return it without invoking fzf.
3. Otherwise (0 matches or 2+): invoke fzf with `--query <query> --select-1 --height 40% --reverse --with-nth 1 --delimiter '\t'`. The `--select-1` flag means fzf auto-selects if there's exactly one filter match.

**Required GIVEN/WHEN/THEN scenarios** (per `fab/project/config.yaml` stage_directives — spec stage requires this format):
- One per subcommand minimum
- Edge cases: empty query, no matches, exactly one match, multiple matches, missing config, malformed YAML, ambiguous unique-match (substring matches multiple names)

**External tool availability** (resolved):
- When `fzf` would be invoked but is not installed: error with install hint and exit 1. Print to stderr: `repo: fzf is not installed. Install it: brew install fzf (macOS) or apt install fzf (Debian).`
- The error fires *only when fzf would actually be invoked*. Subcommands that resolve without fzf (e.g., `repo path <exact-name>` with a unique-substring match, `repo ls`, `repo shell-init zsh`, `repo config init`, `repo config path`) MUST NOT check for fzf and MUST NOT error if it's missing.
- `git` missing: error similarly (`repo: git is not installed`) only when a `clone` subcommand is invoked.
- `code` missing: error similarly only when `repo code` is invoked.
- `open`/`xdg-open` missing: error similarly only when `repo open` is invoked.

**Mark with `[NEEDS CLARIFICATION]`** if not explicitly resolved:
- (none remaining for this spec)

#### 1b. `docs/specs/config-resolution.md`

How `repos.yaml` is found, parsed, validated, and bootstrapped.

**Search order** (resolved at every invocation — no caching, per Constitution Principle II "No Database"):

1. `$REPOS_YAML` if set and file exists
2. `$XDG_CONFIG_HOME/repo/repos.yaml` if `$XDG_CONFIG_HOME` is set
3. `$HOME/.config/repo/repos.yaml` (XDG fallback for systems where `$XDG_CONFIG_HOME` is unset)

**Removed from search order** (compared to bash script):
- `$DOTFILES_DIR/repos.yaml`
- `$HOME/code/bootstrap/dotfiles/repos.yaml`

These were dotfiles-specific paths — Sahil's personal layout leaking into the binary. User explicitly approved removing them.

**Behavior when no config is found**:
- Print to stderr: `repo: no repos.yaml found. Set $REPOS_YAML to a tracked file (e.g., a Dropbox path or a git-tracked dotfile), or run 'repo config init' to bootstrap one at $XDG_CONFIG_HOME/repo/repos.yaml.`
- Exit code: 1

**YAML schema** (unchanged from bash):

```yaml
# Repositories to clone, grouped by parent directory.
# Each key is a directory (~ is expanded). Values are git clone URLs.

~/code/sahil87:
  - git@github.com:sahil87/repo.git
  - git@github.com:sahil87/wt.git

~/code/wvrdz:
  - git@github.com:wvrdz/dev-shell.git
```

- Top-level: map of directory → list of git URLs
- Directory keys: `~` is expanded to `$HOME` (must happen before any filesystem operation)
- Repo name is derived from the URL: take the last path component, strip `.git` suffix
- Repo absolute path: `<expanded-dir>/<derived-name>`

**`repo config init` behavior**:

Determines the write target:
1. If `$REPOS_YAML` is set, write to that path (the user has already declared their intent)
2. Otherwise, write to `$XDG_CONFIG_HOME/repo/repos.yaml`, falling back to `$HOME/.config/repo/repos.yaml`

Behavior:
- If the target file already exists: refuse to overwrite. Print to stderr: `repo config init: <path> already exists. Delete it first or set $REPOS_YAML to a different path.` Exit 1.
- If the parent directory doesn't exist: create it (`mkdir -p` equivalent) with mode 0755.
- Write the embedded starter content. Mode 0644 (file contains repo paths and git URLs — none are credentials; no need to restrict to owner-only).
- Print to stdout: `Created <path>`.
- Print a hint to stderr: `Edit the file to add your repos. Tip: set $REPOS_YAML in your shell rc to point at a version-tracked location (e.g., dotfiles repo, Dropbox).`
- Exit 0.

**Starter content** (embedded via `//go:embed` from `src/internal/config/starter.yaml`):

User chose **option (b)** — self-bootstrap with this repo's URL hardcoded as a copy-pasteable example. The file must be valid YAML so users can run `repo` immediately after `repo config init` and see at least one entry.

```yaml
# Repositories to clone, grouped by parent directory.
# Each key is a directory (~ is expanded). Values are git clone URLs.
#
# Edit this file to add your own repos. Tip: set $REPOS_YAML in your shell rc
# to point at a version-tracked location (a git-tracked dotfile, Dropbox, etc.)
# so this config moves with you across machines.
#
# Example below: clones the `repo` tool itself. Replace or extend.

~/code/sahil87:
  - git@github.com:sahil87/repo.git
```

**`repo config path` behavior**:
- Run the same search order
- Print the path that *would* be used to stdout (whether or not the file exists)
- If no path can be resolved (none of the search-order candidates apply), print to stderr: `repo: no config path resolvable. Set $REPOS_YAML or ensure $XDG_CONFIG_HOME or $HOME is set.` Exit 1.
- Otherwise exit 0.

**Required GIVEN/WHEN/THEN scenarios**:
- `$REPOS_YAML` set + file exists → use it
- `$REPOS_YAML` set + file missing → hard error (do not silently fall through). Print to stderr: `repo: $REPOS_YAML points to <path>, which does not exist. Set $REPOS_YAML to an existing file or unset it.` Exit 1.
- `$REPOS_YAML` unset + `$XDG_CONFIG_HOME` set + file exists → use XDG
- `$REPOS_YAML` unset + `$XDG_CONFIG_HOME` unset + `~/.config/repo/repos.yaml` exists → use HOME fallback
- All three unresolved → error with onboarding hint
- `repo config init` with no existing file → success
- `repo config init` with existing file → refuse, exit 1
- Malformed YAML → parse error with file path and line number

**`[NEEDS CLARIFICATION]` markers**:
- (none remaining for this spec)

#### 1c. `docs/specs/architecture.md`

Repository folder structure + Go package layout + cross-platform strategy.

**Top-level repo layout**:

```
repo/
├── README.md
├── LICENSE
├── justfile                 # one-line recipes per Constitution Principle V
├── .goreleaser.yaml
├── .github/
│   └── workflows/
│       └── release.yml
├── src/                     # all Go source (mirrors fab-kit/src/go/wt convention)
│   ├── go.mod
│   ├── go.sum
│   ├── cmd/
│   │   └── repo/
│   │       ├── main.go
│   │       ├── root.go
│   │       ├── path.go              + path_test.go
│   │       ├── code.go              + code_test.go
│   │       ├── open.go              + open_test.go
│   │       ├── clone.go             + clone_test.go
│   │       ├── ls.go                + ls_test.go
│   │       ├── shell_init.go        + shell_init_test.go
│   │       ├── config.go            + config_test.go
│   │       ├── integration_test.go
│   │       └── testutil_test.go
│   └── internal/
│       ├── config/
│       │   ├── config.go            + config_test.go
│       │   ├── resolve.go           + resolve_test.go
│       │   ├── starter.yaml         # //go:embed
│       │   └── testdata/            # fixtures next to tests, per Go convention
│       │       ├── valid.yaml
│       │       ├── empty.yaml
│       │       └── malformed.yaml
│       ├── repos/
│       │   ├── repos.go             + repos_test.go
│       │   └── testdata/            # if needed
│       ├── fzf/
│       │   └── fzf.go               + fzf_test.go
│       ├── proc/
│       │   └── proc.go              + proc_test.go
│       └── platform/
│           ├── platform.go
│           ├── open_darwin.go       # //go:build darwin
│           └── open_linux.go        # //go:build linux
├── scripts/                 # justfile delegates here
│   ├── build.sh
│   ├── install.sh
│   └── release.sh
├── docs/
│   ├── memory/
│   └── specs/
└── fab/
```

**Conventions**:
- Module path: `github.com/sahil87/repo`, with `go.mod` rooted at `src/`. Tests import as `github.com/sahil87/repo/internal/config` etc.
- All source under `src/` (mirrors `~/code/sahil87/fab-kit/src/go/wt`).
- `cmd/repo/` (not flat `cmd/`) — canonical Go layout, supports `go install github.com/sahil87/repo/cmd/repo@latest`.
- Tests live adjacent to source files (`config.go` + `config_test.go`).
- `testdata/` directories live *next to the tests that use them* (per-package), not centralized. Go's `go test` auto-excludes any `testdata/` from package compilation.
- Cobra is the CLI framework. Imported in `src/go.mod`. Already a project standard (used in `fab-kit/src/go/wt/go.mod`).

**Package responsibilities**:

| Package | Responsibility | Key types/functions |
|---|---|---|
| `cmd/repo` | Cobra command definitions, flag parsing, exit codes. One file per subcommand. `root.go` holds the cobra root command and global flags. `main.go` is a 5-line entrypoint. | `var rootCmd = &cobra.Command{...}` |
| `internal/config` | `repos.yaml` location resolution + YAML loading. Embeds `starter.yaml` for `repo config init`. | `func Resolve() (string, error)`, `func Load(path string) (*Config, error)`, `//go:embed starter.yaml` |
| `internal/repos` | In-memory repo model + queries (matching, listing). Consumes a `*Config` from `internal/config`. | `type Repo struct{Name, Dir, URL, Path string}`, `func (rs Repos) MatchOne(query string) []Repo`, `func (rs Repos) List() []Repo` |
| `internal/fzf` | Fzf wrapper. Shells out via `internal/proc`. | `func Pick(items []string, query string) (string, error)` |
| `internal/proc` | Centralized `exec.CommandContext` wrapper. *All* exec calls in the codebase MUST go through this package. Per Constitution Principle I "Security First". | `func Run(ctx context.Context, name string, args ...string) ([]byte, error)`, `func RunInteractive(ctx context.Context, name string, args ...string) error` |
| `internal/platform` | Cross-platform abstractions, isolated behind build tags. Today: just `Open(path string) error` for Darwin/Linux divergence. | `func Open(path string) error` (defined in `open_darwin.go` and `open_linux.go`) |

**Cross-platform build tags**:
- `open_darwin.go` starts with `//go:build darwin`
- `open_linux.go` starts with `//go:build linux`
- Build matrix for goreleaser: darwin-arm64, darwin-amd64, linux-arm64, linux-amd64. Windows is explicitly *not* supported per the constitution's `Cross-Platform Behavior` section.

**Wrapper boundaries** (per Constitution Principle IV "Wrap, Don't Reinvent"):
- `git` → wrapped in `internal/proc.Run` (not a dedicated package — too thin)
- `fzf` → wrapped in `internal/fzf` because the invocation is non-trivial (multiple flags, stdin piping, query prefill)
- `code`, `open`, `xdg-open` → wrapped via `internal/platform.Open` and `internal/proc.Run`
- YAML → use a Go YAML library (`gopkg.in/yaml.v3` is conventional). Do not parse by hand.

**Required GIVEN/WHEN/THEN scenarios**:
- Build for darwin-arm64 succeeds with only darwin-tagged files
- Build for linux-amd64 succeeds with only linux-tagged files
- All `exec.CommandContext` calls go through `internal/proc` (enforceable via grep audit)
- No package outside `internal/proc` imports `os/exec` directly

**Git operations** (resolved):
- For v0.0.1, git calls happen inline in `cmd/repo/clone.go` via `internal/proc.Run("git", "clone", url, dest)`. No dedicated `internal/git/` package — premature abstraction. The git surface for v0.0.1 is exactly one operation (`git clone`) plus a filesystem check (`<path>/.git` exists), which is not even a git call.
- If the git surface grows (e.g., `git fetch`, `git pull`, `git status`), promote to `internal/git/` then. Not now.

**`[NEEDS CLARIFICATION]` markers**:
- (none remaining for this spec)

#### 1d. `docs/specs/build-and-release.md`

Build system, release pipeline, distribution.

**Justfile recipes** (per Constitution Principle V "Thin Justfile, Fab-Kit Build Pattern" — recipes are one-liners, logic lives in `scripts/`):

```just
default:
    @just --list

build:
    ./scripts/build.sh

install:
    ./scripts/install.sh

test:
    cd src && go test ./...

release tag:
    ./scripts/release.sh {{tag}}
```

**`scripts/build.sh`**:
- Wraps `go build` with version injection via `-ldflags "-X main.version=$(git describe --tags --always)"`
- Outputs to `./bin/repo` (gitignored)
- Used for local development

**`scripts/install.sh`**:
- Calls `build.sh`
- Copies `./bin/repo` to a local cache (e.g., `~/.local/bin/repo` or symlink-friendly path)

**`scripts/release.sh`**:
- Validates the tag matches `v[0-9]+\.[0-9]+\.[0-9]+`
- Calls `git tag` and `git push --tags`
- The push triggers GitHub Actions

**`.goreleaser.yaml`**:
- Builds for: darwin-arm64, darwin-amd64, linux-arm64, linux-amd64
- Archive format: `tar.gz` with naming `repo_{{ .Version }}_{{ .Os }}_{{ .Arch }}`
- Publishes a GitHub Release on `v*` tags
- Updates `homebrew-tap` (the `sahil87/homebrew-tap` repo, see `~/code/bootstrap/dotfiles/repos.yaml` line 11) via the goreleaser homebrew integration

**`.github/workflows/release.yml`**:
- Trigger: push of tag matching `v*`
- Steps: checkout, setup-go, run goreleaser
- Required secrets: `GITHUB_TOKEN` (default), `HOMEBREW_TAP_TOKEN` (for cross-repo push to homebrew-tap)

**Initial release**: v0.0.1. Reasoning: Go implementation is unproven on day one; reserve v1.0.0 for "has run in production for ~2 weeks without friction." User explicitly chose 0.0.1.

**Required GIVEN/WHEN/THEN scenarios**:
- `git tag v0.0.1 && git push --tags` → GitHub Release exists with 4 binaries
- After release publish, `brew install sahil87/tap/repo` works
- `repo --version` and `repo -v` both print the tagged version

**Version reporting** (resolved):
- `repo -v` and `repo --version` MUST both work and print the version string (the value of `rootCmd.Version`, populated at build time via `-ldflags "-X main.version=$(git describe --tags --always)"`).
- The cobra-default `repo version` subcommand MAY exist (no effort spent to suppress cobra's auto-wired subcommand). The flags are the documented form; the subcommand is undocumented but tolerated.
- Output format: bare version string on stdout (e.g., `v0.0.1` or `v0.0.1-2-gabc123` for dev builds), exit 0.

**`[NEEDS CLARIFICATION]` markers**:
- Where is `HOMEBREW_TAP_TOKEN` provisioned? (Out of scope — but flag for setup checklist.)

---

### 2. Update `docs/specs/index.md`

Add 4 rows to the existing empty table:

```markdown
| Spec | Description |
|------|-------------|
| [cli-surface](cli-surface.md) | Subcommands, args, flags, exit codes, stdout/stderr conventions |
| [config-resolution](config-resolution.md) | `repos.yaml` search order, schema, `repo config init` flow |
| [architecture](architecture.md) | `src/` layout, Go package responsibilities, cross-platform strategy |
| [build-and-release](build-and-release.md) | Justfile, scripts/, goreleaser, GitHub Actions, homebrew-tap |
```

Preserve the existing prose at the top of `index.md` (lines 1–13).

## Affected Memory

This change is purely a planning change — no implementation, no behavioral memory to record.

- (none)

The actual Go implementation will be a separate downstream change. *That* change will create memory files under `docs/memory/cli/`, `docs/memory/config/`, etc., based on the specs produced here.

## Impact

**Files created**:
- `docs/specs/cli-surface.md`
- `docs/specs/config-resolution.md`
- `docs/specs/architecture.md`
- `docs/specs/build-and-release.md`

**Files modified**:
- `docs/specs/index.md` — append 4 rows to the empty table

**No code touched.** No `src/`, no `cmd/`, no Go files. The Go binary work happens in a downstream change.

**Constitution status**: `fab/project/constitution.md` has already been updated (out-of-band, before this intake was finalized) to its post-edit state — 6 principles, 2 additional constraints, version 1.1.0. The executing agent SHOULD treat the constitution as authoritative input, not as something to modify.

**Downstream consumers of this change**:
- The next change ("implement Go binary v0.0.1") will read all 4 specs and the constitution as authoritative inputs.
- `/fab-continue` at the spec stage of *that* change will scan these specs to derive tasks.

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
| 17 | Confident | This change creates 4 spec files (`cli-surface.md`, `config-resolution.md`, `architecture.md`, `build-and-release.md`) and updates the specs index; no Go code is written, no constitution edits are pending (already applied) | Discussed — user explicitly asked for "subcommands, folder structure etc in docs/specs"; constitution edits applied out-of-band before this intake was finalized; implementation is a separate downstream change | S:95 R:60 A:95 D:90 |
| 18 | Confident | Help text mirrors the existing bash `_usage` (lines 6–32 of `~/code/bootstrap/dotfiles/bin/repo`), updated for renamed subcommands | Pattern preservation — bash version is the v1 behavioral contract | S:80 R:75 A:85 D:85 |
| 19 | Confident | Match resolution algorithm preserves bash `_resolve_row` behavior: case-insensitive substring on name, exactly-1-match short-circuit, fzf with `--query` and `--select-1` otherwise | Pattern preservation — explicitly required for behavioral parity with bash | S:85 R:60 A:90 D:85 |
| 20 | Confident | YAML library: `gopkg.in/yaml.v3` (Go ecosystem standard) | Constitution Principle IV says "use a Go YAML library, do not parse by hand"; v3 is the conventional choice | S:80 R:75 A:90 D:80 |
| 21 | Confident | Goreleaser build matrix: darwin-arm64, darwin-amd64, linux-arm64, linux-amd64 (no Windows) | Constitution Cross-Platform Behavior section lists these four targets explicitly | S:90 R:80 A:95 D:95 |
| 22 | Certain | `repo -v` and `repo --version` MUST both work (flag form is the documented version interface). The cobra-default `version` subcommand MAY exist (no effort to suppress). Version string injected at build time via `-ldflags "-X main.version=$(git describe --tags --always)"`. | Discussed — user explicitly required `-v`/`--version` to work and stated the `version` subcommand is optional. Pattern reference: wt currently has no version handling, so this establishes the convention. | S:95 R:85 A:90 D:90 |
| 23 | Certain | `repos.yaml` file mode 0644 on `repo config init` | Discussed — user explicitly chose 0644. File contains repo paths and git URLs, none of which are credentials. | S:100 R:90 A:95 D:95 |
| 24 | Certain | `$REPOS_YAML` set but file missing → hard error (do not fall through). Print: `repo: $REPOS_YAML points to <path>, which does not exist. Set $REPOS_YAML to an existing file or unset it.` Exit 1. | Discussed — user explicitly chose hard error. Setting an env var is a declaration of intent; fall-through would mask config bugs. | S:100 R:80 A:90 D:95 |
| 25 | Certain | `fzf` missing → error with install hint and exit 1, but ONLY when fzf would actually be invoked. Subcommands that resolve without fzf (exact-match name, `repo ls`, `repo shell-init zsh`, `repo config init`, `repo config path`) MUST NOT check for fzf and MUST NOT error if missing. Same pattern applies to `git` (only error during clone), `code` (only error during `repo code`), `open`/`xdg-open` (only error during `repo open`). | Discussed — user chose option 1 (error with install hint). Edge-case refinement applied: scope the dependency check to actual invocation, don't preemptively fail. | S:95 R:75 A:85 D:90 |
| 26 | Certain | No `internal/git/` package for v0.0.1 — git calls happen inline in `cmd/repo/clone.go` via `internal/proc.Run("git", "clone", url, dest)`. Promote to a package only if the git surface grows beyond the single `clone` operation. | Discussed — user explicitly chose inline ("don't optimize prematurely"). Current git surface is exactly one operation plus a filesystem check, which doesn't justify a package. | S:100 R:80 A:90 D:95 |

26 assumptions (21 certain, 5 confident, 0 tentative, 0 unresolved).
