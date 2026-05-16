# Plan: Worktree-Aware Path Resolution

**Change**: 260516-7eab-worktree-aware-path-resolution
**Status**: In Progress
**Intake**: `intake.md`
**Spec**: `spec.md`

## Tasks

### Phase 1: Setup

- [x] T001 Create `src/cmd/hop/wt_list.go` with the `WtEntry` struct (per spec § "WtEntry JSON contract"), a package-level `listWorktrees` var seam initialised to a function that builds a 5-second `context.WithTimeout`, invokes `proc.RunCapture(ctx, repoPath, "wt", "list", "--json")`, unmarshals into `[]WtEntry`, and returns the slice. Define a constant `wtListTimeout = 5 * time.Second`. Export nothing — package-internal.
- [x] T002 Write `src/cmd/hop/wt_list_test.go` covering: (a) successful unmarshal of a representative `[]WtEntry` JSON blob (use the seam to inject a fake returning canned bytes), (b) unmarshal failure on malformed JSON surfaces as an error, (c) the seam returns `proc.ErrNotFound` when the underlying runner does, (d) unknown JSON fields are ignored (forward-compat). No real `wt` binary required — all tests swap `listWorktrees` or its inner runner via the seam.

### Phase 2: Core Implementation

- [x] T003 Extend `src/cmd/hop/resolve.go::resolveByName` with first-`/` split logic. Add a helper `resolveWorktreePath(repo *repos.Repo, wtName string) (*repos.Repo, error)` in the same file: validates `repo.Path` exists on disk (clone state = `stateAlreadyCloned`), invokes `listWorktrees(ctx, repo.Path)`, finds the entry whose `Name == wtName` (exact, case-sensitive), and returns a shallow-copied `*repos.Repo` with `Path` replaced by the worktree's `Path` (other fields preserved). Wire `resolveByName` to (a) detect `/`, (b) reject empty LHS with `errExitCode{code: 2, msg: "hop: empty repo name before '/'"}`, (c) reject empty RHS with `errExitCode{code: 2, msg: "hop: empty worktree name after '/'"}`, (d) recurse / call existing match-or-fzf on LHS, (e) call `resolveWorktreePath` on the resolved repo. Map errors:
  - `proc.ErrNotFound` → write `"hop: wt: not found on PATH."` to `os.Stderr` and return `errSilent` (matches `open.go`).
  - Not-cloned → write `"hop: '<name>' is not cloned. Try: hop clone <name>"` to `os.Stderr` and return `errSilent`.
  - No-such-worktree → write `"hop: worktree '<wt>' not found in '<repo>'. Try: wt list (in <path>) or hop ls --trees"` to `os.Stderr` and return `errSilent`.
  - JSON/exec failure → write `"hop: wt list: <err>"` to `os.Stderr` and return `errSilent`.
  Tests in `src/cmd/hop/resolve_test.go` cover each scenario from spec § "Match Resolution: `/`-Split Algorithm" using the `listWorktrees` seam and the existing fixture helpers. Note: stderr writes go through `os.Stderr` because `resolveByName` has no `*cobra.Command` handle — this matches the existing `resolveByName` contract (`resolveOne` is the cobra-aware wrapper, but the new errors must be visible whether called via `resolveOne`, `runDashR`, or any other path).
