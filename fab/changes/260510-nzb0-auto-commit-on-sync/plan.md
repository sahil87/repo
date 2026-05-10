# Plan: hop sync auto-commits dirty trees before pull/push

**Change**: 260510-nzb0-auto-commit-on-sync
**Status**: In Progress
**Intake**: `intake.md`
**Spec**: `spec.md`

## Tasks

<!-- Sequential work items for the apply stage. Checked off [x] as completed. -->

### Phase 1: Setup

<!-- Flag wiring + closure capture so syncOne can see the message string. -->

- [x] T001 Add `-m / --message <msg>` flag to `newSyncCmd` in `src/cmd/hop/sync.go` — register on the `sync` subcommand only (not on root, not on `pull`, not on `push`); type `string`; default `chore: sync via hop`; both long form `--message` and short alias `-m`. The flag value MUST be captured in a closure (or reachable variable) that `syncOne` can read at invocation time.

### Phase 2: Core Implementation

<!-- Helper + plumbing into syncOne. -->

- [x] T002 Add the `commitDirtyTree` helper inline in `src/cmd/hop/sync.go` (per spec implementation hint #13). Signature: `commitDirtyTree(cmd *cobra.Command, r repos.Repo, msg string) (committed bool, gitMissing bool, err error)`. Sequence: (a) `git status --porcelain` via `proc.RunCapture` with its own `context.WithTimeout(context.Background(), cloneTimeout)`; if stdout is empty → return `(false, false, nil)`; (b) `git add --all` via `proc.RunCapture` with its own timeout; on error return `(false, gitMissing, err)` and emit `sync: <name> ✗ commit failed: <err>` (use `lastNonEmptyLine` of stderr if available, else the error string); (c) `git commit -m <msg>` via `proc.RunCaptureBoth` with its own timeout; on error emit `sync: <name> ✗ commit failed: <last-non-empty-line-of-stderr-or-err>` and return `(false, gitMissing, err)`; on success return `(true, false, nil)`. Each git invocation MUST use a fresh `context.WithTimeout`. No `--no-verify`. No shell strings.

- [x] T003 Modify `syncOne` in `src/cmd/hop/sync.go` to: (a) call `commitDirtyTree` first; (b) if it returned `gitMissing`, propagate `(false, true, err)`; (c) if it returned an error (commit failed), propagate `(false, false, err)` — `commitDirtyTree` already wrote the per-repo status line; (d) capture the `committed` bool; (e) run the existing pull/push flow unchanged; (f) on success, emit either `sync: <name> ✓ committed, <pull-summary>, <push-summary>` (when `committed == true`) or the existing `sync: <name> ✓ <pull-summary> <push-summary>` (when `committed == false`). The `commitDirtyTree` call site MUST receive the resolved message string (default or `-m`-overridden) from the closure variable established in T001.

### Phase 3: Integration & Edge Cases

<!-- Doc comment, regression coverage, edge-case scenarios. -->

- [x] T004 Update the `newSyncCmd` doc comment in `src/cmd/hop/sync.go` to reflect the new auto-commit behavior. The current line "No auto-stash, no auto-resolve on rebase conflict, no force-push" applies to the clean-tree path; the updated comment MUST mention auto-commit on dirty trees and clarify that auto-stash, auto-resolve, and force-push remain absent. Also update the `syncOne` doc comment to describe the dirty-detection → add → commit → pull → push sequence.

- [x] T005 [P] Add table-driven tests in `src/cmd/hop/sync_test.go` covering the new behavior. <!-- rework: cycle 1 — cases 7 (dirty + commit success + rebase conflict) and 8 (dirty + commit success + push fail) are missing per review must-fix findings. Add them to satisfy the plan's own enumerated cases and provide regression coverage for spec A9 on the dirty-tree path. --> Follow the existing patterns (`pullSyncYAMLFixture`, `initBareRepoWithCommit`, `fixtureGroup`). Required cases:
  1. Clean tree → no auto-commit, today's `sync: <name> ✓ <pull> <push>` line shape (regression baseline).
  2. Dirty tree → default message `chore: sync via hop`, success line `sync: <name> ✓ committed, <pull>, <push>`, exit 0, post-sync `git status --porcelain` empty.
  3. Dirty tree with untracked file → both files in the resulting commit.
  4. Dirty tree + `-m "fix(zsh): reload"` → commit message exactly `fix(zsh): reload`; default not present in `git log`.
  5. Clean tree + `-m "would-be"` → no commit produced; line is the clean-tree shape (no `committed,` token).
  6. Dirty tree + pre-commit hook rejection → `sync: <name> ✗ commit failed: <err>` on stderr; pull/push NOT invoked; exit non-zero.
  7. Dirty tree + commit success + rebase conflict → existing `sync: <name> ✗ rebase conflict — resolve manually with: git -C <path> rebase --continue` line emitted unchanged.
  8. Dirty tree + commit success + push fail → existing `sync: <name> ✗ push failed: <err>` line emitted unchanged; local commit remains in history.
  9. Batch `--all` mixed: dirty (success) + clean (success) + dirty (commit fail) → per-repo lines in YAML order, summary `synced=2 skipped=0 failed=1`.
  10. `hop push -m "x"` → cobra rejects `-m` with "unknown flag" error (verifies flag is sync-only).
  11. `hop pull -m "x"` → cobra rejects `-m` with "unknown flag" error (verifies flag is sync-only).
  12. Dirty tree + `-m ""` → empty string passed to git, git's own validation surfaces as commit failure; per-repo line follows `commit failed:` shape.
  13. Dirty tree + multi-line `-m` value → message containing `\n` is passed verbatim as a single argv element; resulting commit has subject + body separated by blank line.

### Phase 4: Verification

<!-- Build + vet + full test pass. -->

- [x] T006 Run `go vet ./...`, `go build ./...`, and `go test ./...` from `src/`. All must pass cleanly. If a test fails: fix per Test Integrity (tests conform to spec; do not bend the implementation). On three consecutive failed retries on the same test, escalate.

## Execution Order

- T001 → T002 → T003 (closure variable from T001 is consumed by T003; T002 is a helper called by T003).
- T004 can run any time after T003 (purely doc edits).
- T005 [P] can author tests in parallel with T002–T004 but MUST run last for verification (depends on the implementation existing).
- T006 runs last.

## Acceptance

<!-- Declarative acceptance criteria used by the review stage. Mirrors spec acceptance criteria A1–A16. -->

### Functional Completeness

- [ ] A-001 `hop sync <name>` on a dirty repo invokes `git status --porcelain` → `git add --all` → `git commit -m "chore: sync via hop"` → `git pull --rebase` → `git push`, in that exact order (spec A1).
- [ ] A-002 `hop sync <name>` on a clean repo invokes `git status --porcelain` → `git pull --rebase` → `git push`, skipping `git add` and `git commit`; behavior is identical to the pre-change baseline (spec A2).
- [ ] A-003 `hop sync <name> -m "<msg>"` on a dirty repo uses `<msg>` verbatim as the commit message; the default `chore: sync via hop` is not used when `-m` is present (spec A3).
- [ ] A-004 `hop sync <name> -m "<msg>"` on a clean repo has no observable effect; no commit is produced; pull/push run as today (spec A4).
- [ ] A-005 The `-m / --message` flag is registered on `sync` only — `hop pull -m "x"` and `hop push -m "x"` reject the unknown flag with cobra's standard error (spec A5).

### Behavioral Correctness

- [ ] A-006 When the auto-commit step fails (non-zero exit from `git add` or `git commit`, including `pre-commit` hook rejection), `git pull --rebase` and `git push` MUST NOT run for that repo, and stderr contains `sync: <name> ✗ commit failed: <err>` (spec A6).
- [ ] A-007 `hop sync <name>` MUST NOT pass `--no-verify` to `git commit` — verified by source audit (`grep --no-verify src/cmd/hop/sync.go` returns nothing) and by a hook-rejection test that asserts the documented failure path (spec A7).
- [ ] A-008 Per-repo success line on a dirty-tree sync matches `sync: <name> ✓ committed, <pull-summary>, <push-summary>`. Per-repo success line on a clean-tree sync matches today's `sync: <name> ✓ <pull-summary> <push-summary>` (no `committed,` token, no commas between summaries) (spec A8).

### Removal Verification

- [ ] A-009 The existing `sync: <name> ✗ rebase conflict — resolve manually with: git -C <path> rebase --continue` line and the existing `sync: <name> ✗ push failed: <err>` line are emitted exactly as today; this change MUST NOT modify their wording or format (spec A9).

### Scenario Coverage

- [ ] A-010 Batch summary line `summary: synced=N skipped=M failed=K` continues to count auto-committed-then-pushed repos as `synced` and commit-failed repos as `failed`; the mixed clean+dirty+failed batch test in `sync_test.go` asserts the line `summary: synced=2 skipped=0 failed=1` (spec A10).

### Edge Cases & Error Handling

- [ ] A-011 Each git invocation in the sync flow runs under its own `context.WithTimeout(context.Background(), cloneTimeout)`; no shared parent context caps the per-repo step sequence — verified by source audit showing one `WithTimeout` per git call (spec A11).

### Security

- [ ] A-012 Every new git invocation in `sync.go` routes through `internal/proc.RunCapture` or `proc.RunCaptureBoth` — `grep --include='*.go' --exclude='*_test.go' -rn '"os/exec"' src/cmd/hop/sync.go` returns zero matches; no shell strings; explicit argument slices only (spec A12, Constitution I).

### Removal Verification

- [ ] A-013 `src/cmd/hop/push.go` is unchanged by this change — verified by `git diff main -- src/cmd/hop/push.go` showing no edits (spec A13).

### Code Quality

- [ ] A-014 `src/cmd/hop/sync.go`'s `newSyncCmd` doc comment is updated to mention auto-commit on dirty trees while preserving the "no auto-stash, no auto-resolve, no force-push" clarification for the clean-tree path (spec A14).
- [x] A-015 The `hop sync` row in `docs/memory/cli/subcommands.md`'s Inventory table reflects the new auto-commit step and the `-m / --message` flag; the "`hop pull` / `hop push` / `hop sync` per-line output" section documents the `committed,` prefix on success and the `sync: <name> ✗ commit failed: <err>` line on failure (spec A15). [Note: hydrated during the hydrate stage; this acceptance item exists to confirm the cross-reference is captured in the plan and surfaces during review.]
- [ ] A-016 The change builds and tests pass cleanly: `go vet ./...`, `go build ./...`, and `go test ./...` from `src/` succeed. Cross-platform CI matrix (darwin-arm64, darwin-amd64, linux-arm64, linux-amd64) is unchanged in scope by this PR (no platform-specific code introduced) (spec A16).

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Auto-commit is the default behavior for `hop sync` on dirty trees, not opt-in | Confirmed from spec #1 (carry-forward) — Constitution Principle III supports convention over flag | S:95 R:55 A:90 D:90 |
| 2 | Certain | `-m / --message` flag overrides the default commit message but does NOT toggle auto-commit on/off | Confirmed from spec #2 (carry-forward) | S:95 R:80 A:95 D:95 |
| 3 | Certain | Default commit message is `chore: sync via hop` | Confirmed from spec #3 (carry-forward) — fixed string, not configurable | S:100 R:90 A:95 D:95 |
| 4 | Certain | Stage scope is `git add --all` (includes untracked files), matching xpush | Confirmed from spec #4 (carry-forward) | S:95 R:60 A:90 D:90 |
| 5 | Certain | Clean tree (empty `git status --porcelain`) skips commit and behaves identical to today's `hop sync` | Confirmed from spec #5 (carry-forward) | S:95 R:90 A:95 D:95 |
| 6 | Certain | `hop push` is NOT changed; remains a pure `git push` wrapper | Confirmed from spec #6 (carry-forward) | S:100 R:75 A:95 D:95 |
| 7 | Certain | Pre-commit hooks are respected; no `--no-verify`. Hook failure aborts the repo's sync | Confirmed from spec #7 (carry-forward) | S:95 R:75 A:95 D:90 |
| 8 | Certain | Dirty detection uses `git status --porcelain` (matches xpush) | Confirmed from spec #8 (carry-forward) | S:100 R:90 A:95 D:95 |
| 9 | Certain | Order of operations: `git status --porcelain` → `git add --all` → `git commit -m <msg>` → `git pull --rebase` → `git push`; any failure aborts that repo's sync | Confirmed from spec #9 (carry-forward) | S:95 R:75 A:95 D:95 |
| 10 | Certain | Constitution Principle IV (Wrap, Don't Reinvent) is satisfied — composing multiple git invocations is extension, not reinvention | Confirmed from spec #10 (carry-forward) | S:95 R:90 A:95 D:90 |
| 11 | Certain | Status output for dirty-tree sync mentions the auto-commit (`committed,` token in success line) | Confirmed from spec #11 (carry-forward) | S:95 R:95 A:85 D:75 |
| 12 | Certain | New flag is `-m`/`--message` (string, default `chore: sync via hop`) — matches `git commit -m` muscle memory | Confirmed from spec #12 (carry-forward) | S:95 R:90 A:90 D:85 |
| 13 | Certain | The auto-commit helper goes inline in `sync.go` (not extracted to a shared file) | Confirmed from spec #13 (carry-forward) — non-binding implementation hint, applied here | S:95 R:90 A:80 D:75 |
| 14 | Certain | Each git invocation gets its own 10-minute timeout via `proc.RunCapture` / `cloneTimeout` | Confirmed from spec #14 (carry-forward) | S:95 R:90 A:95 D:90 |
| 15 | Certain | Failure message format follows existing pattern: `sync: <name> ✗ commit failed: <err>` (mirrors `push failed: <err>`) | Confirmed from spec #15 (carry-forward) | S:95 R:95 A:90 D:85 |
| 16 | Certain | The `sync.go` package-level doc comment is updated to reflect the new auto-commit behavior | Confirmed from spec #16 (carry-forward) | S:95 R:95 A:95 D:90 |
| 17 | Certain | Status-line wording uses comma-separation: `sync: <name> ✓ committed, <pull-summary>, <push-summary>` | Confirmed from spec #17 (carry-forward) | S:95 R:95 A:75 D:55 |
| 18 | Certain | `git status --porcelain` runs as a separate proc invocation so its stdout can be inspected before deciding on commit | Confirmed from spec #18 (carry-forward) | S:90 R:90 A:95 D:90 |
| 19 | Certain | The auto-commit branch lives in the per-repo `syncOne` flow only — no changes to `syncSingle`, `syncBatch`, or `runBatch` | Confirmed from spec #19 (carry-forward) | S:95 R:95 A:95 D:95 |
| 20 | Certain | Empty `-m ""` falls through to git's own validation; multi-line `-m` values are passed verbatim as a single argv element | Confirmed from spec #20 (carry-forward) — argv-slice passing is the proc.RunCapture contract | S:90 R:90 A:90 D:80 |
| 21 | Certain | Successful auto-commit followed by a failed push leaves the local commit in place — no automatic rollback | Confirmed from spec #21 (carry-forward) | S:90 R:60 A:85 D:85 |
| 22 | Certain | The `-m / --message` flag is registered on `sync` only; `pull` and `push` reject it via cobra's unknown-flag handling | Confirmed from spec #22 (carry-forward) | S:95 R:95 A:95 D:95 |

22 assumptions (22 certain, 0 confident, 0 tentative, 0 unresolved).

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`
