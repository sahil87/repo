# Plan: Unify repo-verb grammar — `hop <repo> <verb>`, drop `cd`/`where` subcommands

**Change**: 260507-9lk0-tighten-bare-form-binary-error
**Status**: In Progress
**Intake**: `intake.md`
**Spec**: `spec.md`

## Tasks

### Phase 1: Setup (file moves)

- [ ] T001 `git mv src/cmd/hop/where.go src/cmd/hop/resolve.go` to preserve history. No content edits in this task — just the rename.
- [ ] T002 `git mv src/cmd/hop/where_test.go src/cmd/hop/resolve_test.go`. No content edits in this task.
- [ ] T003 `git rm src/cmd/hop/cd.go src/cmd/hop/cd_test.go`. The `cdHint` constant migrates to `root.go` in T005; the test surface is replaced by `bare_name_test.go` in T010.

### Phase 2: Core Implementation

- [ ] T004 Edit `src/cmd/hop/resolve.go` (formerly `where.go`): drop the `newWhereCmd()` factory and its imports if any become unused. Keep all helpers (`loadRepos`, `resolveByName`, `buildPickerLines`, `resolveOne`, `resolveAndPrint`, the `errFzfMissing`/`errFzfCancelled`/`errSilent` sentinels, `fzfMissingHint`).
- [ ] T005 Edit `src/cmd/hop/root.go`:
    1. Add three constants at the top of the file (next to `rootLong`):
       - `cdHint` (relocated from `cd.go`, with new wording: `hop: 'cd' is shell-only. Add 'eval "$(hop shell-init zsh)"' to your zshrc, or use: cd "$(hop "<name>" where)"`)
       - `bareNameHint` (new): `hop: bare-name dispatch is shell-only. Add 'eval "$(hop shell-init zsh)"' to your zshrc, or use: hop "<name>" where`
       - `toolFormHintFmt` (new, format string): `hop: '%s' is not a hop verb (cd, where). For tool-form, install the shim: eval "$(hop shell-init zsh)", or use: hop -R "<name>" <tool> [args...]`
    2. Replace `rootLong` with the new help text from spec.md "CLI: Help text" requirement (verbatim).
    3. Bump `Args: cobra.MaximumNArgs(1)` → `Args: cobra.MaximumNArgs(2)`.
    4. Rewrite `RunE` to dispatch on `len(args)`:
       - `len(args) == 0` → `resolveAndPrint(cmd, "")` (bare picker, unchanged).
       - `len(args) == 1` → return `&errExitCode{code: 2, msg: bareNameHint}`.
       - `len(args) == 2 && args[1] == "where"` → `resolveAndPrint(cmd, args[0])`.
       - `len(args) == 2 && args[1] == "cd"` → return `&errExitCode{code: 2, msg: cdHint}`.
       - `len(args) == 2` (anything else) → return `&errExitCode{code: 2, msg: fmt.Sprintf(toolFormHintFmt, args[1])}`.
    5. Remove `newWhereCmd()` and `newCdCmd()` from the `cmd.AddCommand(...)` list.
    6. Add `"fmt"` to the import list (needed for `fmt.Sprintf`); `"github.com/spf13/cobra"` is already imported.
- [ ] T006 Edit `src/cmd/hop/shell_init.go`:
    1. In the case-list at line ~46, drop `cd|` and `where|` from the alternation. New list: `clone|ls|shell-init|config|update|help|--help|-h|--version|completion`.
    2. In the repo-name `*)` branch (lines ~52-66), expand the dispatch from 3 cases to 5 (in this order, first match wins):
       - `$# == 1` → `_hop_dispatch cd "$1"` (unchanged).
       - `$# >= 2` and `$2 == "cd"` → `_hop_dispatch cd "$1"` (NEW).
       - `$# >= 2` and `$2 == "where"` → `command hop "$1" where` (NEW).
       - `$# >= 2` and `$2 == "-R"` → `command hop -R "$1" "${@:3}"` (unchanged).
       - else → `command hop -R "$1" "$2" "${@:3}"` (tool-form sugar, unchanged).
    3. Update `_hop_dispatch()`'s `cd)` arm: drop the no-`$2` fallback (the `if [[ -z "$2" ]]; then command hop cd; return $?; fi` block); change the resolver call from `command hop where "$2"` to `command hop "$2" where`.
    4. Update the doc-comment block at the top of `posixInit` to reflect the new ladder (the comment currently describes the 5-step ladder including the bare-name and -R rewrite — extend it to mention the new $2 == cd / $2 == where branches).

