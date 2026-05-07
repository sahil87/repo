# Intake: hop config scan — populate hop.yaml from on-disk repos

**Change**: 260506-ceh2-config-scan-populate-repos
**Created**: 2026-05-06
**Status**: Draft

## Origin

User raised onboarding friction: `hop`'s value is invisible until `hop.yaml` is hand-curated. The starter file written by `hop config init` points only at the `hop` repo itself — useful as a "does this work" check, not as "this is *my* world."

The interaction was conversational (`/fab-discuss` → multi-round Q&A). Key design decisions reached, in order:

1. **Separate command, not an init flag.** `hop config scan <dir>` — a new subcommand under `config`. Init bootstraps the file; scan populates it. Each does one thing. (Constitution VI: justified by being a config-namespace operation, not a new top-level — slots alongside `init` and `where`.)
2. **Print full file by default; `--write` merges.** Both modes route through `internal/yamled`'s comment-preserving render. `--write` performs an atomic file replace (existing yamled `atomicWrite` contract). Print mode emits the same rendered YAML to stdout (header commentary on stderr) — i.e., "what `--write` would have produced, sent to stdout instead." Symmetric: same input, same shape, different sink.
3. **Group assignment is auto-derived.** For each found repo:
   - Compute `(org, name)` from the URL via existing `DeriveOrg`/`DeriveName`.
   - Resolve the on-disk path to its canonical form (follow symlinks).
   - If canonical path equals `<code_root>/<org>/<name>` → emit URL into `default` (flat list — convention applies).
   - Otherwise → invent a group named after the parent dir basename, slugified to `^[a-z][a-z0-9_-]*$`, with explicit `dir:` set to the canonical parent dir. One group per distinct parent dir (granularity: `~/code/experiments` and `~/code/experiments-old` produce two groups, `experiments` and `experiments-old`).
