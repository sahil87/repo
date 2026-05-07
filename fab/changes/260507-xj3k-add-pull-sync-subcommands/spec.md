# Spec: Add `hop pull` and `hop sync` Subcommands

**Change**: 260507-xj3k-add-pull-sync-subcommands
**Created**: 2026-05-07
**Affected memory**: `docs/memory/cli/subcommands.md`, `docs/memory/cli/match-resolution.md`, `docs/memory/architecture/package-layout.md`

## Non-Goals

- `--continue` flag for resumable batches — deferred per intake assumption #14; persisted batch state would violate Constitution II (no database/cache), and re-running `pull --all` is cheap (already-up-to-date is fast).
- `--no-fetch` flag on `pull` — deferred per intake assumption #12; the niche workflow is covered by `hop <name> -R git merge FETCH_HEAD` (Constitution VI minimal surface area).
- Concurrent batch execution — sequential v1 per intake assumption #7; predictable output ordering matters more than wall-clock for small registries; concurrency is a follow-up.
- Auto-stash, auto-resolve, merge fallback for `sync` — verbatim git errors per intake assumption #4; Constitution IV (wrap, don't reinvent) — the user is expected to commit/stash/resolve.
- Force-push on `sync` push step — non-fast-forward rejections surface verbatim; the user must manually `-R git push --force-with-lease` if needed.
- New top-level `hop where`/`hop cd` etc. — `pull`/`sync` join the existing top-level subcommand inventory; no other grammar changes.
- Resurrecting the `hop open` / `hop code` precedent — `pull`/`sync` are NOT thin tool wrappers (sync composes `git pull --rebase` + `git push`; both add batch resolution and cross-repo summary), so they fit the registry-verb category alongside `hop clone`, not the tool-form category.

## CLI Surface: New Top-Level Subcommands

### Requirement: `hop pull` Subcommand

The binary SHALL expose `hop pull [<name-or-group>] [--all]` as a top-level subcommand that wraps `git pull` over a single repo, every cloned repo in a named group, or every cloned repo in the registry.

The signature MUST be:

```
hop pull [<name-or-group>] [--all]
```

The `--all` flag MUST be a boolean. No other flags are added in v1.

#### Scenario: Single-repo pull resolves uniquely

- **GIVEN** `hop.yaml` lists exactly one repo whose `Name` is `outbox` and `<path>/.git` exists
- **WHEN** I run `hop pull outbox`
- **THEN** the binary runs `git -C <path> pull` via `internal/proc.RunCapture`
- **AND** stdout is empty
- **AND** stderr emits one status line: `pull: outbox ✓ <git stdout last line>`
- **AND** exit code is 0

#### Scenario: Group-name positional pulls every cloned repo in the group

- **GIVEN** `hop.yaml` defines a group `default` containing 3 repos, 2 of them cloned and 1 missing
- **WHEN** I run `hop pull default`
- **THEN** the binary iterates the 3 repos in YAML source order
- **AND** the 2 cloned repos run `git -C <path> pull` each
- **AND** the missing repo emits `skip: <name> not cloned` to stderr
- **AND** the final stderr line is `summary: pulled=2 skipped=1 failed=0`
- **AND** exit code is 0

#### Scenario: `--all` overrides positional and pulls the full registry

- **GIVEN** `hop.yaml` has 5 repos, all cloned
- **WHEN** I run `hop pull --all`
- **THEN** the binary iterates all 5 repos in YAML source order
- **AND** each repo runs `git -C <path> pull`
- **AND** the final stderr line is `summary: pulled=5 skipped=0 failed=0`
- **AND** exit code is 0

#### Scenario: Missing positional and missing `--all` is a usage error

- **GIVEN** the user invokes `hop pull` with no positional and no `--all`
- **WHEN** the command runs
- **THEN** stderr shows `hop pull: missing <name-or-group>. Pass a name, a group, or --all.`
- **AND** exit code is 2

#### Scenario: `--all` combined with positional is a usage error

- **GIVEN** the user invokes `hop pull foo --all`
- **WHEN** the command runs
- **THEN** stderr shows `hop pull: --all conflicts with positional <name-or-group>`
- **AND** exit code is 2

### Requirement: `hop sync` Subcommand

The binary SHALL expose `hop sync [<name-or-group>] [--all]` as a top-level subcommand that wraps `git pull --rebase` then `git push` over a single repo, every cloned repo in a named group, or every cloned repo in the registry. The signature, flag set, and resolution rules MUST mirror `hop pull` exactly.

#### Scenario: Single-repo sync runs pull-rebase then push

- **GIVEN** `hop.yaml` lists `outbox` cloned at `<path>` with a clean working tree, an upstream tracking branch, and one local commit ahead of the upstream
- **WHEN** I run `hop sync outbox`
- **THEN** the binary first runs `git -C <path> pull --rebase`
- **AND** on success, runs `git -C <path> push`
- **AND** stderr emits `sync: outbox ✓ <pull-summary> <push-summary>`
- **AND** exit code is 0

