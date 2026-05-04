# Architecture

> Repository folder layout, Go package responsibilities, and cross-platform strategy for the `hop` binary.

## Top-Level Repository Layout

```
hop/
├── README.md
├── LICENSE
├── justfile                              # one-line recipes per Constitution Principle V
├── .gitignore                            # ignores ./bin/
├── src/                                  # all Go source (mirrors fab-kit/src/go/wt convention)
│   ├── go.mod                            # module: github.com/sahil87/hop
│   ├── go.sum
│   ├── cmd/
│   │   └── hop/
│   │       ├── main.go                   # entrypoint + translateExit + extractDashC + runDashC
│   │       ├── root.go                   # cobra root + version handling + rootLong help text
│   │       ├── where.go                  # `hop where`, bare `hop <name>`, shared resolver helpers
│   │       ├── code.go                   # `hop code`
│   │       ├── open.go                   # `hop open`
│   │       ├── cd.go                     # `hop cd` (binary form: prints hint + exit 2)
│   │       ├── clone.go                  # `hop clone` (single, --all, ad-hoc URL)
│   │       ├── ls.go                     # `hop ls`
│   │       ├── shell_init.go             # `hop shell-init zsh`
│   │       ├── config.go                 # `hop config init` and `hop config where`
│   │       ├── *_test.go                 # adjacent unit tests per file
│   │       ├── dashc_test.go             # extractDashC argv-split tests
│   │       ├── integration_test.go       # builds the binary and exercises it end-to-end
│   │       └── testutil_test.go          # shared test helpers
│   └── internal/
│       ├── config/
│       │   ├── config.go                 # YAML schema, Load(path), grouped-schema validator
│       │   ├── resolve.go                # Resolve() and ResolveWriteTarget()
│       │   ├── starter.yaml              # //go:embed for `hop config init`
│       │   ├── *_test.go
│       │   └── testdata/
│       │       ├── valid.yaml
│       │       ├── valid-mixed.yaml
│       │       ├── valid-empty-group.yaml
│       │       ├── empty.yaml
│       │       ├── malformed.yaml
│       │       ├── missing-repos.yaml
│       │       ├── invalid-empty-dir.yaml
│       │       ├── invalid-group-name.yaml
│       │       ├── invalid-unknown-top.yaml
│       │       ├── invalid-unknown-group-key.yaml
│       │       ├── invalid-url-collision.yaml
│       │       └── dup-in-group.yaml
│       ├── repos/
│       │   ├── repos.go                  # Repo, Repos, FromConfig, MatchOne, ExpandDir, DeriveName, DeriveOrg
│       │   └── repos_test.go
│       ├── yamled/
│       │   ├── yamled.go                 # AppendURL — comment-preserving, atomic write
│       │   └── yamled_test.go
│       ├── fzf/
│       │   ├── fzf.go                    # Pick(ctx, lines, query) wrapper
│       │   └── fzf_test.go
│       ├── proc/
│       │   ├── proc.go                   # Run, RunInteractive, RunForeground, ExitCode, ErrNotFound
│       │   └── proc_test.go
│       └── platform/
│           ├── platform.go               # package doc only
│           ├── open_darwin.go            # //go:build darwin
│           ├── open_linux.go             # //go:build linux
│           └── platform_test.go
├── scripts/                              # justfile delegates here
│   ├── build.sh
│   ├── install.sh
│   └── release.sh
├── .github/
│   ├── workflows/
│   │   └── release.yml                   # tag-driven release pipeline
│   └── formula-template.rb               # Homebrew formula template (sed-substituted)
├── docs/
│   ├── memory/                           # post-implementation reality (auto-hydrated)
│   └── specs/                            # pre-implementation design (this directory)
└── fab/                                  # fab-kit workflow artifacts
    ├── backlog.md
    ├── changes/
    └── project/
```

## Conventions

| Convention | Value |
|---|---|
| Module path | `github.com/sahil87/hop` |
| `go.mod` location | `src/go.mod` (rooted at `src/`, not repo root — mirrors `fab-kit/src/go/wt`) |
| Subcommand layout | `src/cmd/hop/<verb>.go` (one file per subcommand) |
| Internal packages | `src/internal/<pkg>/` (flat — no nested packages) |
| Tests | Adjacent to source (`config.go` + `config_test.go`) |
| Test fixtures | `testdata/` next to the tests that use them (per-package, not centralized) |
| CLI framework | `github.com/spf13/cobra` v1.8.1 |
| YAML library | `gopkg.in/yaml.v3` |
| Go version | `1.22` |

