# Intake: Add `hop push` subcommand

**Change**: 260508-rdgf-add-hop-push-subcommand
**Created**: 2026-05-08
**Status**: Draft

## Origin

> Just like we have hop pull <repo> and hop sync <repo>, add a hop push <repo>

One-shot natural-language request. The `hop pull` and `hop sync` subcommands shipped together in #17 (`d4c5cf1`). This change extends that pair with `hop push`, completing the symmetry: `pull` (fetch+merge), `push` (publish only), `sync` (rebase+push as one unit).

## Why

1. **Asymmetric gap.** `hop pull <name-or-group>` and `hop sync <name-or-group>` exist; there is no `hop push`. The current way to push without re-pulling is `hop -R <name> git push` (single repo only — no group/`--all` form, no per-repo summary).
2. **Real workflow.** Pushing without first pulling is common after `hop sync` rebase failures, after local commits made via tool-form, or after `hop -R name git commit ...` flows. Forcing a `pull --rebase` (via `sync`) before push is wrong when the user knows the local branch is already up to date.
3. **Constitution VI justification — could this be a flag on an existing subcommand?** Considered: `hop sync --no-pull` (overloads `sync`, fights its name) and `hop pull --push-after` (mixes verbs, surprising). Rejected. `push` is its own verb in git; users will reach for `hop push <name>` by analogy with `hop pull <name>`. A new top-level subcommand is the clean answer; the tool-form fallback (`hop -R name git push`) remains for users who want it.
4. **Cost is near-zero.** `hop pull` and `hop sync` already share `runBatch` (`batch.go`), `resolveTargets`, and the per-repo status-line conventions. `hop push` reuses all three — implementation is essentially a third file mirroring `pull.go` with `git push` instead of `git pull`.

## What Changes

### New subcommand: `hop push [<name-or-group>] [--all]`

A direct sibling of `hop pull`. Wraps `git push` over a single repo (substring match on `Name`), every cloned repo in a named group (exact group match), or every cloned repo in the registry (`--all`). Same signature, flag set, resolution rules, and output conventions as `hop pull`.

**File**: new `src/cmd/hop/push.go` modeled directly on `src/cmd/hop/pull.go`.

```go
// newPushCmd builds the `hop push` subcommand.
//
//	hop push [<name-or-group>] [--all]
func newPushCmd() *cobra.Command {
    var all bool
    cmd := &cobra.Command{
        Use:               "push [<name-or-group>] [--all]",
        Short:             "Run 'git push' in a repo, group, or every cloned repo with --all",
        Args:              cobra.MaximumNArgs(1),
        ValidArgsFunction: completeRepoOrGroupNames,
        RunE: func(cmd *cobra.Command, args []string) error {
            // identical shape to pull.RunE, with "push" in error messages
            // and pushSingle/pushBatch dispatch
        },
    }
    cmd.Flags().BoolVar(&all, "all", false, "run 'git push' in every cloned repo from hop.yaml")
    return cmd
}
```

**Per-repo op (mirrors `pullOne`)**:

```go
func pushOne(cmd *cobra.Command, r repos.Repo) (ok, gitMissing bool, err error) {
    ctx, cancel := context.WithTimeout(context.Background(), cloneTimeout)
    defer cancel()
    out, err := proc.RunCapture(ctx, r.Path, "git", "push")
    if err != nil {
        if errors.Is(err, proc.ErrNotFound) {
            return false, true, err
        }
        fmt.Fprintf(cmd.ErrOrStderr(), "push: %s ✗ %v\n", r.Name, err)
        return false, false, err
    }
    fmt.Fprintf(cmd.ErrOrStderr(), "push: %s ✓ %s\n", r.Name, lastNonEmptyLine(string(out)))
    return true, false, nil
}
```

**Batch dispatch**: `runBatch(cmd, targets, "push", "pushed", pushOne)`.

### Wiring (`root.go`)

Add `newPushCmd()` to `rootCmd.AddCommand(...)` next to `newPullCmd()` and `newSyncCmd()`.

### Help text (`root.go::rootLong`)

Insert three new lines in the `Usage:` block, between the `pull` lines and `sync` lines (or after `sync` — see Open Questions below):

