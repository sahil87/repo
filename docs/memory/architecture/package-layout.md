# Package Layout

How the Go source tree is organized for the `hop` binary. Module path remains `github.com/sahil87/repo` (deliberately unchanged in this rename — module path is internal, binary name is what users see; renaming the module is a separate concern deferred unless friction emerges). The module is rooted at `src/go.mod`, not the repo root.

## Tree

```
src/
├── go.mod                        # module github.com/sahil87/repo, go 1.22
├── go.sum
├── cmd/hop/                      # one cobra entrypoint (renamed from cmd/repo/)
│   ├── main.go                   # entrypoint + translateExit + extractDashC + runDashC
│   ├── root.go                   # newRootCmd, rootLong help text, AddCommand wiring
│   ├── where.go                  # newWhereCmd + shared loadRepos/resolveOne/resolveByName/buildPickerLines (was path.go)
│   ├── code.go, open.go, cd.go   # one file per subcommand
│   ├── clone.go, ls.go
│   ├── shell_init.go             # zshInit static prefix + cobra GenZshCompletion at runtime
│   ├── config.go                 # config + nested init/where subcommands
│   ├── *_test.go                 # adjacent unit tests
│   ├── dashc_test.go             # extractDashC unit tests
│   ├── integration_test.go       # builds the binary and exercises it end-to-end
│   └── testutil_test.go          # shared test helpers
└── internal/
    ├── config/                   # YAML schema, search order, embedded starter
    │   ├── config.go             # yaml.Node-based loader, group validation, URL uniqueness
    │   ├── resolve.go            # $HOP_CONFIG search order
    │   ├── starter.yaml          # //go:embed (grouped form)
    │   ├── *_test.go
    │   └── testdata/             # valid + invalid fixtures (mixed shapes, bad names, dup URLs, etc.)
    ├── repos/                    # in-memory Repo model + match
    │   ├── repos.go              # FromConfig, MatchOne, ExpandDir, DeriveName, DeriveOrg
    │   └── repos_test.go
    ├── yamled/                   # comment-preserving YAML node-level edits (NEW)
    │   ├── yamled.go             # AppendURL, ErrGroupNotFound, atomic write
    │   └── yamled_test.go
    ├── fzf/                      # fzf wrapper
    │   ├── fzf.go
    │   └── fzf_test.go
    ├── proc/                     # centralized exec.CommandContext
    │   ├── proc.go               # Run, RunInteractive, RunForeground, ExitCode, ErrNotFound
    │   └── proc_test.go
    └── platform/                 # OS abstraction with build tags
        ├── platform.go           # package doc only
        ├── open_darwin.go        # //go:build darwin
        ├── open_linux.go         # //go:build linux
        └── platform_test.go
```

## Conventions

| Convention | Value |
|---|---|
| Module path | `github.com/sahil87/repo` (unchanged for v1) |
| `go.mod` location | `src/go.mod` (not repo root — mirrors `fab-kit/src/go/wt`) |
| Go version | `1.22` |
| CLI framework | `github.com/spf13/cobra` v1.8.1 |
| YAML library | `gopkg.in/yaml.v3` |
| Tests | Adjacent to source (`config.go` + `config_test.go`) |
| Test fixtures | `testdata/` next to the tests that use them (per-package, not centralized) |
| `internal/<pkg>/` shape | Flat — no nested sub-packages |

## Cobra wiring

Each subcommand is exposed via a `func newXxxCmd() *cobra.Command` factory in its own file. `root.go::newRootCmd()` constructs the root and calls `AddCommand(newWhereCmd(), newCodeCmd(), …)`. `main.go::main()`:

1. Builds `rootCmd := newRootCmd()`.
2. Sets `rootCmd.Version = version` (the package-level `var version = "dev"`, overridden via `-ldflags "-X main.version=…"` at build time — see [build/local](../build/local.md)).
3. Sets `rootForCompletion = rootCmd` (a package-level var used by `shell-init zsh` to call `GenZshCompletion` without threading rootCmd through the factory).
4. Inspects `os.Args` for `-C` via `extractDashC`; if present, resolves the target via `resolveByName` and execs the child via `proc.RunForeground` with `os.Exit(code)` — bypassing cobra entirely.
5. Otherwise calls `rootCmd.Execute()`. Errors are mapped to exit codes via `translateExit`.

`rootCmd` sets `SilenceUsage = true` and `SilenceErrors = true` so we control all stderr/exit emission via `translateExit`. Bare-form (`hop` or `hop <name>`) is implemented by `RunE` checking `len(args)` and dispatching to the same `resolveAndPrint` helper used by `hop where`.

### Why pre-Execute argv inspection for `-C`

Cobra's flag parser would try to dispatch `<cmd>...` after `-C <name>` as a subcommand (or its args), which fails for arbitrary child commands like `hop -C name git status`. Pre-Execute inspection of `os.Args` lets us split argv into the hop portion (just `-C <name>`) and the child portion (the rest), then run the child directly via `proc.RunForeground`. The split is a single function (`extractDashC`), unit-tested in `dashc_test.go`.

## `internal/yamled`

New package introduced by this change. Owns node-level YAML edits — comment-preserving append into a group's URL list. See [wrapper-boundaries](wrapper-boundaries.md) for why it's a separate package from `internal/config`.

API:

```go
func AppendURL(path, group, url string) error
var ErrGroupNotFound = errors.New("yamled: group not found")
```

`AppendURL` reads the file as a `*yaml.Node` tree, navigates `repos.<group>`, appends a new scalar to either the sequence body (flat group) or the `urls:` child sequence (map-shaped group), then marshals and atomically writes back via temp file + rename. Comments are preserved by the yaml.v3 round-trip; **indentation is normalized to yaml.v3 defaults** (this is a deliberate design choice, not a guarantee — comment preservation is the contract, byte-perfect formatting is not).

Errors are wrapped fmt.Errorf strings; missing-group is additionally wrapped via `%w` with `ErrGroupNotFound` so callers can detect via `errors.Is`.

## Cross-references

- Wrapper boundaries (`internal/proc`, `internal/fzf`, `internal/platform` build tags, `internal/yamled` separation): [wrapper-boundaries](wrapper-boundaries.md)
- Build pipeline: [build/local](../build/local.md)
