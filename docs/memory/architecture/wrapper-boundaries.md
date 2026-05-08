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
| `RunCapture(ctx, dir, name, args...) ([]byte, error)` | `Run` with an explicit `cmd.Dir`. Captures stdout, stderr passes through. Used by `internal/scan` for `git remote` / `git remote get-url` invocations scoped to a discovered repo's working tree (cmd.Dir is preferred over `git -C` so the subprocess sees the canonical cwd directly). |
| `RunInteractive(ctx, stdin io.Reader, name, args...) (string, error)` | Pipes stdin, captures stdout to string; stderr passes through. Used for fzf. |
| `RunForeground(ctx, dir, name, args...) (int, error)` | Runs a child with `cmd.Dir = dir` and stdin/stdout/stderr **inherited** from the parent. Returns the child's exit code on success (error nil); returns `(-1, ErrNotFound)` if the binary is missing; returns `(-1, err)` for other I/O / exec failures. Used by `hop -R` (and the shim's tool-form, which rewrites to `-R`). Implemented as a thin wrapper over `RunForegroundEnv(ctx, dir, nil, name, args...)`. |
| `RunForegroundEnv(ctx, dir, env, name, args...) (int, error)` | `RunForeground` with an explicit env override. When `env` is `nil`, the subprocess inherits the parent's environment (identical to `RunForeground`). When `env` is non-nil, the subprocess sees exactly those entries — callers SHOULD start from `os.Environ()` and append/override entries to extend the parent env rather than replace it. Used by `hop <name> open` to set `WT_CD_FILE` and `WT_WRAPPER` on top of the parent env when delegating to `wt`. |
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

## `internal/scan` — directory walk + repo classification

`Walk(ctx, root, opts) ([]Found, []Skip, error)`:

- Stack-based DFS with `(path, depth)` frames. Depth-bounded; symlinks followed with `(device, inode)` loop dedup (`syscall.Stat_t` keys). Each `Found.Path` is `filepath.EvalSymlinks`-resolved (canonical).
- Classifies each candidate dir via first-match-wins (`classifyDir`): worktree (`.git` is a regular file) → bare repo (HEAD + config + objects/, no `.git`) → normal repo (`.git` is a directory) → plain dir (recurse). `ReasonSubmodule` is part of the public Skip enum but never emitted — the no-descent invariant ("never enqueue children of a registered repo") makes nested `.git` dirs unreachable through DFS.
- All `git` invocations route through `Options.GitRunner`, which production binds to `internal/proc.RunCapture` (Constitution Principle I). Tests inject a fake `GitRunner` so no real `git` subprocess spawns. Each invocation gets a 5-second `context.WithTimeout`.
- The package is **UI-free**: knows about repos and skips, knows nothing about groups, slugify, conflict resolution, YAML, or stderr UX. The CLI layer (`cmd/hop/config_scan.go`) handles those concerns.

Why a dedicated package: discovery is non-trivial (DFS + inode dedup + classifier + git invocation), benefits from isolated unit tests with an injected `GitRunner`, and slots cleanly alongside `internal/yamled` and `internal/update` as a per-feature internal package. See [config/scan](../config/scan.md) for the per-rule details.

## What is NOT wrapped

Per Constitution Principle IV ("Wrap, Don't Reinvent") — wrap external tools, but don't over-package:

| External call | Where | Why no wrapper package |
|---|---|---|
| `git clone`, `git pull`, `git pull --rebase`, `git push` | `cmd/hop/clone.go`, `cmd/hop/pull.go`, `cmd/hop/sync.go` each call `proc.RunCapture(ctx, path, "git", ...)` (or `proc.Run` for clone) inline | Each call site is one line; `internal/proc.RunCapture` already enforces the cmd.Dir + argv-slice contract. A dedicated `internal/git/` package would be a thin pass-through that adds an indirection without containing logic. Promote later if a verb composes more git operations (e.g., a `status`-then-`fetch` flow that benefits from a single function boundary). |
| YAML parsing | `internal/config/config.go` calls `yaml.Unmarshal` directly into `*yaml.Node` | `gopkg.in/yaml.v3` already is the wrapper. |
| `-R` child exec | `cmd/hop/main.go::runDashR` calls `proc.RunForeground` | Wrapping is `internal/proc`'s job; the binary just composes. |
| `wt open` invocation | `cmd/hop/open.go::runOpen` calls `proc.RunForegroundEnv(ctx, repo.Path, env, "wt", "open")` inline | Single operation; a dedicated `internal/wt/` package would be a 5-line pass-through. The env contract (`WT_CD_FILE`, `WT_WRAPPER`) is wt's, not hop's, and is documented inline in `runOpen`. Promote later if hop grows other wt-delegating verbs that share env construction. |

