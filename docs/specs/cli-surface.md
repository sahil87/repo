# CLI Surface

> Canonical contract for what the `hop` binary exposes to users.
> Source of truth for argument parsing, exit codes, stdout/stderr conventions, and help text.

## Subcommand Inventory

| Subcommand | Args | Behavior summary | Exit codes |
|---|---|---|---|
| `hop` | (none) | fzf picker over all repos; print selected absolute path on stdout | 0 selected, 130 cancelled |
| `hop <name>` | `<name>` | Match-or-fzf to a single repo; print absolute path on stdout | 0 selected, 1 no match, 130 cancelled |
| `hop where <name>` | `<name>` | Identical to `hop <name>` (explicit form). Renamed from v0.0.1's `hop path` for voice-fit with the binary name. | same as above |
| `hop -C <name> <cmd>...` | global flag + child argv | Resolve `<name>`, then exec `<cmd>...` with `cwd = <resolved-path>` and inherited stdio. Bypasses cobra parsing for `<cmd>...` | child's exit code; 1 if resolution fails; 2 on usage error |
| `hop code [<name>]` | optional `<name>` | Resolve via match-or-fzf; `code <path>` | 0 launched, 1 resolution failed |
| `hop open [<name>]` | optional `<name>` | Resolve; `open <path>` (Darwin) or `xdg-open <path>` (Linux) | 0 opened, 1 resolution failed, 2 unsupported OS |
| `hop cd <name>` | `<name>` | Binary form: print hint to stderr, exit 2. Shell-function form (after `eval`): cd into the resolved path. | Binary: 2. Shell function: 0 success, 1 no match |
| `hop clone [<name>] \| --all` | optional `<name>` or `--all` | Clone single (resolved) or all missing repos | 0 success, 1 path conflict, non-zero on git failure |
| `hop clone <url>` | 1 (URL form, detected by `looksLikeURL`) | Ad-hoc clone with auto-registration. Flags: `--group`, `--no-add`, `--no-cd`, `--name`. | 0 success, 1 missing group / path conflict / git failure |
| `hop ls` | (none) | Print all repos as `name<spaces>path` columns | 0 |
| `hop shell-init zsh` | `zsh` (required) | Emit zsh function wrapper + cobra-generated completion to stdout | 0 success, 2 unsupported shell |
| `hop config init` | (none) | Bootstrap a starter `hop.yaml` at the resolved location | 0 written, 1 file exists, 2 write error |
| `hop config where` | (none) | Print the resolved config path on stdout. Renamed from v0.0.1's `config path`. | 0 resolved, 1 unresolvable |
| `hop -h \| --help \| help` | (none) | Print help text on stdout | 0 |
| `hop -v \| --version` | (none) | Print version string on stdout | 0 |

> `hop path` (v0.0.1) and `hop config path` (v0.0.1) have been removed without aliases. Use `hop where` and `hop config where`.

### Match Resolution Algorithm

Used by `hop`, `hop <name>`, `hop where`, `hop -C`, `hop code`, `hop open`, `hop cd`, `hop clone`.

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

- **stdout**: resolved absolute paths (`hop`, `hop where`), the `hop ls` table, version string, config path (`hop config where`), shell integration (`hop shell-init zsh`), help text, "Created <path>" message from `hop config init`, the landed path from `hop clone <url>` (used by the shell shim for cd-on-success).
- **stderr**: status messages (`clone: <url> → <path>`, `skip: <reason>`), error messages, hints. The `hop config init` post-write tip also goes to stderr.
- The `hop cd` binary form's exit-2 hint goes to **stderr**.
- `hop -C` inherits stdin/stdout/stderr from the parent — the child's output passes through unchanged.

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

#### `hop -C` exec-in-context

> **GIVEN** `hop.yaml` resolves `outbox` to `~/code/sahil87/outbox`
> **WHEN** I run `hop -C outbox git status`
> **THEN** `git status` runs with `cwd = ~/code/sahil87/outbox`
> **AND** stdin/stdout/stderr are inherited (interactive prompts work)
> **AND** the parent shell's cwd is unchanged
> **AND** exit code matches `git status`'s exit code

> **GIVEN** an arbitrary child command with its own flags
> **WHEN** I run `hop -C outbox jq '.foo' file.json`
> **THEN** `<cmd>...` argv is forwarded verbatim — cobra does NOT try to parse `jq`'s flags as `hop` flags
> **AND** the child receives `jq '.foo' file.json` as its argv

