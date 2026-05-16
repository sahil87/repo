# Intake: Add `hop pull` and `hop sync` Subcommands

**Change**: 260507-xj3k-add-pull-sync-subcommands
**Created**: 2026-05-07
**Status**: Draft

## Origin

> Add a "hop pull <repo-name>" and "hop sync <repo-name>" (Or should this be hop <repo-name> pull|sync ? )

The user raised the grammar question (top-level subcommand vs. `$2` repo-verb) directly in the prompt. Resolved conversationally before intake generation:

1. **Grammar**: top-level subcommands (`hop pull <name>`, `hop sync <name>`) — parallel to `hop clone <name>`. Per-repo operations that produce no shell-cwd side effect fit the top-level form; the repo-verb slot ($2) stays reserved for shell-integrated forms (`cd`, `where`) and exec wrappers (`-R`, tool-form). Constitution VI justification recorded under Design Decisions in the spec.
2. **Verb semantics**: `pull` = `git pull` (fetch + merge on current branch); `sync` = `git pull --rebase` then `git push` (linear history, bidirectional).
3. **Batch scope**: positional argument accepts a **repo name OR a group name** from `hop.yaml`, plus an `--all` flag for "every cloned repo." Ambiguity tiebreaker (when a repo and a group share a name) is an open question — see Open Questions.
4. **Sync conflict policy**: `git pull --rebase` first; if rebase fails, surface git's error and exit non-zero (no auto-resolution, no merge fallback).

## Why

**Problem.** hop today is a locator — it can find, list, clone, and exec inside repos, but it cannot keep them up to date. Maintaining N cloned repos means manually `cd`ing into each and running `git pull` (and optionally `git push`). For a registry-driven workflow, the missing verbs are obvious: "pull this repo" and "sync this repo" (where sync means bidirectional).

**Consequence of not fixing.** Users either (a) write their own shell loops (`for d in $(hop ls | awk '{print $2}'); do (cd $d && git pull); done`), (b) use a separate tool like `mr` or `gita`, or (c) live with stale local checkouts. None of these compose well with hop's existing grammar — the user is forced out of the `hop` mental model the moment they want to update.

**Why these two verbs over alternatives.** `pull` is the safe, read-only unidirectional update — matches the dominant `git pull` mental model. `sync` is the bidirectional verb (rebase + push) for repos where the user actively commits; it composes the two operations users almost always run together. Splitting them lets the user choose the safety/throughput tradeoff per invocation rather than baking one in.

**Why top-level (not repo-verb).** Three reasons:

1. **Parallel to `clone`**. `hop clone <name>` is already the top-level per-repo registry verb; `pull` and `sync` extend that pattern. The mental model is "registry operations live at $1; shell-integrated and exec verbs live at $2."
2. **Batch composability**. Top-level lets the same verb take `<name>`, a group, or `--all` — `hop pull --all`, `hop pull default`, `hop pull outbox` all read naturally. The repo-verb form would require ugly synthesis (`hop --all pull`?) or a separate batch verb.
3. **No shell-integration coupling**. `cd` and `where` need shell-integration awareness (the binary errors with hints when invoked directly); `pull` and `sync` are pure subprocess wrappers — they always work in the binary. They don't belong in the shell-only $2 slot.