Tests import packages as `github.com/sahil87/hop/internal/config`, etc.

`go test` automatically excludes any `testdata/` directory from package compilation, so per-package fixtures are the idiomatic placement. Tests load fixtures with relative paths like `os.ReadFile("testdata/valid.yaml")`.

## Package Responsibilities

### `cmd/hop`

Cobra command definitions, flag parsing, exit code handling, the `-C` argv splitter.

| File | Exports / contents |
|---|---|
| `main.go` | `func main()` — builds rootCmd, sets `Version`, captures `rootForCompletion`, runs `extractDashC` (pre-cobra), calls `Execute()`. Defines `translateExit` (sole stderr/exit path), `extractDashC` (argv splitter for `-C`), `runDashC` (resolve + `proc.RunForeground`), and the typed sentinels (`errSilent`, `errFzfMissing`, `errFzfCancelled`, `errExitCode`). Holds the package-level `var version = "dev"` (overridden via `-ldflags "-X main.version=…"`). |
| `root.go` | `func newRootCmd() *cobra.Command` — root command with `RunE` for bare-form (no subcommand or single positional). Sets `Version`, `SilenceUsage = true`, `SilenceErrors = true`. Holds `rootLong` (the help-text Usage table and Notes block) and the `AddCommand` wiring. |
| `where.go` | `func newWhereCmd() *cobra.Command` — `hop where <name>`. Hosts shared helpers: `loadRepos()`, `resolveByName(query)`, `resolveOne(cmd, query)`, `resolveAndPrint(cmd, query)`, `buildPickerLines(rs)`. Also defines `fzfMissingHint`. |
| `code.go` | `func newCodeCmd() *cobra.Command`. Defines `codeMissingHint`. |
| `open.go` | `func newOpenCmd() *cobra.Command`. Calls `platform.Open` and formats the missing-tool stderr using `platform.OpenTool()`. |
| `cd.go` | `func newCdCmd() *cobra.Command` — prints `cdHint` to stderr and returns `errExitCode{code: 2}`. |
| `clone.go` | `func newCloneCmd() *cobra.Command` — handles three forms: `<name>`, `--all`, `<url>`. URL detection via `looksLikeURL`. Holds `cloneTimeout` (10 minutes), `gitMissingHint`. URL form delegates the YAML write to `internal/yamled.AppendURL`. |
| `ls.go` | `func newLsCmd() *cobra.Command` — `cobra.NoArgs`. |
| `shell_init.go` | `func newShellInitCmd() *cobra.Command`. Emits `zshInit` (a Go raw-string constant) followed by the cobra-generated `_hop` completion via `rootForCompletion.GenZshCompletion(out)`. |
| `config.go` | `func newConfigCmd() *cobra.Command` — parent for `init` and `where`; uses `cobra.Command{Use: "config"}` with `AddCommand(newConfigInitCmd(), newConfigWhereCmd())`. |

### `internal/config`

Configuration file resolution and YAML loading. Strict grouped-schema validator.

| Symbol | Signature / purpose |
|---|---|
| `type Config` | `struct { CodeRoot string; Groups []Group }`. `Groups` preserves YAML source order. |
| `type Group` | `struct { Name, Dir string; URLs []string }`. `Dir == ""` → convention-driven flat group. |
| `func Resolve() (string, error)` | Search-order resolution per `config-resolution.md`. Hard-errors on misconfig. |
| `func ResolveWriteTarget() (string, error)` | Same search order, no `os.Stat`. Used by `config init` and `config where`. |
| `func Load(path string) (*Config, error)` | Reads file, parses to `*yaml.Node`, validates schema (top-level keys, group bodies, group name regex, URL uniqueness across and within groups), produces `*Config`. Errors include file path. |
| `func WriteStarter(path string) error` | Writes the embedded `starter.yaml` to `path`. Refuses if the file exists. Creates parent dirs (mode 0755). File mode 0644. Used by `hop config init`. |
| `func StarterContent() []byte` | Exposes the embedded bytes for tests. |
| `var ErrNoConfig` | Sentinel for "no config could be resolved" — exported but currently the actual returned errors use `fmt.Errorf` with the exact user-facing strings. |
| `//go:embed starter.yaml` | Embeds the starter content. |

