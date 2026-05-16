# Intake: Eager Worktree-Aware Tab Completion

**Change**: 260516-odle-eager-worktree-completion
**Created**: 2026-05-16
**Status**: Draft

## Origin

> Eager worktree-aware tab completion for the repo positional. Today the shim's tab completion (cobra-generated, backed by `src/cmd/hop/repo_completion.go::completeRepoNames`) surfaces worktree candidates ONLY when the user has already typed `/` (the `<repo>/<partial>` branch added by change `7eab`). For the pre-slash case — `h outb<TAB>` — a unique repo match completes to `h outbox ` with a trailing space, signalling "argument is done." If the user actually wanted a worktree, they must backspace, type `/`, and Tab again.
>
> The improvement: when `h <prefix><TAB>` uniquely resolves to a cloned repo AND that repo has ≥2 worktrees, return BOTH the bare repo name AND every `<repo>/<wt-name>` candidate, using `cobra.ShellCompDirectiveNoSpace` so the shell does not auto-finalize the argument.

**Mode**: One-shot / design-discussion handoff. A substantive design discussion preceded this intake (latency measurements, candidate ordering, bash-vs-zsh degradation, cache scope). Decisions captured in the Assumptions table below — none were left for the user to answer.

**Builds on**: change `7eab` (`260516-7eab-worktree-aware-path-resolution`) — the same `completeWorktreeCandidates` helper and the `listWorktrees` seam established for the post-slash branch are reused unchanged.

## Why

1. **Pain point**: Today's completion is **asymmetric** with respect to worktree discovery. A user who knows their repo has worktrees must (a) Tab-complete to the bare name, (b) notice the trailing space committed `$1`, (c) backspace one character, (d) type `/`, (e) Tab again to see worktree candidates. The slash-trigger is a hidden affordance — discoverable only by reading docs or experimenting. New users won't find it.

2. **Consequence of doing nothing**: Change `7eab` shipped the `<repo>/<wt>` grammar but the tab-completion ergonomics surface it only to users who already know it exists. The investment in grammar pays no compounding dividend on every Tab press. Worktrees remain a power-user feature instead of an everyday one.

3. **Why this approach** (eager expansion + `NoSpace`) **over alternatives**:
   - **Status quo**: rejected — hidden affordance, see above.
   - **Always expand on first Tab regardless of worktree count**: rejected — a 1-worktree repo would force the user to press Space to commit "main," paying a keystroke cost for no discoverability gain. The ≥2 threshold means only repos where worktree-vs-main is a real choice trigger the menu.
   - **Defer to a flag (e.g., `--complete-worktrees`)**: rejected — Constitution III (convention over configuration). The right default is to surface what exists.
   - **New top-level subcommand (e.g., `hop wt-complete`)**: rejected — Constitution VI (minimal surface area). Completion is an existing surface; extend it, don't add to it.

4. **Why the latency is acceptable**: spot-checked across `~/code/sahil87/` repos — median 6.3–8.1ms for `wt list --json`, essentially flat from 1 to 48 worktrees (run-kit, worst case). Eager invocation on every Tab over a unique repo prefix is fine. The post-slash path already pays this cost; the new pre-slash path is symmetric.

## What Changes

### Surface 1: `src/cmd/hop/repo_completion.go::completeRepoNames`

Extend the existing `completeRepoNames` function with a pre-slash eager-expansion branch. The post-slash branch (`strings.Contains(toComplete, "/")` → `completeWorktreeCandidates`) is preserved unchanged.

**Current shape** (post-load, post-subcommand-filter, names collected from `rs`):

```go
names := make([]string, 0, len(rs))
for _, r := range rs {
    if _, collides := subNames[r.Name]; collides {
        continue
    }
    names = append(names, r.Name)
}
return names, cobra.ShellCompDirectiveNoFileComp
```

**New shape**: after collecting candidate `names`, attempt eager expansion when the **unique-match** condition holds:

