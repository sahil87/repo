# Config Init / Bootstrap

How `repo config init` and `repo config path` behave. Implemented in `src/cmd/repo/config.go`; the actual file write is in `src/internal/config/config.go::WriteStarter`. Starter content is embedded from `src/internal/config/starter.yaml`.

## `repo config init`

1. Calls `config.ResolveWriteTarget()` ([search-order](search-order.md)) — does NOT trigger the missing-file hard error.
2. Calls `config.WriteStarter(target)`:
   - If target exists → returns:
     ```
     repo config init: <path> already exists. Delete it first or set $REPOS_YAML to a different path.
     ```
     Exit 1. Existing file is untouched.
   - Creates parent dir via `os.MkdirAll(dir, 0o755)` if absent.
   - Writes `starterContent` (embedded via `//go:embed starter.yaml`) with file mode **0644**.
3. Stdout: `Created <path>`.
4. Stderr: `Edit the file to add your repos. Tip: set $REPOS_YAML in your shell rc to point at a version-tracked location (a git-tracked dotfile, Dropbox, etc.) so this config moves with you across machines.`
5. Exit 0.

The `0644` mode is intentional: the file contains repo paths and public git URLs — no credentials. Treating it as sensitive (0600) would be theater.

## Embedded starter content

Stored verbatim at `src/internal/config/starter.yaml` and pulled in via `//go:embed`. Self-bootstrapping — points at this repo so a fresh user can `repo` (fzf shows one entry) or `repo clone repo` immediately:

```yaml
# Repositories to clone, grouped by parent directory.
# Each key is a directory (~ is expanded). Values are git clone URLs.
#
# Edit this file to add your own repos. Tip: set $REPOS_YAML in your shell rc
# to point at a version-tracked location (a git-tracked dotfile, Dropbox, etc.)
# so this config moves with you across machines.
#
# Example below: clones the `repo` tool itself. Replace or extend.

~/code/sahil87:
  - git@github.com:sahil87/repo.git
```

`config.StarterContent() []byte` exposes the embedded bytes for tests that compare exact contents.

## `repo config path`

Prints `config.ResolveWriteTarget()` to stdout. Exit 0 unless nothing resolves at all (no env vars, no `$HOME`). Never errors on missing file — it's a debug aid, not a load.

Both `init` and `path` are exempt from the standard "load `repos.yaml` first" flow; they run cleanly even when no config exists yet.
