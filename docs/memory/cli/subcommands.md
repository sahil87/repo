# CLI Subcommands

What each subcommand of the `hop` binary actually does. Source files live in `src/cmd/hop/`.

Match resolution algorithm used by `hop`, `hop where`, `hop cd`, `hop clone`, `hop -R` is documented separately in [match-resolution](match-resolution.md).

## Inventory

| Subcommand | File | Args | Behavior |
|---|---|---|---|
| `hop` (bare) | `root.go` | 0 or 1 positional | Resolves via match-or-fzf, prints abs path on stdout |
| `hop where <name>` | `where.go` | exactly 1 | Same handler as bare form (`resolveAndPrint`). Renamed from v0.0.1's `path` for voice-fit with the binary name |
| `hop cd <name>` | `cd.go` | any | Always exits 2 with the binary-form hint; the shell wrapper from `shell-init <shell>` is what actually changes cwd |
| `hop clone [<name>]` / `--all` | `clone.go` | 0 or 1, plus `--all` flag | Single resolves via match-or-fzf; `--all` iterates the full list and prints a summary |
| `hop clone <url>` | `clone.go` | 1 (URL form) | Ad-hoc clone with auto-registration. Detects URL via `looksLikeURL`. Supports `--group`, `--no-add`, `--no-cd`, `--name` flags. See [Ad-hoc URL clone](#ad-hoc-url-clone) below |
| `hop ls` | `ls.go` | none (`cobra.NoArgs`) | Prints aligned `name<spaces>path` rows; empty list prints nothing |
| `hop shell-init <shell>` | `shell_init.go` | exactly 1 | `zsh` → emits `posixInit` prefix + cobra-generated `_hop` zsh completion + `compdef _hop h hi`; `bash` → `posixInit` + cobra-generated `__start_hop` bash completion + `complete -o default -F __start_hop h hi`; missing or other → exit 2 with exact stderr |
| `hop config init` | `config.go` | none | Writes embedded `starter.yaml` to `ResolveWriteTarget()`. Post-write stderr tip points users at `hop config scan <dir>` (and the `$HOP_CONFIG` portability tip) — see [config/init-bootstrap](../config/init-bootstrap.md) |
| `hop config where` | `config.go` | none | Prints `ResolveWriteTarget()` to stdout (never errors on missing file). Renamed from v0.0.1's `config path` for consistency with `hop where` |
| `hop config scan <dir>` | `config_scan.go` | exactly 1 dir; `--write` (bool, default false), `--depth N` (int, default 3, must be >= 1) | Walk `<dir>` for git repos (DFS, depth-bounded, symlink-following with (dev,inode) loop dedup), auto-derive groups (convention match → `default`; non-match → invent group from parent-dir basename, slugified), render YAML to stdout (default) or merge into the resolved `hop.yaml` (`--write`, atomic + comment-preserving via `internal/yamled.MergeScan`). Skips worktrees, bare repos, no-remote repos; submodules excluded by the no-descent invariant. Exit codes: 0 success (any repo count, including zero); 1 hop.yaml missing, load error, or write/merge failure; 2 usage error (missing `<dir>`, dir validation failure, `--depth < 1`). See [config/scan](../config/scan.md) |
| `hop update` | `update.go` | none | Self-update via Homebrew. Detects brew install via `os.Executable` + `EvalSymlinks` + `/Cellar/` substring; non-brew installs exit 0 with a manual-update hint. Logic lives in `internal/update`; subprocess calls go through `internal/proc` |
| `hop <name> -R <cmd>...` | `main.go` | repo name + flag + child argv | Resolves `<name>` to a path, then execs `<cmd>...` with `Dir=<path>` and inherited stdio. User-facing form puts the repo first; the shim rewrites it to the binary's internal shape `hop -R <name> <cmd>...` before exec. Implemented via pre-Execute argv inspection in `main.go::extractDashR`, which scans for `-R` and treats the next token as `<name>` and everything after as `<cmd>...`. Bypasses cobra subcommand parsing for the post-`<name>` argv. Spelled `-R` (not `-C`) because hop is repo-scoped, not directory-scoped |
| `hop <name> <tool> [args...]` | (shim only) | 2+ args | Sugar for `hop <name> -R <tool> [args...]`. Lives in `shell_init.go::posixInit`, NOT the binary. The binary errors on this argv shape (cobra's max-1-arg root). See [Tool-form dispatch](#tool-form-dispatch) below |
| `hop -v` / `hop --version` | cobra | — | Auto-wired by cobra when `rootCmd.Version` is set; output is the `var version` value (default `dev`, overridden via `-ldflags "-X main.version=..."`) |
| `hop help` / `-h` / `--help` | cobra | — | Cobra-rendered help, with `rootLong` providing the `Usage:` table and `Notes:` block from `root.go` |

## Removed subcommands

The `path` subcommand has been removed (no alias). Use `hop where <name>` or the bare form `hop <name>`.

The `config path` subcommand has been removed (no alias). Use `hop config where`.

The `code` subcommand has been removed (no alias). Use the shim's tool-form: `hop <name> code` (if `code` is on PATH) — the shim rewrites this to `command hop -R <name> code`. Or invoke the binary directly: `hop -R <name> code`.

The `open` subcommand has been removed (no alias). Cross-platform abstraction was dropped along with the `internal/platform` package. Use the shim's tool-form: `hop <name> open` (Darwin) or `hop <name> xdg-open` (Linux). Or invoke the binary directly: `hop -R <name> open`.

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
| 2 | `errExitCode{code: 2}` — used by `cd`, `shell-init`, and `-R` for usage errors |
| 130 | `errFzfCancelled` — fzf user cancellation (Esc / Ctrl-C) |

Cobra's `SilenceUsage: true` and `SilenceErrors: true` are set on `rootCmd`, so `translateExit` is the sole stderr/exit path for top-level errors.

The `-R` flag bypasses cobra entirely (pre-Execute argv inspection) and uses `os.Exit` directly with the child's exit code (or 2 for usage errors, 1 for resolution errors).

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
| `git` | `clone.go::gitMissingHint` | `hop: git is not installed.` (also reused by `hop config scan` — lazy-checked at the first `git remote` invocation; empty scan trees with zero `.git` discoveries succeed without invoking `git`) |
| `<cmd>` for `-R` / tool-form | `main.go::runDashR` (formatted) | `hop: -R: '<cmd>' not found.` (when `<cmd>` is missing on PATH at exec time). Covers tool-form invocations like `hop <name> open` / `hop <name> xdg-open` since both rewrite to `-R` |
| `brew` | `internal/update` | `hop update: brew not found on PATH.` (only when binary is brew-installed) |

The fzf/git hints trigger `errSilent` (exit 1) directly — the subcommand writes the hint to `cmd.ErrOrStderr()` and returns `errSilent`. The `-R` path bypasses cobra and writes directly to `os.Stderr` via `runDashR`. `brew` follows a slightly different path: `internal/update.Run` writes the hint and returns `proc.ErrNotFound`; the cobra wrapper in `cmd/hop/update.go` then catches `proc.ErrNotFound` via `errors.Is` and converts it to `errSilent`. The user-visible behavior is identical (single hint line on stderr, exit 1) — the indirection exists so `internal/update` stays free of cobra-specific sentinels.

## `hop cd` binary-form text

`cd.go::cdHint`:

```
hop: 'cd' is shell-only. Add 'eval "$(hop shell-init zsh)"' to your zshrc, or use: cd "$(hop where "<name>")"
```

## `hop shell-init <shell>` emitted text

The shared portion (`shell_init.go::posixInit`) is identical for zsh and bash — both shells understand `[[ ]]`, `${@:N}` slicing, and `local`. Only the appended completion script differs.

The shared `posixInit` defines:

- `hop()` function with this 4-step resolution ladder (top-down, first match wins). The grammar is `subcommand` xor `repo` in `$1` — never a tool — so `$1` has only two interpretations and the ladder needs no PATH inspection or builtin/keyword filtering:
  1. **No args** → `command hop` (bare picker).
  2. **`__complete*`** → `command hop "$@"`. Cobra's hidden completion entrypoint must reach the binary; without this branch the function would route `__complete` to the bare-name dispatcher and break tab completion.
  3. **Known subcommand** (`cd|clone|where|ls|shell-init|config|update|help|--help|-h|--version|completion`) → `_hop_dispatch "$@"`. The `help` token is in this list because cobra auto-wires `hop help [subcommand]`. (No `open` token — the `open` subcommand was removed; `hop open` falls into the otherwise branch and is treated as a repo name.)
  4. **Flag-prefixed (`-*`)** → `command hop "$@"`.
  5. **Otherwise** — `$1` is treated as a repo name:
     - **`$# == 1`** → `_hop_dispatch cd "$1"` (bare-name → `cd`).
     - **`$2 == "-R"`** → `command hop -R "$1" "${@:3}"` (canonical exec form). The shim rewrites the user-facing `hop <name> -R <cmd>...` to the binary's internal shape `hop -R <name> <cmd>...` so `extractDashR` continues to see `-R` followed by `<name>` followed by `<cmd>...`.
     - **otherwise (`$# >= 2`, `$2 != -R`)** → `command hop -R "$1" "$2" "${@:3}"` (tool-form sugar). Missing tools surface via the binary's `hop: -R: '<cmd>' not found.` error — there is no shim-side PATH check or cheerful escape hatch.

The shim does NOT call `command -v` or `type` on `$1` or `$2`, and does NOT print cheerful errors. The pre-flip ladder needed those because `$1` could be a tool, repo, or subcommand; the post-flip grammar removes the overload at the source. Direct binary invocations of the tool-form (`/path/to/hop <name> <tool>`) hit cobra's `accepts at most 1 arg(s)` error — the tool-form is shim-only.
- `_hop_dispatch()` helper — handles the shell-mutating `cd` path (`command hop where "$2"` then `cd --`), and the URL-detected `clone` path (`cd --` to the printed path on success).
- `h() { hop "$@"; }` — single-letter alias.
- `hi() { command hop "$@"; }` — un-shadowed alias (calls the binary directly, bypassing the shim).

The cobra-generated completion is appended at runtime — `rootCmd.GenZshCompletion(out)` for zsh, `rootCmd.GenBashCompletionV2(out, true)` for bash. The `rootCmd` reference is captured in `main.go::rootForCompletion` (a package-level var set in `main()`). After the cobra completion, the shell-init emits the alias-completion line:
- zsh: `compdef _hop h hi`
- bash: `complete -o default -F __start_hop h hi`

so the `h` and `hi` aliases share the same completion logic — without this, tab completion would only work on `hop`, not on the aliases.

## Tool-form dispatch

The shim's tool-form sugar (`hop <name> <tool> [args...]`) is a generalization of the removed `hop code` and `hop open` subcommands to any binary on PATH. The repo name lives in `$1` and the tool name lives in `$2` — never the other way around:

| User types | Shim behavior | Binary sees |
|---|---|---|
| `hop dotfiles cursor` | otherwise → `command hop -R dotfiles cursor` | argv `[hop, -R, dotfiles, cursor]` → `extractDashR` → exec `cursor` with `cwd = <dotfiles>` |
| `hop outbox git status` | otherwise → `command hop -R outbox git status` | argv `[hop, -R, outbox, git, status]` → exec `git status` with `cwd = <outbox>` |
| `hop dotfiles /bin/pwd` | otherwise → `command hop -R dotfiles /bin/pwd` | exec `/bin/pwd` with `cwd = <dotfiles>` |
| `hop outbox pwd` | otherwise → `command hop -R outbox pwd` | exec `/bin/pwd` (the on-PATH binary, not the builtin) with `cwd = <outbox>` — prints the path. No special handling; the simpler grammar earns this redundancy |
| `hop outbox open` (Darwin) | otherwise → `command hop -R outbox open` | exec `open` with `cwd = <outbox>` — opens Finder at the repo dir |
| `hop outbox xdg-open .` (Linux) | otherwise → `command hop -R outbox xdg-open .` | exec `xdg-open .` with `cwd = <outbox>` |
| `hop outbox notarealbinary` | otherwise → `command hop -R outbox notarealbinary` | `hop: -R: 'notarealbinary' not found.`, exit 1 |
| `hop outbox -R git status` | otherwise → `command hop -R outbox git status` (canonical exec form, $2 == -R) | same as `hop outbox git status` |
| `hop where outbox` | rule 3 → `_hop_dispatch where outbox` | `hop where` runs (subcommand wins; the shim never tries tool-form for known subcommands) |
| `hop outbox` (1 arg, outbox is a repo) | otherwise → `_hop_dispatch cd outbox` | (no binary call; `cd` happens in the parent shell) |

The form is **shim-only**: the binary does not interpret it. Direct binary invocations (`/path/to/hop dotfiles cursor`) hit cobra's root which has `cobra.MaximumNArgs(1)` and errors with `accepts at most 1 arg(s), received 2`. Scripts and CI jobs that bypass the shim must use `hop -R <name> <tool>` explicitly.

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

All `brew` invocations route through `internal/proc` (Constitution Principle I — no direct `os/exec` outside `internal/proc`). The package exposes `Run(currentVersion string, out, errOut io.Writer) error` as its single public entry point. The `out`/`errOut` writers receive only the wrapper messages this package emits; subprocess output from `brew` is routed by `internal/proc` to the parent's `os.Stdout`/`os.Stderr` directly. Production callers pass `os.Stdout` and `os.Stderr` (via `cmd.OutOrStdout()` / `cmd.ErrOrStderr()`) to keep both consistent. When `brew` is missing on PATH, `Run` returns `proc.ErrNotFound`; the cobra wrapper converts that to `errSilent` so `translateExit` doesn't print a second error line.
