# Match Resolution

Algorithm shared by every subcommand that takes a `<name>` argument (`repo`, `repo path`, `repo code`, `repo open`, `repo cd`, `repo clone`). Implemented in `src/cmd/repo/path.go::resolveOne` and `src/internal/repos/repos.go::MatchOne`.

## Algorithm

1. `loadRepos()` reads `repos.yaml` (via `config.Resolve` → `config.Load` → `repos.FromConfig`) and returns the full `Repos` list, sorted by directory key then by URL.
2. If `query == ""` → skip step 3 and go straight to fzf with the full list (no `--query` flag). This handles bare `repo`, `repo code`, etc.
3. If `query != ""` → filter via `Repos.MatchOne(query)`: case-insensitive substring on `Name` only (not Path, not URL).
   - Exactly **1 match** → return it directly. Fzf is **not** invoked. (This is the path that works without fzf installed.)
   - 0 or 2+ matches → fall through to fzf.
4. Fzf invocation: `internal/fzf.Pick(ctx, lines, query)` pipes the **full repo list** (not the filtered subset) to fzf via stdin. Each line is `name\tpath\turl`. Fzf displays only the first column (`--with-nth 1 --delimiter '\t'`).
5. The returned line's first column is matched back to the source `Repos` to recover the full `*Repo`.

## Fzf invocation

`internal/fzf/fzf.go::buildArgs`:

```
fzf --query <q> --select-1 --height 40% --reverse --with-nth 1 --delimiter '<TAB>'
```

`--query` is **omitted** when `query == ""` (not passed as `--query ""`). `--select-1` makes fzf auto-pick when its filter narrows to exactly 1.

`Pick` uses `context.Background()` (no timeout — user is at the keyboard).

## Cancellation and missing fzf

- Fzf exit 130 (Esc / Ctrl-C) → `proc.Run` returns a non-nil error; `resolveOne` returns `errFzfCancelled` → `translateExit` maps to exit 130.
- Fzf not on PATH → `proc.ErrNotFound` propagates; `resolveOne` writes `fzfMissingHint` to stderr and returns `errSilent` (exit 1). The hint is centralized so every caller of the resolver gets the same message.

## Why the full list (not the filtered subset) goes to fzf

Sending the unfiltered list with `--query <q>` lets the user clear the query inside fzf to browse all repos when their initial query yielded zero matches. The bash original behaved this way; the Go port preserves it.

## Sort order

`repos.FromConfig` sorts `Repos` by directory key, then by URL within each directory. yaml.v3 unmarshals `map[string][]string` without preserving source order, so deterministic sort is the next-best contract — `repo ls` and the bare-form picker always show the same order across invocations.
