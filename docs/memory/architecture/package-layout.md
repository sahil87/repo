# Package Layout

How the Go source tree is organized for the `repo` binary. Module `github.com/sahil87/repo`, rooted at `src/go.mod` (not the repo root).

## Tree

```
src/
├── go.mod                        # module github.com/sahil87/repo, go 1.22
├── go.sum
├── cmd/repo/                     # one cobra entrypoint
│   ├── main.go                   # entrypoint + translateExit
│   ├── root.go                   # newRootCmd, rootLong help text, AddCommand wiring
│   ├── path.go                   # newPathCmd + shared resolveOne / resolveAndPrint
│   ├── code.go, open.go, cd.go   # one file per subcommand
│   ├── clone.go, ls.go
│   ├── shell_init.go             # zshInit raw string + factory
│   ├── config.go                 # config + nested init/path subcommands
│   ├── *_test.go                 # adjacent unit tests
│   ├── integration_test.go       # builds the binary and exercises it end-to-end
│   └── testutil_test.go          # shared test helpers
└── internal/
    ├── config/                   # YAML schema, search order, embedded starter
    │   ├── config.go, resolve.go
    │   ├── starter.yaml          # //go:embed
    │   ├── *_test.go
    │   └── testdata/{valid,empty,malformed}.yaml
    ├── repos/                    # in-memory Repo model + match
    │   ├── repos.go
    │   └── repos_test.go
    ├── fzf/                      # fzf wrapper
    │   ├── fzf.go
    │   └── fzf_test.go
    ├── proc/                     # centralized exec.CommandContext
    │   ├── proc.go
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
| Module path | `github.com/sahil87/repo` |
| `go.mod` location | `src/go.mod` (not repo root — mirrors `fab-kit/src/go/wt`) |
| Go version | `1.22` |
| CLI framework | `github.com/spf13/cobra` v1.8.1 |
| YAML library | `gopkg.in/yaml.v3` |
| Tests | Adjacent to source (`config.go` + `config_test.go`) |
| Test fixtures | `testdata/` next to the tests that use them (per-package, not centralized) |
| `internal/<pkg>/` shape | Flat — no nested sub-packages |

## Cobra wiring

Each subcommand is exposed via a `func newXxxCmd() *cobra.Command` factory in its own file. `root.go::newRootCmd()` constructs the root and calls `AddCommand(newPathCmd(), newCodeCmd(), …)`. `main.go::main()`:

1. Builds `rootCmd := newRootCmd()`.
2. Sets `rootCmd.Version = version` (the package-level `var version = "dev"`, overridden via `-ldflags "-X main.version=…"` at build time — see [build/local](../build/local.md)).
3. Calls `rootCmd.Execute()`. Errors are mapped to exit codes via `translateExit`.

`rootCmd` sets `SilenceUsage = true` and `SilenceErrors = true` so we control all stderr/exit emission via `translateExit`. Bare-form (`repo` or `repo <name>`) is implemented by `RunE` checking `len(args)` and dispatching to the same `resolveAndPrint` helper used by `repo path`.

## Cross-references

- Wrapper boundaries (`internal/proc`, `internal/fzf`, `internal/platform` build tags): [wrapper-boundaries](wrapper-boundaries.md)
- Build pipeline: [build/local](../build/local.md)
