# CLI Surface

> Canonical contract for what the `hop` binary exposes to users.
> Source of truth for argument parsing, exit codes, stdout/stderr conventions, and help text.

## Subcommand Inventory

| Subcommand | Args | Behavior summary | Exit codes |
|---|---|---|---|
| `hop` | (none) | fzf picker over all repos; print selected absolute path on stdout | 0 selected, 130 cancelled |
| `hop <name>` | `<name>` | Match-or-fzf to a single repo; print absolute path on stdout | 0 selected, 1 no match, 130 cancelled |
| `hop where <name>` | `<name>` | Identical to `hop <name>` (explicit form). Renamed from v0.0.1's `hop path` for voice-fit with the binary name. | same as above |
| `hop -R <name> <cmd>...` | global flag + child argv | Resolve `<name>`, then exec `<cmd>...` with `cwd = <resolved-path>` and inherited stdio. Bypasses cobra parsing for `<cmd>...` | child's exit code; 1 if resolution fails; 2 on usage error |
| `hop <tool> <name> [args...]` | (shim only) | Sugar for `hop -R <name> <tool> [args...]`. Implemented in `hop shell-init` output; the binary itself does not interpret this form. | tool's exit code; 1 if `<tool>` resolves to a builtin/missing or `<name>` fails to resolve |
| `hop open [<name>]` | optional `<name>` | Resolve; `open <path>` (Darwin) or `xdg-open <path>` (Linux) | 0 opened, 1 resolution failed, 2 unsupported OS |
| `hop cd <name>` | `<name>` | Binary form: print hint to stderr, exit 2. Shell-function form (after `eval`): cd into the resolved path. | Binary: 2. Shell function: 0 success, 1 no match |
| `hop clone [<name>] \| --all` | optional `<name>` or `--all` | Clone single (resolved) or all missing repos | 0 success, 1 path conflict, non-zero on git failure |
| `hop clone <url>` | 1 (URL form, detected by `looksLikeURL`) | Ad-hoc clone with auto-registration. Flags: `--group`, `--no-add`, `--no-cd`, `--name`. | 0 success, 1 missing group / path conflict / git failure |
| `hop ls` | (none) | Print all repos as `name<spaces>path` columns | 0 |
| `hop shell-init <shell>` | `zsh` or `bash` (required) | Emit shell function wrapper + cobra-generated completion to stdout | 0 success, 2 unsupported shell |
| `hop config init` | (none) | Bootstrap a starter `hop.yaml` at the resolved location | 0 written, 1 file exists, 2 write error |
| `hop config where` | (none) | Print the resolved config path on stdout. Renamed from v0.0.1's `config path`. | 0 resolved, 1 unresolvable |
| `hop update` | (none) | Self-update the `hop` binary via Homebrew. No-op (with hint) when the binary was not installed via brew. | 0 success, 1 brew failure |
| `hop -h \| --help \| help` | (none) | Print help text on stdout | 0 |
| `hop -v \| --version` | (none) | Print version string on stdout | 0 |

> `hop path` (v0.0.1) and `hop config path` (v0.0.1) have been removed without aliases. Use `hop where` and `hop config where`.

### Match Resolution Algorithm

Used by `hop`, `hop <name>`, `hop where`, `hop -R`, `hop open`, `hop cd`, `hop clone`.

1. Build the list of all known repos from `hop.yaml`. Each entry has `(Name, Group, Dir, URL, Path)`. The list preserves YAML source order (groups in `cfg.Groups` order, URLs within each group in source order).
2. If `<name>` is non-empty: filter by case-insensitive substring match on `Name` (not Path, not URL, not Group).
3. If exactly **1 match**: return it directly without invoking fzf.
4. Otherwise (0 matches OR 2+ matches): invoke fzf with these flags, piping the **full repo list** (not the filtered subset) on stdin so the user can clear the query inside fzf to browse all repos:
   ```
   fzf --query <name> --select-1 --height 40% --reverse --with-nth 1 --delimiter '\t'
   ```
   The `--select-1` flag makes fzf auto-select if its filter narrows to exactly 1.
