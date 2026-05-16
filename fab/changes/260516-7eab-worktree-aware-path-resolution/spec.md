# Spec: Worktree-Aware Path Resolution

**Change**: 260516-7eab-worktree-aware-path-resolution
**Created**: 2026-05-16
**Affected memory**: `docs/memory/cli/subcommands.md`, `docs/memory/cli/match-resolution.md`, `docs/memory/architecture/wrapper-boundaries.md`

## Non-Goals

- **Cross-tree batch git ops** (e.g., `hop sync --all-trees`) — out of scope; worktrees usually carry mid-flight branches and auto-rebasing/pushing across them is footgun-shaped.
- **`hop clone --wt-init`** — out of scope; couples hop ↔ wt tighter than the "Wrap, don't reinvent" spirit warrants.
- **Worktree creation, deletion, init from hop** — wt owns these. hop only *locates* worktrees.
- **hop.yaml worktree fields** — violates Constitution Principle II (No Database); worktree state stays derivable from `wt list --json` at request time.
- **`hop ls --trees` positional** (e.g., `hop ls --trees outbox` for single-repo deep listing) — out of v1 scope; stays all-or-nothing. Trivially additive later if demanded.

---

## CLI Surface: Grammar Extension `<name>/<wt-name>`

### Requirement: Optional `/<wt-name>` suffix on the repo positional