### `internal/repos`

In-memory repo model and queries. Consumes `*config.Config`.

| Symbol | Signature / purpose |
|---|---|
| `type Repo` | `struct { Name, Group, Dir, URL, Path string }`. `Path = filepath.Join(Dir, Name)`. `Dir` is fully expanded (`~` resolved). `Group` records which group the repo came from. |
| `type Repos []Repo` | Ordered list (preserves YAML source order). |
| `func FromConfig(cfg *config.Config) (Repos, error)` | Walks `cfg.Groups`, applies path resolution per the schema rules (map-shaped `<dir>/<name>`, flat `<code_root>/<org>/<name>`), strips `.git` from URL basenames to derive names. |
| `func (rs Repos) MatchOne(query string) Repos` | Returns case-insensitive substring matches on `Name`. |
| `func ExpandDir(dir, codeRootHint string) string` | `~`/relative/absolute resolution. Relative + non-empty hint → joined with the *expanded* hint. |
| `func DeriveName(url string) string` | Last `/`-separated component of URL (after stripping `.git`). |
| `func DeriveOrg(url string) string` | Path between `host:` (SSH) or `host/` (HTTPS) and the last component. |

### `internal/yamled`

Comment-preserving YAML write-back, used by `hop clone <url>` to auto-register URLs.

| Symbol | Signature / purpose |
|---|---|
| `func AppendURL(path, group, url string) error` | Reads `path`, parses to `*yaml.Node`, navigates `repos.<group>`, appends to either the sequence body (flat group) or the `urls:` child sequence (map-shaped group). Marshals and atomically writes via temp file + `os.Rename` in the same directory. |
| `var ErrGroupNotFound` | Wrapped via `%w` when the named group is absent. Detect with `errors.Is(err, yamled.ErrGroupNotFound)`. |

**Contract**: comments are preserved through the yaml.v3 round-trip. **Indentation is normalized to yaml.v3 defaults** — byte-perfect formatting is *not* guaranteed.

Why a separate package from `internal/config`: `config` validates and consumes; `yamled` produces a node tree, navigates, mutates, writes. Separating concerns keeps each independently testable and avoids entangling load-time validation with write-time edits.

### `internal/fzf`

Fzf wrapper.

| Symbol | Signature / purpose |
|---|---|
| `func Pick(ctx context.Context, lines []string, query string) (string, error)` | Pipes `lines` to fzf via stdin (joined with `\n`); returns the selected line. Argv built by `buildArgs(query)`: `--query <q>` (omitted when empty), `--select-1 --height 40% --reverse --with-nth 1 --delimiter '\t'`. All exec goes through `proc.RunInteractive`. Returns `proc.ErrNotFound` for "fzf not installed" so callers can produce the install hint. |

A package-level `var runInteractive = proc.RunInteractive` provides a test seam for asserting argv composition without spawning fzf.

### `internal/proc`

Centralized `exec.CommandContext` wrapper. **All** subprocess invocations in production code MUST go through this package — enforced by Constitution Principle I (Security First) and verified at apply time via grep audit:

```
grep --include='*.go' --exclude='*_test.go' -rn '"os/exec"' src/internal/ src/cmd/
# → matches restricted to src/internal/proc/

grep --include='*.go' --exclude='*_test.go' -rn 'exec\.Command\b' src/
# → zero matches (only exec.CommandContext is permitted)
```

Test files MAY use `os/exec` directly — to spawn the built binary in integration tests, or to set up local git fixtures (e.g., `git init --bare` for ad-hoc URL clone tests). The audits scope to non-test code.

