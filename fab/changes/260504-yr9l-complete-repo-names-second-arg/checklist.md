# Quality Checklist: Complete repo names in $2 of two-arg forms

**Change**: 260504-yr9l-complete-repo-names-second-arg
**Generated**: 2026-05-04
**Spec**: `spec.md`

## Functional Completeness

- [ ] CHK-001 `Tab completion of $2 in hop -R <TAB>`: cobra entrypoint `hop __complete -R ""` returns repo-name candidates from `hop.yaml` via `completeRepoNamesForFlag` (registered with `cmd.RegisterFlagCompletionFunc("R", ...)` against a hidden `-R` cobra flag in `root.go`)
- [ ] CHK-002 `Tab completion of $2 in tool-form (hop <tool> <TAB>)`: cobra entrypoint `hop __complete <binary-on-PATH> ""` returns repo-name candidates when `<binary>` is non-subcommand and resolves via `exec.LookPath` to an absolute path
- [ ] CHK-003 `Existing $1 completion behavior is preserved`: `hop __complete ""` still returns all repo names with subcommand-collision filter applied
- [ ] CHK-004 `Completion-aware skip in main`: `main.go::main` does NOT invoke `extractDashR` when `os.Args[1]` is `__complete` or `__completeNoDesc`; otherwise the existing `-R` interception runs unchanged
- [ ] CHK-005 `shouldCompleteRepoForSecondArg detects tool-form shape`: helper returns true for `[<binary-on-PATH>]`; false for `["clone"]`, `[<missing-binary>]`, `["sh", "name"]`, and `[]`. (Note: helper does NOT have an `args[0] == "-R"` branch — `-R` completion is wired via `RegisterFlagCompletionFunc`, not this helper.)
- [ ] CHK-005a `Position 3+ of -R returns no candidates`: `completeRepoNames` checks `cmd.Flag("R").Changed` and returns no candidates when true (handles `hop -R <name> <TAB>`, where cobra has consumed `-R <name>` as a flag pair so `args=[]`)
- [ ] CHK-005b `completeRepoNamesForFlag does NOT apply subcommand-collision filter`: a repo named `clone` (or other subcommand name) IS returned as a `-R` candidate (verified by `TestCompletionDashRSurfacesRepoNamedClone`)

## Behavioral Correctness

- [ ] CHK-006 The `len(args) > 0` early bail in `completeRepoNames` is replaced with `len(args) > 0 && !shouldCompleteRepoForSecondArg(cmd, args)` — verifiable by inspection of `repo_completion.go`
- [ ] CHK-007 `isCompletionInvocation` is package-private, located in `main.go` adjacent to `extractDashR`, and gated only on `args[1] in {"__complete", "__completeNoDesc"}` (not on the wider cobra completion command set)

## Scenario Coverage

- [ ] CHK-008 `hop __complete -R ""` returns full candidate list — covered by new test in `repo_completion_test.go` (T007)
- [ ] CHK-009 `hop __complete -R "alph"` returns full candidate list (shell does prefix matching) — implicitly covered by CHK-008 plus existing `TestCompletionReturnsAllNamesForShellFiltering` pattern
- [ ] CHK-010 Position 3+ of `-R` returns no candidates — covered by new test (T007: `runArgs(t, "__complete", "-R", "name", "")`)
- [ ] CHK-011 Tool-form with binary on PATH returns repo names — covered by new test (T007: `runArgs(t, "__complete", "sh", "")`)
- [ ] CHK-012 Tool-form with binary NOT on PATH returns no candidates — covered by new test (T007: `runArgs(t, "__complete", "hop-nonexistent-tool-xyzzy", "")`)
- [ ] CHK-013 Position 3+ of tool-form returns no candidates — covered by new test (T007: `runArgs(t, "__complete", "sh", "name", "")`)
- [ ] CHK-014 Bare `hop <TAB>` still returns repo names — regression-guarded by existing `TestCompletionListsRepoNames`
- [ ] CHK-015 Subcommand-collision filter still applies at `$1` — regression-guarded by existing `TestCompletionRootFiltersSubcommandCollisions`
- [ ] CHK-016 Normal `hop -R <name> <cmd>` invocation still execs child correctly — regression-guarded by existing `dashr_test.go` tests; manually verifiable by running `hop -R outbox echo hi`
- [ ] CHK-017 Malformed `hop -R` (no value) still produces `extractDashR`'s "requires a value" error and exit 2 — regression-guarded by existing `TestExtractDashRNoValue` family

## Edge Cases & Error Handling

- [ ] CHK-018 `isCompletionInvocation` returns false for `len(args) < 2` (defensive — no panic)
- [ ] CHK-019 Missing `hop.yaml` during `__complete -R ""` surfaces zero candidates with no error (existing `loadRepos` failure path returns `(nil, ShellCompDirectiveNoFileComp)`)
- [ ] CHK-020 A binary that's both on PATH and a hop subcommand (e.g. `ls`) takes the subcommand path — `shouldCompleteRepoForSecondArg` returns false because the subcommand check fires first

## Code Quality

- [ ] CHK-021 Pattern consistency: `isCompletionInvocation` follows `extractDashR`'s style (package-private helper in `main.go`, doc comment explaining purpose, args-slice signature)
- [ ] CHK-022 Pattern consistency: `shouldCompleteRepoForSecondArg` follows existing helpers in `repo_completion.go` (lowercase, `cmd *cobra.Command` first arg, doc comment)
- [ ] CHK-023 No unnecessary duplication: subcommand-collision logic uses `cmd.Commands()` + `IsAvailableCommand()` (mirroring the existing pattern in `completeRepoNames` lines 30-35), not a new hardcoded list
- [ ] CHK-024 Imports added correctly: `repo_completion.go` adds `os/exec` and `path/filepath` only if not already present; no unused imports
- [ ] CHK-025 No new dependencies: `go.mod` is unchanged (verified by inspection)

## Notes

- Check items as you review: `- [x]`
- All items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] CHK-NNN **N/A**: {reason}`
