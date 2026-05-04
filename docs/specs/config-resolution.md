# Config Resolution

> How `hop.yaml` is found, parsed, validated, and bootstrapped.

## Search Order

Resolved at every invocation — no caching, per Constitution Principle II ("No Database").

1. **`$HOP_CONFIG`** if set and non-empty
2. **`$XDG_CONFIG_HOME/hop/hop.yaml`** if `$XDG_CONFIG_HOME` is set
3. **`$HOME/.config/hop/hop.yaml`** (XDG fallback for systems where `$XDG_CONFIG_HOME` is unset)

The first candidate that resolves is used. Resolution is *not* a fallthrough chain on existence — if `$HOP_CONFIG` is set, candidates 2 and 3 are not consulted (see "Hard error on `$HOP_CONFIG` set but file missing" below).

### Two entry points

- `Resolve() (string, error)` — used by every load path. Hard-errors on misconfig (e.g., `$HOP_CONFIG` set but missing file; nothing resolves).
- `ResolveWriteTarget() (string, error)` — used by `hop config init` and `hop config where`. Returns the path that *would* be used regardless of file existence; never hard-errors on missing file.

### No fallback to legacy paths

The v0.0.1 search order (`$REPOS_YAML`, `$XDG_CONFIG_HOME/repo/repos.yaml`, `$HOME/.config/repo/repos.yaml`) has been **removed without a migration path**. A user with a v0.0.1 `repos.yaml` will see "no hop.yaml found" until they `cp` and edit it to the new schema and new path.

### Hard error on `$HOP_CONFIG` set but file missing

If `$HOP_CONFIG` is set, the user has declared their intent. Falling through to the next candidate would silently mask config bugs (a typo in the path, a deleted file, a broken Dropbox sync).

> **GIVEN** `$HOP_CONFIG=/nonexistent/path.yaml`
> **WHEN** any subcommand needing config runs
> **THEN** stderr shows: `hop: $HOP_CONFIG points to /nonexistent/path.yaml, which does not exist. Set $HOP_CONFIG to an existing file or unset it.`
> **AND** exit code is 1
> **AND** candidates 2 and 3 are NOT consulted

Other resolution scenarios:

> **GIVEN** `$HOP_CONFIG` is unset, `$XDG_CONFIG_HOME=/Users/sahil/.config`, and `/Users/sahil/.config/hop/hop.yaml` exists
> **WHEN** any subcommand needing config runs
> **THEN** that file is loaded
> **AND** candidate 3 is not consulted

> **GIVEN** all of `$HOP_CONFIG`, `$XDG_CONFIG_HOME` are unset, but `~/.config/hop/hop.yaml` exists
> **WHEN** any subcommand needing config runs
> **THEN** `~/.config/hop/hop.yaml` is loaded

> **GIVEN** all three candidates resolve to nothing (no env vars, no file at `~/.config/hop/hop.yaml`)
> **WHEN** any subcommand needing config runs
> **THEN** stderr shows: `hop: no hop.yaml found. Set $HOP_CONFIG to a tracked file (e.g., a Dropbox path or a git-tracked dotfile), or run 'hop config init' to bootstrap one at $XDG_CONFIG_HOME/hop/hop.yaml.`
> **AND** exit code is 1

> **NOTE**: `hop config init` and `hop config where` do **not** require an existing `hop.yaml`. They use `ResolveWriteTarget()` and run cleanly when the file is absent. All other subcommands require a loadable file.

## YAML Schema

Two top-level keys: optional `config:` and required `repos:`.

```yaml
config:
  code_root: ~/code   # optional; defaults to ~

repos:
  default:
    - git@github.com:sahil87/hop.git
    - git@github.com:sahil87/wt.git

  vendor:
    dir: ~/vendor
    urls:
      - git@github.com:some-vendor/their-tool.git

  experiments:
    dir: experiments         # relative — joined with code_root
    urls:
      - git@github.com:sahil87/sandbox.git
```

Parsed via `gopkg.in/yaml.v3` at the `yaml.Node` level (not directly into a typed struct) so that group source order is preserved end-to-end (parsing, listing, picker order, ad-hoc append).

### Top-level

