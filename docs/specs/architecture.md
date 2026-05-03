# Architecture

> Repository folder layout, Go package responsibilities, and cross-platform strategy for the `repo` binary.

## Top-Level Repository Layout

```
repo/
├── README.md
├── LICENSE
├── justfile                              # one-line recipes per Constitution Principle V
├── .gitignore                            # ignores ./bin/
├── src/                                  # all Go source (mirrors fab-kit/src/go/wt convention)
│   ├── go.mod                            # module: github.com/sahil87/repo
│   ├── go.sum
│   ├── cmd/
│   │   └── repo/
│   │       ├── main.go                   # entrypoint
│   │       ├── root.go                   # cobra root + version handling
│   │       ├── path.go                   # `repo path` and bare `repo <name>`
│   │       ├── code.go                   # `repo code`
│   │       ├── open.go                   # `repo open`
│   │       ├── cd.go                     # `repo cd` (binary form)
│   │       ├── clone.go                  # `repo clone` (single + --all)
│   │       ├── ls.go                     # `repo ls`
│   │       ├── shell_init.go             # `repo shell-init zsh`
│   │       ├── config.go                 # `repo config init` and `repo config path`
│   │       ├── *_test.go                 # adjacent unit tests per file
│   │       ├── integration_test.go       # end-to-end tests
│   │       └── testutil_test.go          # shared test helpers
│   └── internal/
│       ├── config/
│       │   ├── config.go                 # YAML schema, Load(path) (*Config, error)
│       │   ├── resolve.go                # Resolve() (string, error)
│       │   ├── starter.yaml              # //go:embed for `repo config init`
│       │   ├── *_test.go
│       │   └── testdata/
│       │       ├── valid.yaml
│       │       ├── empty.yaml
│       │       └── malformed.yaml
│       ├── repos/
│       │   ├── repos.go                  # Repo, Repos, MatchOne, List
│       │   └── repos_test.go
│       ├── fzf/
│       │   ├── fzf.go                    # Pick(items, query) wrapper
│       │   └── fzf_test.go
│       ├── proc/
│       │   ├── proc.go                   # Run, RunInteractive — centralized exec
│       │   └── proc_test.go
│       └── platform/
│           ├── platform.go               # OS-agnostic types
│           ├── open_darwin.go            # //go:build darwin
│           └── open_linux.go             # //go:build linux
├── scripts/                              # justfile delegates here
│   ├── build.sh
│   └── install.sh
├── docs/
│   ├── memory/                           # post-implementation reality (auto-generated)
│   └── specs/                            # pre-implementation design (this directory)
└── fab/                                  # fab-kit workflow artifacts
    ├── backlog.md
    ├── changes/
    └── project/
```

## Conventions

| Convention | Value |
|---|---|
| Module path | `github.com/sahil87/repo` |
| `go.mod` location | `src/go.mod` (rooted at `src/`, not repo root) |
| Subcommand layout | `src/cmd/repo/<verb>.go` (one file per subcommand) |
| Internal packages | `src/internal/<pkg>/` (flat — no nested packages) |
| Tests | Adjacent to source (`config.go` + `config_test.go`) |
| Test fixtures | `testdata/` next to the tests that use them (per-package, not centralized) |
| CLI framework | `github.com/spf13/cobra` |
| YAML library | `gopkg.in/yaml.v3` |

This mirrors the `fab-kit/src/go/wt` convention. Tests import packages as `github.com/sahil87/repo/internal/config`, etc.

`go test` automatically excludes any `testdata/` directory from package compilation, so per-package fixtures are the idiomatic placement. Tests load fixtures with relative paths like `os.ReadFile("testdata/valid.yaml")`.

## Package Responsibilities

### `cmd/repo`

Cobra command definitions, flag parsing, exit code handling.

