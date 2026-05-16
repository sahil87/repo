# Tasks: Flip to repo-first grammar

**Change**: 260506-koa2-flip-to-repo-first-grammar
**Spec**: `spec.md`
**Intake**: `intake.md`

## Phase 1: Setup

<!-- No new dependencies, no scaffolding. The change is pure refactor. -->

(No setup tasks needed — refactor only.)

## Phase 2: Core Implementation

<!-- Source code changes: shim rewrite, deletions, help text. Order matters for some. -->

- [x] T001 Rewrite `posixInit` in `src/cmd/hop/shell_init.go` to the new 4-step ladder. Replace the entire `hop()` function body with the new structure (per spec § Shim Behavior: Collapsed Precedence Ladder). Remove the `command -v "$1"` leading-slash check, the `type "$1"` builtin/keyword detection, both cheerful-error printf branches, and the `$2`-is-flag fallback. Remove `open` from the known-subcommand case-list. Update the comment header (lines 14-31) to describe the new ladder concisely. Preserve `_hop_dispatch`, `h()`, `hi()`, and the completion-suffix code path.

- [x] T002 Delete `src/cmd/hop/open.go` and `src/cmd/hop/open_test.go` entirely (whole files).

- [x] T003 Delete the `src/internal/platform/` directory and all its contents (`platform.go`, `open_darwin.go`, `open_linux.go`, `platform_test.go`).

- [x] T004 Remove `rootCmd.AddCommand(newOpenCmd())` from `src/cmd/hop/root.go::newRootCmd()`. Remove any `internal/platform` import from `root.go` if present. Verify by `grep -rn 'internal/platform\|newOpenCmd\|OpenCmd' src/` returning zero matches in production code.

- [x] T005 Update `src/cmd/hop/root.go::rootLong` help text per spec § Help Text: Updated Surface. Remove the `hop open <name>` row from the Usage table; replace `hop -R <name> <cmd>...` with `hop <name> -R <cmd>...`; replace shim sugar `hop <tool> <name>` with `hop <name> <tool>`. Update the `Notes:` block: remove builtin-filtering note, add the new "repo name always comes first" note.

- [x] T006 [P] Update `src/cmd/hop/shell_init_test.go` to assert the new shim emit:
  - Test that emitted stdout does NOT contain `command -v "$1"`, `type "$1"`, `is a shell builtin`, `is not a known subcommand or a binary on PATH`
  - Test that the case-list does NOT contain `open|`
  - Test that the new structure exists: `_hop_dispatch cd "$1"` for the 1-arg path, `command hop -R "$1"` for the multi-arg path
  - Update or remove any existing tests that asserted the old precedence-ladder structure (cheerful errors, builtin filtering)

- [x] T007 [P] Update `src/cmd/hop/integration_test.go`:
  - Remove any test cases that exercise `hop open <name>` (subcommand deleted)
  - Update tool-form integration test cases (if any) to use the new arg order: `hop <name> <tool>` instead of `hop <tool> <name>`
  - Update `-R` integration test cases (if any) that exercise the shim's user-facing form to use `hop <name> -R <cmd>...` (binary direct-invocation tests with `hop -R <name> <cmd>...` are unchanged — `extractDashR` internal shape is unchanged per Design Decision #1)

- [x] T008 [P] Verify `src/cmd/hop/dashr_test.go` is unchanged (per Design Decision #1, the binary's `-R` internal shape is preserved). If any test exercises a user-facing form (rare), update it to the new shape; otherwise leave the file alone.

## Phase 3: Integration & Edge Cases

- [x] T009 Run `cd src && go build ./...` and confirm the build succeeds with no references to `internal/platform` and no orphaned imports.

- [x] T010 Run `cd src && go test ./...` and confirm all tests pass. Triage failures: tests asserting old shim structure should already be updated by T006; tests asserting `hop open` behavior should already be removed by T007. Any other failure is a regression — investigate and fix.

- [x] T011 [P] Update `docs/specs/cli-surface.md` per spec § Spec Updates → Requirement: cli-surface.md Substantive Edits. Specifically:
  - Subcommand Inventory table: remove `hop open` row; flip `-R` row to `hop <name> -R <cmd>...`; flip shim-sugar row to `hop <name> <tool> [args...]`
  - Match Resolution Algorithm caller list: remove `hop open`
  - Behavioral Scenarios: flip `hop -R` exec scenarios; flip tool-form scenarios; remove `hop open` scenario; remove cheerful-error scenarios (replace with single note: "Missing tool surfaces via the binary's `hop: -R: '<cmd>' not found.` error.")
  - Stdout/stderr Conventions: remove `hop open` references
  - External Tool Availability table: remove `open`/`xdg-open` row
  - Design Decisions: delete old #10 (precedence ladder), #11 (`hop code` removal), #12 (builtin filtering); add new decision: "Grammar is `subcommand` xor `repo`. The first positional is one or the other — never a tool. This collapses the shim's precedence ladder, eliminates builtin filtering, and makes tab completion work in the repo slot for free."
  - Renumber subsequent decisions

- [x] T012 [P] Update `docs/specs/architecture.md`: remove `open.go`, `open_test.go` from the `cmd/hop/` listing; remove the entire `internal/platform/` block from the layout tree.

## Phase 4: Polish

<!-- Memory updates happen at hydrate stage, but the spec lists them under Affected Memory. They are written at hydrate, not here. Same for backlog/changelog. -->

(No polish tasks — memory updates are deferred to the hydrate stage per Fab convention.)

---

## Execution Order

- **Phase 2 inner ordering**:
  - T001 → T006 (shell_init_test depends on T001's emit content)
  - T002, T003 are independent file deletions; can run in any order
  - T004 must come after T002 (deletes the import the file once required)
  - T005 is independent of T001-T004 (touches only `rootLong`)
- T007 [P] is independent of T001-T005 (touches `integration_test.go`)
- T008 [P] is independent (verification task)
- **Phase 3 ordering**:
  - T009 must run after Phase 2 completes
  - T010 must run after T009
  - T011, T012 [P] are documentation; can run in parallel with each other and with T009/T010 (but should run after T002-T005 to reflect the final source state)

**Parallelizable groups**:
- Group A (Phase 2): T002, T003 in parallel; T001 alone; T004 after T002; T005 alone; T006 after T001; T007, T008 in parallel
- Group B (Phase 3): T009 → T010; T011, T012 in parallel
