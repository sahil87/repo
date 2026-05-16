# Spec: Add `hop <name> open` verb

**Change**: 260507-0cjh-add-open-verb
**Created**: 2026-05-08
**Affected memory**: `docs/memory/cli/subcommands.md`, `docs/memory/architecture/wrapper-boundaries.md`, `docs/memory/build/release-pipeline.md`

## Non-Goals

- **No-arg invocation `hop open`** — the verb is only valid as `hop <name> open` (2-arg form). Unlike wt, hop does not detect cwd-as-repo or surface a picker on no-arg. Users wanting picker UX can pipe `hop` (bare picker, prints path) into another command.
- **Cross-platform abstraction inside hop** — hop does not own platform-specific open behavior; the prior `internal/platform` package was deleted and is not being revived. All app detection and launching is delegated to `wt`.
- **No `--app` flag on `hop <name> open`** — the menu UX is the only entry point in this change. A future `--app` could pass through to `wt open --app <name>`, but is not in scope here.
- **No tab-completion for repo names appearing alongside the `open` verb position** — the verb-position completion (offering `cd`/`where`/`open` after a repo name) is added; per-app completion (e.g., suggesting `code`/`cursor` after `hop x -R`) is unchanged and remains out of scope.

## CLI: `hop <name> open` verb

### Requirement: Verb recognition at args[1]

The root command's 2-arg dispatch SHALL recognize `open` as a verb at `args[1]`, alongside `where` and `cd`. When matched, the binary SHALL invoke a new handler `runOpen(cmd, args[0])` that resolves the repo and delegates to `wt open`.

#### Scenario: Verb intercepts before tool-form rewrite

- **GIVEN** the user runs `hop outbox open` (with the shell shim installed) and `outbox` is a repo defined in `hop.yaml`
- **WHEN** the shim's rule-5 dispatch evaluates `$2`
- **THEN** the shim SHALL match the `$2 == "open"` arm and route through `_hop_dispatch open "$1"`
- **AND** the shim SHALL NOT fall through to the otherwise-branch's tool-form rewrite (i.e., `command hop -R outbox open` MUST NOT execute)

#### Scenario: Binary-direct invocation works without the shim

- **GIVEN** the user runs `/path/to/hop outbox open` directly (bypassing the shim)
- **WHEN** the binary's `RunE` evaluates `args[1]`
- **THEN** the binary SHALL match `case "open"` and invoke `runOpen` (i.e., the binary MUST NOT print `toolFormHintFmt` or exit 2)

#### Scenario: Unknown repo name surfaces resolution error

- **GIVEN** the user runs `hop nope open` and `nope` does not match any entry in `hop.yaml`
- **WHEN** `runOpen` calls `resolveByName("nope")`
- **THEN** the binary SHALL surface the same error path as `hop nope where` (fzf-prefilled picker on tty, `errSilent` exit 1 on fzf cancellation; `fzfMissingHint` if fzf is absent)
- **AND** the binary SHALL NOT exec `wt`

### Requirement: Chdir into resolved repo before exec'ing wt

`runOpen` SHALL set `cmd.Dir` to the resolved repo path when invoking `wt open` via `proc.RunForeground`, so wt's `ValidateGitRepo()` cwd-check passes regardless of the parent shell's cwd.

#### Scenario: Open from outside the target repo

- **GIVEN** the user's parent shell cwd is `/home/sahil/Downloads`
- **AND** `outbox` resolves to `/home/sahil/code/sahil87/outbox`
- **WHEN** the user runs `hop outbox open`
- **THEN** hop SHALL invoke `wt open` with `cmd.Dir = /home/sahil/code/sahil87/outbox`
- **AND** wt SHALL see that path as its working directory and pass its `ValidateGitRepo()` check

### Requirement: WT_CD_FILE temp-file mechanism for "Open here" cd

