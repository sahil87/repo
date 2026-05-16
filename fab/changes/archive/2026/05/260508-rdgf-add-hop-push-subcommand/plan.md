# Plan: Add `hop push` subcommand

**Change**: 260508-rdgf-add-hop-push-subcommand
**Status**: In Progress
**Intake**: `intake.md`
**Spec**: `spec.md`

## Tasks

<!-- Sequential work items for the apply stage. Checked off [x] as completed. -->

### Phase 1: Setup

<!-- No new dependencies, build config, or scaffolding required — `push` reuses
     the same imports, helpers, and cobra factory shape as `pull`/`sync`. Phase
     intentionally empty. -->

### Phase 2: Core Implementation

<!-- Primary functionality. Source the new push subcommand, register it in
     root, and update the shell shim alternation. -->

- [x] T001 Create `src/cmd/hop/push.go` modeled directly on `src/cmd/hop/pull.go`. Define `newPushCmd()` (cobra factory: `Use: "push [<name-or-group>] [--all]"`, `Short: "Run 'git push' in a repo, group, or every cloned repo with --all"`, `cobra.MaximumNArgs(1)`, `ValidArgsFunction: completeRepoOrGroupNames`, `--all` bool flag, dispatch via `resolveTargets` → `pushSingle`/`pushBatch`), `pushSingle` (`cloneState`/`stateAlreadyCloned` skip, `pushOne` invocation, `gitMissingHint` on `gitMissing`, `errSilent` on failure), `pushBatch` (`runBatch(cmd, targets, "push", "pushed", pushOne)`), and `pushOne` (10-min `cloneTimeout`, `proc.RunCapture(ctx, r.Path, "git", "push")`, `proc.ErrNotFound` → `gitMissing=true`, success line `push: <name> ✓ <lastNonEmptyLine(out)>` to stderr, failure line `push: <name> ✗ <err>` to stderr). Keep `lastNonEmptyLine` exclusively in `pull.go` (same package — no duplication).
- [x] T002 Register `newPushCmd()` in `src/cmd/hop/root.go::newRootCmd()` `cmd.AddCommand(...)` between `newPullCmd()` and `newSyncCmd()`.
- [x] T003 Update `rootLong` in `src/cmd/hop/root.go`: insert three `hop push` lines into the `Usage:` block between the `hop pull` lines and the `hop sync` lines (`hop push <name>`, `hop push <group>`, `hop push --all` with the same column alignment as the pull/sync lines), and rewrite the `Notes:` sentence `pull and sync accept ...` → `pull, push, and sync accept ...`.
- [x] T004 Update `posixInit` in `src/cmd/hop/shell_init.go`: change the rule-3 alternation `clone|pull|sync|ls|...` → `clone|pull|push|sync|ls|...`. Update neighboring comment(s) referencing the pull/sync alternation precedent so they cite push too.

### Phase 3: Integration & Edge Cases

<!-- Tests covering the spec's scenarios and edge cases — mirror pull_test.go
     test-by-test per spec assumption #15. -->

- [x] T005 Create `src/cmd/hop/push_test.go` modeled on `src/cmd/hop/pull_test.go`. Mirror the existing pull tests test-by-test (renaming `Pull` → `Push`, `pull` → `push`, `pulled` → `pushed`): `TestPushUsageErrorWhenNoArgsAndNoAll`, `TestPushUsageErrorWhenAllAndPositional`, `TestPushSingleNotClonedExitsWithSkipMessage`, `TestPushBatchGroupSkipsNotClonedAndReportsSummary`, `TestPushBatchAllIteratesAllRepos`, `TestPushBatchOutputOrderMatchesYAMLSourceOrder`, `TestPushBatchGroupOnlyIncludesGroupMembers`, `TestPushStdoutIsEmpty`, `TestPushCobraRejectsTwoPositionals`, `TestPushSingleHappyPathAgainstRealGit`. Reuse the package-shared `pullSyncYAMLFixture`, `makeClonedRepoDirs`, `initBareRepoWithCommit`, `runArgs`, `fixtureGroup`, and `writeReposFixture` helpers — do NOT duplicate them.

### Phase 4: Polish

<!-- Memory and spec doc updates per the spec's Memory section. -->