The repo positional accepted by `hop`, `hop <name> where`, `hop <name> cd` (shell), `hop <name> open`, `hop <name> -R <cmd>...`, and the shim's tool-form `hop <name> <tool> [args...]` SHALL accept an optional `/<wt-name>` suffix. When the suffix is present, the resolved `*repos.Repo`'s `Path` field SHALL be the absolute path of the named worktree (as reported by `wt list --json` invoked in the repo's main checkout). All other `Repo` fields (`Name`, `Group`, `URL`, `Dir`) SHALL remain derived from the configured registry entry — they describe the *registry* identity, not the on-disk worktree.

The split is performed on the **first** `/` character in the query, not on the last. The LHS is resolved by the existing match-or-fzf algorithm; the RHS is matched exactly (case-sensitive) against the `name` field of `wt list --json` entries.

When no `/` is present in the query, resolution behavior is unchanged from today (full backward compatibility).

#### Scenario: Worktree resolution for `where`

- **GIVEN** `hop.yaml` lists a repo `outbox` resolving to `~/code/sahil87/outbox` AND a worktree named `feat-x` exists at `~/code/sahil87/outbox.worktrees/feat-x`
- **WHEN** I run `hop outbox/feat-x where`
- **THEN** stdout is `~/code/sahil87/outbox.worktrees/feat-x` followed by a newline
- **AND** stderr is empty
- **AND** exit code is 0

#### Scenario: Worktree resolution for `open`

- **GIVEN** the same setup as above
- **WHEN** I run `hop outbox/feat-x open`
- **THEN** the binary execs `wt open ~/code/sahil87/outbox.worktrees/feat-x` (positional path arg, stdio inherited)
- **AND** wt's interactive app menu reaches the user's terminal targeting the worktree path
- **AND** exit code matches wt's exit code

#### Scenario: Worktree resolution for `-R`

- **GIVEN** the same setup
- **WHEN** I run `hop outbox/feat-x -R git status` (under the shim, which rewrites to `command hop -R outbox/feat-x git status`)
- **THEN** the binary execs `git status` with `cwd = ~/code/sahil87/outbox.worktrees/feat-x`
- **AND** stdin/stdout/stderr are inherited
- **AND** exit code matches `git status`'s exit code

#### Scenario: Worktree resolution for tool-form sugar

- **GIVEN** the same setup (shim active)
- **WHEN** I run `hop outbox/feat-x cursor .`
- **THEN** the shim rewrites to `command hop -R outbox/feat-x cursor .`
- **AND** the binary execs `cursor .` with `cwd = ~/code/sahil87/outbox.worktrees/feat-x`

#### Scenario: Worktree resolution for bare-name `cd` via shim

- **GIVEN** the same setup AND the user has run `eval "$(hop shell-init zsh)"`
- **WHEN** they run `hop outbox/feat-x` (1 arg)
- **THEN** the shim's bare-name branch fires (`_hop_dispatch cd "outbox/feat-x"`)
- **AND** the helper runs `command hop "outbox/feat-x" where` to resolve the worktree path
- **AND** `cd --` lands in `~/code/sahil87/outbox.worktrees/feat-x`

### Requirement: `<name>/main` resolves to the main worktree path

When the RHS of the `/` suffix is `main` (or any name that `wt list --json` reports for the main worktree), resolution SHALL yield the main checkout's path — the same path bare `hop <name>` would resolve to. No special-case branching in hop is required; the `wt list --json` round-trip naturally yields the main entry when `is_main: true`.

#### Scenario: Explicit main-worktree resolution

- **GIVEN** `hop.yaml` lists `outbox` resolving to `~/code/sahil87/outbox`
- **WHEN** I run `hop outbox/main where`
- **THEN** stdout is `~/code/sahil87/outbox` followed by a newline
- **AND** exit code is 0

### Requirement: LHS-only queries are unaffected

A query without `/` (the existing form) SHALL resolve exactly as it did before this change. `wt list --json` is NOT invoked when no `/` is present.

#### Scenario: Backward-compatible bare query

- **GIVEN** the same setup
- **WHEN** I run `hop outbox where`
- **THEN** `wt` is NOT invoked (no subprocess spawned)
- **AND** stdout is the main checkout's path
- **AND** behavior is byte-for-byte identical to pre-change `hop outbox where`

---

## CLI Surface: `hop ls --trees` flag

### Requirement: `--trees` flag fans out across configured cloned repos

`hop ls` SHALL accept a `--trees` boolean flag (default `false`). When `--trees` is set, `hop ls` SHALL iterate the configured registry in `repos.FromConfig` source order and, for each repo whose `.git` exists on disk (clone state = `stateAlreadyCloned`), invoke `wt list --json` in that repo's main checkout and emit a per-repo summary line. Non-cloned repos SHALL surface a `(not cloned)` indicator instead of fanning out wt.

`hop ls` without `--trees` SHALL behave exactly as it does today (aligned `name  path` columns).

#### Scenario: `--trees` against a mixed registry

- **GIVEN** `hop.yaml` lists four repos: `outbox` (cloned, 3 worktrees: `main`, `feat-x` dirty, `hotfix` with 2 unpushed commits), `dotfiles` (cloned, 1 worktree: `main`), `hop` (cloned, 2 worktrees: `main`, `refactor-resolve` current), `loom` (not cloned)
- **WHEN** I run `hop ls --trees`
- **THEN** stdout contains four rows, one per registry entry, in YAML source order
- **AND** the `outbox` row indicates 3 worktrees with `feat-x` flagged dirty and `hotfix` flagged with unpushed commits
- **AND** the `loom` row indicates `(not cloned)` and `wt list` was NOT invoked for it
- **AND** exit code is 0

#### Scenario: Per-row output format

- **GIVEN** the same setup
- **WHEN** I inspect stdout
- **THEN** each cloned-repo row SHALL match the shape:
  ```
  {name}<spaces>{N} tree(s)  ({worktree-list})
  ```
  where `{worktree-list}` is a comma-separated list of worktree names, each suffixed with `*` if dirty and `↑N` (N = `unpushed` count) if any commits are unpushed. The current worktree (`is_current: true`) is unmarked (the listing is per-repo; there is no ambient "current" outside a single repo's main checkout).
- **AND** non-cloned rows SHALL match: `{name}<spaces>(not cloned)`
- **AND** the `{name}<spaces>` column is left-aligned with width = length of the longest configured repo name + 2 (matching the existing `hop ls` alignment rule)

#### Scenario: Per-row output format — empty worktree list

- **GIVEN** a cloned repo with only the main worktree
- **WHEN** the row is rendered
- **THEN** the row SHALL match `{name}<spaces>1 tree  (main)` (singular `tree`, the main entry shown explicitly)

#### Scenario: `--trees` with one repo's `wt list` failing

- **GIVEN** `hop.yaml` lists `outbox` (cloned, wt healthy) and `weirdrepo` (cloned, but its `.git` is corrupt so `wt list --json` exits non-zero or returns malformed JSON)
- **WHEN** I run `hop ls --trees`
- **THEN** stdout still contains both rows, in source order
- **AND** the `weirdrepo` row indicates `(wt list failed: <err>)` — the failure is surfaced inline per-row, not as a fatal abort
- **AND** stderr is empty (per-row failures stay in the table)
- **AND** exit code is 0 — `hop ls --trees` is a listing op; per-row failures degrade gracefully, mirroring the `(not cloned)` convention

### Requirement: `--trees` fails fast when `wt` is missing

When `--trees` is set AND `wt` is not on `PATH`, `hop ls --trees` SHALL print `hop: wt: not found on PATH.` to stderr and exit 1 (reusing the existing `errSilent` pattern from `cmd/hop/open.go`). The check is lazy — `wt` missing is detected on the first `wt list --json` invocation, not preflighted. Subsequent invocations within the same run are skipped (no repeated identical error lines).

#### Scenario: `wt` missing during `--trees`

- **GIVEN** `wt` is not on PATH
- **WHEN** I run `hop ls --trees`
- **THEN** stderr contains exactly one line: `hop: wt: not found on PATH.`
- **AND** stdout may contain the rows for any non-cloned repos enumerated before the first wt invocation, OR may be empty (implementation MAY choose either — both satisfy the contract; deciding for "empty stdout" simplifies the loop)
- **AND** exit code is 1

### Requirement: `--trees` does not affect default `hop ls`

`hop ls` invoked without `--trees` SHALL produce byte-for-byte identical output to today's `hop ls`. No new dependencies, no new subprocess invocations, no behavioral drift on the default code path.

#### Scenario: Default `hop ls` unchanged

- **GIVEN** `wt` is not on PATH
- **WHEN** I run `hop ls`
- **THEN** stdout is the aligned `name<spaces>path` listing as before
- **AND** exit code is 0
- **AND** `wt` is NOT invoked

---

## Match Resolution: `/`-Split Algorithm

### Requirement: Split on first `/` before LHS resolution

`resolveByName` (and by extension `resolveOne`) SHALL inspect its `query` argument for a `/` character before invoking the existing match-or-fzf algorithm. If a `/` is present at any position, the query SHALL be split on the **first** `/`:

- **LHS** (substring before the first `/`) — passed to the existing match-or-fzf algorithm to resolve to a `*repos.Repo`. An empty LHS (`/<wt>`) SHALL be a usage error.
- **RHS** (substring after the first `/`) — the worktree name to resolve within the LHS repo's worktrees.

When no `/` is present, behavior is unchanged.

When `/` is present and the LHS resolution succeeds, the function SHALL invoke `wt list --json` in the resolved repo's main checkout via `internal/proc.RunCapture`, parse the result as `[]WtEntry`, find the entry whose `Name` field equals the RHS (case-sensitive exact match), and return a `*repos.Repo` whose `Path` field is the matched entry's `Path`. All other `Repo` fields (`Name`, `Group`, `URL`, `Dir`) SHALL be preserved from the LHS-resolved repo.

#### Scenario: Multi-`/` query

- **GIVEN** `hop.yaml` lists `outbox` AND wt has a worktree literally named `feat-x/sub` (wt permits `/` in worktree names)
- **WHEN** I run `hop outbox/feat-x/sub where`
- **THEN** the LHS is `outbox` (first `/` split) and the RHS is `feat-x/sub`
- **AND** wt's JSON is searched for an entry with `name: "feat-x/sub"`
- **AND** if found, its path is returned

> **NOTE**: First-`/` split is deliberate — repo names in `hop.yaml` are URL basenames (no `/`), so the first `/` unambiguously separates LHS from RHS, even when wt worktree names contain `/`.

### Requirement: Worktree-name match is exact and case-sensitive

The RHS-to-`wt list --json` match SHALL be exact equality on the `name` field — no substring, no case-folding. This mirrors the case-sensitive group-name match in `resolveTargets` (`resolve.go`).

#### Scenario: Case-sensitive miss

- **GIVEN** a worktree named `Feat-X` exists (uppercase F)
- **WHEN** I run `hop outbox/feat-x where`
- **THEN** the match fails (no entry's `name` equals `"feat-x"`)
- **AND** the no-such-worktree error path fires (see below)

### Requirement: No-such-worktree error

When LHS resolution succeeds but the RHS does not match any `wt list --json` entry's `name`, `resolveByName` SHALL return an error whose displayed form is:

```
hop: worktree '<wt-name>' not found in '<repo-name>'. Try: wt list (in <repo-path>) or hop ls --trees
```

The error SHALL trigger `errSilent` exit semantics (the helper writes the line to the cobra command's stderr and returns `errSilent`, exit code 1).

#### Scenario: Unknown worktree name

- **GIVEN** `hop.yaml` lists `outbox` AND no worktree named `nonexistent` exists in `outbox`
- **WHEN** I run `hop outbox/nonexistent where`
- **THEN** stderr contains: `hop: worktree 'nonexistent' not found in 'outbox'. Try: wt list (in ~/code/sahil87/outbox) or hop ls --trees`
- **AND** stdout is empty
- **AND** exit code is 1

### Requirement: Missing `wt` binary error

When LHS resolution succeeds AND the worktree resolution step needs to invoke `wt list --json` AND `wt` is not on `PATH`, `resolveByName` SHALL return an error whose displayed form is exactly the existing wording from `cmd/hop/open.go`:

```
hop: wt: not found on PATH.
```

with exit code 1 (`errSilent`). No new wording is introduced.

#### Scenario: `wt` missing on PATH during worktree resolution

- **GIVEN** `wt` is not on PATH AND `hop.yaml` lists `outbox`
- **WHEN** I run `hop outbox/feat-x where`
- **THEN** stderr contains exactly: `hop: wt: not found on PATH.`
- **AND** stdout is empty
- **AND** exit code is 1

### Requirement: `wt list --json` failure surfaces as a real error

When `wt list --json` invocation returns a non-zero exit code (other than `ErrNotFound`) OR when its stdout fails to parse as `[]WtEntry`, the failure SHALL be surfaced as a `hop: wt list: <err>` line on stderr with exit code 1. There SHALL NOT be a silent fallback to the main worktree path — an unparseable wt response is a real failure.

#### Scenario: Malformed `wt list --json` output

- **GIVEN** a hypothetical wt version that returns `{not-json}` for `wt list --json`
- **WHEN** I run `hop outbox/feat-x where`
- **THEN** stderr contains a line starting with `hop: wt list:` followed by the unmarshal error
- **AND** stdout is empty
- **AND** exit code is 1

### Requirement: Empty RHS is a usage error

A query with a trailing `/` and no RHS (e.g., `hop outbox/`) SHALL be rejected with a usage error: `hop: empty worktree name after '/'`. Exit code 2. The empty-RHS query SHALL NOT silently fall back to bare-repo resolution — the trailing `/` is almost certainly a typo or a tab-completion artifact.

#### Scenario: Trailing `/`

- **GIVEN** `hop.yaml` lists `outbox`
- **WHEN** I run `hop outbox/ where`
- **THEN** stderr contains: `hop: empty worktree name after '/'`
- **AND** stdout is empty
- **AND** exit code is 2

### Requirement: Empty LHS is a usage error

A query with a leading `/` and no LHS (e.g., `hop /feat-x`) SHALL be rejected with: `hop: empty repo name before '/'`. Exit code 2.

#### Scenario: Leading `/`

- **GIVEN** `hop.yaml` is non-empty
- **WHEN** I run `hop /feat-x where`
- **THEN** stderr contains: `hop: empty repo name before '/'`
- **AND** stdout is empty
- **AND** exit code is 2

### Requirement: Repo-not-cloned short-circuit applies before wt invocation

When the LHS resolves to a registered repo whose `.git` does NOT exist on disk (clone state ≠ `stateAlreadyCloned`), the existing "not cloned" error path SHALL fire BEFORE `wt list --json` is invoked. wt SHALL NOT be invoked against a missing main checkout.

> Note: today's `where` verb does NOT enforce cloned-state — it prints the resolved registry path even when the repo isn't cloned. The cloned-state guard added here applies ONLY to queries that include a `/` suffix (which require an on-disk checkout to invoke `wt list` in). Bare queries (`hop outbox where` with no `/`) retain their existing un-guarded behavior.

#### Scenario: Worktree query against an uncloned repo

- **GIVEN** `hop.yaml` lists `loom` AND `loom` is not cloned
- **WHEN** I run `hop loom/feat-x where`
- **THEN** stderr contains: `hop: 'loom' is not cloned. Try: hop clone loom`
- **AND** `wt list --json` is NOT invoked
- **AND** stdout is empty
- **AND** exit code is 1

#### Scenario: Bare query against an uncloned repo is unchanged

- **GIVEN** the same setup (loom not cloned)
- **WHEN** I run `hop loom where`
- **THEN** stdout is the registry-derived path of loom (e.g., `~/code/sahil87/loom`)
- **AND** exit code is 0
- **AND** behavior is byte-identical to pre-change `hop loom where`

---

## Wrapper Boundaries: `wt list --json` Integration

### Requirement: `wt list --json` invocation routes through `internal/proc`

All `wt list --json` invocations SHALL go through `internal/proc.RunCapture` with an explicit `cmd.Dir` set to the resolved repo's main checkout path. Direct `os/exec` usage outside `internal/proc/` remains prohibited per Constitution Principle I (audit-enforced by `wrapper-boundaries.md`).

#### Scenario: Subprocess invocation contract

- **GIVEN** the implementation of `resolveByName`'s worktree branch
- **WHEN** the source is audited via `grep --include='*.go' --exclude='*_test.go' -rn '"os/exec"' src/internal/ src/cmd/`
- **THEN** the result contains no matches outside `src/internal/proc/`
- **AND** `wt list --json` calls appear only via `proc.RunCapture` or via a small helper that itself calls `proc.RunCapture`

### Requirement: Per-call timeout

Each `wt list --json` invocation SHALL use a `context.WithTimeout` of **5 seconds**, matching the precedent set by `internal/scan` for `git remote` invocations. wt list is a local op (reads `.git/worktrees/`) with no network round-trip.

#### Scenario: Hypothetical hung wt

- **GIVEN** a wt invocation that never returns
- **WHEN** the 5-second timeout elapses
- **THEN** the context is cancelled
- **AND** `RunCapture` returns a context-deadline-exceeded error
- **AND** the caller surfaces it as `hop: wt list: <err>` with exit code 1

### Requirement: `WtEntry` JSON contract

The Go type unmarshalled from `wt list --json` output SHALL match wt's documented schema:

```go
type WtEntry struct {
    Name      string `json:"name"`
    Branch    string `json:"branch"`
    Path      string `json:"path"`
    IsMain    bool   `json:"is_main"`
    IsCurrent bool   `json:"is_current"`
    Dirty     bool   `json:"dirty"`
    Unpushed  int    `json:"unpushed"`
}
```

Unknown JSON fields SHALL be ignored (Go's `encoding/json` default — no `DisallowUnknownFields`) so future wt schema additions don't break hop.

#### Scenario: Future wt adds new fields

- **GIVEN** a hypothetical future wt that adds a new `last_commit` field to its JSON
- **WHEN** hop parses the JSON
- **THEN** the new field is silently ignored
- **AND** the existing fields populate `WtEntry` correctly
- **AND** no error is raised

### Requirement: Wrapper package decision

The `wt list --json` invocation + JSON unmarshal logic SHALL live inline in `src/cmd/hop/resolve.go` (or a sibling file in `cmd/hop/`) — NOT in a new `src/internal/wt/` package — until at least three independent call sites exist. This follows the "promote later" pattern documented in `architecture/wrapper-boundaries.md` (`internal/git/` non-creation precedent). Initial call sites: `resolve.go::resolveByName` (single-worktree resolution) and `ls.go::runLs` (fan-out for `--trees`) = 2 sites. A small shared helper (e.g., `listWorktrees(ctx, repoPath string) ([]WtEntry, error)`) is acceptable inside `cmd/hop/`.

#### Scenario: Audit confirms no `internal/wt/`

- **GIVEN** the change has shipped
- **WHEN** `ls src/internal/` is inspected
- **THEN** no `wt/` directory exists
- **AND** the wt invocation helper lives in `cmd/hop/` (file location TBD in plan)

---

## Shell Integration: Worktree Tab Completion

### Requirement: Completion completes worktree names after `/`

The cobra `ValidArgsFunction` driving completion for the root command's repo-positional slot (`completeRepoNames` in `cmd/hop/repo_completion.go`) SHALL be prefix-aware: when the `toComplete` token contains a `/`, it SHALL be split on the first `/`, the LHS used to identify a configured repo, and `wt list --json` invoked in that repo's main checkout to supply candidate worktree names. Returned candidates SHALL each be prefixed with `<repo>/` (the full token the user is composing) so the shell's prefix-match-on-`toComplete` works as cobra expects.

Completion candidate filtering uses `cobra.ShellCompDirectiveNoFileComp` (matching the existing `completeRepoNames` posture). On any failure (LHS doesn't match a configured repo, `wt list` errors or returns malformed JSON, `wt` is missing), the completion SHALL return `nil` (no candidates) with `NoFileComp` — silently. Completion is best-effort UX; it MUST NOT print to stderr.

When `toComplete` has no `/`, completion behavior is unchanged from today.

#### Scenario: User taps `<TAB>` after `outbox/`

- **GIVEN** the user has typed `hop outbox/` and presses `<TAB>` (under the cobra-generated zsh/bash completion installed via `hop shell-init`)
- **AND** `hop.yaml` lists `outbox` (cloned) with worktrees `main`, `feat-x`, `hotfix`
- **WHEN** completion runs
- **THEN** candidate list is `[outbox/main, outbox/feat-x, outbox/hotfix]` (in `wt list --json` order)
- **AND** the shell's prefix-match filters the list down based on the user's continued typing

#### Scenario: User types `outbox/feat<TAB>`

- **GIVEN** the same setup
- **WHEN** completion runs with `toComplete = "outbox/feat"`
- **THEN** candidate list contains `outbox/feat-x` (and any other worktree whose name starts with `feat`)

#### Scenario: Completion against an uncloned repo

- **GIVEN** `hop.yaml` lists `loom` (not cloned)
- **WHEN** the user types `hop loom/<TAB>`
- **THEN** completion returns no candidates (`nil`, `NoFileComp`)
- **AND** completion does NOT print anything to stderr
- **AND** the shell falls back to no-completion (no spurious errors visible)

#### Scenario: Completion when `wt` is missing on PATH

- **GIVEN** `wt` is not on PATH
- **WHEN** the user types `hop outbox/<TAB>`
- **THEN** completion returns no candidates silently
- **AND** stderr is not polluted with the `not found` line during completion (that line surfaces at command-run time, not tab time)

### Requirement: Completion at `args[1]` slot is unchanged

The verb-position completion (offering `cd`, `where`, `open` after a fully-typed repo name) SHALL be unaffected. The `/`-prefix branch operates only on `args[0]` / the `toComplete` token, not on subsequent positions.

#### Scenario: Verb completion still works after a worktree-aware repo arg

- **GIVEN** the user has typed `hop outbox/feat-x ` (with trailing space) and presses `<TAB>`
- **WHEN** completion runs
- **THEN** the candidate list is `[cd, where, open]` (unchanged from today)

---

## Design Decisions

1. **Split on first `/`, not last.**
   - *Why*: Repo names from `hop.yaml` are URL basenames (no `/`), so the first `/` unambiguously separates LHS from RHS. wt worktree names MAY contain `/` (wt permits it), so splitting on the last `/` would incorrectly reroute multi-`/` worktree names through the wrong repo.
   - *Rejected*: Last-`/` split (breaks multi-`/` worktree names); regex with anchored repo-name (over-engineered).

2. **Inline implementation in `cmd/hop/` over a new `internal/wt/` package.**
   - *Why*: Two initial call sites (`resolve.go` and `ls.go`) is below the "promote-later" threshold documented in `architecture/wrapper-boundaries.md` (e.g., `internal/git/` non-creation despite many call sites). A small `listWorktrees` helper in `cmd/hop/` is enough; extracting to `internal/wt/` adds an indirection without containing logic.
   - *Rejected*: New `internal/wt/` package (premature abstraction).

3. **5-second per-call timeout for `wt list --json`.**
   - *Why*: Matches `internal/scan`'s `git remote` per-call timeout (the nearest precedent). wt list is a local op (no network), so 5 seconds is generous.
   - *Rejected*: 10-minute `cloneTimeout` (way too long for a local op); no timeout (Constitution I requires `CommandContext` everywhere — a context without timeout misses the intent).

4. **Empty RHS (`hop outbox/`) is a usage error, not a silent fallback.**
   - *Why*: Trailing `/` is almost certainly a typo or a tab-completion artifact. Silent fallback hides real errors.
   - *Rejected*: Silent fallback to bare-repo resolution (hides typos).

5. **`hop ls --trees` per-row failures degrade gracefully (no abort).**
   - *Why*: Listing ops should surface what they can. A single corrupt `.git` shouldn't blank the table.
   - *Rejected*: Abort on first per-row failure (over-strict for a discovery verb).

6. **No `internal/wt/` package, no Go-side wt-shim parity.** The Go side stays a thin `proc.RunCapture` caller with JSON unmarshal. Env-var orchestration (`WT_CD_FILE`, `WT_WRAPPER`) stays in the existing `_hop_dispatch open` arm in the shim, not in any new wt wrapper. This change introduces NO new env-var contracts.

7. **Worktree-name match is exact, case-sensitive.**
   - *Why*: Mirrors the case-sensitive group-name match in `resolveTargets`. wt names are user-curated and intentional; case-insensitive substring would re-introduce ambiguity wt itself avoids.
   - *Rejected*: Case-insensitive substring (re-introduces ambiguity).

8. **Repo-not-cloned guard applies ONLY to worktree-suffixed queries.**
   - *Why*: Today's `hop <name> where` is permissive — it prints the resolved registry path even for uncloned repos (useful as "where *would* this land?"). Worktree-suffixed queries can't degrade that way (wt can't list worktrees of a non-existent checkout), so they need the guard, but bare queries should keep their existing permissive behavior.
   - *Rejected*: Universal cloned-state guard (would break existing `hop <name> where` semantics).

---

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Grammar uses `/<wt-name>` suffix on the repo positional | Confirmed from intake #1 — `/` is safe because hop.yaml repo names have no `/`; alternatives explicitly rejected in intake | S:95 R:60 A:90 D:90 |
| 2 | Certain | Resolution implemented in `resolveByName` (and through it, `resolveOne`) so every verb inherits | Confirmed from intake #2 — single seam; DRY; matches existing wrapper pattern | S:95 R:70 A:95 D:95 |
| 3 | Certain | `wt list --json` is the only wt subcommand hop invokes from Go (besides existing `wt open`) | Confirmed from intake #3 — wt's documented machine-readable interface; Constitution IV | S:95 R:85 A:95 D:95 |
| 4 | Certain | `hop ls --trees` is a flag, not a new top-level subcommand | Confirmed from intake #4 — Constitution VI; folding into `ls` is the right home | S:95 R:80 A:95 D:95 |
| 5 | Certain | hop.yaml gains NO worktree fields | Confirmed from intake #5 — Constitution II (No Database) | S:95 R:30 A:95 D:95 |
| 6 | Certain | Missing wt reuses the existing `hop: wt: not found on PATH.` message verbatim | Confirmed from intake #6 — consistent with `open.go`; no new wording | S:95 R:90 A:95 D:95 |
| 7 | Certain | Cross-tree batch git ops are out of scope (encoded in Non-Goals) | Confirmed from intake #7 — discussed and rejected; footgun-shaped | S:95 R:80 A:90 D:95 |
| 8 | Certain | `hop clone --wt-init` is out of scope (encoded in Non-Goals) | Confirmed from intake #8 — discussed and rejected; coupling concern | S:95 R:80 A:90 D:95 |
| 9 | Certain | Shim completion (zsh + bash) ships with v1 | Confirmed from intake #9 — required for syntax discoverability | S:95 R:65 A:90 D:90 |
| 10 | Certain | `hop <name>/main` resolves to main worktree via wt's JSON contract (no special-case in hop) | Confirmed from intake #10 — wt's `is_main: true` entry naturally has the main path | S:90 R:85 A:90 D:95 |
| 11 | Certain | No-such-worktree error wording: `hop: worktree '<wt-name>' not found in '<repo-name>'. Try: wt list (in <repo-path>) or hop ls --trees` | Confirmed from intake #11 — mirrors hop's existing error-with-hint pattern | S:90 R:90 A:90 D:90 |
| 12 | Certain | Worktree-name match is exact (case-sensitive) | Confirmed from intake #12 — parallels `resolveTargets` group-name match | S:90 R:80 A:85 D:90 |
| 13 | Certain | `wt list --json` invocations route through `internal/proc.RunCapture` | Confirmed from intake #13 — Constitution I; not a choice | S:95 R:90 A:95 D:95 |
| 14 | Certain | Empty RHS (`hop outbox/`) is a usage error (exit 2) | Confirmed from intake #14 — surface typos, don't hide them | S:90 R:80 A:85 D:90 |
| 15 | Certain | Empty LHS (`hop /feat-x`) is also a usage error (exit 2) | New in spec — symmetric to empty RHS; same rationale | S:90 R:80 A:85 D:90 |
| 16 | Certain | First-`/` split (not last) — survives multi-`/` worktree names | New in spec — wt permits `/` in names; first-split is the correct disambiguation under "repo names have no `/`" | S:90 R:75 A:90 D:90 |
| 17 | Certain | Split on `/` happens INSIDE `resolveByName`, not in callers — every verb inherits via the existing seam | Upgraded from intake #2 — concretized to the function name | S:95 R:75 A:95 D:95 |
| 18 | Certain | Cloned-state guard applies only to `/`-suffixed queries; bare queries keep existing permissive behavior | New in spec — preserves backward compat for `hop <name> where` on uncloned repos (today permissive); only the worktree branch needs the guard | S:90 R:75 A:90 D:90 |
| 19 | Certain | `wt list --json` failure (non-zero exit, malformed JSON) is surfaced as `hop: wt list: <err>` with exit 1 — no silent fallback | Confirmed from intake — unparseable wt response is a real failure | S:90 R:85 A:90 D:90 |
| 20 | Certain | `wt list --json` per-call timeout = 5 seconds, matching `internal/scan`'s precedent | Upgraded from intake #18 (was Confident) — settled on the in-codebase precedent | S:90 R:85 A:90 D:90 |
| 21 | Certain | Worktree-resolution helper lives in `cmd/hop/` (inline), not in a new `internal/wt/` package | Upgraded from intake #16 (was Confident) — promote-later pattern documented in `wrapper-boundaries.md`; two call sites is below the threshold | S:90 R:80 A:90 D:90 |
| 22 | Certain | Completion failures during `<repo>/<TAB>` are silent (return nil candidates, no stderr) | New in spec — completion is best-effort; stderr noise during TAB is a UX bug | S:90 R:85 A:85 D:90 |
| 23 | Certain | `WtEntry` JSON unmarshal ignores unknown fields (Go default — no `DisallowUnknownFields`) | New in spec — future-proofs against wt schema additions | S:90 R:85 A:90 D:90 |
| 24 | Certain | `--trees` per-row failures degrade gracefully (inline `(wt list failed: <err>)`), do not abort | New in spec — listing ops should surface what they can | S:85 R:80 A:85 D:85 |
| 25 | Certain | `hop ls` without `--trees` is byte-for-byte unchanged | New in spec — locks in backward compat for the default code path | S:95 R:95 A:95 D:95 |
| 26 | Certain | `hop ls --trees` does NOT accept a positional in v1 (out-of-scope per Non-Goals) | Confirmed from intake #15 (was Confident) — settled as a Non-Goal | S:85 R:80 A:85 D:85 |
| 27 | Confident | `hop ls --trees` row format: `{name}<spaces>{N} tree(s)  ({wt-list})` with `*` for dirty and `↑N` for unpushed | New in spec — concrete shape chosen from intake's "tentative" #17; mirrors existing `hop ls` alignment and uses unambiguous compact glyphs. Spec-stage refinement of intake's tentative format | S:75 R:75 A:80 D:75 |
| 28 | Confident | `--trees` does not show `is_current` (per-repo listing has no ambient current — current is shell-cwd-dependent and out of scope for a static fan-out) | New in spec — keeps the per-row format static and reproducible; avoids the "where am I right now?" semantic question that depends on shell cwd | S:75 R:80 A:80 D:75 |
| 29 | Confident | Worktree completion candidates are prefixed with `<repo>/` (the full token being composed) | New in spec — required by cobra's prefix-match-on-toComplete contract; without the prefix the shell would replace `outbox/feat` with bare `feat-x` | S:80 R:80 A:85 D:80 |
| 30 | Confident | Worktree resolution helper signature: `listWorktrees(ctx context.Context, repoPath string) ([]WtEntry, error)` in `cmd/hop/` | New in spec — small, testable, matches `internal/scan` style; final filename TBD in plan | S:75 R:75 A:80 D:80 |

30 assumptions (26 certain, 4 confident, 0 tentative, 0 unresolved).