- **`config:`** *(optional)* — map of global config. Currently has one field, `code_root`. Unknown keys → load error: `hop: parse <path>: unknown config field '<name>'`.
- **`repos:`** *(required)* — map of `group_name → group_body`. Missing `repos:` → `hop: parse <path>: missing required field 'repos'`.
- Any other top-level key → `hop: parse <path>: unknown top-level field '<name>'. Valid: 'config', 'repos'.`

### `config.code_root`

Optional. Defaults to `~` (literal tilde, expanded to `$HOME` at use time). Accepted forms:

- Absolute (`/srv/code`) → used verbatim.
- `~`-prefixed (`~`, `~/code`) → `~` expands to `$HOME`.
- Relative (`code`) → resolved relative to `$HOME`.

`code_root` is the convention root for **flat groups** (groups whose body is a list of URL strings — no explicit `dir:`). Map-shaped groups override it via their own `dir:` (which itself may be absolute, `~`-prefixed, or relative — relative `dir:` is joined with `code_root`).

### Group body — two shapes

Each group's value is one of:

1. **Flat list (convention-driven)**: a YAML list of URL strings. Each URL resolves to `<code_root>/<org-from-url>/<name-from-url>`. When `org` is empty (rare — bare path, no slash), the org component is dropped: `<code_root>/<name>`.
2. **Map with `dir` and `urls`**: a YAML map with optional `dir` and optional `urls`. Each URL resolves to `<dir>/<name-from-url>`. `dir` may be absolute, `~`-prefixed, or relative (relative is joined with `code_root`). Empty `dir: ""` → `hop: parse <path>: group '<name>' has empty 'dir'`.

Other YAML shapes for a group body → `hop: parse <path>: group '<name>' must be a list of URLs or a map with 'dir' and 'urls'.`

Map-shaped groups with unknown keys → `hop: parse <path>: group '<name>' has unknown field '<key>'. Valid: 'dir', 'urls'.`

### Group name validation

Validated against `^[a-z][a-z0-9_-]*$`. Mismatches → `hop: parse <path>: invalid group name '<name>'. Group names must match ^[a-z][a-z0-9_-]*$`.

`default` is just a name — not magic, not auto-created. But `hop clone <url>` (with no `--group` flag) defaults to it; if `default` is absent, that command errors.

### URL uniqueness

- **Same URL in two groups** → load error: `hop: parse <path>: URL '<url>' appears in groups '<a>' and '<b>'; a URL must belong to exactly one group.`
- **Same URL twice in one group** → load error: `hop: parse <path>: URL '<url>' is listed twice in group '<name>'.`
- **Two groups with the same `dir` value** → valid (URL-uniqueness covers actual on-disk collisions).
- **Two repos with the same derived `Name` in different groups** → valid (each lives at a different path; fzf disambiguates with `[group]` suffix).

### URL parsing

URLs are parsed by `repos.DeriveName` and `repos.DeriveOrg`:

| URL | DeriveName | DeriveOrg |
|---|---|---|
| `git@github.com:sahil87/hop.git` | `hop` | `sahil87` |
| `https://github.com/sahil87/hop.git` | `hop` | `sahil87` |
| `git@gitlab.com:org/group/sub/proj.git` | `proj` | `org/group/sub` |
| `https://github.com/sahil87/hop` (no `.git`) | `hop` | `sahil87` |
| `file:///tmp/local-repo.git` | `local-repo` | `tmp` |
| `plain-name` | `plain-name` | `""` |

Algorithm:

1. Strip a trailing `.git` if present.
2. SSH form (`git@host:path`): take the substring after the first `:`.
3. HTTPS / scheme form (`scheme://host/path`): take the substring after `host/`.
4. The last `/`-separated component is `Name`.
5. Everything before that last `/` is `Org` (may contain `/` for nested GitLab groups; preserved as nested directory structure on disk).

URLs that don't match these patterns parse permissively — `Name` becomes the last path component and `Org` is empty. The downstream `git clone` is what validates URL semantics.

### Empty / placeholder groups

Both forms are valid:

- **Flat list with no entries**: `default: []`
- **Map with `dir:` but no `urls:`**: `experiments: { dir: ~/code/experiments }`

The latter is useful as a `--group` target for `hop clone <url>` before any URLs have been registered.

### Loading semantics