| Symbol | Signature / purpose |
|---|---|
| `func Run(ctx, name, args...) ([]byte, error)` | Non-interactive. Captures stdout to bytes; stderr passes through to parent. Used for `git`, `code`, `open`/`xdg-open`. |
| `func RunInteractive(ctx, stdin io.Reader, name, args...) (string, error)` | Pipes stdin, captures stdout to string; stderr passes through. Used for `fzf`. |
| `func RunForeground(ctx, dir, name, args...) (int, error)` | Runs a child with `cmd.Dir = dir` and stdin/stdout/stderr inherited. Returns the child's exit code on success (error nil); `(-1, ErrNotFound)` if the binary is missing; `(-1, err)` for other failures. Used by `hop -C`. |
| `var ErrNotFound` | Sentinel returned when the binary is not on PATH. Callers use `errors.Is(err, proc.ErrNotFound)`. |
| `func ExitCode(err error) (int, bool)` | Helper to extract a child's exit code from `*exec.ExitError` without callers importing `os/exec`. |

Default timeouts: callers MUST construct a `context.Context` with a timeout for non-interactive ops (e.g., 5s for read-only ops, 10 minutes for `git clone`). Interactive operations (fzf) and `-C` use `context.Background()` (no timeout) because the user is at the keyboard or running an arbitrary child.

### `internal/platform`

Cross-platform abstractions, isolated behind build tags.

| Symbol | Signature / purpose |
|---|---|
| `func Open(ctx, path string) error` | Opens `path` in the OS file manager. Implementation in `open_darwin.go` (`open <path>`) and `open_linux.go` (`xdg-open <path>`). Both delegate to `internal/proc.Run`. |
| `func OpenTool() string` | Returns `"open"` (Darwin) or `"xdg-open"` (Linux). Used by `cmd/hop/open.go` to format the missing-tool stderr without knowing which OS it's on. |

`open_darwin.go` starts with `//go:build darwin`; `open_linux.go` starts with `//go:build linux`. Other platforms (Windows) fail at link time — by design (Constitution Cross-Platform Behavior).

`platform.go` declares the package only (no exported symbols; its job is to host the build-tagged files and the package doc).

## Wrapper Boundaries

Per Constitution Principle IV ("Wrap, Don't Reinvent"):

| External tool | Wrapper |
|---|---|
| `git` | Inline `internal/proc.Run("git", "clone", ...)` calls in `cmd/hop/clone.go`. No dedicated `internal/git/` package — premature for two operations (`git clone` for registered names, `git clone` for ad-hoc URLs). |
| `fzf` | `internal/fzf.Pick` — non-trivial invocation (multiple flags, stdin piping, query prefill), used by 5+ subcommands. Worth one file. |
| `code` | Inline `internal/proc.Run("code", path)` in `cmd/hop/code.go`. |
| `open` / `xdg-open` | `internal/platform.Open` — wrapped because the choice is platform-specific. |
| `<cmd>` (for `-C`) | Inline `internal/proc.RunForeground(...)` in `cmd/hop/main.go::runDashC`. |
| YAML (read) | `gopkg.in/yaml.v3` directly in `internal/config/config.go`. Not re-wrapped — yaml.v3 already is the wrapper. |
| YAML (write-back) | `internal/yamled.AppendURL` — wrapped because comment-preserving write requires node-level navigation, atomic temp+rename, and an `ErrGroupNotFound` sentinel. |

### Why `internal/git/` does NOT exist

The entire git surface is `git clone <url> <dest>` (in single, `--all`, and ad-hoc URL paths). Wrapping one operation in a 5-line package is premature abstraction. If the surface grows (e.g., `git fetch`, `git pull`, `git status`), promote to a package then.

## Composability Primitives

The grouped-schema rename introduced two primitives that other operations build on:

- **`hop where <name>`** — path resolver. Stdin/stdout-friendly: `cd "$(hop where outbox)"` works as a shell composition. The bare form `hop <name>` does the same thing.
- **`hop -C <name> <cmd>...`** — exec-in-context. `git -C`-style: run a child command with cwd set to the resolved repo dir, without leaving the parent shell's cwd changed.

Future verbs (`sync`, `autosync`, `features`) build on these rather than each one re-implementing path resolution and exec.

## Cross-Platform Strategy

| Platform | Status |
|---|---|
| darwin-arm64 | Supported |
| darwin-amd64 | Supported |
| linux-arm64 | Supported |
| linux-amd64 | Supported |
| Windows | Not supported (per Constitution Cross-Platform Behavior section) |