#### Scenario: Sync with no commits to push reports success

- **GIVEN** `outbox` is up to date with origin and has no local commits
- **WHEN** I run `hop sync outbox`
- **THEN** `git pull --rebase` succeeds
- **AND** `git push` returns "Everything up-to-date"
- **AND** stderr emits `sync: outbox ✓ <pull-summary> <push-summary>`
- **AND** exit code is 0

### Requirement: Argument Resolution Order

For both `pull` and `sync`, the positional argument MUST be resolved in this order; the first match wins:

1. If `--all` is passed, the positional MUST be rejected (usage error, exit 2).
2. If the positional matches a **group name** in `hop.yaml` exactly (case-sensitive), the target set is every URL in that group whose `<path>/.git` exists.
3. Otherwise, the positional is treated as a **repo name** and resolved via the standard match-or-fzf algorithm (`internal/repos.MatchOne` case-insensitive substring on `Name`, with fzf for ambiguous/zero matches).
4. If positional is omitted AND `--all` is absent, the command MUST exit 2 with a usage error (no implicit picker, no implicit `--all`).

#### Scenario: Group name beats substring repo name

- **GIVEN** `hop.yaml` defines a group named `tools` AND has a repo named `tools-shared`
- **WHEN** I run `hop pull tools`
- **THEN** the resolver matches the group `tools` (rule 2) and pulls every cloned repo in that group
- **AND** the repo `tools-shared` is NOT included unless it is itself a member of the `tools` group

#### Scenario: Substring repo match falls through when no group matches

- **GIVEN** `hop.yaml` has no group named `outb`, but exactly one repo whose `Name` contains `outb` (`outbox`)
- **WHEN** I run `hop pull outb`
- **THEN** rule 2 fails (no group), rule 3 succeeds (unique substring match)
- **AND** the binary pulls `outbox`

#### Scenario: Ambiguous repo name invokes fzf

- **GIVEN** no group matches the positional AND two repos match the substring (`outbox`, `outbox-shared`)
- **WHEN** I run `hop pull outbox`
- **THEN** fzf opens with `--query outbox` and both candidates filtered
- **AND** if the user picks one, the binary pulls only that repo (single-repo mode)
- **AND** if the user cancels (Esc), exit code is 130

### Requirement: Per-Repo Pull Behavior

For `hop pull`, each target repo MUST be processed as follows:

1. Resolve the absolute path. If `<path>/.git` does NOT exist, emit `skip: <name> not cloned` to stderr; in batch mode continue with the next repo, in single-repo mode return exit 1.
2. Run `git -C <path> pull` via `internal/proc.RunCapture` with a 10-minute timeout (matching `clone.go::cloneTimeout`). The command MUST NOT pass `--rebase` — `pull` is a verbatim wrapper.
3. On success, emit `pull: <name> ✓ <one-line summary>` to stderr where `<one-line summary>` is the last non-empty line of git's stdout (e.g., "Already up to date." or "Fast-forward").
4. On failure (non-zero git exit), emit `pull: <name> ✗ <error>` to stderr and record the failure for the batch summary; the verbatim git stderr SHALL be passed through.
5. If `git` is not on PATH, emit `gitMissingHint` (`hop: git is not installed.`) to stderr exactly once and exit 1 immediately (do not continue iterating the batch).

#### Scenario: Pull runs through internal/proc with the working directory set

- **GIVEN** `outbox` resolves to `/home/u/code/outbox`
- **WHEN** I run `hop pull outbox`
- **THEN** the binary calls `proc.RunCapture(ctx, "/home/u/code/outbox", "git", "pull")`
- **AND** no shell string is constructed; argv is an explicit slice (Constitution I)

#### Scenario: Single-repo pull skip-not-cloned exits 1

- **GIVEN** `outbox` resolves to `<path>` but `<path>/.git` does not exist
- **WHEN** I run `hop pull outbox`
- **THEN** stderr shows `skip: outbox not cloned`
- **AND** no `git pull` is invoked
- **AND** exit code is 1

#### Scenario: Pull failure surfaces verbatim git stderr

- **GIVEN** `outbox` is cloned but the upstream is unreachable (network error)
- **WHEN** I run `hop pull outbox`
- **THEN** git's stderr is forwarded verbatim
- **AND** stderr emits `pull: outbox ✗ <error summary>`
- **AND** in single-repo mode exit code is 1; in batch mode the failure is counted toward `failed=`

### Requirement: Per-Repo Sync Behavior

For `hop sync`, each target repo MUST be processed as follows:

1. Resolve the absolute path. If `<path>/.git` does NOT exist, emit `skip: <name> not cloned` to stderr; behave identically to `pull` (continue or exit 1).
2. Run `git -C <path> pull --rebase` via `internal/proc.RunCapture` (10-minute timeout).
   - On rebase conflict (git exits non-zero with `CONFLICT` substring in stderr), emit `sync: <name> ✗ rebase conflict — resolve manually with: git -C <path> rebase --continue` to stderr; record failure; do NOT invoke `git push`.
   - On any other rebase failure (dirty tree, no upstream, detached HEAD, network), emit `sync: <name> ✗ <error>` to stderr; record failure; do NOT push. Git's stderr is forwarded verbatim. The implementation MAY prepend a one-line hop hint when stderr matches a detached-HEAD or no-upstream pattern (intake assumption #13).