- The YAML is parsed using `gopkg.in/yaml.v3` (per Constitution Principle IV — Wrap, Don't Reinvent).
- Group source order is preserved on `cfg.Groups`. `hop ls` and the bare-form picker reflect this order.
- **Empty file** (zero bytes) → `&Config{CodeRoot: "~", Groups: nil}`, no error. `hop ls` prints nothing; matchers return zero candidates.
- **Comments-only / null content** → same as empty.
- **Malformed YAML** → `hop: parse <path>: <yaml.v3 error with line>`. Exit 1.
- **Non-mapping top-level** → `hop: parse <path>: top-level must be a mapping`.

### Permissive on URL contents

Anything is accepted as a URL string (subject to URL uniqueness validation). A YAML-valid but semantically odd entry like `- not a url` parses fine; `Name` becomes `not a url`. Errors surface at `git clone` invocation time, not load time.

> **GIVEN** `hop.yaml` contains:
>
> ```yaml
> repos:
>   default:
>     - not a url
> ```
>
> **WHEN** any subcommand loads it
> **THEN** the load succeeds (YAML is valid, schema validates)
> **AND** the repo's name is derived as `not a url`
> **AND** clone-style operations on this repo will fail at git invocation time, not load time

## `hop config init`

Bootstrap a starter `hop.yaml`.

### Write target

Uses `ResolveWriteTarget()` (same search order as `Resolve()`, but no `os.Stat` checks — returns the path that *would* be used):

1. If `$HOP_CONFIG` is set: write to that path. (User has declared intent.)
2. Else if `$XDG_CONFIG_HOME` is set: write to `$XDG_CONFIG_HOME/hop/hop.yaml`.
3. Else: write to `$HOME/.config/hop/hop.yaml`.

### Behavior

- If the target file already exists: refuse to overwrite. Print to stderr: `hop config init: <path> already exists. Delete it first or set $HOP_CONFIG to a different path.` Exit 1. The existing file is untouched.
- If the parent directory doesn't exist: create it (`os.MkdirAll`, mode 0755).
- Write the embedded starter content. **Mode 0644.**
- Print to stdout: `Created <path>`.
- Print to stderr: `Edit the file to add your repos. Tip: set $HOP_CONFIG in your shell rc to point at a version-tracked location (a git-tracked dotfile, Dropbox, etc.) so this config moves with you across machines.`
- Exit 0.

### Embedded starter content

Stored at `src/internal/config/starter.yaml`, embedded via `//go:embed`. Self-bootstrapping — points at this repo so a fresh user can `hop` (fzf shows one entry) or `hop clone hop` immediately. Also serves as a copy-pasteable example of the grouped schema.

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

The starter parses cleanly under the schema validator (verified by `TestStarterParses` in `config_test.go`).

### Scenarios

> **GIVEN** `$HOP_CONFIG` is unset, `~/.config/hop/hop.yaml` does not exist
> **WHEN** I run `hop config init`
> **THEN** `~/.config/hop/hop.yaml` is created with mode 0644 containing the starter content
> **AND** stdout shows `Created /Users/sahil/.config/hop/hop.yaml`
> **AND** exit code is 0

> **GIVEN** the same setup but `$XDG_CONFIG_HOME=/Users/sahil/.cfg`
> **WHEN** I run `hop config init`
> **THEN** `/Users/sahil/.cfg/hop/hop.yaml` is created (parent dirs created if absent)

> **GIVEN** `$HOP_CONFIG=/tmp/test.yaml` is set, `/tmp/test.yaml` does not exist
> **WHEN** I run `hop config init`
> **THEN** `/tmp/test.yaml` is created (parent `/tmp` already exists)

> **GIVEN** `~/.config/hop/hop.yaml` already exists
> **WHEN** I run `hop config init`
> **THEN** stderr shows the "already exists" message
> **AND** exit code is 1
> **AND** the existing file is unmodified

## `hop config where`

Print the resolved config write target on stdout. Renamed from v0.0.1's `hop config path` for voice-fit consistency with `hop where`.

### Behavior

- Run the same `ResolveWriteTarget()` order as `init`.
- Print the path that *would* be used to stdout (whether or not the file exists).
- Exit 0 if a path resolves; exit 1 if nothing resolves at all (extremely rare — requires `$HOP_CONFIG` unset, `$XDG_CONFIG_HOME` unset, and `$HOME` unset).
- Never errors on missing file — it's a debug aid, not a load.

> **NOTE**: This subcommand does NOT trigger the "$HOP_CONFIG set but file missing" hard error — `ResolveWriteTarget()` doesn't `os.Stat` candidate 1.

### Scenarios

> **GIVEN** `$HOP_CONFIG=/tmp/foo.yaml`
> **WHEN** I run `hop config where`
> **THEN** stdout is `/tmp/foo.yaml`
> **AND** exit code is 0 (regardless of whether `/tmp/foo.yaml` exists)

> **GIVEN** `$HOP_CONFIG` is unset, `$XDG_CONFIG_HOME=/Users/sahil/.config`
> **WHEN** I run `hop config where`
> **THEN** stdout is `/Users/sahil/.config/hop/hop.yaml`
> **AND** exit code is 0

> **GIVEN** all env vars unset, `$HOME=/Users/sahil`
> **WHEN** I run `hop config where`
> **THEN** stdout is `/Users/sahil/.config/hop/hop.yaml`
> **AND** exit code is 0

## Comment-Preserving Writes (`hop clone <url>` auto-registration)

When `hop clone <url>` appends a URL to a group, the write goes through `internal/yamled.AppendURL`:

- Reads the file as a `*yaml.Node` tree (preserving comments).
- Navigates `repos.<group>`; appends a new scalar to the sequence body (flat group) or the `urls:` child sequence (map-shaped group).
- Marshals and atomically writes back via temp file + `os.Rename` in the same directory.
- **Comment preservation is the contract.** Indentation is normalized to yaml.v3 defaults — byte-perfect formatting is *not* guaranteed.
- `ErrGroupNotFound` is returned (wrapped via `%w`) when the named group is absent.

> **GIVEN** `hop.yaml` has the `default` flat group with two existing URLs and a head comment
> **WHEN** `hop clone git@github.com:user/new.git` runs successfully
> **THEN** the new URL is appended after the existing two
> **AND** comments and blank-line structure around `default:` are preserved
> **AND** the write is atomic (temp file + rename in same dir)

## Design Decisions

1. **No caching.** The YAML is re-read on every invocation. Per Constitution Principle II — "No Database." The file is small (typically <1KB); re-parsing is cheap.
2. **Hard error on `$HOP_CONFIG` set but file missing.** Setting an env var is intent. Silent fallthrough would mask config bugs. Diverges from "permissive search" in favor of fail-fast.
3. **Mode 0644 for the starter.** The file contains repo paths and public git URLs — no credentials. Treating it as sensitive (0600) would be theater.
4. **Grouped schema instead of flat `dir → URLs`.** v0.0.1's flat schema (`~/code/sahil87: [...]`) coupled "where on disk" to "logical grouping" — every URL had to live under a directory key. The grouped schema lets `default:` use convention-driven paths (`<code_root>/<org>/<name>`) while named groups (`vendor:`, `experiments:`) override on a case-by-case basis. This reduces line noise for the common case (most repos go under `~/code/<org>/<name>`).
5. **`code_root` defaults to `~`, not `~/code`.** The default is the most permissive — users on machines without a `~/code/` convention still get a sensible fallback (`~/<org>/<name>`). The starter `hop.yaml` sets `code_root: ~/code` explicitly; users who want the bare-`~` default can omit the field or delete that line.
6. **`$HOP_CONFIG` replaces `$REPOS_YAML`.** No fallback to the legacy env var. The rename is a clean break — keeping both would have meant carrying two precedence chains forever for negligible benefit (the legacy env var was personal infrastructure, not a public contract).
7. **Schema validator is strict on top-level and group keys.** Unknown top-level fields, unknown group fields, and unknown `config.*` fields all hard-fail at load time. This catches typos (`Repos:`, `urs:`, `dir_:`) immediately rather than letting them silently no-op.
8. **Self-bootstrapping starter content includes this repo's URL.** Trade-off: hardcodes one URL into the binary; benefit: a fresh user runs `hop` immediately after init and sees something work. The URL is also the canonical "how do I clone the tool that just bootstrapped me" reference.
9. **Comment-preserving writes via a separate `internal/yamled` package.** `internal/config` validates and consumes; `yamled` produces a node tree, navigates, mutates, writes. Different responsibilities (validator vs. mutator) — keeping them separate lets each be tested independently and avoids tangling load-time validation with write-time edits.
