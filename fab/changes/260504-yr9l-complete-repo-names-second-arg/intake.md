# Intake: Complete repo names in $2 of two-arg forms

**Change**: 260504-yr9l-complete-repo-names-second-arg
**Created**: 2026-05-04
**Status**: Draft

## Origin

> tab completion for repo names in $2 of two-arg forms (-R and tool-form)

Initiated as a `/fab-discuss` → `/fab-draft` flow. The user noticed that tab completion for repo names today only fires for `$1` (e.g. `hop <TAB>`, `hop where <TAB>`, `hop cd <TAB>`, etc.) but not for the `$2` slot of either two-arg form:

- `hop -R <TAB>` — global flag form, `$2` is the repo name
- `hop cursor <TAB>` — tool-form sugar in the shim (`hop <tool> <name>`), `$2` is the repo name

We walked the completion surface and pinpointed the bail in `src/cmd/hop/repo_completion.go::completeRepoNames` (lines 23–25) that returns `nil, ShellCompDirectiveNoFileComp` whenever `len(args) > 0`. Then we probed cobra's `__complete` machinery against the live binary and discovered a second, deeper obstacle for the `-R` form — see Why §3 below.

## Why

### 1. The pain point

Repo-name tab completion is the single biggest ergonomic win of `hop`'s shell integration. Without it, users either type the full repo name from memory or fall back to fzf. Today completion fires for the first positional of every repo-consuming subcommand (`hop <TAB>`, `hop where <TAB>`, `hop cd <TAB>`, `hop open <TAB>`, `hop clone <TAB>`) but **not** for the two two-arg shapes that put the repo name in `$2`:

```
hop -R <TAB>           # silent — should complete repo names
hop cursor <TAB>       # silent — should complete repo names (cursor is a tool on PATH)
```

Both shapes are documented in `docs/specs/cli-surface.md` and `docs/memory/cli/subcommands.md`, and `hop -R` is now the canonical way to run a tool inside a repo's directory (the dedicated `hop code` subcommand was removed in PR #9 in favor of tool-form). So completion gaps here directly degrade the workflow we just promoted.

### 2. The consequence of doing nothing

Users who learned to expect tab completion from the `$1` slot lose the affordance the moment they reach for `-R` or tool-form. They either type the repo name longhand or give up and use `hop cd <name>` followed by the tool — which defeats the purpose of `-R`/tool-form.

### 3. Why this approach over alternatives

The natural reading of "make completion work for `$2`" is: just relax the `len(args) > 0` bail in `completeRepoNames`. That handles tool-form. But probing the binary surfaced a second obstacle for `-R`:

```
$ hop __complete -R "" ""
hop: -R requires a command to execute. Usage: hop -R <name> <cmd>...
```

`extractDashR` in `main.go::main()` runs **before** `rootCmd.Execute()` — it intercepts any argv containing `-R` and dispatches to `runDashR`. Cobra's `__complete` machinery never gets a chance to run. So `-R` completion requires a small change in `main.go` too: when the argv shape is `hop __complete -R ...`, skip `extractDashR` and let cobra handle it.

For tool-form, the obstacle is purely the `len(args) > 0` bail. The shim already forwards `__complete*` to the binary verbatim (`shell_init.go:46`), so cobra sees the raw tokens.

Alternatives considered and rejected:

- **Add a flag completion via `cmd.RegisterFlagCompletionFunc("R", ...)`**: doesn't apply, because `extractDashR` consumes `-R` before cobra sees it. Would need to be paired with the main.go skip-on-`__complete` change anyway, at which point `RegisterFlagCompletionFunc` becomes redundant — `completeRepoNames` already runs as the root's `ValidArgsFunction` and can detect the `-R` shape from `args`.
- **Move `-R` handling out of `extractDashR` and into a real cobra flag**: much larger blast radius. The whole reason `-R` is intercepted pre-cobra is that the post-`<name>` argv is a child command line that cobra would otherwise misparse. Out of scope.
- **Custom completion entrypoint**: same problem — extractDashR runs first.

The chosen approach is the smallest viable change: a single-line skip in `main.go::main()` (only when argv shape is `__complete*`) plus a small expansion of `completeRepoNames` to handle the two-arg shapes.

## What Changes

### Change 1 — `main.go`: skip `extractDashR` for completion argv

Today `main()` runs `extractDashR(os.Args)` unconditionally before `rootCmd.Execute()`. That intercepts `hop __complete -R "" ""` and routes it to `runDashR` (or, in the empty-`-R` case, the malformed-error path) before cobra's completion machinery can run.