### Phase 3: Integration & Edge Cases (tests)

- [ ] T007 Edit `src/cmd/hop/resolve_test.go` (formerly `where_test.go`): drop the cobra-surface tests that target `newWhereCmd`'s subcommand registration:
    - `TestWhereExactMatch`
    - `TestBareSingleArgDelegatesToWhere` (the bare 1-arg form now errors, not delegates)
    - `TestWhereRequiresArg`
    - `TestWhereConfigMissingError`
  Keep `TestPathSubcommandRemoved`, `TestBuildPickerLinesGroupSuffixOnCollision`, `TestBuildPickerLinesNoCollision`, and the `singleRepoYAML` fixture constant (used by `bare_name_test.go` too).
- [ ] T008 Edit `src/cmd/hop/integration_test.go`:
    1. `TestIntegrationCdHint`: change the invocation from `bin, "cd", "anything"` to `bin, "anything", "cd"`. Update the hint substring assertions: keep the `'cd' is shell-only` check; update `cd "$(hop where "<name>")"` substring to `cd "$(hop "<name>" where)"`.
    2. `TestIntegrationWhereAndLs`: change `bin, "where", "alpha"` to `bin, "alpha", "where"`. Keep the path assertion.
    3. `TestIntegrationShellInitBashSourceable`: change the inline bash script from `hop where probe` to `hop probe where`. The shim's $2 == where branch now routes this to `command hop probe where`. The expected output is unchanged (resolved path).
- [ ] T009 Edit `src/cmd/hop/shell_init_test.go`:
    1. `TestShellInitZshContainsHopFunctionAndAliases`: change the assertion `command hop where "$2"` → `command hop "$2" where`.
    2. ADD a new test `TestShellInitZshDoesNotListCdOrWhereAsSubcommand` that uses the same phase-1 anchor (`|completion)` + `shell-init`) as the existing `TestShellInitZshDoesNotListCodeAsSubcommand`, then asserts `cd` and `where` are NOT in the located case-list line.
    3. ADD a new test `TestShellInitZshEmitsCdVerbBranch` asserting the shim emits the literal substring `_hop_dispatch cd "$1"` AND the new explicit-cd branch (the case-list now has both 1-arg bare-name and 2-arg explicit-cd routing through the same helper call — the substring assertion already passes once; ADD a structural assertion that two distinct branches emit it, e.g. by counting occurrences or anchoring on surrounding context).
    4. ADD a new test `TestShellInitZshEmitsWhereVerbBranch` asserting the shim emits the literal substring `command hop "$1" where`.
