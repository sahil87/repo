# Spec: Add `hop push` subcommand

**Change**: 260508-rdgf-add-hop-push-subcommand
**Created**: 2026-05-09
**Affected memory**: `docs/memory/cli/subcommands.md`
**Intake**: [intake.md](intake.md)

## Non-Goals

- **Force-push, set-upstream, or other `git push` flags.** Users wanting these reach for `hop -R <name> git push --force` (tool-form via the shim, or canonical `hop -R <name> git push --force` directly). Constitution III (Convention Over Configuration) — convention is "push current branch upstream"; nuanced cases stay in `-R`.
- **Changing `hop pull` or `hop sync` behavior.** This change only adds `push`. The output sections, exit codes, and resolution rules of pull/sync are referenced for symmetry but not modified.
- **Pre-flight checks** (e.g. detecting an unpushed branch with no upstream and offering `--set-upstream`). git's own error message surfaces verbatim through `proc.RunCapture` — that is the contract for this change.
- **Worktree-aware behavior.** `hop push` runs `git push` in `r.Path`; if `r.Path` is a primary checkout that hosts worktrees elsewhere, those worktrees are not pushed. Same as `pull` and `sync`.
- **Stash management** before/after push. No state mutation beyond what `git push` itself does.

## CLI: `hop push` subcommand

### Requirement: Subcommand surface
The binary SHALL expose a `push` subcommand with the form `hop push [<name-or-group>] [--all]`. It SHALL accept at most one positional argument (`cobra.MaximumNArgs(1)`). The positional, when present, SHALL be a repo-name substring or a group name; resolution SHALL delegate to the existing `resolveTargets` helper used by `hop pull` and `hop sync`. The `--all` flag SHALL run the operation across every cloned repo in the registry. Tab completion SHALL use `completeRepoOrGroupNames`, identical to `pull`/`sync`.

#### Scenario: Single-repo push, success
- **GIVEN** `hop.yaml` resolves `outbox` uniquely and `<path>/.git` exists
- **WHEN** I run `hop push outbox`
- **THEN** `git push` runs in `<path>` via `proc.RunCapture` with a 10-minute timeout (`cloneTimeout`)
- **AND** stderr shows `push: outbox ✓ <last-non-empty-line-of-git-stdout>` (e.g., `push: outbox ✓ Everything up-to-date` or `push: outbox ✓ <src> -> <dst>`)
- **AND** stdout is empty
- **AND** exit code is 0

#### Scenario: Single-repo push, git-side failure
- **GIVEN** `hop.yaml` resolves `outbox` uniquely and `<path>/.git` exists
- **WHEN** I run `hop push outbox` and `git push` exits non-zero (e.g., non-fast-forward, no upstream)
- **THEN** git's own stderr is forwarded verbatim by `proc.RunCapture`
- **AND** stderr also shows `push: outbox ✗ <err>` (the hop-emitted summary line)
- **AND** stdout is empty
- **AND** exit code is 1 (`errSilent`)

#### Scenario: Single-repo not cloned
- **GIVEN** `hop.yaml` resolves `outbox` uniquely but `<path>/.git` does NOT exist
- **WHEN** I run `hop push outbox`
- **THEN** `git push` is NOT invoked
- **AND** stderr shows `skip: outbox not cloned`
- **AND** exit code is 1 (`errSilent`)

#### Scenario: Group-name push (batch)
- **GIVEN** `hop.yaml` defines a group `vendor` with three repos, two cloned and one not
- **WHEN** I run `hop push vendor`
- **THEN** the resolver returns the three group members in YAML source order
- **AND** the not-cloned repo emits `skip: <name> not cloned` and counts toward `skipped`
- **AND** the two cloned repos each emit `push: <name> ✓ <last-line>` (or `push: <name> ✗ <err>`) and count toward `pushed` or `failed`
- **AND** the final stderr line is `summary: pushed=N skipped=M failed=K`
- **AND** exit code is 0 if `failed == 0`, else 1

#### Scenario: `--all` push
- **GIVEN** `hop.yaml` lists 5 repos, all cloned, all pushable
- **WHEN** I run `hop push --all`
- **THEN** `git push` runs in each repo sequentially, in YAML source order
- **AND** stderr emits 5 `push: <name> ✓ <last-line>` lines
- **AND** the final stderr line is `summary: pushed=5 skipped=0 failed=0`
- **AND** exit code is 0

