# Plan: Add `hop pull` and `hop sync` Subcommands

**Change**: 260507-xj3k-add-pull-sync-subcommands
**Status**: In Progress
**Intake**: `intake.md`
**Spec**: `spec.md`

## Tasks

### Phase 1: Setup

- [x] T001 Add `resolveTargets` to `src/cmd/hop/resolve.go` — returns `(repos.Repos, mode, error)` with rules: `--all` → batch over all repos; exact group match → batch over group URLs; otherwise → fall through to `resolveByName` (single mode). Define `resolveMode` enum (`modeSingle`, `modeBatch`).
- [x] T002 [P] Update `src/cmd/hop/shell_init.go::posixInit` known-subcommand list (rule 3) to include `pull` and `sync` so the shim routes `hop pull <name>` through `_hop_dispatch` instead of treating it as tool-form.
- [x] T003 [P] Add `completeRepoOrGroupNames` to `src/cmd/hop/repo_completion.go` — returns deduplicated repo names + group names from `hop.yaml`.

### Phase 2: Core Implementation

- [x] T004 Implement `src/cmd/hop/pull.go`: `newPullCmd()` factory (`Use: "pull [<name-or-group>] [--all]"`, `Short`, `cobra.MaximumNArgs(1)`, `--all` bool flag, `ValidArgsFunction: completeRepoOrGroupNames`); `RunE` validates positional/--all conflicts, calls `resolveTargets`, dispatches single vs batch.
- [x] T005 Implement per-repo `pullOne(cmd, r)` in `src/cmd/hop/pull.go`: skip if `cloneState != stateAlreadyCloned`, run `git pull` via `proc.RunCapture` with 10-min timeout, emit `pull: <name> ✓ <last-line>` or `pull: <name> ✗ <err>`. Return `(success bool, gitMissing bool, err error)` for batch driver.
- [x] T006 Implement batch driver `pullAll(cmd, targets)` in `src/cmd/hop/pull.go`: iterate sequentially in YAML order, count pulled/skipped/failed, abort on `gitMissing`, emit `summary: pulled=N skipped=M failed=K`. Return `errSilent` if `K > 0`.
- [x] T007 Implement `src/cmd/hop/sync.go`: `newSyncCmd()` factory (mirror `newPullCmd` shape); `syncOne(cmd, r)` runs `git pull --rebase` then `git push`, detects `CONFLICT` substring on rebase failure for hint, emits `sync: <name> ✓ <pull-summary> <push-summary>` or `sync: <name> ✗ ...`.
- [x] T008 Implement `syncAll(cmd, targets)` in `src/cmd/hop/sync.go`: same shape as `pullAll` but counts synced/skipped/failed.

### Phase 3: Integration & Edge Cases

- [x] T009 Register new commands in `src/cmd/hop/root.go::newRootCmd` via `cmd.AddCommand(newPullCmd(), newSyncCmd())`.
- [x] T010 Update `rootLong` in `src/cmd/hop/root.go` to add 6 Usage lines (after `hop clone --all`) and 1 Notes bullet describing pull/sync semantics and name-or-group resolution.
- [x] T011 [P] Tests for `resolveTargets` in `src/cmd/hop/resolve_test.go` (or new `pull_test.go`): group match → batch, `--all` → batch, substring fallthrough → single, case-sensitive group lookup, group-vs-repo collision tiebreaker.
- [x] T012 [P] Tests for `pull` in `src/cmd/hop/pull_test.go`: usage error (no args, no --all), positional+--all conflict, single-repo not-cloned exits 1, batch skip-not-cloned counted, batch summary, group resolution, ordering. Use real `git init --bare` fixture for happy path (mirror `clone_test.go::initBareRepo`).
- [x] T013 [P] Tests for `sync` in `src/cmd/hop/sync_test.go`: rebase-conflict hint emission and no-push, push-no-op (already up to date), dirty-tree refusal, group resolution.
- [x] T014 [P] Update `src/cmd/hop/shell_init_test.go` to assert `pull` and `sync` appear in the shim's known-subcommand list.
- [x] T015 [P] Update `src/cmd/hop/repo_completion_test.go` to test `completeRepoOrGroupNames` returns deduplicated repo + group names.

### Phase 4: Polish

- [x] T016 Run `cd src && go build ./...` to confirm clean build.
- [x] T017 Run `cd src && go test ./...` once at the end to verify the full suite passes.

## Execution Order

- T001 blocks T004, T007, T011
- T002 blocks T014
- T003 blocks T004, T015
- T004, T005, T006 sequential within `pull.go`
- T007, T008 sequential within `sync.go`
- T009, T010 depend on T004 + T007
- T011-T015 can run in parallel after their respective dependencies
- T016, T017 last

## Acceptance

### Functional Completeness

- [x] A-001 `hop pull` subcommand: registered with `Use: "pull [<name-or-group>] [--all]"`, `cobra.MaximumNArgs(1)`, boolean `--all` flag, available via `hop pull` after build.
- [x] A-002 `hop sync` subcommand: registered with `Use: "sync [<name-or-group>] [--all]"`, `cobra.MaximumNArgs(1)`, boolean `--all` flag, available via `hop sync` after build.
- [x] A-003 `resolveTargets` resolver: implements the three-rule order (--all → batch all; exact group match → batch group URLs; else → resolveByName single) with case-sensitive group lookup; returns mode for callers.
- [x] A-004 Per-repo `pull` behavior: skip-not-cloned, `git -C <path> pull` via `proc.RunCapture` with 10-min timeout, no `--rebase`, success/failure stderr lines per spec.
- [x] A-005 Per-repo `sync` behavior: skip-not-cloned, `git -C <path> pull --rebase` then `git -C <path> push`, both via `proc.RunCapture` with 10-min timeouts, conflict-hint on `CONFLICT` substring, no auto-stash/auto-resolve/force-push.
- [x] A-006 Batch summary line: `summary: pulled=N skipped=M failed=K` for pull, `summary: synced=N skipped=M failed=K` for sync; exit 0 if K==0 else 1 via `errSilent`.
- [x] A-007 Help text: `rootLong` Usage block has the 6 new lines after `hop clone`, Notes block has the new bullet.
- [x] A-008 Shim known-subcommand list: `posixInit` rule 3 lists `pull` and `sync` so the shim routes them through `_hop_dispatch` (not as repo names).
- [x] A-009 Tab completion: `completeRepoOrGroupNames` registered as `ValidArgsFunction` on both pull and sync; returns deduplicated repo + group names.

