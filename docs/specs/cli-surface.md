# CLI Surface

> Canonical contract for what the `repo` binary exposes to users.
> Source of truth for argument parsing, exit codes, stdout/stderr conventions, and help text.

## Subcommand Inventory (v0.0.1)

11 subcommands. The first 9 mirror the existing bash script (`~/code/bootstrap/dotfiles/bin/repo`) for feature parity. The last 2 (`config init`, `config path`) are onboarding affordances.

| Subcommand | Args | Behavior summary | Exit codes |
|---|---|---|---|
| `repo` | (none) | fzf picker over all repos; print selected absolute path on stdout | 0 selected, 130 cancelled |
| `repo <name>` | `<name>` | Match-or-fzf to a single repo; print absolute path on stdout | 0 selected, 1 no match, 130 cancelled |
| `repo path <name>` | `<name>` | Identical to `repo <name>` (explicit form) | same as above |
| `repo code [<name>]` | optional `<name>` | Resolve via match-or-fzf; `code <path>` | 0 launched, 1 resolution failed |
| `repo open [<name>]` | optional `<name>` | Resolve; `open <path>` (Darwin) or `xdg-open <path>` (Linux) | 0 opened, 1 resolution failed, 2 unsupported OS |
| `repo cd <name>` | `<name>` | Binary form: print hint to stderr, exit 2. Shell-function form (after `eval`): cd into the resolved path. | Binary: 2. Shell function: 0 success, 1 no match |
| `repo clone [<name>] \| --all` | optional `<name>` or `--all` | Clone single (resolved) or all missing repos | 0 success, 1 path conflict, non-zero on git failure |
| `repo ls` | (none) | Print all repos as `name<TAB>path` columns | 0 |
| `repo shell-init zsh` | `zsh` (required) | Emit zsh function wrapper + completion to stdout | 0 success, 2 unsupported shell |
| `repo config init` | (none) | Bootstrap a starter `repos.yaml` at the resolved location | 0 written, 1 file exists, 2 write error |
| `repo config path` | (none) | Print the resolved config path on stdout | 0 resolved, 1 unresolvable |
| `repo -h \| --help \| help` | (none) | Print help text on stdout | 0 |
| `repo -v \| --version` | (none) | Print version string on stdout | 0 |

### Match Resolution Algorithm

Used by `repo`, `repo <name>`, `repo path`, `repo code`, `repo open`, `repo cd`, `repo clone`.

1. Build the list of all known repos from `repos.yaml`. Each entry has `(name, path, url)`.
2. If `<name>` is non-empty: filter by case-insensitive substring match on `name`.
3. If exactly **1 match**: return it directly without invoking fzf.
4. Otherwise (0 matches OR 2+ matches): invoke fzf with these flags:
   ```
   fzf --query <name> --select-1 --height 40% --reverse --with-nth 1 --delimiter '\t'
   ```
   The `--select-1` flag makes fzf auto-select if its filter narrows to exactly 1.
5. If `<name>` is empty: invoke fzf with no `--query` filter — full picker.

### Stdout / stderr Conventions

- **stdout**: resolved absolute paths (`repo`, `repo path`), the `repo ls` table, version string, config path (`repo config path`), shell integration (`repo shell-init zsh`), help text, "Created <path>" message from `repo config init`.
- **stderr**: status messages (`clone: <url> → <path>`, `skip: <reason>`), error messages, hints.
- The `repo cd` binary form's exit-2 hint goes to **stderr**.

### Behavioral Scenarios (GIVEN/WHEN/THEN)

#### Bare picker

> **GIVEN** `repos.yaml` lists 3 repos
> **WHEN** I run `repo` with no arguments
> **THEN** fzf opens with all 3 repos visible
> **AND** selecting one prints its absolute path to stdout
> **AND** exit code is 0

#### Unique substring match

> **GIVEN** `repos.yaml` has exactly one repo named `repo`
> **WHEN** I run `repo repo`
> **THEN** fzf is NOT invoked
> **AND** stdout is the absolute path to that repo
> **AND** exit code is 0

#### Ambiguous substring match

> **GIVEN** `repos.yaml` has repos named `repo` and `repos-shared`
> **WHEN** I run `repo repo`
> **THEN** fzf opens with both candidates filtered (`--query repo`)
> **AND** if the user picks one, exit code 0
> **AND** if the user cancels (Esc), exit code 130

#### Zero substring match