#### Scenario: Usage error — `--all` with positional
- **GIVEN** any registry state
- **WHEN** I run `hop push outbox --all`
- **THEN** stderr shows `hop push: --all conflicts with positional <name-or-group>`
- **AND** exit code is 2

#### Scenario: Usage error — neither positional nor `--all`
- **GIVEN** any registry state
- **WHEN** I run `hop push` (no args)
- **THEN** stderr shows `hop push: missing <name-or-group>. Pass a name, a group, or --all.`
- **AND** exit code is 2

#### Scenario: `git` missing on PATH (batch)
- **GIVEN** `git` is not on PATH and a batch resolution returns at least one cloned repo
- **WHEN** the first `git push` invocation returns `proc.ErrNotFound`
- **THEN** stderr emits `gitMissingHint` (`hop: git is not installed.`) once
- **AND** the batch aborts immediately — no further repos attempted
- **AND** no `summary:` line is emitted
- **AND** exit code is 1 (`errSilent`)

#### Scenario: `git` missing on PATH (single)
- **GIVEN** `git` is not on PATH, `<path>/.git` exists, and a single-repo resolution returned `outbox`
- **WHEN** I run `hop push outbox` and `git push` returns `proc.ErrNotFound`
- **THEN** stderr emits `gitMissingHint` once
- **AND** exit code is 1 (`errSilent`)

#### Scenario: Ambiguous substring match resolves via fzf
- **GIVEN** `hop.yaml` has repos `outbox` and `outbox-shared` both as cloned single repos
- **WHEN** I run `hop push outbox`
- **THEN** `resolveTargets` invokes fzf prefilled with `--query outbox` (per the existing match-or-fzf algorithm)
- **AND** if the user picks one, push proceeds in that repo (success: exit 0; failure: exit 1)
- **AND** if the user cancels (Esc), exit code is 130

#### Scenario: fzf missing
- **GIVEN** the resolver would need fzf (zero or 2+ substring matches) and fzf is not on PATH
- **WHEN** I run `hop push <ambiguous>`
- **THEN** stderr shows `fzfMissingHint` (`hop: fzf is not installed. ...`)
- **AND** exit code is 1 (`errSilent`)

### Requirement: Implementation reuse
The implementation SHALL reuse existing helpers — no duplication. Specifically:
- Argument resolution SHALL delegate to `resolveTargets(query, all)` (the same helper used by `pull` and `sync`).
- Single-repo not-cloned detection SHALL use `cloneState(r.Path)` and check `stateAlreadyCloned`.
- Batch iteration SHALL delegate to `runBatch(cmd, targets, "push", "pushed", pushOne)` — the same generic helper used by `pull` and `sync`.
- The success-line suffix SHALL be `lastNonEmptyLine(string(out))` (the helper already in `pull.go`).
- The per-call timeout SHALL be `cloneTimeout` (10 minutes, defined in `clone.go`).
- The fzf-missing hint, fzf-cancelled sentinel, and silent-error sentinel (`fzfMissingHint`, `errFzfCancelled`, `errSilent`) SHALL be the same constants used by `pull`/`sync`.

#### Scenario: No new shared helper extraction
- **GIVEN** the implementation file `src/cmd/hop/push.go` is added
- **WHEN** the change is reviewed
- **THEN** there is no new "common" or "git-op" abstraction layered between push and pull/sync
- **AND** the three subcommands remain three sibling files with parallel structure (`pull.go`, `push.go`, `sync.go`)

### Requirement: Output format and exit codes mirror `hop pull`
The per-repo stderr lines, the batch summary line, and the exit-code policy SHALL be identical in shape to `hop pull`, with `pull` replaced by `push` and `pulled` replaced by `pushed`.

#### Scenario: Per-repo lines
- **GIVEN** any push invocation
- **WHEN** a repo succeeds
- **THEN** stderr shows `push: <name> ✓ <last-line>`
- **WHEN** a repo fails
- **THEN** stderr shows `push: <name> ✗ <err>` AND git's own stderr is forwarded verbatim
- **WHEN** a repo is not cloned
- **THEN** stderr shows `skip: <name> not cloned`

