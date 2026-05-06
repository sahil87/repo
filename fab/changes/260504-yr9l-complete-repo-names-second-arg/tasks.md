# Tasks: Complete repo names in $2 of two-arg forms

**Change**: 260504-yr9l-complete-repo-names-second-arg
**Spec**: `spec.md`
**Intake**: `intake.md`

## Phase 1: Setup

<!-- No setup needed: no new dependencies, no new files in core code. Tests live in existing/new test files alongside source. -->

## Phase 2: Core Implementation

- [x] T001 Add `isCompletionInvocation(args []string) bool` helper to `src/cmd/hop/main.go`. Returns `true` iff `len(args) >= 2 && args[1] in {"__complete", "__completeNoDesc"}`. Place near `extractDashR` with a doc comment explaining its single purpose (suppress `-R` interception during cobra completion).
- [x] T002 Wire the helper into `src/cmd/hop/main.go::main`: gate the existing `extractDashR(os.Args)` block with `if !isCompletionInvocation(os.Args) { ... }`. Update the inline comment above the gate to explain why completion entrypoints must reach `rootCmd.Execute()`.
- [x] T003 [P] Add `shouldCompleteRepoForSecondArg(cmd *cobra.Command, args []string) bool` helper to `src/cmd/hop/repo_completion.go`. Returns `true` iff `len(args) == 1` AND `cmd.Parent() == nil` (root only — subcommands have their own arity) AND `args[0]` is not a known available subcommand of `cmd` AND `exec.LookPath(args[0])` returns an absolute path. Document the mirror to `shell_init.go` shim rules 4 and 6. Note: `-R` completion is wired separately via `cmd.RegisterFlagCompletionFunc` against a hidden persistent `-R` cobra flag in `root.go::newRootCmd` — cobra consumes `-R <value>` before `ValidArgsFunction` runs, so an `args[0] == "-R"` branch is unreachable.
- [x] T004 Update `completeRepoNames` in `src/cmd/hop/repo_completion.go`: replace the early bail `if len(args) > 0 { return nil, ... }` with `if len(args) > 0 && !shouldCompleteRepoForSecondArg(cmd, args) { return nil, ... }`. Existing logic below (loadRepos, subcommand-collision filter, name accumulation) is unchanged. Add `os/exec` and `path/filepath` imports as needed.

## Phase 3: Integration & Edge Cases

- [x] T005 [P] Add `src/cmd/hop/main_test.go` with unit tests for `isCompletionInvocation`: returns `true` for `["hop", "__complete", "-R", "", ""]` and `["hop", "__completeNoDesc", "where", ""]`; returns `false` for `["hop", "-R", "name", "ls"]`, `["hop"]`, and `[]` (defensive). Package-private, follows `dashr_test.go` conventions.
- [x] T006 [P] Extend `src/cmd/hop/repo_completion_test.go` with unit tests for `shouldCompleteRepoForSecondArg` calling the helper directly with `newRootCmd()` as `cmd` (covers tool-form only — `-R` is wired via flag-completion and tested end-to-end in T007):
  - `args = []` → `false` (length != 1)
  - `args = ["sh"]` → `true` (assumes `sh` is on PATH; skip with `t.Skip` if `exec.LookPath("sh")` fails)
  - `args = ["hop-nonexistent-tool-xyzzy"]` → `false` (defensive guard against accidental PATH presence: skip with `t.Skip` if it resolves)
  - `args = ["clone"]` → `false` (subcommand wins; verifies subcommand check uses cobra introspection)
  - `args = ["sh", "name"]` → `false` (length != 1)
- [x] T007 [P] Extend `src/cmd/hop/repo_completion_test.go` with end-to-end completion tests using `cobra.ShellCompRequestCmd` (matches existing pattern at lines 22-35):
  - `runArgs(t, cobra.ShellCompRequestCmd, "-R", "")` → stdout contains all repo names from fixture
  - `runArgs(t, cobra.ShellCompRequestCmd, "sh", "")` → stdout contains all repo names from fixture (skip if `sh` not on PATH)
  - `runArgs(t, cobra.ShellCompRequestCmd, "hop-nonexistent-tool-xyzzy", "")` → no candidates (use `candidatesFrom` helper at line 113)
  - `runArgs(t, cobra.ShellCompRequestCmd, "-R", "name", "")` → no candidates (third position)
  - `runArgs(t, cobra.ShellCompRequestCmd, "sh", "name", "")` → no candidates (third position; skip if `sh` not on PATH)
- [x] T008 Run `cd src && go build ./... && go vet ./... && go test ./cmd/hop/...` from the repo root. Resolve any failures. The existing `TestCompletionListsRepoNames`, `TestCompletionRootFiltersSubcommandCollisions`, and `TestCompletionForSubcommands` MUST still pass — they regression-guard the `args == []` path.

## Phase 4: Polish

<!-- Memory updates are deferred to /fab-continue (hydrate). Spec doc updates to docs/specs/cli-surface.md are out of scope for this stage — specs are human-curated per docs/specs/index.md. -->

---

## Execution Order

- T001 → T002 (T002 calls the helper added in T001)
- T003 → T004 (T004 calls the helper added in T003)
- T001/T002 and T003/T004 can run independently (different files): the [P] marker on T003 reflects this independence from T001-T002
- T005, T006, T007 are mutually independent (different test files / different test functions) and can run after their respective helpers exist (T005 after T001; T006 after T003; T007 after T002+T004)
- T008 runs last (whole-package build + vet + test gate)
