# Plan: Add `w` and `s` tmux verbs

**Change**: 260507-is5r-add-w-s-tmux-verbs
**Status**: In Progress
**Intake**: `intake.md`
**Spec**: `spec.md`

## Tasks

### Phase 1: Binary — hint constants and RunE branches

- [x] T001 Add `wHint` and `sHint` const declarations in `src/cmd/hop/root.go` next to `cdHint`. Exact text per spec (`wHint`: tmux-window shell-only message; `sHint`: tmux-session shell-only message).
- [x] T002 Update `toolFormHintFmt` constant in `src/cmd/hop/root.go` to enumerate `(cd, where, w, s)` instead of `(cd, where)`.
- [x] T003 Add `case "w"` and `case "s"` to the 2-arg `args[1]` switch in `src/cmd/hop/root.go::newRootCmd().RunE`. Each returns `&errExitCode{code: 2, msg: <verb>Hint}`.
- [x] T004 Update the `rootLong` Notes block in `src/cmd/hop/root.go` so the verb grammar line lists `cd`, `where`, `w`, `s` as known $2 verbs (currently lists only `cd`, `where`).

### Phase 2: Shim — dispatch ladder branches and `_hop_dispatch` arms

- [x] T005 In `src/cmd/hop/shell_init.go::posixInit`, refactor the rule-5 verb-guard so the chain tests `$2` against `cd`, `where`, `w`, `s`, `-R`, otherwise — preserving the existing extra-args forwarding for `cd`/`where` (the `if [[ $# -gt 2 ]]` guard does NOT apply to `w`/`s`, which intentionally take optional 3rd/4th positional args).
- [x] T006 Add `w)` arm to `_hop_dispatch` in `src/cmd/hop/shell_init.go::posixInit`. Behavior: resolve path via `command hop "$2" where`, default window name to repo name, error if `$TMUX` unset (with hint to use `s`), otherwise `tmux new-window -c "$path" -n "$name"`.
- [x] T007 Add `s)` arm to `_hop_dispatch` in `src/cmd/hop/shell_init.go::posixInit`. Behavior: resolve path via `command hop "$2" where`, default session name and window name to repo name, check `tmux has-session -t "$session"` and error if exists, then branch on `$TMUX`: outside → `tmux new-session -s "$session" -c "$path" -n "$window"`; inside → `tmux new-session -d ...` then `tmux switch-client -t "$session"`.
- [x] T008 Update the `posixInit` doc comment header (`Resolution order in the hop() function`) to enumerate the new `$2 == "w"` and `$2 == "s"` lines and the `_hop_dispatch w)` / `s)` arms.

### Phase 3: Tests — binary

- [x] T009 [P] Add `TestVerbW_BinaryFormPrintsHint` in `src/cmd/hop/bare_name_test.go`: invoke `runArgs(t, "hop", "w")`, assert `*errExitCode` with code 2, `msg == wHint`, empty stdout. (Note: pattern matches existing `TestBareNameCdVerb` — first positional `"hop"` is the repo name from `singleRepoYAML`, second is the verb.)
- [x] T010 [P] Add `TestVerbS_BinaryFormPrintsHint` in `src/cmd/hop/bare_name_test.go`: invoke `runArgs(t, "hop", "s")`, assert `*errExitCode` with code 2, `msg == sHint`, empty stdout.
- [x] T011 [P] Add `TestToolFormHintEnumeratesAllVerbs` in `src/cmd/hop/bare_name_test.go`: invoke `runArgs(t, "hop", "notreal")`, assert stderr contains `(cd, where, w, s)`. (Effectively pins the new `toolFormHintFmt` text — the existing `TestBareNameToolForm` does byte-equality which auto-covers, but a substring assertion makes the verb-enumeration intent explicit and survives reformatting.)

### Phase 4: Tests — shim

