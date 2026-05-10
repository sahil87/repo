# Spec: hop sync auto-commits dirty trees before pull/push

**Change**: 260510-nzb0-auto-commit-on-sync
**Created**: 2026-05-10
**Affected memory**: `docs/memory/cli/subcommands.md`

## Overview

`hop sync` today wraps `git pull --rebase` then `git push` over a single repo, a named group, or every cloned repo via `--all`. It silently no-ops the most common reason a user reaches for a sync verb: an uncommitted edit to a tracked file (especially symlinked dotfiles). Today the user must (a) drop into the repo, (b) `git add && git commit`, then (c) run `hop sync` — three steps and an external dependency on a shell helper (`xpush`) that hardcodes its own DIRS list, duplicating the registry that `hop.yaml` already owns.

This change extends `hop sync` to auto-commit dirty trees as the default per-repo behavior before the existing rebase + push flow, with a new `-m / --message` flag to override the default commit message. `hop push` is deliberately NOT changed (push without rebase is the riskier op; auto-commit-and-push without an upstream sync would be a footgun). Clean trees behave identically to today.

## Non-Goals

- `hop push` auto-commit — push without rebase is intentionally untouched; users wanting commit-and-push without an upstream sync can compose `hop -R <name> git ...`
- A `--no-verify` flag — Constitution Test Integrity ethos says we don't bypass user-installed hooks; users wanting this can use `hop -R <name> git commit --no-verify` directly
- Stash/unstash semantics around the rebase — out of scope; the rebase happens after the commit, so there is no working-tree state to stash
- Default commit message configurable via `hop.yaml` or env var — Constitution Principle III (Convention Over Configuration); the default is a fixed string
- Surfacing the new commit's SHA in the per-repo status line — keep the line short; users can `git log -1` if curious

## CLI: `hop sync` auto-commit

### Requirement: Dirty-tree detection

`hop sync` SHALL detect a dirty working tree before the existing pull/push flow by running `git status --porcelain` in the target repo. A non-empty result MUST be treated as "dirty"; an empty result MUST be treated as "clean".

#### Scenario: Clean tree skips the auto-commit branch

- **GIVEN** a target repo whose `git status --porcelain` output is empty
- **WHEN** `hop sync <name>` runs
- **THEN** the auto-commit branch (add + commit) MUST NOT execute
- **AND** the existing `git pull --rebase` then `git push` flow MUST run unchanged
- **AND** the per-repo status line MUST NOT contain a `committed,` token

#### Scenario: Dirty tree triggers the auto-commit branch

- **GIVEN** a target repo with an unstaged tracked-file modification (`git status --porcelain` returns one or more lines)
- **WHEN** `hop sync <name>` runs
- **THEN** the auto-commit branch MUST execute before the existing pull/push flow

### Requirement: Stage scope is `git add --all`

When the tree is dirty, `hop sync` SHALL stage every modification, deletion, and untracked file via `git add --all` in the target repo's working directory. The scope MUST match `xpush`'s `git add --all :/` semantic (i.e., a "snapshot the working tree" verb, not a tracked-only stage).

#### Scenario: Untracked file is included in the auto-commit

- **GIVEN** a target repo with one new untracked file and one modified tracked file
- **WHEN** `hop sync <name>` runs
- **THEN** the resulting commit MUST contain both files
- **AND** `git status --porcelain` after the sync MUST be empty (modulo files added during the rebase/push)

### Requirement: Commit step

When the tree is dirty, `hop sync` SHALL run `git commit -m <message>` after `git add --all` succeeds. The `<message>` value comes from the `-m / --message` flag when provided, or from the default message when the flag is absent.

The commit MUST respect user-installed hooks (`pre-commit`, `commit-msg`, `pre-push`). hop MUST NOT pass `--no-verify` and MUST NOT otherwise bypass hooks.

#### Scenario: Commit succeeds with default message

- **GIVEN** a dirty target repo with no `-m` flag passed
- **WHEN** `hop sync <name>` runs
- **THEN** `git commit -m "chore: sync via hop"` MUST be invoked after `git add --all`
- **AND** the resulting commit's message MUST be exactly `chore: sync via hop`

#### Scenario: Pre-commit hook rejects the commit

