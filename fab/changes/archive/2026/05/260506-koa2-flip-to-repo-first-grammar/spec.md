# Spec: Flip to repo-first grammar

**Change**: 260506-koa2-flip-to-repo-first-grammar
**Created**: 2026-05-06
**Affected memory**: `docs/memory/cli/subcommands.md`, `docs/memory/cli/match-resolution.md`, `docs/memory/architecture/package-layout.md`, `docs/memory/architecture/wrapper-boundaries.md`

## Non-Goals

- **Backwards compatibility for old arg order** — no transitional alias, no warning, no detection. `hop -R outbox git status` (old form) hits cobra's parse error after the flip; users update muscle memory. Consistent with v0.x policy.
- **Verb-on-repo sugar** — `hop outbox where`, `hop outbox ls`, `hop outbox config` are NOT auto-rewritten to verb-first form. Subcommands stay strictly verb-first.
- **Platform abstraction for `open`/`xdg-open`** — `hop open` is removed; users invoke `hop outbox open` (Darwin) or `hop outbox xdg-open` (Linux) explicitly. Cross-platform users who need portable scripts write their own one-liner.
- **Changes to bare-form behavior** — `hop`, `hop <name>`, `hop where <name>`, `hop cd <name>`, `hop ls`, `hop clone *`, `hop config *`, `hop shell-init <shell>`, `hop update`, `hop -h`, `hop --version` are all unchanged in argv shape and behavior.
- **Match resolution algorithm changes** — `MatchOne` and the fzf invocation flow are untouched. The flip only changes which slot the repo name occupies, not how it resolves.
- **`fzf`/`git`/`brew` wrapping** — `internal/proc`, `internal/fzf`, `internal/yamled`, `internal/update` packages are untouched.

## CLI Surface: Repo-First Grammar

### Requirement: Two-Slot Grammar

The hop binary's grammar SHALL be `subcommand` xor `repo`, with no third interpretation. The first positional argument SHALL be either:

- A **known subcommand** — one of `where`, `cd`, `clone`, `ls`, `shell-init`, `config`, `update`, `help`, `completion`, OR
- A **repo name** — interpreted as a `<name>` query for match-resolution

Tools (binaries on PATH) SHALL NOT be interpreted as subcommands by the binary or by the shim. The shim MAY rewrite a 2-arg form `hop <name> <tool>` to `hop -R <name> <tool>` (tool-form sugar), but `<tool>` always occupies `$2`, never `$1`.

#### Scenario: Subcommand in $1

- **GIVEN** `where` is a known subcommand
- **WHEN** I run `hop where outbox`
- **THEN** the binary dispatches to the `where` cobra command
- **AND** stdout is the absolute path of outbox

#### Scenario: Repo name in $1

- **GIVEN** `outbox` is a repo in `hop.yaml` and not a known subcommand
- **WHEN** I run `hop outbox`
- **THEN** the binary dispatches to the bare-form handler (`resolveAndPrint`)
- **AND** stdout is the absolute path of outbox

#### Scenario: Tool name in $1 (treated as repo)

- **GIVEN** `cursor` is a binary on PATH AND `cursor` is also a repo in `hop.yaml`
- **WHEN** I run `hop cursor` (binary, 1 arg)
- **THEN** the binary treats `cursor` as a repo name (not a tool) and prints its path
- **AND** the binary NEVER inspects PATH for `$1` interpretation

#### Scenario: Tool name in $1 (no matching repo)

- **GIVEN** `cursor` is a binary on PATH AND `cursor` is NOT a repo in `hop.yaml`
- **WHEN** I run `hop cursor` (binary, 1 arg)
- **THEN** match-resolution finds zero matches
- **AND** fzf is invoked with `--query cursor` per the existing match-resolution algorithm

### Requirement: `-R` Canonical Form (Flipped)

`hop -R <name> <cmd>...` (old form) SHALL no longer be the canonical form for exec-in-context. The new canonical form SHALL be `hop <name> -R <cmd>...`.

`extractDashR` (in `src/cmd/hop/main.go`) SHALL scan argv looking for `-R`, take the **preceding** token as `<name>`, and take **everything after** `-R` as `<cmd>...`.