The cost is two new top-level subcommands (Constitution VI requires justification, captured here and in the spec's Design Decisions section).

## What Changes

### New top-level subcommand: `hop pull`

Wraps `git pull` for one repo, a group of repos, or every cloned repo.

**Signature**:
```
hop pull [<name-or-group>] [--all]
```

**Argument resolution** (positional):

1. If `--all` is passed, ignore positional and iterate every cloned repo from `hop.yaml`.
2. If positional matches a **group name** in `hop.yaml` exactly (case-sensitive), iterate every repo in that group that is cloned.
3. Otherwise, treat as a **repo name** and apply the standard match algorithm (case-insensitive substring on `Name`; fzf for ambiguous/zero matches; `--select-1` for narrow-to-1).
4. If positional is omitted AND `--all` is absent, exit 2 with usage error.

**Per-repo behavior** (single repo and each repo in batch mode):

1. Resolve absolute path. If `<path>/.git` does not exist → emit `skip: <name> not cloned` to stderr; continue (batch) or exit 1 (single).
2. Run `git -C <path> pull` via `internal/proc.RunCapture` with a 10-minute timeout. (No `--rebase` — `pull` is a verbatim wrapper.)
3. On success: emit `pull: <name> ✓ <one-line summary>` to stderr. The summary is git's stdout last line (e.g., "Already up to date." or "Fast-forward").
4. On failure: emit `pull: <name> ✗ <error>` to stderr; record failure for batch summary.

**Batch summary** (multi-repo invocations only):

Final stderr line: `summary: pulled=N skipped=M failed=K`. Exit 0 if `K == 0`, else 1.

**Exit codes**:
- 0: all pulls succeeded (or "already up to date")
- 1: any pull failed; or single-repo resolution failure; or repo not cloned (single mode); or git missing
- 2: usage error (no positional and no `--all`); or `--all` combined with positional
- 130: fzf cancelled

### New top-level subcommand: `hop sync`

Wraps `git pull --rebase` + `git push` for one repo, a group, or every cloned repo. Same signature shape as `pull`.

**Signature**:
```
hop sync [<name-or-group>] [--all]
```

**Per-repo behavior**:

1. Resolve absolute path. If `<path>/.git` does not exist → emit `skip: <name> not cloned` to stderr; continue (batch) or exit 1 (single).
2. Run `git -C <path> pull --rebase` via `internal/proc.RunCapture` (10-minute timeout).
   - On rebase conflict (git exits non-zero with "CONFLICT" in stderr): emit `sync: <name> ✗ rebase conflict — resolve manually with: git -C <path> rebase --continue` to stderr; record failure; do NOT push.
   - On other failure: emit `sync: <name> ✗ <error>`; record failure.
3. On rebase success, run `git -C <path> push`.
   - Push failure (e.g., remote rejected non-fast-forward, network): emit `sync: <name> ✗ push failed: <error>`; record failure.
   - No commits to push: git returns "Everything up-to-date" — treat as success.
4. On full success: emit `sync: <name> ✓ <pull-summary> <push-summary>` to stderr.

**No auto-stash, no force-push, no auto-resolve.** If the working tree is dirty, `git pull --rebase` will refuse — that error surfaces verbatim. The user is expected to commit or stash first.

**Batch summary**: `summary: synced=N skipped=M failed=K`. Same exit code policy as `pull`.

### Stdout/stderr conventions

Both `pull` and `sync` follow `hop clone`'s convention:
- **stdout**: empty (no path printing, no log capture). The shim does not `cd` after these verbs.
- **stderr**: per-repo status lines (`pull: <name> ✓|✗ ...`, `skip: ...`) and the batch summary line. Errors and hints.

This matches `hop clone`'s output shape — these are "do work, report it" verbs, not path-printers.

### Help text

`rootLong` in `src/cmd/hop/root.go` adds two lines to the `Usage:` block (after the existing `hop clone …` rows):

```
  hop pull <name>           Run 'git pull' in the named repo
  hop pull <group>          Run 'git pull' in every cloned repo of <group>
  hop pull --all            Run 'git pull' in every cloned repo
  hop sync <name>           Run 'git pull --rebase' then 'git push' in <name>
  hop sync <group>          Run sync in every cloned repo of <group>
  hop sync --all            Run sync in every cloned repo
```

`Notes:` block gains a bullet:
> `pull` and `sync` are bulk-friendly: a positional argument is treated first as a group name (exact match), then as a repo name (substring). Use `--all` for the full registry. `sync` is `pull --rebase` + `push` — linear history, no auto-resolve on conflict.

### Code layout

Two new files following existing per-subcommand convention:

- `src/cmd/hop/pull.go` — `newPullCmd()` factory, argument resolver (`resolveTargets`), per-repo runner, batch summary printer.
- `src/cmd/hop/sync.go` — `newSyncCmd()` factory, reusing the resolver from `pull.go` (extracted to a shared helper). The rebase+push composition is inline.

Both subcommands MUST route all subprocess calls through `internal/proc.RunCapture` (Constitution Principle I). No direct `os/exec`.

A small shared helper for "iterate cloned repos in a target set, collect results, print summary" is extracted to keep `pull.go` and `sync.go` short. Candidate location: `src/cmd/hop/batch.go` or inline in `pull.go` if the helper stays under ~30 lines.

### Test coverage

Match existing test patterns:
- `pull_test.go` — argument resolution (name vs. group vs. `--all`), missing-clone skip, batch summary, fake-git harness via `internal/proc` test seam.
- `sync_test.go` — rebase-conflict path, push-failure path, dirty-tree refusal, batch summary, group resolution.
- `integration_test.go` extension — end-to-end against a temp git server for the happy path.

## Affected Memory

- `cli/subcommands`: (modify) add `pull` and `sync` rows to the inventory; update Usage block reference.
- `cli/match-resolution`: (modify) document the **name-or-group** positional resolution shared by `pull` and `sync` (exact group match first, then substring repo match). This is a NEW resolution mode — not the same as the existing `Name`-only substring match.
- `architecture/package-layout`: (modify) note `pull.go` and `sync.go` under `src/cmd/hop/`; mention the shared batch helper if extracted.

No new memory domain; everything fits under existing `cli` and `architecture` domains.

## Impact

**Code areas**:
- `src/cmd/hop/` — new `pull.go`, `sync.go`, possibly `batch.go`.
- `src/cmd/hop/root.go` — `rootCmd.AddCommand(newPullCmd(), newSyncCmd())`; update `rootLong` Usage and Notes blocks.
- `src/cmd/hop/main.go` — no change (cobra dispatch handles new subcommands automatically).
- `src/internal/repos/` (or wherever `hop ls` derives the repo list) — may extend with `ResolveByGroup(name string) []Repo` if not already present.

**Dependencies**: No new Go modules. `git` is already a dependency for `hop clone` and `hop config scan`.

**Cross-platform**: Pure subprocess wrappers — work identically on darwin-arm64, darwin-amd64, linux-arm64, linux-amd64. No platform-specific paths.

**Backwards compatibility**: Pure addition. No existing subcommand changes shape. The `hop <name> <tool>` shim sugar already routes `hop outbox pull` → `command hop -R outbox pull` (running the `pull` *binary* if installed); after this change, the user-facing `hop pull <name>` form is the new top-level subcommand and the shim does NOT route it through tool-form (rule 3 — known subcommand — wins over rule 5).

**Shell completion**: Cobra-generated completion picks up the new subcommands automatically. The positional-arg completion for `pull`/`sync` SHOULD complete repo names AND group names (since the positional is name-or-group). New completion function: `completeRepoOrGroupNames` in `src/cmd/hop/repo_completion.go`.

## Open Questions

_All open questions resolved during intake — see Assumptions table below._

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Top-level subcommands (`hop pull <name>`, `hop sync <name>`) rather than repo-verb form | User explicitly chose this in pre-intake conversation; matches `hop clone` parallel structure | S:95 R:80 A:80 D:90 |
| 2 | Certain | `pull` = `git pull`; `sync` = `git pull --rebase` + `git push` | User-confirmed in pre-intake conversation | S:95 R:70 A:75 D:90 |
| 3 | Certain | Positional argument accepts name OR group (exact group match first, then substring repo match), plus `--all` | User-selected in pre-intake conversation | S:90 R:65 A:75 D:85 |
| 4 | Certain | Sync rebase failure surfaces verbatim — no auto-stash, no auto-resolve, no merge fallback | Constitution Principle IV (wrap, don't reinvent) — rule-determined; git's error is the right error | S:90 R:80 A:95 D:90 |
| 5 | Certain | Both subcommands route subprocess calls through `internal/proc.RunCapture` with 10-minute timeout | Constitution Principle I (security first) mandates `proc` for all subprocess — rule-determined; 10min matches `clone` precedent | S:95 R:85 A:95 D:90 |
| 6 | Certain | stdout is empty; per-repo status and batch summary go to stderr | Precedent rule — matches `hop clone` exactly; spec-documented convention for "do work, report it" verbs | S:90 R:80 A:95 D:90 |
| 7 | Confident | Batch operations run sequentially (not concurrently) | Predictable output ordering matters more than wall-clock for small registries; concurrency is a follow-up | S:70 R:80 A:80 D:75 |
| 8 | Certain | Skip-if-not-cloned in batch mode (don't fail the whole batch) | Precedent rule — matches `hop clone --all` skip-if-already-cloned posture verbatim | S:90 R:85 A:95 D:90 |
| 9 | Certain | No new memory domain — extends `cli` and `architecture` | Verified by reading `docs/memory/index.md` — both domains exist with relevant files | S:95 R:90 A:95 D:95 |
| 10 | Confident | Group-vs-repo name collision: group wins (exact match beats substring) | Collision is theoretical (group names user-curated, repo names URL-derived); per-repo status output makes mismatch obvious; user can rename the group in `hop.yaml` to escape | S:75 R:70 A:80 D:75 |
| 11 | Confident | Cobra completion completes both repo names and group names in the positional slot | Discoverability matters for completion; group count is small (typically 1-5) so list growth is minimal; trivial extension of existing `completeRepoNames` | S:80 R:85 A:85 D:80 |
| 12 | Certain | Drop `--no-fetch` flag from `pull`; rely on `hop <name> -R git merge FETCH_HEAD` for the niche workflow | Constitution VI (minimal surface area) — rule-determined; tool-form already covers the use case | S:90 R:90 A:90 D:85 |
| 13 | Confident | Pass git's error through verbatim for `sync` failures (detached HEAD, no upstream, dirty tree, rebase conflict, push rejected). MAY prepend a one-line hop hint when stderr matches detached-HEAD or no-upstream patterns. | Constitution IV — git's errors are already precise; re-implementing pre-checks duplicates state machine. Hint matching is opt-in and cheap. | S:75 R:75 A:80 D:75 |
| 14 | Confident | No `--continue` flag for resumable batches in v1 | Re-running `pull --all` is cheap (already-up-to-date is fast); persisted state would violate Constitution II (no database/cache); easy to add later if needed | S:80 R:85 A:85 D:80 |

14 assumptions (9 certain, 5 confident, 0 tentative, 0 unresolved).
