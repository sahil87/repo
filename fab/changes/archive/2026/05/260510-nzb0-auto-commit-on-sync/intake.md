# Intake: hop sync auto-commits dirty trees before pull/push

**Change**: 260510-nzb0-auto-commit-on-sync
**Created**: 2026-05-10
**Status**: Draft

## Origin

> User has a shell helper `xpush` (`/home/sahil/code/bootstrap/lifetracker/bin/xpush`) that, for a list of dotfiles-style repos, runs `git add --all :/`, `git commit -m "<msg>"`, `git pull --rebase`, then `git push`. The use case is "I edited a dotfile that's symlinked from the dotfiles repo, just get it to the cloud." `hop dotfiles pull` already replaces `xpull`, but `hop sync` doesn't replace `xpush` — it assumes changes are already committed. The user wants `hop sync` to fully replace `xpush` so they no longer need the bash helper.

The interaction was conversational: the user described the gap, then walked through ten sub-decisions with the agent. Every key decision below was explicitly agreed upon during that discussion — no inference, no defaults assumed silently.

Mode: **conversational** — the design space was explored and pinned before this intake was created.

## Why

1. **Problem**: `hop sync` today does `git pull --rebase && git push` per target repo, but it silently no-ops the most common reason the user reaches for a sync verb: an uncommitted edit to a tracked file (especially symlinked dotfiles). Today the user must (a) drop into the repo, (b) `git add && git commit`, then (c) run `hop sync`. That's three steps and an external dependency on `xpush` to compress them.
2. **Consequence if we don't fix it**: The user keeps the `xpush` bash helper around forever. `xpush` has a hardcoded `DIRS` array — every new dotfiles-adjacent repo requires editing the script. `hop.yaml` is already the source of truth for the same registry; not using it duplicates the inventory in two places (one declarative, one imperative-shell), which drifts. Users beyond the original author can't use `xpush` without copying and editing the script.
3. **Why this approach over alternatives**:
   - **Add a `--commit` flag to `hop sync`** — rejected. The user's use case is "I want my dirty tree synced"; making them remember a flag for the common case is friction. The flag would just be on every invocation.
   - **Add a separate `hop commit` subcommand** — rejected. Constitution Principle VI (Minimal Surface Area) — "could this be a flag on an existing subcommand?" Yes: `hop sync` already wraps the right git operations. Adding a top-level commit verb fails the justification test.
   - **Auto-commit on `hop push` too** — rejected. Pushing without rebasing is the riskier op; we don't want muscle-memory `hop push` to silently commit-and-push a dirty tree. `hop sync` is the safe verb because it always rebases first.
   - **Chosen**: extend `hop sync` to auto-commit dirty trees as default behavior, with `-m` to override the commit message. Same semantic surface as `xpush` but driven by `hop.yaml` instead of a hardcoded `DIRS` array.

## What Changes

### `hop sync` per-repo flow (when working tree is dirty)

Today's `hop sync <name>` flow:

```
git -C <path> pull --rebase
git -C <path> push
```

New flow:

```
# 1. Detect dirty
status=$(git -C <path> status --porcelain)
if [[ -n "$status" ]]; then
    # 2. Stage everything (untracked + modified + deleted)
    git -C <path> add --all
    # 3. Commit (default message, or -m override)
    git -C <path> commit -m "<message>"
fi

# 4. Existing rebase + push
git -C <path> pull --rebase
git -C <path> push
```

If `git status --porcelain` is empty, steps 2 and 3 are skipped and behavior is identical to today.

### New flag: `-m / --message <msg>`

Override the default commit message. Does NOT toggle auto-commit on/off — auto-commit is always on for dirty trees.

```
hop sync dotfiles                          # commits with "chore: sync via hop"
hop sync dotfiles -m "fix(zsh): reload"    # commits with "fix(zsh): reload"
hop sync --all -m "bulk update"            # all dirty repos in batch use the same message
```

