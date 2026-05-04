# Config Search Order

How `hop.yaml` is located on every invocation. Implemented in `src/internal/config/resolve.go`.

Two entry points:

- `Resolve() (string, error)` — used by every load path. Hard-errors on misconfig.
- `ResolveWriteTarget() (string, error)` — used by `hop config init` and `hop config where`. Returns the path that *would* be used regardless of file existence; never hard-errors on missing file.

## Search order (both functions)

1. `$HOP_CONFIG` if set and non-empty
2. `$XDG_CONFIG_HOME/hop/hop.yaml` if `$XDG_CONFIG_HOME` is set
3. `$HOME/.config/hop/hop.yaml`

The first candidate that resolves wins. There is no caching — re-resolved on every invocation (Constitution Principle II "No Database").

## `Resolve()` semantics

- Candidate 1: if `$HOP_CONFIG` is set and the file exists → return it. If set but file missing → hard error (do **not** fall through):
  ```
  hop: $HOP_CONFIG points to <path>, which does not exist. Set $HOP_CONFIG to an existing file or unset it.
  ```
  Setting an env var is intent; falling through would mask config bugs.
- Candidates 2 and 3: each `os.Stat` checked. Missing → fall to next candidate (no error).
- All three exhausted → return:
  ```
  hop: no hop.yaml found. Set $HOP_CONFIG to a tracked file (e.g., a Dropbox path or a git-tracked dotfile), or run 'hop config init' to bootstrap one at $XDG_CONFIG_HOME/hop/hop.yaml.
  ```
- Sentinel `ErrNoConfig` is exported but the actual returned errors use `fmt.Errorf` with the exact messages above (callers don't currently `errors.Is` the sentinel).

## `ResolveWriteTarget()` semantics

Identical search order, but:

- Returns candidate 1 even when the file does not exist (no `os.Stat`).
- Returns candidate 2 / 3 paths without `os.Stat` checks — the caller (`hop config init`) writes there and creates parents as needed; `hop config where` just prints the path.
- Errors only when nothing resolves at all (no `$HOP_CONFIG`, no `$XDG_CONFIG_HOME`, no `$HOME`):
  ```
  hop: no config path resolvable. Set $HOP_CONFIG or ensure $XDG_CONFIG_HOME or $HOME is set.
  ```

## No fallback to legacy paths

The previous v0.0.1 search order (`$REPOS_YAML`, `$XDG_CONFIG_HOME/repo/repos.yaml`, `$HOME/.config/repo/repos.yaml`) is **gone**. There is no fallback chain. A user with a v0.0.1 `repos.yaml` will see "no hop.yaml found" until they `cp` and edit it to the new schema (and new path).

## Cross-references

- YAML schema and parsing: [yaml-schema](yaml-schema.md)
- Bootstrap behavior of `hop config init`: [init-bootstrap](init-bootstrap.md)
