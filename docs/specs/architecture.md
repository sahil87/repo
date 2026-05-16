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
│   │       ├── main.go                   # entrypoint + translateExit + extractDashR + runDashR
│   │       ├── root.go                   # cobra root + version + rootLong + repo-verb dispatch (RunE) + cdHint/bareNameHint/toolFormHintFmt constants
│   │       ├── resolve.go                # bare `hop <name>` resolver helpers (loadRepos, resolveByName, resolveWorktreePath, resolveOne, resolveAndPrint, buildPickerLines) — called by root's RunE for `hop <name> where`; first-`/` split for `<name>/<wt>` grammar
│   │       ├── wt_list.go                # `WtEntry` JSON contract + `listWorktrees` package-level var seam + `wtListTimeout = 5 * time.Second`; used by `resolveWorktreePath` and `runLsTrees`
│   │       ├── clone.go                  # `hop clone` (single, --all, ad-hoc URL)
│   │       ├── ls.go                     # `hop ls` + `--trees` flag (`runLsTrees` fans `wt list --json` across cloned repos)
│   │       ├── shell_init.go             # `hop shell-init <shell>` (zsh + bash, shared posixInit)
│   │       ├── config.go                 # `hop config init` and `hop config where`
│   │       ├── update.go                 # `hop update` (delegates to `internal/update`)
│   │       ├── *_test.go                 # adjacent unit tests per file
│   │       ├── dashr_test.go             # extractDashR argv-split tests
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
│       └── update/
│           ├── update.go                 # Run(version) — Homebrew self-update
│           └── update_test.go
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

Cobra command definitions, flag parsing, exit code handling, the `-R` argv splitter.

