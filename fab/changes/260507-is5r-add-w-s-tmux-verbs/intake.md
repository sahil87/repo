# Intake: Add `w` and `s` tmux verbs

**Change**: 260507-is5r-add-w-s-tmux-verbs
**Created**: 2026-05-07
**Status**: Draft

## Origin

Conversational `/fab-discuss` session. User started from a workflow question:

> I am in runkit (think tmux), in a window, in a session. I want to type "h outbox <something>" so that a new window / tab opens up cd'ed into that repo. What's the right command for this?

The discussion explored three resolution strategies for overloading `$2` (reserved-word table in shim, binary-owns-dispatch, sigil prefix). The user chose **reserved-word table at $2** with single-letter verbs, framed as "command 1st priority, system tool 2nd."

Refinement converged on two verbs:

- `w` — new tmux **w**indow, cwd = repo
- `s` — new tmux **s**ession + window, cwd = repo

Mid-discussion, the matte-butte branch was rebased onto origin/main, which had merged PR #14 — the v0.x repo-verb grammar flip. That flip materially changed the landscape: the reserved-word-at-$2 mechanism the user proposed is no longer hypothetical, it's the **established pattern** (`cd` and `where` already live there). Adding `w`/`s` extends a known mechanism rather than introducing a new one.

User also confirmed:
- Cobra's generic `accepts at most 2 arg(s)` error is acceptable for direct binary invocations of the 3-arg form (`hop outbox w api`). The shim handles 3-arg forms; the binary only ever emits hints for the 2-arg form.
- The shim calling `tmux` directly (not via `internal/proc`) is acceptable — `internal/proc` is a binary-side abstraction; the shim is the layer that talks to tmux.

## Why

**Problem**: Hop today routes the user into the *current* shell (`h outbox` → cd in place) or execs a tool in the *current* pane (`h outbox vim`). Neither produces a fresh tmux window or session anchored at the repo. Users working in tmux/runkit who want a new pane/window/session for a different repo have to type a multi-step sequence: `tmux new-window -c "$(hop outbox where)"` or similar. This breaks hop's value prop ("the locator just works").

**Consequence if not fixed**: The friction stays. Users either build personal aliases on top of hop (fragmenting the UX) or stay in fewer windows than they'd like and lose context-switching ergonomics. Hop's positioning as "the canonical way to navigate to a repo" weakens the moment the navigation target is anything other than $PWD.

**Why this approach over alternatives**: Three resolution strategies were considered for adding new verbs at `$2`:

1. **Reserved-word table in shim** (chosen) — Tiny, stable allowlist at `$2`. Already the pattern post-PR #14. Single-letter verbs minimize PATH-name shadowing. Trades: shadows `w` and `s` as tool-form names forever; escape hatch is `-R`.
2. **Binary owns dispatch** (rejected) — Pulls the binary into terminal-multiplexer territory, violating Constitution VI (Minimal Surface Area). Also can't mutate the parent shell from a child process, so window/session creation can't fully live in the binary.
3. **Sigil prefix** (rejected) — `h outbox :w` would be unambiguous but ugly, and doesn't honor the "command 1st" priority — it just makes verbs and tools disjoint instead of prioritized.

Within (1), single-letter verbs were chosen over wordlists (`window`, `session`) because:
- Collision risk with PATH binaries reachable via tool-form is near zero (no one runs `h <repo> w` expecting a binary called `w`)
- Visually distinguish hop verbs from tool names at a glance
- Match the existing `h` / `hi` one-letter-alias family in `shell-init`

## What Changes

### Verb 1: `w` — new tmux window

**User-facing forms** (shim-only):

```
h outbox w              # new window in current session, cwd = outbox repo, named "outbox"
h outbox w api          # new window in current session, cwd = outbox repo, named "api"
```

**Behavior matrix**:

| Location | Command | Result |
|---|---|---|
| Inside tmux (`$TMUX` set) | `h outbox w` | `tmux new-window -c <path> -n outbox` then switch focus to it. Window is named the repo name by default. |
| Inside tmux | `h outbox w api` | `tmux new-window -c <path> -n api`, switch focus |
| Outside tmux (`$TMUX` unset) | `h outbox w` | Error to stderr: `hop: 'w' requires an active tmux session. Use 'h <name> s' to start one.` Exit 1. |
| Outside tmux | `h outbox w api` | Same error as above |

