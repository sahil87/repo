# Wrapper Boundaries

How `hop` wraps external tools. Enforces Constitution Principle I (Security First) and Principle IV (Wrap, Don't Reinvent).

## `internal/proc` — the security choke point

All subprocess invocations in production code MUST go through `src/internal/proc/proc.go`. No production package outside `internal/proc/` may import `os/exec` directly. Verified by audit:

```
grep --include='*.go' --exclude='*_test.go' -rn '"os/exec"' src/internal/ src/cmd/
# → matches restricted to src/internal/proc/

grep --include='*.go' --exclude='*_test.go' -rn 'exec\.Command\b' src/
# → zero matches (only exec.CommandContext is permitted)
```

Test files (`*_test.go`) MAY use `os/exec` directly — to spawn the built binary in integration tests, or to set up local git fixtures (e.g., `git init --bare` for ad-hoc URL clone tests). The audits scope to non-test code.

### API

| Symbol | Signature |
|---|---|
| `Run(ctx, name, args...) ([]byte, error)` | Non-interactive. Captures stdout to bytes; stderr passes through to parent. |
| `RunInteractive(ctx, stdin io.Reader, name, args...) (string, error)` | Pipes stdin, captures stdout to string; stderr passes through. Used for fzf. |
| `RunForeground(ctx, dir, name, args...) (int, error)` | Runs a child with `cmd.Dir = dir` and stdin/stdout/stderr **inherited** from the parent. Returns the child's exit code on success (error nil); returns `(-1, ErrNotFound)` if the binary is missing; returns `(-1, err)` for other I/O / exec failures. Used by `hop -R` (and the shim's tool-form, which rewrites to `-R`). |
| `var ErrNotFound` | Sentinel returned when the binary is not on PATH. Callers use `errors.Is(err, proc.ErrNotFound)` to produce install-hint messages. |
| `ExitCode(err) (int, bool)` | Helper to extract the child's exit code from an `*exec.ExitError` without callers needing to import `os/exec`. |

All three runner functions use `exec.CommandContext(ctx, name, args...)` — never `exec.Command`, never shell strings. Callers supply the `context.Context` (with timeout for non-interactive ops; `context.Background()` for fzf and `-R` since the user is at the keyboard / running an arbitrary child).

## `internal/fzf` — fzf wrapper

`Pick(ctx, lines []string, query string) (string, error)`:

- Joins `lines` with `\n` and pipes via stdin to `fzf`.
- Argv built by `buildArgs(query)`: `--query <q>` (omitted when empty), then `--select-1 --height 40% --reverse --with-nth 1 --delimiter '\t'`.
- All exec goes through `proc.RunInteractive` — no direct `os/exec`.
- A package-level `var runInteractive = proc.RunInteractive` provides a test seam for asserting argv composition without spawning fzf.
- Errors propagate directly; callers `errors.Is(err, proc.ErrNotFound)` to detect missing fzf.

Why a dedicated package: the invocation is non-trivial (multiple flags, stdin piping, query prefill) and used by 5+ subcommands. Worth one file.

## `internal/update` — Homebrew self-update

`Run(currentVersion string, out, errOut io.Writer) error`:

- Detects whether the binary was installed via Homebrew by walking `os.Executable()` through `filepath.EvalSymlinks` and checking for `/Cellar/` in the resolved path. Non-brew installs print a manual-update hint to `out` and return nil (exit 0).
- Refreshes the brew index (`brew update --quiet`, 30s timeout via `proc.Run`).
- Queries the latest tap formula version (`brew info --json=v2 sahil87/tap/hop`, parses `formulae[0].versions.stable`).
- Compares against `currentVersion` after stripping any leading `v` (binary reports `v0.0.3`, brew reports `0.0.3`).
- On mismatch, runs `brew upgrade sahil87/tap/hop` with a 120s timeout via `proc.RunForeground` so brew's progress streams through.
- All `brew` invocations route through `internal/proc` (Constitution Principle I).