1. Filter `rs` via `rs.MatchOne(toComplete)` (the same case-insensitive substring matcher used by `resolveByName`). The pre-slash branch runs even when `toComplete == ""`; in that case `MatchOne` returns the full list, len != 1, and eager expansion is skipped.
2. If `len(matches) != 1` → return the original `names` list with `ShellCompDirectiveNoFileComp` (today's behavior).
3. If exactly one match AND the matched repo's `Name` is in `subNames` (subcommand collision) → return `names` (today's behavior — collision filter still wins; the bare-form resolver never sees collide names).
4. Otherwise (exactly one match, not a collision):
   - `cloneState(repo.Path)` → if not `stateAlreadyCloned`, return `names` (today's behavior).
   - `listWorktrees(context.Background(), repo.Path)` → on any error, return `names` (silent fallback, matches `completeWorktreeCandidates` precedent).
   - If `len(entries) < 2` → return `names` (today's behavior — repo has only the main worktree).
   - **Eager-expansion fires**: build a new candidate list `[repo.Name, repo.Name+"/"+entries[0].Name, repo.Name+"/"+entries[1].Name, ...]` and return with `cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace`.

**Candidate ordering inside the eager branch**:
- Position 0: bare `<repo>` (intuitive default — Enter-on-first-match still means "use main checkout").
- Positions 1..N: `<repo>/<wt-name>` in `wt list --json` source order (wt's own ordering — typically main first, then by creation date). This matches what `hop ls --trees` displays. Alphabetical was considered and rejected; source-order is consistent with the rest of hop's listing behavior.

**Directive flags**:
- `ShellCompDirectiveNoFileComp` — preserved from today's behavior (suppress shell's default filename completion when no match).
- `ShellCompDirectiveNoSpace` — NEW, added ONLY on the eager-fire path. The bitwise OR composes correctly (`a | b`). Without `NoSpace`, the shell auto-finalizes the argument after the user picks bare `<repo>`, defeating the menu.

**Silent-fallback contract**: every failure mode in the eager branch returns the original `names` list with today's directive. The function MUST NOT write to stderr (existing contract, enforced by the file's package-level comment block). All errors silently degrade to today's behavior.

### Surface 2: `src/cmd/hop/repo_completion_test.go`

New table-driven tests covering five cases. Existing tests for the post-slash branch and the ambiguous-prefix branch stay unchanged.

| Case | toComplete | Repo state | Expected candidates | Expected directive |
|---|---|---|---|---|
| (a) Unique match, 1 worktree | `"outb"` (matches `outbox`) | cloned, `wt list --json` returns 1 entry (main only) | `["outbox"]` | `ShellCompDirectiveNoFileComp` (no NoSpace) |
| (b) Unique match, ≥2 worktrees | `"outb"` | cloned, `wt list --json` returns `[main, feat-x, bugfix-y]` | `["outbox", "outbox/main", "outbox/feat-x", "outbox/bugfix-y"]` | `ShellCompDirectiveNoFileComp \| ShellCompDirectiveNoSpace` |
| (c) Unique match, uncloned | `"outb"` | `cloneState` returns `stateMissing` | `["outbox"]` | `ShellCompDirectiveNoFileComp` (no NoSpace) — silent fallback |
| (d) Unique match, `wt list --json` errors | `"outb"` | cloned, `listWorktrees` seam returns error | `["outbox"]` | `ShellCompDirectiveNoFileComp` (no NoSpace) — silent fallback |
| (e) Ambiguous prefix (2+ matches) | `"co"` (matches `code`, `colors`, ...) | — | `["code", "colors", ...]` (today's full list) | `ShellCompDirectiveNoFileComp` (no NoSpace) — eager branch skipped entirely |

Test injection uses the existing `listWorktrees` package-level `var` seam (per `wt_list.go`). `cloneState` is invoked at the real `repo.Path`; tests construct test repos with/without `.git` directories using the same fixture pattern already used by `repo_completion_test.go`'s existing tests (verify by reading the file before adding new cases).

A sixth implicit case — empty `toComplete` — falls into the ambiguous-prefix case because `MatchOne("")` returns all repos; len != 1, eager branch skipped. No dedicated test row needed; case (e) covers it structurally.

### Surface 3: `docs/memory/cli/subcommands.md`

Modify the `hop shell-init` section's tab-completion subsection (where worktree completion in the `<repo>/` branch is currently documented). Add a paragraph describing the eager pre-slash expansion: trigger condition (`MatchOne(toComplete)` returns exactly one cloned repo with ≥2 worktrees), candidate shape (bare + suffixed), directive (`NoSpace`), and the silent-fallback contract for failure modes.

### Surface 4: `docs/memory/cli/match-resolution.md`

Modify the worktree-resolution sub-step description. Add a note (probably near the existing "Worktree error paths" table or in a new "Tab Completion" subsection at the end) that the same `listWorktrees` seam now drives two completion entry points: the post-slash `completeWorktreeCandidates` and the new pre-slash eager branch. Cross-link to `subcommands.md`.

## Affected Memory

- `cli/subcommands.md`: (modify) extend the `hop shell-init` tab-completion documentation with the eager pre-slash expansion paragraph
- `cli/match-resolution.md`: (modify) note that the worktree-resolution seam now serves two completion entry points

## Impact

**Code (single function + tests)**:
- `src/cmd/hop/repo_completion.go` — extend `completeRepoNames`
- `src/cmd/hop/repo_completion_test.go` — add 4 new test cases (existing tests unchanged)

**No new dependencies, no new flags, no new subcommands, no new package.** Reuses:
- `rs.MatchOne` (already imported via `repos` package — used by `completeWorktreeCandidates`)
- `cloneState` (package-local, already used)
- `listWorktrees` (the seam from `wt_list.go`, established by change `7eab`)
- `cobra.ShellCompDirectiveNoSpace` (cobra-provided constant)

**Memory**:
- 2 files modified (see above)
- No new memory files, no new domains

**Constitutional fit**:
- Principle I (Security): no new subprocess paths; reuses the existing `listWorktrees` seam which routes through `proc.RunCapture`.
- Principle III (Convention over Configuration): no new flags. Behavior is on-by-default and self-evident from the menu.
- Principle IV (Wrap, Don't Reinvent): reuses `wt list --json` via the existing seam. No reimplementation.
- Principle VI (Minimal Surface Area): modifies one function. No new subcommand.
- Test Integrity: tests verify the spec (the five cases above are spec-derived).

**Latency**: spot-checked at 6.3–8.1ms median per Tab press against unique repo prefix. Cold-cache / network-mounted checkouts will be slower but those paths already exist in the post-slash branch.

**Cross-platform**: no platform-specific code. Behavior is identical across darwin-arm64, darwin-amd64, linux-arm64, linux-amd64.

## Open Questions

<!-- All design questions surfaced during pre-intake discussion are encoded as
     Confident/Tentative entries in the Assumptions table below. None requires
     user input at intake time. -->

(none — see Assumptions table)

## Clarifications

### Session 2026-05-16

| # | Action | Detail |
|---|--------|--------|
| 15 | Confirmed | Filter-first ordering — collide filter runs before eager-expansion check |
| 8 | Confirmed | Bare `<repo>` first, then `wt list --json` source order |
| 9 | Confirmed | `NoFileComp \| NoSpace` on eager-fire; `NoFileComp` only elsewhere |
| 10 | Confirmed | Bash / no-menu-select degrades to flat candidate list |
| 11 | Confirmed | Extra Space keystroke for "use main" accepted as price of discoverability |
| 12 | Confirmed | No caching across Tab presses |
| 13 | Confirmed | `listWorktrees` seam for test injection; existing fixture pattern |
| 14 | Confirmed | Empty `toComplete` naturally falls into ambiguous-prefix case |

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Eager expansion fires ONLY when `len(matches) == 1` (unique match). Ambiguous prefixes (0 or 2+ matches) keep today's behavior verbatim. | Constitution + design discussion: ambiguous prefixes must remain cheap (no `wt list --json` fan-out across N repos). The unique-match guard is the cost gate. | S:95 R:90 A:95 D:95 |
| 2 | Certain | Eager expansion fires ONLY when the resolved repo is cloned (`cloneState == stateAlreadyCloned`). | Uncloned repos have no `.git` to run `wt list --json` against. Existing precedent: `completeWorktreeCandidates` applies the same guard. | S:95 R:90 A:95 D:90 |
| 3 | Certain | Eager expansion fires ONLY when `wt list --json` returns ≥2 entries (main + ≥1 extra). Repos with only the main worktree keep today's behavior (trailing space, finalize $1). | Design discussion: the ≥2 threshold means the menu only appears when worktree-vs-main is a real choice. 1-worktree repos have nothing to disambiguate, so paying the NoSpace keystroke cost there is pure regression. | S:90 R:90 A:90 D:95 |
| 4 | Certain | Failure modes (uncloned, `wt` missing, `wt list --json` non-zero exit, malformed JSON) silently fall back to today's behavior — no stderr writes from completion. | Existing contract enforced by the file's package-level comment block and by `completeWorktreeCandidates`'s precedent. Tab completion must never produce noise. | S:95 R:95 A:95 D:95 |
| 5 | Certain | The post-slash branch (`completeWorktreeCandidates`, triggered by `strings.Contains(toComplete, "/")`) is preserved unchanged. | Surgical change — eager expansion is additive, not a replacement for the slash trigger. Users who have already typed `/` skip the eager check entirely. | S:95 R:95 A:95 D:95 |
| 6 | Certain | Reuse the existing `listWorktrees` package-level seam from `wt_list.go`. No new wrapper, no new package, no promotion to `internal/wt/`. | Constitution IV (Wrap, Don't Reinvent) + the seam was deliberately designed for this kind of extension. Promotion threshold is "3+ call sites with divergent needs" — this is the third call site but the need is identical. | S:90 R:85 A:95 D:90 |
| 7 | Certain | Change type is `feat` (new user-visible completion behavior). | Pre-intake discussion explicitly landed on `feat` over `refactor`: the user-visible Tab outcome changes shape (now offers a menu instead of finalizing). Even though no grammar changes, the UX surface expands. | S:90 R:90 A:90 D:75 |
| 8 | Certain | Candidate ordering: bare `<repo>` at position 0, followed by `<repo>/<wt>` entries in `wt list --json` source order (typically main first, then by creation date). | Clarified — user confirmed | S:95 R:75 A:80 D:70 |
| 9 | Certain | Directive on eager-fire: `ShellCompDirectiveNoFileComp \| ShellCompDirectiveNoSpace` (bitwise OR). All other branches: today's `ShellCompDirectiveNoFileComp` only. | Clarified — user confirmed | S:95 R:90 A:85 D:80 |
| 10 | Certain | Bash users (and zsh users without `menu select`) get a flat candidate list rather than arrow-key navigation. Acceptable degradation — the candidates are still surfaced, disambiguated by typing more characters. | Clarified — user confirmed | S:95 R:80 A:75 D:70 |
| 11 | Certain | Bare-cd cost: under the new behavior, `h outbox<TAB>` against a multi-worktree repo no longer auto-spaces — the user pays one extra Space keystroke to commit "use main." Accepted as the price of discoverability. | Clarified — user confirmed | S:95 R:80 A:70 D:75 |
| 12 | Certain | No caching of `wt list --json` results across Tab presses. Each Tab invocation that triggers eager expansion runs `wt list --json` once. | Clarified — user confirmed | S:95 R:75 A:80 D:75 |
| 13 | Certain | Tests use the existing `listWorktrees` seam for injection. `cloneState` is invoked at real `repo.Path`; test fixtures use the same `.git` directory pattern as existing `repo_completion_test.go` tests. | Clarified — user confirmed | S:95 R:75 A:80 D:75 |
| 14 | Certain | Empty `toComplete` (`h <TAB>` with nothing typed) falls into the ambiguous-prefix case naturally. `MatchOne("")` returns all repos, `len != 1`, eager branch is skipped. No special-case code needed. | Clarified — user confirmed | S:95 R:90 A:85 D:80 |
| 15 | Certain | Subcommand-collision filter runs BEFORE the eager-expansion check. A repo whose `Name` collides with a cobra subcommand (e.g., a repo literally named `clone`) is filtered out of `names` first; the eager check then operates only on non-colliding candidates. Consistent with the post-slash branch (which doesn't bypass the filter) and matches user-facing reality: cobra dispatches collide-names to the subcommand before the bare-form resolver ever sees them. A spec-level test pins this ordering. | Clarified — user confirmed filter-first ordering | S:95 R:60 A:55 D:50 |

15 assumptions (15 certain, 0 confident, 0 tentative, 0 unresolved).