- [x] T012 [P] Add `TestShellInitZshEmitsWVerbBranch` in `src/cmd/hop/shell_init_test.go`: assert emitted output contains `"$2" == "w"` and `_hop_dispatch w "$1"`.
- [x] T013 [P] Add `TestShellInitZshEmitsSVerbBranch` in `src/cmd/hop/shell_init_test.go`: assert emitted output contains `"$2" == "s"` and `_hop_dispatch s "$1"`.
- [x] T014 [P] Add `TestShellInitZshDispatchHasWArm` in `src/cmd/hop/shell_init_test.go`: assert emitted `_hop_dispatch` contains a `w)` arm and references `tmux new-window`.
- [x] T015 [P] Add `TestShellInitZshDispatchHasSArm` in `src/cmd/hop/shell_init_test.go`: assert emitted `_hop_dispatch` contains an `s)` arm and references `tmux new-session`, `tmux has-session`, and `tmux switch-client`.
- [x] T016 [P] Add `TestShellInitZshWErrorsOutsideTmux` in `src/cmd/hop/shell_init_test.go`: assert emitted `w)` arm contains the `requires an active tmux session` error string and a `$TMUX` test.
- [x] T017 [P] Add `TestShellInitZshSChecksSessionExists` in `src/cmd/hop/shell_init_test.go`: assert emitted `s)` arm contains the "already exists" hint string.
- [x] T018 [P] Update `TestShellInitBashEmitsFunctionAndCompletion` in `src/cmd/hop/shell_init_test.go` (or add a new bash-specific assertion) to verify the `w)` and `s)` arms appear in bash output too (same `posixInit` content; this is a sanity guard).

### Phase 5: Build verification and tidy

- [x] T019 Run `go build ./...` from `src/` to verify compilation.
- [x] T020 Run `go test ./...` from `src/` to verify all tests pass (existing + new).
- [x] T021 Run `go vet ./...` from `src/` to catch any vet violations.

## Execution Order

- T001–T004 (binary changes) are sequential within the file but can run before the shim changes
- T005 must precede T006/T007 (the verb-guard refactor places the new branches in the dispatch ladder)
- T006 and T007 are independent file-region edits but both modify `_hop_dispatch` — keep sequential to avoid edit conflicts
- T008 (doc comment update) follows T005–T007
- Phase 3 tests (T009–T011) depend on Phase 1 (T001–T003); marked [P] within the phase
- Phase 4 tests (T012–T018) depend on Phase 2 (T005–T008); marked [P] within the phase
- Phase 5 (T019–T021) runs after all implementation and tests

## Acceptance

### Functional Completeness

- [x] A-001 Shim w-routing: `hop shell-init zsh` output contains `[[ "$2" == "w" ]]` branch routing to `_hop_dispatch w "$1" "${@:3}"`
- [x] A-002 Shim s-routing: `hop shell-init zsh` output contains `[[ "$2" == "s" ]]` branch routing to `_hop_dispatch s "$1" "${@:3}"`
- [x] A-003 Shim w-dispatch: `_hop_dispatch` in shell-init output has a `w)` arm that calls `tmux new-window -c <path> -n <name>` (no `-d` flag) and tests `$TMUX`
- [x] A-004 Shim s-dispatch: `_hop_dispatch` in shell-init output has an `s)` arm that calls `tmux has-session`, `tmux new-session`, and `tmux switch-client` (with `$TMUX` branch)
- [x] A-005 Binary wHint: `hop <name> w` (direct binary, no shim) returns `errExitCode{code:2}` with `msg == wHint`
- [x] A-006 Binary sHint: `hop <name> s` (direct binary, no shim) returns `errExitCode{code:2}` with `msg == sHint`
- [x] A-007 Tool-form enum: `toolFormHintFmt` constant text contains `(cd, where, w, s)`
- [x] A-008 Hint constants live with cdHint: `wHint` and `sHint` are declared in `src/cmd/hop/root.go` adjacent to `cdHint`

### Behavioral Correctness