- **GIVEN** a dirty target repo with a `pre-commit` hook that exits non-zero (e.g., gofmt rejection)
- **WHEN** `hop sync <name>` runs
- **THEN** `git commit` MUST exit non-zero
- **AND** hop MUST NOT invoke `git pull --rebase` for this repo
- **AND** hop MUST NOT invoke `git push` for this repo
- **AND** the per-repo status line MUST be `sync: <name> ✗ commit failed: <last-line-of-git-stderr>`

### Requirement: Default commit message

When `-m / --message` is not provided, the commit message SHALL be the literal string `chore: sync via hop`. This default is fixed in code; it MUST NOT be configurable via `hop.yaml` or environment variables.

#### Scenario: Default message used when flag absent

- **GIVEN** any dirty target repo and no `-m` flag
- **WHEN** `hop sync <name>` runs
- **THEN** the commit's message MUST equal `chore: sync via hop` exactly (no trailing newline, no metadata, no SHA suffix)

### Requirement: `-m / --message` flag

`hop sync` SHALL accept a `-m / --message <msg>` flag whose value is a Go `string` and overrides the default commit message. The flag MUST NOT toggle auto-commit on or off — auto-commit fires whenever the tree is dirty, regardless of whether `-m` is set. The flag MAY be present even when no repo in the resolved target set is dirty (in which case the value has no observable effect).

The flag MUST be registered on the `sync` subcommand only (not on `pull` or `push`). The short alias MUST be `-m` (matches `git commit -m` muscle memory). No additional aliases.

#### Scenario: Custom message overrides the default

- **GIVEN** a dirty target repo
- **WHEN** `hop sync <name> -m "fix(zsh): reload prompt"` runs
- **THEN** the commit's message MUST equal `fix(zsh): reload prompt` exactly
- **AND** the default `chore: sync via hop` MUST NOT appear in the commit history

#### Scenario: `-m` with `--all` applies the same message to every dirty repo

- **GIVEN** three target repos resolved via `--all`, of which two are dirty and one is clean
- **WHEN** `hop sync --all -m "bulk update"` runs
- **THEN** the two dirty repos MUST each produce one commit with message `bulk update`
- **AND** the clean repo MUST NOT produce a commit
- **AND** the per-repo status lines MUST reflect each repo's outcome independently

#### Scenario: `-m` on a clean tree has no observable effect

- **GIVEN** a target repo whose working tree is clean
- **WHEN** `hop sync <name> -m "would-be message"` runs
- **THEN** no commit MUST be produced
- **AND** the per-repo status line MUST NOT contain a `committed,` token
- **AND** the existing pull/push flow MUST run unchanged

#### Scenario: Empty `-m` value falls back to git's own validation

- **GIVEN** a dirty target repo
- **WHEN** `hop sync <name> -m ""` runs
- **THEN** hop SHALL pass the empty string verbatim to `git commit -m ""`
- **AND** git's own behavior (typically aborting with "Aborting commit due to empty commit message.") MUST surface as the commit failure
- **AND** hop MUST emit `sync: <name> ✗ commit failed: <git-stderr-tail>` and skip the rebase/push for this repo

#### Scenario: Multi-line `-m` value passes through to git

- **GIVEN** a dirty target repo
- **WHEN** `hop sync <name> -m $'subject\n\nbody line 1\nbody line 2'` runs
- **THEN** hop MUST pass the multi-line string as a single argv element to `git commit -m`
- **AND** git MUST accept it as a multi-line commit message (subject + body separated by a blank line)

### Requirement: Order of operations on dirty repo

When the tree is dirty, `hop sync` SHALL execute the following steps in this exact order, per repo:

1. `git status --porcelain` — dirty detection
2. `git add --all` — stage everything
3. `git commit -m <message>` — commit (respecting hooks)
4. `git pull --rebase` — existing rebase step
5. `git push` — existing push step

Any step's failure MUST abort that repo's sync immediately — no subsequent step runs for that repo. In batch mode, the failure increments the `failed` counter and the next target proceeds.

#### Scenario: Add failure aborts before commit

- **GIVEN** a dirty target repo where `git add --all` fails (e.g., I/O error, permission)
- **WHEN** `hop sync <name>` runs
- **THEN** `git commit`, `git pull --rebase`, and `git push` MUST NOT run for this repo
- **AND** the per-repo status line MUST surface the add failure

#### Scenario: Commit success then rebase conflict