3. On rebase success, run `git -C <path> push` via `internal/proc.RunCapture`.
   - On push success (including "Everything up-to-date"), emit `sync: <name> ✓ <pull-summary> <push-summary>`.
   - On push failure (non-fast-forward, network, etc.), emit `sync: <name> ✗ push failed: <error>`; record failure. Git's stderr is forwarded verbatim.
4. There SHALL be no auto-stash, no force-push, no auto-resolve. Dirty-tree refusal from `git pull --rebase` surfaces verbatim.
5. If `git` is not on PATH, emit `gitMissingHint` once and exit 1 immediately (matching `pull`).

#### Scenario: Rebase conflict emits hint and skips push

- **GIVEN** `outbox` has a local commit and the upstream has a conflicting commit on the same lines
- **WHEN** I run `hop sync outbox`
- **THEN** `git pull --rebase` exits non-zero with `CONFLICT` in stderr
- **AND** stderr emits `sync: outbox ✗ rebase conflict — resolve manually with: git -C <path> rebase --continue`
- **AND** `git push` is NOT invoked
- **AND** in single-repo mode exit code is 1

#### Scenario: Push rejection surfaces verbatim

- **GIVEN** `outbox` rebased cleanly but the remote has been pushed to since the last fetch (non-fast-forward)
- **WHEN** I run `hop sync outbox`
- **THEN** `git push` exits non-zero with the standard non-fast-forward stderr
- **AND** stderr emits `sync: outbox ✗ push failed: <verbatim git error>`
- **AND** exit code is 1 (single) or counted toward `failed=` (batch)

#### Scenario: Dirty working tree refuses verbatim

- **GIVEN** `outbox` has uncommitted changes
- **WHEN** I run `hop sync outbox`
- **THEN** `git pull --rebase` refuses with its standard "cannot pull with rebase: You have unstaged changes" stderr
- **AND** the binary emits `sync: outbox ✗ <error>` and forwards git's stderr verbatim
- **AND** `git push` is NOT invoked

### Requirement: Batch Summary Line

When the target set has more than one repo (group name or `--all`), the binary MUST emit a final summary line to stderr after iterating the set:

- For `pull`: `summary: pulled=N skipped=M failed=K`
- For `sync`: `summary: synced=N skipped=M failed=K`

