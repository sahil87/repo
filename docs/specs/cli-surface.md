# CLI Surface

> Canonical contract for what the `hop` binary exposes to users.
> Source of truth for argument parsing, exit codes, stdout/stderr conventions, and help text.

## Subcommand Inventory

| Subcommand | Args | Behavior summary | Exit codes |
|---|---|---|---|
| `hop` | (none) | fzf picker over all repos; print selected absolute path on stdout | 0 selected, 130 cancelled |
| `hop <name>` | `<name>` | Binary form: print bare-name hint to stderr, exit 2 (1-arg dispatch is shell-only — shorthand for `hop <name> cd`). Shell-function form (after `eval`): cd into the resolved path. | Binary: 2. Shell function: 0 success, 1 no match |
| `hop <name> cd` | `<name> cd` | Binary form: print `cd` hint to stderr, exit 2 (cd is shell-only). Shell-function form: cd into the resolved path. | Binary: 2. Shell function: 0 success, 1 no match |
| `hop <name> where` | `<name> where` | Resolve `<name>` and print absolute path on stdout. Replaces v0.x's top-level `hop where <name>` subcommand (removed). | 0 selected, 1 no match, 130 cancelled |
| `hop <name> -R <cmd>...` | positional + flag + child argv | Resolve `<name>`, then exec `<cmd>...` with `cwd = <resolved-path>` and inherited stdio. Implemented in the shim, which rewrites to the binary's internal `command hop -R <name> <cmd>...` shape. | child's exit code; 1 if resolution fails; 2 on usage error |
| `hop <name> <tool> [args...]` | (shim only) | Sugar for `hop <name> -R <tool> [args...]`. Implemented in `hop shell-init` output; the binary itself errors with the tool-form hint (`'<tool>' is not a hop verb (cd, where)...`). | tool's exit code; 1 if `<tool>` is missing on PATH (via the binary's `-R` error path) or `<name>` fails to resolve |
| `hop clone [<name>] \| --all` | optional `<name>` or `--all` | Clone single (resolved) or all missing repos | 0 success, 1 path conflict, non-zero on git failure |
| `hop clone <url>` | 1 (URL form, detected by `looksLikeURL`) | Ad-hoc clone with auto-registration. Flags: `--group`, `--no-add`, `--no-cd`, `--name`. | 0 success, 1 missing group / path conflict / git failure |
| `hop ls` | (none) | Print all repos as `name<spaces>path` columns | 0 |
| `hop shell-init <shell>` | `zsh` or `bash` (required) | Emit shell function wrapper + cobra-generated completion to stdout | 0 success, 2 unsupported shell |
| `hop config init` | (none) | Bootstrap a starter `hop.yaml` at the resolved location | 0 written, 1 file exists, 2 write error |
| `hop config where` | (none) | Print the resolved config path on stdout. Renamed from v0.0.1's `config path`. | 0 resolved, 1 unresolvable |
| `hop config print` | (none) | Print the resolved `hop.yaml` contents to stdout (raw bytes, comment-preserving). | 0 success, 1 unresolvable / read error |
| `hop config scan <dir>` | exactly 1 (directory) | Walk `<dir>` (default `--depth 3`), discover git repos via stat + `git remote`, and emit a merged `hop.yaml` to stdout (default) or merge in place via `--write` (atomic, comment-preserving). Auto-derives groups: convention-match repos go to `default`; non-convention repos land in invented map-shaped groups keyed off the parent dir basename. | 0 success (incl. zero repos found); 1 missing `hop.yaml` / git missing / write failure; 2 usage error (missing arg, dir validation, `--depth < 1`) |
| `hop update` | (none) | Self-update the `hop` binary via Homebrew. No-op (with hint) when the binary was not installed via brew. | 0 success, 1 brew failure |
| `hop -h \| --help \| help` | (none) | Print help text on stdout | 0 |
| `hop -v \| --version` | (none) | Print version string on stdout | 0 |