Proposed: detect whether `os.Args` looks like a completion invocation and skip `extractDashR` in that case. The cobra-internal entrypoints are `__complete` and `__completeNoDesc` (both already handled by the shim's `__complete*` glob).

```go
func main() {
    rootCmd := newRootCmd()
    rootCmd.Version = version
    rootForCompletion = rootCmd

    // -R must be handled before cobra parses argv (the post-<name> argv is a child
    // command line, not a hop subcommand). EXCEPT for cobra's hidden completion
    // entrypoints — those need to reach Execute() so completeRepoNames can run.
    if !isCompletionInvocation(os.Args) {
        if target, child, ok, err := extractDashR(os.Args); ok {
            if err != nil {
                fmt.Fprintln(os.Stderr, err.Error())
                os.Exit(2)
            }
            os.Exit(runDashR(target, child))
        }
    }

    if err := rootCmd.Execute(); err != nil {
        os.Exit(translateExit(err))
    }
}

// isCompletionInvocation reports whether argv is a cobra completion entrypoint
// (`__complete` or `__completeNoDesc` as args[1]). Used to suppress -R interception
// during tab completion so that the completion machinery can run.
func isCompletionInvocation(args []string) bool {
    if len(args) < 2 {
        return false
    }
    return args[1] == "__complete" || args[1] == "__completeNoDesc"
}
```

Tested by hand: with this skip in place, `hop __complete -R "" ""` reaches cobra's machinery, which dispatches to root's `ValidArgsFunction` with `args = ["-R", ""]` and `toComplete = ""`.

### Change 2 — `repo_completion.go::completeRepoNames`: handle two-arg shapes

Today (lines 22–25):

```go
func completeRepoNames(cmd *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
    if len(args) > 0 {
        return nil, cobra.ShellCompDirectiveNoFileComp
    }
    // ... load repos, filter subcommand collisions, return names
}
```

Proposed: replace the early bail with shape detection. Recognize two two-arg patterns where completing repo names is correct:

1. **`-R` form**: `args[0] == "-R"` and `len(args) == 1` (i.e., we're completing the slot right after `-R`)
2. **Tool-form**: `len(args) == 1`, `args[0]` is NOT a known root subcommand, AND `args[0]` resolves via `exec.LookPath` to an absolute path on PATH

For everything else with `len(args) > 0` (e.g., `hop -R name <TAB>` or `hop cursor name <TAB>`), keep returning empty — the third+ position is the child command's argv, not hop's.

Sketch:

```go
func completeRepoNames(cmd *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
    if len(args) > 0 {
        if !shouldCompleteRepoForSecondArg(cmd, args) {
            return nil, cobra.ShellCompDirectiveNoFileComp
        }
        // fall through to repo loading
    }
    rs, err := loadRepos()
    // ... existing logic
}

// shouldCompleteRepoForSecondArg reports whether the current completion context
// is the $2 slot of one of the two-arg forms that take a repo name there:
//   - `hop -R <name>` (global flag form; -R is consumed by main.go::extractDashR
//     in normal execution but reaches cobra during __complete via
//     isCompletionInvocation skip)
//   - `hop <tool> <name>` (tool-form sugar; <tool> must be a binary on PATH and
//     not a known root subcommand — mirrors shim rules 4 and 6)
func shouldCompleteRepoForSecondArg(cmd *cobra.Command, args []string) bool {
    if len(args) != 1 {
        return false
    }
    first := args[0]
    if first == "-R" {
        return true
    }
    // Tool-form: don't shadow subcommand dispatch; require absolute-path PATH hit.
    for _, sub := range cmd.Commands() {
        if sub.IsAvailableCommand() && sub.Name() == first {
            return false
        }
    }
    p, err := exec.LookPath(first)
    if err != nil {
        return false
    }
    return filepath.IsAbs(p)
}
```

Note: the subcommand-collision filter that already exists for `len(args) == 0` (lines 30–35) is a no-op here because we've already gated on "not a known subcommand", but we keep it to preserve the existing `args == 0` behavior unchanged.

### Change 3 — Tests

Add unit tests in `src/cmd/hop/repo_completion_test.go` (or extend if it exists):

- `completeRepoNames` with `args = []` returns repo names (existing behavior — regression guard)
- `completeRepoNames` with `args = ["-R"]` returns repo names
- `completeRepoNames` with `args = ["sometool"]` where `sometool` is on PATH returns repo names
- `completeRepoNames` with `args = ["sometool"]` where `sometool` is NOT on PATH returns empty
- `completeRepoNames` with `args = ["clone"]` (a known subcommand) returns empty (subcommand wins)
- `completeRepoNames` with `args = ["-R", "name"]` returns empty (third position is child argv)
- `completeRepoNames` with `args = ["sometool", "name"]` returns empty (third position is child argv)
- `isCompletionInvocation` with `["hop", "__complete", "-R", "", ""]` returns true
- `isCompletionInvocation` with `["hop", "-R", "name", "ls"]` returns false
- `isCompletionInvocation` with `["hop"]` returns false

For `exec.LookPath`-dependent cases, use a binary that is guaranteed to be on PATH on darwin and linux runners — `ls` or `sh` — *but* `ls` is also a hop subcommand, so it would be filtered by the subcommand check. Use `sh` (POSIX-guaranteed) for "tool exists" and a clearly fake name like `hop-nonexistent-tool-xyzzy` for "tool missing".

### Change 4 — Memory updates

Two memory files need a sentence each on the new completion behavior:

- `docs/memory/cli/subcommands.md` — extend the existing "tab completion" coverage (around line 117 and the tool-form / `-R` rows) with a note that `$2` of `-R` and tool-form completes repo names.
- `docs/memory/architecture/wrapper-boundaries.md` — if it documents the `extractDashR` pre-cobra interception, note the `__complete` skip.

(These are documented as `(modify)` entries below; the spec stage will confirm exact placement.)

### Change 5 — Spec updates

- `docs/specs/cli-surface.md` — add GIVEN/WHEN/THEN scenarios for the two new completion shapes alongside the existing zsh/bash completion scenarios (lines ~283–291).

## Affected Memory

- `cli/subcommands.md`: (modify) document `$2` repo-name completion for `-R` and tool-form
- `architecture/package-layout.md`: (modify) note the `__complete` skip in `main.go::main` next to the existing `extractDashR` description (verified: lines 12, 20, 72, 79 mention `extractDashR`)

## Impact

- **Code**: `src/cmd/hop/main.go` (add `isCompletionInvocation`, gate `extractDashR`), `src/cmd/hop/repo_completion.go` (extend `completeRepoNames`, add `shouldCompleteRepoForSecondArg`)
- **Tests**: `src/cmd/hop/repo_completion_test.go` (extend or create), possibly `src/cmd/hop/main_test.go` for `isCompletionInvocation`
- **Specs**: `docs/specs/cli-surface.md` (add scenarios)
- **Memory**: `docs/memory/cli/subcommands.md` (document new completion shapes)
- **No new dependencies**: `exec.LookPath` and `filepath.IsAbs` are stdlib.
- **Constitution**: aligns with V (Wrap, Don't Reinvent — using cobra's completion hooks rather than hand-rolling), VI (Minimal Surface Area — no new flags or subcommands).
- **Cross-platform**: `exec.LookPath` works the same on darwin and linux (the two supported targets).

## Open Questions

- Should we also handle the `-R=<name>` syntax in completion? Today `completeRepoNames` would see `args = ["-R=foo"]` which doesn't match either branch — completion would silently return empty. Probably acceptable since `-R=name` is an explicit power-user spelling, but worth confirming during spec.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Scope is repo-name completion in `$2` of `-R` and tool-form only — no new flags, no completion of child commands, no completion of `-R=<name>` glued form (unless trivially adopted) | Discussed in /fab-discuss session — user explicitly framed scope as "repo names tab completion in both $1 and $2 (in the 2-argument form)" | S:90 R:85 A:90 D:90 |
| 2 | Certain | The `len(args) > 0` early bail in `completeRepoNames` is the cause of the tool-form gap | Verified by reading the code (repo_completion.go:23-25) and probing the binary (`hop __complete cursor ""` returns directive 4 with no candidates) | S:95 R:85 A:95 D:95 |
| 3 | Certain | `extractDashR` in `main.go` short-circuits `-R` completion before cobra sees it | Verified by probing: `hop __complete -R "" ""` returns the malformed-`-R` error from `extractDashR`, never reaching cobra | S:95 R:80 A:95 D:95 |
| 4 | Confident | Tool-form completion gates on `exec.LookPath` returning an absolute path AND `args[0]` not being a known root subcommand, mirroring shim rules 4 and 6 | The shim's logic (shell_init.go:48 subcommand list and shell_init.go:65-66 leading-slash check) is the authoritative model — completion should mirror it so suggestions match what the shim will actually dispatch as tool-form | S:80 R:75 A:85 D:80 |
| 5 | Confident | `-R` completion triggers when `args == ["-R"]` only — not for `args == ["-R", "name"]` (that's the child command's argv, not a repo slot) | Matches the documented `-R` shape (`hop -R <name> <cmd>...`) — only the `<name>` slot takes a repo name | S:85 R:80 A:85 D:85 |
| 6 | Confident | The `main.go` change is gated on `args[1] in {"__complete", "__completeNoDesc"}` — cobra's two completion entrypoints | These are the only entrypoints the cobra-generated completion script invokes; the shim's `__complete*` glob covers both | S:80 R:80 A:85 D:85 |
| 7 | Tentative | Tests use `sh` as the "tool exists" fixture and a fake name as "tool missing" — `ls` is unusable because it's a hop subcommand | `sh` is POSIX-guaranteed on darwin and linux runners; the choice is plausible but other binaries (`true`, `cat`) would also work | S:55 R:80 A:75 D:60 | <!-- assumed: test fixture choice — `sh` is convenient but spec stage may pick differently -->
| 8 | Confident | Memory updates touch `cli/subcommands.md` (extend completion section) and `architecture/package-layout.md` (note `__complete` skip near existing `extractDashR` description) | Verified by grep against `docs/memory/`: `extractDashR`/`__complete` mentioned in `architecture/package-layout.md` lines 12/20/72/79 and `cli/subcommands.md` completion section ~line 117. No other memory files surface these symbols | S:80 R:75 A:85 D:80 |
| 9 | Confident | Constitution compliance: §V "Wrap, Don't Reinvent" (use cobra's completion hooks) and §VI "Minimal Surface Area" (no new flags/subcommands) — both satisfied | The change adds no new user-visible surface; it extends the behavior of an existing completion hook | S:80 R:85 A:85 D:80 |

9 assumptions (3 certain, 5 confident, 1 tentative, 0 unresolved).