> **GIVEN** `<name>` matches no repo
> **WHEN** I run `hop -C nope echo hi`
> **THEN** stderr shows the standard match-or-fzf no-candidate behavior
> **AND** exit code is 1 (resolution failed)

> **GIVEN** `<cmd>` is not on PATH
> **WHEN** I run `hop -C outbox notarealbinary`
> **THEN** stderr shows `hop: 'notarealbinary' not found`
> **AND** exit code is 1

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

#### `hop shell-init zsh`

> **WHEN** I run `hop shell-init zsh`
> **THEN** stdout contains the static `zshInit` prefix defining `hop()`, `_hop_dispatch()`, `h()`, `hi()`
> **AND** stdout contains the cobra-generated `_hop` completion function (appended at runtime via `rootCmd.GenZshCompletion`)
> **AND** running `eval "$(hop shell-init zsh)"` in a zsh shell defines `hop` as a function (verifiable via `whence -w hop`)
> **AND** exit code is 0

> **WHEN** I run `hop shell-init` with no shell argument
> **THEN** stderr shows `hop shell-init: missing shell. Supported: zsh`
> **AND** exit code is 2

> **WHEN** I run `hop shell-init bash`
> **THEN** stderr shows `hop shell-init: unsupported shell 'bash'. Supported: zsh`
> **AND** exit code is 2

#### `hop --version` / `-v`

> **WHEN** I run `hop --version` or `hop -v`
> **THEN** stdout is a single line containing the version string (e.g., `v0.1.0` or `v0.1.0-2-gabc123` for dev builds from `git describe`)
> **AND** exit code is 0

> **NOTE**: Cobra also auto-wires a `hop version` subcommand from `rootCmd.Version`; this still works (no effort spent suppressing it).

### External Tool Availability

External tools (`fzf`, `git`, `code`, `open`, `xdg-open`) are checked **lazily** — only when the subcommand actually needs them. Subcommands that resolve without an external tool MUST NOT preemptively check or fail.

| Tool | Required by | Behavior if missing |
|---|---|---|
| `fzf` | `hop`, `hop <name>` (when match is ambiguous), `hop where` (ambiguous), `hop -C` (ambiguous), `hop code` (ambiguous), `hop open` (ambiguous), `hop clone <name>` (ambiguous) | Print to stderr: `hop: fzf is not installed. Install it: brew install fzf (macOS) or apt install fzf (Debian).` Exit 1. |
| `git` | `hop clone` (any form) | Print to stderr: `hop: git is not installed.` Exit 1. |
| `code` | `hop code` | Print to stderr: `hop code: 'code' command not found. Install VSCode and ensure 'code' is on your PATH.` Exit 1. |
| `open` (Darwin) / `xdg-open` (Linux) | `hop open` | Print to stderr: `hop open: '<tool>' not found.` Exit 1. |
| `<cmd>` | `hop -C <name> <cmd>...` | Print to stderr: `hop: '<cmd>' not found`. Exit 1. |

Subcommands that don't need a tool MUST work without it. Examples:
- `hop where foo` (when `foo` is a unique substring match) does not invoke fzf — works without `fzf` installed.
- `hop ls` does not invoke any external tool.
- `hop shell-init zsh` does not invoke any external tool — emits stdout text only.
- `hop config init` and `hop config where` do not invoke any external tool.

### Help Text

`hop -h | --help | help` emits help text to stdout. Cobra renders the help; the `Usage:` table and `Notes:` block come from `rootLong` in `src/cmd/hop/root.go`. Top-level structure mirrors the inventory table above.

