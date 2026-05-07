# Spec: Add `w` and `s` tmux verbs

**Change**: 260507-is5r-add-w-s-tmux-verbs
**Created**: 2026-05-07
**Affected memory**: `docs/memory/cli/subcommands.md`

## Non-Goals

- **Run-command-in-window** (`h <repo> w api -- vim`) — defer until a real second use case arises. The `--` separator and tail-args parsing add complexity not justified by current need.
- **Detached / unfocused creation** (`-d` flag) — the verbs always focus the new window/session. No flag surface until asked.
- **Pane-split verb** (`p`) — splits are a different mental model (in-place division) than windows/sessions (new context). Defer.
- **Auto-dedup of duplicate window/session names** — tmux allows duplicates and disambiguates by index. Adding name-suffixing logic is premature optimization.
- **Tmux-availability check at install time** — match the lazy-tool pattern used for `fzf`, `git`, `brew`. Failure surfaces only at invocation.
- **Cross-multiplexer support** (screen, wezterm, kitty, iTerm) — `w` and `s` are explicitly tmux-bound. If demand for other multiplexers arises, it's a separate change with its own design.
- **Direct-binary support for the 3-arg form** (`hop <repo> w <name>`) — cobra's `MaximumNArgs(2)` cap stays. Direct-binary 3-arg invocations get cobra's generic `accepts at most 2 arg(s), received 3` error. The shim handles 3-arg forms; bypassing the shim is an edge case for scripts/CI.

## CLI: `w` verb (tmux window)

### Requirement: Shim shall route `$2 == "w"` to a window-creation handler

The shim's `posixInit` (in `src/cmd/hop/shell_init.go`) SHALL recognize `w` as a hop verb at `$2`. The dispatch occurs in rule 5 of the existing dispatch ladder, between the `where` branch and the `-R` branch. The handler SHALL receive `$1` (repo name) and `${@:3}` (optional window name and any trailing args) and route through `_hop_dispatch w`.

#### Scenario: `h outbox w` inside tmux with no window name

- **GIVEN** the user has the shim installed and is inside an active tmux session (`$TMUX` is set)
- **AND** `outbox` is a known repo in `hop.yaml` resolving to `/home/user/code/outbox`
- **WHEN** the user runs `h outbox w`
- **THEN** a new tmux window SHALL be created in the current session with `cwd=/home/user/code/outbox` and name `outbox`
- **AND** focus SHALL switch to the new window
- **AND** exit code SHALL be 0

#### Scenario: `h outbox w api` inside tmux with explicit window name

- **GIVEN** the user has the shim installed and is inside an active tmux session
- **AND** `outbox` is a known repo
- **WHEN** the user runs `h outbox w api`
- **THEN** a new tmux window SHALL be created with `cwd=<outbox-path>` and name `api`
- **AND** focus SHALL switch to the new window
- **AND** exit code SHALL be 0

#### Scenario: `h outbox w` outside tmux

- **GIVEN** the user has the shim installed but is not inside a tmux session (`$TMUX` is unset)
- **WHEN** the user runs `h outbox w`
- **THEN** the shim SHALL print to stderr: `hop: 'w' requires an active tmux session. Use 'h <name> s' to start one.`
- **AND** exit code SHALL be 1
- **AND** no tmux process SHALL be invoked

#### Scenario: `h notarealrepo w` (resolution failure)

- **GIVEN** the user has the shim installed and is inside tmux
- **AND** `notarealrepo` is not in `hop.yaml`
- **WHEN** the user runs `h notarealrepo w`
- **THEN** the shim SHALL invoke `command hop "notarealrepo" where`, which fails (no match)
- **AND** the binary SHALL emit its standard resolution error to stderr
- **AND** the shim SHALL NOT proceed to invoke tmux
- **AND** exit code SHALL propagate from the failed `hop where` invocation

### Requirement: Binary shall emit `wHint` when invoked directly with `args[1] == "w"`

The binary's root command `RunE` (in `src/cmd/hop/root.go`) SHALL handle the case `len(args) == 2 && args[1] == "w"` by returning `errExitCode{code: 2, msg: wHint}`. The constant `wHint` SHALL live next to `cdHint` and SHALL read:

```
hop: 'w' is shell-only (tmux window). Add 'eval "$(hop shell-init zsh)"' to your zshrc and run inside a tmux session.
```