Type: `string`. Default: `chore: sync via hop`. No short-form alias other than `-m`.

### Default commit message

`chore: sync via hop`

Chosen because:
- **Conventional Commits friendly**: `chore:` prefix is recognized by changelog tooling and signals "no behavior change you need to read the diff for"
- **Greppable**: `git log --grep "via hop"` finds every auto-commit later, so future-you can audit or rewrite if needed
- **Short**: 19 chars, doesn't pollute one-line `git log` output
- **Honest**: explicitly attributes the commit to hop, so the user can identify "I made this manually" vs "hop made this"

### Stage scope: `git add --all`

Stages tracked modifications, deletions, AND untracked files. Matches xpush's `git add --all :/`. The user explicitly confirmed they want new files in the dotfiles dir picked up — this is not just an incremental commit of tracked changes, it's a "snapshot the working tree" verb.

### Order of operations on dirty repo

1. `git add --all`
2. `git commit -m "<message>"` (respecting hooks — see below)
3. `git pull --rebase`
4. `git push`

Any step's failure aborts that repo's sync. The existing batch-mode summary (`synced=N skipped=M failed=K`) accommodates per-repo failures naturally.

### Pre-commit hooks

Hooks are respected. If a `pre-commit` (or `commit-msg`, `pre-push`) hook fails, the underlying git command fails, hop surfaces the failure verbatim, and that repo's sync aborts. **No `--no-verify`** — the project's "fix root causes" principle and Constitution Test Integrity-adjacent reasoning say we don't bypass user-installed tooling.

If a user wants to bypass hooks, they can use the underlying git commands directly or `hop -R <name> git commit --no-verify`.

### Status output

When auto-commit fires, the per-repo line should mention it:

```
sync: dotfiles ✓ committed, fast-forward, Everything up-to-date.
```

When the tree was clean (no commit fired), behavior matches today:

```
sync: dotfiles ✓ Already up to date. Everything up-to-date.
```

Exact wording is open for the spec stage — the principle is that the user should be able to tell from output alone whether hop made a commit on their behalf.

### Failure messages

- Commit failure (e.g., hook rejection): `sync: <name> ✗ commit failed: <last line of git stderr>` — the rebase/push are NOT attempted.
- Existing rebase-conflict and push-failure messages remain unchanged.

### `hop push` — explicitly NOT changed

The asymmetry is intentional. `hop push` stays a pure `git push` wrapper. Reasons:
- Pushing without pulling first is the riskier op; we don't want auto-commit-and-push without a rebase ahead of it
- Users have muscle memory for `git push` semantics; `hop push` should match
- Users wanting "commit + push without rebase" can use `hop -R <name> git ...` or compose

The package's existing `hop push` (`src/cmd/hop/push.go`) is untouched.

### Clean-tree behavior unchanged

If `git status --porcelain` is empty:
- No staging, no commit
- Rebase + push run as today
- The per-repo status line should look identical to today (no extra "clean tree" mention — the absence of "committed" in the output speaks for itself)

This matches xpush's "No changes to commit, skipping commit" semantic without adding noise.

### Sample full session

```
$ hop sync dotfiles
sync: dotfiles ✓ committed, fast-forward, Everything up-to-date.
$ git -C ~/code/sahil87/dotfiles log --oneline -1
a1b2c3d chore: sync via hop

$ hop sync dotfiles -m "fix(zsh): reload prompt on chpwd"
sync: dotfiles ✓ committed, fast-forward, Everything up-to-date.
$ git -C ~/code/sahil87/dotfiles log --oneline -1
d4e5f6a fix(zsh): reload prompt on chpwd

$ hop sync --all
sync: dotfiles ✓ committed, fast-forward, Everything up-to-date.
sync: outbox ✓ Already up to date. Everything up-to-date.
sync: hop ✗ commit failed: pre-commit hook failed (gofmt)
summary: synced=2 skipped=0 failed=1
```

## Affected Memory