The `-R=<value>` syntax is removed — there is no longer a value attached to `-R`. `-R` is a bare flag. (The old form's `-R=name cmd...` had a value; the new form has no value because the name precedes `-R`.)

#### Scenario: New canonical exec form

- **GIVEN** `hop.yaml` resolves `outbox` to `~/code/sahil87/outbox`
- **WHEN** I run `hop outbox -R git status`
- **THEN** `git status` runs with `cwd = ~/code/sahil87/outbox`
- **AND** stdin/stdout/stderr are inherited
- **AND** the parent shell's cwd is unchanged
- **AND** exit code matches `git status`'s exit code

#### Scenario: -R missing the cmd

- **WHEN** I run `hop outbox -R` (no cmd after -R)
- **THEN** stderr shows `hop: -R requires a command to execute. Usage: hop <name> -R <cmd>...`
- **AND** exit code is 2

#### Scenario: -R missing the name

- **WHEN** I run `hop -R git status` (no name preceding -R)
- **THEN** stderr shows `hop: -R requires a name preceding it. Usage: hop <name> -R <cmd>...`
- **AND** exit code is 2

#### Scenario: -R name does not resolve

- **GIVEN** `nope` matches no repo in `hop.yaml`
- **WHEN** I run `hop nope -R echo hi`
- **THEN** stderr shows the standard match-or-fzf no-candidate behavior (or the fzf-cancelled message if user Esc's)
- **AND** exit code is 1 (resolution failed) or 130 (cancelled)

#### Scenario: -R cmd not on PATH

- **WHEN** I run `hop outbox -R notarealbinary`
- **THEN** stderr shows `hop: -R: 'notarealbinary' not found.`
- **AND** exit code is 1

#### Scenario: -R forwards child argv verbatim

- **GIVEN** `hop.yaml` resolves `outbox`
- **WHEN** I run `hop outbox -R jq '.foo' file.json`
- **THEN** `<cmd>...` argv is forwarded verbatim — cobra does NOT try to parse `jq`'s flags as `hop` flags
- **AND** the child receives `jq '.foo' file.json` as its argv

#### Scenario: Old form rejected

- **WHEN** I run `hop -R outbox git status` (old form, name after -R)
- **THEN** the binary surfaces a usage error (no name preceding -R)
- **AND** exit code is 2
- **AND** there is NO compatibility shim suggesting "did you mean ...?"

### Requirement: Tool-Form Sugar (Flipped)

The shim emitted by `hop shell-init <shell>` SHALL recognize a tool-form: when `$1` is non-empty (and is not a known subcommand, flag, or `__complete*`), `$2` is non-empty and not a flag (`-*`), and `$# >= 2`, it SHALL rewrite the call to:

```
command hop -R "$1" "$2" "${@:3}"
```

The shim SHALL NOT inspect PATH for `$1` (`$1` is a repo name, not a tool). The shim SHALL NOT inspect PATH for `$2` either (the binary's `-R` path returns `hop: -R: '<cmd>' not found.` for missing tools — that's the canonical error path).

The binary itself SHALL NOT interpret the tool-form. Invoking the binary directly with `hop outbox cursor` argv hits cobra's "accepts at most 1 arg" error (cursor is not a known subcommand or flag).

#### Scenario: Tool-form basic case

- **GIVEN** `cursor` is on PATH AND `dotfiles` resolves uniquely
- **WHEN** I run `hop dotfiles cursor` under the shim
- **THEN** the shim runs `command hop -R dotfiles cursor`
- **AND** the binary execs `cursor` with `cwd = <dotfiles-path>`
- **AND** exit code matches `cursor`'s

#### Scenario: Tool-form with extra args

- **GIVEN** `git` is on PATH AND `outbox` resolves uniquely
- **WHEN** I run `hop outbox git status --short` under the shim
- **THEN** the shim runs `command hop -R outbox git status --short`
- **AND** the binary execs `git` with argv `[git, status, --short]` and `cwd = <outbox-path>`

#### Scenario: Tool-form where $2 is a builtin (no special handling)

- **GIVEN** `pwd` is a shell builtin AND `outbox` resolves uniquely
- **WHEN** I run `hop outbox pwd` under the shim
- **THEN** the shim runs `command hop -R outbox pwd`
- **AND** the binary execs `/bin/pwd` (the on-PATH binary, not the builtin) with `cwd = <outbox-path>`
- **AND** stdout is the absolute path of outbox
- **AND** there is NO cheerful-error escape hatch — the grammar accepts this redundancy intentionally

#### Scenario: Tool-form where $2 is not on PATH

- **GIVEN** `outbox` resolves uniquely AND `notarealbinary` is not on PATH
- **WHEN** I run `hop outbox notarealbinary` under the shim
- **THEN** the shim runs `command hop -R outbox notarealbinary`
- **AND** the binary emits `hop: -R: 'notarealbinary' not found.` to stderr
- **AND** exit code is 1
- **AND** there is NO cheerful 3-line error from the shim

#### Scenario: Tool-form where $1 is a hop subcommand

- **GIVEN** `where` is a known hop subcommand
- **WHEN** I run `hop where outbox` under the shim
- **THEN** the shim dispatches to the `where` subcommand (subcommand wins)
- **AND** stdout is outbox's absolute path
- **AND** the shim does NOT try tool-form

#### Scenario: Direct binary invocation (no shim)

- **GIVEN** the user has not run `eval "$(hop shell-init zsh)"`
- **WHEN** they run `hop outbox cursor` (binary directly, 2 args)
- **THEN** cobra rejects with `Error: accepts at most 1 arg(s), received 2`
- **AND** exit code is 1
- **AND** the binary does NOT attempt tool-form (which is shim-only)

### Requirement: Removed `hop open` Subcommand

The `hop open` subcommand SHALL be removed from the binary. The `internal/platform` package SHALL be deleted. Cross-platform `open`/`xdg-open` invocation SHALL be the user's responsibility via tool-form (`hop <name> open` on Darwin, `hop <name> xdg-open` on Linux).

The known-subcommand list in the shim's posixInit SHALL NOT contain `open`.

#### Scenario: Old `hop open <name>` rejected

- **WHEN** I run `hop open outbox`
- **THEN** cobra rejects with `Error: unknown command "open" for "hop"`
- **AND** exit code is 1

#### Scenario: New form on Darwin

- **GIVEN** the user is on Darwin AND `open` is on PATH
- **WHEN** I run `hop outbox open` under the shim
- **THEN** the shim runs `command hop -R outbox open`
- **AND** the binary execs `open` with `cwd = <outbox-path>` and no args
- **AND** Darwin's `open` opens the current directory in Finder (its default behavior with no args)

#### Scenario: New form on Linux

- **GIVEN** the user is on Linux AND `xdg-open` is on PATH
- **WHEN** I run `hop outbox xdg-open .` under the shim
- **THEN** the shim runs `command hop -R outbox xdg-open .`
- **AND** the binary execs `xdg-open` with argv `[xdg-open, .]` and `cwd = <outbox-path>`

#### Scenario: `hop open` removed from match-resolution caller list

- **WHEN** I read `docs/memory/cli/match-resolution.md`
- **THEN** the algorithm's caller list does NOT include `hop open`
- **AND** the caller list still includes `hop`, `hop where`, `hop cd`, `hop clone`, `hop -R`

## Shim Behavior: Collapsed Precedence Ladder

### Requirement: Four-Step Precedence Ladder

The shim's `hop()` function SHALL implement exactly the following resolution order, in order, first match wins:

1. `$# == 0` → `command hop` (bare picker)
2. `$1 == __complete*` → `command hop "$@"` (cobra completion forwarding)
3. `$1 ∈ {cd, clone, where, ls, shell-init, config, update, help, --help, -h, --version, completion}` → `_hop_dispatch "$@"`
4. `$1 == -*` (any other flag) → `command hop "$@"`
5. otherwise (`$1` is treated as a repo name):
   - `$# == 1` → `_hop_dispatch cd "$1"` (bare-name → cd)
   - `$2 == "-R"` → `command hop -R "$1" "${@:3}"` (canonical exec form, shim-rewritten so the binary still sees `-R <name> <cmd>...` shape internally — see Requirement: extractDashR Inversion below for binary-side semantics)
   - otherwise → `command hop -R "$1" "$2" "${@:3}"` (tool-form sugar)

The shim SHALL NOT include any of the following (all removed from today's `posixInit`):

- The leading-slash check on `command -v "$1"` (PATH inspection of `$1`)
- The `type "$1"` builtin/keyword/alias/function detection branch
- The cheerful-error stderr printf for builtins/keywords (`'pwd' is a shell builtin...`)
- The cheerful-error stderr printf for not-on-PATH typos (`'notarealbinary' is not a known subcommand or a binary on PATH`)
- The "$2 is a flag" fallback to forward to the binary (replaced by step 5's tool-form unconditional rewrite, which forwards flag-shaped `$2` through `-R` and lets the binary handle it)

> **NOTE on the shim-to-binary `-R` rewrite**: The shim rewrites `hop <name> -R <cmd>...` to `command hop -R <name> <cmd>...` (binary-side `-R` flag in front of the name). This means the **binary's** `extractDashR` continues to expect the old shape (`-R <name> <cmd>...`) — only the **shim** flips. This is the simpler path: the binary's pre-cobra interceptor stays mechanically identical, and the shim does the user-facing flip. **The Requirement immediately below (extractDashR Inversion) is therefore REVISED**: `extractDashR` does NOT change its argv expectations; only its help text mentions the new user-facing form. Implementation deletes the `-R=<value>` shorthand only if we want a stricter parse — see Design Decision below.

#### Scenario: Bare picker

- **WHEN** I run `hop` (no args) under the shim
- **THEN** the shim runs `command hop`
- **AND** the binary opens fzf

#### Scenario: __complete forwarding

- **WHEN** the cobra-generated completion script runs `hop __complete some-prefix` under the shim
- **THEN** the shim forwards verbatim to `command hop __complete some-prefix`
- **AND** the shim does NOT route through `_hop_dispatch`

#### Scenario: Subcommand dispatch

- **WHEN** I run `hop where outbox` under the shim
- **THEN** the shim's case matches `where` and dispatches via `_hop_dispatch`
- **AND** `_hop_dispatch` falls through to `command hop where outbox`

#### Scenario: Flag forwarding

- **WHEN** I run `hop --version` under the shim
- **THEN** the shim's `-*` case matches and runs `command hop --version`

#### Scenario: Bare-name → cd

- **GIVEN** `outbox` is a repo
- **WHEN** I run `hop outbox` (1 arg) under the shim
- **THEN** the shim's "otherwise" case fires
- **AND** `$# == 1`, so the shim runs `_hop_dispatch cd outbox`
- **AND** the parent shell's cwd changes to outbox's path

#### Scenario: Canonical exec form via shim

- **GIVEN** `outbox` resolves uniquely
- **WHEN** I run `hop outbox -R git status` under the shim
- **THEN** the shim's "otherwise" case fires (`$1 == outbox`, not a subcommand)
- **AND** `$2 == -R`, so the shim runs `command hop -R outbox git status`
- **AND** the binary's `extractDashR` parses `-R outbox git status` (its existing shape) and execs `git status` in outbox

#### Scenario: Tool-form via shim

- **GIVEN** `outbox` resolves uniquely AND `cursor` is on PATH
- **WHEN** I run `hop outbox cursor` under the shim
- **THEN** the shim's "otherwise" case fires
- **AND** `$# >= 2` and `$2 != -R`, so the shim runs `command hop -R outbox cursor`
- **AND** the binary execs `cursor` in outbox

#### Scenario: Shim function size

- **WHEN** I read the `hop()` function body in `posixInit`
- **THEN** the function body is at most ~30 lines (excluding comments)
- **AND** the function does NOT call `command -v`
- **AND** the function does NOT call `type`
- **AND** the function does NOT print cheerful errors to stderr

## Binary Behavior: Pre-Cobra `-R` Interception (Unchanged)

### Requirement: extractDashR Internal Shape Unchanged

The binary's `extractDashR` function (in `src/cmd/hop/main.go`) SHALL continue to scan argv for `-R` followed by `<name>` followed by `<cmd>...` (the existing internal shape). The shim is responsible for rewriting user-facing `hop <name> -R <cmd>...` into the internal `hop -R <name> <cmd>...` shape before the binary sees it.

The user-facing help text (`rootLong` in `root.go` and `cli-surface.md`) SHALL document the new user-facing form (`hop <name> -R <cmd>...`) and NOT the internal binary shape.

The `-R=<value>` syntax (e.g., `hop -R=outbox git status`) MAY be retained for direct binary invocations (no shim). Whether to retain or delete is a Design Decision below.

#### Scenario: Direct binary invocation with -R

- **GIVEN** the user invokes the binary directly without the shim
- **WHEN** they run `/usr/local/bin/hop -R outbox git status`
- **THEN** `extractDashR` parses correctly (its existing logic)
- **AND** the binary execs `git status` in outbox
- **AND** exit code matches git's

#### Scenario: User-facing help text

- **WHEN** I run `hop -h`
- **THEN** the help text shows `hop <name> -R <cmd>...` as the canonical exec form
- **AND** the help text does NOT show `hop -R <name> <cmd>...`
- **AND** the `Notes:` block describes that `-R` follows the repo name

## Help Text: Updated Surface

### Requirement: Updated rootLong

`src/cmd/hop/root.go::rootLong` SHALL be updated to:

- Remove the `hop open <name>` row from the Usage table
- Replace the `hop -R <name> <cmd>...` row with `hop <name> -R <cmd>...`
- Replace the shim sugar row `hop <tool> <name> [args...]` with `hop <name> <tool> [args...]`
- Update the `Notes:` block:
  - Remove the note about `hop <tool> <name>` arg order
  - Remove any reference to builtin/keyword filtering (no longer applicable)
  - Add: "The shim's `hop <name> <tool>` and `hop <name> -R <cmd>...` forms run a tool inside a repo. The repo name always comes first."

#### Scenario: Help text shows new forms

- **WHEN** I run `hop -h`
- **THEN** stdout contains the rows: `hop <name> -R <cmd>...`, `hop <name> <tool> [args...]`
- **AND** stdout does NOT contain `hop -R <name>` (old form)
- **AND** stdout does NOT contain `hop open` (subcommand removed)

## Test Updates

### Requirement: dashr_test.go Unchanged Internally

Since `extractDashR`'s internal argv shape is unchanged (per "Binary Behavior" requirement above), the tests in `src/cmd/hop/dashr_test.go` SHALL remain mechanically unchanged. The tests verify the binary's internal interceptor; they do not test the user-facing form.

#### Scenario: Existing dashr tests pass

- **WHEN** I run `cd src && go test ./cmd/hop/ -run TestExtractDashR`
- **THEN** all existing test cases pass without modification

### Requirement: Integration Tests Updated for Shim Output

`src/cmd/hop/integration_test.go` SHALL be updated to:

- Remove any test cases that exercise `hop open` (subcommand deleted)
- Verify the shim's posixInit body does NOT contain `command -v`, `type`, the cheerful-error printf strings, or the `open` token in the known-subcommand case-list
- Add test cases verifying the shim rewrites `hop <name> <tool>` to `command hop -R <name> <tool>` and `hop <name> -R <cmd>` to `command hop -R <name> <cmd>` (these can be unit-tested by capturing the shim emit and inspecting it as a string, OR by spawning a real shell with the shim sourced — pick whichever the existing test pattern uses)

#### Scenario: Shim emit contains expected forms

- **WHEN** I run `hop shell-init zsh` and capture stdout
- **THEN** stdout contains the new ladder structure (4 effective steps in the `*)` case branch)
- **AND** stdout does NOT contain `command -v "$1"`
- **AND** stdout does NOT contain `type "$1"`
- **AND** stdout does NOT contain `is a shell builtin`
- **AND** stdout does NOT contain `is not a known subcommand or a binary on PATH`
- **AND** stdout does NOT contain `open|` in the known-subcommand case-list

#### Scenario: hop open tests deleted

- **WHEN** I list `src/cmd/hop/`
- **THEN** `open.go` and `open_test.go` are NOT present
- **AND** `src/internal/platform/` directory is NOT present

#### Scenario: Existing -R integration tests pass with new shim

- **GIVEN** an integration test that runs the shim function `hop outbox -R git status`
- **WHEN** the test executes via a real shell with the shim sourced
- **THEN** the shim rewrites to `command hop -R outbox git status`
- **AND** the binary execs git correctly

## Memory Updates

### Requirement: cli/subcommands.md Reflects New Grammar

`docs/memory/cli/subcommands.md` SHALL be updated to:

- Remove the `hop open [<name>]` row from the Inventory table
- Update the `hop -R` row to show `hop <name> -R <cmd>...`
- Update the shim-sugar row to show `hop <name> <tool> [args...]`
- Add a "Removed subcommands" entry: `The open subcommand has been removed (no alias). Use the shim's tool-form: hop <name> open (Darwin) or hop <name> xdg-open (Linux). Or invoke the binary directly: hop -R <name> open.`
- Replace the "Tool-form dispatch" section (precedence ladder) with a description of the new 4-step ladder
- Remove the cheerful-error documentation entirely

#### Scenario: Inventory reflects new grammar

- **WHEN** I read `docs/memory/cli/subcommands.md`
- **THEN** the Inventory table does NOT contain `hop open`
- **AND** the Inventory table contains `hop <name> -R <cmd>...` (not `hop -R <name> <cmd>...`)
- **AND** the Inventory table contains `hop <name> <tool> [args...]` (not `hop <tool> <name> [args...]`)

### Requirement: cli/match-resolution.md Updated

`docs/memory/cli/match-resolution.md` SHALL drop `hop open` from the list of subcommands using the algorithm.

#### Scenario: match-resolution caller list updated

- **WHEN** I read `docs/memory/cli/match-resolution.md`
- **THEN** the opening paragraph lists callers as `hop`, `hop where`, `hop cd`, `hop clone`, `hop -R` (no `hop open`)

### Requirement: architecture/package-layout.md Updated

`docs/memory/architecture/package-layout.md` SHALL remove the `internal/platform/` and `cmd/hop/open.go` / `open_test.go` entries.

#### Scenario: package-layout reflects deletions

- **WHEN** I read `docs/memory/architecture/package-layout.md`
- **THEN** there is no `internal/platform/` entry in the layout tree
- **AND** there is no `open.go` or `open_test.go` entry under `cmd/hop/`

### Requirement: architecture/wrapper-boundaries.md Updated

`docs/memory/architecture/wrapper-boundaries.md` SHALL remove the entire `## internal/platform — OS isolation via build tags` section. The "What is NOT wrapped" table SHALL remain unchanged. The "Composability primitives" section's `hop -R <name> <cmd>...` example SHALL be updated to the new form `hop <name> -R <cmd>...`.

#### Scenario: wrapper-boundaries no longer documents internal/platform

- **WHEN** I read `docs/memory/architecture/wrapper-boundaries.md`
- **THEN** there is no section titled `## internal/platform`
- **AND** there is no reference to `OpenTool()`, `open_darwin.go`, `open_linux.go`
- **AND** the `hop -R` composability example shows `hop <name> -R <cmd>...`

## Spec Updates: cli-surface.md and architecture.md

### Requirement: cli-surface.md Substantive Edits

`docs/specs/cli-surface.md` SHALL be edited per intake Change 5: flip the canonical-form rows for `-R` and tool-form, remove `hop open`, replace cheerful-error scenarios with a single note that the binary's `-R: '<cmd>' not found` covers missing tools, delete Design Decisions #10/#11/#12, add a new design decision summarizing the two-slot grammar.

#### Scenario: cli-surface.md final state

- **WHEN** I read `docs/specs/cli-surface.md`
- **THEN** the Subcommand Inventory table does NOT contain a `hop open` row
- **AND** the Subcommand Inventory `-R` row reads `hop <name> -R <cmd>...`
- **AND** the Subcommand Inventory shim sugar row reads `hop <name> <tool> [args...]`
- **AND** the Behavioral Scenarios section's old "Bare picker", "Unique substring match", "Ambiguous substring match", "Zero substring match", "Group disambiguation in picker", "`hop cd` binary form", "`hop cd` shell-function form", "Bare-name dispatch (shell shim)" scenarios are unchanged
- **AND** the Behavioral Scenarios section's `hop -R` exec-in-context, tool-form, and `hop clone` scenarios are flipped or unchanged per the intake's Change 5
- **AND** the Behavioral Scenarios section's `hop open <name>` scenario is removed
- **AND** the Match Resolution Algorithm caller list does NOT include `hop open`
- **AND** the Stdout/stderr Conventions section does NOT mention `hop open`
- **AND** the External Tool Availability table does NOT have an `open`/`xdg-open` row
- **AND** the Design Decisions section does NOT contain the old #10, #11, #12 (precedence ladder, hop code removal, builtin filtering)
- **AND** the Design Decisions section contains a new decision: `Grammar is subcommand xor repo. The first positional is one or the other — never a tool.`

### Requirement: architecture.md Layout Update

`docs/specs/architecture.md` SHALL remove the `internal/platform/` entries from the layout tree and remove `open.go` and `open_test.go` from the `cmd/hop/` listing.

#### Scenario: architecture.md final state

- **WHEN** I read `docs/specs/architecture.md`
- **THEN** there is no line listing `open.go` or `open_test.go`
- **AND** there is no line listing `internal/platform/`, `open_darwin.go`, `open_linux.go`, or `platform.go` (under `internal/`)

## Source Code Deletions

### Requirement: Files Deleted

The following files SHALL be deleted entirely:

- `src/cmd/hop/open.go`
- `src/cmd/hop/open_test.go`
- `src/internal/platform/platform.go`
- `src/internal/platform/open_darwin.go`
- `src/internal/platform/open_linux.go`
- `src/internal/platform/platform_test.go`
- The `src/internal/platform/` directory itself (after the files are deleted)

#### Scenario: Files removed from working tree

- **WHEN** I run `git status` after the change
- **THEN** the deleted files appear in the index as removed
- **AND** running `cd src && go build ./...` succeeds (no references to `internal/platform` remain)

### Requirement: Imports Cleaned

`src/cmd/hop/main.go`, `src/cmd/hop/root.go`, and any other source file SHALL NOT import `github.com/sahil87/hop/internal/platform` or reference `platform.Open`, `platform.OpenTool`, or any other symbol from that package.

`src/cmd/hop/root.go::newRootCmd()` SHALL NOT include `rootCmd.AddCommand(newOpenCmd())`.

#### Scenario: No platform imports

- **WHEN** I run `grep -rn 'internal/platform' src/`
- **THEN** there are zero matches in production code (matches in test fixtures, if any, are not expected; if found, those tests need updating too)

#### Scenario: No newOpenCmd reference

- **WHEN** I run `grep -rn 'newOpenCmd\|OpenCmd' src/`
- **THEN** there are zero matches

## Backwards Compatibility (Explicit None)

### Requirement: No Compatibility Shim

The change SHALL NOT include any compatibility shim, alias, deprecation warning, or migration helper for the old argv shapes (`hop -R <name> <cmd>...`, `hop <tool> <name>`, `hop open <name>`). Users hit cobra's standard error messages and update their muscle memory.

This is consistent with the v0.x policy already documented in `cli-surface.md` (path → where, config path → config where, hop code → tool-form, all without aliases).

#### Scenario: Old -R form errors via cobra

- **WHEN** I run `hop -R outbox git status` (old form, post-flip)
- **THEN** the binary's `extractDashR` succeeds (binary internal shape is unchanged)
- **AND** the canonical user-facing form per the new help text is `hop outbox -R git status`
- **AND** users invoking the binary directly with the old form still see correct exec behavior, but the documentation does NOT promote it
- **NOTE**: This scenario reflects the deliberate choice in the Binary Behavior requirement above — the binary's internal shape is unchanged for shim-rewrite simplicity. Users who invoke the binary directly with the old form continue to work; only the user-facing canonical form (and the shim's argv expectation) flips.

#### Scenario: Old tool-form shape via shim errors

- **GIVEN** `code` is on PATH AND `outbox` is a repo
- **WHEN** I run `hop code outbox` under the shim (old form)
- **THEN** the shim's case-list does NOT match `code` (it's not a known subcommand)
- **AND** the shim's "otherwise" case fires, treating `code` as a repo name (`$1`)
- **AND** since `code` is not in `hop.yaml`, match-resolution finds zero matches and falls to fzf with `--query code`
- **AND** the shim runs `command hop -R code outbox` (`$1=code`, `$2=outbox`)
- **AND** the binary's `-R` tries to resolve `code` (which is not a repo) — match-resolution finds zero matches, falls to fzf with `--query code`
- **AND** if the user picks no match (Esc), exit 130; if the user picks one, the binary execs `outbox` (likely not on PATH) → `hop: -R: 'outbox' not found.`, exit 1
- **NOTE**: This is the unintended-input behavior. The grammar accepts the old form but interprets it under the new rules — there is no "did you mean" hint. Users debug by reading help.

#### Scenario: Old `hop open <name>` errors via cobra

- **WHEN** I run `hop open outbox` (post-flip)
- **THEN** the shim's case-list still contains `open`? **NO** — the case-list removes `open` per Change 3. So the shim's "otherwise" case fires, treating `open` as a repo name.
- **AND** the shim runs `command hop -R open outbox` (`$1=open`, `$2=outbox`)
- **AND** the binary's `-R` tries to resolve `open` (which is not a repo) — match-resolution finds zero matches, falls to fzf with `--query open`
- **AND** if the user picks one, the binary execs `outbox` (not on PATH) → `hop: -R: 'outbox' not found.`, exit 1
- **NOTE**: Same as above — old syntax produces wrong-but-explicable behavior, no special hint.

## Design Decisions

1. **Shim flips, binary's internal -R shape stays.** The shim rewrites `hop <name> -R <cmd>...` to `command hop -R <name> <cmd>...` (binary-internal old shape). This minimizes binary churn — `extractDashR` and `dashr_test.go` are unchanged. Only the user-facing form flips.
   - *Why*: Two implementation paths considered: (A) flip both shim and binary (extractDashR scans for `-R`, takes preceding token as name, following tokens as cmd; helped by symmetry but breaks all existing dashr tests and risks subtle parse bugs); (B) flip shim only, binary keeps old internal shape (simpler, less risk, fewer tests to update). Chose B for risk reduction. The user-facing form (which is what users see and the help text describes) is what flips.
   - *Rejected*: Flipping `extractDashR` too. Net code change in `extractDashR` would be ~30 lines for ~zero behavioral benefit — the binary is internal infrastructure, not user-facing. Direct binary invocations (no shim) are rare and documented; users who script against the binary use `-R` directly.

2. **`hop open` removal is total — no replacement subcommand.** Users invoke `open`/`xdg-open` directly via tool-form.
   - *Why*: Per Constitution Principle VI (Minimal Surface Area) and Principle IV (Wrap, Don't Reinvent), once tool-form covers the use case generically, the dedicated `open` subcommand is redundant special-casing. The `internal/platform` package's only purpose was to abstract Darwin vs. Linux for `hop open`; without the subcommand, the package has no remaining use.
   - *Rejected*: A "platform-aware tool-form" that detects `$2 == open` and routes to `xdg-open` on Linux. This re-introduces the special case the flip is removing.

3. **Tool-form has no fallthrough fallback for flag-shaped `$2`.** If `$2` starts with `-`, the shim still rewrites to `command hop -R "$1" "$2" "${@:3}"` — the binary handles it.
   - *Why*: Today's shim has a "let the binary surface the error" fallback for flag-shaped `$2` (lines 97-100 of `shell_init.go`). After the flip, that fallback is unnecessary because the new `-R` user-facing form *expects* a flag-shaped `$2 == -R`, and any other flag-shaped `$2` is a usage error that the binary's `-R` path surfaces correctly.
   - *Rejected*: Keeping the fallback. It's dead weight in the new grammar.

4. **`$2` PATH inspection is removed entirely.** The shim does NOT check `command -v "$2"` before rewriting to `-R`. Missing tools surface via the binary's `hop: -R: '<cmd>' not found.` error.
   - *Why*: The PATH check existed (today's `command -v "$1"` line 65) to disambiguate tool vs. typo when `$1` was a tool. After the flip, `$2` is unambiguously a tool name (or `-R`). No disambiguation needed; the binary's error is the canonical path.
   - *Rejected*: Keeping the PATH check on `$2`. Adds complexity for no UX gain — the binary's error is identical in shape and equally cheerful.

5. **Verb-on-repo sugar is NOT included** (e.g., `hop outbox where` does not auto-rewrite to `hop where outbox`).
   - *Why*: Adding it later is non-breaking; removing it would be breaking. Subcommands stay strictly verb-first. Most subcommands (`config init`, `clone --all`, `shell-init zsh`) don't even have a "repo" to put first, so the symmetry would be partial.
   - *Rejected*: Adding it now. Premature; users haven't asked for it.

6. **`hop outbox pwd` is allowed to "just work."** No special handling.
   - *Why*: `/bin/pwd` execs cleanly with `cwd = <outbox-path>`, prints the path. Functionally equivalent to `hop outbox` and `hop where outbox`. Adding a special case to detect this would re-introduce the builtin-filtering complexity the flip is deleting.
   - *Rejected*: Filtering builtins from tool-form. Today's filter exists because `$1`'s ambiguity created the trap; the flip removes the ambiguity, so the trap doesn't exist.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Pure flip (Option A): user-facing canonical form is `hop <name> -R <cmd>...` and tool-form is `hop <name> <tool> [args...]`. | Confirmed from intake #1. User chose explicitly. | S:95 R:60 A:90 D:95 |
| 2 | Certain | `hop open` subcommand is removed; `internal/platform` package is deleted. | Confirmed from intake #2. | S:95 R:55 A:90 D:90 |
| 3 | Certain | No compatibility shim or alias for old arg orders. | Confirmed from intake #3. Consistent with v0.x policy. | S:90 R:60 A:95 D:95 |
| 4 | Certain | yr9l is rejected and archived. | Confirmed from intake #4. Already archived during intake creation. | S:100 R:90 A:95 D:100 |
| 5 | Certain | No verb-on-repo sugar. | Confirmed from intake #5. | S:90 R:80 A:90 D:90 |
| 6 | Certain | `hop outbox pwd` runs `/bin/pwd` with no special handling. | Confirmed from intake #6. | S:85 R:90 A:90 D:85 |
| 7 | Certain | Shim's `hop()` function collapses to a 4-step ladder (5 effective branches counting the `*)` case sub-branches). | Confirmed from intake #7. Specified in detail in the spec body. | S:90 R:75 A:85 D:85 |
| 8 | Certain | Shim's `_hop_dispatch`, `h()`, `hi()`, and the rest of `posixInit` (completion suffix, etc.) are preserved unchanged. | Derived from "minimal change" principle — only the `hop()` function body changes. The completion suffix, `h`/`hi` aliases, and `_hop_dispatch` helper are untouched. | S:90 R:90 A:95 D:95 |
| 9 | Certain | Builtin/keyword filtering and cheerful-error escape hatches are deleted from `posixInit`. | Confirmed from intake #9. Specified explicitly in the Shim Behavior requirement. | S:90 R:80 A:90 D:90 |
| 10 | Certain | Tab completion for the repo slot works for `$1` after the flip with no new completion code. | Confirmed from intake #10. Verified against today's `repo_completion.go::completeRepoNames`. | S:90 R:85 A:90 D:90 |
| 11 | Certain | Help text in `rootLong` and the `Notes:` block updates to reflect new shapes. | Confirmed from intake #12. | S:90 R:85 A:90 D:85 |
| 12 | Certain | The known-subcommand case-list in `posixInit` removes `open`. | Confirmed from intake #3 (Change 3) and #2 (Change 4). | S:95 R:85 A:95 D:95 |
| 13 | Confident | Binary's `extractDashR` internal argv shape is **unchanged** — only the shim's user-facing form flips. The shim rewrites `hop <name> -R <cmd>...` to `command hop -R <name> <cmd>...` before the binary sees it. | New decision at spec stage (not in intake): originally proposed flipping both shim and binary, but reconsidered — flipping only the shim minimizes binary churn (zero changes to `extractDashR` and `dashr_test.go`), reduces test risk, and preserves direct-binary `hop -R outbox <cmd>` invocations for users scripting against the binary. The user-facing form is what flips, and the shim is the user-facing layer. Documented as Design Decision #1 in the spec. | S:75 R:65 A:80 D:75 |
| 14 | Confident | The `-R=<value>` syntax (e.g., `hop -R=outbox git status`) is retained for direct binary invocations. | Symmetric with assumption 13: the binary's internal shape is unchanged, and `extractDashR` already handles `-R=value`. No reason to delete it. The user-facing form (`hop outbox -R git status`) doesn't have a `=` syntax because there's no value to attach to `-R` in the new form. | S:75 R:90 A:85 D:80 |
| 15 | Confident | Integration tests update: remove `hop open` cases, add shim-emit assertions verifying the new ladder structure and absence of `command -v`, `type`, cheerful-error strings, and the `open` token in the case-list. | Mechanical from the spec requirements. The test pattern (capture shim emit, assert string contents) is consistent with `shell_init_test.go`. | S:80 R:80 A:85 D:85 |
| 16 | Confident | Memory updates (cli/subcommands, cli/match-resolution, architecture/package-layout, architecture/wrapper-boundaries) are mechanical from the requirements. | Each memory file has explicit "remove X, update Y to show Z" directives in the spec. No design judgment needed at hydrate time. | S:85 R:90 A:90 D:90 |
| 17 | Confident | Spec updates (cli-surface.md, architecture.md) are substantive but mechanical from the intake's Change 5/6/7. | Same as 16. The intake itself enumerates the substantive edits. | S:85 R:85 A:90 D:90 |
| 18 | Confident | Estimated net deletion: ~200+ lines (shim body, spec design decisions, memory entries, deleted files), replaced by ~50 lines of new shim body and updated docs. | Walked through the intake's Impact section line-by-line; verified `posixInit` shrinkage by counting lines. | S:75 R:90 A:75 D:80 |

18 assumptions (12 certain, 6 confident, 0 tentative, 0 unresolved).