| File | Exports / contents |
|---|---|
| `main.go` | `func main()` — constructs root, calls `Execute()`, exits non-zero on error |
| `root.go` | `func newRootCmd() *cobra.Command` — root command with `RunE` for bare-form (no subcommand or single positional). Sets `Version`, `SilenceUsage`, `SilenceErrors`. |
| `path.go` | `func newPathCmd() *cobra.Command` — `repo path <name>`. Shared `resolveAndPrint(name string) error` helper used by `root.go`'s bare form. |
| `code.go` | `func newCodeCmd() *cobra.Command` |
| `open.go` | `func newOpenCmd() *cobra.Command` |
| `cd.go` | `func newCdCmd() *cobra.Command` — prints hint and exits 2 |
| `clone.go` | `func newCloneCmd() *cobra.Command` — handles `--all` flag dispatch |
| `ls.go` | `func newLsCmd() *cobra.Command` |
| `shell_init.go` | `func newShellInitCmd() *cobra.Command`. The emitted zsh function/completion is a Go raw string constant (`const zshInit = ...`) — no template engine needed. |
| `config.go` | `func newConfigCmd() *cobra.Command` — parent for `init` and `path` subcommands; uses `cobra.Command{Use: "config"}` with `AddCommand(newConfigInitCmd(), newConfigPathCmd())` |

`main.go` skeleton:

```go
package main

import (
	"fmt"
	"os"
)

var version = "dev"  // overridden via -ldflags at build time

func main() {
	rootCmd := newRootCmd()
	rootCmd.Version = version
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

### `internal/config`

Configuration file resolution and YAML loading.

| Symbol | Signature / purpose |
|---|---|
| `type Config` | Top-level YAML structure. Field: `Entries map[string][]string` (directory → list of URLs), populated by yaml.v3 unmarshal. |
| `func Resolve() (string, error)` | Search-order resolution per `config-resolution.md`. Returns the resolved path or error if none can be determined. |
| `func Load(path string) (*Config, error)` | Reads, unmarshals, validates the file. Wraps yaml.v3 errors with file path and line number. |
| `func WriteStarter(path string) error` | Writes the embedded `starter.yaml` to `path`. Refuses if the file exists. Used by `repo config init`. |
| `//go:embed starter.yaml` | Embeds the starter content. |

### `internal/repos`

In-memory repo model and queries. Consumes `*config.Config`.

| Symbol | Signature / purpose |
|---|---|
| `type Repo` | `struct { Name, Dir, URL, Path string }`. `Path = Dir + "/" + Name`, with `~` already expanded in `Dir`. |
| `type Repos []Repo` | Ordered list. |
| `func FromConfig(cfg *config.Config) (Repos, error)` | Converts config to repo list. Expands `~` in directory keys. Strips `.git` from URL basenames to derive names. |
| `func (rs Repos) MatchOne(query string) Repos` | Returns case-insensitive substring matches on `Name`. |
| `func (rs Repos) List() Repos` | Returns all repos (identity, exists for symmetry). |

### `internal/fzf`

Fzf wrapper.

| Symbol | Signature / purpose |
|---|---|
| `func Pick(ctx context.Context, lines []string, query string) (string, error)` | Pipes `lines` to fzf via stdin; returns the selected line. Uses `internal/proc`. Flags: `--query <query> --select-1 --height 40% --reverse --with-nth 1 --delimiter '\t'`. Returns specific error type for "fzf not installed" so callers can produce the install hint. |

The picker contract: callers format `lines` as tab-separated strings (`name<TAB>path<TAB>url`), and fzf displays only the first column (`--with-nth 1`).

### `internal/proc`

Centralized `exec.CommandContext` wrapper. **All** subprocess invocations in the codebase MUST go through this package — enforced by Constitution Principle I (Security First) and verified at apply time via grep audit (`grep -rn "os/exec" src/internal/ src/cmd/` should return hits only in `proc.go`).

| Symbol | Signature / purpose |
|---|---|
| `func Run(ctx context.Context, name string, args ...string) ([]byte, error)` | Runs a command, returns stdout as bytes. Used for non-interactive subprocesses (`git`, `code`). |
| `func RunInteractive(ctx context.Context, stdin io.Reader, name string, args ...string) (string, error)` | Runs a command with stdin piped from `stdin`, returns stdout as a string. Used for `fzf`. |
| `var ErrNotFound` | Sentinel error indicating the binary itself is not on PATH. Callers use `errors.Is(err, proc.ErrNotFound)` to produce install-hint messages. |

