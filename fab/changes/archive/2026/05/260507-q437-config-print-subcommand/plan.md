# Plan: Add `hop config print` subcommand

**Change**: 260507-q437-config-print-subcommand
**Status**: In Progress
**Intake**: `intake.md`
**Spec**: `spec.md`

## Tasks

<!-- Sequential work items for the apply stage. Checked off [x] as completed. -->

### Phase 1: Setup

- [x] T001 Add `newConfigPrintCmd()` factory in `src/cmd/hop/config.go` (mirrors `newConfigWhereCmd()` shape — `Use: "print"`, `Short: "print the resolved hop.yaml contents to stdout"`, `Args: cobra.NoArgs`, empty `RunE` body to be filled in T002). Add `"os"` to the imports if not already present.
- [x] T002 Register `newConfigPrintCmd()` in `newConfigCmd()`'s `cmd.AddCommand(...)` call (sibling of `newConfigInitCmd`, `newConfigWhereCmd`, `newConfigScanCmd`) and update `Short` from `"config helpers (init, where, scan)"` to `"config helpers (init, where, scan, print)"` in `src/cmd/hop/config.go`.

### Phase 2: Core Implementation

- [x] T003 Implement `RunE` body in `newConfigPrintCmd()` (`src/cmd/hop/config.go`): call `config.Resolve()` (NOT `ResolveWriteTarget`), then `os.ReadFile(path)`, then write the bytes to `cmd.OutOrStdout()`. Return any error from `Resolve()` directly (preserves existing error wording).
- [x] T004 Wrap `os.ReadFile` errors as `hop config print: read <path>: <underlying err>` in `src/cmd/hop/config.go::newConfigPrintCmd().RunE`.

### Phase 3: Integration & Edge Cases

- [x] T005 [P] Add `TestConfigPrintEmitsFileBytes` in `src/cmd/hop/config_test.go`: write a fixture `hop.yaml` containing comments and inline whitespace, set `$HOP_CONFIG`, run `hop config print`, assert stdout equals the on-disk bytes verbatim, assert stderr is empty, assert no error.
- [x] T006 [P] Add `TestConfigPrintMissingFileErrors` in `src/cmd/hop/config_test.go`: set `$HOP_CONFIG` to a non-existent path, run `hop config print`, assert error is non-nil, assert error message contains `points to` and `does not exist`.
- [x] T007 [P] Add `TestConfigPrintNoConfigErrors` in `src/cmd/hop/config_test.go`: clear `HOP_CONFIG`/`XDG_CONFIG_HOME`, set `HOME` to a temp dir with no `~/.config/hop/hop.yaml`, run `hop config print`, assert error contains `no hop.yaml found`.
- [x] T008 Update `TestConfigScanListedUnderConfigHelp` in `src/cmd/hop/config_test.go` so its name slice asserts `print` is also listed in `hop config --help` output (rename the test if appropriate, e.g., to `TestConfigSubcommandsListedUnderConfigHelp`, OR add a new sibling test — pick whichever is cleaner).

### Phase 4: Polish

- [x] T009 Update `docs/specs/cli-surface.md` § Subcommand Inventory: insert a new row for `hop config print` between `hop config where` and `hop config scan <dir>` with `Behavior summary` = "Print the resolved hop.yaml contents to stdout (raw bytes, comment-preserving)" and `Exit codes` = "0 success, 1 unresolvable / read error". Also update the `### Help Text` § `Usage:` block enumeration to include `hop config print` between `hop config where` and `hop config scan <dir>`.
- [x] T010 <!-- rework: original plan covered the spec doc only; the spec's Help Text section describes what the binary's `rootLong` Usage block enumerates, so the binary's `rootLong` constant must also list `hop config print` between `hop config where` and `hop config scan <dir>` — otherwise `hop --help` output diverges from the spec contract --> Add the line `hop config print          print the resolved hop.yaml contents to stdout` to `rootLong` in `src/cmd/hop/root.go`, inserted between the `hop config where` line and the `hop config scan <dir>` line, with column alignment matching the surrounding entries.
- [x] T011 <!-- rework: should-fix cleanups from review --> Tighten test assertions and drop redundancies in `src/cmd/hop/config_test.go`: (a) in `TestConfigSubcommandsListedUnderConfigHelp`, replace the loose `strings.Contains(gotOut, name)` substring check with a more discriminating fragment for `print` (e.g., assert the line `"  print "` appears, OR check that the full Short `"print the resolved hop.yaml contents"` appears) so the test cannot pass merely because `print` is a substring of an unrelated Short; (b) in `TestConfigPrintNoConfigErrors`, drop the redundant `os.Unsetenv("HOP_CONFIG")` and `os.Unsetenv("XDG_CONFIG_HOME")` calls — `clearConfigEnv(t)` already unsets both.

## Execution Order

- T001 → T002 → T003 → T004 (factory must exist before body, body before error-wrapping refinement)
- T005, T006, T007 are mutually independent (`[P]`) — all require T002 (subcommand registered) to dispatch correctly
- T008 depends on T002 (the `print` line in help text only appears after registration)
- T009 (doc update) is independent of the code path and may run any time
- T010 (rootLong update) is independent and may run any time
- T011 (test cleanups) requires T008 (renamed test exists)