- [x] T006 Update `docs/memory/cli/subcommands.md`: (a) add a `hop push [<name-or-group>] / --all` row to the **Inventory** table between the `hop pull` and `hop sync` rows with the same level of detail as the existing pull row (file `push.go`, args summary, behavior summary, exit codes 0/1/2/130, `gitMissingHint` early-abort); (b) rename the section heading `## hop pull / hop sync per-line output` to `## hop pull / hop push / hop sync per-line output` and extend its body to enumerate push's success/failure forms (`push: <name> ✓ <last-line>`, `push: <name> ✗ <err>`, summary `summary: pushed=N skipped=M failed=K`) without duplicating the shared `skip:` line; (c) update the shim resolution-ladder narrative paragraph that lists the alternation `clone|pull|sync|ls|...` to include `push` between `pull` and `sync`, mirroring the existing pull/sync rationale.
- [x] T007 Update `docs/specs/cli-surface.md`: (a) add a `hop push [<name-or-group>] / --all` row to the **Subcommand Inventory** table between the `hop pull` row and the `hop sync` row; (b) extend the `### Help Text` `Usage:` enumeration to include the three `hop push` lines in their pull-then-push-then-sync position; (c) rewrite any narrative referencing "`pull` and `sync` accept ..." to "`pull`, `push`, and `sync` accept ...".

### Phase 5: Verification

<!-- Run the build and tests to confirm the implementation is correct. -->

- [x] T008 Run `cd src && go build ./...` and confirm the package compiles cleanly.
- [x] T009 Run `cd src && go test ./cmd/hop/... -run TestPush` and confirm every new push test passes.
- [x] T010 Run `cd src && go test ./cmd/hop/...` and confirm no regression in the pull/sync/clone/ls/etc. test suites.

## Acceptance

<!-- Declarative acceptance criteria used by the review stage. -->

### Functional Completeness

- [x] A-001 Subcommand surface: `hop push [<name-or-group>] [--all]` is registered, accepts at most 1 positional via `cobra.MaximumNArgs(1)`, uses `completeRepoOrGroupNames` for tab completion, and dispatches through the existing `resolveTargets` helper.
- [x] A-002 Implementation reuse: `push.go` imports and reuses `runBatch`, `resolveTargets`, `cloneState`/`stateAlreadyCloned`, `cloneTimeout`, `errSilent`, `errFzfMissing`, `fzfMissingHint`, `gitMissingHint`, `lastNonEmptyLine`, and `completeRepoOrGroupNames` — no new abstraction is introduced and `lastNonEmptyLine` is NOT redefined.
- [x] A-003 Output format mirror: per-repo success line is `push: <name> ✓ <last-line>`, failure line is `push: <name> ✗ <err>`, not-cloned skip line is `skip: <name> not cloned`, and batch summary is `summary: pushed=<N> skipped=<M> failed=<K>` — all on stderr.
- [x] A-004 Stdout discipline: stdout is empty for every `hop push` invocation (success, failure, batch, single, usage error).
- [x] A-005 Exit-code mapping mirrors pull: 0 success, 1 single-repo not-cloned / single-repo failure / batch failed>0 / git missing / fzf missing, 2 usage error, 130 fzf cancelled.
- [x] A-006 Subcommand registration: `newPushCmd()` is wired in `root.go::newRootCmd()::AddCommand(...)` between `newPullCmd()` and `newSyncCmd()`.
- [x] A-007 `rootLong` Usage block: contains the three new `hop push` lines positioned between the `hop pull` lines and the `hop sync` lines.
- [x] A-008 `rootLong` Notes block: the sentence reads `pull, push, and sync accept ...` (not `pull and sync accept ...`).
- [x] A-009 Shell shim alternation: `posixInit` includes `push` between `pull` and `sync` in the rule-3 known-subcommand alternation, ensuring `hop push <name>` is dispatched as a subcommand instead of being misrouted into tool-form.
- [x] A-010 Memory inventory row: `docs/memory/cli/subcommands.md` has a `hop push` row between the `hop pull` and `hop sync` rows with the same level of detail (file reference, args summary, behavior, exit codes, gitMissing abort).
- [x] A-011 Memory output-section rename: the section formerly titled `## hop pull / hop sync per-line output` is now `## hop pull / hop push / hop sync per-line output` and enumerates push's success, failure, and summary forms.
- [x] A-012 Memory shim narrative: the resolution-ladder narrative paragraph in `subcommands.md` lists `push` in the alternation alongside `pull` and `sync`, with the rule-5 misroute rationale referenced.
- [x] A-013 Spec inventory update: `docs/specs/cli-surface.md` has a `hop push` row in the **Subcommand Inventory** table between the pull and sync rows.
- [x] A-014 Spec Help Text Usage block: the `Usage:` enumeration in the **Help Text** section lists the three `hop push` lines in their pull-then-push-then-sync position.

