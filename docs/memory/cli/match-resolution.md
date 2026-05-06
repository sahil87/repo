# Match Resolution

Algorithm shared by every subcommand that takes a `<name>` argument (`hop`, `hop where`, `hop cd`, `hop clone`, `hop -R`). Implemented in `src/cmd/hop/where.go::resolveByName` and `src/internal/repos/repos.go::MatchOne`.

## Algorithm

1. `loadRepos()` reads `hop.yaml` (via `config.Resolve` → `config.Load` → `repos.FromConfig`) and returns the full `Repos` list, in YAML source order (groups in `cfg.Groups` order, URLs within each group in source order — yaml.v3 `yaml.Node` round-trip preserves this).
2. If `query == ""` → skip step 3 and go straight to fzf with the full list (no `--query` flag). This handles bare `hop`, `hop clone` (no name), etc.
3. If `query != ""` → filter via `Repos.MatchOne(query)`: case-insensitive substring on `Name` only (not Path, not URL).
   - Exactly **1 match** → return it directly. Fzf is **not** invoked. (This is the path that works without fzf installed.)
   - 0 or 2+ matches → fall through to fzf.
4. Fzf invocation: `internal/fzf.Pick(ctx, lines, query)` pipes the **full repo list** (not the filtered subset) to fzf via stdin. Each line is `display\tpath\turl`. Fzf displays only the first column (`--with-nth 1 --delimiter '\t'`). The display column is built by `buildPickerLines` (see below).
5. The returned line's path column (the second tab-separated field) is matched back to the source `Repos` to recover the full `*Repo`.

## Group disambiguation in the picker

`buildPickerLines` (in `src/cmd/hop/where.go`) computes a count of how many repos share each `Name`. When `nameCount[r.Name] > 1`, the displayed first column is `<name> [<group>]` rather than just `<name>`. When the name is unique across groups, the suffix is omitted to keep picker lines short.

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