## Acceptance

### Functional Completeness

- [x] A-001 Subcommand registration: `newConfigPrintCmd()` exists in `src/cmd/hop/config.go`, is added to `newConfigCmd()`'s subcommand list alongside `init`/`where`/`scan`, and `newConfigCmd().Short` reads `"config helpers (init, where, scan, print)"`.
- [x] A-002 Help-text discovery: `hop config --help` lists `print` (alongside `init`, `where`, `scan`); `hop --help`'s `config` short-line reflects the updated `Short`.
- [x] A-003 Argument surface: `hop config print` accepts zero positional args (`cobra.NoArgs`) and defines no flags. Extra positionals or unknown flags produce a non-zero exit before `RunE` runs.
- [x] A-004 Path resolution via `config.Resolve()`: `RunE` calls `config.Resolve()` (NOT `ResolveWriteTarget()`); errors from `Resolve()` propagate unchanged (preserves the existing `$HOP_CONFIG points to ...` and `no hop.yaml found ...` messages).
- [x] A-005 Raw byte output: `RunE` reads the resolved path with `os.ReadFile` and writes the bytes verbatim to `cmd.OutOrStdout()` — no parsing, no normalization, no synthetic header/framing/trailing newline.
- [x] A-006 Read error surfacing: `os.ReadFile` errors are wrapped as `hop config print: read <path>: <underlying error>`.
- [x] A-007 Stdout/stderr discipline: file bytes go to stdout via `cmd.OutOrStdout()`; success path writes nothing to stderr; errors propagate via cobra's standard return path.
- [x] A-008 No external tools invoked: `RunE` uses only `config.Resolve()`, `os.ReadFile`, and stdout writes — no `git`/`fzf`/`code`/`open` calls.
- [x] A-009 Help text strings: `Short = "print the resolved hop.yaml contents to stdout"`, no `Long` set, `Args = cobra.NoArgs`.

### Behavioral Correctness

- [x] A-010 `Short` line update: `newConfigCmd()`'s `Short` field includes `print` in its enumeration so root-level help reflects the new verb.

### Scenario Coverage

- [x] A-011 Scenario "`$HOP_CONFIG` set, file exists" → `TestConfigPrintEmitsFileBytes` passes (stdout matches file bytes verbatim, stderr empty).
- [x] A-012 Scenario "`$HOP_CONFIG` set, file missing" → `TestConfigPrintMissingFileErrors` passes (error contains `points to` and `does not exist`).
- [x] A-013 Scenario "no config in any search location" → `TestConfigPrintNoConfigErrors` passes (error contains `no hop.yaml found`).
- [x] A-014 Scenario "subcommand appears under `hop config --help`" → updated `TestConfigScanListedUnderConfigHelp` (or new sibling) asserts `print` is listed.
- [x] A-015 Scenario "comments and formatting preserved" → covered by the `TestConfigPrintEmitsFileBytes` fixture containing comments + inline whitespace and a verbatim byte-for-byte assertion.

### Edge Cases & Error Handling

- [x] A-016 Empty file → empty stdout, no error: covered behaviorally by the `os.ReadFile` + write-bytes path (zero bytes in, zero bytes out, no synthetic content).
- [x] A-017 Cobra rejects extra positionals: `cobra.NoArgs` enforced at parse time before `RunE`.

### Code Quality

- [x] A-018 Pattern consistency: `newConfigPrintCmd()` mirrors `newConfigWhereCmd()`'s factory shape (closure-returning function, same field ordering, same indent, same idiom for writing to `cmd.OutOrStdout()`).
- [x] A-019 No unnecessary duplication: Reuses `config.Resolve()` and stdlib `os.ReadFile`; does NOT reimplement search-order logic, error wording, or alternative load paths.
- [x] A-020 No god functions: `RunE` body is short (well under 50 lines) — single purpose, no branching beyond error returns.
- [x] A-021 No magic strings: error prefix `"hop config print: read %s: %w"` is the only inline literal and is single-use; no constants needed.
- [x] A-022 **N/A**: Composition over inheritance — Go cobra commands are composed via factory functions already; no inheritance to consider.

### Documentation

- [x] A-023 `docs/specs/cli-surface.md` § Subcommand Inventory has a new `hop config print` row between `hop config where` and `hop config scan <dir>`.
- [x] A-024 `docs/specs/cli-surface.md` § Help Text `Usage:` enumeration includes `hop config print` between `hop config where` and `hop config scan <dir>`.
- [x] A-025 `src/cmd/hop/root.go::rootLong` `Usage:` block lists `hop config print` between `hop config where` and `hop config scan <dir>` — keeps the binary's actual `hop --help` output in sync with the spec contract documented in `docs/specs/cli-surface.md` § Help Text.
- [x] A-026 `TestConfigSubcommandsListedUnderConfigHelp` asserts on a discriminating fragment for `print` (not just the substring `print`, which appears in other Shorts like `where`'s "print the resolved hop.yaml path").

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`
- Memory file (`docs/memory/cli/subcommands.md`) is updated during hydrate, NOT apply.
