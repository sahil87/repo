# Spec: Unify repo-verb grammar — `hop <repo> <verb>`, drop `cd`/`where` subcommands

**Change**: 260507-9lk0-tighten-bare-form-binary-error
**Created**: 2026-05-07
**Affected memory**:
- `docs/memory/cli/subcommands.md` (modify)
- `docs/memory/cli/match-resolution.md` (modify)
- `docs/memory/architecture/wrapper-boundaries.md` (verify only)

## Non-Goals

- **Tab completion at `$2`** — punted to a follow-up. `cd`/`where` verb completion plus PATH-completing tools is its own change.
- **Binary absorbing tool-form** — the binary errors on `hop <name> <tool>`. Tool-form remains shim-only (Option X, not Y). Justification in Design Decision #13.
- **`hop` (0 args, picker)** — unchanged in both binary and shim.
- **`hop clone`** (any form) — unchanged. `hop clone <url>` print contract (IPC with shim) preserved.
- **`hop -R <name> <cmd>...` and `extractDashR`** — unchanged. Tool-form sugar already routes through `-R`.
- **Memory file moves/restructuring** — content updates only; no new domains.
- **Backwards-compat shims, aliases, deprecation periods** — clean v0.x break (user explicitly waived compat).

## CLI: Grammar

### Requirement: Two-positional repo-verb grammar at root

The `hop` binary's root command SHALL accept up to two positional arguments. The first positional MUST be either a known subcommand (`clone`, `ls`, `shell-init`, `config`, `update`, `help`, `completion`) OR a repo name — never a tool. The second positional, when present, MUST be a verb (`cd`, `where`), the `-R` flag, or any other token (treated as a tool-form attempt). The grammar is **subcommand xor repo** at `$1`; verbs and tool tokens live only at `$2` in the repo-first form.

#### Scenario: Bare picker (0 args)

- **GIVEN** any valid `hop.yaml`
- **WHEN** the user runs `hop` with no arguments
- **THEN** fzf opens with all repos visible
- **AND** selecting one prints its absolute path to stdout
- **AND** exit code is 0

#### Scenario: Subcommand at `$1` wins over repo-name interpretation

- **GIVEN** `hop.yaml` happens to contain a repo named `ls`
- **WHEN** the user runs `hop ls`
- **THEN** the `ls` subcommand executes (prints all repos)
- **AND** the repo named `ls` is NOT resolved as a name

### Requirement: Top-level `cd` and `where` subcommands SHALL NOT exist

The `hop cd <name>` and `hop where <name>` subcommand factories MUST be removed from the cobra command tree. The `cd` and `where` verbs SHALL exist only at `$2` in the repo-first form (`hop <name> cd`, `hop <name> where`). The `where` verb under the `config` namespace (`hop config where`) is unaffected — it is a different namespace and does not collide.

#### Scenario: `hop cd <name>` rejected at the binary