#### Scenario: Batch summary
- **GIVEN** a batch invocation (group or `--all`) completes without `git` missing
- **WHEN** the iteration ends
- **THEN** the final stderr line is `summary: pushed=<N> skipped=<M> failed=<K>`

#### Scenario: Exit code mapping
- **GIVEN** the invocation has produced its output
- **WHEN** the cobra wrapper returns
- **THEN** exit code is one of:

  | Code | Trigger |
  |---|---|
  | 0 | Success (single push or batch with `failed == 0`) |
  | 1 | Single-repo not-cloned, single-repo failure, batch with `failed > 0`, `git` missing, or `fzf` missing |
  | 2 | Usage error (`--all` + positional, or both missing) |
  | 130 | fzf cancelled (Esc / Ctrl-C) during single-repo resolution |

### Requirement: Stdout/stderr discipline
All hop-emitted status output SHALL go to stderr. Stdout SHALL be empty for `hop push` (so the shim does not `cd` after the verb).

#### Scenario: Stdout is empty across all paths
- **GIVEN** any `hop push` invocation (success, failure, batch, single, usage error)
- **WHEN** the command completes
- **THEN** stdout is empty (no path, no summary, no diagnostics)
- **AND** all status, summary, and error lines are on stderr

## CLI: Root wiring and help

### Requirement: Subcommand registration
`newPushCmd()` SHALL be added to the root command's `AddCommand(...)` call in `src/cmd/hop/root.go`, between `newPullCmd()` and `newSyncCmd()` (alphabetical within the git-verb cluster: pull, push, sync). The factory function SHALL live in `src/cmd/hop/push.go`.

#### Scenario: `hop --help` lists push
- **WHEN** I run `hop --help`
- **THEN** the cobra-rendered help lists `push` in the available-commands section between `pull` and `sync`
- **AND** the short description is `Run 'git push' in a repo, group, or every cloned repo with --all`

#### Scenario: `hop push --help`
- **WHEN** I run `hop push --help`
- **THEN** stdout shows usage `push [<name-or-group>] [--all]` and the `--all` flag description (`run 'git push' in every cloned repo from hop.yaml`)
- **AND** exit code is 0

### Requirement: `rootLong` Usage block updates
The `Usage:` block in `src/cmd/hop/root.go::rootLong` SHALL include three new lines, inserted between the `pull` lines and the `sync` lines (preserving the pull-then-push-then-sync ordering):

```
  hop push <name>           Run 'git push' in the named repo
  hop push <group>          Run 'git push' in every cloned repo of <group>
  hop push --all            Run 'git push' in every cloned repo
```

The `Notes:` block SHALL change the existing `pull and sync accept ...` sentence to `pull, push, and sync accept ...` to keep the note accurate.

#### Scenario: Usage block ordering
- **GIVEN** `rootLong` is rendered
- **WHEN** I read the `Usage:` enumeration
- **THEN** the three `hop push` lines appear after the three `hop pull` lines and before the three `hop sync` lines

### Requirement: Shell shim subcommand alternation
The `posixInit` shell function emitted by `hop shell-init zsh|bash` SHALL include `push` in the known-subcommand alternation so that `hop push <name>` is dispatched as a subcommand (rule 3) and not misrouted into tool-form (rule 5). The exact change SHALL be:

```sh
# before
clone|pull|sync|ls|shell-init|config|update|help|--help|-h|--version|completion)
# after
clone|pull|push|sync|ls|shell-init|config|update|help|--help|-h|--version|completion)
```

#### Scenario: Shim routes `hop push` as subcommand
- **GIVEN** the user has run `eval "$(hop shell-init zsh)"`
- **WHEN** they run `hop push outbox`
- **THEN** the shim's rule-3 branch matches (`push` is a known subcommand)
- **AND** the call is dispatched via `_hop_dispatch "$@"`
- **AND** the binary's `push` subcommand RunE handles the request — NOT the tool-form `command hop -R push outbox` path

