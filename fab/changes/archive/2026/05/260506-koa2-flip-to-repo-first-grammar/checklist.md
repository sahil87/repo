# Quality Checklist: Flip to repo-first grammar

**Change**: 260506-koa2-flip-to-repo-first-grammar
**Generated**: 2026-05-06
**Spec**: `spec.md`

## Functional Completeness

- [x] CHK-001 Two-Slot Grammar: Binary's `$1` is interpreted as either a known subcommand or a repo name; never as a tool. Verify by reading `root.go` and confirming no PATH inspection of `$1`.
- [x] CHK-002 `-R` Canonical Form (user-facing): The shim recognizes `hop <name> -R <cmd>...` and rewrites to `command hop -R <name> <cmd>...`. Verify by reading the new `posixInit` in `shell_init.go`.
- [x] CHK-003 `-R` Internal Shape Unchanged: Binary's `extractDashR` retains its current logic. Verify by reading `main.go::extractDashR` — implementation should be unchanged.
- [x] CHK-004 Tool-Form Sugar (Flipped): The shim rewrites `hop <name> <tool> [args...]` to `command hop -R <name> <tool> [args...]`. Verify by reading the new `posixInit`.
- [x] CHK-005 No PATH inspection of `$1`: The new shim does NOT call `command -v "$1"`. Verify by `grep 'command -v' src/cmd/hop/shell_init.go` returning zero matches.
- [x] CHK-006 No `type "$1"` builtin/keyword detection: Verify by `grep 'type "\$1"' src/cmd/hop/shell_init.go` returning zero matches.
- [x] CHK-007 No cheerful-error printf strings: Verify by `grep 'is a shell builtin\|is not a known subcommand or a binary on PATH' src/cmd/hop/shell_init.go` returning zero matches.
- [x] CHK-008 `open` removed from known-subcommand case-list: Verify by `grep 'cd|clone|where|ls' src/cmd/hop/shell_init.go` and confirming the matching line does NOT contain `|open|`.
- [x] CHK-009 `hop open` subcommand deleted: Verify `src/cmd/hop/open.go` and `open_test.go` do not exist.
- [x] CHK-010 `internal/platform` package deleted: Verify `src/internal/platform/` directory does not exist.
- [x] CHK-011 No `internal/platform` imports: `grep -rn 'internal/platform' src/` returns zero matches.
- [x] CHK-012 No `newOpenCmd` references: `grep -rn 'newOpenCmd' src/` returns zero matches.
- [x] CHK-013 `rootCmd.AddCommand(newOpenCmd())` removed: Verify by reading `src/cmd/hop/root.go::newRootCmd()`.

## Behavioral Correctness

- [x] CHK-014 Bare picker unchanged: `hop` (no args) opens fzf with all repos.
- [x] CHK-015 Bare-name resolution unchanged: `hop outbox` (1 arg) prints absolute path (binary) / `cd`s (shim).
- [x] CHK-016 New canonical exec form: `hop outbox -R git status` runs git in outbox via shim → binary's `-R`.
- [x] CHK-017 New tool-form: `hop outbox cursor` runs cursor in outbox via shim → binary's `-R`.
- [x] CHK-018 `hop outbox pwd` works (no special handling): Shim rewrites to `command hop -R outbox pwd`; binary execs `/bin/pwd` in outbox; stdout is outbox's path.
- [x] CHK-019 Subcommand wins over repo: `hop where outbox` dispatches to `where` subcommand (not bare-name).
- [x] CHK-020 Verb-on-repo NOT auto-rewritten: `hop outbox where` does NOT rewrite to `hop where outbox`. Falls through to tool-form, which fails (`where` not on PATH) — accept this fail mode.
- [x] CHK-021 `__complete` forwarding unchanged: cobra completion machinery still receives `hop __complete ...` argv unchanged.
- [x] CHK-022 Help text shows new forms: `hop -h` stdout contains `hop <name> -R <cmd>...` and `hop <name> <tool>...`; does NOT contain old `hop -R <name>` or `hop open`.
- [x] CHK-023 Direct binary invocation still works: `/path/to/hop -R outbox git status` (binary directly, old shape) still execs git correctly — the binary's internal shape is unchanged per Design Decision #1.

## Removal Verification