| File | Exports / contents |
|---|---|
| `main.go` | `func main()` — builds rootCmd, sets `Version`, captures `rootForCompletion`, runs `extractDashR` (pre-cobra), calls `Execute()`. Defines `translateExit` (sole stderr/exit path), `extractDashR` (argv splitter for `-R`), `runDashR` (resolve + `proc.RunForeground`), and the typed sentinels (`errSilent`, `errFzfMissing`, `errFzfCancelled`, `errExitCode`). Holds the package-level `var version = "dev"` (overridden via `-ldflags "-X main.version=…"`). |
| `root.go` | `func newRootCmd() *cobra.Command` — root command with `Args: cobra.MaximumNArgs(2)` and a `RunE` that implements the repo-verb grammar at $1/$2 (0 args → bare picker; 1 arg → bareNameHint exit-2; 2 args → switch on $2: `where` resolves, `cd` errors with `cdHint`, anything-else errors with `fmt.Sprintf(toolFormHintFmt, $2)`). Sets `Version`, `SilenceUsage = true`, `SilenceErrors = true`. Holds `rootLong` (the help-text Usage table and Notes block), the three hint constants (`bareNameHint`, `cdHint`, `toolFormHintFmt`), and the `AddCommand` wiring. The `cd` and `where` subcommand factories were removed in the repo-verb grammar flip — verbs live at $2, not $1. |
| `resolve.go` | Resolution helpers shared across the root command and `clone`. Exports: `loadRepos()`, `resolveByName(query)`, `resolveWorktreePath(repo, wtName)`, `resolveOne(cmd, query)`, `resolveAndPrint(cmd, query)`, `buildPickerLines(rs)`. Also defines `fzfMissingHint`, `wtMissingHint`, `errFzfCancelled`, `errFzfMissing`, `errSilent`. `resolveByName` splits on the first `/` for the `<name>/<wt>` grammar (change `7eab`) — empty LHS/RHS yields `*errExitCode{code: 2}`; otherwise recurses on the LHS, then delegates to `resolveWorktreePath` which guards against uncloned repos, invokes `listWorktrees` (from `wt_list.go`), and returns a shallow-copied `*Repo` whose `Path` is the worktree's. Renamed from `where.go` when the `hop where <name>` subcommand was removed (the file no longer hosts a cobra factory — just the helpers). |
| `wt_list.go` | `WtEntry` struct (`{Name, Branch, Path, IsMain, IsCurrent, Dirty, Unpushed}`), `wtListTimeout = 5 * time.Second` constant, `var listWorktrees = defaultListWorktrees` package-level seam, `defaultListWorktrees(ctx, repoPath)` and `unmarshalWtEntries(out)`. Wraps a single `proc.RunCapture` invocation of `wt list --json` with a 5-second per-call timeout (matching `internal/scan`'s `git remote` precedent) and a `[]WtEntry` unmarshal. The seam pattern mirrors `internal/fzf/fzf.go::runInteractive` — tests inject fakes without spawning a real `wt`. Two production consumers: `resolveWorktreePath` and `runLsTrees`. Helper stays inline in `cmd/hop/` rather than a dedicated `internal/wt/` package — two call sites is below the threshold, same "promote later" rationale as `internal/git/`. |
| `clone.go` | `func newCloneCmd() *cobra.Command` — handles three forms: `<name>`, `--all`, `<url>`. URL detection via `looksLikeURL`. Holds `cloneTimeout` (10 minutes), `gitMissingHint`. URL form delegates the YAML write to `internal/yamled.AppendURL`. |
| `ls.go` | `func newLsCmd() *cobra.Command` — `cobra.NoArgs` plus a `--trees` boolean flag. Default path (`runLsPlain`) is unchanged from pre-`7eab`. With `--trees`, `runLsTrees` fans `wt list --json` across configured repos in source order via `listWorktrees`, emitting per-row `(not cloned)`, `(wt list failed: <err>)`, or `{N} tree(s)  (<wt-list>)` summaries. The FIRST `proc.ErrNotFound` from wt aborts the run with the `wtMissingHint`. Defines `wtDirtyGlyph = "*"` and `wtUnpushedGlyph = "↑"` constants. |
| `shell_init.go` | `func newShellInitCmd() *cobra.Command`. Emits the shared `posixInit` raw-string constant (defines `hop()`, `_hop_dispatch()`, `h()`, `hi()`; tool-form dispatch built in) followed by cobra-generated completion: `rootForCompletion.GenZshCompletion(out)` + `compdef _hop h hi` for zsh, `rootForCompletion.GenBashCompletionV2(out, true)` + `complete -o default -F __start_hop h hi` for bash. |
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
| `func RunForeground(ctx, dir, name, args...) (int, error)` | Runs a child with `cmd.Dir = dir` and stdin/stdout/stderr inherited. Returns the child's exit code on success (error nil); `(-1, ErrNotFound)` if the binary is missing; `(-1, err)` for other failures. Used by `hop -R` (and the shim's tool-form sugar). |
| `var ErrNotFound` | Sentinel returned when the binary is not on PATH. Callers use `errors.Is(err, proc.ErrNotFound)`. |
| `func ExitCode(err error) (int, bool)` | Helper to extract a child's exit code from `*exec.ExitError` without callers importing `os/exec`. |

Default timeouts: callers MUST construct a `context.Context` with a timeout for non-interactive ops (e.g., 5s for read-only ops, 10 minutes for `git clone`). Interactive operations (fzf) and `-R` use `context.Background()` (no timeout) because the user is at the keyboard or running an arbitrary child.

## Wrapper Boundaries

Per Constitution Principle IV ("Wrap, Don't Reinvent"):