- `cli/subcommands.md`: (modify) — Update the `hop sync` row in the inventory table to reflect the auto-commit step. Update the "`hop pull` / `hop push` / `hop sync` per-line output" section to add the `committed, ` prefix for sync's success line and the new `commit failed: <err>` failure line. Note the new `-m / --message` flag. Update the doc comment in `sync.go` (currently says "no auto-stash, no auto-resolve, no force-push") to reflect the new auto-commit behavior — clean tree still has no auto-stash, but dirty tree now auto-commits.

No new memory file warranted — the change is one paragraph's worth of behavior delta on existing `cli/subcommands.md` content. If the spec stage decides the auto-commit semantics deserve their own subsection (e.g., a "Sync auto-commit" anchor), that's a fine call at hydrate time.

## Impact

**Source files (entry points for the implementer)**:
- `src/cmd/hop/sync.go` — primary changes: register `-m / --message` flag, modify `syncOne` to call a new `commitDirtyTree` step before the existing rebase+push, update the per-repo status line emitter
- `src/cmd/hop/sync_test.go` — table-driven tests for: dirty tree → commit → rebase + push success; clean tree → no commit → rebase + push success; dirty tree → commit fails (hook rejection) → no rebase, no push; dirty tree → commit succeeds → rebase conflict (existing path); dirty tree → commit succeeds → push fails (existing path); `-m` flag overrides default message; batch mode mixes clean and dirty repos in the same run
- `src/cmd/hop/push.go` — NO changes (deliberately, as discussed)

**Possible refactor**: a `commitDirtyTree(ctx, path, msg)` helper. Whether it lives inline in `sync.go` or extracted to a shared file (alongside `lastNonEmptyLine` from `pull.go`) is the implementer's call. Recommend inline unless other subcommands grow a similar need.

**Tests**: existing `sync_test.go` continues to pass for the clean-tree path (regression-free); new test cases cover the dirty-tree paths.

**No new dependencies**: uses the same `internal/proc.RunCapture` and `git status` / `git add` / `git commit` invocations that the rest of the codebase already shells out to. Each git step gets its own 10-minute timeout (matches existing `cloneTimeout` reuse in `pull.go` / `sync.go`).

**APIs**: no public API surface change beyond the new `-m` flag. No `hop.yaml` schema change. No env var change.

## Open Questions

The discussion pinned 10 decisions explicitly. Remaining open at intake stage:

- Status-line wording for the auto-commit case — "committed, " prefix is the proposed shape but final wording is a spec-stage call (e.g., is it `committed, ` or `committed via hop, ` or just an extra `commit:` line?). Reversibility is high (one-line text change).
- Whether to surface the commit SHA in the status line — proposal: no, keep it short; users can `git log -1` if curious. Reversibility: high.
- Whether the spec wants the `commitDirtyTree` helper in a new file or inline — implementer's call at apply.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Auto-commit is the default behavior for `hop sync` on dirty trees, not opt-in | Discussed — user explicitly chose default-on, no flag required for the common case. Constitution Principle III (Convention Over Configuration) supports this | S:95 R:55 A:90 D:90 |
| 2 | Certain | `-m / --message` flag overrides the default commit message but does NOT toggle auto-commit on/off | Discussed — user explicitly stated "with or without `-m`, dirty trees are auto-committed" | S:95 R:80 A:95 D:95 |
| 3 | Certain | Default commit message is `chore: sync via hop` | Discussed — user explicitly chose this exact string for conventional-commits + greppability | S:100 R:90 A:95 D:95 |
| 4 | Certain | Stage scope is `git add --all` (includes untracked files), matching xpush | Discussed — user explicitly confirmed they want new files in dotfiles dir picked up | S:95 R:60 A:90 D:90 |
| 5 | Certain | Clean tree (empty `git status --porcelain`) skips commit and behaves identical to today's `hop sync` | Discussed — user confirmed semantic match to xpush "No changes to commit, skipping" | S:95 R:90 A:95 D:95 |
| 6 | Certain | `hop push` is NOT changed; remains a pure `git push` wrapper | Discussed — user explicitly chose the asymmetry. Push-without-pull is riskier | S:100 R:75 A:95 D:95 |
| 7 | Certain | Pre-commit hooks are respected; no `--no-verify`. Hook failure aborts the repo's sync | Discussed — matches Constitution Test Integrity ethos and "fix root causes" principle | S:95 R:75 A:95 D:90 |
| 8 | Certain | Dirty detection uses `git status --porcelain` (matches xpush) | Discussed — user explicitly named the command | S:100 R:90 A:95 D:95 |
| 9 | Certain | Order of operations: `git add --all` → `git commit -m <msg>` → `git pull --rebase` → `git push`; any failure aborts that repo's sync | Discussed — user pinned the sequence; existing batch-summary logic absorbs per-repo failures | S:95 R:75 A:95 D:95 |
| 10 | Certain | Constitution Principle IV (Wrap, Don't Reinvent) is satisfied — composing multiple git invocations into one verb is extension, not reinvention | Discussed — user explicitly considered this and concluded the principle governs replacing tools, not extending them | S:95 R:90 A:95 D:90 |
| 11 | Certain | Status output for dirty-tree sync mentions the auto-commit (e.g., `committed, ` prefix) so the user can tell hop made a commit | Clarified — user confirmed | S:95 R:95 A:85 D:75 |
| 12 | Certain | New flag is `-m`/`--message` (matches `git commit -m` muscle memory), `string` type, default `chore: sync via hop` | Clarified — user confirmed | S:95 R:90 A:90 D:85 |
| 13 | Certain | The `commitDirtyTree` helper goes inline in `sync.go` (not extracted) unless another subcommand surfaces a similar need later | Clarified — user confirmed | S:95 R:90 A:80 D:75 |
| 14 | Certain | Each git invocation (status/add/commit/pull/push) gets its own 10-minute timeout via `proc.RunCapture` and `cloneTimeout` (matches existing sync's per-call independent timeouts) | Clarified — user confirmed | S:95 R:90 A:95 D:90 |
| 15 | Certain | Failure message format follows existing pattern: `sync: <name> ✗ commit failed: <err>` (mirrors `push failed: <err>`) | Clarified — user confirmed | S:95 R:95 A:90 D:85 |
| 16 | Certain | The `sync.go` package-level doc comment ("no auto-stash, no auto-resolve, no force-push") will be updated to reflect the new auto-commit behavior — clean tree still has no auto-stash, but dirty tree now commits | Clarified — user confirmed | S:95 R:95 A:95 D:90 |
| 17 | Certain | The status-line "committed" wording uses comma-separation: `sync: <name> ✓ committed, <pull-summary>, <push-summary>` | Clarified — user confirmed (chose comma-prefix over dedicated commit-line or verb-swap shapes) | S:95 R:95 A:75 D:55 |

17 assumptions (17 certain, 0 confident, 0 tentative, 0 unresolved).

## Clarifications

### Session 2026-05-10

| # | Action | Detail |
|---|--------|--------|
| 17 | Confirmed | Comma-prefix shape: `sync: <name> ✓ committed, <pull>, <push>` (chose over dedicated commit-line or verb-swap alternatives) |
| 11 | Confirmed | (bulk) Status mentions auto-commit |
| 12 | Confirmed | (bulk) `-m`/`--message` string flag, default `chore: sync via hop` |
| 13 | Confirmed | (bulk) `commitDirtyTree` helper inline in `sync.go` |
| 14 | Confirmed | (bulk) Per-call 10-minute timeouts via `cloneTimeout` |
| 15 | Confirmed | (bulk) Failure shape `sync: <name> ✗ commit failed: <err>` |
| 16 | Confirmed | (bulk) Update `sync.go` package doc comment to reflect auto-commit behavior |