#### Scenario: Without the shim alternation update (negative)
- **GIVEN** a hypothetical shim that does NOT include `push` in the alternation
- **WHEN** the user runs `hop push outbox` under that shim
- **THEN** rule 5 fires (`$1 = "push"` is treated as a repo name)
- **AND** the shim runs `command hop -R push outbox`
- **AND** the binary errors with `hop: -R: 'push' not found.` (since `push` is not on PATH)

> This negative scenario documents why the alternation update is mandatory; it is not a behavior the implementation should produce.

## Memory: documentation updates

### Requirement: Inventory row for `hop push`
`docs/memory/cli/subcommands.md` SHALL gain a new row in the **Inventory** table positioned between the `hop pull` row and the `hop sync` row. The row SHALL describe `hop push [<name-or-group>] [--all]` with the same level of detail as the existing pull row (file reference, args summary, behavior summary, exit codes, where the resolver lives).

#### Scenario: Inventory ordering and detail
- **GIVEN** the updated `subcommands.md`
- **WHEN** I read the **Inventory** table
- **THEN** there are three sibling rows in this order: `hop pull`, `hop push`, `hop sync`
- **AND** the push row references `push.go`, `cobra.MaximumNArgs(1)`, `proc.RunCapture`, `cloneTimeout`, the per-repo and summary line formats, exit codes 0/1/2/130, and the `gitMissingHint` early-abort behavior

### Requirement: Combined per-line output section
The existing `## hop pull / hop sync per-line output` section SHALL be renamed to `## hop pull / hop push / hop sync per-line output`. The section body SHALL be extended to enumerate push's success and failure lines (`push: <name> ✓ <last-line>`, `push: <name> ✗ <err>`) and the push summary form (`summary: pushed=N skipped=M failed=K`). The skip line (`skip: <name> not cloned`) is shared and SHALL NOT be duplicated.

#### Scenario: Section title and content
- **GIVEN** the updated `subcommands.md`
- **WHEN** I read the section now titled `## hop pull / hop push / hop sync per-line output`
- **THEN** the success bullet enumerates pull's, push's, and sync's success forms
- **AND** the failure bullets enumerate pull's `pull: ✗`, push's `push: ✗`, sync's three forms (`sync: ✗ <err>`, rebase-conflict, push-failed)
- **AND** the summary bullet enumerates all three `summary:` shapes
- **AND** the timeout note (per-call 10-minute `cloneTimeout`) covers all three

### Requirement: Shim alternation note
The narrative paragraph in `docs/memory/cli/subcommands.md` that lists the shim's known-subcommand alternation (`clone|pull|sync|ls|...`) SHALL be updated to include `push`, with the rationale "without push here, rule 5 misroutes `hop push <name>` into tool-form" stated explicitly (mirroring the existing pull/sync explanation).

#### Scenario: Narrative update
- **GIVEN** the updated `subcommands.md`
- **WHEN** I read the shim-resolution-ladder narrative
- **THEN** the listed alternation includes `push` between `pull` and `sync`
- **AND** the explanation cites the same misroute reasoning that exists for pull/sync

### Requirement: Spec inventory update
`docs/specs/cli-surface.md` SHALL gain a `hop push` row in the **Subcommand Inventory** table, and the `Usage:` block enumeration in the **Help Text** section SHALL include the three `hop push` lines in their pull-then-push-then-sync position. The narrative reference to "`pull` and `sync` accept ..." SHALL be updated to "`pull`, `push`, and `sync` accept ...".

#### Scenario: Spec inventory completeness
- **GIVEN** the updated `cli-surface.md`
- **WHEN** I read the **Subcommand Inventory** table
- **THEN** there is a row for `hop push [<name>] | <group> | --all` describing the behavior summary and the exit-code policy
- **AND** the row sits between the existing pull and sync rows

## Design Decisions

1. **`push` is a top-level subcommand, not a flag on `pull`/`sync`.**
   - *Why*: Push is its own git verb. Symmetric with `hop pull` (and matches user analogy from the request). Constitution VI's "could this be a flag" test fails: `--no-pull` on sync would fight the verb, `--push-after` on pull would surprise. A new subcommand earns the surface-area cost; a flag would be a worse design.
   - *Rejected*: `hop sync --no-pull` (overloads sync, fights its name); `hop pull --push-after` (mixes verbs, hides intent); deferring to `hop -R name git push` (works but breaks group/`--all` symmetry — and the user explicitly asked for the `hop push <repo>` form).

