# Wrapper Boundaries

How `repo` wraps external tools and isolates platform-specific code. Enforces Constitution Principle I (Security First) and Principle IV (Wrap, Don't Reinvent).

## `internal/proc` — the security choke point

All subprocess invocations in production code MUST go through `src/internal/proc/proc.go`. No production package outside `internal/proc/` may import `os/exec` directly. Verified by audit:

```
grep --include='*.go' --exclude='*_test.go' -rn '"os/exec"' src/internal/ src/cmd/
# → matches restricted to src/internal/proc/

grep --include='*.go' --exclude='*_test.go' -rn 'exec\.Command\b' src/
# → zero matches (only exec.CommandContext is permitted)
```

Test files (`*_test.go`) MAY use `os/exec` directly to spawn the built binary in integration tests; the audits scope to non-test code.

### API

| Symbol | Signature |
|---|---|
| `Run(ctx, name, args...) ([]byte, error)` | Non-interactive. Captures stdout to bytes; stderr passes through to parent. |
| `RunInteractive(ctx, stdin io.Reader, name, args...) (string, error)` | Pipes stdin, captures stdout to string; stderr passes through. Used for fzf. |
| `var ErrNotFound` | Sentinel returned when the binary is not on PATH. Callers use `errors.Is(err, proc.ErrNotFound)` to produce install-hint messages. |

Both functions use `exec.CommandContext(ctx, name, args...)` — never `exec.Command`, never shell strings. Callers supply the `context.Context` (with timeout for non-interactive ops; `context.Background()` for fzf since the user is at the keyboard).

## `internal/fzf` — fzf wrapper

`Pick(ctx, lines []string, query string) (string, error)`:

- Joins `lines` with `\n` and pipes via stdin to `fzf`.
- Argv built by `buildArgs(query)`: `--query <q>` (omitted when empty), then `--select-1 --height 40% --reverse --with-nth 1 --delimiter '\t'`.
- All exec goes through `proc.RunInteractive` — no direct `os/exec`.
- A package-level `var runInteractive = proc.RunInteractive` provides a test seam for asserting argv composition without spawning fzf.
- Errors propagate directly; callers `errors.Is(err, proc.ErrNotFound)` to detect missing fzf.

Why a dedicated package: the invocation is non-trivial (multiple flags, stdin piping, query prefill) and used by 5+ subcommands. Worth one file.

## `internal/platform` — OS isolation via build tags

`platform.go` declares the package only (no exported symbols). The two build-tagged files implement `Open`:

- `open_darwin.go` — `//go:build darwin`; calls `proc.Run(ctx, "open", path)`. `OpenTool() string` returns `"open"`.
- `open_linux.go` — `//go:build linux`; calls `proc.Run(ctx, "xdg-open", path)`. `OpenTool() string` returns `"xdg-open"`.

`OpenTool()` exists so `cmd/repo/open.go` can format the missing-tool stderr (`repo open: 'open' not found.` vs `repo open: 'xdg-open' not found.`) without knowing which OS it's on.

Other platforms (Windows) fail at link time — by design (Constitution Cross-Platform Behavior).

Cross-platform builds verified by `cd src && GOOS=darwin GOARCH=arm64 go build ./...` and `cd src && GOOS=linux GOARCH=amd64 go build ./...` — both succeed.

## What is NOT wrapped

Per Constitution Principle IV ("Wrap, Don't Reinvent") — wrap external tools, but don't over-package:

| External call | Where | Why no wrapper package |
|---|---|---|
| `git clone` | `cmd/repo/clone.go` calls `proc.Run(ctx, "git", "clone", url, path)` inline | Single operation; a 5-line `internal/git/` package is premature abstraction. Promote later if `git fetch` / `git pull` / `git status` get added. |
| `code` | `cmd/repo/code.go` calls `proc.Run(ctx, "code", path)` inline | Single operation. |
| YAML parsing | `internal/config/config.go` calls `yaml.Unmarshal` directly | `gopkg.in/yaml.v3` already is the wrapper. |

## Security guarantees

1. **`exec.CommandContext` everywhere** — kernel never sees a shell string; argv is an explicit slice.
2. **User input passes as args, not shell tokens** — repo names from `repos.yaml` reach `git clone` via `proc.Run("git", "clone", url, path)`; fzf queries reach fzf as `--query <q>` (a single arg) and the candidate list is on stdin.
3. **No `sh -c`, no `bash -c`, no command-string interpolation anywhere in production code.**