- **GIVEN** the user invokes the binary directly
- **WHEN** they run `hop cd <name>`
- **THEN** cobra rejects the invocation with `Error: unknown command "cd" for "hop"`
- **AND** exit code is non-zero (cobra's default for unknown commands)

#### Scenario: `hop where <name>` rejected at the binary

- **GIVEN** the user invokes the binary directly
- **WHEN** they run `hop where <name>`
- **THEN** cobra rejects the invocation with `Error: unknown command "where" for "hop"`
- **AND** exit code is non-zero

#### Scenario: `hop config where` survives unchanged

- **GIVEN** any valid `hop.yaml`
- **WHEN** the user runs `hop config where`
- **THEN** the resolved config path is printed to stdout
- **AND** exit code is 0

## CLI: Bare-name and verb dispatch (binary)

### Requirement: Bare-name 1-arg form errors in the binary (B2 shorthand)

When the binary is invoked with exactly one positional argument that is not a known subcommand or flag, the binary SHALL exit with code 2 and write the following exact line to stderr:

```
hop: bare-name dispatch is shell-only. Add 'eval "$(hop shell-init zsh)"' to your zshrc, or use: hop "<name>" where
```

The `<name>` token in the hint MUST be the literal string `<name>` (a placeholder), not the actual repo argument. The 1-arg bare form is shorthand for `hop <name> cd`; both are shell-only.

#### Scenario: Direct-binary 1-arg invocation

- **GIVEN** the user invokes the binary directly (no shim)
- **WHEN** they run `hop foo` where `foo` matches a repo
- **THEN** the binary writes the bare-name hint to stderr
- **AND** stdout is empty
- **AND** exit code is 2

#### Scenario: Shim 1-arg invocation routes to `cd`

- **GIVEN** the user has run `eval "$(hop shell-init zsh)"`
- **WHEN** they run `hop foo` where `foo` matches a repo
- **THEN** the shim runs `_hop_dispatch cd "foo"` (1-arg branch in the shim)
- **AND** the shim resolves the path via `command hop "foo" where` and `cd`s into it
- **AND** the parent shell's cwd is changed

### Requirement: `cd` verb at `$2` errors in the binary

When the binary receives 2 positional arguments and `args[1] == "cd"`, the binary SHALL exit with code 2 and write the following exact line to stderr:

```
hop: 'cd' is shell-only. Add 'eval "$(hop shell-init zsh)"' to your zshrc, or use: cd "$(hop "<name>" where)"
```

The constant `cdHint` (currently in `src/cmd/hop/cd.go`) SHALL be relocated to `src/cmd/hop/root.go` and updated to the new wording. The fallback example (`cd "$(hop "<name>" where)"`) reflects the new repo-first form for `where`.

#### Scenario: Direct-binary 2-arg `cd` invocation

- **GIVEN** the user invokes the binary directly
- **WHEN** they run `hop foo cd`
- **THEN** the binary writes the updated `cd` hint to stderr
- **AND** stdout is empty
- **AND** exit code is 2

#### Scenario: Shim 2-arg `cd` invocation

- **GIVEN** the user has run `eval "$(hop shell-init zsh)"`
- **WHEN** they run `hop foo cd`
- **THEN** the shim's repo-name branch matches `$2 == cd` and dispatches to `_hop_dispatch cd "foo"`
- **AND** the shim resolves the path via `command hop "foo" where` and `cd`s into it

### Requirement: `where` verb at `$2` works in the binary

When the binary receives 2 positional arguments and `args[1] == "where"`, the binary SHALL resolve `args[0]` via the existing match-or-fzf algorithm and print the resolved absolute path to stdout. Exit code follows the existing `hop <name>` resolution semantics: 0 on success, 1 on no match / fzf missing, 130 on user cancellation.

#### Scenario: Direct-binary 2-arg `where` happy path

- **GIVEN** `hop.yaml` resolves `outbox` uniquely to `~/code/sahil87/outbox`
- **WHEN** the user runs `hop outbox where`
- **THEN** stdout is `~/code/sahil87/outbox\n`
- **AND** stderr is empty
- **AND** exit code is 0

#### Scenario: `where` verb on no match falls through to fzf

- **GIVEN** `hop.yaml` has repos `alpha`, `beta`, `gamma` and `fzf` is on PATH
- **WHEN** the user runs `hop zzz where`
- **THEN** fzf opens with `--query zzz` and zero filtered candidates
- **AND** the user can clear the query inside fzf to browse and pick

### Requirement: Tool-form at `$2` errors in the binary (Option X)

When the binary receives 2+ positional arguments and `args[1]` is neither `where` nor `cd` nor `-R`, the binary SHALL exit with code 2 and write the following exact line to stderr:

```
hop: '<tool>' is not a hop verb (cd, where). For tool-form, install the shim: eval "$(hop shell-init zsh)", or use: hop -R "<name>" <tool> [args...]
```

In the message, `<tool>` MUST be replaced with the literal value of `args[1]` (e.g., `cursor`); `<name>` and `<tool> [args...]` MUST appear as literal placeholder text (the hint is generic, not parameterized on the actual repo). Tool-form is shim-only; the binary does NOT absorb it.

#### Scenario: Direct-binary tool-form attempt (2 args)

- **GIVEN** the user invokes the binary directly
- **WHEN** they run `hop foo cursor`
- **THEN** stderr is `hop: 'cursor' is not a hop verb (cd, where). For tool-form, install the shim: eval "$(hop shell-init zsh)", or use: hop -R "<name>" <tool> [args...]`
- **AND** stdout is empty
- **AND** exit code is 2

#### Scenario: Direct-binary tool-form attempt with extra args (4 args)

- **GIVEN** the user invokes the binary directly
- **WHEN** they run `hop foo somerandomtool a b c`
- **THEN** the binary's `Args: cobra.MaximumNArgs(2)` rejects the invocation (cobra's `accepts at most 2 arg(s), received 4` error)
- **AND** exit code is non-zero
- **NOTE**: The `MaximumNArgs(2)` cap is an intentional narrow contract — the binary does not need to error-handle arbitrary tool-form argv shapes; it only handles the 2-arg disambiguation between `where`, `cd`, and "anything else."

### Requirement: `hop <name> -R <cmd>...` is unaffected

The pre-cobra `extractDashR` argv inspection in `src/cmd/hop/main.go` SHALL be unchanged. The shim continues to rewrite the user-facing `hop <name> -R <cmd>...` form to the binary's internal `command hop -R <name> <cmd>...` shape before invocation, so `extractDashR` keeps its existing argv-shape contract.

#### Scenario: Shim canonical exec form

- **GIVEN** the user has run `eval "$(hop shell-init zsh)"` and `outbox` resolves uniquely
- **WHEN** they run `hop outbox -R git status`
- **THEN** the shim runs `command hop -R outbox git status`
- **AND** `extractDashR` parses `-R outbox` and treats `git status` as the child argv
- **AND** `git status` runs with `cwd = <outbox-path>`
- **AND** exit code matches `git status`'s

## CLI: Shim ($1/$2 dispatch ladder)

### Requirement: Shim's known-subcommand list at `$1` SHALL drop `cd` and `where`

The case-statement in `posixInit` (in `src/cmd/hop/shell_init.go`) MUST remove `cd|` and `where|` from the known-subcommand alternation. The remaining list is:

```
clone|ls|shell-init|config|update|help|--help|-h|--version|completion
```

#### Scenario: `hop where ...` at `$1` is treated as a repo name by the shim

- **GIVEN** the user has run `eval "$(hop shell-init zsh)"` and `where` is NOT a repo name
- **WHEN** they run `hop where outbox`
- **THEN** the shim's known-subcommand branch does NOT match (`where` was removed)
- **AND** the shim falls through to the repo-name branch (rule 5) with `$1 = "where"`, `$2 = "outbox"`
- **AND** the shim runs `command hop -R "where" "outbox"` (tool-form sugar — `outbox` is treated as a tool name and `where` as a "repo")
- **AND** the binary's `resolveByName("where")` matches no repo and returns the standard no-match error
- **NOTE**: This is intentional behavior under the new grammar. Old scripts using `hop where <name>` MUST migrate to `hop <name> where`. There is no compatibility shim.

#### Scenario: `hop cd ...` at `$1` is treated as a repo name by the shim