The exit code MUST be 0 if `K == 0`, else 1 (matching `hop clone --all`'s policy via `errSilent`).

#### Scenario: Batch with mixed outcomes reports each count and exits 1

- **GIVEN** a target set of 4 repos: 2 succeed, 1 is not cloned, 1 fails the pull
- **WHEN** I run `hop pull <group>`
- **THEN** stderr shows the per-repo lines and a final `summary: pulled=2 skipped=1 failed=1`
- **AND** exit code is 1

#### Scenario: Empty target set still emits a summary

- **GIVEN** a group whose every member has `<path>/.git` missing
- **WHEN** I run `hop pull <group>`
- **THEN** stderr emits `skip:` lines for every member
- **AND** the final stderr line is `summary: pulled=0 skipped=N failed=0`
- **AND** exit code is 0

### Requirement: Sequential Iteration

Batch operations (group name or `--all`) MUST iterate target repos sequentially in YAML source order — same order as `repos.FromConfig` produces and `hop ls` displays. Concurrency is explicitly out of scope for v1 (see Non-Goals).

#### Scenario: Output order is stable

- **GIVEN** a group with 3 repos in YAML source order `a`, `b`, `c`
- **WHEN** I run `hop pull <group>` twice with the same hop.yaml
- **THEN** the per-repo stderr lines appear in the order `a`, `b`, `c` both times

### Requirement: Stdout / Stderr Conventions

Both subcommands MUST mirror `hop clone`'s output discipline:

- **stdout**: empty. Neither `pull` nor `sync` prints any path on stdout. The shell shim does NOT `cd` after these verbs.
- **stderr**: per-repo status lines, `skip:` lines, the batch summary line, and any error/hint text.

#### Scenario: Single-repo pull writes nothing to stdout

- **GIVEN** any successful single-repo `hop pull <name>` invocation
- **WHEN** I capture stdout and stderr separately
- **THEN** stdout is the empty string
- **AND** stderr contains exactly one `pull: <name> ✓ ...` line

### Requirement: Exit Codes

`pull` and `sync` MUST use these exit codes (consistent with `clone`):

| Code | Trigger |
|---|---|
| 0 | All targets succeeded (or "already up to date" / "Everything up-to-date") |
| 1 | Any single-repo failure; any batch with `failed > 0`; single-repo `not cloned`; `git` missing on PATH |
| 2 | Usage error: positional missing AND `--all` absent; positional combined with `--all` |
| 130 | Fzf cancelled during ambiguous repo-name resolution |

#### Scenario: Fzf cancellation in single-repo mode exits 130

- **GIVEN** the positional matches multiple repo names
- **WHEN** I run `hop pull <ambiguous>` and press Esc in fzf
- **THEN** exit code is 130 (`errFzfCancelled`)

## Match Resolution: Name-or-Group Resolver

### Requirement: New Resolver Mode

A new resolver `resolveTargets(query string, all bool) ([]repos.Repo, mode, error)` SHALL be introduced for `pull` and `sync` and SHALL NOT replace the existing `resolveOne`/`resolveByName` (used by `hop`, `hop <name> where`, `hop clone`). The resolver returns the target set plus the mode (`single` vs `batch`) so callers can switch on output formatting. The mode determines exit-code behavior (single-repo failure → exit 1; batch → exit 1 only if any failed).

The resolver MUST honor:

1. `all == true` → return every URL in `repos.FromConfig` order; mode is `batch`.
2. `query` exactly matches a group name in `hop.yaml` (case-sensitive) → return every URL in that group; mode is `batch`.
3. Otherwise → fall through to `resolveByName` (case-insensitive substring on `Name`, with fzf); mode is `single`.

`single` mode wraps the result in a one-element slice.

#### Scenario: Resolver returns batch mode for group match

- **GIVEN** `hop.yaml` defines a group `vendor` with 3 URLs
- **WHEN** the resolver is called with `query = "vendor", all = false`
- **THEN** it returns 3 repos and mode `batch`
- **AND** no fzf invocation occurs

#### Scenario: Resolver returns batch mode for `--all`

- **WHEN** the resolver is called with `query = "", all = true`
- **THEN** it returns every repo and mode `batch`

#### Scenario: Resolver falls through to single-repo match

- **GIVEN** `hop.yaml` has no group named `out` and one repo whose `Name` substring-matches
- **WHEN** the resolver is called with `query = "out", all = false`
- **THEN** it returns one repo and mode `single`

### Requirement: Group-Match Case Sensitivity

Group-name lookup MUST be case-sensitive (`vendor` does NOT match `Vendor`). This matches `findGroup` in `clone.go::findGroup` and config/yaml-schema's group-name semantics. The case-insensitive substring match remains the rule for the repo-name fallback, preserving the existing `MatchOne` contract.

#### Scenario: Case-mismatched group falls through

- **GIVEN** `hop.yaml` defines a group `default` and no repo whose `Name` substring-matches `Default`
- **WHEN** the resolver is called with `query = "Default", all = false`
- **THEN** rule 2 fails (case-sensitive), rule 3 falls through to `resolveByName`
- **AND** if there are zero substring matches AND no exact group match, fzf opens with `--query Default` over the full repo list

### Requirement: Group-vs-Repo Collision Tiebreaker

When a positional argument exactly equals a group name AND also (case-insensitively) substring-matches one or more repo names, the **group match wins** (rule 2 fires before rule 3). Mismatches are observable via the per-repo `pull:` / `sync:` lines, so users can rename the group in `hop.yaml` to disambiguate if needed.

#### Scenario: Group `outbox` wins over repo `outbox`

- **GIVEN** `hop.yaml` defines a group named `outbox` with 2 URLs AND has a separate repo (in another group) whose `Name` is `outbox`
- **WHEN** I run `hop pull outbox`
- **THEN** the resolver matches the group `outbox` and pulls its 2 URLs in batch mode
- **AND** the standalone `outbox` repo (in the other group) is NOT pulled unless it is a member of the `outbox` group

## Subprocess Execution

### Requirement: All Git Invocations Through `internal/proc`

Every `git pull`, `git pull --rebase`, and `git push` invocation in `pull.go` and `sync.go` MUST route through `internal/proc.RunCapture` with `dir = <repo path>` and an explicit argv slice. Direct use of `os/exec` is prohibited (Constitution I).

#### Scenario: All subprocess calls use proc.RunCapture

- **GIVEN** the source files `src/cmd/hop/pull.go` and `src/cmd/hop/sync.go`
- **WHEN** they are inspected for subprocess calls
- **THEN** every git invocation goes through `proc.RunCapture(ctx, path, "git", ...)`
- **AND** there are zero `os/exec.Command` references outside `internal/proc`

### Requirement: 10-Minute Timeout per Git Call

Each git invocation MUST be wrapped in a `context.WithTimeout(context.Background(), 10*time.Minute)` matching `clone.go::cloneTimeout`. The timeout is per-invocation, not per-batch — a batch of N repos can run for up to `N * 10` minutes total in the worst case.

#### Scenario: Per-call timeout is independent

- **GIVEN** a batch of 3 repos, where the 1st takes 2 minutes and the 2nd hangs
- **WHEN** I run `hop pull <group>`
- **THEN** the 1st pull completes successfully
- **AND** the 2nd pull is cancelled at the 10-minute mark with a context-deadline error
- **AND** the 3rd pull begins after the 2nd's cancellation

### Requirement: `git` Missing on PATH

When `proc.RunCapture` returns `proc.ErrNotFound` for a git invocation, the binary MUST emit `gitMissingHint` (`hop: git is not installed.`) to stderr exactly once and exit 1 immediately. Subsequent repos in the batch MUST NOT be attempted.

#### Scenario: Git missing aborts the batch

- **GIVEN** `git` is not on PATH and the user runs `hop pull --all` over 5 repos
- **WHEN** the first `proc.RunCapture` returns `ErrNotFound`
- **THEN** stderr emits `hop: git is not installed.` once
- **AND** no further `proc.RunCapture` invocations occur
- **AND** exit code is 1
- **AND** no `summary:` line is printed (the batch did not complete)

## Help Text and Discoverability

### Requirement: Cobra `Use` and `Short`

Each subcommand MUST register with cobra using:

- `pull`: `Use: "pull [<name-or-group>] [--all]"`, `Short: "Run 'git pull' in a repo, group, or every cloned repo with --all"`.
- `sync`: `Use: "sync [<name-or-group>] [--all]"`, `Short: "Run 'git pull --rebase' then 'git push' in a repo, group, or every cloned repo with --all"`.

The cobra `Args` validator MUST be `cobra.MaximumNArgs(1)` for both. The `--all` flag MUST be a boolean defaulting to `false`.

#### Scenario: Cobra rejects 2+ positionals

- **WHEN** the user runs `hop pull a b`
- **THEN** cobra's `MaximumNArgs(1)` fires before `RunE`
- **AND** stderr shows cobra's "accepts at most 1 arg(s)" usage error
- **AND** exit code is 2

### Requirement: `rootLong` Usage Block

`rootLong` in `src/cmd/hop/root.go` MUST gain six lines (after the `hop clone` block) describing the new subcommands:

```
  hop pull <name>           Run 'git pull' in the named repo
  hop pull <group>          Run 'git pull' in every cloned repo of <group>
  hop pull --all            Run 'git pull' in every cloned repo
  hop sync <name>           Run 'git pull --rebase' then 'git push' in <name>
  hop sync <group>          Run sync in every cloned repo of <group>
  hop sync --all            Run sync in every cloned repo
```

The `Notes:` block MUST gain a bullet:

> `pull` and `sync` accept a repo name OR a group name (exact match) as the positional, plus `--all` for the full registry. `sync` is `pull --rebase` + `push` — linear history, no auto-resolve on conflict.

#### Scenario: Help output documents both verbs

- **WHEN** I run `hop --help`
- **THEN** stdout contains the six new `Usage:` lines in the order above
- **AND** stdout contains the new `Notes:` bullet

### Requirement: Cobra `AddCommand` Wiring

`newRootCmd()` in `src/cmd/hop/root.go` MUST call `rootCmd.AddCommand(newPullCmd(), newSyncCmd())` alongside the existing subcommand registrations. The shell shim's known-subcommand list (in `shell_init.go::posixInit`) MUST be extended to include `pull` and `sync` so the shim routes them through `_hop_dispatch` rather than treating them as repo names.

#### Scenario: Shim routes `hop pull` correctly

- **GIVEN** the user has run `eval "$(hop shell-init zsh)"`
- **WHEN** they run `hop pull outbox`
- **THEN** the shim's rule 3 (known subcommand) matches `pull`
- **AND** the call is routed through `_hop_dispatch pull outbox`
- **AND** the binary's cobra dispatch handles the rest

#### Scenario: Without shim list update, shim would misroute

- **GIVEN** `pull` is NOT in the shim's known-subcommand list
- **WHEN** the user runs `hop pull outbox` under the shim
- **THEN** the shim's rule 5 fires (`$1` treated as repo name) and rewrites to `command hop -R pull outbox`
- **AND** the binary errors with `-R: 'outbox' not found.` (or resolves `pull` as a non-existent repo)
- **AND** this scenario is the regression to prevent — the shim list MUST include `pull` and `sync`

### Requirement: Tab Completion for the Positional

The `pull` and `sync` cobra commands MUST register a `ValidArgsFunction` that completes both repo names and group names from `hop.yaml`. A new helper `completeRepoOrGroupNames` SHALL live in `src/cmd/hop/repo_completion.go` (or alongside the existing `completeCloneArg`) and SHALL deduplicate entries so a name appearing as both a group and a repo is offered once.

#### Scenario: Tab completion offers groups and repos

- **GIVEN** `hop.yaml` defines groups `default` and `vendor` and repos `outbox`, `loom`
- **WHEN** the user types `hop pull <TAB>`
- **THEN** completion offers `default`, `vendor`, `outbox`, `loom` (in some stable order)

## Memory and Architecture

### Requirement: Source Layout

Two new files SHALL be added under `src/cmd/hop/`:

- `src/cmd/hop/pull.go` — `newPullCmd()` factory, per-repo `pullOne` runner, and the resolver entry point.
- `src/cmd/hop/sync.go` — `newSyncCmd()` factory, per-repo `syncOne` runner.

A shared helper for "iterate target repos, run a per-repo function, collect results, print summary" SHALL be extracted to keep `pull.go` and `sync.go` short. Candidate location: `src/cmd/hop/batch.go` (preferred when the helper grows beyond ~30 lines) OR inline in `pull.go` if it stays compact. The shared resolver `resolveTargets` SHALL live in `src/cmd/hop/resolve.go` (next to `resolveByName`) since it composes the existing match-resolution algorithm.

The shell shim's known-subcommand list (`src/cmd/hop/shell_init.go::posixInit`) MUST be updated to include `pull` and `sync`.

`src/cmd/hop/root.go::newRootCmd` MUST register the new commands via `AddCommand` and update `rootLong`.

#### Scenario: Files exist in expected locations after change

- **WHEN** I list `src/cmd/hop/`
- **THEN** the directory contains `pull.go` and `sync.go`
- **AND** `resolve.go` contains a `resolveTargets` function
- **AND** `shell_init.go` lists `pull` and `sync` in the shim's known-subcommand case statement

### Requirement: No Direct Filesystem or Subprocess Outside `internal/`

The new files MUST NOT touch `os/exec`, `os.Stat` for `.git` directly, or any wrapper-bypass. `<path>/.git` existence checks SHALL reuse `cloneState` from `clone.go` (or an equivalent helper) so the on-disk classification stays in one place. All git invocations route through `internal/proc.RunCapture` (Constitution I).

#### Scenario: Cloned-repo check reuses existing helper

- **GIVEN** `pull.go` needs to test whether `<path>/.git` exists
- **WHEN** the implementation is written
- **THEN** it calls `cloneState(path)` (or shares an extracted helper such as `isCloned(path) bool`) rather than re-implementing the stat logic

### Requirement: Memory Updates

The hydration step (out of scope for this spec; performed by `/fab-continue` hydrate) SHALL update:

- `docs/memory/cli/subcommands.md` — add `pull` and `sync` rows to the inventory; update the Usage block reference to include the new lines; add a brief "name-or-group resolution" note linking to `match-resolution.md`.
- `docs/memory/cli/match-resolution.md` — document the new name-or-group resolver as a separate section, noting that it composes the existing substring match algorithm with an exact-group prefix step.
- `docs/memory/architecture/package-layout.md` — note `pull.go`, `sync.go`, and (if extracted) `batch.go` under `src/cmd/hop/`.

No new memory domain is introduced.

#### Scenario: Affected memory list is stable

- **WHEN** the hydrator inspects this spec's "Affected memory" metadata field
- **THEN** it sees three files to update: `cli/subcommands.md`, `cli/match-resolution.md`, `architecture/package-layout.md`

## Test Coverage

### Requirement: Unit Tests Per Subcommand

Two new test files MUST be added (mirroring `clone_test.go`):

- `src/cmd/hop/pull_test.go` — argument resolution (name vs. group vs. `--all`), missing-clone skip, batch summary, fake-git harness via the `internal/proc` test seam.
- `src/cmd/hop/sync_test.go` — rebase-conflict path, push-failure path, dirty-tree refusal, batch summary, group resolution.

`integration_test.go` MAY be extended with a happy-path end-to-end test against a temp git server.

#### Scenario: Tests can run without a real git binary

- **GIVEN** the test seam in `internal/proc` (or a wrapper in test helpers)
- **WHEN** `pull_test.go` runs
- **THEN** the tests use a fake git that returns canned stdout/stderr/exit codes for `pull`, `pull --rebase`, and `push`
- **AND** no real `git clone` or network call occurs

## Design Decisions

1. **Top-level subcommands (`hop pull <name>`, `hop sync <name>`), not repo-verb form.**
   - *Why*: Parallel to `hop clone` (the existing top-level per-repo registry verb). Top-level lets the same verb take a name, a group, or `--all` naturally; the repo-verb $2 slot is reserved for shell-integrated forms (`cd`, `where`) and exec wrappers (`-R`, tool-form). `pull` and `sync` produce no shell-cwd side effect, so they belong at $1. Constitution VI requires explicit justification for new top-level subcommands — this rationale is captured here.
   - *Rejected*: `hop <name> pull` / `hop <name> sync` (repo-verb form). It would force ugly batch syntax (`hop --all pull`?) and conflate registry verbs with shell-integrated verbs. The grammar `subcommand xor repo at $1` (per docs/specs/cli-surface.md design decision #10) is preserved by the top-level form.

2. **All subprocess execution routes through `internal/proc.RunCapture`.**
   - *Why*: Constitution I (security first) mandates that all process execution use `exec.CommandContext` with explicit argv — never shell strings. `internal/proc` is the single chokepoint for that discipline; routing every `git pull`, `git pull --rebase`, and `git push` through it preserves the invariant. 10-minute timeout matches `clone.go::cloneTimeout` precedent.
   - *Rejected*: Direct `os/exec.Command` (violates Constitution I); shell-string composition (injection risk).

3. **Verbatim git error pass-through; no auto-stash, no auto-resolve, no merge fallback.**
   - *Why*: Constitution IV (wrap, don't reinvent) — git's errors are already precise (CONFLICT messages, "cannot pull with rebase: You have unstaged changes", "Updates were rejected because the tip of your current branch is behind"). Re-implementing pre-checks duplicates git's state machine and creates drift. The CONFLICT-detection hint (one-line nudge to `git rebase --continue`) is the only hop-side enrichment, and it triggers ONLY on a known-good substring match.
   - *Rejected*: Auto-stash before sync (violates "wrap, don't reinvent" — `git stash` semantics are non-trivial); merge fallback on rebase conflict (changes the user's branch shape silently); pre-flight working-tree check (duplicates git's own check).

4. **Minimal flags: only `--all`. No `--no-fetch`, no `--continue`, no `--rebase` for `pull`.**
   - *Why*: Constitution VI (minimal surface area). Each flag must justify its existence. `--no-fetch` is covered by `hop <name> -R git merge FETCH_HEAD`; `--continue` would require persisted batch state (violates Constitution II — no database/cache) and is cheap to skip given that re-running `pull --all` is fast for already-up-to-date repos; `--rebase` would muddle `pull` and `sync`'s clear separation (`pull` is the safe verb, `sync` is the rebase+push verb).
   - *Rejected*: Adding speculative flags "in case the user wants them" — adds maintenance surface, complicates docs, dilutes the verbs' identity.

5. **Argument resolution: group-name exact match BEFORE substring repo match (rule 2 before rule 3).**
   - *Why*: Group names in `hop.yaml` are user-curated (intentional) while repo names are URL-derived (incidental). When a user types a string that happens to be both a group name and a substring of a repo name, the curated group is the more likely intent. Mismatches are observable via per-repo status lines so the user can correct. Tied at the SRAD `Confident` grade because the collision is theoretical.
   - *Rejected*: Substring repo match wins (would require users to escape group names with a `--group` flag); reject with "ambiguous" error (raises the user's resolution cost for a theoretical collision).

6. **Sequential iteration in batch mode; no concurrency in v1.**
   - *Why*: Predictable output ordering matters more than wall-clock for small registries (typical hop.yaml has 5-50 repos). Concurrent git operations also raise terminal-output interleaving issues for the verbose progress lines git pull emits. Sequentiality is the conservative starting point and concurrency is a follow-up if real wall-clock pain emerges.
   - *Rejected*: Goroutine-per-repo with merged output (output interleaving, harder error attribution); worker pool with bounded concurrency (premature complexity).

7. **stdout empty; per-repo lines and summary on stderr.**
   - *Why*: Mirrors `hop clone`'s convention exactly (precedent rule). `pull` and `sync` are "do work, report it" verbs — no path to print, no shim `cd`. Reserving stdout for nothing keeps the verbs clean for piping into `xargs` or future composition (e.g. `hop pull --all 2>&1 | grep ✗`).
   - *Rejected*: Printing the changed paths to stdout (no caller use case; would couple to the shim).

8. **`pull` and `sync` exit codes follow `clone --all`'s policy: 0 if no failures, 1 if any failure.**
   - *Why*: Single, clear contract for scripts. Mixing partial-success codes (e.g., 0 for "all attempted, some failed") would make CI integration brittle. The summary line gives the breakdown for human readers; the exit code is binary for machines.
   - *Rejected*: Exit code = number of failed repos (clamps awkwardly at 255); separate code per failure category (over-classification).

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Top-level subcommands (`hop pull <name>`, `hop sync <name>`) rather than repo-verb form | Confirmed from intake #1 — verified parallel to `hop clone` in `docs/specs/cli-surface.md`; Constitution VI justification recorded in Design Decisions | S:95 R:80 A:80 D:90 |
| 2 | Certain | `pull` = `git pull`; `sync` = `git pull --rebase` + `git push` | Confirmed from intake #2 — verb semantics fixed in pre-intake conversation; mapped to per-repo behavior requirements | S:95 R:70 A:75 D:90 |
| 3 | Certain | Positional accepts name OR group (exact group match first, then substring repo match), plus `--all` | Confirmed from intake #3 — codified as the new `resolveTargets` resolver in Match Resolution section; rule order documented | S:90 R:70 A:80 D:90 |
| 4 | Certain | Sync rebase failure surfaces verbatim — no auto-stash, no auto-resolve, no merge fallback | Confirmed from intake #4 — Constitution IV rule-determined; Design Decision #3 captures rejected alternatives | S:90 R:80 A:95 D:90 |
| 5 | Certain | Both subcommands route subprocess calls through `internal/proc.RunCapture` with 10-minute timeout | Confirmed from intake #5 — verified `RunCapture` signature in `src/internal/proc/proc.go`; matches `clone.go::cloneTimeout` | S:95 R:85 A:95 D:95 |
| 6 | Certain | stdout is empty; per-repo status and batch summary go to stderr | Confirmed from intake #6 — verified by reading `clone.go` (status lines via `cmd.ErrOrStderr()`); Design Decision #7 | S:90 R:80 A:95 D:90 |
| 7 | Certain | Batch operations run sequentially (not concurrently) | Confirmed from intake #7 — codified in "Sequential Iteration" requirement; Design Decision #6 explains rejection of goroutine variants. Rule-determined: predictable stderr ordering is a hard constraint | S:90 R:85 A:90 D:90 |
| 8 | Certain | Skip-if-not-cloned in batch mode (don't fail the whole batch); single-repo not-cloned exits 1 | Confirmed from intake #8 — mirrored from `cloneAll` skip-if-already-cloned posture; encoded in per-repo and batch summary requirements | S:90 R:85 A:95 D:90 |
| 9 | Certain | No new memory domain — extends `cli` and `architecture` | Confirmed from intake #9 — verified by reading `docs/memory/index.md`; affected files listed in Memory Updates requirement | S:95 R:90 A:95 D:95 |
| 10 | Certain | Group-vs-repo collision: group wins (exact match beats substring) | Confirmed from intake #10 — user-confirmed in pre-intake conversation; Design Decision #5 documents tradeoffs; `findGroup` precedent in `clone.go` confirms case-sensitive group lookup | S:90 R:80 A:90 D:90 |
| 11 | Certain | Cobra completion completes both repo names and group names in the positional slot | Confirmed from intake #11 — user-confirmed in pre-intake conversation; `completeRepoOrGroupNames` requirement added; trivial extension of the existing `completeCloneArg` pattern | S:90 R:90 A:90 D:90 |
| 12 | Certain | Drop `--no-fetch` flag from `pull`; rely on tool-form for the niche workflow | Confirmed from intake #12 — Constitution VI rule-determined; captured in Non-Goals and Design Decision #4 | S:90 R:90 A:90 D:85 |
| 13 | Certain | Pass git's error through verbatim for `sync` failures; MAY prepend a one-line hop hint when stderr matches detached-HEAD or no-upstream patterns | Confirmed from intake #13 — Constitution IV rule-determined; encoded as MAY in Per-Repo Sync Behavior | S:90 R:80 A:95 D:85 |
| 14 | Certain | No `--continue` flag for resumable batches in v1 | Confirmed from intake #14 — Constitution II rule-determined (no database/cache forbids persisted batch state); listed in Non-Goals | S:90 R:90 A:95 D:90 |
| 15 | Certain | Shell shim's known-subcommand list MUST be updated to include `pull` and `sync` | Spec-level discovery — verified by reading `shell_init.go::posixInit` rule 3 in `docs/memory/cli/subcommands.md`; without this update, the shim's rule 5 would misroute `hop pull <name>` into tool-form. Encoded in `Cobra AddCommand Wiring` scenario | S:95 R:90 A:95 D:95 |
| 16 | Certain | Cobra `Args` validator is `cobra.MaximumNArgs(1)`; `hop pull a b` is rejected by cobra before `RunE` | Spec-level discovery — verified by reading `clone.go` (`cobra.MaximumNArgs(1)`); `--all` does not consume a positional, so the cap stays at 1 | S:95 R:90 A:95 D:95 |
| 17 | Certain | `git` missing on PATH aborts a batch immediately (no further repos attempted); single `gitMissingHint` line on stderr | Spec-level discovery — mirrors `clone.go` and `cloneAll` `proc.ErrNotFound` early-return; encoded in `git Missing on PATH` requirement | S:90 R:85 A:90 D:90 |
| 18 | Certain | Cloned-state check reuses `cloneState` (or an extracted `isCloned`) — does not re-implement `os.Stat` for `.git` | Spec-level discovery — `cloneState` already classifies missing/cloned/path-conflict-not-git in `clone.go` (verified by reading source); Constitution II.B (don't reinvent existing utilities) makes reuse rule-determined | S:90 R:85 A:95 D:90 |
| 19 | Confident | `resolveTargets` returns a `mode` (single vs batch) so callers switch on output and exit-code semantics; this avoids leaking `--all` and group-vs-name detection into per-subcommand code paths | Spec-level discovery — without an explicit mode return, each subcommand would re-derive batch-vs-single from `len(targets)`, but the `single`-mode-with-1-repo case (a unique substring match) needs different exit-code policy than `batch`-mode-with-1-repo (a group with one cloned member) | S:75 R:75 A:80 D:75 |
| 20 | Certain | Per-repo timeout is independent (10 min each), not a shared batch budget; worst-case total = N × 10 min | Spec-level discovery — `clone.go::cloneAll` uses per-iteration `context.WithTimeout(ctx, cloneTimeout)` (verified by reading source); precedent rule — preserve the existing shape | S:90 R:85 A:95 D:90 |

20 assumptions (19 certain, 1 confident, 0 tentative, 0 unresolved).