> `hop path` (v0.0.1) and `hop config path` (v0.0.1) were removed without aliases. The top-level `hop where <name>` and `hop cd <name>` subcommands were removed in the v0.x repo-verb grammar flip — use `hop <name> where` and `hop <name> cd` (or the bare `hop <name>` shorthand) instead. `hop config where` survives unchanged (different namespace).

### Match Resolution Algorithm

Used by `hop`, `hop <name> where`, `hop <name> cd` (via the shim's `_hop_dispatch cd` → `command hop "$2" where`), `hop -R`, `hop clone`.

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

- **stdout**: resolved absolute paths (`hop` bare picker, `hop <name> where`), the `hop ls` table, version string, config path (`hop config where`), shell integration (`hop shell-init <shell>`), help text, "Created <path>" message from `hop config init`, the landed path from `hop clone <url>` (used by the shell shim for cd-on-success). `hop -R` and the shim's tool-form sugar inherit the child's stdout (no hop-owned output).
- **stderr**: status messages (`clone: <url> → <path>`, `skip: <reason>`), error messages, hints. The `hop config init` post-write tip also goes to stderr.
- The `hop <name> cd`, bare `hop <name>`, and tool-form (`hop <name> <tool>`) binary-form exit-2 hints go to **stderr**.
- `hop -R` inherits stdin/stdout/stderr from the parent — the child's output passes through unchanged.

### Behavioral Scenarios (GIVEN/WHEN/THEN)

#### Bare picker

> **GIVEN** `hop.yaml` lists 3 repos
> **WHEN** I run `hop` with no arguments
> **THEN** fzf opens with all 3 repos visible
> **AND** selecting one prints its absolute path to stdout
> **AND** exit code is 0

#### Bare-name 1-arg form (binary)

> **GIVEN** the user invokes the binary directly (no shim)
> **WHEN** they run `hop outbox`
> **THEN** the binary writes the bare-name hint (`hop: bare-name dispatch is shell-only. Add 'eval "$(hop shell-init zsh)"' to your zshrc, or use: hop "<name>" where`) to stderr
> **AND** stdout is empty
> **AND** exit code is 2

#### Unique substring match (`hop <name> where`)

> **GIVEN** `hop.yaml` has exactly one repo named `outbox`
> **WHEN** I run `hop outbox where`
> **THEN** fzf is NOT invoked
> **AND** stdout is the absolute path to that repo
> **AND** exit code is 0

#### Ambiguous substring match (`hop <name> where`)

> **GIVEN** `hop.yaml` has repos `outbox` and `outbox-shared`
> **WHEN** I run `hop outbox where`
> **THEN** fzf opens with both candidates filtered (`--query outbox`)
> **AND** if the user picks one, exit code 0
> **AND** if the user cancels (Esc), exit code 130

#### Zero substring match (`hop <name> where`)

> **GIVEN** `hop.yaml` has repos `alpha`, `beta`, `gamma`
> **WHEN** I run `hop zzz where`
> **THEN** fzf opens with `--query zzz` and zero filtered candidates
> **AND** the user can clear the query inside fzf to see all repos and pick one
> **AND** if the user cancels, exit code 130

#### Group disambiguation in picker

> **GIVEN** `hop.yaml` has a repo named `tools` in group `default` and another named `tools` in group `vendor`
> **WHEN** I run `hop` (bare)
> **THEN** fzf shows two rows: `tools [default]` and `tools [vendor]`
> **AND** the path column (the unique key for match-back) distinguishes them

#### `hop <name> cd` binary form

> **GIVEN** the user has NOT run `eval "$(hop shell-init zsh)"`
> **WHEN** they run `hop <name> cd`
> **THEN** the binary prints to stderr: `hop: 'cd' is shell-only. Add 'eval "$(hop shell-init zsh)"' to your zshrc, or use: cd "$(hop "<name>" where)"`
> **AND** exit code is 2

#### `hop <name> cd` shell-function form

> **GIVEN** the user has run `eval "$(hop shell-init zsh)"`
> **WHEN** they run `hop <name> cd`
> **THEN** the shim's `$2 == "cd"` branch in the repo-name dispatch fires
> **AND** the shim runs `_hop_dispatch cd "<name>"`
> **AND** `_hop_dispatch cd` runs `command hop "<name>" where` to resolve, then `cd -- <resolved-path>`
> **AND** the parent shell's working directory is changed

#### `hop <name> <tool>` binary form (tool-form attempt)

> **GIVEN** the user invokes the binary directly (no shim)
> **WHEN** they run `hop outbox cursor`
> **THEN** the binary prints to stderr: `hop: 'cursor' is not a hop verb (cd, where). For tool-form, install the shim: eval "$(hop shell-init zsh)", or use: hop -R "<name>" <tool> [args...]`
> **AND** exit code is 2

#### Bare-name dispatch (shell shim)

> **GIVEN** the user has run `eval "$(hop shell-init zsh)"` and `hop.yaml` has a repo named `outbox`
> **WHEN** they run `hop outbox` (1 arg, no recognized subcommand)
> **THEN** the shim's repo-name branch matches `$# == 1` and routes through `_hop_dispatch cd "outbox"`
> **AND** the helper runs `command hop "outbox" where` to resolve, then `cd -- <resolved-path>`
> **AND** the parent shell's cwd is changed (no need to type `hop <name> cd`)

The 2-arg explicit form `hop outbox cd` routes the same way (the shim's `$2 == "cd"` branch ALSO calls `_hop_dispatch cd "outbox"`). Both 1-arg and 2-arg forms share the helper.

`h <name>` (single-letter alias) behaves identically; `hi <name>` bypasses the shim and invokes the binary directly (so `hi outbox` errors with the bare-name hint, which is the binary's "I can't change cwd" signal).

#### `hop <name> -R <cmd>...` exec-in-context

The user-facing canonical form is `hop <name> -R <cmd>...` (repo name first). The shim rewrites this to `command hop -R <name> <cmd>...` before the binary sees it, so the binary's `extractDashR` continues to scan argv for `-R` followed by `<name>` followed by `<cmd>...` (the existing internal shape — see Design Decision below).

> **GIVEN** `hop.yaml` resolves `outbox` to `~/code/sahil87/outbox`
> **WHEN** I run `hop outbox -R git status` (under the shim)
> **THEN** the shim runs `command hop -R outbox git status`
> **AND** `git status` runs with `cwd = ~/code/sahil87/outbox`
> **AND** stdin/stdout/stderr are inherited (interactive prompts work)
> **AND** the parent shell's cwd is unchanged
> **AND** exit code matches `git status`'s exit code

> **GIVEN** an arbitrary child command with its own flags
> **WHEN** I run `hop outbox -R jq '.foo' file.json`
> **THEN** `<cmd>...` argv is forwarded verbatim — cobra does NOT try to parse `jq`'s flags as `hop` flags
> **AND** the child receives `jq '.foo' file.json` as its argv

> **GIVEN** `<name>` matches no repo
> **WHEN** I run `hop nope -R echo hi`
> **THEN** stderr shows the standard match-or-fzf no-candidate behavior
> **AND** exit code is 1 (resolution failed)

> **GIVEN** `<cmd>` is not on PATH
> **WHEN** I run `hop outbox -R notarealbinary`
> **THEN** stderr shows `hop: -R: 'notarealbinary' not found.`
> **AND** exit code is 1

> **GIVEN** the user invokes the binary directly without the shim
> **WHEN** they run `/usr/local/bin/hop -R outbox git status` (binary-internal form)
> **THEN** `extractDashR` parses correctly (its existing logic — unchanged)
> **AND** the binary execs `git status` in outbox

#### `hop <name> <tool>` shim sugar

The shim emitted by `hop shell-init` recognizes a tool-form: when `$1` is non-empty (and is not a known subcommand, flag, or `__complete*`), `$2` is non-empty and not `-R`, and `$# >= 2`, it rewrites the call to `command hop -R "$1" "$2" "${@:3}"`. The binary itself does NOT interpret this form — invoking the binary directly with `hop dotfiles cursor` argv just hits cobra's "accepts at most 1 arg" error.

The grammar is **subcommand xor repo**: the first positional is one or the other — never a tool. The shim does NOT inspect PATH for `$1` or `$2`, and there is no builtin/keyword filtering. Missing tools surface via the binary's `hop: -R: '<cmd>' not found.` error.

Resolution order in the shim's `hop()` function (5-step ladder, first match wins):

1. No args → bare picker (`command hop`).
2. `$1` is `__complete*` → `command hop "$@"` (cobra's hidden completion entrypoint).
3. `$1` is a known subcommand (`clone`, `ls`, `shell-init`, `config`, `update`, `help`, `--help`, `-h`, `--version`, `completion` — `cd` and `where` are NOT in this list; they're $2 verbs now) → `_hop_dispatch "$@"`.
4. `$1` is a flag (`-R`, `-h`, `-v`, ...) → `command hop "$@"`.
5. Otherwise (`$1` is treated as a repo name) — dispatch on `$2`:
   - `$# == 1` → `_hop_dispatch cd "$1"` (bare-name → `cd`).
   - `$2 == "cd"` → `_hop_dispatch cd "$1"` (explicit `cd` verb).
   - `$2 == "where"` → `command hop "$1" where` (binary handles via the `where`-verb dispatch).
   - `$2 == "-R"` → `command hop -R "$1" "${@:3}"` (canonical exec form, shim-rewritten so the binary's `extractDashR` sees the existing `-R <name> <cmd>...` shape).
   - Otherwise → `command hop -R "$1" "$2" "${@:3}"` (tool-form sugar).

> **GIVEN** `cursor` is on PATH and `dotfiles` resolves uniquely
> **WHEN** I run `hop dotfiles cursor` under the shim
> **THEN** the shim runs `command hop -R dotfiles cursor`
> **AND** the binary execs `cursor` with `cwd = <dotfiles-path>`
> **AND** exit code matches `cursor`'s

> **GIVEN** the user has a repo named `cursor` AND `cursor` is also on PATH
> **WHEN** I run `hop cursor` (1 arg) under the shim
> **THEN** the shim treats it as bare-name `cd` (rule 5, `$# == 1`) — `cd` into the cursor repo
> **WHEN** I run `hop dotfiles cursor` (2 args)
> **THEN** the shim treats it as tool-form (rule 5, otherwise) — runs `cursor` in dotfiles

> **GIVEN** `ls` is both a known subcommand AND a binary on PATH
> **WHEN** I run `hop ls outbox` under the shim
> **THEN** the shim dispatches to the `hop ls` subcommand (rule 3 wins over rule 5) — cobra rejects the extra `outbox` arg

> **GIVEN** `outbox` resolves uniquely
> **WHEN** I run `hop outbox pwd` under the shim
> **THEN** the shim runs `command hop -R outbox pwd`
> **AND** the binary execs `/bin/pwd` (the on-PATH binary, not the shell builtin) with `cwd = <outbox-path>`
> **AND** stdout is the absolute path of outbox
> **AND** there is NO cheerful-error escape hatch — the grammar accepts this redundancy intentionally (Design Decision below)

> **GIVEN** `outbox` resolves uniquely AND `notarealbinary` is not on PATH
> **WHEN** I run `hop outbox notarealbinary` under the shim
> **THEN** the shim runs `command hop -R outbox notarealbinary`
> **AND** the binary emits `hop: -R: 'notarealbinary' not found.` to stderr
> **AND** exit code is 1
> **NOTE**: The shim does not pre-check PATH; missing tools surface via the binary's `-R` error path.

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

#### `hop config scan <dir>` — populate `hop.yaml` from on-disk repos

`hop config scan` walks `<dir>` (default `--depth 3`, inclusive), discovers git repositories via stat + `git remote`, derives groups from the on-disk layout (convention-match → `default`; non-convention → invented map-shaped group keyed off the parent dir basename), and emits a merged `hop.yaml` to stdout (default) or merges in place via `--write` (atomic, comment-preserving). All `git` invocations route through `internal/proc.RunCapture` with a 5-second per-call `context.WithTimeout`. Walk symlinks are followed with `(dev, inode)` loop dedup. Implementation: `src/cmd/hop/config.go::newConfigScanCmd` + helpers in `src/cmd/hop/config_scan.go`; the walker lives in `src/internal/scan/` and the YAML merge in `src/internal/yamled/MergeScan` + `RenderScan`.

> **GIVEN** `hop.yaml` has `code_root: ~/code` and `~/code/sahil87/hop/.git` exists with `git remote get-url origin` returning `git@github.com:sahil87/hop.git`
> **WHEN** I run `hop config scan ~/code`
> **THEN** the URL lands in the `default` flat group in the rendered YAML
> **AND** stderr summarizes `matched convention (default): 1`
> **AND** exit code is 0

> **GIVEN** the same `hop.yaml` and a non-convention repo at `~/vendor/forks/tool/.git` with URL `git@github.com:other/tool.git`
> **WHEN** I run `hop config scan ~/vendor`
> **THEN** the rendered YAML contains an invented `forks:` group with `dir: ~/vendor/forks` and the URL under `urls:`
> **AND** stderr summarizes `invented groups: 1 (forks)`

> **GIVEN** `~/work` is a symlink to `~/Volumes/Mac/work` (a real directory containing repos)
> **WHEN** I run `hop config scan ~/work`
> **THEN** `EvalSymlinks` resolves the argument and the walk proceeds against the canonical target
> **AND** each `Found.Path` is the canonical (resolved) path

> **GIVEN** `~/code/a/b/c/d/.git` exists at depth 4 from `~/code`
> **WHEN** I run `hop config scan ~/code --depth 3`
> **THEN** that repo is NOT in the rendered YAML (depth bound is inclusive at 3)

> **GIVEN** `~/code/scratch/.git` exists and `git remote` returns empty
> **WHEN** I run `hop config scan ~/code`
> **THEN** the repo is skipped with reason `no remote`
> **AND** stderr's skipped breakdown counts it
> **AND** the URL is NOT rendered into the YAML

> **GIVEN** no `hop.yaml` exists at the resolved path (and `$HOP_CONFIG` is unset)
> **WHEN** I run `hop config scan ~/code`
> **THEN** stderr shows `hop config scan: no hop.yaml found at <ResolveWriteTarget>.` followed by `Run 'hop config init' first, then re-run scan.`
> **AND** exit code is 1
> **AND** no walk is performed (no `git` invocations)

### External Tool Availability

External tools (`fzf`, `git`, `<cmd>` for `-R`) are checked **lazily** — only when the subcommand actually needs them. Subcommands that resolve without an external tool MUST NOT preemptively check or fail.

| Tool | Required by | Behavior if missing |
|---|---|---|
| `fzf` | `hop` (bare picker), `hop <name> where` (ambiguous), `hop -R` (ambiguous), `hop clone <name>` (ambiguous) | Print to stderr: `hop: fzf is not installed. Install it: brew install fzf (macOS) or apt install fzf (Debian).` Exit 1. |
| `git` | `hop clone` (any form); `hop config scan <dir>` (only when the walk finds a `.git` candidate — empty trees succeed without `git`) | Print to stderr: `hop: git is not installed.` Exit 1. |
| `<cmd>` | `hop <name> -R <cmd>...` (and the shim's `hop <name> <tool>` sugar that rewrites to it) | Print to stderr: `hop: -R: '<cmd>' not found.` Exit 1. |
| `brew` | `hop update` (when installed via brew) | Print to stderr: `hop update: brew not found on PATH.` Exit 1. |

Subcommands that don't need a tool MUST work without it. Examples:
- `hop foo where` (when `foo` is a unique substring match) does not invoke fzf — works without `fzf` installed.
- `hop ls` does not invoke any external tool.
- `hop shell-init zsh` and `hop shell-init bash` do not invoke any external tool — emit stdout text only.
- `hop config init` and `hop config where` do not invoke any external tool.

### Help Text

`hop -h | --help | help` emits help text to stdout. Cobra renders the help; the `Usage:` table and `Notes:` block come from `rootLong` in `src/cmd/hop/root.go`. Top-level structure mirrors the inventory table above.

The `Usage:` block enumerates (in this order): `hop`, `hop <name>`, `hop <name> cd`, `hop <name> where`, `hop <name> -R <cmd>...`, `hop <name> <tool>...`, `hop clone <name>`, `hop clone <url>`, `hop clone --all`, `hop clone`, `hop ls`, `hop shell-init <shell>`, `hop config init`, `hop config where`, `hop config print`, `hop config scan <dir>`, `hop update`, `hop -h | --help`, `hop -v | --version`.

The `Notes:` block in `rootLong` documents:
- `hop <name>` and `hop <name> cd` require shell integration (a binary can't change its parent shell's cwd). Without it, use `cd "$(hop <name> where)"`.
- On ambiguous or no-match queries, fzf opens prefilled with the user's query.
- Grammar: first positional is a repo OR a subcommand (mutually exclusive). When it's a repo, second positional is a verb (`cd`, `where`), `-R`, or a tool name.
- Config search order: `$HOP_CONFIG`, then `$XDG_CONFIG_HOME/hop/hop.yaml`, then `$HOME/.config/hop/hop.yaml`.

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
- The repo-verb behavior (`hop` with no subcommand, `hop <name>` 1-arg, `hop <name> <verb>` 2-arg) is implemented via `rootCmd.RunE` (with `cobra.MaximumNArgs(2)`) switching on `len(args)` and `args[1]`. The `where`-verb branch dispatches to `resolveAndPrint`; the bare-name, `cd`-verb, and tool-form branches each return `&errExitCode{code: 2, msg: ...}` with the appropriate hint.

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

1. **The `cd` verb at $2 is shell-only; the binary errors with a hint.** A binary cannot change its parent shell's `cwd`; the function wrapper (emitted by `hop shell-init zsh`) does. The binary's role is to print a hint pointing at the shim install and `hop <name> where`, so users discover the shell integration. Generalizes to: every form that needs the shim errors in the binary; every form the binary can fulfill works in both layers.
2. **Bare-name dispatch (`hop <name>` 1 arg) is shorthand for `hop <name> cd` (Option B2).** Both are shell-only — the binary errors with a hint. This enforces the invariant that any `hop <subform>` either errors in the binary or works in both layers — never two different effects sharing one syntax. The pre-flip behavior (binary printed the path; shim cd'd) was the asymmetry this change eliminates.
3. **`fzf` is invoked lazily, not preflighted.** Subcommands that don't need fzf (`hop ls`, `hop shell-init zsh`, `hop config *`, exact-match resolutions) work without it installed. This matters for minimal environments and CI.
4. **`-R` bypasses cobra rather than using `cobra.Command{DisableFlagParsing: true}`.** Pre-Execute argv inspection is a single small function (`extractDashR`); the alternative would entangle every flag-parsing path with `-R`-aware logic. Unit tests cover the split logic without spawning the binary. Spelled `-R` (not `-C`) because hop primarily operates on **repos** (not arbitrary directories like `git -C` / `make -C` / `tar -C`); `-R` reads as "repo" at the call site.
5. **Match algorithm is substring-on-`Name` only.** Not Path, not URL, not Group. Simple, predictable, matches the bash original. Group disambiguation is a display-time concern only (`buildPickerLines` adds `[<group>]` suffix when names collide across groups).
6. **The `where` verb is the explicit path-printer.** Used as `hop <name> where` (top-level repo-verb form) and `hop config where` (config namespace). The top-level `where` *subcommand* (v0.x's `hop where <name>`) was removed — `hop <name> where` is the replacement; the verb survives, the subcommand position does not. Both answer "where does this resolve to?" The v0.0.1 names (`path`, `config path`) lacked voice-fit with the new binary name and were renamed without aliases (no migration path; the rename was a clean break for v0.x).
7. **`hop clone <url>` infers form from argument shape.** `looksLikeURL` (contains `://` OR (`@` AND `:`)) splits URL form from name form. This keeps `clone` to one verb rather than `clone-url` / `clone-name`. URLs of registered repos still go through name form via `hop clone <name>` — there's no ambiguity because the URL form requires an actual URL shape.
8. **Auto-registration on `hop clone <url>` is opt-out, not opt-in.** The default behavior for an ad-hoc URL clone is "I want this in my registry"; `--no-add` is the escape valve. This matches the dominant use case (try a new repo → keep it). The YAML write is comment-preserving (via `internal/yamled`) so registration doesn't trash hand-curated comments.
9. **`hop update` is a top-level subcommand, not `hop config update` or a flag.** Per Constitution Principle VI, new top-level subcommands need explicit justification. Self-update is a binary-state operation, not config-state — it doesn't fit under `config`, and overloading a flag on the root (e.g. `hop --update`) muddles the bare-form's "print path" semantics. It also matches the convention every Homebrew-installed CLI uses (`fab-kit update`, `gh extension upgrade`). The implementation lives in `internal/update` and routes all subprocess invocations through `internal/proc` per Constitution Principle I (no direct `os/exec` outside `internal/proc`).
10. **Grammar is `subcommand` xor `repo` at $1. The first positional is one or the other — never a tool, never a verb.** When `$1` is a repo, `$2` is a verb (`cd`, `where`), `-R`, or a tool name. The verbs `cd` and `where` are NOT subcommands at `$1` — they exist only at `$2` in the repo-first form. This collapses the shim's precedence ladder (no PATH inspection of `$1`, no builtin filtering, no cheerful-error escape hatches) and makes tab completion work in the repo slot for free (`completeRepoNames` already runs on `$1`). The user-facing canonical exec form is `hop <name> -R <cmd>...` and tool-form sugar is `hop <name> <tool> [args...]` — both shim-only; the binary itself does NOT interpret tool-form. The shim flips, but the binary's `extractDashR` keeps its existing internal shape (`-R <name> <cmd>...`) because the shim rewrites the user-facing form before the binary sees it. The trade-off: scripts and CI invoking the binary directly use `hop -R <name> <cmd>...` explicitly (and `hop <name> where` directly, since the binary's `where`-verb dispatch handles that); the user-facing repo-first form is the shell experience.
11. **`hop open` was removed entirely; no replacement subcommand.** Once tool-form covers the use case generically, the dedicated `open` subcommand is redundant special-casing. Users invoke `hop <name> open` (Darwin) or `hop <name> xdg-open` (Linux) via the shim's tool-form sugar — or `hop <name> -R open` / `hop <name> -R xdg-open` directly. The `internal/platform` package was deleted with the subcommand: its only purpose was to abstract Darwin-vs-Linux for `hop open`. Cross-platform users who need portable scripts write their own one-liner.
12. **`hop code` was removed in favor of `hop <name> code` via tool-form.** Once the shim dispatches `hop <name> <tool>`, a dedicated `code` subcommand is redundant — `hop dotfiles code` (shim) → `command hop -R dotfiles code` → execs `code` with cwd = dotfiles. This removes a top-level subcommand and the `code`-specific install-hint code path. There is no compatibility shim — this is a clean break, consistent with the v0.x policy of renaming/removing subcommands without aliases.
13. **Tool-form is shim-only; the binary errors on `hop <name> <tool>` (Option X).** The binary could absorb tool-form (`hop -R <name> <tool>` is its internal shape), but doing so blurs the binary's role as a path-printer + error-emitter. Keeping tool-form shim-only preserves the binary's narrow contract and matches the existing posture for `cd` and bare-name. The binary's $2 dispatch is therefore three-way: `where` works (path printer), `cd` errors (shell-only hint), anything-else errors (tool-form-not-a-hop-verb hint). The error wording for tool-form (`hop: '<tool>' is not a hop verb (cd, where). For tool-form, install the shim: ..., or use: hop -R "<name>" <tool> [args...]`) tells the user both how to install the shim and how to invoke the binary directly, so the dispatcher's error path is itself a discovery surface.