5. If `<name>` is empty: invoke fzf without `--query` (full picker).

#### Group disambiguation in the picker

When two or more repos share the same `Name` across different groups, the displayed first column is `<name> [<group>]` rather than just `<name>`. When a name is unique across groups, no suffix is added. Two URLs in the *same* group whose derived `Name` collides still render an identical first column (intra-group collisions are out of scope; cross-group collisions are handled).

### Stdout / stderr Conventions

- **stdout**: resolved absolute paths (`hop`, `hop where`), the `hop ls` table, version string, config path (`hop config where`), shell integration (`hop shell-init <shell>`), help text, "Created <path>" message from `hop config init`, the landed path from `hop clone <url>` (used by the shell shim for cd-on-success).
- **stderr**: status messages (`clone: <url> → <path>`, `skip: <reason>`), error messages, hints. The `hop config init` post-write tip also goes to stderr.
- The `hop cd` binary form's exit-2 hint goes to **stderr**.
- `hop -R` inherits stdin/stdout/stderr from the parent — the child's output passes through unchanged.

### Behavioral Scenarios (GIVEN/WHEN/THEN)

#### Bare picker

> **GIVEN** `hop.yaml` lists 3 repos
> **WHEN** I run `hop` with no arguments
> **THEN** fzf opens with all 3 repos visible
> **AND** selecting one prints its absolute path to stdout
> **AND** exit code is 0

#### Unique substring match

> **GIVEN** `hop.yaml` has exactly one repo named `outbox`
> **WHEN** I run `hop outbox`
> **THEN** fzf is NOT invoked
> **AND** stdout is the absolute path to that repo
> **AND** exit code is 0

#### Ambiguous substring match

> **GIVEN** `hop.yaml` has repos `outbox` and `outbox-shared`
> **WHEN** I run `hop outbox`
> **THEN** fzf opens with both candidates filtered (`--query outbox`)
> **AND** if the user picks one, exit code 0
> **AND** if the user cancels (Esc), exit code 130

#### Zero substring match

> **GIVEN** `hop.yaml` has repos `alpha`, `beta`, `gamma`
> **WHEN** I run `hop zzz`
> **THEN** fzf opens with `--query zzz` and zero filtered candidates
> **AND** the user can clear the query inside fzf to see all repos and pick one
> **AND** if the user cancels, exit code 130

#### Group disambiguation in picker

> **GIVEN** `hop.yaml` has a repo named `tools` in group `default` and another named `tools` in group `vendor`
> **WHEN** I run `hop` (bare)
> **THEN** fzf shows two rows: `tools [default]` and `tools [vendor]`
> **AND** the path column (the unique key for match-back) distinguishes them

#### `hop cd` binary form

> **GIVEN** the user has NOT run `eval "$(hop shell-init zsh)"`
> **WHEN** they run `hop cd <name>`
> **THEN** the binary prints to stderr: `hop: 'cd' is shell-only. Add 'eval "$(hop shell-init zsh)"' to your zshrc, or use: cd "$(hop where "<name>")"`
> **AND** exit code is 2

#### `hop cd` shell-function form

> **GIVEN** the user has run `eval "$(hop shell-init zsh)"`
> **WHEN** they run `hop cd <name>`
> **THEN** the shell function calls `command hop where <name>` to resolve
> **AND** runs `cd -- <resolved-path>`
> **AND** the parent shell's working directory is changed

#### Bare-name dispatch (shell shim)

> **GIVEN** the user has run `eval "$(hop shell-init zsh)"` and `hop.yaml` has a repo named `outbox`
> **WHEN** they run `hop outbox` (no subcommand)
> **THEN** the shim recognizes `outbox` is not a known subcommand or flag and routes through `_hop_dispatch`
> **AND** the shim runs `command hop where outbox` to resolve, then `cd --` into the path
> **AND** the parent shell's cwd is changed (no need to type `hop cd`)

`h <name>` (single-letter alias) behaves identically; `hi <name>` bypasses the shim and invokes the binary directly.

#### `hop -R` exec-in-context