- [x] A-009 W default name: emitted `w)` arm uses `${3:-$1}` (or equivalent) so window name defaults to repo name when `$3` is omitted (implementation uses `${3:-$2}` — `$2` is the repo name in the dispatch helper's positional convention; equivalent intent verified)
- [x] A-010 S default names: emitted `s)` arm uses `${3:-$1}` for session and `${4:-$1}` for window (implementation uses `${3:-$2}` and `${4:-$2}` — same dispatch-positional convention as A-009)
- [x] A-011 W focuses by default: `tmux new-window` invocation in `w)` arm omits `-d` flag
- [x] A-012 S inside-tmux uses switch-client: `s)` arm's inside-tmux branch calls `tmux switch-client -t` (not `tmux attach`)
- [x] A-013 S outside-tmux foreground attach: `s)` arm's outside-tmux branch invokes `tmux new-session` without `-d` (foreground attach via the new-session itself)
- [x] A-014 S existing-session error: `s)` arm checks `tmux has-session -t "$session" 2>/dev/null` and prints "already exists" hint + returns 1 on hit
- [x] A-015 Resolution failure: `w)` and `s)` arms invoke `command hop "$2" where` and `return $?` (or equivalent) on failure, before any tmux invocation
- [x] A-016 W outside-tmux error: `w)` arm prints `hop: 'w' requires an active tmux session. Use 'h <name> s' to start one.` and returns 1 when `$TMUX` is unset, before any tmux invocation
- [x] A-017 Branch order preserved: shim rule-5 chain tests `$2` in order `cd`, `where`, `w`, `s`, `-R`, otherwise
- [x] A-018 Extra-args guard scoped: the `if [[ $# -gt 2 ]]; then command hop "$@"` extra-args guard remains in place for `cd`/`where` only, NOT for `w`/`s` (which intentionally accept optional 3rd/4th args)

### Scenario Coverage

- [x] A-019 Test `TestVerbW_BinaryFormPrintsHint` exists in `bare_name_test.go` and asserts wHint exact bytes + exit code 2 + empty stdout
- [x] A-020 Test `TestVerbS_BinaryFormPrintsHint` exists in `bare_name_test.go` and asserts sHint exact bytes + exit code 2 + empty stdout
- [x] A-021 Test `TestToolFormHintEnumeratesAllVerbs` exists in `bare_name_test.go` and asserts stderr contains `(cd, where, w, s)`
- [x] A-022 Test `TestShellInitZshEmitsWVerbBranch` exists in `shell_init_test.go`
- [x] A-023 Test `TestShellInitZshEmitsSVerbBranch` exists in `shell_init_test.go`
- [x] A-024 Test `TestShellInitZshDispatchHasWArm` exists and asserts `tmux new-window` reference
- [x] A-025 Test `TestShellInitZshDispatchHasSArm` exists and asserts `tmux new-session`, `tmux has-session`, `tmux switch-client` references
- [x] A-026 Test `TestShellInitZshWErrorsOutsideTmux` exists and asserts `requires an active tmux session` substring + `$TMUX` test
- [x] A-027 Test `TestShellInitZshSChecksSessionExists` exists and asserts `already exists` substring

### Edge Cases & Error Handling

- [x] A-028 3-arg direct-binary preserves cobra error: `hop <name> w api` direct invocation hits `MaximumNArgs(2)` (not RunE); existing `TestBareNameMaxArgs` covers via cursor/extra — verb-form 3-arg also blocked because `MaximumNArgs(2)` runs before any RunE switch on `args[1]`. Confirmed via existing test pass.
- [x] A-029 Repo resolution failure short-circuits tmux: `_hop_dispatch w` and `s` arms return non-zero from `command hop "$2" where` failure WITHOUT invoking any `tmux` command (verified by inspecting emitted shell — `return $?` immediately follows the `where` capture)
- [x] A-030 Shim NEVER nests tmux from inside: `s)` arm's inside-tmux branch uses `switch-client` and never `attach`/`new-session` without `-d`

### Code Quality

- [x] A-031 Pattern consistency: `wHint`/`sHint` follow the same comment-doc + const-declaration pattern as `cdHint`; shim arms follow the same case-arm + `local` + `command hop ... where` pattern as the existing `cd)` arm
- [x] A-032 No unnecessary duplication: name-default logic uses POSIX `${VAR:-default}` (not nested if/else); session-existence check uses standard `tmux has-session` idiom

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`
- Manual smoke test (recorded in PR description, not gated): run a real tmux session, verify `h <repo> w`, `h <repo> w api`, `h <repo> s`, `h <repo> s sess`, `h <repo> s sess win`, plus the in-tmux and out-of-tmux variants of each