Default timeouts: callers MUST construct a `context.Context` with a timeout (e.g., `context.WithTimeout(context.Background(), 5*time.Second)` for read-only ops, 30s for clone). Interactive operations (fzf) use `context.Background()` (no timeout) because the user is at the keyboard.

### `internal/platform`

Cross-platform abstractions, isolated behind build tags.

| Symbol | Signature / purpose |
|---|---|
| `func Open(ctx context.Context, path string) error` | Opens `path` in the OS file manager. Implementation in `open_darwin.go` (`open <path>`) and `open_linux.go` (`xdg-open <path>`). Both delegate to `internal/proc.Run`. |

`open_darwin.go` starts with `//go:build darwin`; `open_linux.go` starts with `//go:build linux`. Other platforms (Windows) fail at link time — by design (Constitution Cross-Platform Behavior section).

## Wrapper Boundaries

Per Constitution Principle IV ("Wrap, Don't Reinvent"):

| External tool | Wrapper |
|---|---|
| `git` | Inline `internal/proc.Run("git", ...)` calls in `cmd/repo/clone.go`. No dedicated `internal/git/` package — premature for one operation (`git clone`). |
| `fzf` | `internal/fzf.Pick` — the invocation is non-trivial (multiple flags, stdin piping, query prefill), so it warrants a package. |
| `code` | Inline `internal/proc.Run("code", ...)` in `cmd/repo/code.go`. |
| `open` / `xdg-open` | `internal/platform.Open` — wrapped because the choice is platform-specific. |
| YAML | `gopkg.in/yaml.v3` — used directly via `yaml.Unmarshal` in `internal/config/config.go`. Not re-wrapped. |

### Why `internal/git/` does NOT exist

For v0.0.1, the entire git surface is:
1. `git clone <url> <dest>` (in `_clone` and `_clone_all` paths)
2. A filesystem check (`<path>/.git` exists) — not even a git call.

Wrapping one operation in a 5-line package is premature abstraction. If the surface grows (e.g., `git fetch`, `git pull`, `git status` on cloned repos), promote to a package then.

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

1. **All subprocess invocations go through `internal/proc`.** No package outside `internal/proc/` MAY import `os/exec` directly. Verifiable: `grep -rn '"os/exec"' src/cmd src/internal/{config,repos,fzf,platform}` returns nothing.
2. **All `proc.Run`/`proc.RunInteractive` calls use `exec.CommandContext` with explicit argument slices.** Never shell strings, never `exec.Command`.
3. **User input is validated before passing to subprocess.** Repo names from `repos.yaml` are extracted via URL-basename split (no shell metachars survive). Search queries from CLI args are passed to fzf via stdin, not as args, eliminating shell-injection paths.

> **GIVEN** a repo URL `git@github.com:user/repo;ls.git`
> **WHEN** any subcommand resolves the repo
> **THEN** the derived name is `repo;ls` (last `/`-component, `.git` stripped)
> **AND** if used as an arg to `git clone`, it's passed as a literal arg (not shell-evaluated) — `git` will reject the URL or clone into a directory named `repo;ls`, but no shell injection occurs

## Design Decisions

1. **`src/` rooted module, not repo root.** Mirrors `fab-kit/src/go/wt` — the convention for sahil87 Go binaries. Reserves repo root for non-Go artifacts (justfile, scripts, docs).
2. **Cobra over hand-rolled dispatch.** Cobra is already a dependency in `wt`, so weight is amortized. Eleven subcommands with subcommand-specific flags justify the dep.
3. **Flat `internal/<pkg>/` layout.** No `internal/cli/`, no nested packages. Each package has one job: config, repos, fzf, proc, platform.
4. **`testdata/` per-package, not centralized.** Idiomatic Go layout — `go test` auto-excludes any `testdata/` from package compilation, and tests load fixtures with simple relative paths.
5. **`internal/proc/` is the security choke point.** Centralizing exec lets the security audit be a single grep, not a code review of every call site.
6. **No `internal/git/` package for v0.0.1.** One operation doesn't justify a package. Promote later if the git surface grows.
