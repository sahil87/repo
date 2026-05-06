# Spec: Complete repo names in $2 of two-arg forms

**Change**: 260504-yr9l-complete-repo-names-second-arg
**Created**: 2026-05-04
**Affected memory**: `docs/memory/cli/subcommands.md`, `docs/memory/architecture/package-layout.md`

## Non-Goals

- Completion of the child command argv (positions 3+ in `hop -R <name> <cmd>...` and `hop <tool> <name> <args>...`) — those tokens belong to the child program, not hop
- Completion of `-R=<name>` glued form — the `-R=<name>` syntax is an explicit power-user spelling; supporting it requires a different argv shape (no `args` slot for `<name>`) and is out of scope
- Refactoring `extractDashR` into a real (non-hidden) cobra flag with full parsing — the post-`<name>` argv is a child command line that cobra would otherwise misparse; keeping the pre-Execute interception is intentional. (We do register `-R` as a *hidden, dormant* cobra flag, but only as a hook for cobra's flag-value completion machinery — see Design Decision #1. In normal execution `extractDashR` still consumes `-R` before cobra runs.)

## CLI: Repo-name tab completion in `$2`

### Requirement: Tab completion of `$2` in `hop -R <TAB>`

The binary SHALL emit repo-name candidates from `hop.yaml` when the cobra completion entrypoint (`__complete` or `__completeNoDesc`) is invoked with argv shape `[__complete, -R, <toComplete>]` (the user has typed `hop -R ` and is at the repo-name slot).

Two things are required to make this reachable:

1. The pre-cobra `-R` interception in `main.go::main` MUST be skipped when `os.Args[1]` is `__complete` or `__completeNoDesc`. The skip MUST NOT alter behavior for any other argv (including normal `hop -R <name> <cmd>...` invocations).
2. `-R` MUST be registered with the root cobra command as a *hidden* string flag (`MarkHidden`) so cobra's flag parser accepts it during completion (without registration, cobra aborts on the unknown shorthand before `ValidArgsFunction` runs). The flag's value is dormant in normal execution — `extractDashR` still consumes `-R` from `os.Args` before cobra parses it. The flag exists purely as a hook for `cmd.RegisterFlagCompletionFunc("R", completeRepoNamesForFlag)`.

`completeRepoNamesForFlag` SHALL return repo names from `hop.yaml` *without* applying the subcommand-collision filter that `completeRepoNames` applies at the bare `$1` slot — a repo whose name happens to match a hop subcommand (e.g., `clone`) is still a valid `-R` target because cobra has routed via the flag, not the subcommand dispatcher.

#### Scenario: Tab completion of `hop -R <TAB>` returns repo names

- **GIVEN** `hop.yaml` has repos `alpha`, `beta`, `dotfiles`
- **WHEN** the shell runs `hop __complete -R ""` (the cobra entrypoint invoked by tab completion)
- **THEN** stdout contains `alpha`, `beta`, and `dotfiles` as completion candidates
- **AND** stdout contains a `:<directive>` trailer line (cobra's standard completion output)
- **AND** exit code is 0

#### Scenario: Tab completion of `hop -R alph<TAB>` returns full candidate list (shell does prefix filtering)

- **GIVEN** `hop.yaml` has repos `alpha`, `beta`, `dotfiles`
- **WHEN** the shell runs `hop __complete -R "alph"`
- **THEN** stdout contains all three names (cobra hands the full list back; the generated shell script prefix-matches against `toComplete`)
- **AND** exit code is 0

#### Scenario: Tab completion at position 3+ of `-R` form returns no candidates

- **GIVEN** `hop.yaml` has repos `alpha`, `beta`
- **WHEN** the shell runs `hop __complete -R alpha ""` (user has already chosen the repo and is at the child command position)
- **THEN** stdout MUST NOT contain `alpha` or `beta` as candidates (the child command argv is not hop's to complete)
- **AND** exit code is 0

#### Scenario: Normal `hop -R <name> <cmd>` invocation is unaffected by the completion skip

- **GIVEN** `hop.yaml` has repo `outbox`
- **WHEN** the user runs `hop -R outbox echo hi` (not a completion call)
- **THEN** the pre-cobra `extractDashR` interception runs as before
- **AND** the binary execs `echo hi` with `cwd = <outbox>` and exits with the child's exit code

#### Scenario: Malformed `-R` (no value) at runtime is unaffected by the completion skip

- **GIVEN** the user runs `hop -R` with nothing after it
- **WHEN** `main` inspects `os.Args`
- **THEN** because `os.Args[1] == "-R"` (not `__complete`), the skip does NOT fire
- **AND** `extractDashR` produces the existing "hop: -R requires a value." stderr and exit 2

### Requirement: Tab completion of `$2` in tool-form (`hop <tool> <TAB>`)

The root command's `ValidArgsFunction` (`completeRepoNames`) SHALL emit repo-name candidates when `args == [first]` AND `first` would dispatch as tool-form per the shim's rules — namely:

1. `first` is NOT a known root subcommand of `cmd`, AND
2. `exec.LookPath(first)` returns an absolute path

These two conditions mirror shim rules 4 (subcommand check) and 6 (leading-slash check on `command -v`) in `shell_init.go::posixInit`. The completion semantics MUST match the shim's dispatch semantics so that tab-completion only suggests repo names when the shim will actually route the call to tool-form.

#### Scenario: Tab completion of `hop cursor <TAB>` returns repo names

- **GIVEN** `hop.yaml` has repos `alpha`, `dotfiles`
- **AND** a binary on PATH at an absolute path matches `cursor` (or any non-subcommand binary — `sh` is used in tests)
- **WHEN** the shell runs `hop __complete cursor ""`
- **THEN** stdout contains `alpha` and `dotfiles` as completion candidates
- **AND** exit code is 0

#### Scenario: Tab completion of `hop ls <TAB>` returns no repo names (subcommand wins)

- **GIVEN** `hop.yaml` has repos `alpha`, `dotfiles`
- **AND** `ls` is both a hop subcommand AND a binary on PATH
- **WHEN** the shell runs `hop __complete ls ""`
- **THEN** stdout MUST NOT contain `alpha` or `dotfiles` as candidates from the root completion (subcommand dispatch is in effect; the `hop ls` subcommand uses `cobra.NoArgs` and produces no positional candidates)
- **AND** exit code is 0

#### Scenario: Tab completion of `hop nonexistent-tool <TAB>` returns no candidates

- **GIVEN** `hop.yaml` has repos `alpha`, `dotfiles`
- **AND** `nonexistent-tool` (or any clearly-fake name) is NOT on PATH
- **WHEN** the shell runs `hop __complete nonexistent-tool ""`
- **THEN** stdout MUST NOT contain `alpha` or `dotfiles` as candidates (the call cannot dispatch as tool-form)
- **AND** exit code is 0

#### Scenario: Tab completion at position 3+ of tool-form returns no candidates

- **GIVEN** `hop.yaml` has repos `alpha`, `dotfiles`
- **AND** `sh` is a binary on PATH
- **WHEN** the shell runs `hop __complete sh dotfiles ""` (user has already chosen the repo and is at the child argv position)
- **THEN** stdout MUST NOT contain `alpha` or `dotfiles` as candidates (the child argv is not hop's to complete)
- **AND** exit code is 0

### Requirement: Existing `$1` completion behavior is preserved

The behavior for `args == []` (the existing `$1` completion) SHALL NOT change: repo names are still returned, names that collide with subcommand names are still filtered, and a missing config still surfaces zero candidates without error.

#### Scenario: Bare `hop <TAB>` still completes repo names

- **GIVEN** `hop.yaml` has repos `alpha`, `beta`
- **WHEN** the shell runs `hop __complete ""`
- **THEN** stdout contains `alpha` and `beta`
- **AND** exit code is 0

#### Scenario: Subcommand-collision filtering still applies at `$1`

- **GIVEN** `hop.yaml` has repos `alpha` and `clone` (the latter collides with the `hop clone` subcommand)
- **WHEN** the shell runs `hop __complete ""`
- **THEN** `alpha` is in the candidate list
- **AND** `clone` is NOT in the candidate list (collision filter)

## Architecture: pre-cobra `-R` interception

### Requirement: Completion-aware skip in `main`

`main.go::main` MUST gate the `extractDashR` call on a helper `isCompletionInvocation(os.Args)` that returns true exactly when `len(os.Args) >= 2 && os.Args[1] in {"__complete", "__completeNoDesc"}`. When the helper returns true, `extractDashR` is NOT invoked and control falls through to `rootCmd.Execute()` so cobra's completion machinery runs.

The helper MUST be defined in `main.go` (alongside `extractDashR`) and SHALL be exported only at package level (lowercase, no `package main` cross-package consumers exist).

#### Scenario: `isCompletionInvocation` recognizes both cobra completion entrypoints

- **GIVEN** the helper is invoked with `os.Args = ["hop", "__complete", "-R", "", ""]`
- **WHEN** `isCompletionInvocation(os.Args)` is called
- **THEN** it returns `true`

- **GIVEN** the helper is invoked with `os.Args = ["hop", "__completeNoDesc", "where", ""]`
- **WHEN** `isCompletionInvocation(os.Args)` is called
- **THEN** it returns `true`

#### Scenario: `isCompletionInvocation` rejects normal invocations

- **GIVEN** the helper is invoked with `os.Args = ["hop", "-R", "name", "ls"]`
- **WHEN** `isCompletionInvocation(os.Args)` is called
- **THEN** it returns `false`

- **GIVEN** the helper is invoked with `os.Args = ["hop"]` (length < 2)
- **WHEN** `isCompletionInvocation(os.Args)` is called
- **THEN** it returns `false`

## Architecture: completion shape detection in `repo_completion.go`

### Requirement: `shouldCompleteRepoForSecondArg` detects tool-form shape

`repo_completion.go` SHALL define a helper `shouldCompleteRepoForSecondArg(cmd *cobra.Command, args []string) bool` that returns `true` exactly when:

- `len(args) == 1`, AND
- `args[0]` is NOT a known root subcommand of `cmd` (filtered via `cmd.Commands()` and `IsAvailableCommand`), AND
- `exec.LookPath(args[0])` returns a path where `filepath.IsAbs` is true.

The helper covers tool-form (`hop <tool> <TAB>`) only. The `-R` slot is handled separately by `completeRepoNamesForFlag` registered via `cmd.RegisterFlagCompletionFunc("R", ...)` — cobra's flag parser consumes `-R` and its value before `ValidArgsFunction` is invoked, so an `args[0] == "-R"` branch in this helper is unreachable.

`completeRepoNames` MUST call this helper at the existing `len(args) > 0` branch: if it returns true, fall through to the existing repo-loading logic; otherwise, keep returning `(nil, ShellCompDirectiveNoFileComp)`.

### Requirement: `completeRepoNames` suppresses candidates at position 3+ of `-R`

When the user is at the child argv slot of `hop -R <name> <TAB>`, cobra has already consumed `-R <name>` as a flag pair, so `completeRepoNames` is invoked with `args=[]` (visually identical to bare `hop <TAB>`). To distinguish, `completeRepoNames` MUST check `cmd.Flag("R").Changed`: when true, the call originates from past `-R <name>`, and the helper SHALL return no candidates (the remaining argv is the child command's, not hop's).

#### Scenario: `args == ["sh"]` (binary on PATH, not a subcommand) is a repo slot

- **GIVEN** `cmd` is the root cobra command
- **AND** `sh` is on PATH at an absolute path
- **WHEN** `shouldCompleteRepoForSecondArg(cmd, []string{"sh"})` is called
- **THEN** it returns `true`

#### Scenario: `args == ["clone"]` (a known subcommand) is NOT a repo slot

- **GIVEN** `cmd` is the root cobra command, with `clone` registered as a subcommand
- **WHEN** `shouldCompleteRepoForSecondArg(cmd, []string{"clone"})` is called
- **THEN** it returns `false`

#### Scenario: `args == ["hop-nonexistent-tool-xyzzy"]` (not on PATH, not a subcommand) is NOT a repo slot

- **GIVEN** `cmd` is the root cobra command
- **AND** `hop-nonexistent-tool-xyzzy` is NOT on PATH
- **WHEN** `shouldCompleteRepoForSecondArg(cmd, []string{"hop-nonexistent-tool-xyzzy"})` is called
- **THEN** it returns `false`

#### Scenario: `args == ["sh", "name"]` (third position of tool-form) is NOT a repo slot

- **GIVEN** `cmd` is the root cobra command
- **WHEN** `shouldCompleteRepoForSecondArg(cmd, []string{"sh", "name"})` is called
- **THEN** it returns `false` (length != 1)

## Design Decisions

1. **`-R` completion via `RegisterFlagCompletionFunc` on a hidden, dormant cobra flag**
   - *Why*: Cobra's flag parser fails on the unknown `-R` shorthand before `ValidArgsFunction` runs, so a `args[0] == "-R"` branch in a `ValidArgsFunction`-style helper is unreachable. Registering `-R` as a `MarkHidden` cobra string flag (in `root.go::newRootCmd`) lets cobra's parser accept the token during completion; the matching `cmd.RegisterFlagCompletionFunc("R", completeRepoNamesForFlag)` then supplies repo-name candidates for the value slot. The flag is dormant in normal execution because `extractDashR` (gated by `isCompletionInvocation`) still consumes `-R` from `os.Args` before `Execute()` runs. This is the canonical cobra mechanism (Constitution §IV "Wrap, Don't Reinvent") and adds zero user-visible surface (`MarkHidden` keeps it out of `--help`).
   - *Rejected*: Detecting `args[0] == "-R"` inside `shouldCompleteRepoForSecondArg` (the original sketch in the intake) — cobra rejects the unknown shorthand before `ValidArgsFunction` is dispatched, so the helper never sees `["-R"]`. A small skip in `main.go::main` (`isCompletionInvocation`) IS still needed to bypass `extractDashR`'s pre-cobra interception, but it alone is insufficient — cobra still needs to know about `-R` as a flag.

2. **`completeRepoNamesForFlag` does NOT apply the subcommand-collision filter**
   - *Why*: A repo whose name matches a hop subcommand (e.g., `clone`) is a perfectly valid `-R` target — `hop -R clone <cmd>` runs `<cmd>` in the `clone` repo's directory; cobra has routed via the flag, not the subcommand dispatcher. Filtering `clone` from the candidate list here would mislead the user. Tested by `TestCompletionDashRSurfacesRepoNamedClone`.
   - *Rejected*: Reusing `completeRepoNames` (which applies the collision filter) for the flag-completion func — would hide valid `-R` targets and create user-visible asymmetry between the bare `<TAB>` and `-R <TAB>` slots.

3. **`completeRepoNames` checks `cmd.Flag("R").Changed` to suppress candidates at position 3+ of `-R`**
   - *Why*: When the user is at the child argv slot of `hop -R <name> <TAB>`, cobra has already absorbed `-R <name>` as a flag pair, so `completeRepoNames` is invoked with `args=[]` — visually identical to bare `hop <TAB>`. Without disambiguation, `hop -R alpha <TAB>` would offer repo names again. The flag's `Changed` bit is the cleanest signal that we're past the `-R` value slot.
   - *Rejected*: Inspecting `os.Args` directly inside `ValidArgsFunction` — would require parsing argv twice and create coupling between the helper and process-level state.

4. **Tool-form completion gates on `exec.LookPath` + leading-slash check, mirroring shim rules**
   - *Why*: The shim's dispatch logic in `shell_init.go::posixInit` is the authoritative model for what counts as tool-form. The completion should only suggest repo names when the shim would actually route the call as tool-form — otherwise the user sees suggestions that don't match runtime behavior.
   - *Rejected*: A simpler heuristic like "any arg that's not a subcommand" — would offer repo-name completion for `hop nonexistent-tool <TAB>` even though the shim's rule 8 will print a cheerful error and never call the binary. Misleading.

5. **Filter via `filepath.IsAbs(p)` rather than just `err == nil` from `exec.LookPath`**
   - *Why*: Mirrors the shim's leading-slash check on `command -v`, which filters builtins/keywords/aliases/functions. Without this, a builtin like `pwd` (which `command -v` would return as bare) could pass the LookPath check on platforms where `pwd` is also installed as `/bin/pwd`, but the shim's rule 7 would still reject it as a builtin. Aligning the completion gate with the shim gate is the design intent.
   - *Note*: In Go, `exec.LookPath` always returns an absolute path on success on POSIX systems (it walks `$PATH` and joins), so `IsAbs` is effectively a redundant guard — but it's cheap, defensive, and documents intent. Keep it.

6. **Subcommand check uses `cmd.Commands()` + `IsAvailableCommand()`, not a hardcoded list**
   - *Why*: The shim's rule 4 has a hardcoded subcommand list, but cobra already knows the truth via `cmd.Commands()`. Using cobra's introspection avoids drift if a new subcommand is added without remembering to update this list.
   - *Rejected*: Hardcoded list mirroring the shim — would create a second source of truth that drifts; the existing collision filter in `completeRepoNames` already uses cobra introspection.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Scope is repo-name completion in `$2` of `-R` and tool-form only — no `-R=<name>` glued form, no child argv completion, no new flags | Confirmed from intake #1 (user explicitly framed scope) | S:95 R:85 A:90 D:90 |
| 2 | Certain | The `len(args) > 0` early bail in `completeRepoNames` is the cause of the tool-form gap | Confirmed from intake #2 (verified by code reading + binary probing) | S:95 R:85 A:95 D:95 |
| 3 | Certain | `extractDashR` short-circuits `-R` completion before cobra sees it | Confirmed from intake #3 (verified by binary probing) | S:95 R:80 A:95 D:95 |
| 4 | Certain | Tool-form completion gates on `exec.LookPath` returning an absolute path AND `args[0]` not being a known root subcommand | Confirmed from intake #4 — clarified by user. Mirrors shim rules 4 and 6 in shell_init.go | S:95 R:75 A:85 D:80 |
| 5 | Certain | `-R` completion is wired via `cmd.RegisterFlagCompletionFunc("R", completeRepoNamesForFlag)` against a hidden `-R` cobra flag (not via an `args[0] == "-R"` branch in a ValidArgsFunction). At position 3+ of `-R` (`hop -R <name> <TAB>`), `completeRepoNames` returns no candidates by checking `cmd.Flag("R").Changed` | Confirmed from intake #5 (only `<name>` slot, never child argv); spec-stage discovery: cobra's flag parser blocks unregistered `-R` before ValidArgsFunction runs, so the canonical hook is `RegisterFlagCompletionFunc` | S:95 R:80 A:85 D:85 |
| 6 | Certain | `main.go` change is gated on `os.Args[1] in {"__complete", "__completeNoDesc"}` | Confirmed from intake #6 — clarified by user. These are cobra's two completion entrypoints | S:95 R:80 A:85 D:85 |
| 7 | Certain | Tests use `sh` as the "tool exists" fixture (POSIX-guaranteed, not a hop subcommand) and `hop-nonexistent-tool-xyzzy` as "tool missing" | Confirmed from intake #7 — clarified by user | S:95 R:80 A:75 D:60 |
| 8 | Certain | Memory updates touch `cli/subcommands.md` (extend completion section) and `architecture/package-layout.md` (note `__complete` skip near `extractDashR` description) | Confirmed from intake #8 — clarified by user | S:95 R:75 A:85 D:80 |
| 9 | Confident | Constitution compliance: §IV "Wrap, Don't Reinvent" (use cobra's completion hooks) and §VI "Minimal Surface Area" (no new flags) — both satisfied | Spec-stage correction: principle is §IV (not §V; §V is Thin Justfile). No new user-visible surface area; extends existing completion hook | S:90 R:85 A:90 D:90 |
| 10 | Confident | Helper `isCompletionInvocation` lives in `main.go` (alongside `extractDashR`) and `shouldCompleteRepoForSecondArg` lives in `repo_completion.go` (alongside `completeRepoNames`) — colocated with their callers, package-private | New at spec stage. Matches existing layout convention (helpers next to their single caller); `dashr_test.go` already covers `extractDashR`-adjacent logic | S:85 R:75 A:85 D:80 |
| 11 | Confident | Tests live in `repo_completion_test.go` (extending the existing file) and a new `main_test.go` for `isCompletionInvocation` — adjacent to source, package-private | New at spec stage. Test layout convention: tests adjacent to source, named after the file under test (`extractDashR` is in `dashr_test.go` because it's the prominent symbol; `isCompletionInvocation` is small enough to share `main_test.go`) | S:80 R:75 A:85 D:80 |
| 12 | Confident | Subcommand check uses `cmd.Commands()` + `IsAvailableCommand()` (cobra introspection), not a hardcoded list mirroring the shim | New at spec stage. Avoids two sources of truth; existing collision filter already uses this approach (repo_completion.go:30-35) | S:85 R:80 A:90 D:85 |

12 assumptions (8 certain, 4 confident, 0 tentative, 0 unresolved).
