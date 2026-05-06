# CLI Subcommands

What each subcommand of the `hop` binary actually does. Source files live in `src/cmd/hop/`.

Match resolution algorithm used by `hop`, `hop where`, `hop open`, `hop cd`, `hop clone`, `hop -R` is documented separately in [match-resolution](match-resolution.md).

## Inventory

| Subcommand | File | Args | Behavior |
|---|---|---|---|
| `hop` (bare) | `root.go` | 0 or 1 positional | Resolves via match-or-fzf, prints abs path on stdout |
| `hop where <name>` | `where.go` | exactly 1 | Same handler as bare form (`resolveAndPrint`). Renamed from v0.0.1's `path` for voice-fit with the binary name |
| `hop open [<name>]` | `open.go` | 0 or 1 | Resolves, calls `platform.Open(ctx, path)` |
| `hop cd <name>` | `cd.go` | any | Always exits 2 with the binary-form hint; the shell wrapper from `shell-init <shell>` is what actually changes cwd |
| `hop clone [<name>]` / `--all` | `clone.go` | 0 or 1, plus `--all` flag | Single resolves via match-or-fzf; `--all` iterates the full list and prints a summary |
| `hop clone <url>` | `clone.go` | 1 (URL form) | Ad-hoc clone with auto-registration. Detects URL via `looksLikeURL`. Supports `--group`, `--no-add`, `--no-cd`, `--name` flags. See [Ad-hoc URL clone](#ad-hoc-url-clone) below |
| `hop ls` | `ls.go` | none (`cobra.NoArgs`) | Prints aligned `name<spaces>path` rows; empty list prints nothing |
| `hop shell-init <shell>` | `shell_init.go` | exactly 1 | `zsh` → emits `posixInit` prefix + cobra-generated `_hop` zsh completion + `compdef _hop h hi`; `bash` → `posixInit` + cobra-generated `__start_hop` bash completion + `complete -o default -F __start_hop h hi`; missing or other → exit 2 with exact stderr |
| `hop config init` | `config.go` | none | Writes embedded `starter.yaml` to `ResolveWriteTarget()` |
| `hop config where` | `config.go` | none | Prints `ResolveWriteTarget()` to stdout (never errors on missing file). Renamed from v0.0.1's `config path` for consistency with `hop where` |
| `hop update` | `update.go` | none | Self-update via Homebrew. Detects brew install via `os.Executable` + `EvalSymlinks` + `/Cellar/` substring; non-brew installs exit 0 with a manual-update hint. Logic lives in `internal/update`; subprocess calls go through `internal/proc` |
| `hop -R <name> <cmd>...` | `main.go` | global flag + child argv | Resolves `<name>` to a path, then execs `<cmd>...` with `Dir=<path>` and inherited stdio. Implemented via pre-Execute argv inspection in `main.go::extractDashR`; bypasses cobra subcommand parsing for the post-`<name>` argv. Spelled `-R` (not `-C`) because hop is repo-scoped, not directory-scoped. Tab-completion: `hop -R <TAB>` completes repo names from `hop.yaml` (no subcommand-collision filter — repos named like subcommands are valid `-R` targets); `hop -R <name> <TAB>` returns no candidates (child argv is not hop's). See [Tab completion](#tab-completion) below |
| `hop <tool> <name> [args...]` | (shim only) | 2+ args | Sugar for `hop -R <name> <tool> [args...]`. Lives in `shell_init.go::posixInit`, NOT the binary. The binary errors on this argv shape (cobra's max-1-arg root). Tab-completion: `hop <tool> <TAB>` completes repo names when `<tool>` is on PATH at an absolute path AND not a known hop subcommand (mirrors shim rules 4 and 6). See [Tool-form dispatch](#tool-form-dispatch) below |
| `hop -v` / `hop --version` | cobra | — | Auto-wired by cobra when `rootCmd.Version` is set; output is the `var version` value (default `dev`, overridden via `-ldflags "-X main.version=..."`) |
| `hop help` / `-h` / `--help` | cobra | — | Cobra-rendered help, with `rootLong` providing the `Usage:` table and `Notes:` block from `root.go` |

## Removed subcommands

The `path` subcommand has been removed (no alias). Use `hop where <name>` or the bare form `hop <name>`.

The `config path` subcommand has been removed (no alias). Use `hop config where`.

The `code` subcommand has been removed (no alias). Use the shim's tool-form: `hop code <name>` (if `code` is on PATH) — the shim rewrites this to `command hop -R <name> code`. Or invoke the binary directly: `hop -R <name> code`.

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
| `git` | `clone.go::gitMissingHint` | `hop: git is not installed.` |
| `open`/`xdg-open` | `open.go` (formatted) | `hop open: '<tool>' not found.` (`<tool>` from `platform.OpenTool()`) |
| `<cmd>` for `-R` / tool-form | `main.go::runDashR` (formatted) | `hop: -R: '<cmd>' not found.` (when `<cmd>` is missing on PATH at exec time) |
| `brew` | `internal/update` | `hop update: brew not found on PATH.` (only when binary is brew-installed) |

The fzf/git/open/xdg-open hints trigger `errSilent` (exit 1) directly — the subcommand writes the hint to `cmd.ErrOrStderr()` and returns `errSilent`. The `-R` path bypasses cobra and writes directly to `os.Stderr` via `runDashR`. `brew` follows a slightly different path: `internal/update.Run` writes the hint and returns `proc.ErrNotFound`; the cobra wrapper in `cmd/hop/update.go` then catches `proc.ErrNotFound` via `errors.Is` and converts it to `errSilent`. The user-visible behavior is identical (single hint line on stderr, exit 1) — the indirection exists so `internal/update` stays free of cobra-specific sentinels.

## `hop cd` binary-form text

`cd.go::cdHint`:

```
hop: 'cd' is shell-only. Add 'eval "$(hop shell-init zsh)"' to your zshrc, or use: cd "$(hop where "<name>")"
```

## `hop shell-init <shell>` emitted text

The shared portion (`shell_init.go::posixInit`) is identical for zsh and bash — both shells understand `[[ ]]`, `${@:N}` slicing, and `local`. Only the appended completion script differs.

The shared `posixInit` defines:

- `hop()` function with this resolution ladder (top-down, first match wins):
  1. **No args** → `command hop` (bare picker).
  2. **`__complete*`** → `command hop "$@"`. Cobra's hidden completion entrypoint must reach the binary; without this branch the function would route `__complete` to the bare-name dispatcher and break tab completion.
  3. **Known subcommand** (`cd|clone|where|ls|open|shell-init|config|update|help|--help|-h|--version|completion`) → `_hop_dispatch "$@"`. **Subcommand wins over tool**: a binary named the same as a subcommand can't be reached as tool-form through the shim — user must spell `hop -R <repo> <tool>`. The `help` token is in this list because cobra auto-wires `hop help [subcommand]`; without it the shim would route `hop help` to bare-name `cd` (1 arg) or to the tool-form/cheerful-error path (`hop help open`, 2 args).
  4. **Flag-prefixed (`-*`)** → `command hop "$@"`.
  5. **Single arg, default case** → `_hop_dispatch cd "$1"` (bare-name → `cd`). **Repo wins over tool** for the 1-arg form: a token that's both a repo and a binary on PATH dispatches as repo.
  6. **2+ args, $1 is on PATH (leading-slash check on `command -v $1`), $2 not a flag** → `command hop -R "$2" "$1" "${@:3}"` (tool-form). The leading-slash check filters builtins/keywords/aliases/functions, which return bare names from `command -v`.
  7. **2+ args, $1 is a builtin/keyword/alias/function (non-empty `command -v` but no leading slash), $2 not a flag** → cheerful stderr error: `'$1' is a shell builtin (not a binary)…` plus two-bullet suggestion (`hop where $2` for path; `hop -R $2 /full/path/to/$1` for binary equivalent). Exits 1, does NOT call the binary.
  8. **2+ args, $1 not on PATH and not a builtin (empty `command -v`), $2 not a flag** → cheerful stderr error: `'$1' is not a known subcommand or a binary on PATH.` plus typo-fix and `hop where $1` suggestions. Exits 1, does NOT call the binary.
  9. **Otherwise** ($2 is a flag) → `command hop "$@"` (let the binary surface the error).

Steps 7 and 8 are pure UX additions — without them, the call would fall through to cobra's terse `accepts at most 1 arg(s), received 2`. They live in the shim, not the binary. Direct binary invocations still get cobra's terse error, which is the right behavior for scripts.
- `_hop_dispatch()` helper — handles the shell-mutating `cd` path (`command hop where "$2"` then `cd --`), and the URL-detected `clone` path (`cd --` to the printed path on success).
- `h() { hop "$@"; }` — single-letter alias.
- `hi() { command hop "$@"; }` — un-shadowed alias (calls the binary directly, bypassing the shim).

The cobra-generated completion is appended at runtime — `rootCmd.GenZshCompletion(out)` for zsh, `rootCmd.GenBashCompletionV2(out, true)` for bash. The `rootCmd` reference is captured in `main.go::rootForCompletion` (a package-level var set in `main()`). After the cobra completion, the shell-init emits the alias-completion line:
- zsh: `compdef _hop h hi`
- bash: `complete -o default -F __start_hop h hi`

so the `h` and `hi` aliases share the same completion logic — without this, tab completion would only work on `hop`, not on the aliases.

### Tab completion

Repo-name candidates fire at three slots, all backed by `repo_completion.go`:

| Argv shape | Hook | Candidates |
|---|---|---|
| `hop <TAB>` | root `ValidArgsFunction = completeRepoNames` | repo names from `hop.yaml` minus subcommand-collision names |
| `hop -R <TAB>` | `cmd.RegisterFlagCompletionFunc("R", completeRepoNamesForFlag)` against a hidden `-R` cobra flag (`StringP("R","R","","")` + `MarkHidden("R")` in `root.go::newRootCmd`) | all repo names — no collision filter, since `-R` routed via the flag, not the subcommand dispatcher |
| `hop <tool> <TAB>` | `completeRepoNames` falls through when `shouldCompleteRepoForSecondArg(cmd, args)` is true: `len(args)==1`, `args[0]` is not an available subcommand of `cmd`, and `exec.LookPath(args[0])` returns an absolute path (mirrors shim rules 4 and 6) | all repo names |

Position-3+ slots return no candidates: `hop -R <name> <TAB>` is detected via `cmd.Flag("R").Changed` in `completeRepoNames` (cobra has already absorbed `-R <name>`, so `args=[]` looks like bare `hop <TAB>` without this check); `hop <tool> <name> <TAB>` falls out via the `len(args) != 1` guard in `shouldCompleteRepoForSecondArg`.

For `hop -R <TAB>` to reach cobra's machinery, `main.go::main` skips `extractDashR` when `os.Args[1]` is `__complete` or `__completeNoDesc` (see `isCompletionInvocation` in [architecture/package-layout](../architecture/package-layout.md#cobra-wiring)). The `-R` cobra flag is dormant in normal execution — `extractDashR` consumes `-R` from `os.Args` before `Execute()` runs.

## Tool-form dispatch

The shim's tool-form sugar (`hop <tool> <name> [args...]`) is the canonical replacement for the removed `hop code` subcommand and a generalization to any binary on PATH:

| User types | Shim behavior | Binary sees |
|---|---|---|
| `hop cursor dotfiles` | rule 6 → `command hop -R dotfiles cursor` | argv `[hop, -R, dotfiles, cursor]` → `extractDashR` → exec `cursor` with `cwd = <dotfiles>` |
| `hop git outbox status` | rule 6 → `command hop -R outbox git status` | argv `[hop, -R, outbox, git, status]` → exec `git status` with `cwd = <outbox>` |
| `hop /bin/pwd dotfiles` | rule 6 → `command hop -R dotfiles /bin/pwd` | exec `/bin/pwd` with `cwd = <dotfiles>` (absolute path passes the leading-slash check) |
| `hop pwd dotfiles` | rule 7 → cheerful stderr (builtin), exit 1 | (binary is NOT called) |
| `hop nonexistent dotfiles` | rule 8 → cheerful stderr (typo / not on PATH), exit 1 | (binary is NOT called) |
| `hop ls outbox` | rule 3 → `_hop_dispatch ls outbox` | `hop ls` rejects extra arg (subcommand wins) |
| `hop dotfiles` (1 arg, dotfiles is a repo) | rule 5 → bare-name `cd` | (no binary call; `cd` happens in the parent shell) |
| `hop dotfiles` (1 arg, dotfiles is also a binary on PATH) | rule 5 still fires — repo wins for 1-arg form | (same as above) |

The form is **shim-only**: the binary doesn't interpret it. Direct binary invocations (`/path/to/hop cursor dotfiles`) hit cobra's root which has `cobra.MaximumNArgs(1)` and errors. Scripts and CI jobs must use `hop -R <name> <tool>` explicitly.

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