- **GIVEN** a dirty target repo where `git commit` succeeds but `git pull --rebase` produces a `CONFLICT` marker
- **WHEN** `hop sync <name>` runs
- **THEN** `git push` MUST NOT run for this repo
- **AND** the existing rebase-conflict status line (`sync: <name> ✗ rebase conflict — resolve manually with: git -C <path> rebase --continue`) MUST be emitted unchanged

#### Scenario: Commit and rebase succeed; push fails

- **GIVEN** a dirty target repo where `git commit` and `git pull --rebase` succeed but `git push` fails (non-fast-forward, auth, network)
- **WHEN** `hop sync <name>` runs
- **THEN** the existing push-failure status line (`sync: <name> ✗ push failed: <err>`) MUST be emitted unchanged
- **AND** the local commit produced in step 3 MUST remain in the repo's history (no automatic rollback)

### Requirement: Per-call timeout

Each git invocation in the sync flow (`status`, `add`, `commit`, `pull --rebase`, `push`) SHALL run under its own `context.WithTimeout` with the existing `cloneTimeout` value (10 minutes), via `proc.RunCapture` or `proc.RunCaptureBoth`. Timeouts MUST NOT share a single batch budget across the per-repo step sequence.

#### Scenario: One step's timeout does not consume the next step's budget

- **GIVEN** a dirty target repo where each git step takes 8 minutes
- **WHEN** `hop sync <name>` runs
- **THEN** all five steps MUST complete (each under its own 10-minute window) without timing out
- **AND** no shared parent context MUST cap the cumulative duration to 10 minutes

### Requirement: Subprocess wrapping

All git invocations introduced by this change (`git status --porcelain`, `git add --all`, `git commit -m <msg>`) MUST go through `internal/proc.RunCapture` (or `RunCaptureBoth` if stderr inspection is needed) with explicit argument slices. They MUST NOT use `exec.Command` directly, MUST NOT use shell strings, and MUST NOT import `os/exec` outside `internal/proc`. (Constitution Principle I.)