## `wt` env contract (`cmd/hop/open.go`)

`runOpen` shells out to `wt open` to delegate app detection, menu selection, and launching for `hop <name> open`. wt is a hard runtime dependency declared as a Homebrew formula `depends_on "sahil87/tap/wt"` in `.github/formula-template.rb` (which the release workflow rewrites into `Formula/hop.rb` at tag time).

Two env vars cross the hop→wt boundary:

| Env var | Direction | Purpose |
|---|---|---|
| `WT_CD_FILE` | hop sets, wt reads/writes | hop creates a temp file via `os.CreateTemp("", "hop-open-cd-*")` and exports its path. wt writes the resolved repo path to that file iff the user picks "Open here"; for any other menu choice (editors, terminals, file managers) wt leaves the file empty. After wt exits, hop reads the file and re-emits its contents to stdout. The file is cleaned up via `defer os.Remove`. |
| `WT_WRAPPER` | hop sets, wt reads | hop sets `WT_WRAPPER=1` so wt suppresses its `hint: "Open here" requires the shell wrapper... eval "$(wt shell-setup)"` message — hop is acting as the wrapper for this invocation, and the cd round-trip works through hop's stdout-capture protocol regardless. |

A third env var stays inside the hop→shim boundary:

| Env var | Direction | Purpose |
|---|---|---|
| `HOP_WRAPPER` | shim sets (top-level `export HOP_WRAPPER=1` in `posixInit`), binary reads | The binary uses this to detect that it's running under hop's shell shim, which suppresses `openHereNoShimHint` for the `open` verb (the shim handles the parent-shell `cd` via stdout capture). When unset (binary-direct invocation, tests), the binary emits the hint to stderr to inform the user that "Open here" needs the shim to actually `cd`. The path is still emitted on stdout regardless, so `cd "$(hop <name> open)"` works as a manual composition. |

Why no `internal/wt` wrapper package: the call site is one function (`runOpen`, ~40 lines), the env construction is straightforward (two `append` entries), and the caller-side concerns (resolving the repo, reading back the cd-file, emitting the no-shim hint) are not generic enough to factor into a sub-package. If hop grows additional wt-delegating verbs (a hypothetical `hop <name> openin <app>` shorthand, or `hop <name> tabs` listing wt's app menu), the wrapper extraction becomes a worthwhile refactor — until then the inline call respects the same "Wrap, Don't Reinvent" boundary as `runDashR`.

## Composability primitives

The change introduced two primitives that other operations build on:

- **`hop <name> where`** — path resolver. Stdin/stdout-friendly: `cd "$(hop outbox where)"` works as a shell composition. The repo-verb grammar puts the repo first; the v0.x top-level `hop where <name>` was removed.
- **`hop <name> -R <cmd>...`** — exec-in-context. Repo-scoped: run a child command with cwd set to the resolved repo dir, without leaving the parent shell's cwd changed. The shim's `hop <name> <tool>` tool-form sugar rewrites to this. The shim flips the user-facing form to the binary's internal `hop -R <name> <cmd>...` shape so `extractDashR` (in `cmd/hop/main.go`) is unchanged.

`hop pull` and `hop sync` (added in change `xj3k`) compose a third primitive — `resolveTargets` (`cmd/hop/resolve.go`) — which wraps `resolveByName` with a name-or-group rule order so registry verbs can take a single repo, a group, or `--all` from the same positional slot. Future batch-friendly verbs (e.g., `autosync`, `features`) build on `resolveTargets` rather than each re-implementing the rule order. Path resolution and exec stay on `where` and `-R`.

## Security guarantees

1. **`exec.CommandContext` everywhere** — kernel never sees a shell string; argv is an explicit slice.
2. **User input passes as args, not shell tokens** — repo names from `hop.yaml` reach `git clone` via `proc.Run("git", "clone", url, path)`; fzf queries reach fzf as `--query <q>` (a single arg) and the candidate list is on stdin; `-R`'s child argv is forwarded as a slice to `proc.RunForeground`.
3. **No `sh -c`, no `bash -c`, no command-string interpolation anywhere in production code.**
4. **Atomic file writes** — `internal/yamled` uses temp file + rename in the same directory, preserving the original on rename failure.