**Default window name** when `$3` is omitted: the repo name (`outbox`). Tmux allows duplicate window names; we do not auto-dedup. If the user opens `h outbox w` twice, they get two windows both named `outbox` (distinguishable by tmux's index).

### Verb 2: `s` — new tmux session

**User-facing forms** (shim-only):

```
h outbox s                      # new session named "outbox" with one window named "outbox", cwd = outbox repo
h outbox s outbox-dev           # new session "outbox-dev" with window "outbox", cwd = outbox repo
h outbox s outbox-dev api       # new session "outbox-dev" with window "api", cwd = outbox repo
```

**Behavior matrix**:

| Location | Command | Result |
|---|---|---|
| Inside tmux | `h outbox s [...]` | `tmux new-session -d -s <session> -c <path> -n <window>` then `tmux switch-client -t <session>`. Cannot `attach` from inside tmux (nested-tmux footgun). |
| Outside tmux | `h outbox s [...]` | `tmux new-session -s <session> -c <path> -n <window>` (foreground attach). The shell that ran `h ...` becomes the tmux client. |
| Either | session name already exists | Error to stderr: `hop: tmux session 'outbox-dev' already exists. Use 'h <name> w' to add a window in the current session.` Exit 1. No silent attach. |

**Defaults**:
- Session name (`$3` omitted) = repo name
- Window name (`$4` omitted) = repo name

So `h outbox s` is shorthand for `h outbox s outbox outbox`.

**Asymmetry with `w`**: For `w`, the first optional arg names the window. For `s`, the first optional arg names the session, and the *second* optional arg names the window. This is intentional: each verb's first positional names the primary thing the verb creates. Documented in shell-init help.

### Binary changes

**Hint constants** (added next to `cdHint` in `src/cmd/hop/root.go`):

```go
wHint = "hop: 'w' is shell-only (tmux window). " +
        "Add 'eval \"$(hop shell-init zsh)\"' to your zshrc and run inside a tmux session."

sHint = "hop: 's' is shell-only (tmux session). " +
        "Add 'eval \"$(hop shell-init zsh)\"' to your zshrc."
```

**RunE branches** (root.go, 2-arg case):

```go
case "w":
    return errExitCode{code: 2, msg: wHint}
case "s":
    return errExitCode{code: 2, msg: sHint}
```

These run only when a user invokes the binary directly (`/path/to/hop outbox w`) without the shim. The shim never reaches the binary for `w`/`s` — it handles them entirely shell-side.

**`toolFormHintFmt` update** (root.go):

```go
toolFormHintFmt = "hop: '%s' is not a hop verb (cd, where, w, s). " +
                  "For tool-form, install the shim: eval \"$(hop shell-init zsh)\", " +
                  "or use: hop -R \"<name>\" <tool> [args...]"
```

The enumerated verb list grows from `(cd, where)` to `(cd, where, w, s)`.

**3-arg invocations on the binary** (e.g., `/path/to/hop outbox w api`) hit cobra's `MaximumNArgs(2)` and produce the generic `accepts at most 2 arg(s), received 3` error. Acceptable: shim handles 3-arg forms; binary 3-arg invocations are an edge case for scripts/CI bypassing the shim.

### Shim changes (`src/cmd/hop/shell_init.go::posixInit`)

Two new branches in rule 5 of the dispatch ladder, between `$2 == "where"` and `$2 == "-R"`:

```sh
elif [[ "$2" == "w" ]]; then
    _hop_dispatch w "$1" "${@:3}"
elif [[ "$2" == "s" ]]; then
    _hop_dispatch s "$1" "${@:3}"
```

`_hop_dispatch` gains two arms (`w)` and `s)`) that:
1. Resolve the repo path: `path=$(command hop "$1" where)` — exits non-zero on resolution failure (no further work)
2. Determine the window/session name (default to `$1` if `$3`/`$4` not given)
3. Check `$TMUX` to branch on inside-vs-outside-tmux
4. For `w`: error if `$TMUX` is unset; otherwise `tmux new-window -c "$path" -n "$name"` (tmux auto-focuses new windows by default in most configs; the `-d` flag is what suppresses focus, so omitting it gives us focus)
5. For `s`: check session existence with `tmux has-session -t "$session" 2>/dev/null` — if found, print the "already exists" hint and return 1; otherwise create. If `$TMUX` is unset, `tmux new-session -s ... -c ... -n ...` (foreground attach). If `$TMUX` is set, `tmux new-session -d -s ... -c ... -n ...` then `tmux switch-client -t "$session"`.

All tmux invocations go through the shell (no `internal/proc` involvement) — consistent with how the shim already calls `cd` directly without binary mediation.

### Tests

**Binary** (`src/cmd/hop/bare_name_test.go` or new `verbs_test.go`):
- `TestVerbW_BinaryFormPrintsHint` — 2-arg `hop outbox w` → exit 2, exact stderr matches `wHint`, empty stdout
- `TestVerbS_BinaryFormPrintsHint` — 2-arg `hop outbox s` → exit 2, exact stderr matches `sHint`, empty stdout
- `TestToolFormHintEnumeratesAllVerbs` — `hop outbox notreal` → stderr contains `(cd, where, w, s)`

**Shim** (`src/cmd/hop/shell_init_test.go`):
- `TestShellInitZshContainsWBranch` — emitted shell contains `[[ "$2" == "w" ]]`
- `TestShellInitZshContainsSBranch` — emitted shell contains `[[ "$2" == "s" ]]`
- `TestShellInitZshDispatchHasWArm` — `_hop_dispatch` switch has a `w)` arm
- `TestShellInitZshDispatchHasSArm` — `_hop_dispatch` switch has an `s)` arm

**Integration** (defer to manual smoke test in PR description; we don't have a tmux-in-CI harness):
- The four-cell matrix above (in/out × w/s)
- "session exists" error path
- Default-name resolution

### Docs

- `docs/specs/cli-surface.md` — add `w` and `s` rows to subcommand inventory; new section on tmux-aware verbs; update Design Decisions to record the single-letter rationale and the four-cell behavior matrix
- `docs/memory/cli/subcommands.md` — add `w` and `s` rows; update the rule-5 dispatch ladder narrative; update the "Tool-form dispatch" table to show the new verb branches; document the `wHint`/`sHint` constants
- `README.md` — add tmux-aware navigation example to the "First run" or "Usage" section

## Affected Memory

- `cli/subcommands`: (modify) Add `w` and `s` rows to the `Inventory` table. Update the rule-5 dispatch ladder narrative in the `posixInit` section to document the new `$2 == "w"` / `$2 == "s"` branches and the `_hop_dispatch w)` / `s)` arms. Update the "Tool-form dispatch" examples table with new verb-vs-tool resolution rows. Update the "Binary-form hint texts" section with `wHint` / `sHint`. Update the `toolFormHintFmt` text to enumerate `(cd, where, w, s)`.

## Impact

**Code areas**:
- `src/cmd/hop/root.go` — new hint constants, new RunE branches, updated `toolFormHintFmt`
- `src/cmd/hop/shell_init.go` — new shim branches and `_hop_dispatch` arms
- `src/cmd/hop/bare_name_test.go` (or new file) — binary-side hint tests
- `src/cmd/hop/shell_init_test.go` — shim emission tests

**APIs**: No Go API changes. The `repos` package is read-only here (we resolve via existing `command hop "$1" where`).

**Dependencies**: New runtime dependency on `tmux` — but only for shim users invoking `w`/`s`. Hop binary itself remains tmux-free. Match the lazy-tool pattern (`fzf`, `git`, `brew`): no install-time check; failure surfaces only when the verb is invoked.

**Constitution check**:
- Principle I (Security First): Tmux invocations go through the shell. The shim already passes user-provided repo names through `command hop "$1" where` (binary handles validation). Window/session names from `$3`/`$4` are passed to tmux as quoted argv — no shell interpolation. Acceptable.
- Principle II (No Database): No state added. Window/session existence is queried from tmux at request time.
- Principle III (Convention Over Configuration): Default name = repo name. Zero new flags.
- Principle IV (Wrap, Don't Reinvent): Shell out to `tmux`, don't reimplement.
- Principle V (Thin Justfile): No build changes.
- Principle VI (Minimal Surface Area): Two new verbs at `$2`, slotted into an existing reserved-word mechanism. Justification: replaces a multi-step manual sequence with a one-token gesture; no plausible flag on an existing verb suffices (the verbs differ in *kind*, not in modifier).

**Cross-platform**: tmux is POSIX-only. The shim itself is POSIX-only (zsh+bash). No darwin/linux split.

## Open Questions

- (none — all design decisions resolved during `/fab-discuss`)

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Single-letter verbs `w` and `s` (not wordlists, not sigils) | Discussed — user chose explicitly. Matches existing one-letter alias family (`h`, `hi`). | S:95 R:80 A:85 D:90 |
| 2 | Certain | Reserved-word-at-$2 mechanism (extends existing `cd`/`where` slot) | Discussed — chosen over binary-dispatch and sigil-prefix alternatives. Already the established post-PR-#14 pattern. | S:95 R:75 A:90 D:95 |
| 3 | Certain | Both verbs are shim-only; binary just emits hints | Discussed — user confirmed. Binary cannot mutate parent shell or tmux state from a child process. Mirrors `cd` verb's shell-only nature. | S:95 R:80 A:90 D:95 |
| 4 | Certain | Default window/session name = repo name when not specified | Discussed — chosen for predictability. Tmux allows duplicate names; no auto-dedup needed. | S:90 R:90 A:85 D:90 |
| 5 | Certain | Always focus the new window/session | Discussed — user confirmed. Defeats the ergonomic goal otherwise. No `-d` flag until asked. | S:90 R:90 A:85 D:95 |
| 6 | Certain | `s` errors if session already exists (no silent attach) | Discussed — preserves the `s` vs `w` semantic distinction. User can be explicit. | S:90 R:80 A:80 D:85 |
| 7 | Certain | `w` outside tmux → error, hint to use `s` | Discussed — `tmux new-window` requires a session. Symmetric guidance. | S:90 R:90 A:90 D:95 |
| 8 | Certain | `s` outside tmux → foreground attach; `s` inside tmux → switch-client | Discussed — nested-tmux is forbidden. Matches tmux's own model. | S:90 R:75 A:90 D:90 |
| 9 | Certain | Cobra `MaximumNArgs(2)` stays — generic error for direct 3-arg binary calls | Discussed — user confirmed. Shim handles 3-arg forms; binary 3-arg invocation is a script/CI edge case. | S:90 R:75 A:80 D:90 |
| 10 | Certain | Tmux invocations go through shell, not `internal/proc` | Discussed — user confirmed. `internal/proc` is a binary-side abstraction; the shim is the tmux-aware layer. | S:90 R:70 A:90 D:90 |
| 11 | Certain | Binary `wHint`/`sHint` constants live next to `cdHint` in `root.go` | Pattern match — every other verb hint is in `root.go`. | S:90 R:90 A:95 D:95 |
| 12 | Certain | `toolFormHintFmt` updated to enumerate `(cd, where, w, s)` | Pattern match — current text enumerates known verbs explicitly. | S:90 R:95 A:95 D:95 |
| 13 | Certain | Asymmetric positionals: `w [window]`, `s [session] [window]` | Discussed — accepted. Rule "first optional arg names the primary thing the verb creates" is consistent and learnable. | S:85 R:75 A:80 D:80 |
| 14 | Certain | Out of scope: run-command-in-window, `-d` flag, pane-split verb, auto-dedup | Discussed — explicitly deferred. Add only if a real second use case arises (Constitution VI). | S:90 R:85 A:80 D:90 |
| 15 | Confident | Tmux is a runtime dependency only when shim users invoke `w`/`s` (lazy check) | Pattern match — same as `fzf`, `git`, `brew`. No install-time check; failure surfaces at invocation. | S:80 R:80 A:85 D:85 |
| 16 | Confident | Shim layer name validation: pass `$3`/`$4` through to tmux as quoted argv (no shell interpolation) | Constitution Principle I (Security First) — but tmux's own argv handling is the validation boundary. Acceptable for v1. | S:75 R:70 A:80 D:80 |
| 17 | Confident | New tests live in `bare_name_test.go` (binary) and `shell_init_test.go` (shim) | Pattern match — existing verb tests live there. Could split to `verbs_test.go` if file grows large. | S:80 R:90 A:90 D:80 |

17 assumptions (14 certain, 3 confident, 0 tentative, 0 unresolved).