- [ ] T010 Create `src/cmd/hop/bare_name_test.go`:
    - Package `main`, imports: `errors`, `strings`, `testing`. Reuse fixture `singleRepoYAML` from `resolve_test.go`.
    - `TestBareNameHint`: `runArgs(t, "hop")` (1-arg bare with the fixture's `hop` repo); assert `errors.As(err, &withCode)` with `withCode.code == 2` and `withCode.msg == bareNameHint`.
    - `TestBareNameCdVerb`: `runArgs(t, "hop", "cd")`; assert `code == 2`, `msg == cdHint`. Also assert `strings.Contains(withCode.msg, `cd "$(hop "<name>" where)"`)` to pin the new wording.
    - `TestBareNameWhereVerb`: with the fixture loaded via `writeReposFixture(t, singleRepoYAML)`, run `runArgs(t, "hop", "where")`; assert err is nil and stdout is `/tmp/test-repos/hop\n`.
    - `TestBareNameToolForm`: `runArgs(t, "hop", "cursor")`; assert `code == 2` and `strings.Contains(withCode.msg, "'cursor' is not a hop verb")`.
    - `TestBareNameToolFormWithExtraArgs`: `runArgs(t, "hop", "cursor", "--flag")` — cobra's `MaximumNArgs(2)` rejects this. Assert err is non-nil; the exact message comes from cobra and is not asserted byte-for-byte.

### Phase 4: Polish (specs, README)

- [ ] T011 Edit `docs/specs/cli-surface.md` per spec.md "Specs: Documentation updates / cli-surface.md SHALL be rewritten":
    - Drop the `hop where <name>` and `hop cd <name>` rows from the Subcommand Inventory table (lines ~12, ~15).
    - Update the `hop <name>` row to describe binary-form exit-2 + shim-form cd.
    - Add new rows for `hop <name> cd` and `hop <name> where`.
    - Update the Match Resolution Algorithm caller-list paragraph (line ~31).
    - Re-scope the Unique substring / Ambiguous / Zero substring scenarios (lines ~64-87) as `hop <name> where` scenarios; ADD binary-form `hop <name>` exit-2 scenarios.
    - Rewrite the `hop cd` binary form / shell-function form scenarios (lines ~95-108) as `hop <name> cd` scenarios with the new hint wording.
    - Tighten the Bare-name dispatch (shell shim) scenario to note both 1-arg and 2-arg routing.
    - Replace the Help Text snapshot (line ~389) with the new `rootLong`.
    - Update Design Decisions #1, #2, #6, #10 per spec.md; add new Design Decision #13 for tool-form-shim-only.
- [ ] T012 Edit `docs/specs/architecture.md`:
    - Source-tree diagram (lines ~20-21): drop `cd.go` row; rename `where.go` → `resolve.go` and update its description.
    - File responsibilities table (line ~110): same rename + description update; remove `func newWhereCmd() *cobra.Command` from the listed exports.
    - Composability Primitives bullet (line ~214): change `hop where <name>` → `hop <name> where`; update example `cd "$(hop where outbox)"` → `cd "$(hop outbox where)"`.
- [ ] T013 Inspect `docs/specs/config-resolution.md` line ~251: re-read the voice-fit reference to `hop where`. If the wording reads awkwardly post-flip, light copy edit; otherwise no change.
- [ ] T014 Sweep `README.md` for legacy patterns:
    1. `grep -nE 'hop where [a-z]|hop cd [a-z]|cd "\$\(hop where|cd "\$\(hop ' README.md` to enumerate sites.
    2. Replace `hop where outbox` (line 86) → `hop outbox where`. Update any other matches per spec.md "README.md SHALL be swept".
    3. Re-grep to verify zero residual matches.
- [ ] T015 Run the full test suite from `src/`: `cd src && go test ./...`. All tests must pass. If any fail, triage: (a) test still references `hop where <name>` or `hop cd <name>` — fix by migration; (b) implementation bug — return to T005/T006.
- [ ] T016 Run `cd src && go vet ./...` to catch unused imports / dead code from the deletes. Address any findings.
- [ ] T017 Run a constitution audit grep: `grep --include='*.go' --exclude='*_test.go' -rn '"os/exec"' src/internal/ src/cmd/` SHOULD only match `src/internal/proc/`. `grep --include='*.go' --exclude='*_test.go' -rn 'exec\.Command\b' src/` SHOULD match zero. (Sanity check — this change should not perturb either.)

## Execution Order

- Phase 1 (T001-T003) MUST complete before Phase 2 — file moves precede content edits.
- T004 and T005 may run in parallel (different files, no dependency between them).
- T006 may run in parallel with T004 and T005 (`shell_init.go` is independent).
- T007-T010 depend on T004-T005 (test code references the new constants and behaviors).
- T011-T014 (Phase 4) depend on T005-T006 (the implementation must be settled before specs/README claim it).
- T015-T017 are last (verification of the whole change).

## Acceptance

### Functional Completeness

- [ ] A-001 Two-positional repo-verb grammar at root: `cobra.MaximumNArgs(2)` is set on `rootCmd` in `src/cmd/hop/root.go`.
- [ ] A-002 Subcommands `cd` and `where` are removed from the cobra command tree: `cmd.AddCommand(...)` in `root.go` does NOT contain `newCdCmd()` or `newWhereCmd()`; `grep -rn 'newCdCmd\|newWhereCmd' src/cmd/hop/` returns zero in non-test files.
- [ ] A-003 Bare-name 1-arg form errors with the bare-name hint: the binary's `RunE` returns `&errExitCode{code: 2, msg: bareNameHint}` for 1-arg invocations; `bareNameHint` matches the spec verbatim.
- [ ] A-004 `cd` verb at $2 errors with the updated `cdHint`: `RunE` dispatches on `args[1] == "cd"` and returns `&errExitCode{code: 2, msg: cdHint}`; `cdHint` contains `cd "$(hop "<name>" where)"`.
- [ ] A-005 `where` verb at $2 resolves and prints: `RunE` dispatches on `args[1] == "where"` and calls `resolveAndPrint(cmd, args[0])`.
- [ ] A-006 Tool-form attempt at $2 errors with the parameterized tool-form hint: `RunE` returns `&errExitCode{code: 2, msg: fmt.Sprintf(toolFormHintFmt, args[1])}` for the otherwise case.
- [ ] A-007 `hop -R <name> <cmd>...` and `extractDashR` are unchanged: `src/cmd/hop/main.go::extractDashR` is byte-identical to the pre-change version (verify via `git diff`).
- [ ] A-008 Shim's known-subcommand list at $1 drops `cd` and `where`: the case-list line in `posixInit` does NOT contain `cd|` or `where|`.
- [ ] A-009 Shim repo-name branch dispatches on $2 with five cases (1-arg, $2==cd, $2==where, $2==-R, otherwise): the emitted `posixInit` text contains all five branches in order.
- [ ] A-010 `_hop_dispatch cd` helper drops the no-$2 fallback: the emitted `_hop_dispatch()` does NOT contain `if [[ -z "$2" ]]; then command hop cd`.
- [ ] A-011 `_hop_dispatch cd` resolver call updated: the emitted helper contains `command hop "$2" where` (NOT `command hop where "$2"`).
- [ ] A-012 `rootLong` rewritten: the constant in `root.go` matches the new help text from spec.md verbatim, including the `hop config scan <dir>` row.

### Behavioral Correctness

- [ ] A-013 Direct-binary `hop foo` (1 arg) exits 2 with the bare-name hint on stderr and empty stdout: verified by `bare_name_test.go::TestBareNameHint`.
- [ ] A-014 Direct-binary `hop foo cd` (2 args) exits 2 with the updated `cd` hint: verified by `bare_name_test.go::TestBareNameCdVerb`.
- [ ] A-015 Direct-binary `hop foo where` (2 args) prints the resolved path with exit 0: verified by `bare_name_test.go::TestBareNameWhereVerb`.
- [ ] A-016 Direct-binary `hop foo cursor` (2 args) exits 2 with the parameterized tool-form hint: verified by `bare_name_test.go::TestBareNameToolForm`.
- [ ] A-017 Shim emits `_hop_dispatch cd "$1"` for the 1-arg bare-name and 2-arg explicit-cd branches: verified by `shell_init_test.go::TestShellInitZshEmitsCdVerbBranch` (or equivalent).
- [ ] A-018 Shim emits `command hop "$1" where` for the 2-arg explicit-where branch: verified by `shell_init_test.go::TestShellInitZshEmitsWhereVerbBranch`.
- [ ] A-019 Shim case-list at $1 does not contain `cd` or `where`: verified by `shell_init_test.go::TestShellInitZshDoesNotListCdOrWhereAsSubcommand`.
- [ ] A-020 Bash sourceable integration test exercises the new `$2 == where` branch: `TestIntegrationShellInitBashSourceable` invokes `hop probe where` and asserts the resolved path is printed.

### Removal Verification

- [ ] A-021 `src/cmd/hop/cd.go` does not exist: `ls src/cmd/hop/cd.go` fails.
- [ ] A-022 `src/cmd/hop/cd_test.go` does not exist.
- [ ] A-023 `src/cmd/hop/where.go` does not exist (renamed to `resolve.go`).
- [ ] A-024 `src/cmd/hop/where_test.go` does not exist (renamed to `resolve_test.go`).
- [ ] A-025 `src/cmd/hop/resolve.go` exists and contains the resolver helpers; does NOT contain `newWhereCmd`.
- [ ] A-026 `src/cmd/hop/bare_name_test.go` exists and contains `TestBareNameHint`, `TestBareNameCdVerb`, `TestBareNameWhereVerb`, `TestBareNameToolForm`.

### Scenario Coverage

- [ ] A-027 Subcommand at $1 wins over repo-name interpretation: `hop ls` (with a repo named `ls` in the fixture) routes to the `ls` subcommand. Existing `ls_test.go` covers this implicitly.
- [ ] A-028 `hop config where` survives unchanged: `config_test.go` (or equivalent) still exercises the `config where` path.
- [ ] A-029 Direct-binary `hop cd <name>` (legacy form) errors with the tool-form hint: 2 positionals are accepted by `MaximumNArgs(2)`; `cd` is treated as a (non-existent) repo at $1; `<name>` falls into the 2-arg default branch in RunE → `fmt.Sprintf(toolFormHintFmt, "<name>")` to stderr, exit 2. Verified by `resolve_test.go::TestCdSubcommandRemoved`.
- [ ] A-030 Direct-binary `hop where <name>` (legacy form) errors with the tool-form hint: same dispatch as A-029. Verified by `resolve_test.go::TestWhereSubcommandRemoved`.

### Edge Cases & Error Handling

- [ ] A-031 `MaximumNArgs(2)` rejects 3+ args: `bare_name_test.go::TestBareNameToolFormWithExtraArgs` verifies cobra errors out (exact message owned by cobra).
- [ ] A-032 The `_hop_dispatch cd` helper is never called without $2 after the change: the shim's two callers (1-arg bare-name and 2-arg explicit-cd) both pass $1 as the single argument to `_hop_dispatch cd`. Static review of `posixInit`.
- [ ] A-033 Tool-form hint interpolates the actual `args[1]` value: `fmt.Sprintf(toolFormHintFmt, "cursor")` produces `hop: 'cursor' is not a hop verb (cd, where). For tool-form, install the shim: eval "$(hop shell-init zsh)", or use: hop -R "<name>" <tool> [args...]`.

### Documentation

- [ ] A-034 `docs/specs/cli-surface.md` Subcommand Inventory does NOT contain `hop where <name>` or `hop cd <name>` rows.
- [ ] A-035 `docs/specs/cli-surface.md` Subcommand Inventory contains `hop <name> cd` and `hop <name> where` rows.
- [ ] A-036 `docs/specs/cli-surface.md` Help Text section contains the new `rootLong` verbatim.
- [ ] A-037 `docs/specs/cli-surface.md` Design Decisions #1, #2, #6, #10 are updated; new Design Decision #13 is present.
- [ ] A-038 `docs/specs/architecture.md` source-tree diagram does not contain `cd.go` and renames `where.go` to `resolve.go`.
- [ ] A-039 `docs/specs/architecture.md` file responsibilities table reflects the rename and removes `func newWhereCmd()` from the exports list.
- [ ] A-040 `docs/specs/architecture.md` Composability Primitives example uses `hop outbox where`, not `hop where outbox`.
- [ ] A-041 `README.md` contains zero matches for `hop where [a-z]|hop cd [a-z]|cd "\$\(hop where`.

### Code Quality

- [ ] A-042 Pattern consistency: `RunE`'s args-length switch follows the existing pattern for `hop`'s root command (early-return on error, single dispatch helper for happy paths). Constants (`cdHint`, `bareNameHint`, `toolFormHintFmt`) sit beside `rootLong` in `root.go` rather than in their own file.
- [ ] A-043 No unnecessary duplication: the two callers of `_hop_dispatch cd` (1-arg bare-name and 2-arg explicit-cd) reuse the same dispatch line; the resolver call lives in one place (`_hop_dispatch`).
- [ ] A-044 Readability over cleverness: `RunE`'s dispatch is a flat switch on `args[1]`, not a map or table-driven lookup.
- [ ] A-045 No god functions: `RunE` stays well under 50 lines (the dispatch is 10-15 lines).
- [ ] A-046 No magic strings: `cdHint`, `bareNameHint`, `toolFormHintFmt` are named constants. The verb tokens `"cd"` and `"where"` appear inline in the switch — this is acceptable (they are the verbs themselves, not "magic" values).

### Constitution Alignment

- [ ] A-047 Principle I (Security First): no new subprocess invocations introduced. `grep --include='*.go' --exclude='*_test.go' -rn '"os/exec"' src/internal/ src/cmd/` returns matches only in `src/internal/proc/`.
- [ ] A-048 Principle VI (Minimal Surface Area): two top-level subcommands (`cd`, `where`) removed; the binary's user-visible top-level surface shrinks.
- [ ] Test Integrity: all test changes adapt to the new spec; no implementation code is shaped to fit a fixture (the source of truth is spec.md).

## Notes

- T001/T002 (`git mv`) MUST run before T004 — the rename and the content edits to the renamed file are separate commits-worth of work; combining them risks losing rename detection in `git diff`.
- The shell shim's $2 dispatch order matters: `cd` and `where` must be checked before the otherwise (tool-form) case, so a repo named `where-tool` invoked as `hop where-tool where` routes to the explicit-where branch (because $1 is `where-tool`, $2 is `where` — the shim falls into the repo-name branch since `where-tool` is not in the known-subcommand list, then the $2 == where check fires).
- The shim does NOT pre-check whether `$1` matches a known repo (no `command hop where "$1" >/dev/null 2>&1` lookahead). The grammar is "first positional is a subcommand or a repo, you choose"; mistypes surface via the binary's no-match error.
- Acceptance items reference verification mechanisms (specific test names, file existence checks, grep commands) so review can mark them mechanically.