4. **Remote selection.** `origin` if present, else first remote; no remote → skip with stderr note.
5. **Skip rules.** Bare repos, worktrees (the `.git` is a file, not a directory), submodules (any `.git` nested inside a parent repo's tree), and repos with no remote.
6. **Recursion depth.** Default 3, `--depth N` to override. Stop descending into a directory once `.git` is found there (a repo's children aren't repos).
7. **Symlinks.** Follow them, with inode-based loop detection (track `(device, inode)` pairs, skip already-visited dirs). Resolve to canonical path *before* the convention check so auto-derive is deterministic regardless of how the repo was reached.
8. **`hop.yaml` must exist.** Scan errors with a hint to run `hop config init` first if missing. Reasoning: `code_root` is durable and load-bearing; auto-deriving it from the scan target risks baking in a wrong root if the user runs `scan ~/code/sahil87` (one level too deep). Splitting bootstrap (`init`) from population (`scan`) keeps each command's responsibility clean.
9. **Output ordering.** Existing groups preserved in source order, then `default`, then invented groups alphabetically.
10. **Bundled doc-touch:** update `hop config init`'s post-write stderr tip to mention `hop config scan` as the recommended next step. This is how onboarding users discover the command.

Alternatives considered and rejected:

- **Overload `init` with a folder arg** — overlaps responsibilities. Rejected for the separation-of-concerns argument.
- **`hop doctor` / interactive `init --interactive`** — adjacent ideas worth doing later, not bundled here. Out of scope.
- **Skip non-convention repos with a warning** — rejected; users with intentional layouts (e.g., `~/vendor/*`) would get nothing useful.
- **Single explicit fallback group via `--group <name>`** — strictly less expressive than per-parent-dir invention; user can still consolidate after the fact by editing YAML.
- **Print fragment instead of full file** — initially preferred for honesty about deltas, but caused asymmetry concerns; full file is cleaner for the cold-start use case (`scan > hop.yaml`).
- **Auto-create `hop.yaml` if missing** — collapses cold-start to one command but risks wrong `code_root`. Two-command path (`init && scan --write`) is a one-liner anyway.

## Why

**Problem.** `hop`'s entire value proposition — fast match-or-fzf onto a list of known repos — depends on `hop.yaml` listing those repos. A new user with 30 repos already cloned at `~/code/<org>/<name>` has to either type each URL by hand or write a one-off shell loop (`find ~/code -name .git -maxdepth 3 ...`). The first-time experience is the highest-friction point in adopting the tool.

**Consequence if unfixed.** Users either bounce off during onboarding or land on a starter file that points at one repo (`hop` itself), conclude the tool is uninteresting, and don't return. The tool's `hop ls` / `hop` / `hop <name>` flows become valuable only after the per-user investment of curating YAML, which most users won't make.

**Why this approach.** Filesystem state is already authoritative for repo paths (Constitution II: no database). A scan command formalizes the natural workflow — "look at what's on disk, register what's there" — into a single deterministic operation. Auto-derive is the right default because hop's path conventions (`<code_root>/<org>/<name>`) are designed to *be* discoverable: if the user already organized their disk that way, scan should produce a flat `default` group with no manual group-assignment effort. For users with intentional non-convention layouts (vendor, experiments, work directories), inventing groups from parent dirs preserves their organization without forcing them to design groups upfront.

**Why not just keep recommending manual editing?** The starter file's tip already says "Edit the file to add your repos." That's the status quo. It's not working — onboarding friction is the user-reported pain point.

**Why this is also useful post-onboarding.** After cloning a few new repos into `~/code/<org>/`, re-running `scan ~/code --write` picks them up without manual edits. Same command, same mental model. The scan/onboarding distinction is about timing, not function.

## What Changes

### New subcommand: `hop config scan <dir>`

A new cobra subcommand under `config`, defined in `src/cmd/hop/config.go` (alongside `newConfigInitCmd` and `newConfigWhereCmd`).

**Argument**: exactly one positional, the directory to scan. Path validation (Constitution I): the dir argument is `filepath.Clean`'d, then `filepath.EvalSymlinks` resolved; if the resolved path doesn't exist or isn't a directory, error with `hop config scan: '<dir>' is not a directory.` exit 2.

**Flags**:

- `--write` — merge into resolved `hop.yaml` instead of printing. Default false.
- `--depth N` — max recursion depth. Default 3. `N < 1` → usage error, exit 2.

**Behavior** (high-level pipeline):

1. Resolve `hop.yaml` via `config.Resolve()`. If missing → exit 1 with:
   ```
   hop config scan: no hop.yaml found at <resolved-path>.
   Run 'hop config init' first, then re-run scan.
   ```
   (`Resolve()`'s existing error message is suppressed in favor of this scan-specific one.)
2. Load the existing config to get `CodeRoot` (resolved/expanded for the convention check).
3. Walk the input directory, gathering `(canonical_path, remote_url)` pairs (see "Filesystem walk" below).
4. Auto-derive group placement for each pair (see "Group assignment" below).
5. If `--write`: merge into `hop.yaml` via `internal/yamled` (see "Merge semantics" below).
6. Otherwise: emit a full valid YAML file to stdout, with a stderr header summary.

**Exit codes**:

- `0` — success (any number of repos found, including zero).
- `1` — `hop.yaml` missing; URL collision detected during merge; YAML write failure.
- `2` — usage error (no dir arg, dir not a directory, `--depth < 1`).

### Filesystem walk

Implemented in a new internal package `src/internal/scan/` (new file: `scan.go`, with `scan_test.go`). Public surface:

```go
type Found struct {
    Path string  // canonical path to the repo's working tree (parent of .git)
    URL  string  // remote URL (origin if present, else first remote)
}

type Options struct {
    Depth      int
    GitRunner  func(ctx context.Context, dir string, args ...string) ([]byte, error)  // injectable for tests; defaults to internal/proc
}

func Walk(ctx context.Context, root string, opts Options) ([]Found, []Skip, error)

type Skip struct {
    Path   string
    Reason string  // "no remote", "bare repo", "worktree", "submodule"
}
```

Walk algorithm:

1. Stack-based DFS starting at `root`. Each entry tracks `(path, depth)`. Skip if `depth > opts.Depth`.
2. For each directory entry, `os.Lstat` to check if it's a symlink. If symlink: `os.Stat` (resolves) to get target info; check `(device, inode)` against a visited set; skip on hit.
3. If the entry is a directory and contains `.git`:
   - `os.Stat .git`: if it's a regular file (not directory) → **worktree** → skip with reason "worktree".
   - If `.git` is a directory: check the parent path for whether it's a submodule. Submodule heuristic: walk upward (via canonical paths) and see if any ancestor's `.git` directory contains a file like `modules/<this-repo-name>` matching this repo's git dir, OR — simpler and sufficient — if any *ancestor* on the visited stack is itself a repo (`.git` directory present), this is a nested repo → skip with reason "submodule".
   - If `.git/objects` exists but no `.git/HEAD` work-tree pointer → **bare repo** (rare for working repos, but possible if user `git init --bare`'d) → skip. Concretely: a bare repo has `HEAD`, `config`, `objects/` at top level (no `.git` subdir). If we see a directory containing those at the top level *without* a `.git` subdir, treat it as bare and skip. Bare repos are uncommon under typical scan roots; the heuristic is sufficient.
   - Otherwise: invoke `git -C <path> remote` to list remotes. If no remotes → skip with reason "no remote". Else select `origin` if listed, else first line. Then `git -C <path> remote get-url <selected>` for the URL.
   - After registering the repo (or skipping it), do **not** descend into it — repos' children are never repos for our purposes.
4. Otherwise (no `.git`): recurse into subdirectories.

Git invocations go through `internal/proc.RunCapture(ctx, dir, "git", args...)` (Constitution I: `exec.CommandContext` with explicit args) — extending `internal/proc` if it doesn't already expose a capture variant; the existing `RunForeground` is for inheriting stdio. Each invocation gets a 5-second timeout via `context.WithTimeout`.

The walker does **not** care which directory the user passed — `Walk` works on any rooted path. The CLI layer handles "is the input a real directory" validation.

**Symlink and loop handling**:

- `os.Stat` (not `Lstat`) for the recursion check, so symlinks resolve.
- Dedupe by `(device, inode)` on the *resolved* directory. The first time we see a `(dev, ino)` pair, we descend; subsequent encounters skip.
- Repo paths returned in `Found.Path` are always canonical (`filepath.EvalSymlinks` applied).

### Group assignment (auto-derive)

After `Walk` returns `[]Found`, the CLI layer assigns each Found to a group:

```
for each Found f:
    org, name := DeriveOrg(f.URL), DeriveName(f.URL)
    convention_path := filepath.Join(expandedCodeRoot, org, name)
    if filepath.Clean(f.Path) == filepath.Clean(convention_path):
        place into "default" (flat list, just the URL)
    else:
        parent := filepath.Dir(f.Path)
        group_name := slugify(filepath.Base(parent))
        place into group group_name (map-shaped: dir = parent, urls += f.URL)
```

**Slugify rule**: lowercase, replace any run of non-`[a-z0-9_-]` chars with a single `-`, trim leading/trailing `-`, ensure leading char matches `[a-z]` (prefix `g` if it doesn't, e.g. `9-stuff` → `g9-stuff`). If the result is empty (parent dir basename was something pathological like `///` or all-symbols), skip that repo with stderr `skip: <path>: cannot derive group name from parent dir '<basename>'`.

**Group dir**: the canonical parent path, written verbatim with `~`-prefix substitution where applicable (i.e. if the parent starts with the user's `$HOME`, substitute `~/...` for portability; otherwise absolute path). This matches the style of starter content and existing user configs.

**Conflict resolution**:

- If an invented group name collides with an existing group in `hop.yaml` (loaded config), and the existing group's `dir` differs from our parent → suffix the invented name with the path's tail to disambiguate (e.g., `experiments` → `experiments-2`). Practically rare; emit a stderr note when it happens.
- If the invented group name collides with an existing group whose `dir` *matches* → reuse that group; URLs append to it.
- Two distinct parent dirs that slugify to the same name (e.g., `~/Code/Vendor` and `~/code/vendor` on a case-sensitive FS) → second one gets the `-2` disambiguation suffix.

### Output ordering

When emitting YAML (print or write), groups appear in this order:

1. Existing groups from the loaded `hop.yaml`, in their original source order. Print mode shows them with their original URLs preserved; write mode does not modify them except to append new URLs (see merge semantics).
2. `default` (if not already in #1, or with new URLs to add).
3. Invented groups, sorted alphabetically by group name.

This matches existing semantics — `cfg.Groups` is already source-ordered (`yaml-schema.md`).

### Print mode output

**Stdout** (the YAML file):

```yaml
# hop config — generated by 'hop config scan ~/code' on 2026-05-06.
# Run with --write to merge into <resolved-hop.yaml-path>.

config:
  code_root: ~/code

repos:
  default:
    - git@github.com:sahil87/hop.git
    - git@github.com:sahil87/wt.git
    # ... (existing entries, then new entries from scan, in source-then-discovery order)

  experiments:
    dir: ~/code/experiments
    urls:
      - git@github.com:sahil87/sandbox.git

  vendor:
    dir: ~/vendor
    urls:
      - git@github.com:some-vendor/their-tool.git
```

The `config:` block reflects the *existing* `hop.yaml`'s `code_root` (never overwritten). Comments from the existing file ARE preserved — print mode renders through `internal/yamled`'s comment-preserving path, then emits to stdout instead of writing to disk. <!-- clarified: print mode is exactly "what --write would produce, sent to stdout instead." This eliminates the print/write asymmetry: same comment-preserving render in both modes; the only difference is the destination (stdout vs. atomic file replace). -->


**Stderr** (header / commentary):

```
hop config scan: scanned ~/code (depth 3), found 7 repos.
  matched convention (default): 5
  invented groups: 2 (experiments, vendor)
  skipped: 1 worktree, 1 submodule
Run with --write to merge into ~/.config/hop/hop.yaml.
```

If zero repos are found: stdout still emits the existing config unchanged; stderr says `hop config scan: scanned <dir> (depth N), found 0 repos. Nothing to add.`

### `--write` merge semantics

Goes through `internal/yamled`. One new entry point, sized for this use case:

```go
// MergeScan applies a structured plan of scan additions to the YAML file
// at path in a single atomic write. Comments are preserved.
func MergeScan(path string, plan ScanPlan) error

type ScanPlan struct {
    DefaultURLs    []string         // appended to the "default" flat group; dedup vs. all groups
    InventedGroups []InventedGroup  // appended after existing groups, alphabetical by Name
}

type InventedGroup struct {
    Name string  // already slugified by the caller
    Dir  string  // already canonical (with ~-substitution where applicable)
    URLs []string
}
```

Single atomic op, mirrors the existing `AppendURL`/`atomicWrite` contract, simpler call site than composed primitives. Print mode uses the same code path: render the merged tree in memory, marshal back, emit to stdout instead of writing. <!-- clarified: chose MergeScan over the composed EnsureGroup + AppendURLs pair — atomicity matters (yamled's existing contract), and the ScanPlan type is cheap and self-documenting. -->

Dedup behavior inside `MergeScan`: any URL already present in *any* existing group is silently skipped (matches `AppendURL`'s contract and the parser's URL-uniqueness rule). The CLI layer is responsible for pre-emitting `skip:` stderr lines for those URLs *before* invoking `MergeScan` — yamled stays UI-free.

**Write-mode behavior**:

- Dedupe URLs against the entire loaded config (not just the target group) — if a URL exists in any existing group, skip it (with stderr `skip: <url> already in group '<g>'`). Same rule as the existing parser's URL uniqueness validation.
- New URLs whose target is `default` are appended to `default`. If `default` doesn't exist in the file, create it as a flat group.
- New invented groups are appended after existing groups, alphabetically.
- `code_root` is never modified.
- Comments in the existing file are preserved (yamled's job).
- Atomic write via temp file + rename (yamled's job).

**Stderr** (write mode):

```
hop config scan: scanned ~/code (depth 3), found 7 repos.
  matched convention (default): 5 (3 new, 2 already registered)
  invented groups: 2 (experiments, vendor; both new)
  skipped: 1 worktree, 1 submodule
wrote: ~/.config/hop/hop.yaml
```

Stdout is empty in write mode (no path printed; user already knows where the file lives via `hop config where`).

### Update to `hop config init`'s post-write tip

The stderr tip in `newConfigInitCmd`'s `RunE` (currently in `src/cmd/hop/config.go:34`) gets one new line:

```
Edit the file to add your repos, or run `hop config scan <dir>` to populate from existing on-disk repos.
Tip: set $HOP_CONFIG in your shell rc to point at a version-tracked location (a git-tracked dotfile, Dropbox, etc.) so this config moves with you across machines.
```

(Existing tip preserved; new "or run" half added.) Touches `init-bootstrap.md` memory.

### CLI surface table addition

`docs/specs/cli-surface.md`'s subcommand inventory gets one new row:

```
| `hop config scan <dir>` | exactly 1 dir + flags | Walk <dir> for git repos, auto-derive groups, print full YAML to stdout (or `--write` to merge into hop.yaml). Flags: `--write`, `--depth N`. | 0 success, 1 hop.yaml missing or merge failure, 2 usage error |
```

Plus a behavioral scenario section under "Behavioral Scenarios (GIVEN/WHEN/THEN)" with cases for: convention match, non-convention match, symlink resolution, depth cap, no-remote skip, missing `hop.yaml`.

### Help text

Cobra `Short`: "scan a directory for git repos and populate hop.yaml"
Cobra `Long`: brief paragraph on auto-derive, with one example for each mode (print and `--write`).

## Affected Memory

- `cli/subcommands.md`: (modify) add `hop config scan` row to inventory; cross-reference from `config init`'s tip update; document tool requirement (`git`).
- `cli/match-resolution.md`: (no change) — scan doesn't use match resolution.
- `config/init-bootstrap.md`: (modify) update the post-write tip text and add a "Cross-references" line pointing to a new `config/scan.md`.
- `config/scan.md`: (new) full scan-command memory: search rules, group-assignment algorithm, slugify rule, conflict resolution, depth/symlink handling, write merge semantics, exit codes.
- `config/yaml-schema.md`: (no change) schema is unchanged; scan only writes shapes already supported.
- `architecture/package-layout.md`: (modify) add `internal/scan/` to the package layout listing; describe its role.
- `architecture/wrapper-boundaries.md`: (modify) note that `internal/scan` invokes `git` via `internal/proc` (not direct `os/exec`), preserving the wrapper boundary.

## Impact

**Code areas touched**:

- `src/cmd/hop/config.go` — add `newConfigScanCmd()`; wire into `newConfigCmd`. Update `newConfigInitCmd` post-write tip.
- `src/cmd/hop/config_test.go` — table-driven tests for the new subcommand: arg validation, missing-`hop.yaml` error, print output shape, write merge success, depth flag.
- `src/internal/scan/` — new package: `scan.go` (Walk + Found/Skip types), `scan_test.go` (table-driven over a synthesized `t.TempDir()` tree with fake `.git` markers and an injected `GitRunner`).
- `src/internal/yamled/yamled.go` — extend with `MergeScan` (or `AppendURLs` + `EnsureGroup` pair). Tests in `yamled_test.go`.
- `src/internal/proc/proc.go` — possibly add a `RunCapture` (or use existing capture helper if present) that returns stdout bytes for `git remote` queries. If `internal/proc` already has this, reuse.
- Spec/memory docs as listed in "Affected Memory".

**Dependencies**: no new external Go modules. Uses existing `gopkg.in/yaml.v3` via `internal/yamled`.

**External tool requirements**: `git` must be on PATH when scan finds repos (lazy: per scan-time external tool policy in `cli-surface.md`'s "External Tool Availability"). Add a row for `hop config scan` in that table. Missing `git` → `hop: git is not installed.` exit 1.

**Cross-platform**: works on darwin and linux uniformly. No platform-specific code.

**Security review hooks**:

- The `<dir>` argument is the only user-controlled string passed to a subprocess context. `filepath.EvalSymlinks` and `os.Stat` validate before any `git` invocation. `git` is always invoked with `-C <canonical-path>` and an explicit argument slice (no shell interpolation).
- Repo paths discovered during walk are file paths from the OS, not user-supplied; passed to `git -C` as-is.
- Remote URL strings from `git remote get-url` are passed back to the user as-is in YAML output. They aren't reinterpreted by hop (Constitution: permissive on URL contents). No shell expansion of URLs.

**Performance**: walking ~/code with 30 repos at depth 3 should complete well under 1s (each `git remote` call is ~10ms locally). The 5s timeout per `git` call is generous insurance against pathological repos. No parallelism in v1 — sequential walk is simple and fast enough; revisit if users complain about >1000-repo trees.

**Backward compatibility**: purely additive. No existing command's behavior changes except `hop config init`'s stderr tip (one extra line; not a breaking change). v0.x policy applies (no compat shims needed regardless).

## Open Questions

None — all design questions resolved during clarify session 2026-05-06. See `## Clarifications` below.

## Clarifications

### Session 2026-05-06 (auto)

| # | Question | Resolution |
|---|----------|-----------|
| 21 | yamled API shape — single `MergeScan` or composed `EnsureGroup` + `AppendURLs`? | `MergeScan(path, plan ScanPlan) error` — single atomic op; mirrors existing `AppendURL` atomicity contract; cleaner call site. |
| 16 | Bare-repo detection — stat-based heuristic or `git rev-parse --is-bare-repository` shell-out? | Stat-based (`HEAD` + `config` + `objects/` at top level, no `.git` subdir). Avoids extra git invocation; bare repos rare under typical scan roots. |
| 18 | Print mode — preserve comments from existing `hop.yaml`? | Yes. Route through yamled's comment-preserving render; print is "what `--write` would produce, sent to stdout instead." Eliminates print/write asymmetry. |
| 22 | Tab completion for `<dir>` — verify cobra fallback? | Non-issue — directory completion is shell-handled at path-shaped arg positions; cobra default just needs to not break it. |

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Subcommand placement: `hop config scan` (under existing `config` namespace, not a new top-level) | Constitution VI requires explicit justification for new top-level subcommands; this is a config-state operation that fits cleanly under `config` alongside `init` and `where`. Discussed and confirmed. | S:90 R:75 A:90 D:90 |
| 2 | Certain | Print full file by default, `--write` merges into resolved `hop.yaml` | Discussed extensively; user agreed to symmetry trade-off (print/write differ in shape; print is for cold-start, write is for incremental backfill). | S:95 R:70 A:80 D:90 |
| 3 | Certain | Auto-derive groups: convention match → `default`; non-match → invent group from parent-dir basename (slugified) | Explicitly chosen by user (Q1 = (a)). The convention path is `<code_root>/<org>/<name>` per existing schema. | S:95 R:60 A:85 D:90 |
| 4 | Certain | Remote selection: `origin` if present, else first remote; no remote → skip | User answered Q3 directly. Matches `git`'s own conventions (origin is the conventional default). | S:95 R:85 A:90 D:95 |
| 5 | Certain | Skip rules: bare repos, worktrees, submodules, no-remote | User answered Q3 directly. Each has a distinct stat-level signature, easy to detect. | S:90 R:80 A:85 D:90 |
| 6 | Certain | Default depth 3, `--depth N` flag override | User answered Q4 directly. Depth 3 covers `<root>/<org>/<name>` with one slack level. | S:95 R:90 A:90 D:95 |
| 7 | Certain | Symlinks followed, with inode-based loop detection and canonicalize-before-convention-check | User answered Q4 directly. Inode dedupe is the standard `find -L` approach. Canonical resolution makes auto-derive deterministic. | S:90 R:80 A:80 D:85 |
| 8 | Certain | `hop.yaml` missing → error with hint to run `hop config init` first (do not auto-create) | User answered Q2 directly (agreed with my recommendation). `code_root` is durable; auto-deriving from scan target risks wrong root forever after. | S:95 R:80 A:85 D:90 |
| 9 | Certain | Group-naming: slugify parent dir basename via `^[a-z][a-z0-9_-]*$`; pathological names skip with stderr | User answered Q5 = "slugify". Schema requires this regex (yaml-schema.md). Empty-after-slugify is the only edge that needs a skip. | S:90 R:70 A:90 D:85 |
| 10 | Certain | Group-dir granularity: one group per distinct parent dir (no collapsing) | User answered Q6 = "your choice"; collapsing logic would be magic and surprising. Per-parent-dir is mechanical and predictable. | S:80 R:65 A:80 D:85 |
| 11 | Certain | Output ordering: existing groups first (source order), then `default`, then invented groups alphabetically | User answered Q7 = "default". Diff-friendly; matches cfg.Groups source-order semantics. | S:90 R:90 A:90 D:90 |
| 12 | Certain | Both `--write` and print mode in v1 (not deferred) | User answered Q8 = "in v1". | S:100 R:95 A:95 D:100 |
| 13 | Certain | Bundle: update `hop config init`'s post-write tip to mention `hop config scan` | Onboarding discoverability — without this, scan is invisible to new users. Trivial doc-touch. | S:90 R:95 A:95 D:95 |
| 14 | Confident | New `internal/scan` package (rather than inlining walk logic in `cmd/hop/config.go`) | Scan logic is non-trivial (DFS, inode dedupe, classifier, git invocation), benefits from isolated unit tests with an injected `GitRunner`. Matches existing pattern (`internal/yamled`, `internal/update`, `internal/proc`). | S:80 R:70 A:90 D:80 |
| 15 | Confident | Git invocations go through `internal/proc.RunCapture` (or equivalent) with 5s context timeout | Constitution I: `exec.CommandContext` with explicit args. `internal/proc` is the wrapper layer. 5s is generous; local `git remote` calls are sub-100ms. | S:90 R:80 A:90 D:85 |
| 16 | Certain | Bare-repo detection via stat (`HEAD` + `config` + `objects/` at top level, no `.git` subdir) rather than `git rev-parse --is-bare-repository` | Clarified — locked in. Avoids extra git invocation per candidate; heuristic catches the common case; bare repos are rare under typical scan roots. | S:95 R:75 A:80 D:75 |
| 17 | Confident | Submodule detection via "ancestor on visited stack is itself a repo" — simpler than parsing `.gitmodules` | Mechanically equivalent for the scan use case. Avoids `.gitmodules` parsing complexity. | S:75 R:70 A:80 D:80 |
| 18 | Certain | Print mode DOES preserve comments — routes through yamled's comment-preserving render, then emits to stdout instead of disk | Clarified — flipped from "not preserved" to "preserved" for clean print/write symmetry. Print is exactly "what `--write` would produce, sent to stdout instead." | S:95 R:65 A:75 D:90 |
| 19 | Confident | New file `docs/memory/config/scan.md` (rather than appending to `init-bootstrap.md`) | Scan has enough surface area to warrant its own memory file. Init-bootstrap stays focused on init's behavior. | S:80 R:90 A:85 D:80 |
| 20 | Confident | Group-name collision handling: `-2`, `-3` suffix on dir mismatch; reuse on dir match | Predictable, mechanical; matches the URL-uniqueness rule's spirit (one URL → one group). Rare in practice. | S:75 R:65 A:80 D:75 |
| 21 | Certain | `internal/yamled` API extension: single `MergeScan(path, plan ScanPlan) error` taking structured additions for one atomic write | Clarified — chose MergeScan over composed primitives for atomicity (mirrors yamled's existing AppendURL contract), simpler call site, and self-documenting plan type. | S:95 R:55 A:75 D:90 |
| 22 | Certain | Tab completion for `<dir>` is non-issue: shell-native directory completion handles it; cobra default just needs to not break it | Clarified — completion at path-shaped argument positions is shell-handled; cobra contributes nothing here. No verification needed beyond standard cobra behavior. | S:95 R:90 A:90 D:95 |

22 assumptions (17 certain, 5 confident, 0 tentative, 0 unresolved).
