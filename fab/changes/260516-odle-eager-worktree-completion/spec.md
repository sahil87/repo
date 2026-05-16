# Spec: Eager Worktree-Aware Tab Completion

**Change**: 260516-odle-eager-worktree-completion
**Created**: 2026-05-16
**Affected memory**: `docs/memory/cli/subcommands.md`, `docs/memory/cli/match-resolution.md`

## Non-Goals

- The `<repo>/<partial>` post-slash branch (`completeWorktreeCandidates`) is NOT changed — its behavior, candidate shape, and directive set remain exactly as shipped in change `7eab`.
- Tab completion for non-root commands (e.g., `hop clone <name>`, `hop pull <name>`, `hop sync <name>`) is NOT changed — only the root command's `$1` slot gains the eager branch.
- No caching of `wt list --json` results across Tab presses — every eager-fire invocation runs `wt list --json` fresh.
- No new flags, no new env vars, no new subcommands, no new packages.
- No changes to the `listWorktrees` seam or to `wt_list.go` — the seam is reused as-is for test injection.

## CLI: Repo Positional Tab Completion (Root Command)

### Requirement: Pre-Slash Eager Worktree Expansion

When the root command's `$1` positional is completing a token that does NOT contain `/`, `completeRepoNames` SHALL eagerly expand the candidate list to include worktree-suffixed forms (`<repo>/<wt>`) IF AND ONLY IF all of the following hold simultaneously:

1. `rs.MatchOne(toComplete)` returns exactly one repo (`len(matches) == 1`).
2. That repo's `Name` is NOT in the `subNames` collision set (i.e., it survives the subcommand-collision filter).
3. `cloneState(repo.Path)` returns `stateAlreadyCloned`.
4. `listWorktrees(context.Background(), repo.Path)` returns a slice with `len(entries) >= 2` AND no error.

When all four conditions hold, the function SHALL return a candidate slice composed of:

- Position 0: the bare repo name (`repo.Name`).
- Positions 1..N: `repo.Name + "/" + entries[i].Name` for each `i` in `0..len(entries)-1`, preserving `wt list --json` source order verbatim.

The returned directive on eager-fire SHALL be `cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace` (bitwise OR).

#### Scenario: Unique match with multiple worktrees fires eager expansion

- **GIVEN** `hop.yaml` lists a repo `outbox`, cloned at `$HOME/code/outbox`
- **AND** `wt list --json` in `$HOME/code/outbox` returns three entries in order `[main, feat-x, bugfix-y]`
- **AND** `outbox` does not collide with any cobra subcommand name
- **WHEN** the user invokes tab completion with `toComplete = "outb"` at the root command's `$1` slot
- **THEN** `completeRepoNames` returns candidates `["outbox", "outbox/main", "outbox/feat-x", "outbox/bugfix-y"]`
- **AND** the returned directive is `cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace`

#### Scenario: Unique match with only the main worktree does NOT fire eager expansion

