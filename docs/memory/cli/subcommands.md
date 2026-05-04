# CLI Subcommands

What each subcommand of the `hop` binary actually does. Source files live in `src/cmd/hop/`.

Match resolution algorithm used by `hop`, `hop where`, `hop code`, `hop open`, `hop cd`, `hop clone`, `hop -C` is documented separately in [match-resolution](match-resolution.md).

## Inventory

| Subcommand | File | Args | Behavior |
|---|---|---|---|
| `hop` (bare) | `root.go` | 0 or 1 positional | Resolves via match-or-fzf, prints abs path on stdout |
| `hop where <name>` | `where.go` | exactly 1 | Same handler as bare form (`resolveAndPrint`). Renamed from v0.0.1's `path` for voice-fit with the binary name |
| `hop code [<name>]` | `code.go` | 0 or 1 | Resolves, runs `code <path>` via `internal/proc.Run` (30s timeout) |
| `hop open [<name>]` | `open.go` | 0 or 1 | Resolves, calls `platform.Open(ctx, path)` |
| `hop cd <name>` | `cd.go` | any | Always exits 2 with the binary-form hint; the shell wrapper from `shell-init zsh` is what actually changes cwd |
| `hop clone [<name>]` / `--all` | `clone.go` | 0 or 1, plus `--all` flag | Single resolves via match-or-fzf; `--all` iterates the full list and prints a summary |
| `hop clone <url>` | `clone.go` | 1 (URL form) | Ad-hoc clone with auto-registration. Detects URL via `looksLikeURL`. Supports `--group`, `--no-add`, `--no-cd`, `--name` flags. See [Ad-hoc URL clone](#ad-hoc-url-clone) below |
| `hop ls` | `ls.go` | none (`cobra.NoArgs`) | Prints aligned `name<spaces>path` rows; empty list prints nothing |
| `hop shell-init <shell>` | `shell_init.go` | exactly 1 | `zsh` → emits `zshInit` static prefix + cobra-generated `_hop` completion; missing or non-zsh → exit 2 with exact stderr |
| `hop config init` | `config.go` | none | Writes embedded `starter.yaml` to `ResolveWriteTarget()` |
| `hop config where` | `config.go` | none | Prints `ResolveWriteTarget()` to stdout (never errors on missing file). Renamed from v0.0.1's `config path` for consistency with `hop where` |
| `hop update` | `update.go` | none | Self-update via Homebrew. Detects brew install via `os.Executable` + `EvalSymlinks` + `/Cellar/` substring; non-brew installs exit 0 with a manual-update hint. Logic lives in `internal/update`; subprocess calls go through `internal/proc` |
| `hop -C <name> <cmd>...` | `main.go` | global flag + child argv | Resolves `<name>` to a path, then execs `<cmd>...` with `Dir=<path>` and inherited stdio. Implemented via pre-Execute argv inspection in `main.go::extractDashC`; bypasses cobra subcommand parsing for the post-`<name>` argv |
| `hop -v` / `hop --version` | cobra | — | Auto-wired by cobra when `rootCmd.Version` is set; output is the `var version` value (default `dev`, overridden via `-ldflags "-X main.version=..."`) |
| `hop help` / `-h` / `--help` | cobra | — | Cobra-rendered help, with `rootLong` providing the `Usage:` table and `Notes:` block from `root.go` |

## Removed in this rename

The `path` subcommand has been removed (no alias). Use `hop where <name>` or the bare form `hop <name>`.

The `config path` subcommand has been removed (no alias). Use `hop config where`.

## Ad-hoc URL clone

`hop clone <url>` (URL form, detected by `looksLikeURL`: contains `://` OR (`@` AND `:`)) clones a URL not in `hop.yaml`, then registers it.

Flow:
1. Resolve target group (`--group <name>`, default `default`). Missing group → exit 1 with `hop: no '<group>' group in <config-path>. ...`.
2. Compute landing path. For map-shaped groups (`dir:` set): `<dir>/<name>`. For flat groups: `<code_root>/<org>/<name>` (org dropped if URL has none). `--name` overrides the URL-derived name.
3. Classify on-disk state:
   - `stateMissing` → `git clone <url> <path>`, then append URL to `hop.yaml` (unless `--no-add`), print path on stdout (unless `--no-cd`).
   - `stateAlreadyCloned` → emit `skip: already cloned at <path>` to stderr; still appends YAML and prints path (registers existing checkout).
   - `statePathExistsNotGit` → emit `hop clone: <path> exists but is not a git repo`; exit 1; no YAML write, no stdout.
4. URL already in target group's `urls` list → emit `skip: <url> already registered in '<group>'` to stderr, no YAML write, but still print path (unless `--no-cd`).

YAML write goes through `internal/yamled.AppendURL` (comment-preserving, atomic temp+rename — see [architecture/package-layout](../architecture/package-layout.md#internalyamled)).

## Exit code convention

Defined in `main.go::translateExit`:

| Code | Trigger |
|---|---|
| 0 | Success |
| 1 | Application error (default for all unmatched errors); also `errSilent` (caller already wrote stderr) |
| 2 | `errExitCode{code: 2}` — used by `cd`, `shell-init`, and `-C` for usage errors |
| 130 | `errFzfCancelled` — fzf user cancellation (Esc / Ctrl-C) |

Cobra's `SilenceUsage: true` and `SilenceErrors: true` are set on `rootCmd`, so `translateExit` is the sole stderr/exit path for top-level errors.

The `-C` flag bypasses cobra entirely (pre-Execute argv inspection) and uses `os.Exit` directly with the child's exit code (or 2 for usage errors, 1 for resolution errors).

## Shared helpers (`where.go`)

- `loadRepos() (repos.Repos, error)` — `config.Resolve()` → `config.Load()` → `repos.FromConfig()`. Used by every subcommand that reads `hop.yaml`.
- `resolveByName(query string) (*repos.Repo, error)` — implements the match-or-fzf algorithm without writing to stderr; returns typed sentinels (`errFzfMissing`, `errFzfCancelled`) so callers control which stderr to write to.
- `resolveOne(cmd, query) (*repos.Repo, error)` — cobra-friendly wrapper that writes `fzfMissingHint` to `cmd.ErrOrStderr()` and returns `errSilent` on missing fzf.
- `resolveAndPrint(cmd, query) error` — wraps `resolveOne` and writes `repo.Path` to stdout.
- `buildPickerLines(rs) []string` — builds the tab-separated lines piped to fzf. When two repos share a `Name`, the displayed first column gets a `[<group>]` suffix. The path column (used for match-back) is unique per repo.

## External tool failure messages

Lazy: only checked when the tool is actually invoked. Exact stderr lines:

| Tool | Constant / location | Message |
|---|---|---|
| `fzf` | `where.go::fzfMissingHint` | `hop: fzf is not installed. Install it: brew install fzf (macOS) or apt install fzf (Debian).` |
| `git` | `clone.go::gitMissingHint` | `hop: git is not installed.` |
| `code` | `code.go::codeMissingHint` | `hop code: 'code' command not found. Install VSCode and ensure 'code' is on your PATH.` |
| `open`/`xdg-open` | `open.go` (formatted) | `hop open: '<tool>' not found.` (`<tool>` from `platform.OpenTool()`) |
| `brew` | `internal/update` | `hop update: brew not found on PATH.` (only when binary is brew-installed) |

All five trigger `errSilent` (exit 1) after writing to `cmd.ErrOrStderr()`.

## `hop cd` binary-form text

`cd.go::cdHint`:

```
hop: 'cd' is shell-only. Add 'eval "$(hop shell-init zsh)"' to your zshrc, or use: cd "$(hop where "<name>")"
```

## `hop shell-init zsh` emitted text

The static portion (`shell_init.go::zshInit`) defines:

- `hop()` function with bare-name dispatch — known subcommands (`cd|clone|where|ls|code|open|shell-init|config|update|--help|-h|--version|completion`) route through `_hop_dispatch`; flag-prefixed args pass through to `command hop`; everything else is treated as `cd <name>` (the bare-name dispatch).
- `_hop_dispatch()` helper — handles the shell-mutating `cd` path (`command hop where "$2"` then `cd --`), and the URL-detected `clone` path (`cd --` to the printed path on success).
- `h() { hop "$@"; }` — single-letter alias.
- `hi() { command hop "$@"; }` — un-shadowed alias (calls the binary directly, bypassing the shim).

The cobra-generated zsh completion (a `_hop` function) is appended at runtime via `rootCmd.GenZshCompletion(out)`. The `rootCmd` reference is captured in `main.go::rootForCompletion` (a package-level var set in `main()`).

## `hop clone` per-line output

All status lines go to **stderr** (stdout is reserved for resolved paths, used by the shell shim's `cd`). Formats:

- `clone: <url> → <path>` (before invoking `git clone`)
- `skip: already cloned at <path>` (when `<path>/.git` exists)
- `skip: <url> already registered in '<group>'` (URL form, when URL is already in the target group's `urls` list)
- `hop clone: <path> exists but is not a git repo` (path conflict, exits 1 single / counts toward `failed` for `--all`)
- `summary: cloned=<N> skipped=<M> failed=<F>` (only `--all`)

Clone uses a 10-minute timeout (`clone.go::cloneTimeout`).

## `hop update` — self-update via Homebrew

Implementation in `internal/update`; the cobra factory in `cmd/hop/update.go` is a thin wrapper. The flow:

1. **Detect brew install**: walk `os.Executable()` through `filepath.EvalSymlinks` and check whether the resolved path contains `/Cellar/`. If not, print `hop v<X> was not installed via Homebrew.\nUpdate manually, or reinstall with: brew install sahil87/tap/hop` and exit 0.
2. **Refresh brew index**: `brew update --quiet` with a 30-second timeout (via `proc.Run` + `context.WithTimeout`). Failure exits 1.
3. **Query latest version**: `brew info --json=v2 sahil87/tap/hop` and parse `formulae[0].versions.stable`. The formula name is **fully qualified** (`sahil87/tap/hop`) to dodge a name collision with the Homebrew core `hop` cask (an HWP document viewer).
4. **Compare versions**: normalize both sides by stripping a leading `v` (binary reports `v0.0.3`, brew reports `0.0.3`). If equal, print `Already up to date (v<X>).` and exit 0.
5. **Upgrade**: `brew upgrade sahil87/tap/hop` with a 120-second timeout. Stream brew's stdout/stderr through to the user via `proc.RunForeground` so progress is visible. On success, print `Updated to v<new>.` and exit 0.

All `brew` invocations route through `internal/proc` (Constitution Principle I — no direct `os/exec` outside `internal/proc`). The package exposes `Run(currentVersion string) error` as its single public entry point.