```
  hop push <name>           Run 'git push' in the named repo
  hop push <group>          Run 'git push' in every cloned repo of <group>
  hop push --all            Run 'git push' in every cloned repo
```

Update the `Notes:` block: change "`pull` and `sync` accept …" to "`pull`, `push`, and `sync` accept …".

### Shell shim (`shell_init.go::posixInit`)

Update the known-subcommand alternation so `push` is dispatched as a subcommand (not tool-form):

```sh
# before
clone|pull|sync|ls|shell-init|config|update|help|--help|-h|--version|completion)
# after
clone|pull|push|sync|ls|shell-init|config|update|help|--help|-h|--version|completion)
```

Without this, the shim's rule 5 would route `hop push <name>` to tool-form (`command hop -R push <name>`) and the binary would error with `hop: -R: 'push' not found.`. The same fix was applied for `pull` and `sync` in #17 — same reasoning here.

### Tests

New `src/cmd/hop/push_test.go` mirroring `pull_test.go` test-by-test:
- single-repo success
- single-repo not-cloned skip → exit 1
- ambiguous match resolution (delegates to shared resolver — covered indirectly by pull's tests, but a smoke test confirms wiring)
- group match (batch summary `summary: pushed=N skipped=M failed=K`)
- `--all`
- usage errors (`--all` + positional, missing positional + missing `--all`)
- `git` missing → emits `gitMissingHint` once, aborts batch (no further repos, no summary)

The shared `runBatch` and `resolveTargets` are already covered by pull/sync tests — no new tests needed for those.

### Per-repo output format

All status lines go to **stderr** (consistent with `pull`/`sync` — stdout is empty so the shim does not `cd`):

- `push: <name> ✓ <last-line>` — success, where `<last-line>` is the last non-empty line of git's stdout via `lastNonEmptyLine` (e.g., `Everything up-to-date`, `<refs> -> <ref>`).
- `push: <name> ✗ <err>` — failure (non-fast-forward, no upstream, network, etc.). git's own stderr is forwarded verbatim by `proc.RunCapture`; the hop line summarizes for the per-repo log.
- `skip: <name> not cloned` — `<path>/.git` missing.
- `summary: pushed=<N> skipped=<M> failed=<K>` — batch mode only.

### Exit codes

Identical to `pull`:

| Code | Trigger |
|---|---|
| 0 | All targets pushed (or batch with `failed == 0`) |
| 1 | Single-repo `not cloned`, push failure, or batch `failed > 0` (`errSilent`); also `git` missing |
| 2 | Usage error (`--all` + positional, or missing both) |
| 130 | fzf cancelled (when single-repo resolution invokes fzf) |

## Affected Memory

- `cli/subcommands.md`: (modify) Add a `hop push` row to the Inventory table; extend the existing "`hop pull` / `hop sync` per-line output" section to "`hop pull` / `hop push` / `hop sync` per-line output" (or split — see Open Questions); update the shell shim's known-subcommand alternation listing in the `posixInit` notes; update `pull` and `sync` row references where they cross-reference each other to also mention `push`.

## Impact

**Source files**:
- `src/cmd/hop/push.go` — new file (mirrors `pull.go`, ~130 lines).
- `src/cmd/hop/push_test.go` — new test file (mirrors `pull_test.go`).
- `src/cmd/hop/root.go` — register `newPushCmd()`; update `rootLong` Usage and Notes blocks.
- `src/cmd/hop/shell_init.go::posixInit` — add `push` to the known-subcommand alternation.

**No changes**:
- `src/cmd/hop/batch.go` — `runBatch` is generic over a `batchOp`; no signature change needed.
- `src/cmd/hop/resolve.go` (or wherever `resolveTargets` lives) — same resolver is reused.
- `src/internal/proc` — `RunCapture` already supports the use case.
- `internal/repos`, config code, completion plumbing — no changes.

**Memory**:
- `docs/memory/cli/subcommands.md` — table addition, output-section rename or addition, shim alternation update.
- `docs/specs/cli-surface.md` — Subcommand Inventory row addition; `Usage:` block enumeration update; potentially the Help Text section.

**No DB, no new external tools, no platform-specific code.**

## Open Questions

- None blocking. One minor presentation choice (Open Question 1 below) can be resolved during spec.

1. **Help-text ordering** — list `push` between `pull` and `sync` (alphabetical / "git verb" order: pull, push, sync) or after `sync` (chronological / "complexity" order)? Pull/Push/Sync reads naturally; spec stage will pick.
2. **Memory section title** — keep one combined "`hop pull` / `hop push` / `hop sync` per-line output" section, or split into three? Combined preferred (output formats are nearly identical); spec stage will pick.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Subcommand name is `push` (not `publish`, `up`, etc.) | User said "hop push <repo>" verbatim; matches git's verb | S:100 R:90 A:95 D:100 |
| 2 | Certain | Signature mirrors `hop pull` exactly: `[<name-or-group>] [--all]`, `cobra.MaximumNArgs(1)` | User said "just like we have hop pull"; pull/sync use this exact shape | S:95 R:85 A:95 D:95 |
| 3 | Certain | Reuses `runBatch` and `resolveTargets` rather than duplicating logic | Constitution IV (Wrap, Don't Reinvent); the helpers already exist for this exact purpose | S:90 R:90 A:100 D:100 |
| 4 | Certain | Implementation goes in `src/cmd/hop/push.go` modeled on `pull.go` | Project layout convention — every subcommand has its own file in `src/cmd/hop/` | S:95 R:90 A:100 D:100 |
| 5 | Certain | Per-repo timeout is `cloneTimeout` (10 minutes), reused from `clone.go` | `pull` and `sync` use this same constant; consistency wins, push is bandwidth-bound just like pull | S:90 R:80 A:95 D:95 |
| 6 | Certain | All output goes to stderr; stdout is empty (so the shim does not `cd`) | `pull` and `sync` follow this rule for the same reason — push is not a path-emitter | S:95 R:85 A:100 D:100 |
| 7 | Certain | Shell shim's known-subcommand alternation must include `push` | Same #17 lesson that added `pull|sync`; without it, rule 5 misroutes to tool-form | S:95 R:75 A:100 D:100 |
| 8 | Certain | git missing aborts the batch immediately (single emission of `gitMissingHint`, no summary) | `runBatch` already encodes this; push inherits it for free | S:100 R:90 A:100 D:100 |
| 9 | Certain | Per-repo summary line uses `lastNonEmptyLine(stdout)` for the success suffix | Same helper used by pull/sync; git push's terminal output (e.g., `<src> -> <dst>`) is meaningful | S:90 R:85 A:95 D:95 |
| 10 | Certain | Exit-code policy mirrors pull (0 success / 1 failure or skip-in-single / 2 usage / 130 fzf cancel) | Symmetric verbs should have symmetric exit codes; pull's policy is the canonical pattern | S:95 R:85 A:100 D:100 |
| 11 | Certain | No `--force`, no `--set-upstream`, no extra flags beyond `--all` | Constitution III (Convention Over Configuration); users wanting these reach for `hop -R <name> git push --force` | S:85 R:80 A:90 D:90 |
| 12 | Confident | Memory updates land in `cli/subcommands.md` and the spec at `docs/specs/cli-surface.md`; no new memory file | Pull/sync did not get a new memory file in #17 — they extended the existing CLI memory; same rationale | S:80 R:75 A:90 D:85 |
| 13 | Confident | Help-text Usage block lists push between pull and sync ("pull, push, sync" reads natural) | Mild aesthetic preference; trivially reversible at review | S:70 R:95 A:75 D:75 |
| 14 | Confident | The combined memory section becomes "`hop pull` / `hop push` / `hop sync` per-line output" rather than splitting | Output formats are nearly identical; combining keeps the doc compact | S:75 R:95 A:80 D:75 |
| 15 | Confident | Tests mirror `pull_test.go` test-by-test, no extra coverage for shared helpers | Shared helpers are already tested via pull/sync; duplicating is waste | S:80 R:80 A:90 D:85 |

15 assumptions (11 certain, 4 confident, 0 tentative, 0 unresolved).