> **GIVEN** `hop.yaml` resolves `outbox` to `~/code/sahil87/outbox`
> **WHEN** I run `hop -R outbox git status`
> **THEN** `git status` runs with `cwd = ~/code/sahil87/outbox`
> **AND** stdin/stdout/stderr are inherited (interactive prompts work)
> **AND** the parent shell's cwd is unchanged
> **AND** exit code matches `git status`'s exit code

> **GIVEN** an arbitrary child command with its own flags
> **WHEN** I run `hop -R outbox jq '.foo' file.json`
> **THEN** `<cmd>...` argv is forwarded verbatim — cobra does NOT try to parse `jq`'s flags as `hop` flags
> **AND** the child receives `jq '.foo' file.json` as its argv

> **GIVEN** `<name>` matches no repo
> **WHEN** I run `hop -R nope echo hi`
> **THEN** stderr shows the standard match-or-fzf no-candidate behavior
> **AND** exit code is 1 (resolution failed)

> **GIVEN** `<cmd>` is not on PATH
> **WHEN** I run `hop -R outbox notarealbinary`
> **THEN** stderr shows `hop: -R: 'notarealbinary' not found.`
> **AND** exit code is 1

#### `hop <tool> <name>` shim sugar

The shim emitted by `hop shell-init` recognizes a tool-form: when `$1` is an
executable on PATH and `$2` is non-flag, it rewrites the call to
`command hop -R "$2" "$1" "${@:3}"`. The binary itself does NOT interpret this
form — invoking the binary directly with `hop cursor dotfiles` argv just hits
cobra's "accepts at most 1 arg" error.

Resolution order in the shim's `hop()` function (precedence ladder):