- **GIVEN** the user has run `eval "$(hop shell-init zsh)"` and `cd` is NOT a repo name
- **WHEN** they run `hop cd outbox`
- **THEN** the shim falls through to the repo-name branch with `$1 = "cd"`, `$2 = "outbox"`
- **AND** the shim's `$2 == cd` check fails (it's checking for `cd` at `$2`, not `$1`)
- **AND** the shim runs `command hop -R "cd" "outbox"` (tool-form sugar)
- **AND** the binary's `resolveByName("cd")` matches no repo and errors out
- **NOTE**: Same migration story as the `where` case above.

### Requirement: Shim repo-name branch SHALL dispatch on `$2`

When `$1` is treated as a repo name (rule 5 in the shim's `hop()` ladder), the shim MUST dispatch on `$2` as follows:

| `$#` and `$2` value | Shim action |
|---------------------|-------------|
| `$# == 1` | `_hop_dispatch cd "$1"` (bare-name → `cd`) |
| `$# >= 2` and `$2 == "cd"` | `_hop_dispatch cd "$1"` (explicit `cd` verb) |
| `$# >= 2` and `$2 == "where"` | `command hop "$1" where` (binary handles directly) |
| `$# >= 2` and `$2 == "-R"` | `command hop -R "$1" "${@:3}"` (canonical exec form) |
| `$# >= 2` and `$2` is anything else | `command hop -R "$1" "$2" "${@:3}"` (tool-form sugar) |

The `_hop_dispatch cd` helper itself MUST be updated: the existing `command hop where "$2"` resolution call MUST be changed to `command hop "$2" where`.

#### Scenario: Shim explicit `cd` verb (2 args)

- **GIVEN** the user has run `eval "$(hop shell-init zsh)"` and `outbox` resolves uniquely
- **WHEN** they run `hop outbox cd`
- **THEN** the shim's repo-name branch matches `$2 == "cd"` and runs `_hop_dispatch cd "outbox"`
- **AND** `_hop_dispatch cd` runs `command hop "outbox" where` to resolve, then `cd --` into the path
- **AND** the parent shell's cwd is changed

#### Scenario: Shim `where` verb (2 args)

- **GIVEN** the user has run `eval "$(hop shell-init zsh)"` and `outbox` resolves uniquely
- **WHEN** they run `hop outbox where`
- **THEN** the shim's repo-name branch matches `$2 == "where"` and runs `command hop "outbox" where`
- **AND** the binary's `where`-verb dispatch prints the resolved path to stdout

#### Scenario: Shim tool-form sugar (2 args)

- **GIVEN** the user has run `eval "$(hop shell-init zsh)"` and `dotfiles` resolves uniquely
- **WHEN** they run `hop dotfiles cursor`
- **THEN** the shim's repo-name branch matches the otherwise case (`$2 != cd`, `!= where`, `!= -R`) and runs `command hop -R "dotfiles" "cursor"`
- **AND** the binary execs `cursor` with `cwd = <dotfiles-path>`

### Requirement: `_hop_dispatch cd` helper SHALL drop the no-`$2` fallback branch

The `_hop_dispatch()` helper's `cd)` arm currently contains a no-$2 fallback that runs `command hop cd`:

```bash
cd)
  if [[ -z "$2" ]]; then
    command hop cd
    return $?
  fi
  ...
```

After the change, the only callers of `_hop_dispatch cd` pass a $2 (bare-name dispatch passes `$1`; explicit-`cd`-verb branch passes `$1`). The `command hop cd` invocation would error anyway (the `cd` subcommand is removed), so the fallback branch MUST be removed entirely. The helper becomes:

```bash
_hop_dispatch() {
  case "$1" in
    cd)
      local target
      target="$(command hop "$2" where)" || return $?
      cd -- "$target"
      ;;
    clone)
      ...
    *)
      command hop "$@"
      ;;
  esac
}
```

#### Scenario: `_hop_dispatch cd` always has `$2`

- **GIVEN** the shim's revised repo-name branch
- **WHEN** any caller invokes `_hop_dispatch cd`
- **THEN** `$2` is always set (either `$1` from the bare-name branch or from the explicit-`cd`-verb branch)
- **AND** the helper does not need a no-`$2` fallback

## CLI: Help text (`rootLong`)

### Requirement: `rootLong` SHALL be rewritten for the new grammar

The `rootLong` constant in `src/cmd/hop/root.go` MUST be replaced with the following exact text:

```
hop — locate, open, and operate on repos from hop.yaml.

Getting started:
  1. Run `hop config init` to create a starter hop.yaml.
  2. Edit it to list your repos (each entry: name + git URL + parent dir).
  3. Optional: set $HOP_CONFIG in your shell rc to point at a tracked file
     (git-tracked dotfile, Dropbox, etc.) so it follows you across machines.
  4. For interactive use, install the shim: eval "$(hop shell-init zsh)"

Usage:
  hop                       fzf picker, print selection
  hop <name>                cd into the repo (shell function — needs `eval "$(hop shell-init zsh)"`)
  hop <name> cd             same — explicit verb form
  hop <name> where          echo abs path of matching repo
  hop <name> -R <cmd>...    run <cmd>... with cwd = <name>'s repo dir
  hop <name> <tool>...      shim-only sugar for `hop <name> -R <tool> ...` (e.g., `hop dotfiles cursor`)
  hop clone <name>          git clone the repo if it isn't already on disk
  hop clone <url>           ad-hoc clone: clone the URL, register it in hop.yaml, print landed path
  hop clone --all           clone every repo from hop.yaml that isn't already on disk
  hop clone                 fzf picker, then clone if missing
  hop ls                    list all repos
  hop shell-init <shell>    emit shell integration (zsh or bash). Use: eval "$(hop shell-init zsh)"
  hop config init           bootstrap a starter hop.yaml
  hop config where          print the resolved hop.yaml path
  hop config scan <dir>     scan a directory for git repos and populate hop.yaml
  hop update                self-update the hop binary via Homebrew
  hop -h | --help           show this help
  hop -v | --version        print version

Notes:
  - `hop <name>` and `hop <name> cd` require shell integration (a binary can't change
    its parent shell's cwd). Without it, use:  cd "$(hop <name> where)"
  - On ambiguous or no-match queries, fzf opens prefilled with your query.
  - Grammar: first positional is a repo OR a subcommand (mutually exclusive). When it's
    a repo, second positional is a verb (`cd`, `where`), `-R`, or a tool name.
  - Config search order: $HOP_CONFIG, then $XDG_CONFIG_HOME/hop/hop.yaml, then $HOME/.config/hop/hop.yaml.
```

The `config scan` row MUST be present (it was missing in the prior text — the help-text drift the original Option A surfaced). The `config where` row is preserved.

#### Scenario: `hop -h` reflects the new grammar

- **WHEN** the user runs `hop -h`
- **THEN** stdout contains the new `rootLong` text verbatim (cobra wraps it with its own header/footer)
- **AND** the `Usage:` block shows `hop <name>`, `hop <name> cd`, `hop <name> where` rows in the new order
- **AND** the `Usage:` block does NOT contain `hop where <name>` or `hop cd <name>` rows
- **AND** the `Usage:` block contains `hop config scan <dir>` (drift fixed)
- **AND** exit code is 0

## CLI: Source-tree changes

### Requirement: Source files SHALL be reorganized

| File | Action | Notes |
|------|--------|-------|
| `src/cmd/hop/cd.go` | DELETE | Subcommand factory `newCdCmd` and the old `cdHint` constant are removed. The new `cdHint` (with updated wording) lives in `root.go`. |
| `src/cmd/hop/cd_test.go` | DELETE | Tests covered the removed subcommand surface. New error-path tests are in `bare_name_test.go`. |
| `src/cmd/hop/where.go` | RENAME to `resolve.go` (`git mv`) | All resolution helpers (`loadRepos`, `resolveByName`, `buildPickerLines`, `resolveOne`, `resolveAndPrint`) stay. The `newWhereCmd` factory is dropped. The new filename describes what the file actually contains. |
| `src/cmd/hop/where_test.go` | RENAME to `resolve_test.go` (`git mv`) | Cobra-surface tests targeting `newWhereCmd` are dropped (`TestWhereExactMatch`, `TestBareSingleArgDelegatesToWhere`, `TestWhereRequiresArg`, `TestWhereConfigMissingError`). Helper tests (`TestBuildPickerLinesGroupSuffixOnCollision`, `TestBuildPickerLinesNoCollision`, `TestPathSubcommandRemoved`) and the `singleRepoYAML` fixture stay. |
| `src/cmd/hop/root.go` | MODIFY | `Args: cobra.MaximumNArgs(2)`, new `RunE` dispatching on `args[1]`, new `rootLong`, `cdHint` and `bareNameHint` and `toolFormHint` constants relocated/added here, `newWhereCmd()` and `newCdCmd()` removed from `AddCommand`. |
| `src/cmd/hop/shell_init.go` | MODIFY | Drop `cd|where|` from the known-subcommand alternation; add `$2 == "cd"` and `$2 == "where"` branches to the repo-name dispatch; update `_hop_dispatch cd`'s resolver call (`command hop where "$2"` → `command hop "$2" where`); drop the no-$2 fallback in `_hop_dispatch cd`; update the comment block at the top of `posixInit` to reflect the new ladder. |
| `src/cmd/hop/bare_name_test.go` | NEW | Covers the binary's error/happy paths under the new grammar (see Tests requirement). |

The `git mv` for `where.go` → `resolve.go` and `where_test.go` → `resolve_test.go` MUST preserve git history.

#### Scenario: Source tree after apply

- **GIVEN** the apply stage has completed
- **WHEN** I list `src/cmd/hop/`
- **THEN** `cd.go` and `cd_test.go` do NOT exist
- **AND** `where.go` and `where_test.go` do NOT exist
- **AND** `resolve.go` and `resolve_test.go` exist (contents per renames above)
- **AND** `bare_name_test.go` exists
- **AND** `root.go` and `shell_init.go` reflect the modifications

## CLI: Tests

### Requirement: New `bare_name_test.go` SHALL cover all binary error paths and the `where` happy path

The new file `src/cmd/hop/bare_name_test.go` MUST contain tests for:

1. `hop foo` (1 arg) — exit 2; stderr matches the bare-name hint exactly; stdout empty.
2. `hop foo cd` (2 args) — exit 2; stderr matches the updated `cd` hint exactly (containing `cd "$(hop "<name>" where)"`); stdout empty.
3. `hop foo where` (2 args, fixture `foo` resolves uniquely) — exit 0; stdout is the resolved absolute path; stderr empty.
4. `hop foo somerandomtool` (2 args) — exit 2; stderr matches the tool-form hint exactly (with `'somerandomtool'` interpolated); stdout empty.
5. (Optional) `hop foo somerandomtool extra` (3 args) — exit handled by cobra's `MaximumNArgs(2)` guard.

Tests MUST use the existing `runArgs` helper from `testutil_test.go` and assert exact byte matches against the relocated hint constants (`bareNameHint`, `cdHint`, `toolFormHint`) where applicable.

#### Scenario: Direct-binary 1-arg error test

- **GIVEN** `bare_name_test.go::TestBareNameHint`
- **WHEN** `runArgs(t, "foo")` is called with a fixture that has a `foo` repo
- **THEN** the returned error is an `*errExitCode` with `code == 2`
- **AND** `withCode.msg == bareNameHint`

#### Scenario: Direct-binary 2-arg `where` happy path test

- **GIVEN** `bare_name_test.go::TestWhereVerb` with the `singleRepoYAML` fixture (now in `resolve_test.go`)
- **WHEN** `runArgs(t, "hop", "where")` is called
- **THEN** stdout is `/tmp/test-repos/hop\n`
- **AND** error is nil

### Requirement: Existing tests touching legacy `hop where <name>` / `hop cd <name>` SHALL be migrated

| File | Sites | Migration |
|------|-------|-----------|
| `src/cmd/hop/integration_test.go` | `TestIntegrationCdHint` (4 sites: `hop cd anything` + 3 hint substring asserts) | Update to invoke `hop foo cd` (2-arg form); update hint assertions to match the new wording. |
| `src/cmd/hop/integration_test.go` | `TestIntegrationWhereAndLs` (1 site: `hop where alpha`) | Migrate to `hop alpha where`. |
| `src/cmd/hop/integration_test.go` | `TestIntegrationShellInitBashSourceable` (1 site: `hop where probe` invoked via the bash shim) | Update to test the shim's `$2 == where` branch — invoke as `hop probe where` (the shim now routes this to `command hop probe where`). |
| `src/cmd/hop/shell_init_test.go` | `TestShellInitZshContainsHopFunctionAndAliases` (1 site: assert `command hop where "$2"`) | Update assertion to `command hop "$2" where`. |
| `src/cmd/hop/shell_init_test.go` | Tests asserting subcommand case-list (`TestShellInitZshDoesNotListCodeAsSubcommand`, `TestShellInitZshDoesNotListOpenAsSubcommand`, `TestShellInitZshListsHelpAsSubcommand`) | The phase-1 anchor (`|completion)` + `shell-init`) still works. Phase-2 assertions for `code` and `open` absence still pass. The `help` presence assertion still passes. ADD: a new test (e.g. `TestShellInitZshDoesNotListCdOrWhereAsSubcommand`) that runs the same anchor and asserts `cd` and `where` are NOT in the case-list. |
| `src/cmd/hop/shell_init_test.go` | Add a test asserting the shim emits the new `$2 == "where"` branch (`command hop "$1" where`) and the new `$2 == "cd"` branch (`_hop_dispatch cd "$1"`). |
| `src/cmd/hop/where_test.go` (renamed) | All tests targeting `newWhereCmd`'s cobra surface | Drop. Helper tests (`buildPickerLines`) and the cross-cutting `TestPathSubcommandRemoved` stay (the latter tests that the v0.0.1 `path` subcommand is gone — still relevant). |
| `src/cmd/hop/cd_test.go` | All tests | Delete (the file is deleted). |

The migrations MUST be mechanical (search-and-replace with verification). After migration, the full `go test ./...` suite MUST pass.

#### Scenario: Test suite passes after migration

- **GIVEN** the apply stage has completed all source and test changes
- **WHEN** I run `cd src && go test ./...`
- **THEN** all tests pass
- **AND** there are no references to `newWhereCmd`, `newCdCmd`, `hop where <name>` (as a binary invocation), or `hop cd <name>` (as a binary invocation) in the test files

## Specs: Documentation updates

### Requirement: `docs/specs/cli-surface.md` SHALL be rewritten to describe the new grammar

The Subcommand Inventory table, Match Resolution Algorithm caller list, GIVEN/WHEN/THEN scenarios, Help Text section, and Design Decisions section MUST be updated as enumerated below. The file is hand-curated; updates are surgical line-level edits, not a wholesale rewrite.

| Line range (approx) | Edit |
|---------------------|------|
| 8–26 (Subcommand Inventory table) | Drop the `hop where <name>` row (line 12) and `hop cd <name>` row (line 15). Update the `hop <name>` row (line 11) so its behavior reads "Binary: bare-name dispatch is shell-only — print hint to stderr, exit 2. Shell function: cd into the resolved path." Add new rows for `hop <name> cd` (binary: shell-only hint + exit 2; shell: cd) and `hop <name> where` (binary: print resolved path; exit codes match `hop <name>` pre-flip). The existing `hop <name> -R <cmd>...` and `hop <name> <tool>` rows stay. |
| ~31 (Match Resolution Algorithm caller list) | Update from `hop, hop <name>, hop where, hop -R, hop cd, hop clone` to `hop, hop <name>, hop <name> where, hop <name> cd, hop -R, hop clone`. |
| ~64–87 (Unique substring match / Ambiguous substring match / Zero substring match) | Re-scope as `hop <name> where` scenarios for the path-printing form. ADD binary-form `hop <name>` scenarios asserting exit 2 + stderr hint. |
| ~95–108 (`hop cd` binary form / `hop cd` shell-function form) | Rewrite as `hop <name> cd` scenarios: binary form errors (exit 2, updated hint with `cd "$(hop "<name>" where)"`); shim form `cd`s. Drop the `hop cd <name>` framing. |
| ~110–116 (Bare-name dispatch (shell shim)) | Tighten — note that the shim now routes both `hop <name>` (1 arg) AND `hop <name> cd` (2 args) through `_hop_dispatch cd`. Both invoke `command hop "<name>" where` for resolution. |
| ~120+ (`hop <name> -R <cmd>...` and `hop <name> <tool>` scenarios) | Unchanged in substance — they already use the new grammar. Verify no stale wording referencing the old subcommand layout slipped in. |
| ~389 (Help Text section) | Replace the help block snapshot with the new `rootLong` text from this spec's "CLI: Help text" requirement. |
| ~431 (Design Decision #1: `hop cd` is intentionally split) | Generalize: "The `cd` verb at $2 is shell-only — the binary errors with a hint pointing at the shim install and `hop <name> where`. The bare `hop <name>` (1 arg) is shorthand for `hop <name> cd`. Generalizes to: every form that needs the shim errors in the binary; every form the binary can fulfill works in both layers." |
| ~432 (Design Decision #2: Bare-name dispatch lives only in the shim) | Rewrite: "Bare-name dispatch (`hop <name>` 1 arg) is shorthand for `hop <name> cd` (Option B2). Both are shell-only — the binary errors with a hint. This enforces the invariant that any `hop <subform>` either errors in the binary or works in both layers — never two different effects sharing one syntax." |
| ~437 (Design Decision #6: `hop where` and `hop config where`) | Rewrite: "The `where` verb is the explicit path-printer. Used as `hop <name> where` (top-level) and `hop config where` (config namespace). The top-level `where` subcommand was removed in v0.x — `hop <name> where` is the replacement; the verb survives, the subcommand position does not." |
| ~440 (Design Decision #10: Grammar is `subcommand` xor `repo`) | Reaffirm and extend: "Grammar is `subcommand` xor `repo` at $1. When $1 is a repo, $2 is a verb (`cd`, `where`), `-R`, or a tool name. The verbs `cd` and `where` are NOT subcommands at $1 — they exist only at $2 in the repo-first form." |
| Add new Design Decision #13 | "Tool-form is shim-only; the binary errors on `hop <name> <tool>`. The binary could absorb tool-form (`hop -R <name> <tool>` is its internal shape), but doing so blurs the binary's role as a path-printer + error-emitter. Keeping tool-form shim-only preserves the binary's narrow contract and matches the existing posture for `cd` and bare-name." |

#### Scenario: cli-surface.md is consistent with the implementation post-apply

- **GIVEN** apply has completed
- **WHEN** I `grep -n 'hop where <name>' docs/specs/cli-surface.md` (excluding code-block prose that documents removal)
- **THEN** matches are limited to migration-note context (e.g., "removed without aliases")
- **AND** `grep -n 'hop cd <name>' docs/specs/cli-surface.md` similarly matches only migration notes

### Requirement: `docs/specs/architecture.md` SHALL reflect the file moves

| Line range (approx) | Edit |
|---------------------|------|
| 20–21 (source-tree diagram in §"Top-Level Repository Layout") | Drop the `cd.go` row entirely. Rename the `where.go` row to `resolve.go` and update its description from `# `hop where`, bare `hop <name>`, shared resolver helpers` to `# bare `hop <name>`, `hop <name> where`/`cd` dispatch helpers, shared resolver helpers`. |
| 110 (file responsibilities table in §"`cmd/hop`") | Same rename and description update as line 20–21. Remove `func newWhereCmd() *cobra.Command — `hop where <name>`.` from the listed exports; preserve the helpers list (`loadRepos`, `resolveByName`, `resolveOne`, `resolveAndPrint`, `buildPickerLines`, `fzfMissingHint`). |
| 214 (Composability Primitives — first bullet) | Rewrite the `hop where <name>` mention: change "**`hop where <name>`** — path resolver. Stdin/stdout-friendly: `cd "$(hop where outbox)"` works as a shell composition." to "**`hop <name> where`** — path resolver. Stdin/stdout-friendly: `cd "$(hop outbox where)"` works as a shell composition. The bare form `hop <name>` (1 arg, shim-only) does the same thing." |

#### Scenario: architecture.md reflects the rename and grammar

- **GIVEN** apply has completed
- **WHEN** I read `docs/specs/architecture.md`
- **THEN** there is no `cd.go` mention (other than perhaps a hint in the change history if we keep one)
- **AND** there is no `where.go` mention (other than as a historical note)
- **AND** the example in §"Composability Primitives" uses `hop outbox where`

### Requirement: `docs/specs/config-resolution.md` line ~251 SHALL be inspected for awkwardness

The file's passing reference at line 251 (voice-fit explanation of `hop config where` referencing `hop where`) MUST be re-read in apply. The `where` verb still exists at `$2`, so the voice-fit argument generally holds. Light copy-edit IS allowed if the wording reads awkwardly post-flip; otherwise no edit is required.

#### Scenario: config-resolution.md unchanged or lightly edited

- **GIVEN** apply has completed
- **WHEN** I read line ~251 of `docs/specs/config-resolution.md`
- **THEN** the wording is consistent with the new grammar (no claims that `hop where <name>` is a top-level subcommand)
- **AND** edits, if any, are limited to that single passage

### Requirement: `README.md` SHALL be swept for legacy patterns

The following patterns MUST be searched for and updated in `README.md`:

| Pattern | Replacement |
|---------|-------------|
| `hop where <name>` (any name) | `hop <name> where` |
| `hop cd <name>` | `hop <name>` (preferred for primary examples) or `hop <name> cd` (when explicitness matters) |
| `cd "$(hop where <name>)"` | `cd "$(hop <name> where)"` |
| `cd "$(hop <name>)"` | `cd "$(hop <name> where)"` |

The current README is known to contain `hop where outbox` (line 86, "Quick tour" section). Other matches must be discovered via grep at apply time. Verify zero residual matches afterward.

#### Scenario: README sweep complete

- **GIVEN** apply has completed
- **WHEN** I run `grep -nE 'hop where [a-z]|hop cd [a-z]|cd "\$\(hop where' README.md`
- **THEN** zero matches are found

## Memory: Hydrate-only updates

### Requirement: `docs/memory/cli/subcommands.md` SHALL be rewritten in hydrate

The hydrate stage MUST update `docs/memory/cli/subcommands.md` to:

- Drop the `hop where <name>` row (line 13) and `hop cd <name>` row (line 14) from the Inventory table.
- Update the `hop` (bare) row to reflect `MaximumNArgs(2)` and the new $2 dispatch.
- Add new rows for `hop <name> cd` (binary errors; shim cds) and `hop <name> where` (binary resolves and prints).
- Update the introductory paragraph that says "Match resolution algorithm used by `hop`, `hop where`, `hop cd`, `hop clone`, `hop -R`" to remove `hop where` and `hop cd` from the caller list and add `hop <name> where` and `hop <name> cd` as the new callers.
- Update the "`hop cd` binary-form text" subsection: rename the heading to `cd verb at $2 — binary-form text`; update the `cdHint` constant snippet to the new wording (`cd "$(hop "<name>" where)"`); ADD a parallel subsection for the bare-name hint and the tool-form hint.
- Update the `hop shell-init <shell>` emitted text subsection: revise the 4-step ladder description; remove `cd|where|` from the known-subcommand list; describe the new `$2 == cd` / `$2 == where` branches; document the `_hop_dispatch cd` helper change (`command hop where "$2"` → `command hop "$2" where`); note the dropped no-$2 fallback.

The memory updates MUST happen in hydrate, NOT apply. Apply only edits source code, specs, README, and tests.

### Requirement: `docs/memory/cli/match-resolution.md` SHALL be updated in hydrate

The hydrate stage MUST update the algorithm's caller-list paragraph from:

> Algorithm shared by every subcommand that takes a `<name>` argument (`hop`, `hop where`, `hop cd`, `hop clone`, `hop -R`).

to:

> Algorithm shared by every subcommand that takes a `<name>` argument (`hop`, `hop <name> where`, `hop <name> cd`, `hop clone`, `hop -R`).

The algorithm itself is unchanged.

### Requirement: `docs/memory/architecture/wrapper-boundaries.md` SHALL be verified in hydrate

The hydrate stage MUST grep `docs/memory/architecture/wrapper-boundaries.md` for stale references to `hop where` or `hop cd`. The Composability Primitives section near the bottom currently mentions `hop where <name>` — update that one bullet to `hop <name> where` (mirroring the spec edit). No other changes expected; verify only.

## Design Decisions

1. **Full grammar unification (chosen) over partial fixes.**
   - *Why*: Patching only the binary-vs-shim asymmetry on `hop <name>` (Option A) leaves `cd` and `where` at `$1`, preserving two grammars. Dropping only `cd` collapses one form but leaves `where` at `$1`. Full unification gives one grammar, two fewer top-level subcommands, and naturally generalizes the existing tool-form sugar pattern.
   - *Rejected*: (a) Stay with Option A only — fixes the dichotomy on one form but leaves dual-grammar tax. (b) Drop `cd`, keep `where` — preserves `where <name>` for scripts but leaves the grammar mixed. (c) No change — the help-text drift and dual-dispatch-path complexity compound with each new subcommand.

2. **Bare `hop <name>` (1 arg) is shorthand for `hop <name> cd` (Option B2), not bare-name `where`.**
   - *Why*: Users already type `hop foo` with the shim and expect `cd`; the binary-form prints the path. The new shim still routes 1-arg to `cd`. The binary errors with a hint, mirroring the existing `hop cd` pattern. This eliminates the binary-vs-shim asymmetry the original Option A tackled.
   - *Rejected*: B1 (no shorthand at all) — too aggressive; users would have to type `hop foo cd` for every cd. B3 (rollback Option A and make 1-arg print path in both layers) — re-introduces the asymmetry the change is fixing.

3. **Tool-form remains shim-only; binary errors on `hop <name> <tool>` (Option X).**
   - *Why*: The binary's role is path-printer and error-emitter. Absorbing tool-form (`hop -R <name> <tool>` is its internal shape) blurs that contract. Keeping tool-form shim-only matches the existing posture for `cd` and bare-name and is consistent with the v0.x policy of clear binary/shim layering.
   - *Rejected*: Option Y — binary absorbs tool-form. Would unify the surface further but forces the binary to understand verbs vs. tools at `$2`, expanding its parsing surface unnecessarily.

4. **Clean v0.x break — no aliases, no deprecation period.**
   - *Why*: User explicitly waived backwards compatibility. v0.x is pre-stable; aliases would lock in two grammars and defeat the unification. Migration cost is one search-and-replace per script.
   - *Rejected*: Deprecation period (warn on `hop cd <name>` for one minor release) — adds code paths to maintain and delays the cleanup.

5. **`Args: cobra.MaximumNArgs(2)` rather than per-form subcommands.**
   - *Why*: The dispatch logic at `$2` is small (3 cases: `where`, `cd`, anything-else) and shared with bare-name handling. Adding cobra subcommands for `<name> cd` and `<name> where` would force `<name>` into a subcommand position, which is exactly the grammar the change avoids. Keeping the dispatch in `RunE` mirrors the existing bare-form structure (already in `root.go`).
   - *Rejected*: Per-verb cobra subcommands — would not match the repo-first grammar and would re-introduce the dual-dispatch path.

6. **`cdHint` (and new `bareNameHint`, `toolFormHint`) live in `root.go`, not their own file.**
   - *Why*: All three constants are consumed by `root.go::RunE` only. Splitting them into a new file (`hints.go`) would scatter related concerns. `root.go` already holds `rootLong`; the hints belong with it.
   - *Rejected*: Dedicated `hints.go` — premature abstraction for three string constants.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Drop `hop cd <name>` and `hop where <name>` as top-level subcommands; replace with `hop <name> cd` and `hop <name> where` | Confirmed from intake #1 — user picked full unification | S:95 R:50 A:90 D:95 |
| 2 | Certain | Bare `hop <name>` (1 arg) is shorthand for `hop <name> cd` per option B2: cd in shim, error in binary | Confirmed from intake #2 — user explicitly chose B2 over B1/B3 | S:95 R:60 A:90 D:95 |
| 3 | Certain | Binary errors on `hop <name>` (1 arg) and `hop <name> cd` (2 args) with hint pointing at shim install + `hop <name> where` | Confirmed from intake #3 — mirrors existing `hop cd` pattern | S:95 R:80 A:95 D:95 |
| 4 | Certain | Exit code 2 for all binary error paths (bare-name, `cd` verb, tool-form attempts) | Confirmed from intake #4 — matches existing `errExitCode{code: 2}` taxonomy | S:95 R:90 A:95 D:95 |
| 5 | Certain | No backwards compat — clean v0.x break for `hop cd <name>`, `hop where <name>`, `cd "$(hop where <name>)"`, `cd "$(hop <name>)"` | Confirmed from intake #5 — user explicitly waived compat | S:100 R:50 A:95 D:100 |
| 6 | Certain | `hop config where` survives — different namespace, no collision | Confirmed from intake #6 — self-evident from grammar | S:95 R:95 A:95 D:95 |
| 7 | Certain | `hop` (0 args, picker) unchanged in both binary and shim | Confirmed from intake #7 | S:95 R:95 A:95 D:95 |
| 8 | Certain | `hop clone` (all forms) unchanged | Confirmed from intake #8 — print is IPC contract with shim | S:95 R:90 A:90 D:95 |
| 9 | Certain | `hop -R <name> <cmd>...` and `extractDashR` unchanged | Confirmed from intake #9 — outside the unification scope | S:95 R:95 A:95 D:95 |
| 10 | Certain | Change type: refactor | Confirmed from intake #10 — user direction; matches inference rule | S:95 R:95 A:95 D:95 |
| 11 | Certain | Binary's $2 dispatch: only `where` works; `cd` errors with shell-only hint; anything else errors with tool-form-not-a-hop-verb hint (Option X, not Y) | Confirmed from intake #11 | S:95 R:75 A:85 D:80 |
| 12 | Certain | `where.go` → `resolve.go` rename via `git mv` (preserve history); drop `newWhereCmd` factory in place; helpers stay | Confirmed from intake #12 | S:95 R:90 A:95 D:85 |
| 13 | Certain | `cd.go` and `cd_test.go` deleted outright (no rename target) | Confirmed from intake #13 | S:95 R:95 A:95 D:90 |
| 14 | Certain | New file `bare_name_test.go` for the new error paths and 2-arg happy path | Confirmed from intake #14 | S:95 R:90 A:90 D:85 |
| 15 | Certain | Shim's `_hop_dispatch cd` helper updated: `command hop where "$2"` → `command hop "$2" where` | Confirmed from intake #15 | S:95 R:90 A:95 D:95 |
| 16 | Certain | `cd` and `where` removed from the shim's known-subcommand list at $1 | Confirmed from intake #16 | S:95 R:90 A:95 D:95 |
| 17 | Certain | Tab completion at $2 punted to a follow-up | Confirmed from intake #17 | S:95 R:90 A:85 D:80 |
| 18 | Certain | Updated `cd` hint text changes from `cd "$(hop where "<name>")"` → `cd "$(hop "<name>" where)"` | Confirmed from intake #18 | S:95 R:90 A:95 D:95 |
| 19 | Certain | New Design Decision #13 in spec: "Tool-form is shim-only; binary errors on `hop <name> <tool>`" | Confirmed from intake #19 | S:95 R:80 A:85 D:80 |
| 20 | Certain | Memory updates deferred to hydrate per fab convention | Confirmed from intake #20 | S:95 R:95 A:95 D:90 |
| 21 | Certain | README sweep targets `hop where <name>`, `hop cd <name>`, `cd "$(hop where <name>)"`, `cd "$(hop <name>)"` patterns | Confirmed from intake #21 | S:95 R:90 A:90 D:85 |
| 22 | Certain | Tool-form error message wording: `hop: '<tool>' is not a hop verb (cd, where). For tool-form, install the shim: eval "$(hop shell-init zsh)", or use: hop -R "<name>" <tool> [args...]` | Confirmed from intake #22 | S:95 R:80 A:85 D:85 |
| 23 | Certain | In-code touches scoped to the files enumerated in CLI: Source-tree changes (cd.go, cd_test.go, where.go, where_test.go, root.go, shell_init.go) plus tests (integration_test.go, shell_init_test.go) and the new bare_name_test.go | Confirmed from intake #23 | S:95 R:90 A:95 D:95 |
| 24 | Certain | `docs/specs/architecture.md` updates: source-tree diagram (drop cd.go, rename where.go → resolve.go), file responsibilities row, Composability Primitives example | Confirmed from intake #24 | S:95 R:90 A:95 D:90 |
| 25 | Certain | Shim's `_hop_dispatch cd` helper drops the no-$2 fallback branch (the `command hop cd` invocation) | Confirmed from intake #25 — `cd` subcommand is gone, branch is unreachable | S:95 R:85 A:90 D:85 |
| 26 | Certain | `docs/specs/config-resolution.md` line ~251 — verify only, no edit required unless wording reads awkwardly | Confirmed from intake #26 | S:95 R:95 A:90 D:80 |
| 27 | Certain | `cdHint`, `bareNameHint`, `toolFormHint` constants live in `root.go` (not a separate `hints.go` file) | New in spec — Design Decision #6; constants are consumed only by root.go's RunE | S:90 R:90 A:90 D:85 |
| 28 | Certain | Spec rewrite is surgical line-level edits to docs/specs/cli-surface.md and docs/specs/architecture.md, not a wholesale rewrite | Implied by intake's enumerated edits — preserves hand-curated structure | S:90 R:90 A:90 D:90 |
| 29 | Certain | Help text rewrite includes the `hop config scan <dir>` row (fixing the help-text drift the original Option A surfaced) | Implied by intake §4 ("config scan missing from Usage") and the explicit help block reproduction | S:95 R:95 A:95 D:95 |

29 assumptions (29 certain, 0 confident, 0 tentative, 0 unresolved).