| External tool | Wrapper |
|---|---|
| `git` | Inline `internal/proc.Run("git", "clone", ...)` calls in `cmd/hop/clone.go`. No dedicated `internal/git/` package — premature for two operations (`git clone` for registered names, `git clone` for ad-hoc URLs). |
| `fzf` | `internal/fzf.Pick` — non-trivial invocation (multiple flags, stdin piping, query prefill), used by 5+ subcommands. Worth one file. |
| `<cmd>` (for `-R`) | Inline `internal/proc.RunForeground(...)` in `cmd/hop/main.go::runDashR`. The shim's tool-form sugar (`hop <name> <tool>`) and canonical exec form (`hop <name> -R <cmd>`) both rewrite to `-R`, so they share this exec path. Cross-platform `open`/`xdg-open` is the user's responsibility via tool-form. |
| `wt open` | Inline `internal/proc.RunForeground(ctx, "", "wt", "open", repo.Path)` in `cmd/hop/open.go::runOpen`. Single operation; the binary is a transparent passthrough — env-var orchestration (`WT_CD_FILE`, `WT_WRAPPER`) lives in the shim. |
| `wt list --json` | `cmd/hop/wt_list.go::defaultListWorktrees` wraps `internal/proc.RunCapture` with a 5-second per-call timeout and a `[]WtEntry` JSON unmarshal. The helper is exposed via a package-level `var listWorktrees` seam (mirroring `internal/fzf/fzf.go::runInteractive`) so tests can inject fakes. Two call sites (`resolveWorktreePath`, `runLsTrees`) — below the threshold for a dedicated `internal/wt/` package; same "promote later" rationale as `internal/git/`. |
| YAML (read) | `gopkg.in/yaml.v3` directly in `internal/config/config.go`. Not re-wrapped — yaml.v3 already is the wrapper. |
| YAML (write-back) | `internal/yamled.AppendURL` — wrapped because comment-preserving write requires node-level navigation, atomic temp+rename, and an `ErrGroupNotFound` sentinel. |

### Why `internal/git/` does NOT exist

The entire git surface is `git clone <url> <dest>` (in single, `--all`, and ad-hoc URL paths). Wrapping one operation in a 5-line package is premature abstraction. If the surface grows (e.g., `git fetch`, `git pull`, `git status`), promote to a package then.

## Composability Primitives

The grouped-schema rename introduced two primitives that other operations build on:

