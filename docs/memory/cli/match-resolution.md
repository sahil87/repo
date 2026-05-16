# Match Resolution

Algorithm shared by every form that takes a `<name>` argument (`hop` bare picker, `hop <name> where`, `hop <name> cd` via the shim's `_hop_dispatch cd` → `command hop "$2" where`, `hop clone`, `hop -R`). Implemented in `src/cmd/hop/resolve.go::resolveByName` and `src/internal/repos/repos.go::MatchOne`.

`hop pull` and `hop sync` use a richer resolver — `resolveTargets` — that prepends an exact group-name match step in front of this same algorithm. See [Name-or-Group Resolution](#name-or-group-resolution) below.

## Algorithm

1. **Pre-step — worktree-suffix split (change `7eab`)**: if `query` contains a `/`, split on the **first** `/`. Repo names from `hop.yaml` are URL basenames (single component, no `/`), so the first `/` unambiguously separates the LHS (repo) from the RHS (worktree name) even when wt worktree names themselves contain `/` (wt permits `/` in names). Empty LHS (`/<wt>`) → return `*errExitCode{code: 2, msg: "hop: empty repo name before '/'"}`; empty RHS (`<repo>/`) → return `*errExitCode{code: 2, msg: "hop: empty worktree name after '/'"}` — silent fallback to bare-repo resolution would hide typos and tab-completion artifacts. Otherwise, recurse on the LHS (step 2 onward) to resolve a `*repos.Repo`, then run the worktree-resolution sub-step (below). When no `/` is present, fall through to step 2 with the original query — behavior is unchanged from pre-`7eab`.
2. `loadRepos()` reads `hop.yaml` (via `config.Resolve` → `config.Load` → `repos.FromConfig`) and returns the full `Repos` list, in YAML source order (groups in `cfg.Groups` order, URLs within each group in source order — yaml.v3 `yaml.Node` round-trip preserves this).
3. If `query == ""` → skip step 4 and go straight to fzf with the full list (no `--query` flag). This handles bare `hop`, `hop clone` (no name), etc.
4. If `query != ""` → filter via `Repos.MatchOne(query)`: case-insensitive substring on `Name` only (not Path, not URL).
   - Exactly **1 match** → return it directly. Fzf is **not** invoked. (This is the path that works without fzf installed.)
   - 0 or 2+ matches → fall through to fzf.
5. Fzf invocation: `internal/fzf.Pick(ctx, lines, query)` pipes the **full repo list** (not the filtered subset) to fzf via stdin. Each line is `display\tpath\turl`. Fzf displays only the first column (`--with-nth 1 --delimiter '\t'`). The display column is built by `buildPickerLines` (see below).
6. The returned line's path column (the second tab-separated field) is matched back to the source `Repos` to recover the full `*Repo`.

### Worktree-resolution sub-step (change `7eab`)

Triggered by step 1 when the query contains `/`. Lives in `resolve.go::resolveWorktreePath`, called from `resolveByName` after the LHS resolves.

1. **Cloned-state guard**: `cloneState(repo.Path)` MUST return `stateAlreadyCloned`. Uncloned repos return `*errExitCode{code: 1, msg: "hop: '<name>' is not cloned. Try: hop clone <name>"}` — `wt list --json` is NEVER invoked against a missing main checkout. This guard applies ONLY to `/`-suffixed queries; bare queries (`hop <name> where` with no `/`) keep their existing permissive behavior of resolving registry paths even for repos that haven't been cloned yet.
2. **Invoke `wt list --json`** via the package-level `listWorktrees(ctx, repo.Path)` seam in `wt_list.go`. The default seam routes through `internal/proc.RunCapture` with `cmd.Dir = repo.Path` and a 5-second `context.WithTimeout` (matching `internal/scan`'s `git remote` precedent — wt list is a local op with no network round-trip). Unmarshal into `[]WtEntry` (the JSON contract — see [architecture/wrapper-boundaries](../architecture/wrapper-boundaries.md)).
3. **Match RHS** against each entry's `Name` field: exact equality, case-sensitive. Mirrors the case-sensitive group-name match in `resolveTargets` (wt names are user-curated and intentional; case-insensitive substring would re-introduce ambiguity wt itself avoids). The `hop <name>/main` case naturally resolves to the main checkout's path because wt's JSON entry with `is_main: true` carries that path — no special-case branching in hop.
4. **Return a shallow-copied `*repos.Repo`** whose `Path` is the matched entry's `Path`; `Name`, `Group`, `URL`, `Dir` are preserved from the LHS-resolved repo (they describe the registry identity, not the on-disk worktree). Every verb downstream (`where`, `cd` via shim, `open`, `-R`, tool-form, `pull`, `push`, `sync`) inherits the worktree path automatically because they all consume `repo.Path`.

### Worktree error paths

Each surfaces as `*errExitCode{code: <n>, msg: <pre-formatted stderr line>}` so `translateExit` prints the message verbatim. Listed here for reference; the wordings live as constants in `resolve.go` (`wtMissingHint`) or as inline `fmt.Sprintf` calls.

| Trigger | Exit code | Stderr line |
|---|---|---|
| Empty LHS (`hop /feat-x where`) | 2 | `hop: empty repo name before '/'` |
| Empty RHS (`hop outbox/ where`) | 2 | `hop: empty worktree name after '/'` |
| `/`-suffixed query, repo not cloned | 1 | `hop: '<name>' is not cloned. Try: hop clone <name>` |
| `wt` missing on PATH | 1 | `hop: wt: not found on PATH.` (the `wtMissingHint` constant, shared with `open.go` and `ls.go --trees`) |
| `wt list --json` non-zero exit or malformed JSON | 1 | `hop: wt list: <err>` (no silent fallback to the main path — unparseable wt output is a real failure) |
| No matching worktree name | 1 | `hop: worktree '<wt>' not found in '<name>'. Try: wt list (in <path>) or hop ls --trees` |

`wt list --json` failures (the last two rows) are deliberately loud — silent fallback to the main checkout's path would mask wt schema regressions, fixture corruption, and typos that exact-match avoids. Unknown JSON fields ARE silently ignored (Go's `encoding/json` default — no `DisallowUnknownFields`) so future wt schema additions don't break hop; only structurally invalid JSON or wrong top-level shape surfaces as `hop: wt list: <err>`.

## Group disambiguation in the picker

`buildPickerLines` (in `src/cmd/hop/resolve.go`) computes a count of how many repos share each `Name`. When `nameCount[r.Name] > 1`, the displayed first column is `<name> [<group>]` rather than just `<name>`. When the name is unique across groups, the suffix is omitted to keep picker lines short.

> **Limitation**: Two URLs in the *same* group whose derived `Name` collides still render an identical first column. Cross-group collisions are handled; intra-group collisions are not. (See backlog `[qwrd]`.)

## Fzf invocation

`internal/fzf/fzf.go::buildArgs`:

```
fzf --query <q> --select-1 --height 40% --reverse --with-nth 1 --delimiter '<TAB>'
```

`--query` is **omitted** when `query == ""` (not passed as `--query ""`). `--select-1` makes fzf auto-pick when its filter narrows to exactly 1.

`Pick` uses `context.Background()` (no timeout — user is at the keyboard).

## Cancellation and missing fzf

- Fzf exit 130 (Esc / Ctrl-C) → `proc.Run` returns a non-nil error; `resolveByName` returns `errFzfCancelled` → `translateExit` (or the `-R` path) maps to exit 130.
- Fzf not on PATH → `proc.ErrNotFound` propagates; `resolveByName` returns `errFzfMissing`. The cobra-friendly wrapper `resolveOne` writes `fzfMissingHint` to `cmd.ErrOrStderr()` and returns `errSilent` (exit 1). The `-R` path writes the hint directly to `os.Stderr`.

## Why the full list (not the filtered subset) goes to fzf

Sending the unfiltered list with `--query <q>` lets the user clear the query inside fzf to browse all repos when their initial query yielded zero matches. The bash original behaved this way; the Go port preserves it.

## Order

`repos.FromConfig` walks `cfg.Groups` in source order and emits each group's URLs in source order. Source order is preserved by `config.Load`'s yaml.Node-based parser. Within `hop ls` and the bare-form picker, this gives a stable, user-controlled ordering rather than the alphabetic sort used in v0.0.1 (which was forced by `map[string][]string`'s lack of order preservation).

## Name-or-Group Resolution

Used by `hop pull` and `hop sync`. Implemented in `src/cmd/hop/resolve.go::resolveTargets`. Returns `(repos.Repos, resolveMode, error)` where `resolveMode` is `modeSingle` or `modeBatch` (callers switch on the mode for output formatting and exit-code policy — single-repo failure → exit 1; batch → exit 1 only if any failed).

Resolution rules — first match wins:

1. **`all == true`** — return every repo in `repos.FromConfig` order; mode is `modeBatch`. Positional argument is ignored at this layer (the calling subcommand rejects `--all` combined with a positional as a usage error before invoking `resolveTargets`).
2. **Exact group-name match** (case-sensitive) — `hasGroupExact(rs, query)` scans `r.Group` for an exact equality match. If any repo's group equals `query`, return every repo whose `r.Group == query`; mode is `modeBatch`. Case-sensitivity matches `findGroup`'s contract in `clone.go` and `internal/config`'s YAML schema (group keys are user-curated identifiers; `vendor` does NOT match `Vendor`).
3. **Substring repo-name match** — fall through to `resolveByName` (case-insensitive substring on `Name` with fzf for ambiguous/zero matches; fzf cancellation maps to `errFzfCancelled` → exit 130, fzf-missing maps to `errFzfMissing` so callers emit `fzfMissingHint`). Returns a one-element `repos.Repos`; mode is `modeSingle`.

`resolveTargets` does not re-load YAML for the group-match step — it inspects the `r.Group` field on the already-projected `repos.Repos` slice from `loadRepos()`, avoiding a second `config.Load` round-trip and reusing `FromConfig`'s path-resolution.

The simpler `resolveByName` (and its cobra wrapper `resolveOne`) is still used by `hop` (bare picker), `hop <name> where`, `hop -R`, and `hop clone`'s repo-name argument. Those forms have a single-repo contract — there is no group concept, no `--all`, so there is nothing to add to the algorithm.

### Group-vs-Repo Tiebreaker

When a positional argument exactly equals a group name AND also (case-insensitively) substring-matches one or more repo names, **the group match wins** (rule 2 fires before rule 3). Group names in `hop.yaml` are user-curated and intentional; repo names are derived from URL basenames and incidental. Mismatches are observable via the per-repo `pull:` / `sync:` lines, so users can rename the group in `hop.yaml` to escape the collision.

### Design Decisions

1. **Group-name match before substring repo match (rule 2 before rule 3) — change `xj3k`** (Add `hop pull` and `hop sync` Subcommands).
   - *Why*: User-curated group names should beat URL-derived repo names on collision (the user's intent is the group). Per-repo status lines surface mismatches; renaming the group in `hop.yaml` is the escape hatch.
   - *Rejected*: Substring repo match wins (would force users to escape group names with a `--group` flag); reject with "ambiguous" error (raises the user's resolution cost for a theoretical collision).

2. **Case-sensitive group lookup, case-insensitive repo substring match — change `xj3k`.**
   - *Why*: Case-sensitivity matches `findGroup` (clone.go) and the YAML schema. Repo-name match stays case-insensitive to preserve the existing `MatchOne` contract — users typing `outbox` should still find `Outbox`.

3. **Resolver returns a `mode` so callers switch on output and exit-code semantics — change `xj3k`.**
   - *Why*: Without an explicit mode return, each subcommand would re-derive batch-vs-single from `len(targets)`, but a single-mode-with-1-repo case (a unique substring match) needs different exit-code policy than a batch-mode-with-1-repo (a group with one cloned member). The mode keeps the per-subcommand code paths free of `--all` / group-detection leakage.

### Reusable Pattern

The `resolveTargets` shape — "rule-ordered resolver returning `(targets, mode, error)` for verbs that may operate on a single item or a batch" — is a reusable pattern in `src/cmd/hop/`. Future verbs that take a name OR a group OR `--all` (e.g., a hypothetical `hop fetch` or `hop status`) should reuse `resolveTargets` rather than re-implement the rule order. Adding rules (e.g., `--group <name>` flag, regex match) is a localized change in `resolve.go`.
