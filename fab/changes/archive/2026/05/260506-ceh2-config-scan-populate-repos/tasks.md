# Tasks: hop config scan — populate hop.yaml from on-disk repos

**Change**: 260506-ceh2-config-scan-populate-repos
**Spec**: `spec.md`
**Intake**: `intake.md`

<!--
  TASK FORMAT: - [ ] {ID} [{markers}] {Description with file paths}

  Markers:
    [P]   — Parallelizable (different files, no dependencies on other [P] tasks in same group)

  Tests are co-located with the code they test (test-alongside per code-quality.md).
-->

## Phase 1: Setup & scaffolding

<!-- Establish file/package boundaries and type stubs. No business logic. -->

- [x] T001 [P] Create `src/internal/scan/` package skeleton: `src/internal/scan/scan.go` with package declaration, exported types `Found`, `Skip`, `Options` (matching spec § "Filesystem walk: `internal/scan` package — Public API"), and a stub `Walk(ctx, root, opts) ([]Found, []Skip, error)` returning `nil, nil, nil`. Create `src/internal/scan/scan_test.go` with package declaration only.
- [x] T002 [P] Add type stubs to `src/internal/yamled/yamled.go`: exported `ScanPlan` and `InventedGroup` structs (per spec § "MergeScan signature"), plus a stub `MergeScan(path string, plan ScanPlan) error` returning `nil`. No logic yet.
- [x] T003 [P] Add a `newConfigScanCmd()` factory stub to `src/cmd/hop/config.go` (returns a `*cobra.Command` with `Use`, `Short`, `Args: cobra.ExactArgs(1)`, and a no-op `RunE`); register it on the `config` parent in `newConfigCmd` alongside `init` and `where`.

## Phase 2: Core implementation

<!-- Depth-ordered. Tasks in different files within the same horizontal layer can land in parallel; CLI wiring depends on all three. -->

### Layer 2a — leaf packages (parallel)

