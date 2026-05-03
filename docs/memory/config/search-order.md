# Config Search Order

How `repos.yaml` is located on every invocation. Implemented in `src/internal/config/resolve.go`.

Two entry points:

- `Resolve() (string, error)` — used by every load path. Hard-errors on misconfig.
- `ResolveWriteTarget() (string, error)` — used by `repo config init` and `repo config path`. Returns the path that *would* be used regardless of file existence; never hard-errors on missing file.

## Search order (both functions)

1. `$REPOS_YAML` if set and non-empty
2. `$XDG_CONFIG_HOME/repo/repos.yaml` if `$XDG_CONFIG_HOME` is set
3. `$HOME/.config/repo/repos.yaml`

The first candidate that resolves wins. There is no caching — re-resolved on every invocation (Constitution Principle II "No Database").

## `Resolve()` semantics

- Candidate 1: if `$REPOS_YAML` is set and the file exists → return it. If set but file missing → hard error (do **not** fall through):
  ```
  repo: $REPOS_YAML points to <path>, which does not exist. Set $REPOS_YAML to an existing file or unset it.
  ```
  Setting an env var is intent; falling through would mask config bugs.
- Candidates 2 and 3: each `os.Stat` checked. Missing → fall to next candidate (no error).
- All three exhausted → return:
  ```
  repo: no repos.yaml found. Set $REPOS_YAML to a tracked file (e.g., a Dropbox path or a git-tracked dotfile), or run 'repo config init' to bootstrap one at $XDG_CONFIG_HOME/repo/repos.yaml.
  ```
- Sentinel `ErrNoConfig` is exported but the actual returned errors use `fmt.Errorf` with the exact messages above (callers don't currently `errors.Is` the sentinel).

## `ResolveWriteTarget()` semantics

Identical search order, but:

- Returns candidate 1 even when the file does not exist (no `os.Stat`).
- Returns candidate 2 / 3 paths without `os.Stat` checks — the caller (`repo config init`) writes there and creates parents as needed; `repo config path` just prints the path.
- Errors only when nothing resolves at all (no `$REPOS_YAML`, no `$XDG_CONFIG_HOME`, no `$HOME`):
  ```
  repo: no config path resolvable. Set $REPOS_YAML or ensure $XDG_CONFIG_HOME or $HOME is set.
  ```

## Removed vs. bash original

The bash script also checked `$DOTFILES_DIR/repos.yaml` and `$HOME/code/bootstrap/dotfiles/repos.yaml`. Both are intentionally **removed** — they were Sahil's personal layout leaking into the binary. Users who depended on them set `$REPOS_YAML` instead.

## Cross-references

- YAML schema and parsing: [yaml-schema](yaml-schema.md)
- Bootstrap behavior of `repo config init`: [init-bootstrap](init-bootstrap.md)
