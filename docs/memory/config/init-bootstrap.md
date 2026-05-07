# Config Init / Bootstrap

How `hop config init` and `hop config where` behave. Implemented in `src/cmd/hop/config.go`; the actual file write is in `src/internal/config/config.go::WriteStarter`. Starter content is embedded from `src/internal/config/starter.yaml`.

## `hop config init`

1. Calls `config.ResolveWriteTarget()` ([search-order](search-order.md)) — does NOT trigger the missing-file hard error.
2. Calls `config.WriteStarter(target)`:
   - If target exists → returns:
     ```
     hop config init: <path> already exists. Delete it first or set $HOP_CONFIG to a different path.
     ```
     Exit 1. Existing file is untouched.
   - Creates parent dir via `os.MkdirAll(dir, 0o755)` if absent.
   - Writes `starterContent` (embedded via `//go:embed starter.yaml`) with file mode **0644**.
3. Stdout: `Created <path>`.
4. Stderr (two lines):
   ```
   Edit the file to add your repos, or run `hop config scan <dir>` to populate from existing on-disk repos.
   Tip: set $HOP_CONFIG in your shell rc to point at a version-tracked location (a git-tracked dotfile, Dropbox, etc.) so this config moves with you across machines.
   ```
   The first line surfaces `hop config scan` for onboarding discoverability — without it, scan is invisible to new users. The `Tip:` line is preserved verbatim from the pre-scan wording.
5. Exit 0.

The `0644` mode is intentional: the file contains repo paths and public git URLs — no credentials. Treating it as sensitive (0600) would be theater.

## Embedded starter content

Stored verbatim at `src/internal/config/starter.yaml` and pulled in via `//go:embed`. Self-bootstrapping — points at this repo so a fresh user can `hop` (fzf shows one entry) or `hop clone hop` immediately:

```yaml
# hop config — locator and operations registry.
# Edit to add repos. Tip: set $HOP_CONFIG to a tracked path (dotfiles, Dropbox)
# so this config moves with you across machines.
#
# Two ways to add a repo:
#   1. Append a URL to a flat group (default) — convention applies:
#      path = <config.code_root>/<org-from-url>/<name-from-url>
#   2. Use a named group with explicit `dir:` to override convention.

config:
  code_root: ~/code

repos:
  default:
    - git@github.com:sahil87/hop.git    # the locator tool itself

  # Example: vendor group with explicit dir override.
  # vendor:
  #   dir: ~/vendor
  #   urls:
  #     - git@github.com:some-vendor/their-tool.git
```

`config.StarterContent() []byte` exposes the embedded bytes for tests that compare exact contents.

The starter parses cleanly under the new schema validator (verified by `TestStarterParses` in `config_test.go`).

## `hop config where`

Prints `config.ResolveWriteTarget()` to stdout. Exit 0 unless nothing resolves at all (no env vars, no `$HOME`). Never errors on missing file — it's a debug aid, not a load.

Renamed from v0.0.1's `hop config path` for voice-fit consistency with the locator's `hop where`. Both `init` and `where` are exempt from the standard "load `hop.yaml` first" flow; they run cleanly even when no config exists yet.

## Cross-references

- Bootstrap-then-populate workflow (`hop config init` followed by `hop config scan <dir>`): [scan](scan.md)
- Search order shared by `init`, `where`, and `scan`'s precondition check: [search-order](search-order.md)