Platform-specific code is isolated to `internal/platform/` via build tags. The rest of the codebase is platform-agnostic. A `go build` on any supported OS picks the right `open_*.go` automatically.

### Verification

> **GIVEN** the source tree
> **WHEN** I run `cd src && GOOS=darwin GOARCH=arm64 go build ./...`
> **THEN** the build succeeds using only `open_darwin.go`

> **GIVEN** the source tree
> **WHEN** I run `cd src && GOOS=linux GOARCH=amd64 go build ./...`
> **THEN** the build succeeds using only `open_linux.go`

## Security Boundary

Per Constitution Principle I ("Security First"):

1. **All subprocess invocations go through `internal/proc`.** No production package outside `internal/proc/` MAY import `os/exec` directly. Verifiable: `grep --include='*.go' --exclude='*_test.go' -rn '"os/exec"' src/cmd src/internal/{config,repos,fzf,platform,yamled}` returns nothing.
2. **All `proc.Run`/`proc.RunInteractive`/`proc.RunForeground` calls use `exec.CommandContext` with explicit argument slices.** Never shell strings, never `exec.Command`. Verifiable: `grep --include='*.go' --exclude='*_test.go' -rn 'exec\.Command\b' src/` returns zero hits.
3. **User input is validated (or passed as a single argv element) before reaching subprocess.** Repo names from `hop.yaml` are extracted via URL-basename split (no shell metachars survive). Search queries from CLI args are passed to fzf via stdin (the candidate list) and `--query <q>` (a single arg), eliminating shell-injection paths. The `-C` child argv is forwarded as a slice to `proc.RunForeground`, never concatenated into a string.
4. **Atomic file writes for config edits.** `internal/yamled.AppendURL` writes to a temp file in the same directory and `os.Rename`s into place — preserving the original on rename failure.

> **GIVEN** a repo URL `git@github.com:user/hop;ls.git`
> **WHEN** any subcommand resolves the repo
> **THEN** the derived name is `hop;ls` (last `/`-component, `.git` stripped)
> **AND** if used as an arg to `git clone`, it's passed as a literal arg (not shell-evaluated) — `git` will reject the URL or clone into a directory named `hop;ls`, but no shell injection occurs

## Design Decisions

1. **`src/` rooted module, not repo root.** Mirrors `fab-kit/src/go/wt` — the convention for sahil87 Go binaries. Reserves repo root for non-Go artifacts (justfile, scripts, docs, GitHub workflows).
2. **Cobra over hand-rolled dispatch.** Cobra is already a dependency in `wt`, so weight is amortized. The subcommand count plus per-subcommand flags justifies the dep.
3. **Flat `internal/<pkg>/` layout.** No `internal/cli/`, no nested packages. Each package has one job: config, repos, fzf, proc, platform, yamled.
4. **`testdata/` per-package, not centralized.** Idiomatic Go layout — `go test` auto-excludes any `testdata/` from package compilation, and tests load fixtures with simple relative paths.
5. **`internal/proc/` is the security choke point.** Centralizing exec lets the security audit be a single grep, not a code review of every call site.
6. **No `internal/git/` package yet.** Two operations (`git clone <name>` and `git clone <url>`) don't justify a package — both are inline `proc.Run("git", "clone", ...)`. Promote later if `fetch`/`pull`/`status` get added.
7. **`internal/yamled` is separate from `internal/config`.** Validator and mutator have different responsibilities and different test surfaces. Keeping them separate is worth the extra package.
8. **`-C` argv splitting in `main.go::extractDashC`, pre-cobra.** Cobra's parser would interpret the child's flags as `hop`'s flags. Pre-Execute argv inspection is a single small function, unit-tested in `dashc_test.go`. The alternative — `cobra.Command{DisableFlagParsing: true}` on a `-C` subcommand — would require `-C` to be a subcommand rather than a flag, breaking the `git -C`-style ergonomics.
9. **`rootForCompletion` is a package-level var.** `shell-init zsh` needs `rootCmd` to call `GenZshCompletion`, but threading `rootCmd` through every factory clutters the wiring. `main()` captures it once after `newRootCmd()`; `shell_init.go` reads it. Acceptable singleton for this binary's scale.