### Behavioral Correctness

- [x] A-015 Single-repo success path: `hop push <name>` against a uniquely-resolved cloned repo runs `git push` via `proc.RunCapture` with a 10-minute `cloneTimeout`, prints `push: <name> ✓ <last-line>` to stderr, leaves stdout empty, and exits 0 (verified by `TestPushSingleHappyPathAgainstRealGit`).
- [x] A-016 Single-repo not-cloned path: `hop push <name>` when `<path>/.git` is missing prints `skip: <name> not cloned` to stderr and exits 1 (`errSilent`); `git push` is NOT invoked.
- [x] A-017 Batch group path: `hop push <group>` iterates the group members in YAML source order, emits a `push:` or `skip:` line per repo, and ends with `summary: pushed=N skipped=M failed=K`.
- [x] A-018 `--all` path: `hop push --all` iterates every cloned repo in YAML source order and emits the appropriate per-repo lines plus the summary.
- [x] A-019 Usage errors: `hop push <name> --all` exits 2 with `hop push: --all conflicts with positional <name-or-group>`; `hop push` (no args) exits 2 with `hop push: missing <name-or-group>. Pass a name, a group, or --all.`.
- [x] A-020 `git` missing in batch: emits `gitMissingHint` once on the first `proc.ErrNotFound`, aborts the batch immediately (no further repos attempted, no `summary:` line), and exits 1.

### Scenario Coverage

- [x] A-021 Each spec scenario has a corresponding test (or shares an existing pull/sync test by virtue of `runBatch`/`resolveTargets` reuse): single-repo success, single-repo failure, single-repo not-cloned, group batch, `--all`, usage errors, git missing.

### Edge Cases & Error Handling

- [x] A-022 **N/A**: fzf missing on ambiguous query — covered indirectly by the shared `resolveTargets` helper (verified via the `errFzfMissing` branch in `newPushCmd().RunE` matching pull's exact pattern).
- [x] A-023 **N/A**: fzf cancelled exit 130 — inherited from the shared `resolveTargets` resolver (no push-specific code path).
- [x] A-024 No `--force`/`--set-upstream`/etc.: only the `--all` boolean flag is registered on the cobra command (Constitution III).

### Code Quality

- [x] A-025 Pattern consistency: `push.go` follows the exact structure, naming, and error-handling style of `pull.go` (function order, comment style, sentinel returns, stderr writes via `cmd.ErrOrStderr()`).
- [x] A-026 No unnecessary duplication: `lastNonEmptyLine` is defined only in `pull.go`; `runBatch`, `resolveTargets`, `cloneState`, `cloneTimeout`, `gitMissingHint`, `fzfMissingHint`, `errSilent`, `errFzfMissing` are reused — none redefined.
- [x] A-027 No god functions: every new function in `push.go` is small and focused (`pushOne`, `pushSingle`, `pushBatch`, `newPushCmd`'s `RunE`); no function exceeds the 50-line guidance.
- [x] A-028 No magic strings: status-line prefixes (`push:`, `skip:`, `summary:`), the verb (`"push"`), and the summary label (`"pushed"`) are consistent with the pull/sync convention; no out-of-band literals.

### Security

<!-- Constitution I — process execution security. -->

- [x] A-029 `pushOne` invokes `git` via `proc.RunCapture(ctx, r.Path, "git", "push")` with `context.WithTimeout(context.Background(), cloneTimeout)` — explicit argv slice, no shell string, no `exec.Command` without context (Constitution I).

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`