- **`hop <name> where`** — path resolver. Stdin/stdout-friendly: `cd "$(hop outbox where)"` works as a shell composition. The repo-verb grammar puts the repo first, the verb second; the bare `hop <name>` (1 arg) is the shim-only shorthand for `hop <name> cd`, not for the path-printer.
- **`hop <name> -R <cmd>...`** — exec-in-context. Repo-scoped: run a child command with cwd set to the resolved repo dir, without leaving the parent shell's cwd changed. Spelled `-R` (not `-C` like `git -C` / `make -C`) because hop is **repo-scoped**, not arbitrary-directory-scoped — the resolution path goes through `resolveByName` which only resolves repos in `hop.yaml`. Shim-only user-facing form; the shim rewrites to the binary's internal `command hop -R <name> <cmd>...` shape.
- **`hop <name> <tool> [args...]`** — shim-only tool-form sugar. Rewrites to `command hop -R <name> <tool> [args...]`. Lives in `shell_init.go::posixInit`, NOT the binary. See [cli-surface.md](cli-surface.md#hop-name-tool-shim-sugar) for the resolution ladder.

Future verbs (`sync`, `autosync`, `features`) build on these rather than each one re-implementing path resolution and exec.

## Cross-Platform Strategy

| Platform | Status |
|---|---|
| darwin-arm64 | Supported |
| darwin-amd64 | Supported |
| linux-arm64 | Supported |
| linux-amd64 | Supported |
| Windows | Not supported (per Constitution Cross-Platform Behavior section) |

There is no platform-specific Go code: cross-platform divergence (e.g., Darwin's `open` vs. Linux's `xdg-open`) is handled at the user layer via tool-form (`hop <name> open` on Darwin, `hop <name> xdg-open` on Linux). The rest of the codebase is platform-agnostic.

### Verification

> **GIVEN** the source tree
> **WHEN** I run `cd src && GOOS=darwin GOARCH=arm64 go build ./...`
> **THEN** the build succeeds

> **GIVEN** the source tree
> **WHEN** I run `cd src && GOOS=linux GOARCH=amd64 go build ./...`
> **THEN** the build succeeds

## Security Boundary

Per Constitution Principle I ("Security First"):

1. **All subprocess invocations go through `internal/proc`.** No production package outside `internal/proc/` MAY import `os/exec` directly. Verifiable: `grep --include='*.go' --exclude='*_test.go' -rn '"os/exec"' src/cmd src/internal/{config,repos,fzf,update,yamled}` returns nothing.
2. **All `proc.Run`/`proc.RunInteractive`/`proc.RunForeground` calls use `exec.CommandContext` with explicit argument slices.** Never shell strings, never `exec.Command`. Verifiable: `grep --include='*.go' --exclude='*_test.go' -rn 'exec\.Command\b' src/` returns zero hits.
3. **User input is validated (or passed as a single argv element) before reaching subprocess.** Repo names from `hop.yaml` are extracted via URL-basename split (no shell metachars survive). Search queries from CLI args are passed to fzf via stdin (the candidate list) and `--query <q>` (a single arg), eliminating shell-injection paths. The `-R` child argv is forwarded as a slice to `proc.RunForeground`, never concatenated into a string.
4. **Atomic file writes for config edits.** `internal/yamled.AppendURL` writes to a temp file in the same directory and `os.Rename`s into place — preserving the original on rename failure.

> **GIVEN** a repo URL `git@github.com:user/hop;ls.git`
> **WHEN** any subcommand resolves the repo
> **THEN** the derived name is `hop;ls` (last `/`-component, `.git` stripped)
> **AND** if used as an arg to `git clone`, it's passed as a literal arg (not shell-evaluated) — `git` will reject the URL or clone into a directory named `hop;ls`, but no shell injection occurs

## Design Decisions

1. **`src/` rooted module, not repo root.** Mirrors `fab-kit/src/go/wt` — the convention for sahil87 Go binaries. Reserves repo root for non-Go artifacts (justfile, scripts, docs, GitHub workflows).
2. **Cobra over hand-rolled dispatch.** Cobra is already a dependency in `wt`, so weight is amortized. The subcommand count plus per-subcommand flags justifies the dep.
3. **Flat `internal/<pkg>/` layout.** No `internal/cli/`, no nested packages. Each package has one job: config, repos, fzf, proc, yamled, update.
4. **`testdata/` per-package, not centralized.** Idiomatic Go layout — `go test` auto-excludes any `testdata/` from package compilation, and tests load fixtures with simple relative paths.
5. **`internal/proc/` is the security choke point.** Centralizing exec lets the security audit be a single grep, not a code review of every call site.
6. **No `internal/git/` package yet.** Two operations (`git clone <name>` and `git clone <url>`) don't justify a package — both are inline `proc.Run("git", "clone", ...)`. Promote later if `fetch`/`pull`/`status` get added.
7. **`internal/yamled` is separate from `internal/config`.** Validator and mutator have different responsibilities and different test surfaces. Keeping them separate is worth the extra package.
8. **`-R` argv splitting in `main.go::extractDashR`, pre-cobra.** Cobra's parser would interpret the child's flags as `hop`'s flags. Pre-Execute argv inspection is a single small function, unit-tested in `dashr_test.go`. The alternative — `cobra.Command{DisableFlagParsing: true}` on a `-R` subcommand — would require `-R` to be a subcommand rather than a flag, breaking the flag-style ergonomics. Spelled `-R` (not `-C` like `git -C` / `make -C` / `tar -C`) because hop is repo-scoped (resolves names via `hop.yaml`), not directory-scoped.
9. **`rootForCompletion` is a package-level var.** `shell-init zsh` needs `rootCmd` to call `GenZshCompletion`, but threading `rootCmd` through every factory clutters the wiring. `main()` captures it once after `newRootCmd()`; `shell_init.go` reads it. Acceptable singleton for this binary's scale.