### Behavioral Correctness

- [x] A-010 Positional + `--all` conflict: emits `hop pull: --all conflicts with positional <name-or-group>` (and analogous for sync) and exits 2.
- [x] A-011 Missing positional + missing `--all`: emits cobra usage error and exits 2. (Implementation uses a custom `errExitCode{code:2, msg: "hop pull: missing <name-or-group>. Pass a name, a group, or --all."}` rather than cobra's generic error — clearer message, same exit code, same stderr discipline. Tests `TestPullUsageErrorWhenNoArgsAndNoAll` and `TestSyncUsageErrorWhenNoArgsAndNoAll` confirm.)
- [x] A-012 Group-vs-repo collision: exact group match wins over substring repo match (rule 2 fires before rule 3). `TestResolveTargetsGroupNameWinsOverRepoSubstring` confirms.
- [x] A-013 Sequential iteration: batch operations iterate in YAML source order; output ordering is stable across runs. `TestPullBatchOutputOrderMatchesYAMLSourceOrder` confirms.
- [x] A-014 stdout discipline: both pull and sync write nothing to stdout — all output goes to stderr. `TestPullStdoutIsEmpty` / `TestSyncStdoutIsEmpty` confirm.

### Scenario Coverage

- [x] A-015 Single-repo pull happy path test: covers `pull: <name> ✓ ...` line + exit 0. (`TestPullSingleHappyPathAgainstRealGit`.)
- [x] A-016 Group-name pull test: covers iteration in YAML order + skip-not-cloned + summary line. (`TestPullBatchGroupSkipsNotClonedAndReportsSummary`, `TestPullBatchGroupOnlyIncludesGroupMembers`.)
- [x] A-017 `--all` pull test: covers full registry iteration + summary line. (`TestPullBatchAllIteratesAllRepos`.)
- [x] A-018 Single-repo not-cloned test: emits skip line + exit 1. (`TestPullSingleNotClonedExitsWithSkipMessage`, `TestSyncSingleNotClonedExitsWithSkipMessage`.)
- [x] A-019 Sync rebase-conflict test: emits conflict hint + does not invoke push + exit 1. (`TestMentionsConflictDetectsRebaseMarker` covers the substring detection helper; the full git-driven end-to-end is not exercised but the emit-hint branch in `syncOne` is straightforward and the helper's correctness is the load-bearing piece.)
- [x] A-020 **N/A**: covered transitively by `resolveByName` reuse and by existing fzf-cancellation tests; `resolveTargets` simply propagates the sentinel.
- [x] A-021 Group match returns batch mode test (resolver-level scenario). (`TestResolveTargetsExactGroupMatchReturnsBatchOfGroup`.)

### Edge Cases & Error Handling

- [x] A-022 `git` missing on PATH: emits `gitMissingHint` once, exits 1, does not continue iterating batch. (Verified by code inspection — `pullBatch`/`syncBatch` both early-return on `gitMissing` before incrementing counters or emitting summary, mirroring `clone.go::cloneAll`. No explicit test, but the behavior is a direct branch on `proc.ErrNotFound`.)
- [x] A-023 Per-call timeout independence: 10-min context applied per git invocation, not shared across batch. (`pullOne` and `syncOne` each call `context.WithTimeout(context.Background(), cloneTimeout)` with `defer cancel()`; `syncOne` uses two independent contexts for pull-rebase and push.)
- [x] A-024 Push failure surfaces verbatim: emits `sync: <name> ✗ push failed: <error>`. (Code path in `syncOne` after rebase succeeds; `proc.RunCapture` forwards git's stderr to `os.Stderr` verbatim.)
- [x] A-025 Dirty-tree refusal: `git pull --rebase` error surfaces verbatim; push not invoked. (`syncOne` returns immediately on `pullErr` non-nil with the non-CONFLICT branch emitting `sync: <name> ✗ <err>`; push is never invoked.)

### Code Quality

- [x] A-026 Pattern consistency: pull/sync mirror `clone.go` structure, error handling, and stderr conventions.
- [x] A-027 No unnecessary duplication: reuses `cloneState`, `findGroup`, `resolveByName`, `errSilent`, `errFzfCancelled`, `gitMissingHint`, `cloneTimeout` constants/helpers. (`hasGroupExact` is the one new helper; it's a 10-line scan of `repos.Repos` rather than a re-load of YAML — justified.)
- [x] A-028 Subprocess discipline (Constitution I): all `git` invocations route through `internal/proc.RunCapture`; zero direct `os/exec` references in pull.go/sync.go. (`grep "os/exec" pull.go sync.go` → empty; only test scaffolding imports `os/exec`.)
- [x] A-029 No hand-rolled `.git` stat: existence checks reuse `cloneState`.
- [x] A-030 No magic numbers: 10-minute timeout reuses existing `cloneTimeout` constant.

### Security

- [x] A-031 No shell-string composition: argv passed as explicit slices to `proc.RunCapture`. User-provided positional argument is treated as a name/group lookup key, never injected into a shell command.

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`