- **GIVEN** `hop.yaml` lists a repo `outbox`, cloned
- **AND** `wt list --json` returns exactly one entry (the main worktree)
- **WHEN** the user invokes tab completion with `toComplete = "outb"`
- **THEN** `completeRepoNames` returns the original `names` list (today's behavior — `["outbox"]` after the collision filter)
- **AND** the returned directive is `cobra.ShellCompDirectiveNoFileComp` (no `NoSpace`)

#### Scenario: Bare repo at position 0, worktrees in wt source order

- **GIVEN** the eager-expansion preconditions hold for repo `outbox`
- **AND** `wt list --json` returns entries in the order `[main, feat-x, bugfix-y]`
- **WHEN** eager expansion fires
- **THEN** the candidate at index 0 SHALL be the bare repo name (`"outbox"`)
- **AND** the candidates at indices 1..N SHALL be `["outbox/main", "outbox/feat-x", "outbox/bugfix-y"]` in that exact order
- **AND** no alphabetical or other reordering SHALL be applied

### Requirement: Subcommand-Collision Filter Runs Before Eager Check

The subcommand-collision filter (today's behavior — repos whose `Name` matches a cobra subcommand are excluded from `names`) SHALL run BEFORE the eager-expansion check. If the unique `MatchOne` result is a repo whose `Name` collides with a subcommand, the function SHALL return the post-filter `names` list (which excludes the collided repo) with today's `cobra.ShellCompDirectiveNoFileComp` directive, and SHALL NOT invoke `cloneState` or `listWorktrees`.

#### Scenario: Unique match collides with a subcommand

- **GIVEN** `hop.yaml` lists a repo literally named `clone` (collides with the `hop clone` subcommand)
- **AND** `MatchOne("clo")` returns exactly that single repo
- **WHEN** the user invokes tab completion with `toComplete = "clo"`
- **THEN** the eager-expansion check is skipped
- **AND** `completeRepoNames` returns the post-filter `names` list (`clone` excluded) with directive `cobra.ShellCompDirectiveNoFileComp`
- **AND** `cloneState` and `listWorktrees` are NOT invoked for the collided repo

### Requirement: Silent Fallback on Every Failure Mode

The eager-expansion branch SHALL NOT write to stderr, stdout, or any user-visible surface under any failure condition. Every failure mode SHALL silently fall back to today's behavior: return the post-filter `names` list with directive `cobra.ShellCompDirectiveNoFileComp` (no `NoSpace`). Failure modes include but are not limited to:

- `cloneState` returns any state other than `stateAlreadyCloned` (e.g., `stateMissing`, `statePathExistsNotGit`).
- `cloneState` returns a non-nil error.
- `listWorktrees` returns a non-nil error (including `proc.ErrNotFound` for missing `wt`, malformed JSON, non-zero exit, context timeout).
- `listWorktrees` returns fewer than 2 entries.

#### Scenario: Unique match, repo not cloned

- **GIVEN** `hop.yaml` lists a repo `outbox` whose path is NOT cloned (e.g., no `.git` directory at `repo.Path`)
- **AND** `MatchOne("outb")` returns exactly `outbox`
- **WHEN** the user invokes tab completion with `toComplete = "outb"`
- **THEN** `completeRepoNames` returns `["outbox"]` with directive `cobra.ShellCompDirectiveNoFileComp`
- **AND** no stderr writes occur
- **AND** `listWorktrees` is NOT invoked

#### Scenario: Unique match, `wt list --json` returns an error

- **GIVEN** `hop.yaml` lists a repo `outbox`, cloned
- **AND** the `listWorktrees` seam returns a non-nil error (e.g., `wt` missing, malformed JSON, non-zero exit)
- **WHEN** the user invokes tab completion with `toComplete = "outb"`
- **THEN** `completeRepoNames` returns `["outbox"]` with directive `cobra.ShellCompDirectiveNoFileComp`
- **AND** no stderr writes occur

### Requirement: Ambiguous Prefix Bypasses Eager Branch Entirely

When `rs.MatchOne(toComplete)` returns 0 or 2+ matches, the function SHALL return today's behavior verbatim: the post-filter `names` list with directive `cobra.ShellCompDirectiveNoFileComp`. The eager branch SHALL NOT invoke `cloneState` or `listWorktrees` in this case. This is the cost gate that prevents `wt list --json` fan-out across N repos during an ambiguous Tab press.

#### Scenario: Ambiguous prefix returns full list unchanged

- **GIVEN** `hop.yaml` lists multiple repos whose names start with `co` (e.g., `code`, `colors`, `coredns`)
- **WHEN** the user invokes tab completion with `toComplete = "co"`
- **THEN** `completeRepoNames` returns the full `names` list (all non-collided repos, in source order) with directive `cobra.ShellCompDirectiveNoFileComp`
- **AND** `cloneState` and `listWorktrees` are NOT invoked

#### Scenario: Empty `toComplete` falls into the ambiguous-prefix case

- **GIVEN** `hop.yaml` lists 2+ repos
- **WHEN** the user invokes tab completion with `toComplete = ""` (no characters typed yet)
- **THEN** `MatchOne("")` returns all repos
- **AND** `len(matches) != 1`, so the eager branch is skipped
- **AND** `completeRepoNames` returns the full post-filter `names` list with directive `cobra.ShellCompDirectiveNoFileComp`

### Requirement: Post-Slash Branch Preserved

When `toComplete` contains `/`, control SHALL transfer to `completeWorktreeCandidates` exactly as today. The eager pre-slash branch SHALL NOT alter, wrap, or otherwise affect the post-slash branch's behavior, candidate shape, or directive set.

#### Scenario: Slash-containing toComplete dispatches to post-slash branch unchanged

- **GIVEN** the user has typed `outbox/feat`
- **WHEN** `completeRepoNames` is invoked
- **THEN** control transfers to `completeWorktreeCandidates(toComplete)` as today
- **AND** the eager-expansion code path is not entered
- **AND** the returned candidates and directive are exactly what `completeWorktreeCandidates` would have returned in pre-change behavior

## Tests: Coverage for Eager-Expansion Branch

### Requirement: Table-Driven Tests for All Five Cases

`src/cmd/hop/repo_completion_test.go` SHALL include table-driven tests covering the five eager-branch decision cases below. Tests SHALL use the existing `listWorktrees` package-level `var` seam from `wt_list.go` for fake injection (no real `wt` subprocess spawned). Tests SHALL construct fixture repos using the same `.git` directory pattern already used by `repo_completion_test.go`'s existing tests.

| Case | toComplete | Repo state | Expected candidates | Expected directive |
|---|---|---|---|---|
| (a) Unique match, 1 worktree | `"outb"` matches `outbox` | cloned; `listWorktrees` returns 1 entry | `["outbox"]` | `ShellCompDirectiveNoFileComp` (no `NoSpace`) |
| (b) Unique match, >=2 worktrees | `"outb"` matches `outbox` | cloned; `listWorktrees` returns `[main, feat-x, bugfix-y]` | `["outbox", "outbox/main", "outbox/feat-x", "outbox/bugfix-y"]` | `ShellCompDirectiveNoFileComp \| ShellCompDirectiveNoSpace` |
| (c) Unique match, uncloned | `"outb"` matches `outbox` | `cloneState` returns `stateMissing` | `["outbox"]` | `ShellCompDirectiveNoFileComp` (no `NoSpace`) |
| (d) Unique match, `listWorktrees` errors | `"outb"` matches `outbox` | cloned; `listWorktrees` seam returns error | `["outbox"]` | `ShellCompDirectiveNoFileComp` (no `NoSpace`) |
| (e) Ambiguous prefix (2+ matches) | `"co"` matches multiple | — | full `names` list (today's behavior) | `ShellCompDirectiveNoFileComp` (no `NoSpace`) |

Existing tests for the post-slash branch (`completeWorktreeCandidates`) and the ambiguous-prefix and subcommand-collision branches SHALL remain unchanged and continue to pass.

#### Scenario: Test (b) verifies eager-fire on multi-worktree repo

- **GIVEN** a fixture configuration with one repo `outbox` whose `.git` directory exists
- **AND** the `listWorktrees` seam is overridden to return `[{Name: "main"}, {Name: "feat-x"}, {Name: "bugfix-y"}]` with no error
- **WHEN** `completeRepoNames(rootCmd, []string{}, "outb")` is invoked
- **THEN** the returned candidates equal `["outbox", "outbox/main", "outbox/feat-x", "outbox/bugfix-y"]` in exactly that order
- **AND** the returned directive equals `cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace`

#### Scenario: Test (d) verifies silent fallback on listWorktrees error

- **GIVEN** a fixture configuration with one repo `outbox` whose `.git` directory exists
- **AND** the `listWorktrees` seam is overridden to return `(nil, errors.New("wt list: malformed JSON"))`
- **WHEN** `completeRepoNames(rootCmd, []string{}, "outb")` is invoked
- **THEN** the returned candidates equal `["outbox"]`
- **AND** the returned directive equals `cobra.ShellCompDirectiveNoFileComp` (no `NoSpace`)
- **AND** no stderr output is produced

### Requirement: Tests Must Conform to Spec (Test Integrity)

Per the project Constitution's Test Integrity rule, the five test cases above SHALL verify the requirements stated in this spec. If a test and the implementation disagree, the resolution SHALL be either (a) update the test to match the spec, or (b) update the implementation to match the spec — NEVER alter the implementation solely to accommodate test fixtures.

## Memory Documentation Updates

### Requirement: `cli/subcommands.md` Documents the Eager Pre-Slash Expansion

`docs/memory/cli/subcommands.md` SHALL be modified during hydrate to extend the `hop shell-init` tab-completion subsection (or the section currently documenting the post-slash worktree branch) with a paragraph describing the eager pre-slash expansion. The paragraph SHALL cover:

- The trigger condition (`MatchOne(toComplete)` returns exactly one cloned repo with >=2 worktree entries, after the subcommand-collision filter).
- The candidate shape (bare `<repo>` at position 0, followed by `<repo>/<wt>` entries in `wt list --json` source order).
- The directive on eager-fire (`ShellCompDirectiveNoFileComp | ShellCompDirectiveNoSpace`).
- The silent-fallback contract for every failure mode (no stderr writes from completion).

#### Scenario: Hydrate updates subcommands.md

- **GIVEN** the change reaches the hydrate stage with the implementation merged
- **WHEN** the hydrate agent processes `cli/subcommands.md`
- **THEN** the file contains a paragraph (under the `hop shell-init` section or an adjacent tab-completion section) describing the four bullets above
- **AND** the paragraph cross-references `cli/match-resolution.md` for the shared `listWorktrees` seam

### Requirement: `cli/match-resolution.md` Notes the Shared `listWorktrees` Seam

`docs/memory/cli/match-resolution.md` SHALL be modified during hydrate to note that the `listWorktrees` package-level seam in `wt_list.go` now serves two tab-completion entry points: the post-slash `completeWorktreeCandidates` (existing) and the new pre-slash eager branch in `completeRepoNames`. The note SHALL cross-link to `cli/subcommands.md`.

#### Scenario: Hydrate updates match-resolution.md

- **GIVEN** the change reaches the hydrate stage with the implementation merged
- **WHEN** the hydrate agent processes `cli/match-resolution.md`
- **THEN** the file contains a note (near the existing "Worktree error paths" table or in a new "Tab Completion" subsection at the end) explaining that `listWorktrees` now drives two completion call sites
- **AND** the note cross-links to `cli/subcommands.md`

## Design Decisions

1. **Eager expansion fires only when `len(matches) == 1` (unique-match guard)**
   - *Why*: Ambiguous prefixes must remain cheap — running `wt list --json` across N repos on every ambiguous Tab press would impose unbounded latency. The unique-match guard is the cost gate.
   - *Rejected*: Run `wt list --json` for every candidate when prefix is ambiguous (unbounded latency); always expand on first Tab regardless of worktree count (1-worktree repos pay an extra Space keystroke for no discoverability gain).

2. **Eager expansion fires only when `len(entries) >= 2`**
   - *Why*: The menu should only appear when worktree-vs-main is a real choice. A 1-worktree repo has nothing to disambiguate, so paying the `NoSpace` keystroke cost there would be pure regression.
   - *Rejected*: Always fire when cloned (penalizes the common case); threshold at `>=1` (same effect as always-fire).

3. **`NoSpace` directive on eager-fire only, not on every branch**
   - *Why*: Without `NoSpace`, the shell auto-finalizes the argument after the user picks bare `<repo>`, defeating the menu. `NoSpace` is bitwise-OR'd onto today's `NoFileComp` only on the eager-fire path so non-eager paths preserve today's auto-space behavior.
   - *Rejected*: Always emit `NoSpace` (regresses the 1-worktree-repo bare-cd case to require an extra Space everywhere).

4. **Candidate ordering: bare `<repo>` first, then `wt list --json` source order**
   - *Why*: The bare form is the intuitive default — Enter-on-first-match still means "use main checkout." Source order (typically main first, then by creation date) matches what `hop ls --trees` displays.
   - *Rejected*: Alphabetical ordering (inconsistent with the rest of hop's listing behavior); worktree first, bare last (surprises users who expect bare = default).

5. **Subcommand-collision filter runs BEFORE the eager check**
   - *Why*: Cobra dispatches collide-names to the subcommand before the bare-form resolver ever sees them; offering `<collided-name>/<wt>` candidates would be misleading because the user can never type that token without hitting the subcommand. Filter-first ordering is also consistent with the post-slash branch (which does not bypass the filter at the LHS-resolution step).
   - *Rejected*: Check eager-expansion before the filter (would surface unreachable candidates).

6. **Reuse the existing `listWorktrees` package-level seam**
   - *Why*: Constitution IV (Wrap, Don't Reinvent) plus the seam was deliberately designed for this kind of extension. Adding a third call site does not yet justify promotion to `internal/wt/` because the need is identical (single-shot `wt list --json` call with a 5-second timeout).
   - *Rejected*: New seam (premature abstraction); promote to `internal/wt/` (threshold not yet met — see `architecture/wrapper-boundaries.md`).

7. **No caching of `wt list --json` results across Tab presses**
   - *Why*: Constitution II (No Database) and the measured latency (6.3-8.1ms median) is well below the perception threshold. Caching would require a cache file or in-process state across invocations (the binary is short-lived per Tab press), which contradicts both Constitution II and the existing precedent of `completeWorktreeCandidates` (which also re-invokes per Tab press).
   - *Rejected*: Add a TTL-based cache (violates No Database, adds complexity for negligible UX gain).

## Assumptions

<!-- Scoring source: this table is read by `fab score`. -->

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Eager expansion fires ONLY when `len(matches) == 1`. Ambiguous prefixes keep today's behavior verbatim, with no fan-out across N repos. | Confirmed from intake #1. Constitution-aligned cost gate; design discussion explicit. | S:95 R:90 A:95 D:95 |
| 2 | Certain | Eager expansion fires ONLY when `cloneState(repo.Path) == stateAlreadyCloned`. Uncloned repos silently fall back. | Confirmed from intake #2. Matches `completeWorktreeCandidates`'s precedent — uncloned repos have no `.git` to query. | S:95 R:90 A:95 D:90 |
| 3 | Certain | Eager expansion fires ONLY when `listWorktrees` returns `len(entries) >= 2`. 1-worktree repos keep today's behavior (trailing space, finalize $1). | Confirmed from intake #3. The >=2 threshold means the menu only appears when worktree-vs-main is a real choice. | S:90 R:90 A:90 D:95 |
| 4 | Certain | Every failure mode (uncloned, `wt` missing, `wt list --json` non-zero exit, malformed JSON, context timeout) silently falls back to today's behavior. No stderr writes from completion. | Confirmed from intake #4. Existing contract enforced by the file's package-level comment block and by `completeWorktreeCandidates`'s precedent. | S:95 R:95 A:95 D:95 |
| 5 | Certain | The post-slash branch (`completeWorktreeCandidates`, triggered by `strings.Contains(toComplete, "/")`) is preserved unchanged. The eager branch is additive, not a replacement. | Confirmed from intake #5. Surgical change; existing tests for the post-slash branch must keep passing. | S:95 R:95 A:95 D:95 |
| 6 | Certain | Reuse the existing `listWorktrees` package-level seam from `wt_list.go`. No new wrapper, no new package, no promotion to `internal/wt/`. | Confirmed from intake #6. Constitution IV; seam was deliberately designed for this kind of extension. Third call site does not yet trigger the "3+ with divergent needs" promotion rule. | S:90 R:85 A:95 D:90 |
| 7 | Certain | Change type is `feat` (user-visible Tab outcome changes shape — menu instead of finalize). | Confirmed from intake #7. Pre-intake discussion explicitly landed on `feat` over `refactor`. | S:90 R:90 A:90 D:75 |
| 8 | Certain | Candidate ordering: bare `<repo>` at position 0, then `<repo>/<wt>` entries in `wt list --json` source order. No alphabetical reordering. | Confirmed from intake #8. Matches `hop ls --trees` ordering and the rest of hop's listing behavior. | S:95 R:75 A:80 D:70 |
| 9 | Certain | Directive on eager-fire: `ShellCompDirectiveNoFileComp \| ShellCompDirectiveNoSpace` (bitwise OR). All other branches: today's `ShellCompDirectiveNoFileComp` only. | Confirmed from intake #9. `NoSpace` is essential for the menu to remain interactive after the user selects bare `<repo>`. | S:95 R:90 A:85 D:80 |
| 10 | Certain | Bash users (and zsh users without `menu select`) get a flat candidate list rather than arrow-key navigation. Acceptable degradation. | Confirmed from intake #10. Candidates are still surfaced; further disambiguation by typing more characters. | S:95 R:80 A:75 D:70 |
| 11 | Certain | Bare-cd cost: against a multi-worktree repo, `h outbox<TAB>` no longer auto-spaces — user pays one extra Space keystroke to commit "use main." Accepted as the price of discoverability. | Confirmed from intake #11. The discoverability win for worktrees outweighs the keystroke cost for the bare-cd case. | S:95 R:80 A:70 D:75 |
| 12 | Certain | No caching of `wt list --json` results across Tab presses. Each eager-fire invocation runs `wt list --json` once. | Confirmed from intake #12. Constitution II (No Database); measured latency well below perception threshold. | S:95 R:75 A:80 D:75 |
| 13 | Certain | Tests use the existing `listWorktrees` seam for injection. `cloneState` is invoked at real `repo.Path`; fixtures use the same `.git` directory pattern as existing `repo_completion_test.go` tests. | Confirmed from intake #13. Reuses established test plumbing; no new test helpers. | S:95 R:75 A:80 D:75 |
| 14 | Certain | Empty `toComplete` falls into the ambiguous-prefix case naturally. `MatchOne("")` returns all repos; `len != 1`; eager branch skipped. No special-case code. | Confirmed from intake #14. The unique-match guard structurally handles the empty case. | S:95 R:90 A:85 D:80 |
| 15 | Certain | Subcommand-collision filter runs BEFORE the eager-expansion check. A repo whose `Name` collides with a cobra subcommand is filtered out of `names` first; the eager check operates only on non-collided candidates. A spec-level test pins this ordering. | Confirmed from intake #15. Matches the post-slash branch's behavior and the user-facing reality that cobra dispatches collide-names to the subcommand before the bare-form resolver. | S:95 R:75 A:75 D:70 |

15 assumptions (15 certain, 0 confident, 0 tentative, 0 unresolved).
