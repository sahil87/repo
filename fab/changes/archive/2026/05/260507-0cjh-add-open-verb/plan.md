# Plan: Add `hop <name> open` verb

**Change**: 260507-0cjh-add-open-verb
**Status**: In Progress
**Intake**: `intake.md`
**Spec**: `spec.md`

## Tasks

### Phase 1: Setup

- [x] T001 Create `src/cmd/hop/open.go` with the `runOpen(cmd *cobra.Command, name string) error` function skeleton, the `openHereNoShimHint` constant, and necessary imports (`fmt`, `os`, `context`, `errors`, `github.com/sahil87/hop/internal/proc`, `github.com/spf13/cobra`).

### Phase 2: Core Implementation

- [x] T002 Implement `runOpen` body in `src/cmd/hop/open.go`: resolve repo via `resolveOne(cmd, name)`; create temp file via `os.CreateTemp("", "hop-open-cd-*")` and `defer os.Remove`; build env (parent env + `WT_CD_FILE=<temp>` + `WT_WRAPPER=1`); call `proc.RunForeground` with `dir=repo.Path`, `name="wt"`, `args=["open"]`, env from above; on `proc.ErrNotFound` return `errSilent` (or let translateExit handle naturally — match `-R`'s pattern); on non-zero exit propagate via `errExitCode{code: <wt-exit>}`; read temp file; if non-empty: emit to `cmd.OutOrStdout()` AND if `os.Getenv("HOP_WRAPPER") != "1"` also write `openHereNoShimHint` to `cmd.ErrOrStderr()`; return nil.

- [x] T003 Wire the `case "open"` arm into `src/cmd/hop/root.go` in `newRootCmd`'s `RunE` 2-arg switch (between `case "cd"` and the `default` arm).

- [x] T004 Add `"open"` to the verb completion list in `src/cmd/hop/repo_completion.go` (locate the existing verb-position completion logic; add alongside `where`/`cd`).

- [x] T005 [P] Update `posixInit` in `src/cmd/hop/shell_init.go`: (a) add `export HOP_WRAPPER=1` near the top; (b) extend rule-5 dispatch in `hop()` with a `$2 == "open"` arm routing to `_hop_dispatch open "$1"`; (c) add `open)` arm in `_hop_dispatch` mirroring the `cd)` arm shape (capture stdout, conditional `cd --` if non-empty).

- [x] T006 [P] Update `rootLong` Usage table in `src/cmd/hop/root.go` to include the new `hop <name> open` line; update the Notes section to mention `open` as shell-integration-dependent (alongside `cd`).

### Phase 3: Integration & Edge Cases

- [x] T007 Update `internal/proc` if needed so `RunForeground` accepts a custom env (read existing API: `RunForeground(ctx, dir, name, args...)`). The signature currently doesn't take env — verify whether the function passes parent env or accepts one. If it doesn't, extend it minimally to accept env (or use a sibling helper). Keep API surface tight.

- [x] T008 Write `src/cmd/hop/open_test.go` covering: (a) "Open here" round-trip (fake `wt` script writing to `WT_CD_FILE`; assert hop's stdout contains the path); (b) editor case (fake `wt` writes nothing; assert empty stdout, exit 0); (c) wt non-zero exit propagation; (d) missing-wt path (fake `wt` removed from PATH; assert exit 1, no special hint); (e) no-shim hint (HOP_WRAPPER unset, "Open here" path; assert stderr contains `openHereNoShimHint` and stdout has the path); (f) shim present (HOP_WRAPPER=1, "Open here" path; assert stderr does NOT contain the hint).

- [x] T009 [P] Update `src/cmd/hop/shell_init_test.go` golden expectations to include the new dispatch arms and `HOP_WRAPPER=1` export. Run the tests and update goldens via the project's existing pattern (likely a `-update` flag or fixture-regeneration script — check existing test code).

- [x] T010 [P] Update `.github/formula-template.rb` to include `depends_on "sahil87/tap/wt"` after the `license "MIT"` line. (Already done on this branch — verify the diff is clean.)

- [x] T010a [P] Add unit tests for `RunForegroundEnv` in `src/internal/proc/proc_test.go` (which now exists on main). Cover: (a) env=nil behaves identically to RunForeground (parent env inherited); (b) env=non-nil overrides cleanly (subprocess sees exactly the supplied env); (c) `proc.ErrNotFound` returns when binary missing.

- [x] T010b [P] Add unit test for verb-position completion in `src/cmd/hop/repo_completion_test.go` (which now exists on main). Assert `completeRepoNames(cmd, []string{"outbox"}, "")` returns `["cd", "where", "open"]` with `ShellCompDirectiveNoFileComp`.

### Phase 4: Polish

- [x] T011 Run `go test ./src/...` from the repo root and ensure all tests pass. Run `go vet ./src/...`. Run `gofmt -l src/` and ensure no formatting drift.

- [ ] T012 Manually exercise the verb end-to-end: `just build && just install`, then `hop <some-repo> open` → menu → pick "Open here" → confirm parent shell cds. Repeat with VSCode/Cursor → confirm app launches and parent shell does NOT cd. (User-driven validation; document any deviations.)

## Execution Order

- T001 blocks T002 (T002 fills in the skeleton from T001).
- T002 blocks T003 (T003 calls `runOpen` which T002 implements).
- T002, T003 block T008 (tests need the implementation to exist).
- T005, T006, T010 are independent of code-path tasks and can run in parallel (`[P]`).
- T007 may be a no-op if `proc.RunForeground` already supports env passing — check first.
- T011 runs after all `[P]` and code tasks complete.
- T012 is final manual validation.

## Acceptance

### Functional Completeness

- [x] A-001 Verb recognition at args[1]: `hop <name> open` invokes `runOpen` (binary-direct AND through-shim); `args[1] != "open"` paths (where, cd, tool-form) are unchanged.
- [x] A-002 Resolved repo path becomes wt's cwd via `cmd.Dir`; verified by a test asserting `wt`'s `$PWD` (captured in fake script) matches the resolved path.
- [x] A-003 `WT_CD_FILE` is set to a unique temp path per invocation; the file is removed via `defer os.Remove` (confirmed by test assertion that the file does not exist post-invocation).
- [x] A-004 `WT_WRAPPER=1` is in the env passed to wt (test assertion: fake wt records `os.Getenv("WT_WRAPPER")` and the test reads it back).
- [x] A-005 Stdout emission is conditional on temp file having non-empty contents; empty contents → no stdout (test scenarios for both branches).
- [x] A-006 Exit code propagation: wt exit 0 → hop exit 0; wt exit N → hop exit N (tested for at least one non-zero value).
- [x] A-007 No-shim hint emitted when `HOP_WRAPPER` is unset AND temp file is non-empty; suppressed when `HOP_WRAPPER=1`.
- [x] A-008 Tab completion for `hop <name> <TAB>` includes `open` alongside `cd`/`where`.
- [x] A-009 `rootLong` Usage table includes `hop <name> open` line; Notes section mentions `open` alongside `cd` for shell-integration dependency. (Hydrate-stage follow-up: `docs/specs/cli-surface.md` inventory table AND the Usage-block enumeration sentence both need `hop <name> open` added.)
- [x] A-010 Shim's `posixInit` rule 5 has an `$2 == "open"` arm; `_hop_dispatch` has an `open)` arm; shim exports `HOP_WRAPPER=1` (verified via golden file diff).
- [x] A-011 `.github/formula-template.rb` contains `depends_on "sahil87/tap/wt"` between `license "MIT"` and `on_macos do`.
- [x] A-012 No homebrew-tap commit appears in this change's git history (verified by inspection — the worktree only commits to the hop repo).

### Behavioral Correctness

- [x] A-013 Existing Darwin tool-form `hop <name> open` no longer reaches `/usr/bin/open` — the new verb intercepts at $2 dispatch. (Behavior change documented in spec; verified by tracing through the shim's dispatch logic.)
- [x] A-014 Existing verbs (`where`, `cd`) and tool-form for non-`open` tools (`code`, `cursor`, etc.) work exactly as before — no regression.
- [x] A-015 Bare-name dispatch (`hop <name>` → cd) is unchanged.

### Code Quality / Constitution

- [x] A-016 No `os/exec` import in `src/cmd/hop/open.go` (audit: `grep '"os/exec"' src/cmd/hop/open.go` returns no matches). Constitution Principle I.
- [x] A-017 `runOpen` does not exceed ~50 lines; no nested helper functions in `open.go` beyond the single handler. (Code quality: avoid premature abstraction.)
- [x] A-018 No new top-level subcommand added; `open` is a verb at args[1], slot-compatible with `where`/`cd`. Constitution Principle VI.
- [x] A-019 `go test ./src/...` passes; `go vet ./src/...` clean; `gofmt -l src/` empty.

### Documentation

- [x] A-020 `intake.md` and `spec.md` reflect the implemented design (no drift).
- [x] A-021 Hydration is deferred to the hydrate stage — this change does not modify `docs/memory/` directly.