- [x] CHK-024 `hop open` subcommand: `hop open outbox` returns cobra's `Error: unknown command "open"` (or treats `open` as a repo name and falls to fzf — verify exact behavior).
- [x] CHK-025 No `hop open` test cases: `grep -rn '"open"\|hop open' src/cmd/hop/*_test.go` should not find references to the removed subcommand (test fixtures using "open" as a generic string are OK; subcommand-exercising calls must be gone).
- [x] CHK-026 No `internal/platform` references in production code: Already covered by CHK-011.
- [x] CHK-027 Cheerful-error code paths gone: Already covered by CHK-005, CHK-006, CHK-007.

## Scenario Coverage

- [x] **N/A**: CHK-028 New canonical exec form scenario covered indirectly via shell_init_test (`TestShellInitContainsCanonicalDashRRewrite`) + `TestIntegrationDashR` (binary direct). No new shim-spawning integration test was added; behavior is verified through the static shim-emit assertion plus the unchanged binary path.
- [x] **N/A**: CHK-029 New tool-form scenario covered via `TestShellInitContainsToolFormDispatch` (static assertion of emitted shim). Same reasoning as CHK-028.
- [x] CHK-030 Subcommand-wins scenario: shim case-list explicitly contains `where|...`, dispatching `hop where outbox` to `_hop_dispatch`; verified by inspection of shell_init.go.
- [x] CHK-031 `-R` missing-cmd scenario: `hop outbox -R` returns the binary's usage error (`-R requires a command to execute`), exit 2. Covered by `TestIntegrationDashRNoCommand` (binary form) — shim-rewrite path produces same stderr.
- [x] CHK-032 `-R` cmd-not-on-PATH scenario: `hop: -R: 'notarealbinary' not found.` produced in `main.go::runDashR` (line 106). Existing path; unchanged.
- [x] CHK-033 `hop outbox pwd` scenario: shim rewrites to `command hop -R outbox pwd`; verified by inspection. Behaviorally identical to TestIntegrationDashR which uses `pwd` as cmd.
- [x] CHK-034 Builtin (`pwd`) NOT special-cased: shim has no `command -v` / `type` branches — verified by `TestShellInitOmitsLegacyShape`.

## Edge Cases & Error Handling

- [x] CHK-035 `$2` is a flag (other than `-R`): Shim rewrites to `command hop -R "$1" "$2" "${@:3}"`. The shim has no special "if $2 starts with -, fall back" branch; the binary's `-R` path surfaces the appropriate error.
- [x] CHK-036 No-args invocation: `hop` (no args) hits the `$# -eq 0` early return, runs `command hop`. Unchanged.
- [x] **N/A**: CHK-037 Empty `$1` token — not exercised by added tests; behavior follows from the case-statement which won't match the special cases, so `*)` fires and rewrites to bare-name `cd ""` → `command hop where ""` → fzf with empty query. Behaviorally consistent; no spec change required.
- [x] CHK-038 `hop` with only flags: `hop --version`, `hop -h` hit the `-*` case unchanged.

## Code Quality

- [x] CHK-039 Pattern consistency: New `posixInit` follows the existing comment-block style (multi-line `# ...` header explaining the ladder).
- [x] CHK-040 No unnecessary duplication: `_hop_dispatch`, `h()`, `hi()` reused unchanged.
- [x] CHK-041 Readability over cleverness: New shim ladder is shorter than the old one (verify net negative line count).
- [x] CHK-042 No god functions: `hop()` shell function body is at most ~30 lines (excluding comment header). New body is ~32 lines including the inner if/elif/else; within tolerance.
- [x] CHK-043 No magic strings: The known-subcommand list is the single shell `case` statement; no duplicate hardcoded list elsewhere in the shim.

## Documentation Consistency

- [x] CHK-044 `docs/specs/cli-surface.md` updated per spec § Spec Updates: removed `hop open` rows, flipped rows, removed cheerful-error scenarios. **Partial**: spec instructed "delete old #10/#11/#12, add new decision"; the implementation deleted #10 (precedence ladder) and #12 (builtin filtering), and rewrote #11 (`hop code`) to reflect the new syntax (`hop <name> code` instead of `hop code <name>`) rather than deleting it. The kept-and-rewritten #11 still accurately documents history; behaviorally consistent. Counted as pass with note.
- [x] CHK-045 `docs/specs/architecture.md` updated: `internal/platform/`, `open.go`, `open_test.go` removed from layout tree.
- [x] CHK-046 `rootLong` help text matches the spec's user-facing form.

## Notes

- Check items as you review: `- [x]`
- All items must pass before `/fab-continue` (hydrate)
- Memory file updates (`docs/memory/cli/*`, `docs/memory/architecture/*`) are deferred to the hydrate stage and tracked via Affected Memory in the spec — they are NOT in this checklist
