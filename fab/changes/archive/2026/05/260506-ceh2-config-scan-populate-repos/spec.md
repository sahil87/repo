# Spec: hop config scan — populate hop.yaml from on-disk repos

**Change**: 260506-ceh2-config-scan-populate-repos
**Created**: 2026-05-06
**Affected memory**:
- `docs/memory/cli/subcommands.md`
- `docs/memory/config/init-bootstrap.md`
- `docs/memory/config/scan.md` (new)
- `docs/memory/architecture/package-layout.md`
- `docs/memory/architecture/wrapper-boundaries.md`

## Non-Goals

- **Parallel filesystem walks** — sequential DFS is simple and meets the perf target (<1s for typical 30-repo trees). Parallelism deferred until a concrete >1000-repo complaint surfaces.
- **`git rev-parse --is-bare-repository` shell-out for bare-repo detection** — stat-based heuristic (see Filesystem walk) is sufficient; avoids one extra git invocation per candidate.
- **Comment-faithful migration of v0.0.1 `repos.yaml` schemas** — out of scope; users with v0.0.1 files must hand-port (consistent with `config-resolution.md`'s "no migration path" stance).
- **`hop config init --interactive`** — adjacent feature, not bundled here.
- **`hop doctor` / config validation subcommand** — adjacent feature, not bundled here.
- **`--group <name>` flag for a single explicit fallback group** — strictly less expressive than per-parent-dir invention; users can collapse groups by hand-editing YAML afterward.
- **Auto-creating `hop.yaml` when missing** — risks baking in a wrong `code_root`; users run `hop config init` first by design.
- **Modifying `code_root`** — `code_root` is durable; scan never overwrites it.

## CLI: `hop config scan <dir>`

### Requirement: Subcommand placement

`hop config scan` SHALL be a new cobra subcommand wired under the existing `config` parent (alongside `init` and `where`), defined in `src/cmd/hop/config.go` via a new factory `newConfigScanCmd()` registered with `cmd.AddCommand(...)` in `newConfigCmd`. It SHALL NOT be added as a top-level subcommand (Constitution VI).

#### Scenario: Cobra registration

- **GIVEN** the binary is built
- **WHEN** the user runs `hop config --help`
- **THEN** stdout lists `scan` alongside `init` and `where`
- **AND** the exit code is 0

#### Scenario: Direct invocation reachability

- **GIVEN** `hop.yaml` exists at the resolved path
- **WHEN** the user runs `hop config scan ~/code`
- **THEN** the binary executes the scan command's `RunE`
- **AND** does not error with "unknown command"

### Requirement: Argument validation

The subcommand SHALL accept exactly one positional argument (`<dir>`). Missing argument or extra arguments SHALL produce a usage error with exit code 2 (cobra's `cobra.ExactArgs(1)` rejection).

The `<dir>` argument SHALL be normalized in this order before any further processing:

1. `filepath.Clean(<dir>)`.
2. `filepath.EvalSymlinks(<cleaned>)` to resolve symlinks. EvalSymlinks errors (including ENOENT) are surfaced as a usage error.
3. `os.Stat(<resolved>)` — must indicate a directory; otherwise usage error.

The exact stderr message on validation failure SHALL be:

```
hop config scan: '<dir>' is not a directory.
```

(`<dir>` is the user-supplied value verbatim, not the cleaned/resolved form, so the user recognizes their input.)

#### Scenario: No directory argument

- **GIVEN** the user runs `hop config scan` with no positional
- **WHEN** cobra parses the command
- **THEN** stderr shows cobra's standard "accepts 1 arg(s), received 0" usage error
- **AND** the exit code is 2

#### Scenario: Argument is a file, not a directory

- **GIVEN** `/tmp/notadir.txt` exists and is a regular file
- **WHEN** the user runs `hop config scan /tmp/notadir.txt`
- **THEN** stderr shows `hop config scan: '/tmp/notadir.txt' is not a directory.`
- **AND** the exit code is 2

#### Scenario: Argument does not exist

- **GIVEN** `/no/such/path` does not exist on disk
- **WHEN** the user runs `hop config scan /no/such/path`
- **THEN** stderr shows `hop config scan: '/no/such/path' is not a directory.`
- **AND** the exit code is 2

#### Scenario: Argument is a symlink to a directory

- **GIVEN** `~/work` is a symlink to `~/Volumes/Mac/work` (which is a directory)
- **WHEN** the user runs `hop config scan ~/work`
- **THEN** EvalSymlinks resolves the symlink and the walk proceeds against the canonical target
- **AND** the exit code follows the scan's outcome (0 typically)

### Requirement: Flags

Two flags SHALL be defined:

- `--write` (bool, default `false`) — when true, merge results into the resolved `hop.yaml` via `internal/yamled` (atomic comment-preserving write); when false, emit the rendered YAML to stdout.
- `--depth N` (int, default `3`) — maximum DFS depth (root counts as depth 0). Values `< 1` SHALL be rejected as a usage error with exit code 2 and stderr:
  ```
  hop config scan: --depth must be >= 1.
  ```

#### Scenario: Default depth

- **GIVEN** the user runs `hop config scan ~/code` without `--depth`
- **WHEN** the walk begins
- **THEN** the effective depth limit is 3

#### Scenario: Invalid depth

- **GIVEN** the user runs `hop config scan ~/code --depth 0`
- **WHEN** the command starts
- **THEN** stderr shows `hop config scan: --depth must be >= 1.`
- **AND** the exit code is 2

### Requirement: `hop.yaml` precondition

Before walking, the subcommand SHALL call `config.Resolve()` to locate `hop.yaml`. If `Resolve()` returns an error (no config found, or `$HOP_CONFIG` set but missing), the subcommand SHALL emit a scan-specific stderr message in place of the resolver's default text:

```
hop config scan: no hop.yaml found at <resolved-bootstrap-path>.
Run 'hop config init' first, then re-run scan.
```

The `<resolved-bootstrap-path>` SHALL be the path that `config.ResolveWriteTarget()` would return (i.e., where `hop config init` would create the file). Exit code 1.

The `$HOP_CONFIG`-points-to-missing-file case is treated identically (the user must `init` to that location or unset the env var). The scan-specific message MAY include a hint for the env-var case if implementation-cheap; otherwise the unified message above is sufficient.

#### Scenario: No hop.yaml exists

- **GIVEN** `$HOP_CONFIG` is unset, `~/.config/hop/hop.yaml` does not exist
- **WHEN** the user runs `hop config scan ~/code`
- **THEN** stderr shows the scan-specific "no hop.yaml found at /home/<user>/.config/hop/hop.yaml" message
- **AND** the user receives the "run hop config init first" hint
- **AND** the exit code is 1
- **AND** no walk is performed (no `git` invocations)

#### Scenario: `$HOP_CONFIG` set to non-existent file

- **GIVEN** `$HOP_CONFIG=/tmp/nope.yaml` and `/tmp/nope.yaml` does not exist
- **WHEN** the user runs `hop config scan ~/code`
- **THEN** stderr shows the scan-specific missing-config message
- **AND** the exit code is 1

### Requirement: Exit codes

The subcommand SHALL use this exit code mapping:

| Code | Meaning |
|------|---------|
| 0 | Success (any number of repos found, including zero) |
| 1 | `hop.yaml` missing; YAML write failure; `git` not on PATH; load error on existing `hop.yaml` |
| 2 | Usage error (no `<dir>` arg, dir validation failure, `--depth < 1`) |

#### Scenario: Successful scan with zero repos

- **GIVEN** `hop.yaml` resolves and `~/empty` contains no `.git` directories within depth 3
- **WHEN** the user runs `hop config scan ~/empty`
- **THEN** stderr shows `hop config scan: scanned ~/empty (depth 3), found 0 repos. Nothing to add.`
- **AND** stdout shows the existing `hop.yaml` contents unchanged (print mode)
- **AND** the exit code is 0

### Requirement: Stdout / stderr conventions

- **stdout** in print mode (the default) SHALL contain the rendered YAML file as it would be written by `--write`. **stdout** in write mode SHALL be empty.
- **stderr** SHALL contain the human-readable summary header in both modes (print mode: prefix; write mode: prefix + `wrote: <path>` trailer). All `skip:` lines (per-repo skips) SHALL go to stderr.
- These conventions SHALL match the precedent set by `hop clone` (status on stderr, useful piping payload on stdout).

#### Scenario: Print mode is pipeable

- **GIVEN** the user wants to seed a fresh `hop.yaml`
- **WHEN** they run `hop config scan ~/code > hop.yaml`
- **THEN** the redirection captures only the rendered YAML (no summary noise)
- **AND** the summary still appears on the terminal (stderr)

## Filesystem walk: `internal/scan` package

A new internal package `src/internal/scan/` SHALL implement the directory walk. The package SHALL contain `scan.go` (production) and `scan_test.go` (table-driven tests using a synthesized `t.TempDir()` tree and an injected `GitRunner`).

### Requirement: Public API

The package SHALL export the following symbols:

```go
type Found struct {
    Path string  // canonical (EvalSymlinks-resolved) path to the repo's working tree
    URL  string  // remote URL (origin if present, else first remote)
}

type Skip struct {
    Path   string
    Reason string  // one of: "no remote", "bare repo", "worktree", "submodule"
}

type Options struct {
    Depth     int                                                                        // 0 means "root only"; CLI passes 3 by default
    GitRunner func(ctx context.Context, dir string, args ...string) ([]byte, error)      // injectable; defaults to internal/proc.RunCapture-equivalent
}

func Walk(ctx context.Context, root string, opts Options) ([]Found, []Skip, error)
```

`Walk` SHALL perform a stack-based DFS. The returned `Found` slice SHALL preserve discovery order (first-found wins on ties). `Skip` reasons SHALL be drawn from the closed set above (CLI summary counts on these).

#### Scenario: Caller injects a fake GitRunner for tests

- **GIVEN** a test sets `Options.GitRunner` to a function that returns canned output
- **WHEN** `Walk` is called
- **THEN** no actual `git` subprocess is spawned
- **AND** the test asserts on `Found` and `Skip` slices deterministically

### Requirement: DFS algorithm and depth handling

`Walk` SHALL implement DFS using a stack of `(path, depth)` entries. The root is enqueued at depth 0. For each entry popped:

1. If `depth > opts.Depth` → skip (do not descend, do not register).
2. `os.Lstat(path)` to check link status; if it's a symlink to a directory, follow via `os.Stat` (which resolves) — see "Symlinks and loop detection" below.
3. Classify the directory (see "Repo classification rules"). Once classified as a repo (or a skip), do **not** descend into the directory's children — repos' children are never themselves repos for our purposes.
4. Otherwise (no `.git` present), enqueue immediate subdirectories at `depth+1`.

Depth bound is **inclusive** — a `--depth 3` invocation at root `R` examines `R`, `R/*`, `R/*/*`, and `R/*/*/*` (and stops there).

#### Scenario: Depth halts descent

- **GIVEN** `~/code/sahil87/hop/.git` exists at depth 3 from `~/code`
- **WHEN** the user runs `hop config scan ~/code --depth 3`
- **THEN** the repo is found
- **AND** descent into `~/code/sahil87/hop/<children>` does not occur

#### Scenario: Depth limit excludes deeper repos

- **GIVEN** `~/code/a/b/c/d/.git` exists at depth 4 from `~/code`
- **WHEN** the user runs `hop config scan ~/code --depth 3`
- **THEN** that repo is not in the `Found` list

### Requirement: Repo classification rules

Within a directory `D`, the classifier SHALL apply these rules in order (first match wins):

1. **Worktree** — `D/.git` exists and is a regular file (not a directory). Skip with reason `"worktree"`. Do not descend.
2. **Submodule (heuristic)** — `D/.git` is a directory AND any ancestor on the DFS visited stack is itself a registered repo. Skip with reason `"submodule"`. Do not descend.
3. **Normal repo** — `D/.git` is a directory and no ancestor was a repo. Invoke `git -C D remote` (see "Git invocation contract"). If the output is empty → skip with reason `"no remote"`. Otherwise pick `origin` if listed, else the first line; invoke `git -C D remote get-url <selected>` for the URL; emit a `Found{Path: canonical(D), URL: trimmed-output}`. Do not descend.
4. **Bare repo** — `D` contains `HEAD` (regular file), `config` (regular file), and `objects/` (directory) at top level, AND `D/.git` does not exist. Skip with reason `"bare repo"`. Do not descend.
5. **Plain directory** — none of the above; recurse into children at `depth+1`.

The submodule heuristic SHALL use the visited-ancestor stack maintained during DFS — no `.gitmodules` parsing.

#### Scenario: Worktree detection

- **GIVEN** `~/code/sahil87/hop.worktrees/feature` is a git worktree (`.git` is a file containing `gitdir: ...`)
- **WHEN** the walk encounters that directory
- **THEN** the entry is classified as a worktree
- **AND** appears in `Skip` with reason `"worktree"`
- **AND** does not appear in `Found`

#### Scenario: Submodule detection via ancestor stack

- **GIVEN** `~/code/a/.git` is a directory AND `~/code/a/vendor/lib/.git` is a directory
- **WHEN** the walk descends through `~/code/a` then attempts `~/code/a/vendor/lib`
- **THEN** the rule-3 classification of `~/code/a` (registered as a repo) prevents descent — the submodule is never visited
- **NOTE** this is the practical effect of "do not descend into a registered repo." The submodule branch (rule 2) only fires when classification logic visits a `.git`-containing dir whose ancestor was already a repo (e.g., when the user passes `~/code/a/vendor/lib` as the scan root, an ancestor on the stack is moot — there is no ancestor — and rule 2 does not apply).

> NOTE: Because rule 3 prevents descending into registered repos, submodules under a recognized repo are typically never visited at all. Rule 2 acts as a defensive guard for unusual entry points (e.g., the user passes a submodule path directly, or DFS races where the ancestor classification was deferred). The implementation MAY rely solely on the no-descent invariant if it materially simplifies code; in that case rule 2's stderr never surfaces and the test suite SHALL document the choice.

#### Scenario: Bare repo detection

- **GIVEN** `~/mirrors/old-repo.git` contains `HEAD`, `config`, `objects/` at top level, no `.git` subdir
- **WHEN** the walk encounters that directory
- **THEN** it is classified as a bare repo
- **AND** appears in `Skip` with reason `"bare repo"`
- **AND** does not appear in `Found`

#### Scenario: Repo with no remote

- **GIVEN** `~/code/scratch/.git` exists and `git -C ~/code/scratch remote` produces empty output
- **WHEN** the walk encounters that directory
- **THEN** it is registered as `Skip{Path: ~/code/scratch, Reason: "no remote"}`
- **AND** does not appear in `Found`

#### Scenario: Repo with origin remote

- **GIVEN** `git -C D remote` lists `origin\nupstream`
- **WHEN** the classifier runs
- **THEN** `origin` is selected
- **AND** `git -C D remote get-url origin` provides the URL emitted in `Found.URL`

#### Scenario: Repo with non-origin first remote

- **GIVEN** `git -C D remote` lists `gitlab\nfork` (no `origin`)
- **WHEN** the classifier runs
- **THEN** `gitlab` is selected (first line)
- **AND** the URL comes from `git -C D remote get-url gitlab`

### Requirement: Symlinks and loop detection

Symlinks SHALL be followed during the walk. The walker SHALL maintain a visited set keyed by `(device, inode)` of canonical directory paths (resolved via `os.Stat`). On encountering a directory whose `(dev, ino)` is already in the visited set, the walker SHALL skip it without registering a `Skip` entry (silent dedup; this is loop suppression, not a user-facing skip).

Each `Found.Path` SHALL be the `filepath.EvalSymlinks` resolution of the repo's working tree (i.e., canonical, fully resolved). This canonicalization happens *before* the convention-check at the CLI layer.

#### Scenario: Symlink loop

- **GIVEN** `~/code/loop-a` symlinks to `~/code/loop-b`, and `~/code/loop-b` symlinks back to `~/code/loop-a`
- **WHEN** the walker encounters either
- **THEN** the second encounter is skipped silently (loop terminates)
- **AND** no infinite descent occurs

#### Scenario: Same repo reached via two paths

- **GIVEN** `~/code/foo` is a symlink to `~/code/repos/foo` (a real repo)
- **WHEN** the walk reaches it via either path
- **THEN** `Found.Path` is `~/code/repos/foo` (canonical)
- **AND** the repo is registered exactly once

### Requirement: Git invocation contract

All `git` invocations SHALL go through `internal/proc` (Constitution I). A new helper `RunCapture(ctx, dir, name string, args ...string) ([]byte, error)` SHALL be added to `internal/proc` if no equivalent exists; it SHALL set `cmd.Dir = dir`, capture stdout, route stderr to the parent. (`internal/proc.Run` does not currently accept a `dir` argument; `RunCapture` is the new variant. `Run` MAY be retained alongside, OR `Run` MAY be extended to accept a `dir` — the implementation chooses based on call-site impact.)

Each `git` invocation SHALL use a dedicated `context.Context` with a 5-second timeout (`context.WithTimeout(ctx, 5*time.Second)`).

If `git` is missing on PATH (the first invocation returns `proc.ErrNotFound`), the scan SHALL emit `hop: git is not installed.` to stderr and exit 1. This is checked **lazily** — the error surfaces only when the walk actually finds a `.git` candidate that requires invoking `git`.

#### Scenario: `git` missing on PATH

- **GIVEN** `git` is not on `$PATH` AND the walk encounters a `.git` directory
- **WHEN** the classifier invokes `git -C <dir> remote`
- **THEN** stderr shows `hop: git is not installed.`
- **AND** the exit code is 1
- **AND** the scan halts (does not continue to other candidates)

#### Scenario: `git remote` times out

- **GIVEN** a pathological repo where `git remote` hangs >5s
- **WHEN** the classifier invokes it
- **THEN** the per-invocation `context.WithTimeout` triggers
- **AND** the error propagates as a walk-level error (no Found, no partial output)
- **AND** exit code is 1

## Group assignment (CLI layer)

After `Walk` returns, the CLI layer SHALL assign each `Found` to a group using the rules below. This logic lives in `cmd/hop/config.go` (or a small helper file alongside it) — *not* in `internal/scan`, which is concerned only with discovery.

### Requirement: Convention check

For each `Found{Path, URL}`:

1. Compute `org := repos.DeriveOrg(URL)` and `name := repos.DeriveName(URL)`.
2. Compute `convention := filepath.Join(repos.ExpandDir(cfg.CodeRoot, ""), org, name)` (the canonical convention path for a flat-group entry; `org` is dropped when empty per existing schema rules).
3. If `filepath.Clean(Path) == filepath.Clean(convention)` → assign to the `default` flat group (URL only — no per-repo `dir:`).
4. Otherwise → assign to an *invented* group (next requirement).

Both `Path` and `convention` SHALL be canonical (`Path` already is per `Walk`; `convention` is computed via `ExpandDir` which fully resolves `~`). String comparison is case-sensitive (filesystem case sensitivity is the implicit ground truth — convention check uses the same FS rules as path resolution).

#### Scenario: Path matches convention exactly

- **GIVEN** `cfg.CodeRoot` is `~/code`, the URL is `git@github.com:sahil87/hop.git`, and `Found.Path` is `/home/user/code/sahil87/hop`
- **WHEN** the assignment runs
- **THEN** the URL goes into the `default` flat group

#### Scenario: Path differs from convention

- **GIVEN** the same URL but `Found.Path` is `/home/user/vendor/forks/hop`
- **WHEN** the assignment runs
- **THEN** the URL goes into an invented group (per next requirement)

### Requirement: Invented group naming (slugify)

When a `Found` does not match convention, the CLI SHALL invent a group from the *parent dir basename* of `Path`. The slugify rule SHALL be:

1. Take `base := filepath.Base(filepath.Dir(Path))`.
2. Lowercase.
3. Replace any run of characters not matching `[a-z0-9_-]` with a single `-`.
4. Trim leading and trailing `-`.
5. If the leading character is not in `[a-z]`, prefix `g` (e.g. `9-stuff` → `g9-stuff`).
6. If the result is empty (pathological input like `///`, `___`, all-symbols), the repo SHALL be skipped with stderr:
   ```
   skip: <Path>: cannot derive group name from parent dir '<base>'
   ```
   This skip counts in the summary as a generic skipped entry but does NOT block other repos.

The resulting slug SHALL conform to the existing schema regex `^[a-z][a-z0-9_-]*$` (yaml-schema.md).

#### Scenario: Plain alphabetic basename

- **GIVEN** `Found.Path = ~/vendor/forks/hop` → parent base `forks`
- **WHEN** slugify runs
- **THEN** the group name is `forks`

#### Scenario: Mixed case + symbols

- **GIVEN** parent base `My Stuff!`
- **WHEN** slugify runs
- **THEN** the group name is `my-stuff`

#### Scenario: Numeric leading char

- **GIVEN** parent base `9-experiments`
- **WHEN** slugify runs
- **THEN** the group name is `g9-experiments`

#### Scenario: Pathological input

- **GIVEN** parent base `___` (all underscores → trimmed to empty)
- **WHEN** slugify runs
- **THEN** the result is empty
- **AND** stderr shows the cannot-derive skip line
- **AND** the repo is not added to any group

### Requirement: Per-parent-dir granularity

Each *distinct* parent dir (after canonicalization) SHALL produce one invented group. Two different parent dirs SHALL NOT be merged even if their slugify outputs are textually equal — see "Conflict resolution" for the disambiguation suffix rule.

#### Scenario: Two distinct parent dirs, distinct names

- **GIVEN** `~/code/experiments` and `~/code/experiments-old` each contain repos
- **WHEN** the assignment runs
- **THEN** two invented groups exist: `experiments` and `experiments-old`
- **AND** repos are placed under their respective parent's group

### Requirement: Group dir rendering

The `dir:` field of an invented group SHALL be the canonical parent path, with `$HOME` substituted to `~/...` if the canonical path begins with `$HOME`. Otherwise it SHALL be the absolute path verbatim. This matches the style used in starter content and existing user configs.

#### Scenario: Parent dir under HOME

- **GIVEN** `$HOME=/home/user` and the canonical parent dir is `/home/user/vendor/forks`
- **WHEN** the group is rendered
- **THEN** `dir:` is `~/vendor/forks`

#### Scenario: Parent dir outside HOME

- **GIVEN** the canonical parent dir is `/srv/code/experiments`
- **WHEN** the group is rendered
- **THEN** `dir:` is `/srv/code/experiments`

### Requirement: Conflict resolution

When the CLI is preparing the merge plan against the loaded `cfg.Groups`:

1. **Slug matches existing group, dirs match (canonicalized)** → reuse that existing group; new URLs append to it.
2. **Slug matches existing group, dirs differ** → suffix with the smallest integer `-N` (starting at `-2`) that does not collide with any existing or already-invented group name. Emit a stderr note:
   ```
   note: invented group '<original-slug>' already exists in hop.yaml with a different dir; using '<original-slug>-2' for <new-dir>.
   ```
3. **Two distinct parent dirs slugify to the same name** during a single scan → first one wins; second is suffixed `-2` (and so on). Same stderr note.

#### Scenario: Reuse on dir match

- **GIVEN** `hop.yaml` already has `vendor: { dir: ~/vendor, urls: [...] }` and the scan finds a new repo at `~/vendor/new`
- **WHEN** the merge plan is built
- **THEN** the new URL is appended to the existing `vendor` group
- **AND** no `note:` line is emitted

#### Scenario: Suffix on dir mismatch

- **GIVEN** `hop.yaml` has `vendor: { dir: ~/old-vendor, urls: [...] }` and the scan finds a repo whose parent slugifies to `vendor` but whose canonical parent is `~/new-vendor`
- **WHEN** the merge plan is built
- **THEN** an invented group `vendor-2` is created with `dir: ~/new-vendor`
- **AND** stderr emits the `note:` line

## Output rendering

Both modes SHALL route through `internal/yamled` to preserve comments. The only difference is the sink — stdout (print) vs. atomic file replace (write).

### Requirement: Render path

Print mode and write mode SHALL share the same in-memory render produced by `internal/yamled`. Print mode emits the rendered bytes to `cmd.OutOrStdout()`; write mode performs `internal/yamled.MergeScan` (atomic temp+rename).

#### Scenario: Print mode preserves comments

- **GIVEN** `hop.yaml` contains a head comment `# my repos` and inline comments on each URL
- **WHEN** the user runs `hop config scan ~/code` (print mode)
- **THEN** stdout includes those comments verbatim (modulo yaml.v3 indentation normalization)
- **AND** `hop.yaml` on disk is unchanged

#### Scenario: Write mode preserves comments and is atomic

- **GIVEN** the same `hop.yaml`
- **WHEN** the user runs `hop config scan ~/code --write`
- **THEN** `hop.yaml` is updated via temp+rename in the same directory
- **AND** the file's previous comments are preserved
- **AND** stdout is empty
- **AND** stderr ends with `wrote: <resolved-hop.yaml-path>`

### Requirement: Print mode header

Print mode SHALL prepend a single header comment block to the rendered YAML, formatted as:

```
# hop config — generated by 'hop config scan <user-arg>' on <YYYY-MM-DD> (UTC).
# Run with --write to merge into <resolved-hop.yaml-path>.
```

Where `<user-arg>` is the user-supplied directory string (verbatim, not canonicalized — keeps the line readable), `<YYYY-MM-DD>` is today's date in **UTC** (`time.Now().UTC().Format("2006-01-02")`), and `<resolved-hop.yaml-path>` is from `config.Resolve()`. The literal `(UTC)` suffix SHALL appear immediately after the date so the timezone is unambiguous to the reader.

The header is part of the *stdout* render only (write mode does not modify the file's existing head comments).

#### Scenario: Header content

- **GIVEN** today is 2026-05-07 (UTC) and the user runs `hop config scan ~/code`
- **WHEN** print mode renders
- **THEN** stdout's first two lines are the header comment exactly as specified, with `2026-05-07 (UTC)` as the date stamp

### Requirement: Group ordering

In the rendered YAML output (both modes), groups SHALL appear in this order:

1. Existing groups from the loaded `hop.yaml`, in their original source order (preserving `cfg.Groups` order).
2. `default` (if not already present in #1, AND if scan is contributing entries to it; if `default` already exists in #1 it stays in its source-ordered slot).
3. Invented groups (those not present in #1), sorted alphabetically by group name (post-slugify).

Existing groups retain their existing URLs; scan-contributed URLs are appended within each group at the end of the URL list (or `urls:` sequence for map-shaped groups).

#### Scenario: Mixed existing and invented

- **GIVEN** `hop.yaml` has `vendor` and `default` (in that source order); scan invents `experiments`
- **WHEN** the YAML is rendered
- **THEN** group order is `vendor`, `default`, `experiments`

#### Scenario: Invented groups sorted

- **GIVEN** scan invents `zebra`, `alpha`, `mango`
- **WHEN** rendered
- **THEN** they appear as `alpha`, `mango`, `zebra` (alphabetical) after existing groups and `default`

### Requirement: Stderr summary

Both print and write modes SHALL emit a single summary block on stderr. The block format is:

```
hop config scan: scanned <user-arg> (depth N), found <K> repos.
  matched convention (default): <C> [(write only: <C-new> new, <C-existing> already registered)]
  invented groups: <I> (<comma-separated names>[; <new-status>])
  skipped: <S1> worktree, <S2> submodule, <S3> bare repo, <S4> no remote[, <S5> no group name]
[write only: wrote: <resolved-hop.yaml-path>]
[print only: Run with --write to merge into <resolved-hop.yaml-path>.]
```

Counts of zero MAY be elided per category to keep the line short. Exact text wording MAY vary slightly to fit the actual data (e.g., singular `1 worktree` vs. plural `2 worktrees`); the structure (one summary, one trailer) is binding.

#### Scenario: Print-mode summary

- **GIVEN** scan over `~/code` (depth 3) finds 7 repos: 5 default, 2 invented (`experiments`, `vendor`); skips 1 worktree, 1 submodule
- **WHEN** print mode runs
- **THEN** stderr shows the summary above and ends with `Run with --write to merge into ~/.config/hop/hop.yaml.`

#### Scenario: Write-mode summary

- **GIVEN** the same scan, with 3 of the 5 default URLs already registered in the existing `default` group
- **WHEN** write mode runs
- **THEN** stderr shows `matched convention (default): 5 (3 new, 2 already registered)`
- **AND** ends with `wrote: ~/.config/hop/hop.yaml`

#### Scenario: Zero repos found

- **GIVEN** scan finds zero repos
- **WHEN** print mode runs
- **THEN** stderr shows `hop config scan: scanned <user-arg> (depth N), found 0 repos. Nothing to add.`
- **AND** stdout shows the existing `hop.yaml` contents unchanged
- **AND** the exit code is 0

## `internal/yamled` API extension

The `internal/yamled` package SHALL be extended with a single new public entry point for scan merges. The existing `AppendURL` SHALL remain unchanged.

### Requirement: `MergeScan` signature

```go
// MergeScan applies a structured plan of scan additions to the YAML file at
// path in a single atomic write. Comments are preserved (yaml.v3 round-trip;
// indentation normalized to yaml.v3 defaults — same contract as AppendURL).
//
// Dedup: any URL in plan.DefaultURLs or plan.InventedGroups[i].URLs that
// already appears in any existing group of the loaded file is silently
// skipped (matches AppendURL's contract and the parser's URL-uniqueness rule).
// The CLI layer is responsible for surfacing skip-by-dedup messages — yamled
// stays UI-free.
//
// Group ordering: existing groups preserved in source order; default placed
// per "Group ordering" rules; invented groups appended after existing groups
// in the order given by plan.InventedGroups (caller pre-sorts alphabetically).
func MergeScan(path string, plan ScanPlan) error

type ScanPlan struct {
    DefaultURLs    []string         // appended to the "default" flat group; created if absent
    InventedGroups []InventedGroup  // appended after existing groups
}

type InventedGroup struct {
    Name string  // already slugified by the caller; conforms to ^[a-z][a-z0-9_-]*$
    Dir  string  // already canonical (with ~-substitution where applicable)
    URLs []string
}
```

`MergeScan` SHALL also be the rendering primitive shared with print mode. Print mode SHALL invoke a render-only sibling — either an exported `RenderScan(path string, plan ScanPlan) ([]byte, error)` returning the rendered bytes, OR an internal helper used by both `MergeScan` (which writes) and a public `RenderScan` (which returns bytes). The naming/structure is an implementation detail; the contract is "same render in both modes."

#### Scenario: Default group does not exist

- **GIVEN** `hop.yaml` has no `default` group, only `vendor`
- **WHEN** `MergeScan` runs with `DefaultURLs: ["git@github.com:foo/bar.git"]`
- **THEN** a new `default` flat group is appended with that URL
- **AND** existing `vendor` is unchanged

#### Scenario: URL already registered elsewhere

- **GIVEN** `hop.yaml`'s `vendor` group already contains `git@github.com:foo/bar.git`, and `plan.DefaultURLs` also contains it
- **WHEN** `MergeScan` runs
- **THEN** the URL is silently skipped (not duplicated; no error)
- **AND** the CLI layer was responsible for emitting any user-visible skip note

#### Scenario: Invented group with map shape

- **GIVEN** `plan.InventedGroups = [{Name: "vendor", Dir: "~/vendor", URLs: ["U"]}]`
- **WHEN** `MergeScan` runs
- **THEN** the file gains a `vendor` group rendered as `{ dir: ~/vendor, urls: [U] }`
- **AND** the write is atomic (temp+rename, same dir)

### Requirement: Atomicity and comment preservation

`MergeScan` SHALL use the same `atomicWrite` (temp file + rename in the same directory) as `AppendURL`. File mode SHALL be preserved from the original; if the file does not exist, mode 0644 SHALL be used (matches `WriteStarter`). On rename failure the original is left untouched.

#### Scenario: Rename failure preserves original

- **GIVEN** `hop.yaml` is on a filesystem where `os.Rename` fails (e.g., target dir is read-only after temp creation; contrived test)
- **WHEN** `MergeScan` runs
- **THEN** the original `hop.yaml` is unchanged
- **AND** `MergeScan` returns a non-nil error

## `hop config init` post-write tip update

### Requirement: Tip line extension

The stderr message printed by `newConfigInitCmd`'s `RunE` SHALL be extended to mention `hop config scan`. The existing wording SHALL be preserved as a suffix (it remains the tip about `$HOP_CONFIG`); the new opening sentence is the additive piece.

The exact stderr text SHALL be:

```
Edit the file to add your repos, or run `hop config scan <dir>` to populate from existing on-disk repos.
Tip: set $HOP_CONFIG in your shell rc to point at a version-tracked location (a git-tracked dotfile, Dropbox, etc.) so this config moves with you across machines.
```

(Two lines on stderr; previously one. Behavior of `init` itself is otherwise unchanged.)

#### Scenario: Init followed by scan tip

- **GIVEN** no `hop.yaml` exists at the resolved path
- **WHEN** the user runs `hop config init`
- **THEN** stdout shows `Created <path>`
- **AND** stderr shows the two-line tip exactly as specified
- **AND** the exit code is 0

## External tool requirements

### Requirement: `git` lazy check

`git` SHALL be required only when the walk actually finds a `.git` candidate that requires a `git remote` invocation. Empty scan trees (zero `.git` discoveries) SHALL succeed without invoking `git`. When required and missing, the exact stderr line SHALL be:

```
hop: git is not installed.
```

Exit code 1. This matches the existing `gitMissingHint` precedent in `clone.go`.

The CLI surface table in `docs/specs/cli-surface.md` and the External Tool Availability table SHALL gain a row for `hop config scan` referencing the same `hop: git is not installed.` message.

#### Scenario: Empty tree, git missing

- **GIVEN** `git` is not on PATH AND `~/empty` contains no `.git`
- **WHEN** the user runs `hop config scan ~/empty`
- **THEN** the scan completes successfully (exit 0)
- **AND** no `git is not installed` message is shown

#### Scenario: Non-empty tree, git missing

- **GIVEN** `git` is not on PATH AND `~/code/foo/.git` exists
- **WHEN** the user runs `hop config scan ~/code`
- **THEN** stderr shows `hop: git is not installed.`
- **AND** the exit code is 1

## Security

(Constitution I — Security First.)

### Requirement: Input validation before subprocess

The `<dir>` argument SHALL be validated via `filepath.Clean` → `filepath.EvalSymlinks` → `os.Stat` (must be directory) before any `git` subprocess is invoked. EvalSymlinks failures (including ENOENT) SHALL produce the usage error documented above; no `git` invocation occurs on a failed validation.

#### Scenario: Validation precedes execution

- **GIVEN** `<dir>` does not exist
- **WHEN** the subcommand starts
- **THEN** stderr shows the not-a-directory message
- **AND** no `git` subprocess is spawned (auditable via test seam)

### Requirement: No shell interpolation

All `git` invocations SHALL use `internal/proc.RunCapture` (or equivalent) with explicit argument slices via `exec.CommandContext`. No `git` argument SHALL be passed via `sh -c` or any string-concatenation path.

The `<dir>` argument and discovered repo paths SHALL be passed as the `cmd.Dir` field (or via `git -C <path>` as a single-argument element), never interpolated into a shell string.

Each `git` invocation SHALL use `context.WithTimeout(ctx, 5*time.Second)`.

#### Scenario: Repo path with shell metacharacters

- **GIVEN** a discovered repo at `/tmp/test;rm/foo/.git`
- **WHEN** the classifier runs `git -C /tmp/test;rm/foo remote`
- **THEN** the path is passed as a single argv element to `exec.CommandContext`
- **AND** no shell evaluation occurs
- **AND** `git` either succeeds or fails on the literal path (no command injection)

### Requirement: Permissive on URL contents

URL strings returned by `git remote get-url` SHALL be passed back into the YAML output verbatim. They SHALL NOT be re-interpreted, validated against a regex, or shell-expanded by hop. Existing schema permissiveness on URLs (yaml-schema.md) covers this — the parser already accepts `- not a url` as a valid entry.

#### Scenario: URL with unusual characters

- **GIVEN** a repo whose remote URL is `git@host:owner/repo with spaces.git`
- **WHEN** scan emits the URL into YAML
- **THEN** the URL is YAML-quoted as needed by yaml.v3
- **AND** is reproduced byte-for-byte on subsequent loads

## Design Decisions

1. **Subcommand placement under `hop config`, not top-level.**
   - *Why*: `scan` is a config-state operation that fits cleanly alongside `init` and `where`. Constitution VI requires explicit justification for new top-levels; this case fails that bar (the operation reads/writes config, doesn't introduce a new domain). Slot under `config` is unambiguous from the user's perspective ("things you do to the hop config file").
   - *Rejected*: top-level `hop scan` — would suggest scanning is a standalone hop concept independent of config; misleading.
   - *Rejected*: overload `hop config init --from <dir>` — collapses bootstrap and population; risks wrong `code_root` when scan target is one level deep; init/scan are separable steps with different idempotency stories.

2. **Print/write symmetry: both render through `internal/yamled`.**
   - *Why*: clean mental model ("print is what `--write` would have produced, sent to stdout instead"). Comment preservation in print mode is free if write mode already preserves them. One render path = fewer divergence bugs.
   - *Rejected*: print emits a fragment-only diff — initially preferred for honesty about new entries, but the cold-start use case (`scan > hop.yaml`) wants a full file.
   - *Rejected*: print as plain text + write as YAML — asymmetric, two render paths to maintain.

3. **`MergeScan` over composed `EnsureGroup` + `AppendURLs` primitives.**
   - *Why*: atomicity matters (yamled's existing `AppendURL` contract is one atomic write per call; composing two writes loses atomicity unless the caller orchestrates a node-tree-level transaction, which is exactly what `MergeScan` already is). `ScanPlan` is self-documenting at the call site. Single entry point matches the existing `AppendURL` pattern.
   - *Rejected*: composed primitives — caller would need to assemble a node tree manually OR accept multi-step non-atomic writes.

4. **Stat-based bare-repo detection (not `git rev-parse --is-bare-repository`).**
   - *Why*: avoids one extra `git` invocation per candidate; bare repos are rare under typical scan roots; the heuristic (`HEAD` + `config` + `objects/` + no `.git` subdir) catches the standard `git init --bare` layout reliably.
   - *Rejected*: `git rev-parse` — extra subprocess per candidate, marginal gain; the false-positive class (a non-bare repo that coincidentally has all three top-level entries without `.git/`) does not occur under standard git layouts.

5. **Submodule heuristic: ancestor on visited stack is itself a repo.**
   - *Why*: combined with the "do not descend into a registered repo" invariant, this is mechanically sufficient — submodules under a recognized parent repo are never visited, so the explicit check is mostly defensive. Avoids `.gitmodules` parsing complexity for negligible additional precision.
   - *Rejected*: parse `.gitmodules` of each ancestor — significantly more complex; doesn't change outcomes in the common case.

6. **Default depth 3.**
   - *Why*: covers the canonical `<code_root>/<org>/<name>` (depth 2 from `<code_root>`) plus one slack level for non-convention layouts (e.g., `~/code/work/team/repo`). Empirically sufficient for typical user trees.
   - *Rejected*: unbounded — risk of long walks in pathological trees with no upside; requires either a sentinel like `--depth 0 = unbounded` (overloads zero) or a separate `--unbounded` flag (extra surface).
   - *Rejected*: depth 5 — wider tolerance not justified by typical user organization patterns; users with deep layouts pass `--depth N`.

7. **Follow symlinks with inode-based dedup, canonicalize before convention-check.**
   - *Why*: users intentionally symlink directories (e.g., Time Machine, network mounts, ad-hoc repo aliases). Refusing to follow them would silently skip valid repos. Inode dedup is the standard `find -L` approach and is robust to loops. Canonicalizing before the convention check ensures that a repo at `~/code/foo` is treated identically whether reached via `~/code` or via a symlink at `~/work-code/foo`.
   - *Rejected*: skip symlinks — silent under-discovery; user-confusing.
   - *Rejected*: follow without dedup — infinite loops on cyclic symlinks.

8. **Print mode adds a header comment naming the source command and date.**
   - *Why*: provides a self-describing trace for `scan > hop.yaml` workflows; the user (or a future reader) sees how the file was generated.
   - *Rejected*: no header — loses the "where did this file come from" context.

## Clarifications

### Session 2026-05-07 (auto)

Six Confident assumptions upgraded to Certain after spec-stage triage. Each was a minor formalization (cosmetic format, mechanical rule, deterministic default) with no genuine alternative worth flagging.

| # | Decision | Resolution |
|---|----------|-----------|
| 19 | New `docs/memory/config/scan.md` file | Locked — surface area justifies own file per project convention |
| 20 | Group-name collision: `-2`, `-3` suffix on dir mismatch | Locked — mechanical numeric disambiguation matches schema's URL-uniqueness spirit |
| 23 | Print-mode header comment exact text | Locked — cosmetic-only, no alternative format requested |
| 27 | `Walk` returns DFS discovery order | Locked — required for reproducible tests and slug-tie tiebreaking |
| 28 | Closed `Skip.Reason` enum, slugify-failure at CLI layer | Locked — separation-of-concerns per existing wrapper-boundaries memory |
| 30 | UTC date with explicit `(UTC)` suffix in header | Flipped from local — reproducible across collaborators, matches generated-artifact conventions |

#17 (submodule detection — ancestor-stack heuristic, with note that it may be redundant) retained as Confident: the spec genuinely flags an implementation choice. Apply stage will pick one path.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Subcommand placement: `hop config scan` (under existing `config` parent, not new top-level) | Confirmed from intake #1; Constitution VI signal is unambiguous; existing `init`/`where` siblings make the slot natural. | S:95 R:80 A:95 D:95 |
| 2 | Certain | Print full file by default, `--write` merges into resolved `hop.yaml` | Confirmed from intake #2. Spec adds the header-comment requirement (assumption #23) and codifies stdout/stderr split. | S:95 R:80 A:85 D:95 |
| 3 | Certain | Auto-derive groups: convention match → `default`; non-match → invent group from parent-dir basename (slugified) | Confirmed from intake #3. Spec formalizes the canonicalize-then-compare contract. | S:95 R:70 A:90 D:95 |
| 4 | Certain | Remote selection: `origin` if present, else first remote; no remote → skip | Confirmed from intake #4. Matches `git`'s own conventions. | S:95 R:85 A:95 D:95 |
| 5 | Certain | Skip rules: bare repos, worktrees, submodules, no-remote (closed set of `Skip.Reason` strings) | Confirmed from intake #5. Spec narrows `Skip.Reason` to a closed enum-like set so summary counts can rely on it. | S:95 R:80 A:90 D:95 |
| 6 | Certain | Default depth 3, `--depth N` flag override; `N < 1` is a usage error | Confirmed from intake #6. Spec adds the `< 1` validation rule explicitly (assumption #24). | S:95 R:90 A:95 D:95 |
| 7 | Certain | Symlinks followed with `(dev, inode)` dedup; canonical-path resolution before convention check | Confirmed from intake #7. Spec specifies that loop dedup is silent (no `Skip` entry). | S:95 R:80 A:85 D:90 |
| 8 | Certain | `hop.yaml` missing → exit 1 with scan-specific stderr pointing at `hop config init` | Confirmed from intake #8. Spec specifies that `<resolved-bootstrap-path>` is `ResolveWriteTarget()`'s output. | S:95 R:80 A:85 D:95 |
| 9 | Certain | Group-naming: slugify parent dir basename via the rule sequence (lowercase → replace runs of non-`[a-z0-9_-]` with `-` → trim → ensure leading char `[a-z]` via `g` prefix); empty result → skip with stderr | Confirmed from intake #9. Spec gives the exact algorithm in numbered steps. | S:90 R:75 A:90 D:90 |
| 10 | Certain | Group-dir granularity: one group per distinct parent dir (no collapsing) | Confirmed from intake #10. Spec confirms with the ancillary "different parent dirs that slugify to same name → suffix" rule. | S:85 R:70 A:85 D:90 |
| 11 | Certain | Output ordering: existing groups (source order) → `default` → invented (alphabetical) | Confirmed from intake #11. Spec adds the wrinkle that `default` already in source order keeps its slot. | S:95 R:90 A:95 D:95 |
| 12 | Certain | Both `--write` and print mode in v1 (not deferred) | Confirmed from intake #12. | S:100 R:95 A:95 D:100 |
| 13 | Certain | Bundle: extend `hop config init`'s post-write tip to mention `hop config scan` (exact wording in spec) | Confirmed from intake #13; spec pins the exact stderr text (assumption #25). | S:95 R:95 A:95 D:95 |
| 14 | Certain | New `internal/scan` package with `Walk`, `Found`, `Skip`, `Options` public API | Upgraded from intake #14 (Confident → Certain): spec now pins the public surface; existing `internal/yamled`/`internal/update` precedent makes the pattern unambiguous. | S:90 R:75 A:95 D:90 |
| 15 | Certain | Git invocations via `internal/proc` with 5s `context.WithTimeout` per call; new `RunCapture` helper added if needed | Upgraded from intake #15 (Confident → Certain): Constitution I and existing precedent in `internal/update` make this binding. The new `RunCapture` (or extension of `Run`) is straightforward. | S:95 R:80 A:95 D:90 |
| 16 | Certain | Bare-repo detection via stat (`HEAD` + `config` + `objects/` at top level, no `.git` subdir) | Confirmed from intake #16. | S:95 R:75 A:80 D:80 |
| 17 | Confident | Submodule detection via "ancestor on visited stack is itself a repo" — implementation MAY rely solely on the no-descent invariant if simpler | Confirmed from intake #17, with spec note clarifying that rule 2 may be redundant in practice; either way the user-facing behavior is the same. | S:80 R:75 A:80 D:80 |
| 18 | Certain | Print mode preserves comments — routes through yamled's render (same render as write mode); print is "what `--write` would produce, sent to stdout instead" | Confirmed from intake #18. Spec codifies the shared render-path contract. | S:95 R:70 A:80 D:95 |
| 19 | Certain | New file `docs/memory/config/scan.md` (rather than appending to `init-bootstrap.md`) | Clarified — locked: scan has enough surface area (walk + classify + assign + render + merge) to warrant its own file per the project's per-feature memory convention. | S:95 R:90 A:85 D:80 |
| 20 | Certain | Group-name collision handling: `-2`, `-3` suffix on dir mismatch; reuse on dir match | Clarified — locked: mechanical numeric-suffix disambiguation is the standard pattern (matches existing schema's URL-uniqueness spirit); no alternative offers a meaningful gain. | S:95 R:70 A:85 D:80 |
| 21 | Certain | `internal/yamled` API extension: single `MergeScan(path, ScanPlan) error` + companion render-only path for print mode | Confirmed from intake #21. Spec adds the implementation note that `RenderScan` (or equivalent) is the shared primitive. | S:95 R:60 A:80 D:90 |
| 22 | Certain | Tab completion for `<dir>` is shell-handled; cobra default is sufficient | Confirmed from intake #22. | S:95 R:90 A:90 D:95 |
| 23 | Certain | Print-mode header comment: `# hop config — generated by 'hop config scan <user-arg>' on <YYYY-MM-DD> (UTC).` followed by `# Run with --write to merge into <resolved-hop.yaml-path>.` | Clarified — locked: spec pins the exact text including the `(UTC)` suffix per #30. Cosmetic-only and reversible. | S:95 R:90 A:75 D:80 |
| 24 | Certain | `--depth N` validation: `N < 1` is a usage error (exit 2) with stderr `hop config scan: --depth must be >= 1.` | New at spec stage. Intake said "`N < 1` → usage error, exit 2"; spec pins the exact stderr text. | S:90 R:90 A:95 D:90 |
| 25 | Certain | `hop config init` extended stderr text (exact two-line wording given in spec) | New at spec stage; intake described the change conceptually, spec pins both lines verbatim so apply has no ambiguity. | S:90 R:95 A:90 D:95 |
| 26 | Certain | Stderr summary block format (one line + indented breakdown + trailing tip/wrote line) | New at spec stage. Intake gave two example outputs (print-mode and write-mode); spec generalizes them into a single shape with optional sub-counts. Reversible cosmetic decision. | S:80 R:90 A:90 D:80 |
| 27 | Certain | `Walk` returns `Found` slice in DFS discovery order (deterministic given a stable filesystem) | Clarified — locked: deterministic ordering is the only sensible default; required for reproducible test fixtures and the "first match wins on slug ties" rule. | S:95 R:80 A:90 D:85 |
| 28 | Certain | `Skip` reasons are a closed set of strings (`"no remote"`, `"bare repo"`, `"worktree"`, `"submodule"`); slugify-failures are emitted as a `skip:` stderr line at the CLI layer (not as a `Skip` entry) | Clarified — locked: separation-of-concerns (discovery enum vs. CLI-layer slugify) keeps `internal/scan` UI-free per existing wrapper-boundaries memory. | S:95 R:75 A:85 D:80 |
| 29 | Certain | All `git` calls use `context.WithTimeout(ctx, 5*time.Second)` per invocation | Upgraded from intake's narrative description to a binding requirement. Matches existing `internal/update` precedent (30s/120s elsewhere; 5s is generous for local `git remote`). | S:95 R:85 A:95 D:90 |
| 30 | Certain | Print-mode "header date" is rendered in UTC with explicit `(UTC)` suffix: `... on YYYY-MM-DD (UTC).` | Clarified — user chose UTC over local-time. Reasoning: reproducible across collaborators/machines, matches generated-artifact conventions, explicit suffix removes timezone ambiguity. | S:95 R:95 A:90 D:95 |

30 assumptions (29 certain, 1 confident, 0 tentative, 0 unresolved).
