# Plan: Eager Worktree-Aware Tab Completion

**Change**: 260516-odle-eager-worktree-completion
**Status**: In Progress
**Intake**: `intake.md`
**Spec**: `spec.md`

## Tasks

<!-- Sequential work items for the apply stage. Checked off [x] as completed. -->

### Phase 2: Core Implementation

- [x] T001 Extend `completeRepoNames` in `src/cmd/hop/repo_completion.go` with the pre-slash eager-expansion branch: after the existing subcommand-collision filter, when `rs.MatchOne(toComplete)` returns exactly one repo that survives the filter, call `cloneState(repo.Path)` and (if `stateAlreadyCloned`) `listWorktrees(context.Background(), repo.Path)`; on `len(entries) >= 2` return `[<repo>, <repo>/<wt1>, ...]` with directive `cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace`. Every other path returns today's `names` list with `cobra.ShellCompDirectiveNoFileComp`. Silent fallback on every failure mode — no stderr writes.

### Phase 3: Integration & Edge Cases

- [x] T002 Add five table-driven test cases to `src/cmd/hop/repo_completion_test.go` covering the eager-branch decision cases from the spec: (a) unique match + 1 worktree → bare-only + no `NoSpace`; (b) unique match + >=2 worktrees → eager-fire with `NoSpace`; (c) unique match + uncloned → bare-only fallback, `listWorktrees` not invoked; (d) unique match + `listWorktrees` errors → bare-only fallback, no stderr; (e) ambiguous prefix (2+ matches) → full names list, `listWorktrees` not invoked. Use the existing `withListWorktrees` seam and `makeCompletionFixture` helper.

### Phase 4: Polish

- [x] T003 Run scoped tests (`cd src && go test ./cmd/hop/ -run RepoCompletion -v` then targeted `-run TestCompletionEager`), then the full `./cmd/hop/...` package, then `just build` to confirm the build still succeeds.

## Acceptance

### Functional Completeness

- [x] A-001 Pre-Slash Eager Worktree Expansion: When `MatchOne(toComplete)` returns exactly 1 non-collided repo whose `cloneState` is `stateAlreadyCloned` AND `listWorktrees` returns `>= 2` entries with no error, `completeRepoNames` returns `[<repo>, <repo>/<wt1>, ..., <repo>/<wtN>]` with directive `ShellCompDirectiveNoFileComp | ShellCompDirectiveNoSpace`.
- [x] A-002 Subcommand-Collision Filter Ordering: The subcommand-collision filter runs BEFORE the eager-expansion check. A unique-match repo whose name collides with a cobra subcommand is filtered out, and neither `cloneState` nor `listWorktrees` is invoked for it.
- [x] A-003 Silent Fallback on Every Failure: Every failure mode (uncloned, `cloneState` error, `listWorktrees` error, `listWorktrees` returns `< 2` entries) returns the post-filter `names` list with directive `ShellCompDirectiveNoFileComp` (no `NoSpace`) and writes nothing to stderr.
- [x] A-004 Ambiguous Prefix Bypasses Eager Branch: When `MatchOne(toComplete)` returns 0 or 2+ matches, the function returns the full post-filter `names` list with `ShellCompDirectiveNoFileComp` and does NOT invoke `cloneState` or `listWorktrees`.
- [x] A-005 Post-Slash Branch Preserved: When `toComplete` contains `/`, control transfers to `completeWorktreeCandidates` unchanged; the eager pre-slash branch does not alter its behavior, candidate shape, or directive.
- [x] A-006 Table-Driven Tests for All Five Cases: `repo_completion_test.go` contains the five new test cases (a)–(e) using the existing `listWorktrees` seam for injection.

### Behavioral Correctness

- [x] A-007 Candidate Ordering: On eager-fire, position 0 is the bare `<repo>` name and positions 1..N are `<repo>/<wt>` entries in `wt list --json` source order verbatim — no alphabetical or other reordering.
- [x] A-008 Directive Bitwise OR: On eager-fire the returned directive equals `cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace`; all non-eager branches return exactly `cobra.ShellCompDirectiveNoFileComp`.

### Scenario Coverage

- [x] A-009 Scenario "Unique match with multiple worktrees fires eager expansion" (spec) exercised by test case (b).
- [x] A-010 Scenario "Unique match with only the main worktree does NOT fire eager expansion" exercised by test case (a).
- [x] A-011 Scenario "Unique match collides with a subcommand" exercised — the existing subcommand-collision test continues to pass and a unique match against a colliding name does not invoke `listWorktrees`.
- [x] A-012 Scenario "Unique match, repo not cloned" exercised by test case (c).
- [x] A-013 Scenario "Unique match, `wt list --json` returns an error" exercised by test case (d).
- [x] A-014 Scenario "Ambiguous prefix returns full list unchanged" exercised by test case (e).
- [x] A-015 Scenario "Slash-containing toComplete dispatches to post-slash branch unchanged" — existing post-slash tests continue to pass without modification.

### Edge Cases & Error Handling

- [x] A-016 Empty `toComplete`: `MatchOne("")` returns all repos; `len != 1`; eager branch is skipped naturally with no special-casing.
- [x] A-017 `listWorktrees` non-nil error of any flavor (missing `wt`, malformed JSON, non-zero exit, context timeout) silently falls back — no stderr writes.

### Code Quality

- [x] A-018 Pattern consistency: New code follows the naming, structural, and error-handling patterns of surrounding `completeRepoNames` / `completeWorktreeCandidates` code in `repo_completion.go`.
- [x] A-019 No unnecessary duplication: Reuses `rs.MatchOne`, `cloneState`, and the existing `listWorktrees` package-level seam — no new wrappers, no new packages.
- [x] A-020 Readability over cleverness: The eager branch is a straightforward sequential check (matches → collision → cloneState → listWorktrees → len-guard), with no clever fan-out or premature abstraction.
- [x] A-021 No god functions: `completeRepoNames` stays focused; if the eager branch grows beyond a handful of statements, consider extracting a helper (`expandEagerWorktrees`-style) — but only if extraction genuinely improves readability.

### Security

- [x] A-022 Constitution Principle I (Security): No new subprocess paths are introduced. The eager branch routes through the existing `listWorktrees` seam, which already goes through `proc.RunCapture` with `exec.CommandContext`.

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`

## Deletion Candidates

- None — this change adds new functionality without making existing code redundant. The pre-slash eager branch is purely additive; the post-slash `completeWorktreeCandidates` path is preserved unchanged and continues to serve users who have already typed `/`. No existing helpers, branches, or constants are made obsolete by the new code.