- [x] T004 [P] Implement `RunCapture(ctx context.Context, dir, name string, args ...string) ([]byte, error)` in `src/internal/proc/proc.go` per spec § "Git invocation contract": sets `cmd.Dir = dir`, captures stdout, routes stderr to parent. Reuse the existing `proc.ErrNotFound` mapping. Add unit tests in `src/internal/proc/proc_test.go` covering: success path, non-zero exit propagation, missing-binary → `ErrNotFound`, context-cancel/timeout. Decision deferred to apply: the implementation MAY instead extend `Run` to accept a `dir` and rename — pick whichever minimizes call-site churn.
- [x] T005 [P] Implement `Walk` in `src/internal/scan/scan.go`: stack-based DFS, depth handling (root = depth 0, inclusive bound), classifier rules in order (worktree → submodule → normal repo → bare repo → plain dir per spec § "Repo classification rules"), `(dev, inode)` symlink dedup (silent — no `Skip`), `filepath.EvalSymlinks` canonicalization on `Found.Path`, DFS discovery order preservation, and 5-second `context.WithTimeout` per `git` invocation via the injected `Options.GitRunner` (default = `internal/proc.RunCapture`). Submodule heuristic: implementor picks the simpler approach unless tests demonstrate the no-descent invariant alone is insufficient (spec assumption #17 / NOTE under "Submodule detection via ancestor stack"). Add `src/internal/scan/scan_test.go` table-driven tests over a synthesized `t.TempDir()` tree with fake `.git` markers and a fake `GitRunner`: covers depth halt at registered repo, depth limit excludes deeper repos, worktree skip, submodule skip (or no-descent equivalent — document the choice), bare repo skip, no-remote skip, origin selection, first-remote fallback, symlink loop dedup, same-repo-via-two-paths canonicalization, and `git`-missing → `ErrNotFound` propagation.
- [x] T006 [P] Implement `MergeScan` and the shared render primitive in `src/internal/yamled/yamled.go`: load file, build merged YAML node tree (existing groups preserved in source order; `default` slot per spec § "Group ordering"; invented groups appended in caller-given order), dedup URLs against all existing groups (silent skip — UI-free), atomic write via existing `atomicWrite` (preserve original mode; 0644 if absent). Extract a render-only sibling (e.g., `RenderScan(path string, plan ScanPlan) ([]byte, error)`) that returns the rendered bytes for print mode to share. Add tests to `src/internal/yamled/yamled_test.go` covering: default group does not exist (created), URL already registered elsewhere (silently skipped), invented group rendered as map shape `{ dir, urls }`, comment preservation across merge, atomic temp+rename behavior, rename-failure leaves original untouched, file-mode preservation, `RenderScan` returns identical bytes to what `MergeScan` writes.

### Layer 2b — CLI wiring (depends on T004, T005, T006)

- [x] T007 Implement `newConfigScanCmd().RunE` in `src/cmd/hop/config.go`: arg validation (`filepath.Clean` → `filepath.EvalSymlinks` → `os.Stat` directory check; failure → exit 2 with `hop config scan: '<dir>' is not a directory.` using user-supplied `<dir>` verbatim), `--depth N` validation (`N < 1` → exit 2 with `hop config scan: --depth must be >= 1.`), `hop.yaml` precondition via `config.Resolve()` (missing → exit 1 with the two-line scan-specific message using `config.ResolveWriteTarget()` for the path), call `scan.Walk` with `Options{Depth, GitRunner: proc.RunCapture-bound}`, lazy `git`-missing check (`hop: git is not installed.` exit 1 — only when walk required `git`), build the `ScanPlan` (convention check via `repos.DeriveOrg`/`DeriveName` + `ExpandDir`-resolved `cfg.CodeRoot`; slugify per spec § "Invented group naming"; per-parent-dir granularity; conflict resolution with `-2`/`-3` suffix and `note:` stderr; HOME → `~/...` substitution for invented `dir:`), and the print-vs-write dispatch (print: stdout via `yamled.RenderScan` with the two-line UTC header comment prepended; write: `yamled.MergeScan` then `wrote: <path>` trailer). Emit the stderr summary block per spec § "Stderr summary". Add unit tests to `src/cmd/hop/config_test.go` table-driven over: missing-`hop.yaml`, dir-not-a-directory, dir-is-file, dir-symlink-resolves, `--depth 0`, `--depth N` propagation, `git`-missing, slugify edge cases (plain, mixed-case+symbols, numeric-leading, pathological-empty → skip line), conflict resolution (reuse on dir match, suffix on dir mismatch), HOME substitution, output ordering (existing/default/invented), zero-repos summary text, and the print-mode header containing today's UTC date.
- [x] T008 Extend `newConfigInitCmd`'s post-write stderr tip in `src/cmd/hop/config.go` to the exact two-line text from spec assumption #25 / § "hop config init post-write tip update". Update the existing test in `src/cmd/hop/config_test.go` that asserts on the init tip stderr to match the new two-line wording exactly.

## Phase 3: Integration & edge cases

- [x] T009 Add an integration test in `src/cmd/hop/integration_test.go` exercising the full scan pipeline against a `t.TempDir()` repo tree: include a convention-match repo, a non-convention repo (invented group), a worktree (`.git` is a file), a submodule (nested under a registered repo), a bare-layout dir, a no-remote repo, and a symlink to a repo. Cover both print mode (assert YAML stdout shape + UTC header + stderr summary) and `--write` mode (assert `hop.yaml` mutated in place, comments preserved, atomicity, stderr ends with `wrote: <path>`). Use a fake `git` shim on `PATH` so invocations are deterministic across machines (or inject via the same seam used in T005 if the integration test plumbs `Options.GitRunner` through `RunE` — apply stage decides which seam is cleaner).
- [x] T010 [P] Update `docs/specs/cli-surface.md`: add the `hop config scan <dir>` row to the subcommand inventory table (per intake § "CLI surface table addition"); add a row to the External Tool Availability table for `hop config scan` referencing `hop: git is not installed.`; add Behavioral Scenarios (GIVEN/WHEN/THEN) for convention match, non-convention match, symlink resolution, depth cap, no-remote skip, and missing `hop.yaml`.
- [x] T011 [P] Finalize cobra `Short` and `Long` text on `newConfigScanCmd` in `src/cmd/hop/config.go`: `Short = "scan a directory for git repos and populate hop.yaml"`; `Long` = brief paragraph on auto-derive with one example each for print mode and `--write` (per intake § "Help text"). Verify `hop config --help` lists `scan` alongside `init` and `where` (covered by T009 or a small dedicated test).

## Phase 4: Polish

<!-- None warranted — Phase 3 covers docs, help text, and integration. -->

---

## Execution Order

**Phase 1** (T001–T003) all parallelizable — three independent files, type stubs only.

**Phase 2 layer 2a** (T004, T005, T006) all parallelizable — `internal/proc`, `internal/scan`, and `internal/yamled` have no cross-dependencies at this stage. T005's default `GitRunner` references `proc.RunCapture` from T004, but the `Options.GitRunner` injection seam means T005 can develop and test against fakes without waiting on T004; the wiring is a one-line constructor reference resolved at apply time.

**Phase 2 layer 2b**: T007 depends on T004 + T005 + T006 (CLI calls into all three). T008 is independent of T007 (touches a different `RunE` in the same file) — they SHALL be sequenced rather than parallel only because both edit `src/cmd/hop/config.go` and `src/cmd/hop/config_test.go`; merge-conflict avoidance, not logical dependency.

**Phase 3**: T009 depends on the full pipeline (T007 + T008). T010 and T011 are doc/help-text touches and parallelize with each other (T010 touches `docs/specs/`, T011 touches `src/cmd/hop/config.go`'s help-text strings only).

**Longest dependency chains**:

1. T001 → T005 → T007 → T009 — scan package skeleton → `Walk` implementation → CLI wiring → integration test.
2. T002 → T006 → T007 → T009 — yamled stubs → `MergeScan`/`RenderScan` → CLI wiring → integration test.
3. T003 → T007 → T009 — CLI factory stub → CLI `RunE` → integration test.
4. T001 → T004 → T005 (default `GitRunner`) → T007 → T009 — proc package extension feeds into scan's default runner, then CLI, then integration.

**Deferred-to-apply decisions**:

- T004: whether `RunCapture` is a new function or `Run` is extended with a `dir` parameter (spec § "Git invocation contract" explicitly leaves this open).
- T005: submodule detection — explicit ancestor-stack check vs. relying solely on the no-descent invariant (spec assumption #17 / scenario NOTE).
- T009: whether the integration test injects `Options.GitRunner` through a test seam or relies on a fake `git` binary on `PATH` — pick whichever is cleaner once T007 exists.
