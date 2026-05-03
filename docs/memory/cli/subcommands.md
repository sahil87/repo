# CLI Subcommands

What each subcommand of the `repo` binary actually does, as built in v0.0.1. Source files live in `src/cmd/repo/`.

Match resolution algorithm used by `repo`, `repo path`, `repo code`, `repo open`, `repo cd`, `repo clone` is documented separately in [match-resolution](match-resolution.md).

## Inventory

| Subcommand | File | Args | Behavior |
|---|---|---|---|
| `repo` (bare) | `root.go` | 0 or 1 positional | Resolves via match-or-fzf, prints abs path on stdout |
| `repo path <name>` | `path.go` | exactly 1 | Same handler as bare form (`resolveAndPrint`) |
| `repo code [<name>]` | `code.go` | 0 or 1 | Resolves, runs `code <path>` via `internal/proc.Run` (30s timeout) |
| `repo open [<name>]` | `open.go` | 0 or 1 | Resolves, calls `platform.Open(ctx, path)` |
| `repo cd <name>` | `cd.go` | any | Always exits 2 with the binary-form hint; the shell wrapper from `shell-init zsh` is what actually changes cwd |
| `repo clone [<name>]` / `--all` | `clone.go` | 0 or 1, plus `--all` flag | Single resolves via match-or-fzf; `--all` iterates the full list and prints a summary |
| `repo ls` | `ls.go` | none (`cobra.NoArgs`) | Prints aligned `name<spaces>path` rows; empty list prints nothing |
| `repo shell-init <shell>` | `shell_init.go` | exactly 1 | `zsh` → emits `zshInit` raw string; missing or non-zsh → exit 2 with exact stderr |
| `repo config init` | `config.go` | none | Writes embedded `starter.yaml` to `ResolveWriteTarget()` |
| `repo config path` | `config.go` | none | Prints `ResolveWriteTarget()` to stdout (never errors on missing file) |
| `repo -v` / `repo --version` | cobra | — | Auto-wired by cobra when `rootCmd.Version` is set; output is the `var version` value (default `dev`, overridden via `-ldflags "-X main.version=..."`) |
| `repo help` / `-h` / `--help` | cobra | — | Cobra-rendered help, with `rootLong` providing the `Usage:` table and `Notes:` block from `root.go` |

## Exit code convention

Defined in `main.go::translateExit`:

| Code | Trigger |
|---|---|
| 0 | Success |
| 1 | Application error (default for all unmatched errors); also `errSilent` (caller already wrote stderr) |
| 2 | `errExitCode{code: 2}` — used by `cd` and `shell-init` for usage errors |
| 130 | `errFzfCancelled` — fzf user cancellation (Esc / Ctrl-C) |

Cobra's `SilenceUsage: true` and `SilenceErrors: true` are set on `rootCmd`, so `translateExit` is the sole stderr/exit path for top-level errors.

## Shared helpers (`path.go`)

- `loadRepos() (repos.Repos, error)` — `config.Resolve()` → `config.Load()` → `repos.FromConfig()`. Used by every subcommand that reads `repos.yaml`.
- `resolveOne(cmd, query) (*repos.Repo, error)` — implements the match-or-fzf algorithm, returns `errFzfCancelled` / `errSilent` for cancel / fzf-missing paths.
- `resolveAndPrint(cmd, query) error` — wraps `resolveOne` and writes `repo.Path` to stdout.

## External tool failure messages

Lazy: only checked when the tool is actually invoked. Exact stderr lines:

| Tool | Constant / location | Message |
|---|---|---|
| `fzf` | `path.go::fzfMissingHint` | `repo: fzf is not installed. Install it: brew install fzf (macOS) or apt install fzf (Debian).` |
| `git` | `clone.go::gitMissingHint` | `repo: git is not installed.` |
| `code` | `code.go::codeMissingHint` | `repo code: 'code' command not found. Install VSCode and ensure 'code' is on your PATH.` |
| `open`/`xdg-open` | `open.go` (formatted) | `repo open: '<tool>' not found.` (`<tool>` from `platform.OpenTool()`) |

All four trigger `errSilent` (exit 1) after writing to `cmd.ErrOrStderr()`.

## `repo cd` binary-form text

`cd.go::cdHint`:

```
repo: 'cd' is shell-only. Add 'eval "$(repo shell-init zsh)"' to your zshrc, or use: cd "$(repo path "<name>")"
```

## `repo shell-init zsh` emitted text

Defined as `shell_init.go::zshInit`. Defines `repo()` that intercepts `cd` (calls `command repo path "$2"` and `cd --` to the result), otherwise delegates to `command repo "$@"`. Also defines `_repo() { _files }` and registers `compdef _repo repo` if `compdef` exists.

## `repo clone` per-line output

All status lines go to **stderr** (stdout is reserved for resolved paths). Formats:

- `clone: <url> → <path>` (before invoking `git clone`)
- `skip: already cloned at <path>` (when `<path>/.git` exists)
- `repo clone: <path> exists but is not a git repo` (path conflict, exits 1 single / counts toward `failed` for `--all`)
- `summary: cloned=<N> skipped=<M> failed=<F>` (only `--all`)

Clone uses a 10-minute timeout (`clone.go::cloneTimeout`).