#### Scenario: Direct binary invocation `hop outbox w`

- **GIVEN** the user invokes the binary directly (no shim), passing `outbox w` as args
- **WHEN** the binary's root `RunE` is called with `args == ["outbox", "w"]`
- **THEN** the binary SHALL print `wHint` to stderr (single line, no newline at end beyond what `errExitCode` adds)
- **AND** exit code SHALL be 2
- **AND** stdout SHALL be empty

#### Scenario: Direct binary invocation `hop outbox w api` (3 args)

- **GIVEN** the user invokes the binary directly with 3 positionals
- **WHEN** cobra parses argv before `RunE` runs
- **THEN** cobra's `MaximumNArgs(2)` SHALL reject the invocation with the generic message `accepts at most 2 arg(s), received 3`
- **AND** exit code SHALL be 1 (cobra's default for arg-count violations)
- **AND** `RunE` SHALL NOT be called

## CLI: `s` verb (tmux session)

### Requirement: Shim shall route `$2 == "s"` to a session-creation handler

The shim's `posixInit` SHALL recognize `s` as a hop verb at `$2`. The handler SHALL accept up to two optional positional arguments after the repo name: `$3` is the session name (default = repo name), `$4` is the window name (default = repo name). The handler SHALL route through `_hop_dispatch s`.

#### Scenario: `h outbox s` outside tmux with no names

- **GIVEN** the user has the shim installed and is not inside tmux
- **AND** `outbox` is a known repo resolving to `/home/user/code/outbox`
- **AND** no tmux session named `outbox` currently exists
- **WHEN** the user runs `h outbox s`
- **THEN** a new tmux session SHALL be created with name `outbox`, containing one window named `outbox`, with `cwd=/home/user/code/outbox`
- **AND** the foreground tmux client SHALL attach to the new session
- **AND** the user's terminal SHALL show the new tmux session

#### Scenario: `h outbox s outbox-dev` outside tmux with explicit session name

- **GIVEN** the user is not inside tmux
- **AND** no session named `outbox-dev` exists
- **WHEN** the user runs `h outbox s outbox-dev`
- **THEN** a new session SHALL be created with name `outbox-dev`, containing one window named `outbox` (default), with `cwd=<outbox-path>`
- **AND** foreground tmux client SHALL attach to it

#### Scenario: `h outbox s outbox-dev api` outside tmux with both names

- **GIVEN** the user is not inside tmux
- **AND** no session named `outbox-dev` exists
- **WHEN** the user runs `h outbox s outbox-dev api`
- **THEN** a new session SHALL be created with name `outbox-dev`, containing one window named `api`, with `cwd=<outbox-path>`
- **AND** foreground tmux client SHALL attach to it

#### Scenario: `h outbox s` inside tmux

- **GIVEN** the user is inside an active tmux session (`$TMUX` is set)
- **AND** no session named `outbox` exists
- **WHEN** the user runs `h outbox s`
- **THEN** a new detached session SHALL be created (`tmux new-session -d -s outbox -c <path> -n outbox`)
- **AND** the current tmux client SHALL switch to the new session via `tmux switch-client -t outbox`
- **AND** the shim SHALL NOT attempt `tmux attach` from inside tmux (nested-tmux is forbidden)

#### Scenario: `h outbox s outbox-dev` when session already exists

- **GIVEN** a tmux session named `outbox-dev` already exists (verified via `tmux has-session -t outbox-dev`)
- **WHEN** the user runs `h outbox s outbox-dev`
- **THEN** the shim SHALL print to stderr: `hop: tmux session 'outbox-dev' already exists. Use 'h <name> w' to add a window in the current session.`
- **AND** exit code SHALL be 1
- **AND** no tmux state SHALL be modified (no new session, no switch-client, no attach)

#### Scenario: `h notarealrepo s` (resolution failure)

- **GIVEN** the user runs `s` against an unknown repo
- **WHEN** the shim invokes `command hop "notarealrepo" where`
- **THEN** the resolution SHALL fail and the shim SHALL NOT invoke tmux
- **AND** exit code SHALL propagate from the failed `hop where`

### Requirement: Binary shall emit `sHint` when invoked directly with `args[1] == "s"`

The binary's root `RunE` SHALL handle the case `len(args) == 2 && args[1] == "s"` by returning `errExitCode{code: 2, msg: sHint}`. The constant `sHint` SHALL live next to `wHint` and SHALL read:

```
hop: 's' is shell-only (tmux session). Add 'eval "$(hop shell-init zsh)"' to your zshrc.
```

#### Scenario: Direct binary invocation `hop outbox s`

- **GIVEN** the user invokes the binary directly with `args == ["outbox", "s"]`
- **WHEN** `RunE` is called
- **THEN** the binary SHALL print `sHint` to stderr
- **AND** exit code SHALL be 2
- **AND** stdout SHALL be empty

## CLI: tool-form hint enumerates new verbs

### Requirement: `toolFormHintFmt` shall enumerate `cd, where, w, s`

The constant `toolFormHintFmt` (in `src/cmd/hop/root.go`) SHALL be updated so its enumerated verb list grows from `(cd, where)` to `(cd, where, w, s)`. The full text becomes:

```
hop: '%s' is not a hop verb (cd, where, w, s). For tool-form, install the shim: eval "$(hop shell-init zsh)", or use: hop -R "<name>" <tool> [args...]
```

#### Scenario: Tool-form fall-through with unknown verb

- **GIVEN** the user invokes the binary directly with `args == ["outbox", "notreal"]`
- **AND** `notreal` is not in the verb set `{cd, where, w, s}`
- **WHEN** `RunE`'s 2-arg default branch runs
- **THEN** stderr SHALL contain `hop: 'notreal' is not a hop verb (cd, where, w, s).`
- **AND** the line SHALL include the shim install hint and the `-R` escape form
- **AND** exit code SHALL be 2

## Shell init: dispatch ladder

### Requirement: `posixInit` rule 5 shall add `w` and `s` branches between `where` and `-R`

The shim's rule-5 dispatch ladder SHALL gain two new branches placed after `$2 == "where"` and before `$2 == "-R"`, preserving the existing branch order for known verbs. Branch placement order: `cd`, `where`, `w`, `s`, `-R`, otherwise.

#### Scenario: Emitted shell contains `w` branch

- **WHEN** `hop shell-init zsh` is run
- **THEN** the emitted shell SHALL contain a branch matching `[[ "$2" == "w" ]]` that routes to `_hop_dispatch w "$1" "${@:3}"`

#### Scenario: Emitted shell contains `s` branch

- **WHEN** `hop shell-init zsh` is run
- **THEN** the emitted shell SHALL contain a branch matching `[[ "$2" == "s" ]]` that routes to `_hop_dispatch s "$1" "${@:3}"`

#### Scenario: Branch ordering

- **WHEN** `hop shell-init zsh` is run
- **THEN** the emitted shell's rule-5 conditional chain SHALL test `$2` values in this order: `cd`, `where`, `w`, `s`, `-R`, then fall through to tool-form
- **AND** the same ordering SHALL hold for `hop shell-init bash`

### Requirement: `_hop_dispatch` shall gain `w)` and `s)` arms

The `_hop_dispatch` helper SHALL add two new case arms (`w)` and `s)`) that:
1. Resolve the repo path via `command hop "$2" where` and capture stdout (where `$2` here refers to dispatch's own positional, which is the repo name)
2. Compute the window/session name (default to the repo name if the optional positional is absent)
3. Inspect `$TMUX` to branch on inside-vs-outside-tmux behavior
4. Invoke `tmux` with appropriate subcommand and arguments

#### Scenario: `_hop_dispatch w` inside tmux

- **GIVEN** dispatch is invoked as `_hop_dispatch w outbox` (no window name) from inside tmux
- **WHEN** the `w)` arm runs
- **THEN** it SHALL resolve the path via `command hop outbox where`
- **AND** invoke `tmux new-window -c "$path" -n outbox` (window name defaults to repo)
- **AND** tmux's default behavior SHALL focus the new window (no `-d` flag passed)

#### Scenario: `_hop_dispatch w` outside tmux

- **GIVEN** dispatch is invoked as `_hop_dispatch w outbox` from outside tmux
- **WHEN** the `w)` arm runs
- **THEN** it SHALL print `hop: 'w' requires an active tmux session. Use 'h <name> s' to start one.` to stderr
- **AND** return 1
- **AND** SHALL NOT invoke tmux

#### Scenario: `_hop_dispatch s` outside tmux, session does not exist

- **GIVEN** dispatch is invoked as `_hop_dispatch s outbox outbox-dev api` from outside tmux
- **AND** `tmux has-session -t outbox-dev` returns non-zero
- **WHEN** the `s)` arm runs
- **THEN** it SHALL invoke `tmux new-session -s outbox-dev -c "$path" -n api` (foreground attach)

#### Scenario: `_hop_dispatch s` inside tmux, session does not exist

- **GIVEN** dispatch is invoked as `_hop_dispatch s outbox` (no session/window names) from inside tmux
- **AND** `tmux has-session -t outbox` returns non-zero
- **WHEN** the `s)` arm runs
- **THEN** it SHALL invoke `tmux new-session -d -s outbox -c "$path" -n outbox`
- **AND** invoke `tmux switch-client -t outbox`

#### Scenario: `_hop_dispatch s` when session already exists

- **GIVEN** dispatch is invoked as `_hop_dispatch s outbox outbox-dev`
- **AND** `tmux has-session -t outbox-dev` returns 0 (session exists)
- **WHEN** the `s)` arm runs
- **THEN** it SHALL print the "already exists" hint to stderr
- **AND** return 1
- **AND** SHALL NOT invoke `tmux new-session` or `tmux switch-client` or `tmux attach`

## Tests: binary

### Requirement: Test SHALL verify `wHint` is printed for `hop <name> w` direct binary invocation

A test in `src/cmd/hop/bare_name_test.go` (or a new `verbs_test.go`) SHALL exercise the binary's RunE with `args == ["outbox", "w"]` and assert exact stderr matching `wHint`, empty stdout, and exit code 2.

#### Scenario: Test asserts wHint exact bytes

- **GIVEN** a test invokes the root command with `args == ["outbox", "w"]`
- **WHEN** the test inspects the result
- **THEN** stderr SHALL match `wHint` exactly (single line)
- **AND** stdout SHALL be empty
- **AND** the returned error SHALL be `errExitCode{code: 2}`

### Requirement: Test SHALL verify `sHint` is printed for `hop <name> s` direct binary invocation

A test SHALL exercise the binary's RunE with `args == ["outbox", "s"]` and assert exact stderr matching `sHint`, empty stdout, and exit code 2.

#### Scenario: Test asserts sHint exact bytes

- **GIVEN** a test invokes the root command with `args == ["outbox", "s"]`
- **WHEN** the test inspects the result
- **THEN** stderr SHALL match `sHint` exactly
- **AND** stdout SHALL be empty
- **AND** the returned error SHALL be `errExitCode{code: 2}`

### Requirement: Test SHALL verify `toolFormHintFmt` enumerates all four verbs

A test SHALL exercise the binary's RunE with `args == ["outbox", "notreal"]` and assert that the formatted error message contains the substring `(cd, where, w, s)`.

#### Scenario: Tool-form hint enumerates verbs

- **GIVEN** a test invokes the root command with `args == ["outbox", "notreal"]`
- **WHEN** the test inspects stderr
- **THEN** stderr SHALL contain `(cd, where, w, s)`
- **AND** exit code SHALL be 2

## Tests: shim

### Requirement: Tests SHALL verify shim emits `w` and `s` branches and dispatch arms

Tests in `src/cmd/hop/shell_init_test.go` SHALL verify that the output of `hop shell-init zsh` and `hop shell-init bash` contains:
1. A branch testing `$2 == "w"` that routes to `_hop_dispatch w`
2. A branch testing `$2 == "s"` that routes to `_hop_dispatch s`
3. A `w)` arm in the `_hop_dispatch` switch
4. An `s)` arm in the `_hop_dispatch` switch

#### Scenario: zsh emission contains w branch

- **WHEN** `hop shell-init zsh` is run
- **THEN** the emitted output SHALL contain a substring matching `[[ "$2" == "w" ]]`
- **AND** SHALL contain `_hop_dispatch w "$1"`

#### Scenario: zsh emission contains s branch

- **WHEN** `hop shell-init zsh` is run
- **THEN** the emitted output SHALL contain a substring matching `[[ "$2" == "s" ]]`
- **AND** SHALL contain `_hop_dispatch s "$1"`

#### Scenario: bash emission contains both branches

- **WHEN** `hop shell-init bash` is run
- **THEN** both `w` and `s` branches SHALL be present (same shared `posixInit` content)

#### Scenario: dispatch helper has w and s arms

- **WHEN** `hop shell-init zsh` is run
- **THEN** the emitted `_hop_dispatch` function SHALL contain a `w)` case arm
- **AND** SHALL contain an `s)` case arm

## Documentation

### Requirement: `docs/memory/cli/subcommands.md` SHALL document the new verbs and dispatch behavior

The hydrate stage SHALL update `docs/memory/cli/subcommands.md` to:
1. Add `hop <name> w` and `hop <name> s` rows to the Inventory table
2. Update the rule-5 dispatch ladder narrative in the `posixInit` section to enumerate the new branches in their correct order (`cd`, `where`, `w`, `s`, `-R`, otherwise)
3. Add `wHint` and `sHint` text under the "Binary-form hint texts" section
4. Update the `toolFormHintFmt` text to show `(cd, where, w, s)`
5. Update the "Tool-form dispatch" examples table with at least one row each for `w` and `s` showing the shim's tmux invocation
6. Note the lazy `tmux` runtime dependency under "External tool failure messages" with the appropriate hint text

#### Scenario: Hydrate updates subcommands.md

- **GIVEN** the implementation is complete and review has passed
- **WHEN** the hydrate stage runs
- **THEN** `docs/memory/cli/subcommands.md` SHALL contain rows for `hop <name> w` and `hop <name> s` in the Inventory table
- **AND** the rule-5 dispatch ladder narrative SHALL list the verb branches in the order `cd`, `where`, `w`, `s`, `-R`, otherwise

### Requirement: `docs/specs/cli-surface.md` SHALL document the verb set growth

The hydrate stage SHALL update `docs/specs/cli-surface.md` to record the verb set expansion as a Design Decision (or update existing Decisions list) and add `w`/`s` rows to the subcommand inventory there.

#### Scenario: cli-surface.md captures the design

- **WHEN** hydrate runs
- **THEN** `docs/specs/cli-surface.md` SHALL document the addition of `w` and `s` as $2 verbs with their behavior matrix and the single-letter rationale

### Requirement: README SHALL show a tmux-aware navigation example

`README.md` SHALL gain a short example demonstrating `h <name> w` (or `s`) under the "First run" or "Usage" section so new users discover the verbs.

#### Scenario: README example exists

- **WHEN** hydrate runs
- **THEN** `README.md` SHALL contain a usage example invoking `h <name> w` and one invoking `h <name> s`

## Design Decisions

1. **Single-letter verbs (`w`, `s`) over wordlists (`window`, `session`) or sigils (`:w`, `:s`)**: Single letters minimize collision risk with PATH binaries reachable via tool-form, visually distinguish hop verbs from tool names at a glance, and match the existing `h` / `hi` one-letter alias family in `shell-init`. The "command 1st priority, system tool 2nd" requirement is satisfied because the reserved-word table at `$2` checks before falling through to the tool-form `-R` rewrite.
   - *Why*: User confirmed during `/fab-discuss`. Memorability + low collision = best ergonomic trade.
   - *Rejected*:
     - **Wordlists** (`window`/`session`) — more shadowing surface (every additional verb shadows another potential PATH name); typing cost; less consistent with one-letter alias family.
     - **Sigil prefix** (`:w`/`:s`) — uglier; treats verbs and tools as disjoint rather than prioritized; doesn't match the existing `cd`/`where` precedent.
     - **Binary-owned dispatch** — pulls binary into terminal-multiplexer territory (Constitution VI violation); can't mutate parent shell from a child process anyway.

2. **Both verbs are shim-only (binary just emits hints)**: Binary `RunE` for `args[1] == "w"` or `"s"` returns `errExitCode{code: 2, msg: <verb>Hint}`. Shim handles all tmux invocation directly.
   - *Why*: Binary cannot mutate the parent shell or tmux client state (it's a child process). Mirrors `cd` verb's existing shell-only nature. Keeps binary tmux-free (no new dependency in compiled artifact).
   - *Rejected*: Binary spawning tmux subprocess — would create a tmux session detached from the user's interactive session; user would not see the new window/session.

3. **Default name = repo name**: When `$3` (window name for `w`, session name for `s`) is omitted, default to the repo name (`$1`). When `$4` (window name for `s`) is omitted, also default to repo name.
   - *Why*: Predictable, matches user mental model ("the outbox window"). No auto-dedup needed — tmux allows duplicates and disambiguates by index.
   - *Rejected*:
     - **Tmux's default** (incrementing index, current command name) — anonymous windows defeat the discoverability value of named-per-repo workflow.
     - **Auto-dedup with suffix** (`outbox-2`) — premature; add only if duplicate-name collisions become a real complaint.

4. **Always focus the new window/session**: `w` uses `tmux new-window` without `-d`; `s` uses `tmux switch-client` (inside tmux) or attaches in foreground (outside tmux).
   - *Why*: The whole point of the gesture is "take me to the repo." Not focusing means `Ctrl-b n` afterward, defeating the ergonomics.
   - *Rejected*: `-d` / detached as default — rare use case (background prep). Add as a flag later if asked.

5. **`s` errors when session already exists (no silent attach)**: `tmux has-session -t <name>` check before `new-session`. On hit, print hint and exit 1.
   - *Why*: Preserves the `s` vs `w` semantic distinction. `s` means "create a new session"; if the user wants to add a window to an existing session, the right verb is `w`. Silent attach would conflate the two.
   - *Rejected*:
     - **Silent attach** — surprising, conflates verbs.
     - **Suffix and create** (`outbox-dev-2`) — premature dedup, unclear naming intent.

6. **`w` outside tmux errors with hint to use `s`**: `tmux new-window` requires an existing session. Without one, `w` is meaningless.
   - *Why*: Symmetric guidance — error directs the user to the right verb (`s`) for their actual situation.
   - *Rejected*: Silently fall through to `s` — surprising verb-mutation; users should be explicit about session creation.

7. **`s` inside tmux uses `switch-client`, not `attach`**: Nested tmux is forbidden; attaching from inside tmux is a footgun.
   - *Why*: Matches tmux's own model. `switch-client -t <session>` is the correct in-tmux idiom.
   - *Rejected*: `attach` from inside tmux — tmux refuses with "sessions should be nested with care, set $TMUX to force."

8. **Asymmetric positionals (`w [window]` vs `s [session] [window]`)**: For `w`, the first optional arg names the window. For `s`, the first optional arg names the session, and the second names the window.
   - *Why*: Consistent rule — "the first optional arg names the primary thing the verb creates." Documented in shell-init help.
   - *Rejected*: Force same-shape positionals — would either lose `s`'s window-naming capability or require flags, both worse than learning a one-line rule.

9. **Cobra `MaximumNArgs(2)` cap stays**: 3-arg direct-binary invocations get cobra's generic `accepts at most 2 arg(s)` error.
   - *Why*: User confirmed acceptable. Direct-binary 3-arg invocation is an edge case for scripts/CI bypassing the shim. Raising the cap would require RunE to handle 3-arg cases for verbs that the binary can't actually execute (shell-only).
   - *Rejected*: Raise to `MaximumNArgs(4)` and add explicit hint branches — more code, weaker cobra validation, no real win since 3-arg form is shim-only.

10. **Tmux invocations go through the shell, not `internal/proc`**: The shim calls `tmux` directly (alongside `cd`, which is also shell-resident).
    - *Why*: `internal/proc` is a binary-side abstraction (Constitution Principle I — security-first subprocess execution from Go). The shim is a different layer. Tmux input comes from the user's typed command (already shell-quoted) and from `command hop "$1" where` (binary-validated path).
    - *Rejected*: Adding tmux invocation to the binary so it can route through `internal/proc` — defeats the shim-only design and adds binary surface.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Single-letter verbs `w` and `s` (not wordlists, not sigils) | Confirmed from intake #1. User chose during `/fab-discuss`; matches existing one-letter alias family. | S:95 R:80 A:85 D:90 |
| 2 | Certain | Reserved-word-at-$2 mechanism (extends existing `cd`/`where` slot) | Confirmed from intake #2. Already the post-PR-#14 pattern. | S:95 R:75 A:90 D:95 |
| 3 | Certain | Both verbs are shim-only; binary just emits hints | Confirmed from intake #3. User confirmed during discuss. | S:95 R:80 A:90 D:95 |
| 4 | Certain | Default window/session name = repo name when not specified | Confirmed from intake #4. | S:90 R:90 A:85 D:90 |
| 5 | Certain | Always focus the new window/session | Confirmed from intake #5. | S:90 R:90 A:85 D:95 |
| 6 | Certain | `s` errors if session already exists (no silent attach) | Confirmed from intake #6. | S:90 R:80 A:80 D:85 |
| 7 | Certain | `w` outside tmux → error, hint to use `s` | Confirmed from intake #7. | S:90 R:90 A:90 D:95 |
| 8 | Certain | `s` outside tmux → foreground attach; `s` inside tmux → switch-client | Confirmed from intake #8. | S:90 R:75 A:90 D:90 |
| 9 | Certain | Cobra `MaximumNArgs(2)` stays — generic error for direct 3-arg binary calls | Confirmed from intake #9. | S:90 R:75 A:80 D:90 |
| 10 | Certain | Tmux invocations go through shell, not `internal/proc` | Confirmed from intake #10. | S:90 R:70 A:90 D:90 |
| 11 | Certain | Binary `wHint`/`sHint` constants live next to `cdHint` in `root.go` | Confirmed from intake #11. Pattern match. | S:90 R:90 A:95 D:95 |
| 12 | Certain | `toolFormHintFmt` updated to enumerate `(cd, where, w, s)` | Confirmed from intake #12. | S:90 R:95 A:95 D:95 |
| 13 | Certain | Asymmetric positionals: `w [window]`, `s [session] [window]` | Confirmed from intake #13. | S:85 R:75 A:80 D:80 |
| 14 | Certain | Out of scope: run-command-in-window, `-d` flag, pane-split verb, auto-dedup, cross-multiplexer | Confirmed from intake #14, expanded with cross-multiplexer exclusion. | S:90 R:85 A:80 D:90 |
| 15 | Certain | Branch order in shim rule-5 ladder: `cd`, `where`, `w`, `s`, `-R`, otherwise | New — preserves existing branch order; appends new verbs before `-R` (last specific branch before fall-through) | S:85 R:90 A:90 D:95 |
| 16 | Certain | Use `tmux has-session -t <name>` to check session existence | New — standard tmux idiom, exits 0 on hit, non-zero on miss | S:90 R:90 A:95 D:95 |
| 17 | Certain | Window-creation tmux invocation: `tmux new-window -c "$path" -n "$name"` (no `-d` flag → auto-focus) | New — tmux `new-window` defaults to focusing the new window unless `-d` is passed | S:90 R:85 A:90 D:90 |
| 18 | Certain | Session-creation: outside tmux uses `tmux new-session -s <s> -c <path> -n <w>` (foreground); inside tmux uses `-d` then `tmux switch-client -t <s>` | New — derived from intake #8 and tmux's nested-session prohibition | S:90 R:80 A:90 D:90 |
| 19 | Certain | Tests live in `bare_name_test.go` for binary hints and `shell_init_test.go` for shim emissions | Upgraded from intake #17 — confirmed by reading existing test layout | S:90 R:90 A:95 D:90 |
| 20 | Confident | Tmux is a runtime dependency only when shim users invoke `w`/`s` (lazy, no install-time check) | Confirmed from intake #15. Pattern match (`fzf`, `git`, `brew`). | S:80 R:80 A:85 D:85 |
| 21 | Confident | Window/session name passed to tmux as quoted argv (no shell interpolation) | Confirmed from intake #16. Tmux's argv handling is the validation boundary. | S:75 R:70 A:80 D:80 |
| 22 | Confident | Hint text wording: `wHint` references "tmux session"; `sHint` references "shim only"; both link to shell-init | New — derived from existing hint patterns (`bareNameHint`, `cdHint`); exact wording chosen for parallelism | S:80 R:90 A:85 D:80 |
| 23 | Confident | Integration tests deferred to manual smoke test in PR description (no tmux-in-CI harness) | New — practical constraint; tmux requires a tty. Documented as "Test plan" in PR. | S:75 R:85 A:85 D:80 |
| 24 | Confident | `_hop_dispatch w` / `s` arms read `$3`/`$4` for optional names; default `name="${3:-$1}"` (and `${4:-$1}` for `s` window) | New — standard POSIX parameter expansion; handles unset vs empty correctly | S:80 R:90 A:90 D:85 |
| 25 | Confident | Resolution failures (bad repo name) propagate via `command hop "$1" where` exit code; shim does not invoke tmux on resolution failure | New — derived from how existing `cd)` arm in `_hop_dispatch` handles resolution errors | S:80 R:85 A:90 D:85 |

25 assumptions (19 certain, 6 confident, 0 tentative, 0 unresolved).