1. No args → bare picker (`command hop`).
2. `$1` is a flag (`-R`, `-h`, `-v`, ...) → `command hop "$@"`.
3. `$1` is `__complete*` → `command hop "$@"` (cobra's hidden completion entrypoint).
4. `$1` is a known subcommand (`cd`, `clone`, `where`, `ls`, `open`, `shell-init`, `config`, `update`, `--help`, `-h`, `--version`, `completion`) → `_hop_dispatch "$@"`. **Subcommand wins over tool**: a binary on PATH named the same as a subcommand can never be reached as tool-form through the shim — the user must spell it as `hop -R <repo> <tool>`.
5. `$1` is the only argument → `_hop_dispatch cd "$1"` (bare-name → `cd`). **Repo wins over tool** for the 1-arg form: even if `$1` is also a binary on PATH, the shim treats it as a repo name. With 2+ args there is no competing repo interpretation, so tool-form fires.
6. `$1` is on PATH (absolute) AND `$2` is non-flag → tool-form: `command hop -R "$2" "$1" "${@:3}"`.
7. `$1` is a builtin/keyword/alias/function (non-empty `command -v` but no leading slash) AND `$2` is non-flag → cheerful stderr error suggesting `hop where <repo>` and `hop -R <repo> /full/path/to/<tool>`; exit 1.
8. `$1` is not on PATH at all (empty `command -v`) AND `$2` is non-flag → cheerful stderr error suggesting `hop where <token>`; exit 1.
9. Otherwise (`$2` is a flag) → `command hop "$@"` (let the binary surface the error).

Steps 7 and 8 exist purely for UX: without them, the calls would fall through to the binary which errors with cobra's terse "accepts at most 1 arg(s)" — useless for the user to debug. The cheerful errors are emitted by the shim, NOT the binary. Direct binary invocations (`/path/to/hop pwd dotfiles`) still hit cobra's terse error.

The PATH check uses `command -v "$1"` and tests that the result begins with `/`. Builtins, keywords, aliases, and shell functions return bare names (e.g. `pwd`, `if`, `cd`) and fail this check — so `hop pwd dotfiles` does NOT fire tool-form (`pwd` is a builtin) and falls through to step 7 where the binary errors. Users wanting to invoke `/bin/pwd` as a tool can spell the absolute path: `hop /bin/pwd dotfiles`.

> **GIVEN** `cursor` is on PATH and `dotfiles` resolves uniquely
> **WHEN** I run `hop cursor dotfiles` under the shim
> **THEN** the shim runs `command hop -R dotfiles cursor`
> **AND** the binary execs `cursor` with `cwd = <dotfiles-path>`
> **AND** exit code matches `cursor`'s

> **GIVEN** the user has a repo named `cursor` AND `cursor` is also on PATH
> **WHEN** I run `hop cursor` (1 arg) under the shim
> **THEN** the shim treats it as bare-name `cd` (step 5) — `cd` into the cursor repo
> **WHEN** I run `hop cursor dotfiles` (2 args)
> **THEN** the shim treats it as tool-form (step 6) — runs `cursor` in dotfiles

> **GIVEN** `ls` is both a known subcommand AND a binary on PATH
> **WHEN** I run `hop ls outbox` under the shim
> **THEN** the shim dispatches to the `hop ls` subcommand (step 4 wins over step 6) — cobra rejects the extra `outbox` arg

> **GIVEN** `pwd` is a shell builtin (`command -v pwd` returns `pwd`, no leading slash)
> **WHEN** I run `hop pwd dotfiles` under the shim
> **THEN** the shim writes a cheerful 3-line stderr message:
>   ```
>   hop: 'pwd' is a shell builtin (not a binary), so it can't run as a tool inside a repo.
>     - To get the path: hop where dotfiles
>     - To run a binary by that name: hop -R dotfiles /full/path/to/pwd
>   ```
> **AND** exits 1 (does NOT call the binary)

> **GIVEN** `nonexistent` is not on PATH and not a known subcommand
> **WHEN** I run `hop nonexistent dotfiles` under the shim
> **THEN** the shim writes a cheerful 3-line stderr message:
>   ```
>   hop: 'nonexistent' is not a known subcommand or a binary on PATH.
>     - If you meant tool-form: install 'nonexistent' or check the spelling.
>     - If you meant the path of 'nonexistent': hop where nonexistent
>   ```
> **AND** exits 1 (does NOT call the binary)

#### `hop clone <name>` (registered repo)

> **GIVEN** `<name>` resolves to `(name=foo, path=~/code/foo, url=git@github.com:user/foo.git)` and `~/code/foo` does not exist
> **WHEN** I run `hop clone foo`
> **THEN** stderr shows `clone: git@github.com:user/foo.git → ~/code/foo`
> **AND** `git clone git@github.com:user/foo.git ~/code/foo` runs (10-minute timeout)
> **AND** exit code matches git's exit code

> **GIVEN** the same resolution, but `~/code/foo/.git` already exists
> **WHEN** I run `hop clone foo`
> **THEN** stderr shows `skip: already cloned at ~/code/foo`
> **AND** exit code is 0

> **GIVEN** the same resolution, but `~/code/foo` exists and is NOT a git repo
> **WHEN** I run `hop clone foo`
> **THEN** stderr shows `hop clone: ~/code/foo exists but is not a git repo`
> **AND** exit code is 1

#### `hop clone --all`

> **GIVEN** `hop.yaml` has 5 repos, 2 already cloned
> **WHEN** I run `hop clone --all`
> **THEN** stderr shows `clone:` lines for the 3 missing and `skip:` lines for the 2 cloned
> **AND** the final stderr line is `summary: cloned=3 skipped=2 failed=0`
> **AND** exit code is 0 if `failed == 0`, else non-zero

#### `hop clone <url>` — ad-hoc URL clone with auto-registration

`hop clone` distinguishes URL form from name form via `looksLikeURL`: the argument contains `://` OR (`@` AND `:`). On URL form:

1. Resolve the target group (`--group <name>`, default `default`). Missing group → exit 1 with `hop: no '<group>' group in <config-path>. ...`.
2. Compute landing path:
   - Map-shaped group with `dir:` set: `<dir>/<name>`.
   - Flat group: `<code_root>/<org-from-url>/<name-from-url>` (the `org` segment is dropped if the URL has none).
   - `--name <override>` replaces the URL-derived name.
3. Classify on-disk state and act:
   - **Missing path** → `git clone <url> <path>`, then (unless `--no-add`) append URL to `hop.yaml` via `internal/yamled.AppendURL`. Print landed path to stdout (unless `--no-cd`).
   - **Already cloned** (`<path>/.git` exists) → emit `skip: already cloned at <path>` to stderr; still appends YAML and prints path (registers an existing checkout).
   - **Path exists, not a git repo** → emit `hop clone: <path> exists but is not a git repo`; exit 1; no YAML write, no stdout.
4. URL already in target group's `urls` list → emit `skip: <url> already registered in '<group>'` to stderr; no YAML write; still print path (unless `--no-cd`) so the shim can `cd` to it.

The YAML write is **comment-preserving and atomic** (temp file + rename via `internal/yamled`); see [architecture.md](architecture.md#internalyamled).

> **GIVEN** `hop.yaml` has a `default` flat group, `code_root = ~/code`, and `~/code/sahil87/loom` does not exist
> **WHEN** I run `hop clone git@github.com:sahil87/loom.git`
> **THEN** `git clone` runs into `~/code/sahil87/loom`
> **AND** the URL is appended to the `default` group in `hop.yaml` (comments preserved, atomic write)
> **AND** stdout is `~/code/sahil87/loom` (consumed by the shim's `cd`)
> **AND** exit code is 0

> **GIVEN** the same setup, plus `--group vendor` and a map-shaped `vendor: { dir: ~/vendor, urls: [...] }` group
> **WHEN** I run `hop clone --group vendor git@github.com:other/tool.git`
> **THEN** the landing path is `~/vendor/tool`
> **AND** the URL is appended to `vendor.urls` in `hop.yaml`

> **GIVEN** `--no-add` is passed
> **WHEN** I run `hop clone --no-add <url>`
> **THEN** the clone proceeds but `hop.yaml` is NOT modified

> **GIVEN** `--no-cd` is passed
> **WHEN** I run `hop clone --no-cd <url>` (under the shim or not)
> **THEN** stdout suppresses the landed path, so the shim does not `cd`

> **GIVEN** `--name foo`
> **WHEN** I run `hop clone --name foo git@github.com:user/bar.git`
> **THEN** the landing path uses `foo`, not the URL-derived `bar`

#### `hop ls`

> **GIVEN** `hop.yaml` has 3 repos across 2 groups (preserving source order: group A then group B)
> **WHEN** I run `hop ls`
> **THEN** stdout shows 3 rows in YAML source order, each `name<spaces>path`, aligned (column-style)
> **AND** exit code is 0
> **AND** an empty `hop.yaml` produces no output (still exit 0)

#### `hop shell-init <shell>`

> **WHEN** I run `hop shell-init zsh`
> **THEN** stdout contains the shared `posixInit` prefix defining `hop()`, `_hop_dispatch()`, `h()`, `hi()` (with bare-name dispatch + tool-form)
> **AND** stdout contains the cobra-generated `_hop` completion function (appended at runtime via `rootCmd.GenZshCompletion`)
> **AND** stdout contains `compdef _hop h hi` so the `h` and `hi` aliases share the completion
> **AND** running `eval "$(hop shell-init zsh)"` in a zsh shell defines `hop` as a function (verifiable via `whence -w hop`)
> **AND** exit code is 0

> **WHEN** I run `hop shell-init bash`
> **THEN** stdout contains the same shared `posixInit` prefix (works in both shells — uses `[[ ]]`, `${@:N}`, `local`)
> **AND** stdout contains the cobra-generated `__start_hop` bash completion function (via `rootCmd.GenBashCompletionV2`)
> **AND** stdout contains `complete -o default -F __start_hop h hi` so the aliases share the completion
> **AND** exit code is 0

> **WHEN** I run `hop shell-init` with no shell argument
> **THEN** stderr shows `hop shell-init: missing shell. Supported: zsh, bash`
> **AND** exit code is 2

> **WHEN** I run `hop shell-init fish`
> **THEN** stderr shows `hop shell-init: unsupported shell 'fish'. Supported: zsh, bash`
> **AND** exit code is 2

#### `hop --version` / `-v`

> **WHEN** I run `hop --version` or `hop -v`
> **THEN** stdout is a single line containing the version string (e.g., `v0.1.0` or `v0.1.0-2-gabc123` for dev builds from `git describe`)
> **AND** exit code is 0

> **NOTE**: Cobra also auto-wires a `hop version` subcommand from `rootCmd.Version`; this still works (no effort spent suppressing it).

#### `hop update`

`hop update` self-upgrades the binary via Homebrew. It MUST detect whether the binary was installed via brew (by walking `os.Executable` through `EvalSymlinks` and checking for `/Cellar/` in the resolved path); when it wasn't, it MUST exit 0 after printing a hint pointing at the manual install command — the binary cannot upgrade what it didn't install.

The brew formula is referenced as `sahil87/tap/hop` (fully qualified) to disambiguate from the Homebrew core `hop` cask (an HWP document viewer) that would otherwise shadow the formula.

Version comparison MUST normalize the leading `v` — the binary reports versions with the `v` prefix (e.g. `v0.0.3` from the build's `git describe` ldflag), while `brew info --json=v2` reports the bare form (`0.0.3`). The comparison uses the bare form on both sides.

> **GIVEN** the binary was installed via Homebrew and the tap formula is at the same version
> **WHEN** I run `hop update`
> **THEN** stdout shows `Current version: v<X>`, then `Checking for updates...`, then `Already up to date (v<X>).`
> **AND** exit code is 0
> **AND** `brew upgrade` is NOT invoked

> **GIVEN** the binary was installed via Homebrew and the tap has a newer version
> **WHEN** I run `hop update`
> **THEN** stdout shows `Updating v<old> → v<new>...` followed by `brew upgrade` output
> **AND** on success, stdout ends with `Updated to v<new>.`
> **AND** exit code is 0

> **GIVEN** the binary was NOT installed via Homebrew (e.g. `just local-install`, manual `go install`, or downloaded tarball)
> **WHEN** I run `hop update`
> **THEN** stdout shows `hop v<X> was not installed via Homebrew.` followed by a manual-update hint pointing at `brew install sahil87/tap/hop`
> **AND** `brew` is NOT invoked
> **AND** exit code is 0

> **GIVEN** `brew update` or `brew info` fails (network error, brew not on PATH, etc.)
> **WHEN** I run `hop update`
> **THEN** stderr shows the failure reason
> **AND** exit code is 1

### External Tool Availability

External tools (`fzf`, `git`, `code`, `open`, `xdg-open`) are checked **lazily** — only when the subcommand actually needs them. Subcommands that resolve without an external tool MUST NOT preemptively check or fail.

| Tool | Required by | Behavior if missing |
|---|---|---|
| `fzf` | `hop`, `hop <name>` (when match is ambiguous), `hop where` (ambiguous), `hop -R` (ambiguous), `hop open` (ambiguous), `hop clone <name>` (ambiguous) | Print to stderr: `hop: fzf is not installed. Install it: brew install fzf (macOS) or apt install fzf (Debian).` Exit 1. |
| `git` | `hop clone` (any form) | Print to stderr: `hop: git is not installed.` Exit 1. |
| `open` (Darwin) / `xdg-open` (Linux) | `hop open` | Print to stderr: `hop open: '<tool>' not found.` Exit 1. |
| `<cmd>` | `hop -R <name> <cmd>...` (and the shim's `hop <tool> <name>` sugar that rewrites to it) | Print to stderr: `hop: -R: '<cmd>' not found.` Exit 1. |
| `brew` | `hop update` (when installed via brew) | Print to stderr: `hop update: brew not found on PATH.` Exit 1. |

Subcommands that don't need a tool MUST work without it. Examples:
- `hop where foo` (when `foo` is a unique substring match) does not invoke fzf — works without `fzf` installed.
- `hop ls` does not invoke any external tool.
- `hop shell-init zsh` and `hop shell-init bash` do not invoke any external tool — emit stdout text only.
- `hop config init` and `hop config where` do not invoke any external tool.

### Help Text

`hop -h | --help | help` emits help text to stdout. Cobra renders the help; the `Usage:` table and `Notes:` block come from `rootLong` in `src/cmd/hop/root.go`. Top-level structure mirrors the inventory table above.

The `Notes:` block in `rootLong` documents:
- `hop cd` requires the shell integration; without it, use `cd "$(hop where <name>)"` or `cd "$(hop <name>)"`.
- The shim's `hop <tool> <name>` tool-form sugar (and that it isn't recognized by the binary directly).
- Config search order: `$HOP_CONFIG`, then `$XDG_CONFIG_HOME/hop/hop.yaml`, then `$HOME/.config/hop/hop.yaml`.
- Run `hop config init` to bootstrap.

### Cobra Wiring

- `rootCmd` is defined in `src/cmd/hop/root.go::newRootCmd()`.
- Each subcommand has its own file under `src/cmd/hop/` with a `func newXxxCmd() *cobra.Command` factory.
- `main.go::main()`:
  1. Builds `rootCmd := newRootCmd()`.
  2. Sets `rootCmd.Version = version` (the package-level `var version = "dev"`, overridden via `-ldflags "-X main.version=…"` at build time).
  3. Captures `rootForCompletion = rootCmd` so `shell-init` can call `GenZshCompletion` / `GenBashCompletionV2` without threading `rootCmd` through factories.
  4. Inspects `os.Args` for `-R` via `extractDashR` (pre-cobra). If present, resolves the target via `resolveByName`, then calls `proc.RunForeground` and `os.Exit(code)` — bypassing cobra entirely.
  5. Otherwise calls `rootCmd.Execute()`. Errors are mapped to exit codes via `translateExit`.
- `rootCmd.SilenceUsage = true` and `rootCmd.SilenceErrors = true` — `translateExit` is the sole stderr/exit path.
- The bare-form behavior (`hop` with no subcommand, `hop <name>` with one positional arg) is implemented via `rootCmd.RunE` checking args and dispatching to the same `resolveAndPrint` helper used by `hop where`.

#### Why `-R` bypasses cobra

Cobra's parser would try to interpret `<cmd>...` after `-R <name>` as `hop`'s own subcommand or its flags, breaking arbitrary children like `hop -R name git status` or `hop -R name jq '.foo' file.json`. Pre-Execute argv inspection (`extractDashR`) splits argv into the hop portion (`-R <name>`) and the child portion (everything else) so the child runs with its own argv intact. Unit-tested in `dashr_test.go`.

### Exit Code Conventions

Defined centrally in `main.go::translateExit`:

| Code | Meaning |
|---|---|
| 0 | Success |
| 1 | Application error (no match, missing tool, file already exists, write error, child resolution error, etc.); also `errSilent` (caller already wrote stderr) |
| 2 | Usage error (`cd` binary form, `shell-init` missing/unsupported shell, `-R` usage error) |
| 130 | User cancelled — fzf Esc / Ctrl-C (`errFzfCancelled`) |

The `-R` flag bypasses cobra entirely and uses `os.Exit` directly with the child's exit code (or 2 for usage errors, 1 for resolution errors).

### Design Decisions

1. **`hop cd` is intentionally split between binary and shell function.** A binary cannot change its parent shell's `cwd`; the function wrapper (emitted by `hop shell-init zsh`) does. The binary's role is to print a hint when invoked directly, so users discover the shell integration.
2. **Bare-name dispatch lives only in the shim, not the binary.** `hop outbox` from the binary still prints the path (so `cd "$(hop outbox)"` and shell-pipelines work). The shim's bare-name dispatch is a UX layer added on top — invoking the binary directly remains a pure path printer.
3. **`fzf` is invoked lazily, not preflighted.** Subcommands that don't need fzf (`hop ls`, `hop shell-init zsh`, `hop config *`, exact-match resolutions) work without it installed. This matters for minimal environments and CI.
4. **`-R` bypasses cobra rather than using `cobra.Command{DisableFlagParsing: true}`.** Pre-Execute argv inspection is a single small function (`extractDashR`); the alternative would entangle every flag-parsing path with `-R`-aware logic. Unit tests cover the split logic without spawning the binary. Spelled `-R` (not `-C`) because hop primarily operates on **repos** (not arbitrary directories like `git -C` / `make -C` / `tar -C`); `-R` reads as "repo" at the call site.
5. **Match algorithm is substring-on-`Name` only.** Not Path, not URL, not Group. Simple, predictable, matches the bash original. Group disambiguation is a display-time concern only (`buildPickerLines` adds `[<group>]` suffix when names collide across groups).
6. **`hop where` and `hop config where` use the same verb for symmetry.** Both answer "where would this go / where does this resolve to?" The v0.0.1 names (`path`, `config path`) lacked voice-fit with the new binary name and were renamed without aliases (no migration path; the rename was a clean break for v0.x).
7. **`hop clone <url>` infers form from argument shape.** `looksLikeURL` (contains `://` OR (`@` AND `:`)) splits URL form from name form. This keeps `clone` to one verb rather than `clone-url` / `clone-name`. URLs of registered repos still go through name form via `hop clone <name>` — there's no ambiguity because the URL form requires an actual URL shape.
8. **Auto-registration on `hop clone <url>` is opt-out, not opt-in.** The default behavior for an ad-hoc URL clone is "I want this in my registry"; `--no-add` is the escape valve. This matches the dominant use case (try a new repo → keep it). The YAML write is comment-preserving (via `internal/yamled`) so registration doesn't trash hand-curated comments.
9. **`hop update` is a top-level subcommand, not `hop config update` or a flag.** Per Constitution Principle VI, new top-level subcommands need explicit justification. Self-update is a binary-state operation, not config-state — it doesn't fit under `config`, and overloading a flag on the root (e.g. `hop --update`) muddles the bare-form's "print path" semantics. It also matches the convention every Homebrew-installed CLI uses (`fab-kit update`, `gh extension upgrade`). The implementation lives in `internal/update` and routes all subprocess invocations through `internal/proc` per Constitution Principle I (no direct `os/exec` outside `internal/proc`).
10. **`hop <tool> <name>` is shim-only sugar, not a binary parsing rule.** The shim rewrites tool-form to `command hop -R <name> <tool> ...` before delegating to the binary, so the binary's grammar stays the simple "first token is a subcommand or repo name" model. Putting it in the binary would require a maintained reserved-subcommand list (`ls`, `where`, `cd`, ...), a maintained PATH-detection rule, and a precedence ladder for collisions — all permanent surface-area cost in exchange for a UX win that only matters when the user is already inside a shell. Per Constitution Principle VI ("could this be a separate tool?"), the shim is that separate tool. The trade-off: tool-form is unavailable to scripts and CI invocations of the `hop` binary; those use cases use `hop -R <name> <tool>` explicitly.
11. **`hop code` was removed in favor of `hop code <name>` via tool-form.** Once the shim dispatches `hop <tool> <name>`, a dedicated `code` subcommand is redundant — `hop code dotfiles` (shim) → `command hop -R dotfiles code` → execs `code` with cwd = dotfiles. This removes a top-level subcommand and the `code`-specific install-hint code path. Note: the `code` token is no longer in the shim's known-subcommand case-list, so the shim correctly routes `hop code <repo>` through tool-form rather than dispatching to a (no-longer-existent) subcommand. There is no compatibility shim — this is a clean break, consistent with the v0.x policy of renaming/removing subcommands without aliases.
12. **Tool-form filters builtins/keywords via leading-slash check on `command -v`.** Builtins (`pwd`, `cd`, `echo`, `[`), keywords (`if`, `for`, `case`), aliases, and shell functions return bare names from `command -v`. Only real binaries return absolute paths. Filtering them out prevents the shim from rewriting `hop pwd dotfiles` into `command hop -R dotfiles pwd` (which would invoke `/bin/pwd` — almost never what the user means and confusingly different from the shell builtin in `cd` semantics). Users who genuinely want `/bin/pwd` as the tool can spell the absolute path: `hop /bin/pwd dotfiles` (the `/*` check passes for absolute paths).
