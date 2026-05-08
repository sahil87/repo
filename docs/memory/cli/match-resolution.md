# Match Resolution

Algorithm shared by every form that takes a `<name>` argument (`hop` bare picker, `hop <name> where`, `hop <name> cd` via the shim's `_hop_dispatch cd` → `command hop "$2" where`, `hop clone`, `hop -R`). Implemented in `src/cmd/hop/resolve.go::resolveByName` and `src/internal/repos/repos.go::MatchOne`.

`hop pull` and `hop sync` use a richer resolver — `resolveTargets` — that prepends an exact group-name match step in front of this same algorithm. See [Name-or-Group Resolution](#name-or-group-resolution) below.

## Algorithm

1. `loadRepos()` reads `hop.yaml` (via `config.Resolve` → `config.Load` → `repos.FromConfig`) and returns the full `Repos` list, in YAML source order (groups in `cfg.Groups` order, URLs within each group in source order — yaml.v3 `yaml.Node` round-trip preserves this).
2. If `query == ""` → skip step 3 and go straight to fzf with the full list (no `--query` flag). This handles bare `hop`, `hop clone` (no name), etc.
3. If `query != ""` → filter via `Repos.MatchOne(query)`: case-insensitive substring on `Name` only (not Path, not URL).
   - Exactly **1 match** → return it directly. Fzf is **not** invoked. (This is the path that works without fzf installed.)
   - 0 or 2+ matches → fall through to fzf.
4. Fzf invocation: `internal/fzf.Pick(ctx, lines, query)` pipes the **full repo list** (not the filtered subset) to fzf via stdin. Each line is `display\tpath\turl`. Fzf displays only the first column (`--with-nth 1 --delimiter '\t'`). The display column is built by `buildPickerLines` (see below).
5. The returned line's path column (the second tab-separated field) is matched back to the source `Repos` to recover the full `*Repo`.

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
