# Config Scan

How `hop config scan <dir>` discovers on-disk git repos and populates `hop.yaml`. The CLI lives in `src/cmd/hop/config_scan.go` (factory in `src/cmd/hop/config.go::newConfigScanCmd`); the filesystem walk lives in `src/internal/scan/scan.go`; YAML emission goes through `src/internal/yamled/yamled.go::MergeScan` / `RenderScan`.

Spec: [`fab/changes/260506-ceh2-config-scan-populate-repos/spec.md`](../../../fab/changes/260506-ceh2-config-scan-populate-repos/spec.md). Intake (full design rationale): [`intake.md`](../../../fab/changes/260506-ceh2-config-scan-populate-repos/intake.md).

## Overview

`hop config scan <dir>` walks `<dir>`, classifies each candidate directory, and emits a YAML file containing every discovered repo placed into the appropriate group. Two modes share the same render path:

- **Print mode** (default) — render to stdout. The exact bytes that `--write` would produce.
- **Write mode** (`--write`) — atomic, comment-preserving in-place merge into the resolved `hop.yaml` via `yamled.MergeScan`.

Flags:

| Flag | Default | Notes |
|---|---|---|
| `--write` | `false` | Merge into resolved `hop.yaml` instead of printing. |
| `--depth N` | `3` | Maximum DFS depth (root is depth 0; depth bound is **inclusive** — `--depth 3` examines up through `R/*/*/*`). `N < 1` → exit 2 with `hop config scan: --depth must be >= 1.`. |

`code_root` is **never** modified by scan — it is durable and load-bearing. Users `hop config init` first to set `code_root`, then `hop config scan` to populate.

## Argument validation

The single positional `<dir>` is normalized in this order before any further processing:

1. `filepath.Clean(<dir>)`
2. `filepath.EvalSymlinks(<cleaned>)` — resolves symlinks. Failure (including ENOENT) → usage error.
3. `os.Stat(<resolved>)` — must indicate a directory; otherwise usage error.

The exact stderr on validation failure (with the user-supplied form, not the cleaned/resolved form):

```
hop config scan: '<dir>' is not a directory.
```

Exit 2. No `git` invocation occurs on a failed validation (Constitution I).

## `hop.yaml` precondition

Before walking, the subcommand calls `config.Resolve()` to locate `hop.yaml`. If `Resolve()` returns an error (no config found, or `$HOP_CONFIG` set but missing), scan emits a scan-specific stderr message in place of the resolver's default text:

```
hop config scan: no hop.yaml found at <bootstrap-path>.
Run 'hop config init' first, then re-run scan.
```

`<bootstrap-path>` is `config.ResolveWriteTarget()`'s output (the path that `hop config init` would write). Exit 1. No walk is performed (no `git` invocations).

## DFS algorithm and depth handling

`scan.Walk` (in `src/internal/scan/scan.go`) implements a stack-based DFS using `stackEntry{path, depth}`. The root is enqueued at depth 0. For each popped entry:

1. If `depth > opts.Depth` → skip (do not descend, do not register).
2. `os.Stat(path)` (resolves symlinks) — if it fails or the entry isn't a directory, skip silently.
3. **(dev, inode) dedup**: keyed by `syscall.Stat_t.{Dev,Ino}`. If the inode is already in the visited set → skip silently (loop suppression — not a user-facing skip).
4. `filepath.EvalSymlinks(path)` to canonicalize before classification (per spec § "Symlinks and loop detection").
5. `classifyDir(canonical)` → first-match-wins (see Repo classification below).
6. After classifying as a repo (or skip), do **not** descend into the directory's children — repos' children are never themselves repos.
7. Otherwise (plain dir): enqueue immediate subdirectories at `depth+1` in **reverse lexical order** so the DFS pop order yields lexical visit order (deterministic for tests and slug-tie tiebreaking).

## Repo classification rules

Implemented in `scan.go::classifyDir`. First-match-wins:

1. **Worktree** — `D/.git` exists and is a regular file. Skip with reason `"worktree"`. Do not descend.
2. **Normal repo** — `D/.git` is a directory. Invoke `git -C D remote`; if empty → skip with reason `"no remote"`. Otherwise pick `origin` if listed (else first non-empty line); invoke `git -C D remote get-url <selected>` for the URL; emit `Found{Path: canonical(D), URL: trim(out)}`. Do not descend.
3. **Bare repo (heuristic)** — `D` contains `HEAD` (regular file), `config` (regular file), and `objects/` (directory) at top level, AND `D/.git` does not exist. Skip with reason `"bare repo"`. Do not descend. Stat-only — does not shell out to `git rev-parse --is-bare-repository`.
4. **Plain directory** — none of the above; recurse into children at `depth+1`.

### Submodule handling