> **GIVEN** `repos.yaml` has repos `alpha`, `beta`, `gamma`
> **WHEN** I run `repo zzz`
> **THEN** fzf opens with `--query zzz` and zero filtered candidates
> **AND** the user can clear the query to see all repos and pick one
> **AND** if the user cancels, exit code 130

#### `repo cd` binary form

> **GIVEN** the user has NOT run `eval "$(repo shell-init zsh)"`
> **WHEN** they run `repo cd <name>`
> **THEN** the binary prints to stderr: `repo: 'cd' is shell-only. Add 'eval "$(repo shell-init zsh)"' to your zshrc, or use: cd "$(repo path "<name>")"`
> **AND** exit code is 2

#### `repo cd` shell-function form

> **GIVEN** the user has run `eval "$(repo shell-init zsh)"`
> **WHEN** they run `repo cd <name>`
> **THEN** the shell function calls `command repo path <name>` to resolve
> **AND** runs `cd -- <resolved-path>`
> **AND** the parent shell's working directory is changed

#### `repo clone` single

> **GIVEN** `<name>` resolves to `(name=foo, path=~/code/foo, url=git@github.com:user/foo.git)` and `~/code/foo` does not exist
> **WHEN** I run `repo clone foo`
> **THEN** stderr shows `clone: git@github.com:user/foo.git → ~/code/foo`
> **AND** `git clone git@github.com:user/foo.git ~/code/foo` runs
> **AND** exit code matches git's exit code

> **GIVEN** the same resolution, but `~/code/foo/.git` already exists
> **WHEN** I run `repo clone foo`
> **THEN** stderr shows `skip: already cloned at ~/code/foo`
> **AND** exit code is 0

> **GIVEN** the same resolution, but `~/code/foo` exists and is NOT a git repo
> **WHEN** I run `repo clone foo`
> **THEN** stderr shows `repo clone: ~/code/foo exists but is not a git repo`
> **AND** exit code is 1

#### `repo clone --all`

> **GIVEN** `repos.yaml` has 5 repos, 2 already cloned
> **WHEN** I run `repo clone --all`
> **THEN** stderr/stdout shows `clone:` lines for the 3 missing and `skip:` lines for the 2 cloned
> **AND** the final line is `summary: cloned=3 skipped=2 failed=0`
> **AND** exit code is 0 if `failed == 0`, else non-zero

#### `repo ls`

> **GIVEN** `repos.yaml` has 3 repos
> **WHEN** I run `repo ls`
> **THEN** stdout shows 3 rows, each `name<spaces>path`, aligned (column-style)
> **AND** exit code is 0

#### `repo shell-init zsh`

> **WHEN** I run `repo shell-init zsh`
> **THEN** stdout contains a zsh function definition for `repo` that intercepts the `cd` subcommand
> **AND** stdout contains a `compdef _repo repo` line for completion
> **AND** running `eval "$(repo shell-init zsh)"` in a zsh shell defines `repo` as a function (verifiable via `whence -w repo`)
> **AND** exit code is 0

> **WHEN** I run `repo shell-init` with no shell argument
> **THEN** stderr shows `repo shell-init: missing shell. Supported: zsh`
> **AND** exit code is 2

> **WHEN** I run `repo shell-init bash`
> **THEN** stderr shows `repo shell-init: unsupported shell 'bash'. Supported: zsh`
> **AND** exit code is 2

#### `repo --version` / `-v`

> **WHEN** I run `repo --version` or `repo -v`
> **THEN** stdout is a single line containing the version string (e.g., `v0.0.1` or `v0.0.1-2-gabc123` for dev builds from `git describe`)
> **AND** exit code is 0

> **NOTE**: The cobra-default `repo version` subcommand MAY also work (no effort spent suppressing it). The flag forms are the documented interface.

### External Tool Availability

External tools (`fzf`, `git`, `code`, `open`, `xdg-open`) are checked **lazily** — only when the subcommand actually needs them. Subcommands that resolve without an external tool MUST NOT preemptively check or fail.

| Tool | Required by | Behavior if missing |
|---|---|---|
| `fzf` | `repo`, `repo <name>` (when match is ambiguous), `repo path` (ambiguous), `repo code` (ambiguous), `repo open` (ambiguous), `repo clone` (ambiguous) | Print to stderr: `repo: fzf is not installed. Install it: brew install fzf (macOS) or apt install fzf (Debian).` Exit 1. |
| `git` | `repo clone`, `repo clone --all` | Print to stderr: `repo: git is not installed.` Exit 1. |
| `code` | `repo code` | Print to stderr: `repo code: 'code' command not found. Install VSCode and ensure 'code' is on your PATH.` Exit 1. |
| `open` (Darwin) / `xdg-open` (Linux) | `repo open` | Print to stderr: `repo open: '<tool>' not found.` Exit 1. |