The subprocess working directory MUST be set via `cmd.Dir = repo.Path` (delegated to `proc.RunCapture`'s `dir` argument), not via `git -C <path>`. The user-supplied repo path MUST come from the resolved `repos.Repo` (already validated by `loadRepos`) — never from raw user input.

#### Scenario: All new git invocations route through internal/proc

- **GIVEN** the implementation of the auto-commit branch
- **WHEN** the source code is audited via `grep --include='*.go' --exclude='*_test.go' -rn '"os/exec"' src/cmd/hop/`
- **THEN** the audit MUST return zero matches in `src/cmd/hop/sync.go`

## CLI: per-repo status output

### Requirement: Success line on dirty-tree sync

When `hop sync` successfully commits, rebases, and pushes a dirty repo, the per-repo status line written to stderr SHALL match the shape:

```
sync: <name> ✓ committed, <pull-summary>, <push-summary>
```

Where `<pull-summary>` is `lastNonEmptyLine` of `git pull --rebase`'s stdout (e.g., `Fast-forward`, `Already up to date.`) and `<push-summary>` is `lastNonEmptyLine` of `git push`'s stdout (e.g., `Everything up-to-date`). The `committed,` token MUST appear immediately after the green check mark and before the pull summary, separated from each by a single space and a comma.

#### Scenario: Dirty-tree successful sync

- **GIVEN** a dirty target repo `dotfiles` whose pull rebases cleanly and whose push uploads successfully
- **WHEN** `hop sync dotfiles` runs
- **THEN** stderr MUST contain the line `sync: dotfiles ✓ committed, <pull-summary>, <push-summary>`
- **AND** the line MUST appear exactly once for that repo

### Requirement: Success line on clean-tree sync (unchanged)

When `hop sync` runs on a clean tree, the per-repo status line MUST match today's shape exactly:

```
sync: <name> ✓ <pull-summary> <push-summary>
```

(Two summaries separated by a single space; no `committed,` token; no trailing comma.) This is the existing contract from `sync.go::syncOne` and MUST be preserved verbatim.

#### Scenario: Clean-tree sync emits today's line shape

- **GIVEN** a target repo whose `git status --porcelain` is empty
- **WHEN** `hop sync <name>` runs and both pull and push succeed
- **THEN** the per-repo status line MUST be `sync: <name> ✓ <pull-summary> <push-summary>` (matching today's regression baseline)

### Requirement: Failure line on commit failure

When the auto-commit step fails (any non-zero exit from `git add --all` or `git commit`), the per-repo status line written to stderr SHALL match the shape:

```
sync: <name> ✗ commit failed: <err>
```

Where `<err>` is the last non-empty line of git's combined stderr (per the existing `lastNonEmptyLine` convention in `pull.go`), or — when the underlying error has no captured stderr line — the formatted Go error value. The phrasing `commit failed:` mirrors the existing `push failed:` convention.

#### Scenario: Hook-rejection failure line shape

- **GIVEN** a dirty target repo whose `pre-commit` hook prints `gofmt: bad formatting in foo.go` and exits 1
- **WHEN** `hop sync <name>` runs
- **THEN** stderr MUST contain a line matching `sync: <name> ✗ commit failed: <last-non-empty-stderr-line>`
- **AND** git's own stderr MUST also be forwarded verbatim (via `proc.RunCapture`'s passthrough)

### Requirement: Existing failure lines unchanged

The pre-existing rebase-conflict and push-failure status lines (emitted by `sync.go::syncOne`) MUST remain unchanged in wording and format. This change only adds new lines (`committed,` prefix on success and `commit failed: <err>` on failure); it MUST NOT modify the existing `sync: <name> ✗ rebase conflict — resolve manually...` or `sync: <name> ✗ push failed: <err>` lines.

#### Scenario: Rebase conflict preserved verbatim

- **GIVEN** a dirty repo where commit succeeds but `git pull --rebase` produces `CONFLICT`
- **WHEN** `hop sync <name>` runs
- **THEN** stderr MUST contain the line `sync: <name> ✗ rebase conflict — resolve manually with: git -C <path> rebase --continue` exactly as today

### Requirement: Batch summary format unchanged

The existing batch summary line `summary: synced=N skipped=M failed=K` MUST remain unchanged. A repo whose auto-commit succeeded and whose subsequent rebase + push also succeeded counts as `synced=1`. A repo whose auto-commit failed counts as `failed=1` (alongside any other failure mode that already counts as failed).

#### Scenario: Batch summary across mixed clean/dirty/failed repos

- **GIVEN** three repos targeted via `--all`: `dotfiles` (dirty, all steps succeed), `outbox` (clean, pull/push succeed), `hop` (dirty, commit fails on hook)
- **WHEN** `hop sync --all` runs
- **THEN** the per-repo lines MUST be (in resolution order):
  - `sync: dotfiles ✓ committed, <pull>, <push>`
  - `sync: outbox ✓ <pull> <push>`
  - `sync: hop ✗ commit failed: <err>`
- **AND** the final line MUST be `summary: synced=2 skipped=0 failed=1`

## CLI: targeting and resolution (unchanged)

### Requirement: Resolution rules unchanged

`hop sync` MUST continue to delegate target resolution to `resolveTargets` (the shared name-or-group resolver in `cmd/hop/resolve.go`). The single-positional, group, and `--all` rules MUST behave identically to today. No new positional, no new resolution rule.

#### Scenario: Group target with `-m` and mixed cleanliness

- **GIVEN** a group `dotfiles-group` containing two repos, one dirty and one clean
- **WHEN** `hop sync dotfiles-group -m "weekly sync"` runs
- **THEN** both repos MUST be resolved via `resolveTargets` exactly as today
- **AND** the dirty repo MUST commit with message `weekly sync`
- **AND** the clean repo MUST skip the auto-commit branch and run pull/push only

### Requirement: `not cloned` skip unchanged

A target repo whose path is missing or whose `.git` is absent MUST emit `skip: <name> not cloned` (existing string) and MUST NOT enter the auto-commit branch. Single-repo "not cloned" still exits 1; batch "not cloned" still increments `skipped`.

#### Scenario: Not-cloned single repo skips before dirty detection

- **GIVEN** a target repo whose `.git` is absent
- **WHEN** `hop sync <name>` runs
- **THEN** stderr MUST contain `skip: <name> not cloned`
- **AND** `git status --porcelain` MUST NOT be invoked for this repo
- **AND** the process MUST exit 1 (single-repo `errSilent` behavior)

## CLI: scope boundaries (`hop push` unchanged)

### Requirement: `hop push` is not modified

`hop push` (`src/cmd/hop/push.go`) MUST NOT be modified by this change. It MUST remain a pure `git push` wrapper with no auto-commit, no `-m` flag, and no behavioral delta. The asymmetry between `hop sync` (auto-commits) and `hop push` (does not) is intentional: pushing without rebasing first is the riskier op, and `hop push`'s muscle memory should match `git push`.

#### Scenario: `hop push` regression baseline

- **GIVEN** a dirty target repo
- **WHEN** `hop push <name>` runs
- **THEN** no commit MUST be produced
- **AND** the per-repo status line MUST match the existing `push: <name> ✓ <last-line>` or `push: <name> ✗ <err>` shape (no `committed,` token, no `-m` flag accepted)

#### Scenario: `hop push -m` is rejected

- **GIVEN** any target repo
- **WHEN** `hop push <name> -m "anything"` runs
- **THEN** cobra MUST reject the unknown flag with its standard `unknown flag: -m` error
- **AND** the process MUST exit non-zero before any git invocation

## Source: `sync.go` package doc comment

### Requirement: Doc comment updated

The package-level / `newSyncCmd` doc comment in `src/cmd/hop/sync.go` SHALL be updated to reflect the new auto-commit behavior. The current "no auto-stash, no auto-resolve, no force-push" sentence is correct for the clean-tree path but misleading for the dirty-tree path (which now does auto-commit). The updated comment MUST distinguish the two paths.

#### Scenario: Doc comment matches behavior

- **GIVEN** the updated `sync.go`
- **WHEN** a reader greps for "auto-stash" or "auto-commit" in the file
- **THEN** the comment MUST mention auto-commit on dirty trees
- **AND** the comment MUST clarify that auto-stash, auto-resolve, and force-push remain absent

## Implementation hint (non-binding)

The auto-commit logic SHOULD live as a `commitDirtyTree(ctx context.Context, cmd *cobra.Command, r repos.Repo, msg string) (committed bool, err error)` (or similar) helper inline in `sync.go` — not extracted to a shared file — unless another subcommand surfaces a similar need later. Whether the helper signature carries `cmd` or returns just the committed/err pair is the implementer's call.

## Governance

- **Constitution Principle I (Security First)**: All new git invocations route through `internal/proc.RunCapture` (or `RunCaptureBoth`) with `exec.CommandContext` and explicit argument slices. Verified by the Subprocess Wrapping requirement above.
- **Constitution Principle IV (Wrap, Don't Reinvent)**: Composing `git status` + `git add` + `git commit` + `git pull --rebase` + `git push` into one `hop sync` verb is extension, not reinvention. We are wrapping `git` (the battle-tested tool that does what we need); we are NOT building a Go-native commit object writer or rebase engine. Per intake assumption #10, the principle governs replacing tools, not extending the verb's surface area.
- **Constitution Principle VI (Minimal Surface Area)**: The new `-m / --message` flag is justified — without it, users cannot override the default commit message in cases where Conventional Commits or audit conventions matter. The "could this be a flag on an existing subcommand?" test passes: yes, it is a flag, not a new top-level verb. Per intake's "Why" section, alternatives (`--commit` flag, separate `hop commit` verb) were considered and rejected.
- **Cross-Platform Behavior**: The change MUST build and run on darwin-arm64, darwin-amd64, linux-arm64, and linux-amd64. No platform-specific code is introduced — `git`, `proc.RunCapture`, and `proc.RunCaptureBoth` already support all four targets.
- **Test Integrity**: New tests in `sync_test.go` MUST conform to this spec; the implementation MUST conform to this spec. Tests MUST NOT be modified to accommodate implementation quirks; the implementation MUST NOT be modified solely to make tests pass.

## Acceptance criteria

A1. `hop sync <name>` on a dirty repo invokes `git status --porcelain`, then `git add --all`, then `git commit -m "chore: sync via hop"`, then `git pull --rebase`, then `git push`, in that order.
A2. `hop sync <name>` on a clean repo invokes `git status --porcelain`, then `git pull --rebase`, then `git push`, skipping `git add` and `git commit`. Behavior is identical to the pre-change baseline.
A3. `hop sync <name> -m "<msg>"` on a dirty repo uses `<msg>` verbatim as the commit message; the default `chore: sync via hop` is not used when `-m` is present.
A4. `hop sync <name> -m "<msg>"` on a clean repo has no observable effect; no commit is produced; pull/push run as today.
A5. The `-m / --message` flag is registered on `sync` only — `pull` and `push` reject it with cobra's standard unknown-flag error.
A6. When the auto-commit step fails (non-zero exit from `git add` or `git commit`, including `pre-commit` hook rejection), `git pull --rebase` and `git push` MUST NOT run for that repo, and stderr contains `sync: <name> ✗ commit failed: <err>`.
A7. `hop sync <name>` MUST NOT pass `--no-verify` to `git commit` (verified by source audit and by a test in which a hook rejection causes the documented failure path).
A8. Per-repo success line on a dirty-tree sync matches `sync: <name> ✓ committed, <pull-summary>, <push-summary>`. Per-repo success line on a clean-tree sync matches today's `sync: <name> ✓ <pull-summary> <push-summary>` (no `committed,` token, no commas between summaries).
A9. The existing `sync: <name> ✗ rebase conflict — resolve manually with: git -C <path> rebase --continue` line and the existing `sync: <name> ✗ push failed: <err>` line are emitted exactly as today (no wording changes from this change).
A10. Batch summary line `summary: synced=N skipped=M failed=K` continues to count auto-committed-then-pushed repos as `synced` and commit-failed repos as `failed`.
A11. Each git invocation in the sync flow runs under its own `context.WithTimeout(context.Background(), cloneTimeout)`; no shared parent context caps the per-repo step sequence.
A12. Every new git invocation in `sync.go` routes through `internal/proc.RunCapture` or `proc.RunCaptureBoth` — no `exec.Command` outside `internal/proc`, no shell strings, no `os/exec` import in `cmd/hop/sync.go`.
A13. `src/cmd/hop/push.go` is unchanged by this change (verified by `git diff` showing no edits to that file).
A14. `src/cmd/hop/sync.go`'s `newSyncCmd` doc comment is updated to mention auto-commit on dirty trees.
A15. The `hop sync` row in `docs/memory/cli/subcommands.md`'s Inventory table reflects the new auto-commit step and the `-m / --message` flag. The "`hop pull` / `hop push` / `hop sync` per-line output" section documents the `committed,` prefix on success and the `sync: <name> ✗ commit failed: <err>` line on failure.
A16. The change builds and tests pass on darwin-arm64, darwin-amd64, linux-arm64, and linux-amd64 (existing CI matrix).

## Memory cross-references

- **`docs/memory/cli/subcommands.md` (modify)** — Update the `hop sync` row in the Inventory table to mention dirty-tree detection, auto-commit, and the `-m / --message` flag. Update the "`hop pull` / `hop push` / `hop sync` per-line output" section to document the `committed,` token on success and the new `sync: <name> ✗ commit failed: <err>` line on failure. Note that `hop push` is unchanged. (No new memory file warranted — this is a behavior delta on existing content.)

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Auto-commit is the default behavior for `hop sync` on dirty trees, not opt-in | Confirmed from intake #1 — user explicitly chose default-on; Constitution Principle III supports convention over flag | S:95 R:55 A:90 D:90 |
| 2 | Certain | `-m / --message` flag overrides the default commit message but does NOT toggle auto-commit on/off | Confirmed from intake #2 — user explicitly stated dirty trees are auto-committed with or without `-m` | S:95 R:80 A:95 D:95 |
| 3 | Certain | Default commit message is `chore: sync via hop` | Confirmed from intake #3 — user explicitly chose this exact string for conventional-commits + greppability | S:100 R:90 A:95 D:95 |
| 4 | Certain | Stage scope is `git add --all` (includes untracked files), matching xpush | Confirmed from intake #4 — user explicitly confirmed they want new files in dotfiles dir picked up | S:95 R:60 A:90 D:90 |
| 5 | Certain | Clean tree (empty `git status --porcelain`) skips commit and behaves identical to today's `hop sync` | Confirmed from intake #5 — user confirmed semantic match to xpush "No changes to commit, skipping" | S:95 R:90 A:95 D:95 |
| 6 | Certain | `hop push` is NOT changed; remains a pure `git push` wrapper | Confirmed from intake #6 — user explicitly chose the asymmetry; push-without-pull is riskier | S:100 R:75 A:95 D:95 |
| 7 | Certain | Pre-commit hooks are respected; no `--no-verify`. Hook failure aborts the repo's sync | Confirmed from intake #7 — matches Constitution Test Integrity ethos and "fix root causes" principle | S:95 R:75 A:95 D:90 |
| 8 | Certain | Dirty detection uses `git status --porcelain` (matches xpush) | Confirmed from intake #8 — user explicitly named the command | S:100 R:90 A:95 D:95 |
| 9 | Certain | Order of operations: `git status --porcelain` → `git add --all` → `git commit -m <msg>` → `git pull --rebase` → `git push`; any failure aborts that repo's sync | Confirmed from intake #9 — user pinned the sequence; existing batch-summary logic absorbs per-repo failures | S:95 R:75 A:95 D:95 |
| 10 | Certain | Constitution Principle IV (Wrap, Don't Reinvent) is satisfied — composing multiple git invocations into one verb is extension, not reinvention | Confirmed from intake #10 — captured in the Governance section above | S:95 R:90 A:95 D:90 |
| 11 | Certain | Status output for dirty-tree sync mentions the auto-commit (e.g., `committed,` prefix) so the user can tell hop made a commit | Confirmed from intake #11 — user confirmed in clarifications | S:95 R:95 A:85 D:75 |
| 12 | Certain | New flag is `-m`/`--message` (matches `git commit -m` muscle memory), `string` type, default `chore: sync via hop` | Confirmed from intake #12 — user confirmed in clarifications | S:95 R:90 A:90 D:85 |
| 13 | Certain | The auto-commit helper goes inline in `sync.go` (not extracted) unless another subcommand surfaces a similar need later | Confirmed from intake #13 — user confirmed; captured as a non-binding implementation hint | S:95 R:90 A:80 D:75 |
| 14 | Certain | Each git invocation (status/add/commit/pull/push) gets its own 10-minute timeout via `proc.RunCapture` and `cloneTimeout` (matches existing sync's per-call independent timeouts) | Confirmed from intake #14 — captured as the Per-call timeout requirement | S:95 R:90 A:95 D:90 |
| 15 | Certain | Failure message format follows existing pattern: `sync: <name> ✗ commit failed: <err>` (mirrors `push failed: <err>`) | Confirmed from intake #15 — user confirmed; mirror of existing `push failed:` convention | S:95 R:95 A:90 D:85 |
| 16 | Certain | The `sync.go` package-level doc comment is updated to reflect the new auto-commit behavior — clean tree still has no auto-stash, but dirty tree now commits | Confirmed from intake #16 — user confirmed; captured as the Doc Comment Updated requirement | S:95 R:95 A:95 D:90 |
| 17 | Certain | The status-line "committed" wording uses comma-separation: `sync: <name> ✓ committed, <pull-summary>, <push-summary>` | Confirmed from intake #17 — user confirmed comma-prefix shape over alternatives | S:95 R:95 A:75 D:55 |
| 18 | Certain | `git status --porcelain` runs as a separate proc invocation (not folded into another step) so its stdout can be inspected verbatim before deciding whether to enter the commit branch | Spec-stage decision — `--porcelain` is the documented detection signal; folding it into another command would obscure intent and break the "any step's failure aborts" rule cleanly | S:90 R:90 A:95 D:90 |
| 19 | Certain | The auto-commit branch runs only in the per-repo `syncOne` flow — no changes to `syncSingle`, `syncBatch`, or `runBatch`; `cobra.MaximumNArgs(1)` and the `--all`/positional usage-error rules remain identical | Spec-stage decision — keeps the change surface minimal; `runBatch` already absorbs per-repo failures correctly via the `(ok, gitMissing, err)` tuple | S:95 R:95 A:95 D:95 |
| 20 | Certain | Empty `-m ""` falls through to git's own validation rather than hop preempting it; multi-line `-m` values are passed verbatim as a single argv element to `git commit -m` | Spec-stage decision — surface area minimization; argv-slice passing is already the proc.RunCapture contract, so multi-line strings pass through trivially. Empty strings are a git-domain concern, not a hop-domain concern | S:90 R:90 A:90 D:80 |
| 21 | Certain | A successful auto-commit followed by a failed push leaves the local commit in place — no automatic rollback (no `git reset --soft HEAD^` on push failure) | Spec-stage decision — rolling back would silently discard a commit the user can re-push later via `hop push` or `git push`; the local commit is the safer state. Mirrors today's behavior where a successful pull followed by a failed push doesn't roll back the rebase | S:90 R:60 A:85 D:85 |
| 22 | Certain | The `-m / --message` flag is registered on `sync` only; `pull` and `push` reject it via cobra's unknown-flag handling | Spec-stage decision — Constitution Principle VI (Minimal Surface Area). The flag has no semantic on a verb that doesn't auto-commit | S:95 R:95 A:95 D:95 |

22 assumptions (22 certain, 0 confident, 0 tentative, 0 unresolved).