`ReasonSubmodule` is reserved in the public Skip enum but **never emitted by the current implementation**. The `internal/scan` walker relies solely on the no-descent invariant from rule 2: once a directory is classified as a normal repo, Walk never enqueues its children, so a nested `.git` inside a parent repo is unreachable through DFS. This was an explicit choice (spec assumption #17 permits "the implementation MAY rely solely on the no-descent invariant if it materially simplifies code"). The constant remains exported for forward compatibility.

If a user passes a submodule path directly as the scan root, it is classified as a normal repo (rule 2) and registered as Found — there is no ancestor on the stack to defensively check against.

## Symlinks and loop detection

- Symlinks are followed during the walk (intentional — users symlink directories for Time Machine, network mounts, ad-hoc aliases).
- Loops dedup'd via `(device, inode)` of the canonical directory (`syscall.Stat_t`). On hit → silent skip (no `Skip` entry). Standard `find -L` approach.
- Each `Found.Path` is the `filepath.EvalSymlinks` resolution. The same repo reached via two paths is registered exactly once.

## Git invocation contract

All `git` invocations route through `internal/proc.RunCapture(ctx, dir, "git", args...)` (Constitution Principle I — no direct `os/exec` outside `internal/proc`). The `GitRunner` field on `scan.Options` is the injectable seam; production binds the default `proc.RunCapture`-bound runner, tests inject a fake.

Each invocation gets a 5-second timeout via `context.WithTimeout(ctx, 5*time.Second)` (constant `gitTimeout` in `scan.go`).

`git` is **lazy-checked**: it is only required when the walk actually finds a `.git` candidate that requires `git remote`. Empty scan trees (zero `.git` discoveries) succeed with exit 0 and no `git` invocation. When `git` is missing on PATH AND a `.git` candidate is encountered, the CLI emits `hop: git is not installed.` (the same `gitMissingHint` constant used by `hop clone`) and exits 1. The scan halts on the first `git`-not-found rather than continuing to other candidates.

## Group assignment

The CLI layer (`config_scan.go::buildScanPlan`) assigns each `Found` to a group; this logic is **not** in `internal/scan`, which stays UI-free.

### Convention check

For each `Found{Path, URL}`:

1. `org := repos.DeriveOrg(URL)`, `name := repos.DeriveName(URL)`.
2. `convention := filepath.Join(repos.ExpandDir(cfg.CodeRoot, ""), org, name)` (org dropped when empty).
3. Both sides run through `filepath.EvalSymlinks` before string comparison. This handles platforms where `$HOME` (or its ancestors) is itself symlinked — e.g., macOS, where `t.TempDir()` threads through `/var/folders → /private/var/folders`. EvalSymlinks failure (e.g., the convention dir doesn't exist on disk yet) falls back to `filepath.Clean`.
4. Match → assign to the `default` flat group (URL only, no per-repo `dir:`).
5. No match → invented group (next section).

### Invented group naming (slugify)

Slugify rule (`config_scan.go::slugifyGroupName`):

1. `base := filepath.Base(filepath.Dir(Path))` — the parent dir basename.
2. Lowercase.
3. Replace any run of characters not matching `[a-z0-9_-]` with a single `-`.
4. Trim leading and trailing `-` AND `_`. The extended trim charset (`-_`) is required so all-underscore input (`___`) trims to empty per the spec's pathological-input examples; internal `_` runs are preserved.
5. If empty → skip the repo with stderr:
   ```
   skip: <Path>: cannot derive group name from parent dir '<base>'
   ```
   Counts as a generic skip; does NOT block other repos.
6. If the leading char is not `[a-z]` → prefix `g` (e.g., `9-experiments` → `g9-experiments`).
7. Final defensive check against the schema regex `^[a-z][a-z0-9_-]*$`; non-conforming → treat as empty (bail out).

### Per-parent-dir granularity

One group per *distinct* parent dir (after canonicalization). Two different parent dirs are **never** merged even if their slugify outputs collide — see Conflict resolution.

`config_scan.go` tracks invented groups by `parentDir → index in plan.InventedGroups` so two repos under the same parent share a group.

### Group dir rendering

The `dir:` field of an invented group is the canonical parent path with `$HOME` substituted to `~/...` when the path begins under `$HOME`; otherwise the absolute path verbatim (`config_scan.go::homeSubstitute`). Matches the style used in starter content and existing user configs.

### Conflict resolution

When the merge plan is built (`config_scan.go::resolveInventedName`):

1. **Slug matches existing group, dirs match (canonicalized)** → reuse that existing group; new URLs append to it. No stderr note.
2. **Slug matches existing group, dirs differ** → suffix with the smallest integer `-N` (starting at `-2`) that does not collide with any existing or already-invented group name. Stderr note:
   ```
   note: invented group '<original-slug>' already exists in hop.yaml with a different dir; using '<original-slug>-2' for <new-dir>.
   ```
3. **Two distinct parent dirs slugify to the same name during a single scan** → first one wins; second is suffixed `-2` (and so on). Same stderr note.

The smallest available suffix is found by linear scan starting at 2 (`nextAvailableSuffix`).

## Output rendering

Both modes share the in-memory render produced by `internal/yamled` — the only difference is the sink. Print mode emits to `cmd.OutOrStdout()`; write mode performs `yamled.MergeScan` (atomic temp+rename, file mode preserved).

### Print mode header

Print mode prepends a two-line header comment before the rendered YAML:

```
# hop config — generated by 'hop config scan <user-arg>' on <YYYY-MM-DD> (UTC).
# Run with --write to merge into <resolved-hop.yaml-path>.
```

`<user-arg>` is the user-supplied directory verbatim (not canonicalized); `<YYYY-MM-DD>` is `time.Now().UTC().Format("2006-01-02")` (UTC for reproducibility across collaborators); the literal `(UTC)` suffix removes timezone ambiguity. Header is part of the *stdout* render only — write mode does not modify the file's existing head comments.

### Group ordering

In the rendered YAML (both modes):

1. **Existing groups** from the loaded `hop.yaml`, in their original source order.
2. **`default`** (if not already present in #1, AND scan is contributing entries to it; if `default` already exists in #1 it stays in its source-ordered slot).
3. **Invented groups** (those not present in #1), sorted alphabetically by group name (post-slugify, caller-side via `sort.SliceStable`).

Existing groups retain their existing URLs; scan-contributed URLs are appended within each group at the end of the URL list (or `urls:` sequence for map-shaped groups).

### Stdout / stderr split

- **stdout** in print mode: rendered YAML (header comment + body). In write mode: empty.
- **stderr** in both modes: the human-readable summary block. In write mode it ends with `wrote: <resolved-hop.yaml-path>`; in print mode it ends with `Run with --write to merge into <resolved-hop.yaml-path>.`.
- Per-repo skip lines (slugify failure, dedup) and conflict-resolution `note:` lines also go to stderr.

This matches `hop clone`'s precedent: status to stderr, useful piping payload to stdout. Print mode is pipeable: `hop config scan ~/code > hop.yaml` captures only the rendered YAML.

### Stderr summary block

```
hop config scan: scanned <user-arg> (depth N), found <K> repos.
  matched convention (default): <C> [(<C-new> new, <C-existing> already registered)]   # write-only sub-counts
  invented groups: <I> (<comma-separated names>)
  skipped: <S1> worktree, <S2> bare repo, <S3> no remote[, <S4> no group name]
[write only:  wrote: <resolved-hop.yaml-path>]
[print only:  Run with --write to merge into <resolved-hop.yaml-path>.]
```

Zero-count buckets are elided. Pluralization is per-reason (`1 worktree` vs `2 worktrees`; `1 bare repo` vs `2 bare repos`). Zero-repos case is short-circuited to:

```
hop config scan: scanned <user-arg> (depth N), found 0 repos. Nothing to add.
```

## `--write` merge semantics

Implemented in `internal/yamled.MergeScan` (with `RenderScan` as the shared rendering primitive used by print mode):

- **Dedup**: any URL in `plan.DefaultURLs` or `plan.InventedGroups[i].URLs` already present in any existing group is **silently dropped** (matches the parser's URL-uniqueness rule and `AppendURL`'s contract). The CLI is responsible for any user-visible skip lines.
- **Default group**: if absent in the loaded file, created as a new flat-list group appended after existing groups.
- **Invented groups**: appended after existing groups in the order given by `plan.InventedGroups` (caller pre-sorts alphabetically). Map-shaped: `{ dir: <Dir>, urls: [<URLs>] }`.
- **`code_root`** and existing groups' `dir:`s are never modified.
- **Atomicity**: temp file + rename in the same directory; file mode preserved (defaults to 0644 for new files, matching `WriteStarter`). On rename failure the original is left untouched.
- **Comments**: preserved by yaml.v3 round-trip; indentation is normalized to yaml.v3 defaults — comment preservation is the contract, byte-perfect formatting is not.

Public surface in `internal/yamled/yamled.go`:

```go
func MergeScan(path string, plan ScanPlan) error
func RenderScan(path string, plan ScanPlan) ([]byte, error)

type ScanPlan struct {
    DefaultURLs    []string
    InventedGroups []InventedGroup
}

type InventedGroup struct {
    Name string  // already slugified by the caller; conforms to ^[a-z][a-z0-9_-]*$
    Dir  string  // already canonical (with ~-substitution where applicable)
    URLs []string
}
```

`MergeScan` writes; `RenderScan` returns the bytes that `MergeScan` would write. Both share `mergeScanIntoTree` internally.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | Success (any number of repos found, including zero) |
| 1 | `hop.yaml` missing; YAML write/merge failure; load error on existing `hop.yaml`; `git` not on PATH (lazy) |
| 2 | Usage error (no `<dir>` arg, dir validation failure, `--depth < 1`) |

## Tool requirements

- `git` — required only when the walk actually finds a `.git` candidate (lazy). Missing → `hop: git is not installed.` exit 1.

No other external tools are required by scan.

## Cross-references

- Bootstrap-then-populate workflow and `hop config init`'s post-write tip wording: [init-bootstrap](init-bootstrap.md)
- YAML schema and group regex `^[a-z][a-z0-9_-]*$` that slugify must conform to: [yaml-schema](yaml-schema.md)
- `internal/scan` package role and `Walk`/`Found`/`Skip`/`Options` public surface: [architecture/package-layout](../architecture/package-layout.md)
- Constitution I compliance: `internal/scan` invokes `git` only via `internal/proc.RunCapture`: [architecture/wrapper-boundaries](../architecture/wrapper-boundaries.md)