Subcommands that don't need a tool MUST work without it. Examples:
- `repo path foo` (when `foo` is a unique substring match) does not invoke fzf — works without `fzf` installed.
- `repo ls` does not invoke any external tool — works in minimal environments.
- `repo shell-init zsh` does not invoke any external tool — emits stdout text only.
- `repo config init` and `repo config path` do not invoke any external tool.

### Help Text

`repo -h | --help | help` emits help text to stdout. Structure (mirrors the bash script's `_usage` function with renames and additions):

```
repo — locate, open, or list repos from repos.yaml.

Usage:
  repo <name>            echo abs path of matching repo
  repo path <name>       same, explicit form
  repo code <name>       open VSCode at the repo
  repo open <name>       open the repo in the OS file manager (Finder on macOS)
  repo cd <name>         cd into the repo (shell function — needs `eval "$(repo shell-init zsh)"`)
  repo clone <name>      git clone the repo if it isn't already on disk
  repo clone --all       clone every repo from repos.yaml that isn't already on disk
  repo ls                list all repos
  repo shell-init zsh    emit zsh shell integration (use: eval "$(repo shell-init zsh)")
  repo config init       bootstrap a starter repos.yaml
  repo config path       print the resolved repos.yaml path
  repo                   fzf picker, print selection
  repo code              fzf picker, then open VSCode
  repo open              fzf picker, then open in OS file manager
  repo clone             fzf picker, then clone if missing
  repo -h | --help       show this help
  repo -v | --version    print version

Notes:
  - `repo cd` requires the shell integration (a binary can't change its parent shell's cwd).
    Without it, use:  cd "$(repo <name>)"
  - On ambiguous or no-match queries, fzf opens prefilled with your query.
  - Config: $REPOS_YAML, then $XDG_CONFIG_HOME/repo/repos.yaml.
    Run `repo config init` to bootstrap one.
```

### Cobra Wiring

- `rootCmd` is defined in `src/cmd/repo/root.go`.
- Each subcommand has its own file under `src/cmd/repo/` with a `func newXxxCmd() *cobra.Command` factory.
- `main.go` constructs the root, attaches subcommands, and calls `rootCmd.Execute()`.
- `rootCmd.Version` is set from a `var version string` populated at build time via `-ldflags "-X main.version=$(git describe --tags --always)"`. Default value (when `-X` is not passed): `dev`.
- `rootCmd.SilenceUsage = true` and `rootCmd.SilenceErrors = true` — error printing is handled explicitly so we control the format.
- The bare-form behavior (`repo` with no subcommand, `repo <name>` with one positional arg) is implemented via `rootCmd.RunE` checking args and dispatching to the same handler used by `repo path`.

### Exit Code Conventions

| Code | Meaning |
|---|---|
| 0 | Success |
| 1 | Application error (no match, missing tool, file already exists, write error, etc.) |
| 2 | Usage error (missing required arg, unsupported value, shell-only subcommand invoked from binary) |
| 130 | User cancelled (fzf Esc, SIGINT) |

Cobra's default for unknown commands and parse errors is exit 1, which is acceptable. Where the spec calls for exit 2 (usage error), the subcommand's `RunE` must explicitly `os.Exit(2)` after writing the error to stderr.

### Design Decisions

1. **`repo cd` is intentionally split between binary and shell function.** The binary cannot change its parent shell's `cwd`; the function wrapper (emitted by `repo shell-init zsh`) does. The binary's role is to print a hint when invoked directly, so users discover the shell integration.
2. **`fzf` is invoked lazily, not preflighted.** Subcommands that don't need fzf (`repo ls`, `repo shell-init zsh`, `repo config *`, exact-match resolutions) work without it installed. This matters for minimal environments and CI.
3. **`-v` / `--version` are required; `version` subcommand is tolerated.** Cobra auto-wires both when `rootCmd.Version` is set; we don't suppress the subcommand. The flags are the documented form.
4. **Match algorithm preserves bash behavior exactly.** Case-insensitive substring on the *name* column only (not path, not URL). Exactly-1-match short-circuits fzf. This is a behavioral parity requirement of the migration.