2. **No `--force`, no `--set-upstream`, no extra flags beyond `--all`.**
   - *Why*: Constitution III (Convention Over Configuration). Convention is "push current branch to its tracked upstream"; nuanced cases (force, new upstream) are rare per-repo decisions that don't generalize across a group/`--all` batch. Users wanting them reach for `hop -R name git push --force` (single-repo, intentional). Adding flags now creates one-of-many cases; we can add later if real demand emerges.
   - *Rejected*: `--force-with-lease`, `--set-upstream-to=...` — both are decisions per-repo, not per-batch. Adding now invites scope creep and subtly different semantics across a group.

3. **Reuse `runBatch` and `resolveTargets` rather than extracting a "git-op" abstraction.**
   - *Why*: Constitution IV (Wrap, Don't Reinvent) and the project's "no premature abstraction" rule. `runBatch` is already generic over a `batchOp`; `resolveTargets` is already shared between pull and sync. A third caller does not justify a new abstraction layer.
   - *Rejected*: A new `internal/gitop` package wrapping `git push|pull|sync` — pure speculation, no current consumer beyond the three subcommands; would obscure the per-command differences (e.g., sync's two-context rebase-then-push) without simplifying anything.

4. **All output to stderr; stdout empty.**
   - *Why*: Symmetric with `pull` and `sync`. Stdout is reserved for path-emission (consumed by the shim's `cd`); push is not a path-emitter, so stdout MUST stay empty. This also keeps the shim's resolution ladder from accidentally consuming push output.
   - *Rejected*: Echoing the resolved path or summary on stdout — would invite shim breakage and confuses the contract.

5. **Per-call 10-minute timeout (`cloneTimeout`), not a longer push-specific budget.**
   - *Why*: Push and pull are both bandwidth-bound network operations against the same set of remotes. The existing 10-minute budget already covers worst-case clones; pushes are at most as slow. Reusing the constant keeps configuration surface zero.
   - *Rejected*: A separate `pushTimeout` or `gitOpTimeout` constant — premature; nothing in the codebase suggests push needs different bounds.

6. **Help-text and inventory ordering: pull → push → sync.**
   - *Why*: "Pull, push, sync" reads naturally (it's how git docs and most muscle memory order them — fetch, push, sync-as-rebase-and-push). Trivially reversible if review prefers another order.
   - *Rejected*: Append after sync (chronological "complexity" order: pull, sync, push) — less natural; reads as "we tacked this on later."

## Clarifications

### Session 2026-05-09 (auto mode)

| # | Action | Detail |
|---|--------|--------|
| 19 | Upgraded Confident → Certain | Verified `docs/specs/cli-surface.md` contains both `## Subcommand Inventory` and `### Help Text` sections — both updates required. |
| 20 | Upgraded Confident → Certain | Verified `pull.go:28` and `sync.go:30` both use `ValidArgsFunction: completeRepoOrGroupNames` — push reuses with no plumbing change. |

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Subcommand name is `push` | User said it verbatim; matches git verb. Confirmed from intake #1. | S:100 R:90 A:95 D:100 |
| 2 | Certain | Signature `hop push [<name-or-group>] [--all]`, `cobra.MaximumNArgs(1)` | Mirrors pull/sync exactly. Confirmed from intake #2. | S:95 R:85 A:95 D:95 |
| 3 | Certain | Reuses `runBatch` and `resolveTargets`; no new abstraction | Constitution IV; helpers are already generic. Confirmed from intake #3. Reinforced as Design Decision #3. | S:95 R:90 A:100 D:100 |
| 4 | Certain | Implementation in new `src/cmd/hop/push.go` modeled on `pull.go` | Project layout convention. Confirmed from intake #4. | S:95 R:90 A:100 D:100 |
| 5 | Certain | Per-call timeout = `cloneTimeout` (10 minutes) | Pull/sync use this; push is bandwidth-bound the same way. Confirmed from intake #5; Design Decision #5. | S:90 R:85 A:95 D:95 |
| 6 | Certain | All output → stderr; stdout empty | Pull/sync follow this rule; shim contract requires it. Confirmed from intake #6; Design Decision #4. | S:100 R:90 A:100 D:100 |
| 7 | Certain | Shell shim's known-subcommand alternation includes `push` between `pull` and `sync` | #17 lesson; without it the shim misroutes. Confirmed from intake #7. Tested by negative scenario in spec. | S:100 R:80 A:100 D:100 |
| 8 | Certain | `git` missing aborts batch immediately, single emission of `gitMissingHint`, no summary | `runBatch` already encodes this; push inherits. Confirmed from intake #8. | S:100 R:90 A:100 D:100 |
| 9 | Certain | Success suffix uses `lastNonEmptyLine(stdout)` | Same helper used by pull/sync; git push's terminal output is meaningful. Confirmed from intake #9. | S:95 R:85 A:95 D:95 |
| 10 | Certain | Exit codes mirror pull (0/1/2/130) | Symmetric verbs → symmetric policy. Confirmed from intake #10. | S:100 R:85 A:100 D:100 |
| 11 | Certain | No `--force`, no `--set-upstream`, no extra flags | Constitution III; nuanced flags don't generalize across batches. Confirmed from intake #11; Design Decision #2. | S:90 R:80 A:90 D:90 |
| 12 | Certain | Memory updates land in `cli/subcommands.md` and `docs/specs/cli-surface.md`; no new memory file | Pull/sync didn't get one in #17. Upgraded from intake Confident #12 — confirmed by re-reading both files; the existing structure cleanly accommodates the additions. | S:90 R:75 A:95 D:90 |
| 13 | Certain | Help-text Usage block ordering: pull, push, sync | Design Decision #6 — natural reading order. Upgraded from intake Confident #13. | S:85 R:95 A:90 D:90 |
| 14 | Certain | Combined memory section title: "hop pull / hop push / hop sync per-line output" | Output formats are nearly identical; combining keeps the doc compact. Upgraded from intake Confident #14. | S:85 R:95 A:90 D:90 |
| 15 | Certain | Tests mirror `pull_test.go` test-by-test; no extra coverage of shared helpers | Shared helpers already tested via pull/sync; duplication is waste. Upgraded from intake Confident #15. | S:90 R:85 A:95 D:90 |
| 16 | Certain | Subcommand registration order in `root.go::AddCommand`: between `newPullCmd()` and `newSyncCmd()` | Matches help-text and inventory ordering. New for spec — derived from Design Decision #6. | S:90 R:95 A:95 D:90 |
| 17 | Certain | Single-repo not-cloned uses `cloneState(r.Path) != stateAlreadyCloned`, returning `errSilent` after `skip: <name> not cloned` | Mirrors `pullSingle`. New for spec — explicit so the apply stage doesn't reinvent it. | S:95 R:85 A:100 D:100 |
| 18 | Certain | The `Notes:` block sentence "`pull` and `sync` accept ..." becomes "`pull`, `push`, and `sync` accept ..." | Keeps the note accurate after push lands. New for spec — caught while drafting `rootLong` updates. | S:95 R:95 A:95 D:90 |
| 19 | Certain | The `cli-surface.md` Usage Block enumeration in the Help Text section is updated alongside the Subcommand Inventory table | <!-- clarified: verified `docs/specs/cli-surface.md` contains `## Subcommand Inventory` (line 6) and `### Help Text` (line 411); both enumerations must stay in sync. --> Two separate enumerations in the same file; both must be kept in sync. Confirmed by reading `docs/specs/cli-surface.md`. | S:95 R:90 A:95 D:90 |
| 20 | Certain | No tab-completion regression — `completeRepoOrGroupNames` already handles names+groups for pull/sync; push reuses it directly with no new completion plumbing | <!-- clarified: verified `pull.go:28` and `sync.go:30` both use `ValidArgsFunction: completeRepoOrGroupNames`; reuse for push is purely cobra factory wiring with no plumbing change. --> Same registration pattern; tab completion is a property of the cobra factory, not the command's Run. Confirmed by reading `pull.go` and `sync.go` cobra wiring. | S:95 R:85 A:100 D:95 |

20 assumptions (20 certain, 0 confident, 0 tentative, 0 unresolved).