The `Notes:` block in `rootLong` documents:
- `hop cd` requires the shell integration; without it, use `cd "$(hop where <name>)"` or `cd "$(hop <name>)"`.
- Bare-name dispatch (the shim's `hop <name>` shortcut for `hop cd`).
- Config search order: `$HOP_CONFIG`, then `$XDG_CONFIG_HOME/hop/hop.yaml`, then `$HOME/.config/hop/hop.yaml`.
- Run `hop config init` to bootstrap.

### Cobra Wiring

- `rootCmd` is defined in `src/cmd/hop/root.go::newRootCmd()`.
- Each subcommand has its own file under `src/cmd/hop/` with a `func newXxxCmd() *cobra.Command` factory.
- `main.go::main()`:
  1. Builds `rootCmd := newRootCmd()`.
  2. Sets `rootCmd.Version = version` (the package-level `var version = "dev"`, overridden via `-ldflags "-X main.version=…"` at build time).
  3. Captures `rootForCompletion = rootCmd` so `shell-init zsh` can call `GenZshCompletion` without threading `rootCmd` through factories.
  4. Inspects `os.Args` for `-C` via `extractDashC` (pre-cobra). If present, resolves the target via `resolveByName`, then calls `proc.RunForeground` and `os.Exit(code)` — bypassing cobra entirely.
  5. Otherwise calls `rootCmd.Execute()`. Errors are mapped to exit codes via `translateExit`.
- `rootCmd.SilenceUsage = true` and `rootCmd.SilenceErrors = true` — `translateExit` is the sole stderr/exit path.
- The bare-form behavior (`hop` with no subcommand, `hop <name>` with one positional arg) is implemented via `rootCmd.RunE` checking args and dispatching to the same `resolveAndPrint` helper used by `hop where`.

#### Why `-C` bypasses cobra

Cobra's parser would try to interpret `<cmd>...` after `-C <name>` as `hop`'s own subcommand or its flags, breaking arbitrary children like `hop -C name git status` or `hop -C name jq '.foo' file.json`. Pre-Execute argv inspection (`extractDashC`) splits argv into the hop portion (`-C <name>`) and the child portion (everything else) so the child runs with its own argv intact. Unit-tested in `dashc_test.go`.

### Exit Code Conventions

Defined centrally in `main.go::translateExit`:

| Code | Meaning |
|---|---|
| 0 | Success |
| 1 | Application error (no match, missing tool, file already exists, write error, child resolution error, etc.); also `errSilent` (caller already wrote stderr) |
| 2 | Usage error (`cd` binary form, `shell-init` missing/unsupported shell, `-C` usage error) |
| 130 | User cancelled — fzf Esc / Ctrl-C (`errFzfCancelled`) |

The `-C` flag bypasses cobra entirely and uses `os.Exit` directly with the child's exit code (or 2 for usage errors, 1 for resolution errors).

### Design Decisions

1. **`hop cd` is intentionally split between binary and shell function.** A binary cannot change its parent shell's `cwd`; the function wrapper (emitted by `hop shell-init zsh`) does. The binary's role is to print a hint when invoked directly, so users discover the shell integration.
2. **Bare-name dispatch lives only in the shim, not the binary.** `hop outbox` from the binary still prints the path (so `cd "$(hop outbox)"` and shell-pipelines work). The shim's bare-name dispatch is a UX layer added on top — invoking the binary directly remains a pure path printer.
3. **`fzf` is invoked lazily, not preflighted.** Subcommands that don't need fzf (`hop ls`, `hop shell-init zsh`, `hop config *`, exact-match resolutions) work without it installed. This matters for minimal environments and CI.
4. **`-C` bypasses cobra rather than using `cobra.Command{DisableFlagParsing: true}`.** Pre-Execute argv inspection is a single small function (`extractDashC`); the alternative would entangle every flag-parsing path with `-C`-aware logic. Unit tests cover the split logic without spawning the binary.
5. **Match algorithm is substring-on-`Name` only.** Not Path, not URL, not Group. Simple, predictable, matches the bash original. Group disambiguation is a display-time concern only (`buildPickerLines` adds `[<group>]` suffix when names collide across groups).
6. **`hop where` and `hop config where` use the same verb for symmetry.** Both answer "where would this go / where does this resolve to?" The v0.0.1 names (`path`, `config path`) lacked voice-fit with the new binary name and were renamed without aliases (no migration path; the rename was a clean break for v0.x).
7. **`hop clone <url>` infers form from argument shape.** `looksLikeURL` (contains `://` OR (`@` AND `:`)) splits URL form from name form. This keeps `clone` to one verb rather than `clone-url` / `clone-name`. URLs of registered repos still go through name form via `hop clone <name>` — there's no ambiguity because the URL form requires an actual URL shape.
8. **Auto-registration on `hop clone <url>` is opt-out, not opt-in.** The default behavior for an ad-hoc URL clone is "I want this in my registry"; `--no-add` is the escape valve. This matches the dominant use case (try a new repo → keep it). The YAML write is comment-preserving (via `internal/yamled`) so registration doesn't trash hand-curated comments.