Stream routing — `out` and `errOut` receive **only the wrapper messages this package emits** ("Current version:", "Already up to date.", error hints). Subprocess stdout/stderr from `brew update`, `brew info`, and `brew upgrade` is intentionally NOT routed through these writers — `internal/proc` owns subprocess streams (`proc.Run` pipes child stderr to the parent's `os.Stderr`; `proc.RunForeground` inherits all three streams). The split is deliberate: subprocess streams are tty-aware (brew prints colored progress); wrapper messages are small and may be redirected for tests or embedding. Production callers pass `os.Stdout` / `os.Stderr` to keep both consistent.

Error contract — when `brew` is missing on PATH, `Run` writes `hop update: brew not found on PATH.` to `errOut` and returns `proc.ErrNotFound`. The cobra wrapper in `cmd/hop/update.go` catches this with `errors.Is` and converts to `errSilent` so `translateExit` does not also print the underlying "binary not found on PATH" message.

Formula name: **`sahil87/tap/hop` (fully qualified)** to dodge a name collision with the Homebrew core `hop` cask (an HWP document viewer).

## `internal/yamled` — comment-preserving YAML write-back

`AppendURL(path, group, url string) error`:

- Reads the file, parses to `*yaml.Node`, navigates `repos.<group>`, appends a new scalar to either the sequence body (flat group) or the `urls:` child sequence (map-shaped group), marshals, and writes back via temp file + `os.Rename` (atomic on the same filesystem).
- Comments are preserved by the yaml.v3 round-trip. **Indentation is normalized to yaml.v3 defaults** — comment preservation is the contract, byte-perfect formatting is not.
- `ErrGroupNotFound` is a sentinel wrapped via `%w` when the named group is absent. Detect via `errors.Is(err, yamled.ErrGroupNotFound)`.

Why a dedicated package separate from `internal/config`: `config` validates and consumes; `yamled` produces a node tree, navigates, mutates, writes. Different responsibilities — `config` is the schema validator; `yamled` is a node-level mutator. Either can be tested independently.

## What is NOT wrapped

Per Constitution Principle IV ("Wrap, Don't Reinvent") — wrap external tools, but don't over-package:

| External call | Where | Why no wrapper package |
|---|---|---|
| `git clone` | `cmd/hop/clone.go` calls `proc.Run(ctx, "git", "clone", url, path)` inline | Single operation; a 5-line `internal/git/` package is premature abstraction. Promote later if `git fetch` / `git pull` / `git status` get added. |
| YAML parsing | `internal/config/config.go` calls `yaml.Unmarshal` directly into `*yaml.Node` | `gopkg.in/yaml.v3` already is the wrapper. |
| `-R` child exec | `cmd/hop/main.go::runDashR` calls `proc.RunForeground` | Wrapping is `internal/proc`'s job; the binary just composes. |

## Composability primitives

The change introduced two primitives that other operations build on:

- **`hop where <name>`** — path resolver. Stdin/stdout-friendly: `cd "$(hop where outbox)"` works as a shell composition.
- **`hop <name> -R <cmd>...`** — exec-in-context. Repo-scoped: run a child command with cwd set to the resolved repo dir, without leaving the parent shell's cwd changed. The shim's `hop <name> <tool>` tool-form sugar rewrites to this. The shim flips the user-facing form to the binary's internal `hop -R <name> <cmd>...` shape so `extractDashR` (in `cmd/hop/main.go`) is unchanged.

Future verbs (`sync`, `autosync`, `features`) build on these rather than each one re-implementing path resolution and exec.

## Security guarantees

1. **`exec.CommandContext` everywhere** — kernel never sees a shell string; argv is an explicit slice.
2. **User input passes as args, not shell tokens** — repo names from `hop.yaml` reach `git clone` via `proc.Run("git", "clone", url, path)`; fzf queries reach fzf as `--query <q>` (a single arg) and the candidate list is on stdin; `-R`'s child argv is forwarded as a slice to `proc.RunForeground`.
3. **No `sh -c`, no `bash -c`, no command-string interpolation anywhere in production code.**
4. **Atomic file writes** — `internal/yamled` uses temp file + rename in the same directory, preserving the original on rename failure.