`runOpen` SHALL create a temporary file via `os.CreateTemp("", "hop-open-cd-*")`, set `WT_CD_FILE=<temp-path>` in the env passed to `wt open`, read the file's contents after wt exits, and emit those contents to stdout iff non-empty. The file SHALL be removed via `defer os.Remove`.

#### Scenario: User picks "Open here"

- **GIVEN** the user has the hop shell shim installed
- **AND** wt presents its menu with "Open here" as one option
- **WHEN** the user selects "Open here"
- **THEN** wt SHALL write the resolved repo path to the path named by `WT_CD_FILE`
- **AND** hop SHALL read that file after wt exits, write the path to `cmd.OutOrStdout()`, and exit 0
- **AND** the shim SHALL capture stdout and `cd --` into the path

#### Scenario: User picks any non-"Open here" app

- **GIVEN** the user has the hop shell shim installed
- **WHEN** the user selects any other menu option (e.g., VSCode, Cursor, Finder, tmux window)
- **THEN** wt SHALL launch the chosen app and SHALL NOT write to the `WT_CD_FILE` path (the file remains empty)
- **AND** hop SHALL emit nothing to stdout and exit 0
- **AND** the shim SHALL NOT mutate the parent shell's cwd

#### Scenario: User cancels wt's menu

- **GIVEN** wt presents its menu
- **WHEN** the user cancels (e.g., picks the "Cancel" option or sends SIGINT)
- **THEN** wt's exit code SHALL be propagated by hop via `errExitCode{code: <wt-exit>}` if non-zero
- **AND** hop SHALL emit nothing to stdout
- **AND** the shim SHALL NOT mutate the parent shell's cwd

### Requirement: WT_WRAPPER=1 env suppresses wt's install-hint

`runOpen` SHALL set `WT_WRAPPER=1` in the env passed to `wt open`, so wt suppresses its `hint: "Open here" requires the shell wrapper... eval "$(wt shell-setup)"` message. Hop is acting as the wrapper; the hint is a noisy false positive in this configuration.

#### Scenario: WT_WRAPPER suppresses the wt-shell-setup hint

- **GIVEN** the user runs `hop outbox open` and selects "Open here"
- **WHEN** wt's `OpenInApp("open_here", ...)` checks `WT_WRAPPER`
- **THEN** wt SHALL NOT print the `hint: "Open here" requires the shell wrapper...` line to stderr
- **AND** wt SHALL still write the path to `WT_CD_FILE` (the cd-file path operates independently of the hint)

### Requirement: Binary-direct "Open here" prints a hint when shim is absent

When `runOpen` is invoked binary-direct (the env var `HOP_WRAPPER=1` is not set, indicating no hop shim is wrapping the call) and the user selects "Open here", the binary SHALL still print the path to stdout, AND SHALL print a hint to stderr informing the user that the shim is required for parent-shell cd.

#### Scenario: Open here without shim emits hint

- **GIVEN** the user runs `/path/to/hop outbox open` directly (no hop shim, `HOP_WRAPPER` unset)
- **WHEN** the user selects "Open here" from wt's menu and wt writes to `WT_CD_FILE`
- **THEN** hop SHALL print the path to stdout (so the user can compose `cd "$(hop outbox open)"` manually)
- **AND** hop SHALL print to stderr: `hop: 'Open here' requires the shell shim to cd. Add 'eval "$(hop shell-init zsh)"' to your zshrc, or use: cd "$(hop "<name>" open)"`
- **AND** hop SHALL exit 0

#### Scenario: Open here with shim does not emit hint

- **GIVEN** the user runs `hop outbox open` via the hop shim (which sets `HOP_WRAPPER=1`)
- **WHEN** the user selects "Open here"
- **THEN** hop SHALL print the path to stdout
- **AND** hop SHALL NOT print the no-shim hint to stderr (`HOP_WRAPPER=1` indicates the shim will handle cd)
- **AND** hop SHALL exit 0

### Requirement: Subprocess execution via internal/proc

All subprocess invocations of `wt` SHALL go through `proc.RunForeground` (per Constitution Principle I — Security First). `cmd/hop/open.go` MUST NOT import `os/exec` directly.

