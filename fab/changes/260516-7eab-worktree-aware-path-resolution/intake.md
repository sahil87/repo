# Intake: Worktree-Aware Path Resolution

**Change**: 260516-7eab-worktree-aware-path-resolution
**Created**: 2026-05-16
**Status**: Draft

## Origin

> Worktree-aware path resolution for hop — extend the repo-arg grammar with an optional `/<wt-name>` suffix so every existing verb inherits worktree-awareness.

Reached via discussion in `/fab-discuss`, then handed off to `/fab-new`. Key shape decisions (grammar, scope folding into `ls --trees`, no DB, error messages, completion) were settled in conversation before this intake was written and are encoded as Certain / Confident assumptions in the table below — downstream agents should not re-litigate them.

The conversation also explicitly declined three alternative scopes (cross-tree batch git ops, `hop clone --wt-init`, hop.yaml worktree fields) — see the "Explicitly out of scope" section under What Changes.

Mode: conversational design session → one-shot intake generation. No live questions remain at intake time.

## Why

**The pain point.** Today, to land inside a worktree of a hop-registered repo, the user must `cd` to the main checkout first and then run `wt <wt-name>` (or use wt's picker). That works, but it breaks the one-shot `hop <name> <tool>` ergonomics that hop is designed around. You can't say "the `feat-x` worktree of `outbox`" in a single utterance through hop.

**The consequence if not fixed.** hop and wt — both sahil-authored sibling locators with overlapping mental models ("type the name, land in the right directory") — stay un-composed. Users either learn two unrelated grammars or open scripts to bridge them. Every new hop verb (pull, push, sync, open, -R, tool-form) silently lacks a worktree dimension, even though wt could fulfill it trivially.

**Why this approach over alternatives.** hop is a locator for repos; wt is a locator for worktrees within a repo. The composition seam already exists at exactly one point — `hop <name> open` execs `wt open <path>` (`src/cmd/hop/open.go:30`). Extending hop's path resolver (not adding new verbs) means:

1. **Minimal surface area** (Constitution VI). Zero new top-level subcommands; one optional grammar suffix; one flag (`--trees`) folded onto an existing subcommand.
2. **Verbs inherit for free.** Implementing this in `resolveOne` / `resolveByName` means every verb that calls those — `where`, `cd`, `open`, `pull`, `push`, `sync`, `-R`, tool-form — automatically gains worktree-awareness without per-verb code. Cobra wiring, exit codes, and stdout/stderr semantics propagate.
3. **No state duplication** (Constitution II). Worktree existence and paths are derived from `wt list --json` at request time. hop.yaml stays repo-only; the filesystem and wt remain authoritative for worktree state.
4. **Wrap, don't reinvent** (Constitution IV). wt already has a clean `list --json` interface returning `[{name, branch, path, is_main, is_current, dirty, unpushed}]`. Hop shells out and consumes JSON — no worktree concept inside hop's data model.

Rejected alternatives are enumerated in the "Alternatives considered and rejected" subsection under What Changes.

## What Changes

### 1. Grammar extension — `hop <name>/<wt-name>`

The optional `/<wt-name>` suffix on the repo positional resolves to that worktree's absolute path. The `/` separator is safe because repo names in `hop.yaml` are URL basenames (single component, no `/`) — verified by `docs/specs/config-resolution.md` and the `repos.FromConfig` projection.

Resolution mechanism (implemented inside `resolveByName` / `resolveOne` in `src/cmd/hop/resolve.go`):

1. Split the incoming query on the first `/`. If no `/` is present, the existing path applies unchanged (full backward compatibility).
2. Resolve the repo half (LHS) using the existing match-or-fzf algorithm — produces an `*repos.Repo` with an absolute `Path`.
3. Shell out to `wt list --json` with `Dir = repo.Path` via `internal/proc.RunCapture` (independent context with a short timeout — wt is a local op, no network). Parse the resulting `[{name, branch, path, is_main, is_current, dirty, unpushed}]` array.
4. Find the entry whose `name` equals the RHS (exact match — wt's names are user-curated; substring would re-introduce the ambiguity wt itself avoids). Return a `*repos.Repo` whose `Path` is the worktree's absolute path. The `Name`, `Group`, `URL` fields stay derived from the original repo (they describe the registry entry, not the on-disk worktree).
5. Verbs that already use `repo.Path` (`where`, `cd` via shim, `open` → `wt open`, `-R`, tool-form, `pull`, `push`, `sync`) inherit the new path automatically.

Examples:

```
hop outbox/feat-x                 # shim cd into the feat-x worktree of outbox
hop outbox/feat-x where           # print the worktree's absolute path
hop outbox/feat-x open            # delegate to wt's app menu, pre-targeted at the worktree path
hop outbox/feat-x git status      # tool-form, rooted at the worktree
hop outbox/feat-x -R make test    # exec-in-context, rooted at the worktree
```

### 2. `hop ls --trees` flag

A flag on the existing `ls` subcommand (NOT a new top-level subcommand — folded into `ls` to honour Constitution Principle VI). Fans out `wt list --json` across configured cloned repos and prints a compact per-repo summary with worktree counts and dirty/unpushed indicators.

Answers the question: "where did I leave that branch?" — the use case that motivates having multiple worktrees in the first place.

Indicative output shape (exact format to be settled in spec):

```
$ hop ls --trees
outbox          3 trees  (main, feat-x*, hotfix~)
dotfiles        1 tree   (main)
hop             2 trees  (main, refactor-resolve*)
loom            (not cloned)
```

Where `*` flags dirty and `~` flags unpushed (or some equivalent — final glyph choice in spec). Non-cloned repos surface as `(not cloned)` rather than fanning out wt against a missing directory.

### 3. Shim completion (zsh + bash)

The shell shim emitted by `hop shell-init` MUST complete worktree names after the `/`. When completion sees a `<repo>/` prefix, it runs `wt list --json` in that repo and offers the names. Without this, the syntax is undiscoverable; with it, hop's `/` suffix is as ergonomic as repo-name completion already is.

Implementation lives in `src/cmd/hop/shell_init.go` (the zsh/bash branches). The completion function for the repo positional becomes prefix-aware: no `/` → existing repo-name completion; `<repo>/<partial>` → resolve LHS, run `wt list --json`, offer matching names. The cobra-generated completion handles flag/subcommand completion; the repo-arg slot was already custom (via cobra's `ValidArgsFunction`) — this extends that custom function.

### Edge cases

- **`hop <name>/main`** — wt treats `main` as the main worktree's name in its JSON output. Resolves to the same path as bare `hop <name>` (no special-case in hop; the `wt list --json` round-trip yields the main checkout's path naturally).
- **Missing `wt` binary** — reuse the existing `ErrNotFound` pattern from `src/cmd/hop/open.go:32-33`: write `hop: wt: not found on PATH.` to stderr, exit 1 (`errSilent`). wt is already declared as a Homebrew formula dependency (`depends_on "sahil87/tap/wt"` in `.github/formula-template.rb`); the runtime hint covers non-brew installs and manual removal.
- **No-such-worktree** — clear, actionable error: `hop: worktree '<wt-name>' not found in '<repo>'`. Suggest follow-up: `Try: wt list (in <repo-path>) or hop ls --trees`.
- **Repo not cloned** — the existing "not cloned" error path fires before we ever invoke `wt list`. No change to that branch.
- **`wt list --json` failure** (non-zero exit, malformed JSON) — surface as `hop: wt list: <err>`. Don't silently fall back to the main path; an unparseable wt response is a real failure, not a graceful-degradation moment.
- **Empty RHS** (`hop outbox/`) — treat as a usage error: `hop: empty worktree name after '/'`. Don't silently treat as bare `hop outbox` — the trailing `/` is almost certainly a typo or completion-in-progress.

### Alternatives considered and rejected

1. **`--wt <name>` flag form** (e.g., `hop outbox --wt feat-x where`). Rejected: more typing, breaks the positional-first grammar hop deliberately settled on (Constitution VI's spirit + the v0.x repo-verb grammar flip), and flag-parsing the repo positional gets awkward when verbs follow.

2. **`hop trees` as a new top-level subcommand.** Rejected: Constitution Principle VI ("Minimal Surface Area") explicitly says "could this be a flag on an existing subcommand?" must be answered "no" before adding one. Here it cannot be — `ls` already lists registered repos; `--trees` is a content modifier on that same listing.

3. **Two-positional form** `hop <name> <wt-name> <verb>` (e.g., `hop outbox feat-x where`). Rejected: collides with the existing repo-verb grammar where `args[1]` is a verb or tool name. Adding "or worktree-name" would need a sentinel, multi-step disambiguation, or a new positional cap — all of which complicate the grammar table in `root.go` and the shim's 5-step ladder. The `/` suffix lives inside a single positional and bypasses cobra's positional counting entirely.

### Explicitly out of scope

These were discussed and declined; downstream stages SHALL NOT pull them back in without a new change.

1. **Cross-tree batch git ops** (e.g., `hop sync --all-trees`). Worktrees are usually mid-flight feature branches; auto-rebasing or auto-pushing across them is footgun-shaped and overrides intentional branch state. The rare demand is satisfiable today via `wt list --json --path '*' | xargs -I {} git -C {} pull --rebase` or equivalent.
2. **`hop clone --wt-init`** (auto-create a worktree at clone time). Couples hop ↔ wt tighter than the "Wrap, don't reinvent" spirit warrants — clone is a registry-level op, worktree creation is a per-checkout choice. Belongs in a user's shell rc or a hook, not hop's verb surface.
3. **hop.yaml worktree fields.** Would violate Constitution Principle II ("No Database"). State MUST remain derivable from yaml + filesystem + `wt list --json` at request time. Hardcoding worktree lists in yaml duplicates state and rots immediately as the user runs `wt create` / `wt remove` outside hop.

## Affected Memory

- `cli/subcommands.md`: (modify) `ls` row gains `--trees` flag; add an entry documenting the `<name>/<wt-name>` grammar extension under the bare-name and verb rows; document the new resolution sub-step in `resolveByName`.
- `cli/match-resolution.md`: (modify) extend the algorithm section to describe the `/`-split → resolve repo → `wt list --json` → match RHS pipeline; document the new error paths (missing wt, no-such-worktree, empty RHS).
- `architecture/wrapper-boundaries.md`: (modify) add `wt list --json` to the wrapped-tool surface alongside the existing `wt open` entry; document the JSON contract hop consumes.

## Impact

**Source code**:

- `src/cmd/hop/resolve.go` — primary site. Extend `resolveByName` (or thread a new helper through `resolveOne`) to split on `/`, invoke wt, and rebuild the returned `*repos.Repo` with the worktree path.
- `src/cmd/hop/ls.go` — add `--trees` flag, the fan-out loop, and the summary formatter.
- `src/cmd/hop/shell_init.go` — extend the repo-positional completion function (zsh + bash branches) for the `<repo>/<partial>` case.
- `src/cmd/hop/root.go` — likely no change. Cobra still sees one positional for the repo slot; the `/` is parsed inside resolution, not by cobra.
- Potentially a new helper file (e.g., `src/internal/wt/wt.go`) for the `wt list --json` JSON unmarshal + invocation, kept small. Alternative: inline in `resolve.go` if the call site count stays at 2 (resolve + ls --trees). Decide in spec.

**External tools**:

- `wt list --json` joins `wt open` as a wrapped tool. Both go through `internal/proc` (Constitution I).
- No new third-party Go dependencies — JSON unmarshal uses `encoding/json` from the stdlib.

**Tests** (will be enumerated in plan):

- Unit: resolve.go `/`-split parser, wt-list JSON unmarshal, error paths.
- Integration: end-to-end `hop <name>/<wt-name> where` with a fake wt fixture; `hop ls --trees` with a fixture set.
- Shim: completion-script behavior for the `/` case (likely a bash/zsh harness test).

**APIs / contracts**:

- Adds a new error stderr line shape: `hop: worktree '<wt-name>' not found in '<repo>'`.
- Reuses the existing `hop: wt: not found on PATH.` line — no new wording.
- `hop ls`'s default output is unchanged (no `--trees`); behavior diverges only with the flag.

**Cross-platform**:

- wt is cross-platform-supported on darwin and linux (the platforms hop supports per Constitution Cross-Platform Behavior). No new platform abstraction needed.

## Open Questions

(None blocking — design is settled. The questions below are spec-stage refinements, not gates.)

- Exact stdout format for `hop ls --trees`: column alignment, dirty/unpushed glyphs, how to indicate the currently-active worktree (`is_current` from wt's JSON). Settle in spec.
- Whether `hop ls --trees` should also accept a name/group positional (e.g., `hop ls --trees outbox` for a single-repo deep listing) or stay all-or-nothing. Default assumption: stay all-or-nothing for v1; add positional later if demanded.
- Whether to extract a small `internal/wt` package now (anticipating future wt-delegating verbs) or inline the call in `resolve.go`. Decide in spec — leaning inline per "promote later" pattern documented in `architecture/wrapper-boundaries.md`.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Grammar extension uses `/<wt-name>` suffix on the repo positional (not `--wt` flag, not 2-positional form) | Discussed and chosen — `/` is safe because hop.yaml repo names are URL basenames (single component, no `/`); preserves positional-first grammar; alternatives explicitly rejected | S:95 R:60 A:90 D:90 |
| 2 | Certain | Resolution implemented in `resolveOne`/`resolveByName` so all verbs inherit for free | Discussed — minimal-surface-area + DRY; the existing seam already routes every verb through this function | S:95 R:70 A:95 D:95 |
| 3 | Certain | Shell out to `wt list --json` (no JSON-less alternative parsing) | wt's documented machine-readable interface; aligns with Constitution IV (Wrap, Don't Reinvent) | S:95 R:85 A:95 D:95 |
| 4 | Certain | `hop trees` rejected as top-level subcommand; folded into `hop ls --trees` flag | Constitution VI (Minimal Surface Area) — explicit "could this be a flag" question must be answered yes when it can be | S:95 R:80 A:95 D:95 |
| 5 | Certain | hop.yaml gains NO worktree fields | Constitution II (No Database) — state derived from `wt list --json` at request time | S:95 R:30 A:95 D:95 |
| 6 | Certain | Missing wt reuses existing `hop: wt: not found on PATH.` message from open.go | Same UX surface, already documented in `architecture/wrapper-boundaries.md`; new wording would be inconsistent | S:95 R:90 A:95 D:95 |
| 7 | Certain | Cross-tree batch git ops (`hop sync --all-trees`) explicitly out of scope | Discussed and rejected — footgun-shaped; satisfiable via wt + xargs today | S:95 R:80 A:90 D:95 |
| 8 | Certain | `hop clone --wt-init` explicitly out of scope | Discussed and rejected — couples hop↔wt tighter than warranted; belongs in shell rc/hook | S:95 R:80 A:90 D:95 |
| 9 | Certain | Shim completion (zsh + bash) is part of v1, not deferred | Discussed and stated as a MUST — "required for the syntax to be discoverable"; without completion the feature exists but no one finds it | S:95 R:65 A:90 D:90 |
| 10 | Certain | `hop <name>/main` resolves to main worktree path (same as bare `hop <name>`) via wt's JSON contract | wt's documented behavior treats `main` as the main worktree's name in its JSON output; no special-case needed in hop — the round-trip naturally yields the main path | S:90 R:85 A:90 D:95 |
| 11 | Certain | No-such-worktree error wording: `hop: worktree '<wt-name>' not found in '<repo>'` with `wt list` / `hop ls --trees` suggestion | Discussed and specified; mirrors hop's existing error-with-hint pattern (e.g., `gitMissingHint`, `fzfMissingHint`) | S:90 R:90 A:90 D:90 |
| 12 | Certain | Worktree name match is exact (case-sensitive), not substring | wt names are user-curated and intentional (parallels the case-sensitive group-name match in `resolveTargets`); substring would re-introduce the ambiguity that exact match exists to avoid | S:90 R:80 A:85 D:90 |
| 13 | Certain | `wt list --json` invocation routes through `internal/proc.RunCapture` | Constitution I (Security First) mandates it — no direct `os/exec` outside `internal/proc`; not a choice | S:95 R:90 A:95 D:95 |
| 14 | Certain | Empty RHS (`hop outbox/`) is a usage error, not silently treated as bare repo | Discussed edge case — trailing `/` is almost certainly a typo or completion artifact; silent fallback hides real errors | S:90 R:80 A:85 D:90 |
| 15 | Confident | `hop ls --trees` lists all configured cloned repos (no positional in v1) | Conversation default assumption; positional support trivially additive later if demanded | S:70 R:75 A:80 D:80 |
| 16 | Confident | Worktree resolution layer lives in `resolve.go` (inline) rather than a new `internal/wt` package | Single call site initially; matches `architecture/wrapper-boundaries.md` documented precedent ("promote later if a verb composes more operations" — compare `internal/git/` non-creation despite multiple call sites). If `ls --trees` adds a second call site, spec stage may revisit, but the default is inline | S:80 R:75 A:85 D:80 |
| 17 | Tentative | Exact `hop ls --trees` output format (column shape, dirty/unpushed glyphs, current-worktree indicator) | Discussion left format unfixed; mirror existing `hop ls` two-column alignment as starting point; settle precise format in spec when concrete data shapes are visible | S:50 R:85 A:65 D:50 |
| 18 | Confident | `wt list --json` per-call timeout = 5s (matching `internal/scan`'s `git remote` per-call timeout) | Local op, no network; 5s matches the nearest precedent in the codebase (`internal/scan` `git remote` timeout) and is generous for a local JSON-list operation | S:80 R:85 A:80 D:80 |

18 assumptions (14 certain, 3 confident, 1 tentative, 0 unresolved).