- [x] T004 Extend `src/cmd/hop/ls.go::newLsCmd` with a `--trees` boolean flag (default false). When set, fan out across `loadRepos()` results in source order. For each repo: if not cloned (use existing `cloneState` from `clone.go`), emit `<name><pad>(not cloned)`. Otherwise call `listWorktrees(ctx, repo.Path)` — on success format `<name><pad><N> tree(s)  (<wt-list>)` where each wt is `name[*][↑N]` (`*` if `Dirty`, `↑N` if `Unpushed > 0`); on failure emit `<name><pad>(wt list failed: <err>)`. If the FIRST `wt list` invocation fails with `proc.ErrNotFound`, print `"hop: wt: not found on PATH."` to `cmd.ErrOrStderr()` and return `errSilent` (fail-fast per spec). Subsequent `ErrNotFound`s within the same run never occur because we abort on the first; document this in a comment. Use the existing alignment-width rule (longest name + 2). Singular "tree" when N == 1, "trees" otherwise. Tests in `src/cmd/hop/ls_test.go` cover: mixed registry (cloned + not-cloned), per-row wt failure, missing wt fast-fail, default `hop ls` unchanged (no wt invocation) via seam injection.
- [x] T005 Extend `src/cmd/hop/repo_completion.go::completeRepoNames` with the `/`-prefix branch. When `len(args) == 0` AND `toComplete` contains `/`: split on first `/`, find the configured repo whose `Name == lhs` (exact, case-insensitive to mirror the resolver's MatchOne tolerance — but only accept an UNAMBIGUOUS match; ambiguous LHS yields no candidates). If matched and the repo is cloned, invoke `listWorktrees` against the repo path; on success return `[]string{"<repo>/<wt1>", "<repo>/<wt2>", ...}` with `cobra.ShellCompDirectiveNoFileComp`. On ANY failure (not cloned, wt missing, JSON error, no LHS match, ambiguous LHS) return `nil, cobra.ShellCompDirectiveNoFileComp` silently — no stderr writes. Note: `completeRepoNames` currently receives `toComplete` as the third arg name `_`; rename it to `toComplete` to consume it. Update tests in `src/cmd/hop/repo_completion_test.go` with scenarios from spec § "Shell Integration".

### Phase 3: Integration & Edge Cases

- [x] T006 Add an end-to-end test `TestIntegrationWorktreeResolution` in `src/cmd/hop/integration_test.go` that uses `buildBinary`, installs a fake `wt` shell script (mirror `installFakeWt` from `open_test.go`) that responds to `wt list --json` with a canned JSON blob, and asserts: `hop <name>/<wt> where` prints the worktree's path; `hop <name>/main where` prints the main path; `hop <name>/missing where` exits 1 with the no-such-worktree stderr; `hop <name>/` exits 2 with the empty-RHS hint; `hop /<wt> where` exits 2 with the empty-LHS hint. Cover via the built binary so the cobra plumbing is exercised end-to-end.
- [x] T007 Update `src/cmd/hop/root.go::rootLong` to document the `/<wt-name>` suffix in the Usage section (a single line under the `hop <name>` block) and to document `hop ls --trees` near the existing `hop ls` line. Keep wording terse — this is reference, not a tutorial.

### Phase 4: Polish

- [x] T008 Run `cd src && go test ./...` and `cd src && go vet ./...` from the repo root to confirm the full suite passes and vet is clean. Run `cd src && gofmt -l ./...` to check formatting. Fix any failures before marking complete.

## Execution Order

- T001 blocks T002–T007 (helper + struct are the seam everything else uses).
- T002 can run alongside T003–T005 once T001 lands.
- T003, T004, T005 are independent of each other (different files) — `[P]`-eligible after T001.
- T006 depends on T003 (resolveByName plumbing) AND T004 (only if it asserts `--trees` end-to-end; otherwise just T003).
- T007 (root.go help text) is independent of everything except T001's existence.
- T008 runs last.

## Acceptance

### Functional Completeness

- [x] A-001 Grammar extension: `hop <name>/<wt-name>` resolves to the worktree's absolute path across `where`, `open`, `-R`, and shim `cd`/tool-form. All five verbs inherit via the single `resolveByName` seam.
- [x] A-002 `<name>/main` resolution: `hop <name>/main where` prints the main checkout's path without special-case branching in hop — the `wt list --json` `is_main: true` entry naturally carries the main path.
- [x] A-003 LHS-only queries: A bare query (no `/`) resolves byte-identically to pre-change behavior. `wt` is NOT invoked for `/`-less queries.
- [x] A-004 `hop ls --trees`: Fans out across configured cloned repos, emits per-row summaries in YAML source order, with non-cloned repos surfacing `(not cloned)`.
- [x] A-005 `--trees` per-row format: `{name}<spaces>{N} tree(s)  ({wt-list})` with `*` for dirty and `↑N` for unpushed; singular "tree" when N==1.
- [x] A-006 `--trees` fail-fast on missing wt: First `wt list` invocation hitting `ErrNotFound` prints `hop: wt: not found on PATH.` and exits 1.
- [x] A-007 `--trees` per-row degradation: Per-repo `wt list` failures emit `(wt list failed: <err>)` inline; the table is not aborted.
- [x] A-008 Default `hop ls` unchanged: Invocation without `--trees` produces byte-identical output to pre-change `hop ls` and does NOT invoke `wt`.
- [x] A-009 `/`-split algorithm: `resolveByName` splits on the first `/` (not the last), preserving multi-`/` worktree names.
- [x] A-010 Exact case-sensitive worktree name match: A worktree named `Feat-X` is NOT matched by RHS `feat-x`; the no-such-worktree path fires.
- [x] A-011 No-such-worktree error wording: stderr line matches `hop: worktree '<wt>' not found in '<repo>'. Try: wt list (in <repo-path>) or hop ls --trees`, exit 1.
- [x] A-012 Missing-wt error wording: stderr line matches `hop: wt: not found on PATH.` verbatim (no new wording), exit 1.
- [x] A-013 `wt list` failure surfacing: Non-zero exit or malformed JSON surfaces as `hop: wt list: <err>` with exit 1 — no silent fallback to the main path.
- [x] A-014 Empty RHS: `hop <name>/` exits 2 with `hop: empty worktree name after '/'`.
- [x] A-015 Empty LHS: `hop /<wt>` exits 2 with `hop: empty repo name before '/'`.
- [x] A-016 Repo-not-cloned guard: `/`-suffixed queries against uncloned repos exit 1 with the existing "not cloned" wording BEFORE `wt list` is invoked. Bare queries against uncloned repos remain permissive (today's behavior).

### Behavioral Correctness

- [x] A-017 `wt list --json` invocations go through `internal/proc.RunCapture` with `cmd.Dir = repo.Path` (Constitution Principle I). Source audit `grep --include='*.go' --exclude='*_test.go' -rn '"os/exec"' src/internal/ src/cmd/` returns no matches outside `src/internal/proc/`.
- [x] A-018 Per-call 5-second timeout matches `internal/scan` precedent.
- [x] A-019 `WtEntry` struct uses Go's default JSON unmarshalling (no `DisallowUnknownFields`) — future wt schema additions are silently ignored.

### Scenario Coverage

- [x] A-020 Spec scenarios under "CLI Surface: Grammar Extension" (where, open, -R, tool-form sugar, bare-name cd via shim, explicit main, backward-compatible bare query) are exercised by tests in `resolve_test.go` and `integration_test.go`.
- [x] A-021 Spec scenarios under "CLI Surface: `hop ls --trees` flag" (mixed registry, per-row format, empty wt list, per-row failure, missing wt, default ls unchanged) are exercised by tests in `ls_test.go`.
- [x] A-022 Spec scenarios under "Match Resolution: `/`-Split Algorithm" (multi-`/` query, case-sensitive miss, unknown worktree, missing wt, malformed JSON, empty RHS, empty LHS, uncloned guard) are exercised by tests in `resolve_test.go`.
- [x] A-023 Spec scenarios under "Shell Integration: Worktree Tab Completion" (TAB after `outbox/`, partial typing, uncloned repo, missing wt, verb-position unaffected) are exercised by tests in `repo_completion_test.go`.

### Edge Cases & Error Handling

- [x] A-024 Completion silent-failure: ANY failure mode during `<repo>/<TAB>` returns `nil` candidates + `NoFileComp` without writing to stderr.
- [x] A-025 Verb-position completion at `args[1]` (`cd`, `where`, `open`) is unchanged — the `/`-prefix branch operates only on `args[0]` / `toComplete`.
- [x] A-026 Helper `listWorktrees` is package-internal (lives in `cmd/hop/`), NOT in a new `internal/wt/` package — promote-later threshold not met.
- [x] A-027 Test seam: `listWorktrees` is a package-level `var`, mirroring `internal/fzf/fzf.go::runInteractive`. Tests can inject fakes without needing a real `wt` binary on PATH.

### Code Quality

- [x] A-028 Pattern consistency: new code follows existing `cmd/hop/` style (function naming, error wrapping with `errSilent`, cobra command structure).
- [x] A-029 No unnecessary duplication: reuses `cloneState`/`stateAlreadyCloned` from `clone.go` for the uncloned guard; reuses `proc.RunCapture` (does not reimplement subprocess invocation).
- [x] A-030 Readability over cleverness: each new function fits in well under 50 lines; no god functions.
- [x] A-031 No magic strings: error wordings, timeout, and any glyphs (`*`, `↑`) are extracted into named constants where they would otherwise repeat.

### Security

- [x] A-032 All `wt` invocations route through `internal/proc.RunCapture` with explicit argument slices — no shell strings, no direct `os/exec` outside `internal/proc/` (Constitution Principle I).

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`