#### Scenario: Audit confirms no direct os/exec

- **GIVEN** the implemented `cmd/hop/open.go`
- **WHEN** an audit grep is run: `grep --include='*.go' --exclude='*_test.go' '"os/exec"' src/cmd/hop/open.go`
- **THEN** the audit SHALL return zero matches

### Requirement: Exit code propagation

When `wt` exits with a non-zero code, hop SHALL propagate that exit code via `errExitCode{code: <wt-exit>}`. When `wt` exits 0, hop SHALL exit 0 regardless of whether stdout was emitted.

#### Scenario: wt error exits propagate

- **GIVEN** wt fails internally and exits 5
- **WHEN** `proc.RunForeground` returns an `*exec.ExitError` with exit code 5
- **THEN** `runOpen` SHALL return `&errExitCode{code: 5}` so `translateExit` produces exit 5

#### Scenario: wt missing exits 1

- **GIVEN** `wt` is not on PATH (Homebrew dep contract violated)
- **WHEN** `proc.RunForeground` returns `proc.ErrNotFound`
- **THEN** hop SHALL exit 1 (consistent with the `-R` path's missing-tool behavior)
- **AND** stderr SHALL include the standard "wt not found" message produced by `proc.RunForeground` (no special-case hint constant in hop)

### Requirement: Verb in completion list and Usage table

The cobra `ValidArgsFunction` for the verb position (after a repo name) SHALL include `open` alongside `where` and `cd`. The `rootLong` Usage table SHALL include a row for `hop <name> open`.

#### Scenario: Tab completion offers open

- **GIVEN** the user has typed `hop outbox <TAB>`
- **WHEN** cobra's completion logic evaluates valid args at position 2
- **THEN** the completion list SHALL include `cd`, `where`, AND `open` (in some order; the existing logic determines presentation)

#### Scenario: Help text documents the verb

- **GIVEN** the user runs `hop --help`
- **WHEN** cobra renders `rootLong`
- **THEN** the Usage table SHALL include the line `hop <name> open` with a description such as `open the repo in an app (delegates to wt's menu)`

## CLI: Shell shim integration

### Requirement: Shim recognizes `$2 == "open"` in rule-5 dispatch

In `posixInit` rule 5 (the otherwise-branch where `$1` is treated as a repo name), the dispatch on `$2` SHALL include an arm for `$2 == "open"` that routes through `_hop_dispatch open "$1"`. The arm SHALL be placed alongside the existing `where`/`cd`/`-R` arms.

#### Scenario: Shim dispatches open verb

- **GIVEN** the user runs `hop outbox open` and the shim is loaded
- **WHEN** rule 5 of `hop()` evaluates `$2`
- **THEN** the shim SHALL match the `open` arm and invoke `_hop_dispatch open "outbox"`
- **AND** the shim SHALL NOT fall through to the otherwise-branch (tool-form rewrite)

### Requirement: `_hop_dispatch open)` arm captures stdout and cds conditionally

`_hop_dispatch` SHALL grow an `open)` arm that invokes `command hop "$2" open`, captures stdout into a local variable, and runs `cd -- "<target>"` iff the captured value is non-empty. If the binary exits non-zero, the arm SHALL propagate the exit code (via `return $?`) and SHALL NOT cd.

#### Scenario: Non-empty stdout triggers cd

- **GIVEN** the user picks "Open here" and hop emits a path on stdout
- **WHEN** `_hop_dispatch open "outbox"` runs
- **THEN** `target` SHALL be set to the captured path
- **AND** the shim SHALL run `cd -- "$target"` in the parent shell

#### Scenario: Empty stdout skips cd

- **GIVEN** the user picks any non-"Open here" option and hop emits no stdout
- **WHEN** `_hop_dispatch open "outbox"` runs
- **THEN** `target` SHALL be empty
- **AND** the shim SHALL NOT run `cd` (the conditional `[[ -n "$target" ]]` guard fails)

#### Scenario: Binary failure short-circuits cd

- **GIVEN** the binary exits non-zero (e.g., wt failure, fzf cancellation, missing repo)
- **WHEN** `_hop_dispatch open "outbox"` runs
- **THEN** the shim SHALL `return $?` from the command-substitution failure
- **AND** the shim SHALL NOT attempt `cd` (the early-return prevents it)

### Requirement: Shim exports HOP_WRAPPER=1

The `posixInit` shell function definitions SHALL ensure the binary, when invoked through the shim, sees `HOP_WRAPPER=1` in its environment. This is the binary's signal that the shim is wrapping the call (suppressing the no-shim hint per the runOpen contract).

#### Scenario: HOP_WRAPPER reaches the binary through the shim

- **GIVEN** the shim is loaded via `eval "$(hop shell-init zsh)"`
- **WHEN** the user invokes `hop outbox open` (which routes to `_hop_dispatch open "outbox"` → `command hop "outbox" open`)
- **THEN** the `command hop` subprocess SHALL see `HOP_WRAPPER=1` in its environment

#### Scenario: HOP_WRAPPER absent for direct binary invocation

- **GIVEN** the user runs `/path/to/hop outbox open` directly (no shim)
- **WHEN** the binary reads its env
- **THEN** `HOP_WRAPPER` SHALL be unset (or empty)

### Requirement: Existing verbs and bare-name dispatch unaffected

The new `open` arm SHALL NOT modify any existing dispatch behavior. `hop <name>`, `hop <name> cd`, `hop <name> where`, `hop <name> -R <cmd>...`, and tool-form (`hop <name> <tool>`) SHALL behave exactly as before this change.

#### Scenario: Bare-name still cds

- **GIVEN** the shim is loaded
- **WHEN** the user runs `hop outbox` (single arg)
- **THEN** the shim SHALL route to `_hop_dispatch cd "outbox"` (rule 5, `$# == 1` branch)
- **AND** the parent shell SHALL cd into the resolved path

#### Scenario: where-verb still resolves and prints

- **GIVEN** the shim is loaded
- **WHEN** the user runs `hop outbox where`
- **THEN** the shim SHALL route to `command hop "outbox" where`
- **AND** the binary SHALL print the resolved path to stdout

## Architecture: Wrapper boundaries

### Requirement: wt is invoked via internal/proc, not packaged

The `wt` invocation SHALL be implemented inline in `src/cmd/hop/open.go` using `proc.RunForeground`. There SHALL NOT be a new `internal/wt` wrapper package. This single-call use does not warrant a dedicated package per the "What is NOT wrapped" guidance in `wrapper-boundaries.md`.

#### Scenario: Single inline call, no internal/wt directory

- **GIVEN** the implemented change
- **WHEN** the source tree is inspected
- **THEN** `src/internal/wt/` SHALL NOT exist
- **AND** `src/cmd/hop/open.go` SHALL contain a single `proc.RunForeground` call invoking `wt`

### Requirement: env contract documented

The `wrapper-boundaries.md` memory file SHALL document the env contract for wt invocation: `WT_CD_FILE` (set by hop to a temp path; wt writes resolved path on "Open here") and `WT_WRAPPER=1` (set by hop to suppress wt's install-shim hint).

## Build: Homebrew formula dependency

### Requirement: Formula template declares wt dependency

`.github/formula-template.rb` SHALL declare `depends_on "sahil87/tap/wt"` after the `license "MIT"` line and before the `on_macos` block. The release workflow's `sed`-based substitution SHALL preserve this line in the published `Formula/hop.rb` at the next tagged release.

#### Scenario: Template includes depends_on

- **GIVEN** the implemented change
- **WHEN** `.github/formula-template.rb` is read
- **THEN** the file SHALL contain a line matching `^  depends_on "sahil87/tap/wt"$` between the `license "MIT"` line and the `on_macos do` line

#### Scenario: Future release rewrites Formula/hop.rb with the dep

- **GIVEN** the release workflow runs at a future hop tag
- **WHEN** the `sed` substitution applies VERSION_PLACEHOLDER and SHA_* placeholders to the template
- **THEN** the resulting `Formula/hop.rb` written to `sahil87/homebrew-tap` SHALL include the `depends_on "sahil87/tap/wt"` line verbatim
- **AND** `brew install sahil87/tap/hop` (after that release publishes) SHALL pull `wt` automatically

### Requirement: Live tap untouched in this change

This change SHALL NOT edit or commit to `sahil87/homebrew-tap`. The dep lands in the published formula only when the next hop release is tagged and the workflow rewrites `Formula/hop.rb` from the updated template.

#### Scenario: No homebrew-tap commit in this change

- **GIVEN** the change is shipped
- **WHEN** the homebrew-tap repo's git log is inspected
- **THEN** there SHALL NOT be a new commit attributed to this change (no `hop: depends_on sahil87/tap/wt` commit ahead of the next `hop v<N>` release commit)

## Testing

### Requirement: Fake wt script exercises the cd round-trip

Tests for `runOpen` SHALL use a fake `wt` shell script placed in a temp directory prepended to `PATH`. The fake script SHALL accept `wt open` invocations, optionally write to `$WT_CD_FILE` (simulating "Open here"), and exit 0.

#### Scenario: Fake wt writes to WT_CD_FILE and hop re-emits on stdout

- **GIVEN** a fake `wt` script that writes `$PWD` to `$WT_CD_FILE` on `wt open`
- **AND** that script is in a temp dir prepended to `PATH`
- **WHEN** the test invokes the hop binary as `hop <name> open` with `<name>` resolving to a known temp repo
- **THEN** hop's stdout SHALL contain the temp repo's path
- **AND** hop's exit code SHALL be 0

#### Scenario: Fake wt writes nothing and hop emits nothing

- **GIVEN** a fake `wt` script that does not touch `$WT_CD_FILE` (simulating editor selection)
- **WHEN** the test invokes hop
- **THEN** hop's stdout SHALL be empty
- **AND** hop's exit code SHALL be 0

#### Scenario: Fake wt exits non-zero, hop propagates

- **GIVEN** a fake `wt` script that exits 7
- **WHEN** the test invokes hop
- **THEN** hop's exit code SHALL be 7

### Requirement: Shim golden files cover the new dispatch

Existing tests in `src/cmd/hop/shell_init_test.go` SHALL be extended (not replaced) so the golden output for `hop shell-init zsh` and `hop shell-init bash` includes:
- The new `$2 == "open"` arm in `posixInit` rule 5
- The new `open)` arm in `_hop_dispatch`
- The `HOP_WRAPPER=1` export

#### Scenario: zsh golden file matches

- **GIVEN** the implemented `posixInit` and the test runs `hop shell-init zsh`
- **WHEN** the output is compared to the updated golden
- **THEN** the comparison SHALL match exactly (no diff)

## Design Decisions

1. **Delegate to `wt` rather than reimplement app detection**:
   - *Why*: Constitution Principle IV ("Wrap, Don't Reinvent"). wt's `apps.go` already covers the full set of apps and is maintained as part of the same author's toolchain. Reimplementing it would duplicate ~330 lines of platform-detection code across two repos.
   - *Rejected*: Extracting a shared `openin` Go module imported by both wt and hop. Cleaner long-term, but cross-repo coordination cost is too high for a feature with one consumer per repo. Worth revisiting if a third caller appears.
   - *Rejected*: Reviving an `internal/platform` package inside hop. Walks back the deliberate prior removal and forces hop to own cross-platform open semantics, which it explicitly should not.

2. **Stdout convention for "Open here" cd, not a hop-side temp file**:
   - *Why*: Hop's existing `cd`/`clone` shims already use the stdout convention (`target=$(command hop ... where)`; `cd --` if non-empty). Adding `open` to the same pattern keeps the shim idiomatic.
   - *Rejected*: A `HOP_CD_FILE` temp-file mechanism mirroring wt 1:1. Cleaner separation from stdout but introduces a new env-var convention in hop and adds plumbing to the shim. Not worth the cost for a single verb.
   - *Note*: Internally, hop uses `WT_CD_FILE` (wt's mechanism) to read wt's "Open here" choice — the stdout/temp-file split is asymmetric on purpose. wt → hop uses temp-file (wt's existing contract); hop → shim uses stdout (hop's existing contract).

3. **Replace existing Darwin tool-form behavior rather than picking a new verb name**:
   - *Why*: `open` is the natural verb name. Adding a synonym (e.g., `launch`) creates two ways to spell the same intent. The Darwin one-liner for "Finder at repo dir" is preserved via the menu's Finder choice — one extra keystroke is acceptable.
   - *Rejected*: `hop <name> launch`. Awkward; not idiomatic; users would still try `hop <name> open` first.
   - *Rejected*: Keep tool-form fallback only when no menu app matches the literal `open`. Adds grammar complexity for marginal benefit.

4. **wt as Homebrew formula dependency, not runtime LookPath**:
   - *Why*: Declarative dependency is the right level for "this binary is required to function." Removes the missing-wt error path (one fewer constant, one fewer hint, one fewer test). `Formula/wt.rb` already exists in `sahil87/homebrew-tap`.
   - *Rejected*: Runtime `exec.LookPath("wt")` with a custom install hint. Introduces a hop-owned message that drifts from wt's actual install path. Users on `brew install sahil87/tap/hop` would never see it because the dep makes wt always available.

5. **Live tap (`Formula/hop.rb`) not edited in this change; only `.github/formula-template.rb`**:
   - *Why*: The release workflow rewrites `Formula/hop.rb` from the template at tag-time. Editing the live formula now would advance the `depends_on` line ahead of any binary that needs it — semantic mismatch, even if cosmetic. Cleanest semantics: the dep ships with the binary version that needs it.
   - *Rejected*: Direct edit + push to `sahil87/homebrew-tap` now. User initially asked for this but reconsidered when the timing trade-off was surfaced.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Shell out to `wt open` instead of reimplementing app detection | Confirmed from intake #1; spec-level Design Decision #1 records the rationale and rejected alternatives | S:95 R:80 A:90 D:90 |
| 2 | Certain | Scope limited to `hop <name> open` (no no-arg variants) | Confirmed from intake #2; codified as Non-Goal | S:95 R:80 A:90 D:95 |
| 3 | Certain | "Open here" cd uses stdout convention; binary internally uses `WT_CD_FILE` to read wt's choice | Confirmed from intake #3; Design Decision #2 records the asymmetric split | S:95 R:75 A:90 D:90 |
| 4 | Certain | Binary chdirs into resolved repo path before exec'ing wt | Confirmed from intake #4; codified as a Requirement with explicit scenario | S:95 R:90 A:95 D:95 |
| 5 | Certain | wt is a Homebrew formula dependency, not a runtime LookPath check | Confirmed from intake #13 (upgraded during clarification); Design Decision #4 records the rationale | S:95 R:90 A:95 D:95 |
| 6 | Certain | Replace existing Darwin tool-form behavior with the new verb | Confirmed from intake #6; Design Decision #3 records the rationale | S:95 R:60 A:90 D:85 |
| 7 | Certain | wt's stderr hints (other than the `WT_WRAPPER`-suppressed one) leak through to users | Confirmed from intake #7; codified in the WT_WRAPPER requirement | S:95 R:90 A:95 D:90 |
| 8 | Certain | Replace memory note about prior `open` subcommand removal with new behavior | Confirmed from intake #8; codified in Affected Memory | S:95 R:95 A:95 D:95 |
| 9 | Certain | Use `proc.RunForeground` for the wt invocation; no `os/exec` in `cmd/hop/open.go` | Confirmed from intake #9 (upgraded from Confident); codified as the "subprocess execution via internal/proc" Requirement with audit scenario | S:95 R:90 A:95 D:95 |
| 10 | Certain | New file `src/cmd/hop/open.go` rather than inlining `runOpen` in `root.go` | Confirmed from intake #10 (upgraded from Confident); matches per-subcommand file convention (`clone.go`, `ls.go`, etc.) | S:90 R:85 A:95 D:90 |
| 11 | Certain | Set `WT_WRAPPER=1` in env passed to wt to suppress wt's install-shim hint | Confirmed from intake #11 (upgraded from Confident); codified as a Requirement with explicit scenario | S:95 R:90 A:95 D:95 |
| 12 | Certain | Binary detects shim absence via `HOP_WRAPPER=1` env var (set by the shim) | Confirmed from intake #12 (upgraded from Confident); codified as a Requirement with two scenarios (with-shim, without-shim) | S:90 R:85 A:95 D:95 |
| 13 | Certain | No missing-wt hint constant; `proc.ErrNotFound` path is sufficient | Confirmed from intake #13 (upgraded). Spec-level Requirement codifies the contract: hop exits 1 on `ErrNotFound`; stderr message comes from `proc.RunForeground` itself, not a hop-owned constant | S:95 R:90 A:95 D:95 |
| 14 | Certain | Tab completion for `open` in this change | Confirmed from intake #14 (upgraded); codified as a Requirement with completion scenario | S:90 R:90 A:90 D:95 |
| 15 | Certain | Test via fake `wt` shell script in a temp dir prepended to PATH | Confirmed from intake #15 (upgraded); codified as a Requirement with three scenarios (cd, no-cd, error-propagation) | S:90 R:90 A:90 D:95 |
| 16 | Certain | Live tap (`Formula/hop.rb`) not edited; only `.github/formula-template.rb` | Confirmed from intake #16; Design Decision #5 records the rationale; Requirement codifies that no homebrew-tap commit appears in this change | S:95 R:90 A:95 D:95 |
| 17 | Certain | `HOP_WRAPPER=1` is exported (not function-local) so subprocess hop calls under the shim still see it | Resolved during spec generation. Shim places `export HOP_WRAPPER=1` at top-level so children of any shell-using-the-shim see it. Open question from intake's "scope of HOP_WRAPPER" resolved. | S:90 R:85 A:90 D:90 |
| 18 | Certain | No wt-version compatibility check at runtime | Resolved during spec generation. The brew-formula contract makes version skew rare; if wt's `WT_CD_FILE` contract is ever broken, hop's tests will catch it during release validation. Adding a runtime check would expand surface area for negligible safety. Open question from intake resolved. | S:85 R:80 A:90 D:85 |
| 19 | Certain | No `--app default` pass-through flag in this change | Resolved during spec generation. Codified as Non-Goal. Out of scope; can be added later as a small follow-up if demand emerges. | S:90 R:90 A:95 D:95 |
| 20 | Confident | Shim placement of `export HOP_WRAPPER=1`: at top of `posixInit` immediately after the comment header, before the `hop()` function definition | Convention question — exporting at the top is the simplest place. Adjacent code style in `shell_init.go` is sparse enough that placement is non-load-bearing. | S:75 R:90 A:85 D:80 |
| 21 | Confident | The `open)` arm of `_hop_dispatch` uses the same `local target` declaration style as the `cd)` arm (line 95-97 in shell_init.go) | Existing pattern in `_hop_dispatch` uses `local target` followed by command substitution. New arm should mirror this for consistency. | S:80 R:90 A:90 D:85 |
| 22 | Confident | The new no-shim hint constant is named `openHereNoShimHint` and lives in `cmd/hop/open.go` | Naming consistent with `bareNameHint` / `cdHint` / `toolFormHintFmt` in `root.go`. Locating in `open.go` keeps verb-related constants colocated with the verb's handler. | S:75 R:90 A:90 D:85 |

22 assumptions (19 certain, 3 confident, 0 tentative, 0 unresolved).
