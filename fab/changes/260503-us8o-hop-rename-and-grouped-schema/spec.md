# Spec: Rename to `hop` and Adopt Grouped Schema

**Change**: 260503-us8o-hop-rename-and-grouped-schema
**Created**: 2026-05-04
**Affected memory**:
- `docs/memory/cli/subcommands.md`
- `docs/memory/cli/match-resolution.md`
- `docs/memory/config/yaml-schema.md`
- `docs/memory/config/search-order.md`
- `docs/memory/config/init-bootstrap.md`
- `docs/memory/architecture/package-layout.md`
- `docs/memory/architecture/wrapper-boundaries.md`
- `docs/memory/build/local.md`

## Non-Goals

- `hop sync` / `hop autosync` — schema leaves room (per-group `dir`, group as a unit) but no `sync` verb is shipped.
- `hop features` (cross-repo grep / search) — separate change.
- gh-style URL shorthand (`hop clone foo/bar` → GitHub URL) — deferred.
- Per-group config beyond `dir` (`sync_strategy`, `auto_pull`, `exclude`) — schema reserves no extra keys; `additional fields → load error`.
- Migration command from `repos.yaml` → `hop.yaml` — explicitly out of scope.
- Backward-compatibility fallback to `repos.yaml`, `$REPOS_YAML`, `$XDG_CONFIG_HOME/repo/`, or `~/.config/repo/` — clean rename, no fallback.
- Go module path rename (`github.com/sahil87/repo` → `github.com/sahil87/hop`) — module path stays. Binary name and user-facing surface change; module is internal.
- Windows support — already excluded by Constitution.

## Binary: Naming and Module Layout

### Requirement: Binary Renamed to `hop`
The compiled binary SHALL be named `hop`. The Go package and entrypoint directory SHALL move from `src/cmd/repo/` to `src/cmd/hop/`. The Go module path `github.com/sahil87/repo` SHALL remain unchanged for v1 — module-path rename is out of scope.

#### Scenario: Build produces hop binary
- **GIVEN** the source tree with `src/cmd/hop/`
- **WHEN** `just build` runs
- **THEN** `bin/hop` is produced at the repo root
- **AND** `bin/repo` is NOT produced

#### Scenario: Install copies hop to ~/.local/bin
- **WHEN** `just install` runs
- **THEN** `~/.local/bin/hop` exists
- **AND** `~/.local/bin/repo` is NOT created or modified by this script

### Requirement: All Error Messages, Help Text, and Source Comments Use `hop`
All user-visible strings (cobra `Use:`, `Short:`, `Long:`, error prefixes, install hints) SHALL use `hop` instead of `repo`. Source comments and doc strings SHOULD reference `hop` (`repo` only when describing the legacy state).

#### Scenario: Error prefixes use hop
- **GIVEN** a missing `fzf` binary on PATH
- **WHEN** an ambiguous match triggers fzf
- **THEN** stderr contains `hop: fzf is not installed.`
- **AND** does NOT contain `repo: fzf is not installed.`

#### Scenario: Version flag prints version
- **WHEN** `hop --version` runs
- **THEN** stdout is a single line containing the version string
- **AND** exit code is 0

## CLI: Subcommand Surface

### Requirement: Subcommand `path` Renamed to `where`
The `path` subcommand SHALL be renamed to `where` with no functional change. There SHALL be no `path` alias — it is removed. The handler `resolveAndPrint` is reused.

#### Scenario: `hop where <name>` resolves and prints
- **GIVEN** `hop.yaml` lists a repo named `outbox`
- **WHEN** `hop where outbox` runs
- **THEN** stdout is the absolute path to the `outbox` repo
- **AND** exit code is 0

#### Scenario: `hop path` is unknown
- **WHEN** `hop path outbox` runs
- **THEN** cobra rejects the command (exit non-zero)
- **AND** stderr indicates an unknown subcommand

### Requirement: Subcommand `config path` Renamed to `config where`
The nested `config path` subcommand SHALL be renamed to `config where`. The handler `ResolveWriteTarget` is reused. There SHALL be no `config path` alias.

#### Scenario: `hop config where` prints write target
- **GIVEN** `$HOP_CONFIG=/tmp/foo.yaml`
- **WHEN** `hop config where` runs
- **THEN** stdout is `/tmp/foo.yaml`
- **AND** exit code is 0

### Requirement: All Other Subcommand Names Preserved
The subcommands `cd`, `code`, `open`, `clone`, `ls`, `shell-init`, `config init`, `--help`/`-h`/`help`, and `--version`/`-v` SHALL keep their names from v0.0.1.

#### Scenario: `hop ls` lists repos
- **GIVEN** `hop.yaml` has at least one repo
- **WHEN** `hop ls` runs
- **THEN** stdout shows aligned `name<spaces>path` rows (one per repo)
- **AND** exit code is 0

### Requirement: New Global Flag `-C <name>` Executes a Command in the Resolved Repo's Directory
A new persistent flag `-C <name>` (long form: `--in <name>` or none — see Design Decisions) SHALL be added to the root command. When `-C <name>` is present, the binary resolves `<name>` to a repo path and executes the remaining argv as a subprocess with `Dir` set to that path. The subprocess SHALL inherit stdin, stdout, and stderr from the parent. The exit code of the subprocess SHALL be propagated to the parent's exit code. Cobra subcommand parsing SHALL be short-circuited so that `<cmd...>` is treated as a literal argv tuple, not as a `hop` subcommand.

The implementation technique SHALL use a pre-Execute argv inspection: the binary inspects `os.Args` early (before `cmd.Execute()`), detects `-C <name>` (or `-C=<name>` form), splits argv into `[hopArgs..., resolveTarget, childCmd...]`, resolves `<name>` to a path, and invokes `proc.Run` (or a similar `internal/proc` entrypoint that does NOT capture stdout/stderr — see internal/proc requirements below) with `Dir` set. The remaining argv is forwarded as-is to the child.

#### Scenario: -C executes child command in the resolved directory
- **GIVEN** `hop.yaml` lists a repo named `outbox` resolving to `~/code/sahil87/outbox`
- **AND** the directory `~/code/sahil87/outbox` exists
- **WHEN** `hop -C outbox pwd` runs
- **THEN** the subprocess is `pwd` invoked with `Dir=~/code/sahil87/outbox`
- **AND** stdout from `pwd` (the absolute path) is printed
- **AND** exit code matches `pwd`'s exit code (0)

#### Scenario: -C with unknown name fails before exec
- **GIVEN** `hop.yaml` does not list a repo matching `nonexistent`
- **WHEN** `hop -C nonexistent ls` runs
- **THEN** stderr contains a resolution error
- **AND** the child `ls` is NOT executed
- **AND** exit code is 1

#### Scenario: -C propagates child non-zero exit
- **GIVEN** `hop -C outbox false`
- **WHEN** the command runs
- **THEN** exit code is 1 (matches `false`'s exit)

#### Scenario: -C with multi-word command preserves argv
- **GIVEN** `hop -C outbox git status --short`
- **WHEN** the command runs
- **THEN** the subprocess argv is `[git, status, --short]`
- **AND** the working directory of the subprocess is `~/code/sahil87/outbox`

#### Scenario: -C with no command argument errors
- **GIVEN** `hop -C outbox`
- **WHEN** the command runs (no command words after `<name>`)
- **THEN** stderr shows: `hop: -C requires a command to execute. Usage: hop -C <name> <cmd>...`
- **AND** exit code is 2

#### Scenario: -C with no name argument errors
- **GIVEN** `hop -C` (no value)
- **WHEN** the command runs
- **THEN** stderr shows a usage error
- **AND** exit code is 2

### Requirement: `hop clone <url>` Auto-Registers and Prints Landed Path
When the single positional argument to `hop clone` is detected as a URL (per the URL-detection rule below), the binary SHALL:

1. Derive `name` and `org` from the URL.
2. Determine the target group from `--group <name>` (default: `default`).
3. Resolve the repo's on-disk path using the group's resolution rule (see Schema requirements).
4. Clone the URL into that path via `git clone <url> <path>` (same `proc.Run` invocation as registry-driven clone).
5. Append the URL to the target group's `urls:` list in the resolved `hop.yaml` file using comment-preserving YAML write-back (see Internal: YAML Write-Back).
6. Print the resolved absolute path to stdout (a single line, like `hop where <name>`).

The flags `--no-add`, `--no-cd`, `--name <override>`, and `--group <name>` SHALL be supported. The flag semantics are:

- `--group <name>`: register under that group instead of `default`.
- `--no-add`: skip the YAML write-back. Clone still occurs; path is still printed.
- `--no-cd`: suppress the path print on stdout. Clone and YAML write-back still occur. The shell shim relies on the absence of stdout to skip its `cd`.
- `--name <override>`: override the URL-derived name. Affects the on-disk path component and the entry written to YAML — but the URL stored is verbatim.

URL detection rule:
```
contains "://" OR
(contains "@" AND contains ":")
  → URL
otherwise
  → name (existing registry-driven path)
```

If `<group>` does not exist in the loaded config: print `hop: no '<group>' group in <config-path>. Pass --group <existing-group> or add '<group>:' to your config.` to stderr and exit 1. The default-group case uses literal `default` in this message.

If the path conflict exists (`stateAlreadyCloned`, `statePathExistsNotGit`) the binary SHALL behave as registry-driven clone does today, except that:
- For `stateAlreadyCloned`, `hop clone <url>` still appends to YAML (unless `--no-add`) and still prints the path (unless `--no-cd`). This matches the "auto-register an already-cloned repo into the index" use case.
- For `statePathExistsNotGit`, the existing error message is emitted, no YAML write occurs, exit 1.

#### Scenario: Ad-hoc URL clone registers and lands user
- **GIVEN** `hop.yaml` contains a `default` group with one URL
- **AND** the URL `git@github.com:sahil87/outbox.git` is not in the config
- **AND** `~/code/sahil87/outbox` does not exist
- **WHEN** `hop clone git@github.com:sahil87/outbox.git` runs
- **THEN** `git clone git@github.com:sahil87/outbox.git ~/code/sahil87/outbox` runs (status to stderr)
- **AND** `hop.yaml` is rewritten with the URL appended to `default.urls` (or to `default` if it's a flat list — see Schema)
- **AND** stdout is `~/code/sahil87/outbox` (or the expanded equivalent)
- **AND** existing comments in `hop.yaml` are preserved
- **AND** exit code is 0

#### Scenario: Ad-hoc URL clone with --group <existing>
- **GIVEN** `hop.yaml` contains a `vendor` group with `dir: ~/vendor`
- **WHEN** `hop clone --group vendor git@github.com:vendor/tool.git` runs
- **THEN** the clone target is `~/vendor/tool`
- **AND** the URL is appended to the `vendor` group's `urls:` list
- **AND** stdout is the absolute path

#### Scenario: Ad-hoc URL clone with --group <missing>
- **GIVEN** `hop.yaml` does not contain a group `experiments`
- **WHEN** `hop clone --group experiments git@github.com:foo/bar.git` runs
- **THEN** stderr shows `hop: no 'experiments' group in <config-path>. Pass --group <existing-group> or add 'experiments:' to your config.`
- **AND** exit code is 1
- **AND** the YAML file is unchanged
- **AND** no `git clone` is invoked

#### Scenario: Ad-hoc URL clone defaults to default group; missing default → error
- **GIVEN** `hop.yaml` has groups `vendor` and `experiments` but no `default` group
- **WHEN** `hop clone git@github.com:sahil87/foo.git` runs (no `--group`)
- **THEN** stderr shows `hop: no 'default' group in <config-path>. Pass --group <existing-group> or add 'default:' to your config.`
- **AND** exit code is 1

#### Scenario: --no-add skips YAML write
- **WHEN** `hop clone --no-add git@github.com:sahil87/outbox.git` runs
- **THEN** `git clone` runs normally
- **AND** stdout is the resolved path
- **AND** `hop.yaml` is NOT modified

#### Scenario: --no-cd suppresses stdout path
- **WHEN** `hop clone --no-cd git@github.com:sahil87/outbox.git` runs
- **THEN** `git clone` runs
- **AND** the URL is appended to `default.urls` in `hop.yaml`
- **AND** stdout is empty
- **AND** stderr still shows the `clone:` status line

#### Scenario: --name override changes derived name
- **GIVEN** `hop clone --name my-fork git@github.com:upstream/repo.git`
- **WHEN** the command runs
- **THEN** the on-disk target is `<code_root>/upstream/my-fork`
- **AND** the YAML entry written stores the URL `git@github.com:upstream/repo.git` (verbatim)
- **AND** stdout is `<code_root>/upstream/my-fork`

> **Note on `--name` and YAML round-trip**: The on-disk name is derived from the URL by default, but `--name` overrides it for path resolution. On subsequent loads, the YAML entry is just the URL — there is no per-entry `name` field. This means `hop ls` after `--name`-override clone shows the URL-derived name, NOT the override. The override is a one-shot landing trick for this session, not a persistent rename. This is by design — schema simplicity outweighs a rare use case.

#### Scenario: stateAlreadyCloned with ad-hoc URL still registers
- **GIVEN** `~/code/sahil87/outbox` exists with a `.git` directory
- **AND** `git@github.com:sahil87/outbox.git` is not in `hop.yaml`
- **WHEN** `hop clone git@github.com:sahil87/outbox.git` runs
- **THEN** stderr shows `skip: already cloned at ~/code/sahil87/outbox`
- **AND** the URL is appended to `default.urls` in `hop.yaml` (registers the existing checkout)
- **AND** stdout is the resolved path
- **AND** exit code is 0

#### Scenario: statePathExistsNotGit with ad-hoc URL fails without write
- **GIVEN** `~/code/sahil87/outbox` exists but is NOT a git repo
- **WHEN** `hop clone git@github.com:sahil87/outbox.git` runs
- **THEN** stderr shows `hop clone: ~/code/sahil87/outbox exists but is not a git repo`
- **AND** `hop.yaml` is NOT modified
- **AND** exit code is 1

### Requirement: `hop clone <name>` Behavior Preserved
When the positional argument is detected as a `<name>` (no `://`, no `@:` pair), `hop clone` SHALL behave exactly as `repo clone` did in v0.0.1: resolve via match-or-fzf, clone if missing, skip if `.git` exists, error on conflict.

#### Scenario: Registry-driven clone behaves as v0.0.1
- **GIVEN** `hop.yaml` contains a repo named `outbox` resolving to a missing path
- **WHEN** `hop clone outbox` runs
- **THEN** `git clone <url> <path>` runs
- **AND** stderr shows `clone: <url> → <path>`
- **AND** exit code matches git's exit

### Requirement: `hop clone --all` Preserved
`hop clone --all` SHALL behave as v0.0.1: iterate the full list, clone missing, skip cloned, summary line. The flag does NOT modify `hop.yaml`.

## CLI: Match Resolution

### Requirement: Match Algorithm Preserves Substring-on-Name Semantics
Match resolution (used by `hop`, `hop where`, `hop code`, `hop open`, `hop cd`, `hop clone`, `hop -C`) SHALL match exactly v0.0.1's algorithm: case-insensitive substring on `Name`. Group context does NOT affect matching — a name can match across groups.

### Requirement: Multi-Group Picker Lines Show Group Context
When two or more repos share a derived `Name` across groups, the fzf picker SHALL include the group name in the displayed column so the user can disambiguate. The display format SHALL be `name [group]\tpath\turl` for repos that share a name with another repo, and `name\tpath\turl` for unique names. The fzf flags continue to display only column 1 (`--with-nth 1 --delimiter '\t'`).

#### Scenario: Disambiguation when names collide
- **GIVEN** `hop.yaml` has `default` containing `git@github.com:foo/repo.git` and `vendor` containing `git@github.com:bar/repo.git`
- **WHEN** `hop where repo` runs
- **THEN** the match list has 2 entries
- **AND** the fzf picker displays:
  - `repo [default]`
  - `repo [vendor]`
- **AND** `--query repo` is passed to fzf

#### Scenario: No collision, no group suffix
- **GIVEN** `hop.yaml` has `default` containing `git@github.com:foo/widget.git` (only one repo named `widget`)
- **WHEN** `hop where widget` runs
- **THEN** match resolves directly (1 candidate, no fzf invocation)
- **AND** stdout is the absolute path

### Requirement: Sort Order Preserved Within Group, Then Across Groups
`FromConfig` SHALL produce a deterministic order: groups in YAML appearance order (preserved via the `yaml.Node`-based load), then URLs within each group in the order written. (Today's load uses `map[string][]string` which discards order — the new schema MUST round-trip via `yaml.Node` to preserve group order from the source file.) Within a group's `urls:` list, the source order is the output order.

#### Scenario: hop ls preserves YAML order
- **GIVEN** `hop.yaml` contains groups in order `default`, `vendor`, `experiments`
- **AND** each group's `urls:` list is in a specific order
- **WHEN** `hop ls` runs
- **THEN** the output rows are in YAML order — `default`'s repos first (in source order), then `vendor`'s, then `experiments`'

## Config: Schema (Option 4 — Named Groups)

### Requirement: Top-Level Schema is `config:` (optional) + `repos:` (required)
`hop.yaml` SHALL have a top-level YAML map with two keys:
- `config:` (optional) — currently has one field, `code_root`. Other top-level config keys SHALL be rejected (load error: `hop: parse <path>: unknown config field '<name>'`).
- `repos:` (required) — map of `group_name → group_body`.

If `repos:` is absent, the load SHALL error: `hop: parse <path>: missing required field 'repos'`.

If the top-level map contains keys other than `config` and `repos`, the load SHALL error: `hop: parse <path>: unknown top-level field '<name>'. Valid: 'config', 'repos'.`

#### Scenario: Minimal valid file
```yaml
repos:
  default:
    - git@github.com:sahil87/hop.git
```
- **WHEN** loaded
- **THEN** load succeeds
- **AND** one repo is produced: name `hop`, path `~/code/sahil87/hop` (default `code_root` = `~`, with `~/sahil87/hop` if `code_root` is `~` — see Path Resolution)

#### Scenario: Missing repos field
```yaml
config:
  code_root: ~/code
```
- **WHEN** loaded
- **THEN** load fails with `hop: parse <path>: missing required field 'repos'`

#### Scenario: Unknown top-level field
```yaml
repos:
  default: []
servers:
  - foo
```
- **WHEN** loaded
- **THEN** load fails with `hop: parse <path>: unknown top-level field 'servers'. Valid: 'config', 'repos'.`

### Requirement: `config.code_root` Defaults to `~`
`config.code_root` is optional. If absent, the default is `~` (literal tilde, expanded to `$HOME` at load time). If `config:` block is absent entirely, default is also `~`. If `code_root` is set, it MAY be absolute (`/srv/code`), `~`-prefixed (`~/code`), or a relative path. Relative paths SHALL be resolved relative to the user's `$HOME`.

#### Scenario: Default code_root
```yaml
repos:
  default:
    - git@github.com:foo/bar.git
```
- **WHEN** loaded
- **THEN** `code_root` is `~` (expands to `$HOME`)
- **AND** `bar`'s path is `$HOME/foo/bar`

#### Scenario: Absolute code_root
```yaml
config:
  code_root: /srv/code
repos:
  default:
    - git@github.com:foo/bar.git
```
- **WHEN** loaded
- **THEN** `bar`'s path is `/srv/code/foo/bar`

### Requirement: Group Body — Two Shapes
A group body MAY be either:
- **Flat list (convention-driven):** a YAML list of URL strings. Each URL resolves to `<code_root>/<org-from-url>/<name-from-url>`.
- **Map with `dir` and `urls`:** a YAML map with optional `dir` and required `urls`. Each URL resolves to `<dir>/<name-from-url>`.

Other YAML shapes (scalar, list-of-non-strings, map with unknown keys) SHALL produce a load error:
- Map with keys other than `dir` and `urls`: `hop: parse <path>: group '<name>' has unknown field '<key>'. Valid: 'dir', 'urls'.`
- Group value not a list or a map: `hop: parse <path>: group '<name>' must be a list of URLs or a map with 'dir' and 'urls'.`

#### Scenario: Flat list group
```yaml
repos:
  default:
    - git@github.com:sahil87/hop.git
    - git@github.com:sahil87/wt.git
```
- **WHEN** loaded with `code_root: ~/code`
- **THEN** two repos are produced:
  - `hop` at `~/code/sahil87/hop`
  - `wt` at `~/code/sahil87/wt`

#### Scenario: Map group with dir
```yaml
config:
  code_root: ~/code
repos:
  vendor:
    dir: ~/vendor
    urls:
      - git@github.com:some-vendor/their-tool.git
```
- **WHEN** loaded
- **THEN** one repo is produced: `their-tool` at `~/vendor/their-tool`
- **AND** `code_root` is irrelevant for vendor entries

#### Scenario: Map group without urls
```yaml
repos:
  experiments:
    dir: ~/code/experiments
```
- **WHEN** loaded
- **THEN** load succeeds
- **AND** zero repos are produced from `experiments`
- **AND** `experiments` is recognized as a valid group target for `hop clone <url> --group experiments`

#### Scenario: Empty flat list
```yaml
repos:
  default: []
```
- **WHEN** loaded
- **THEN** load succeeds
- **AND** zero repos are produced

#### Scenario: Map group with unknown key
```yaml
repos:
  vendor:
    dir: ~/vendor
    sync_strategy: aggressive
    urls:
      - git@github.com:foo/bar.git
```
- **WHEN** loaded
- **THEN** load fails with `hop: parse <path>: group 'vendor' has unknown field 'sync_strategy'. Valid: 'dir', 'urls'.`

### Requirement: Relative `dir` Resolves to `code_root`
A `dir:` value with no leading `/` and no leading `~` SHALL be interpreted as relative to `code_root` (after `code_root` is itself expanded). Examples: `dir: vendor` with `code_root: ~/code` → `$HOME/code/vendor`. Empty string `dir: ""` is rejected as invalid (`hop: parse <path>: group '<name>' has empty 'dir'`).

#### Scenario: Relative dir uses code_root
```yaml
config:
  code_root: ~/code
repos:
  experiments:
    dir: experiments
    urls:
      - git@github.com:foo/sandbox.git
```
- **WHEN** loaded
- **THEN** `sandbox`'s path is `$HOME/code/experiments/sandbox`

### Requirement: Group Names Match Identifier Pattern
A group name SHALL match the regex `^[a-z][a-z0-9_-]*$`. Names that don't match SHALL produce a load error: `hop: parse <path>: invalid group name '<name>'. Group names must match ^[a-z][a-z0-9_-]*$`.

#### Scenario: Invalid group name
```yaml
repos:
  My Group:
    - git@github.com:foo/bar.git
```
- **WHEN** loaded
- **THEN** load fails with the invalid group name error

#### Scenario: Valid edge-case names
```yaml
repos:
  a:
    - git@github.com:x/a.git
  group_1:
    - git@github.com:x/b.git
  vendor-stuff:
    - git@github.com:x/c.git
```
- **WHEN** loaded
- **THEN** load succeeds (all three names are valid)

### Requirement: URL Uniqueness Across Groups
The same URL string SHALL NOT appear in two groups. Duplicate URLs across groups produce a load error: `hop: parse <path>: URL '<url>' appears in groups '<a>' and '<b>'; a URL must belong to exactly one group.` Duplicates within a single group SHALL also error: `hop: parse <path>: URL '<url>' is listed twice in group '<name>'.`

#### Scenario: URL in two groups errors
```yaml
repos:
  default:
    - git@github.com:foo/bar.git
  vendor:
    dir: ~/vendor
    urls:
      - git@github.com:foo/bar.git
```
- **WHEN** loaded
- **THEN** load fails with the URL-in-two-groups error

### Requirement: Two Groups May Share `dir`
Two groups MAY have the same `dir` value. Resolution is per-URL, so collisions only matter if two URLs resolve to the same on-disk path — which is caught by the URL-uniqueness rule (since the same URL can't appear twice).

#### Scenario: Shared dir is valid
```yaml
repos:
  scratch:
    dir: ~/scratch
    urls:
      - git@github.com:foo/a.git
  experiments:
    dir: ~/scratch
    urls:
      - git@github.com:foo/b.git
```
- **WHEN** loaded
- **THEN** load succeeds
- **AND** two repos are produced: `a` at `~/scratch/a`, `b` at `~/scratch/b`

### Requirement: Two Repos with Same Derived Name in Different Groups Are Valid
Two URLs that produce the same `name` (last URL component minus `.git`) MAY exist in different groups, yielding two repos with the same `Name` but different `Path` and `Group` fields.

(Match resolution behavior is covered in the Match Resolution requirements above.)

### Requirement: URL Parsing for `name` and `org`
The URL-derivation algorithm SHALL be:
1. Strip a trailing `.git` if present.
2. SSH form (`git@host:path`): split on the first `:`, take the part after.
3. HTTPS form (`https://host/path`): split on `/`, take everything after `host/`.
4. The last `/`-separated component is `name`.
5. Everything before the last component is `org` (may contain `/` for nested GitLab groups; this nests on disk).

For URLs that don't fit either form (e.g., a plain path), the entire input minus a trailing `.git` and minus everything before the last `/` is `name`; `org` is empty (`""`). When `org` is empty and the group is convention-driven, the path is `<code_root>/<name>` (no org component).

#### Scenario: Standard SSH URL
- **GIVEN** URL `git@github.com:sahil87/hop.git`
- **THEN** name=`hop`, org=`sahil87`

#### Scenario: HTTPS URL
- **GIVEN** URL `https://github.com/sahil87/hop.git`
- **THEN** name=`hop`, org=`sahil87`

#### Scenario: Nested GitLab path
- **GIVEN** URL `git@gitlab.com:org/group/sub/proj.git`
- **THEN** name=`proj`, org=`org/group/sub`
- **AND** with `code_root: ~/code` and a flat-list group, on-disk path is `~/code/org/group/sub/proj`

#### Scenario: No .git suffix
- **GIVEN** URL `https://github.com/sahil87/hop`
- **THEN** name=`hop`, org=`sahil87`

### Requirement: Path Resolution
Per-repo path resolution SHALL be:
- Group has `dir`: `<expanded-dir>/<name>`. `org` is ignored.
- Group is flat (no `dir`): `<expanded-code_root>/<org>/<name>`.
- If `org` is empty: `<expanded-code_root>/<name>`.

`~` expansion is the same rule as v0.0.1: `~` alone → `$HOME`; `~/...` → `$HOME/...`; `~user` → verbatim (no Linux user lookup); leading `~` only is special.

## Config: File Locations and Env Var

### Requirement: Config File is `hop.yaml`; Env Var is `$HOP_CONFIG`
The config file SHALL be named `hop.yaml`. The env var SHALL be `$HOP_CONFIG`. Search paths:
1. `$HOP_CONFIG` if set and non-empty
2. `$XDG_CONFIG_HOME/hop/hop.yaml` if `$XDG_CONFIG_HOME` is set
3. `$HOME/.config/hop/hop.yaml`

The `$HOP_CONFIG` set-but-missing hard-error semantics SHALL match v0.0.1's `$REPOS_YAML` behavior — error message uses `$HOP_CONFIG` and the file name `hop.yaml` instead.

There SHALL be no fallback to `repos.yaml`, `$REPOS_YAML`, `$XDG_CONFIG_HOME/repo/`, or `$HOME/.config/repo/`.

#### Scenario: $HOP_CONFIG set but missing
- **GIVEN** `HOP_CONFIG=/nonexistent/path.yaml`
- **WHEN** any subcommand requiring config runs
- **THEN** stderr shows `hop: $HOP_CONFIG points to /nonexistent/path.yaml, which does not exist. Set $HOP_CONFIG to an existing file or unset it.`
- **AND** exit code is 1

#### Scenario: All resolve to nothing
- **GIVEN** `$HOP_CONFIG`, `$XDG_CONFIG_HOME` are unset
- **AND** `~/.config/hop/hop.yaml` does not exist
- **WHEN** any subcommand runs
- **THEN** stderr shows `hop: no hop.yaml found. Set $HOP_CONFIG to a tracked file (e.g., a Dropbox path or a git-tracked dotfile), or run 'hop config init' to bootstrap one at $XDG_CONFIG_HOME/hop/hop.yaml.`
- **AND** exit code is 1

#### Scenario: $REPOS_YAML is ignored
- **GIVEN** `$REPOS_YAML=/some/path/repos.yaml` is set
- **AND** `$HOP_CONFIG` is unset
- **AND** `~/.config/hop/hop.yaml` does not exist
- **WHEN** any subcommand runs
- **THEN** stderr shows the "no hop.yaml found" message (no fallback to `$REPOS_YAML`)
- **AND** exit code is 1

### Requirement: `hop config init` Writes Embedded Starter
`hop config init` SHALL behave like `repo config init`: write the embedded starter content to `ResolveWriteTarget()`, refuse to overwrite existing files, mode 0644, parent dirs created. The error and success messages SHALL reference `hop` and `$HOP_CONFIG`.

The embedded starter content SHALL be the grouped-form starter:
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

The "Created <path>" stdout line and the stderr tip SHALL reference `$HOP_CONFIG` instead of `$REPOS_YAML`.

#### Scenario: hop config init in fresh environment
- **GIVEN** `$HOP_CONFIG` is unset, `~/.config/hop/hop.yaml` does not exist
- **WHEN** `hop config init` runs
- **THEN** `~/.config/hop/hop.yaml` is created with mode 0644 containing the new starter
- **AND** stdout is `Created /Users/<u>/.config/hop/hop.yaml`
- **AND** stderr contains `Edit the file to add your repos. Tip: set $HOP_CONFIG ...`
- **AND** exit code is 0

#### Scenario: hop config init refuses overwrite
- **GIVEN** `~/.config/hop/hop.yaml` already exists
- **WHEN** `hop config init` runs
- **THEN** stderr shows `hop config init: ~/.config/hop/hop.yaml already exists. Delete it first or set $HOP_CONFIG to a different path.`
- **AND** the file is unchanged
- **AND** exit code is 1

## CLI: Shell Integration

### Requirement: `hop shell-init zsh` Emits New Shim Format
`hop shell-init zsh` SHALL emit a zsh function (`hop()`) with bare-name dispatch, plus two alias functions (`h` and `hi`). The shim SHALL include cobra-generated zsh completion (output of `hop completion zsh`) inline OR via a runtime invocation comment. (Implementation: embed at build time using cobra's `GenZshCompletion` into a buffer captured by the `shell-init` handler.)

The shim's `hop()` function SHALL match exactly:
```sh
hop() {
  if [[ $# -eq 0 ]]; then
    command hop
    return $?
  fi
  case "$1" in
    cd|clone|where|ls|code|open|shell-init|config|--help|-h|--version|completion)
      _hop_dispatch "$@"
      ;;
    -*)
      command hop "$@"
      ;;
    *)
      _hop_dispatch cd "$1"
      ;;
  esac
}
```

The shim SHALL define `_hop_dispatch` (per the intake's listing). It SHALL define `h() { hop "$@"; }` and `hi() { command hop "$@"; }`.

#### Scenario: shell-init zsh emits expected functions
- **WHEN** `hop shell-init zsh` runs
- **THEN** stdout contains the literal `hop()` function definition with bare-name dispatch
- **AND** stdout contains `_hop_dispatch()`
- **AND** stdout contains `h() { hop "$@"; }`
- **AND** stdout contains `hi() { command hop "$@"; }`
- **AND** stdout contains the cobra-generated zsh completion (a `_hop` function)
- **AND** stdout contains `compdef _hop hop` or equivalent registration
- **AND** exit code is 0

#### Scenario: shell-init missing shell still errors
- **WHEN** `hop shell-init` runs (no shell argument)
- **THEN** stderr shows `hop shell-init: missing shell. Supported: zsh`
- **AND** exit code is 2

#### Scenario: shell-init unsupported shell still errors
- **WHEN** `hop shell-init bash` runs
- **THEN** stderr shows `hop shell-init: unsupported shell 'bash'. Supported: zsh`
- **AND** exit code is 2

### Requirement: Bare-Name Dispatch Lives in Shim Only
The binary SHALL NOT implement bare-name dispatch. The binary's bare form (`hop` with one positional `<name>`) continues to behave as `hop where <name>` (resolve and print). The binary continues to error/exit on `hop cd` (the binary form) with the existing "shell-only" hint, updated to reference `eval "$(hop shell-init zsh)"`.

#### Scenario: Bare hop <name> from binary prints path
- **GIVEN** `hop.yaml` contains a repo named `outbox`
- **WHEN** `command hop outbox` runs (bypassing the shim)
- **THEN** stdout is the absolute path
- **AND** exit code is 0

#### Scenario: hop cd from binary still errors
- **WHEN** `command hop cd outbox` runs (bypassing the shim)
- **THEN** stderr shows `hop: 'cd' is shell-only. Add 'eval "$(hop shell-init zsh)"' to your zshrc, or use: cd "$(hop where "<name>")"`
- **AND** exit code is 2

## Internal: YAML Write-Back

### Requirement: New `internal/yamled` Package for Comment-Preserving Write
A new internal package `src/internal/yamled/` SHALL be created. It SHALL expose at least:

```go
// AppendURL loads the YAML file at path as a yaml.Node tree, locates the
// `repos.<group>` node, appends `url` to its URLs list (handling both flat-list
// and map-with-urls shapes), and writes the result back to the same path.
// Comments in unmodified portions of the file are preserved. Indentation is
// normalized to yaml.v3's defaults on round-trip — comment preservation is the
// contract, byte-perfect formatting is not.
// Returns an error if the group does not exist, if the path is unwritable, or
// if the YAML is malformed.
func AppendURL(path, group, url string) error
```

The implementation SHALL:
1. Read the file with `os.ReadFile`.
2. Parse it with `yaml.Unmarshal` into a `*yaml.Node` (root document node).
3. Navigate to `repos.<group>`. If the group node is a sequence node, append a new scalar node. If it's a mapping node, navigate to its `urls` child (creating one if absent? — see Open Questions; v1: error if `urls` is absent in a map-shaped group).
4. Marshal the root node back via `yaml.Marshal(root)`.
5. Write to a temp file in the same directory, fsync, rename over the original (atomic write).

`yamled` SHALL NOT validate the schema — that's the caller's job. It only manipulates nodes.

#### Scenario: Append to flat-list group preserves comments
- **GIVEN** a `hop.yaml` with comments and a `default` flat list
- **WHEN** `yamled.AppendURL(path, "default", "git@github.com:x/y.git")` runs
- **THEN** the file is rewritten with the new URL appended at the end of `default`'s list
- **AND** all original comments are preserved verbatim

#### Scenario: Append to map-shaped group with urls
- **GIVEN** a `hop.yaml` with a `vendor` group having `dir:` and `urls:`
- **WHEN** `yamled.AppendURL(path, "vendor", "git@github.com:v/t.git")` runs
- **THEN** the URL is appended to `vendor.urls`'s list
- **AND** comments are preserved

#### Scenario: Append to missing group errors
- **GIVEN** a `hop.yaml` with no `experiments` group
- **WHEN** `yamled.AppendURL(path, "experiments", "...")` runs
- **THEN** an error is returned: `yamled: group 'experiments' not found in <path>`
- **AND** the file is unchanged

#### Scenario: Append to map-shaped group missing urls field errors
- **GIVEN** a `hop.yaml` with `vendor: { dir: ~/vendor }` (no `urls` field)
- **WHEN** `yamled.AppendURL(path, "vendor", "...")` runs
- **THEN** an error is returned: `yamled: group 'vendor' is map-shaped but has no 'urls' field; cannot append`
- **AND** the file is unchanged

> **Justification for v1's strict behavior**: Adding a `urls:` key to a map-shaped group via `yaml.Node` manipulation is technically possible but doubles the implementation surface for an edge case. The `hop config init` starter doesn't produce empty map-shaped groups; the typical user-edited file has a `urls:` list. If/when the empty-map case becomes common, lift this restriction.

### Requirement: Atomic Write
Writes SHALL be atomic: temp file in the same directory, then `os.Rename` over the original. If the rename fails, the original file SHALL be left untouched.

#### Scenario: Crash mid-write leaves original intact
- **GIVEN** a successful temp file is created and synced
- **WHEN** the rename fails (simulated by making target read-only)
- **THEN** an error is returned
- **AND** the original `hop.yaml` is unchanged
- **AND** the temp file is cleaned up (or left for the user to clean — see Open Questions; v1: best-effort `os.Remove` of the temp file on rename failure, log to stderr if remove fails)

## Internal: Config Loader

### Requirement: Loader Uses `yaml.Node` to Preserve Group Order
`config.Load(path)` SHALL parse `hop.yaml` via `yaml.Node` (not directly into `map[string][]string` — that loses order). The new internal model:

```go
type Config struct {
    CodeRoot string  // resolved (default "~")
    Groups   []Group // ordered as they appear in YAML
}

type Group struct {
    Name string
    Dir  string    // empty means convention-driven
    URLs []string
}
```

`config.Load` SHALL:
1. Read the file. Empty file → return `&Config{CodeRoot: "~", Groups: nil}` (no error).
2. Unmarshal into a `*yaml.Node`.
3. Validate the top-level keys (only `config` and `repos`).
4. Validate `config.code_root` and other (currently none) `config.*` keys.
5. Walk `repos.*` mapping nodes, in source order (yaml.Node preserves order).
6. For each group: validate name regex; classify body shape (sequence → flat; mapping → check keys; other → error); collect URLs.
7. Validate URL uniqueness across all groups.
8. Validate URL uniqueness within each group.

#### Scenario: Group order is preserved
- **GIVEN** a `hop.yaml` with groups written in the source order: `experiments`, `default`, `vendor`
- **WHEN** loaded
- **THEN** `cfg.Groups[0].Name == "experiments"`, `cfg.Groups[1].Name == "default"`, `cfg.Groups[2].Name == "vendor"`

### Requirement: `repos.FromConfig` Builds Flat List with Group Field
`repos.FromConfig(cfg)` SHALL produce a flat `Repos` list where each `Repo` has a new `Group` field carrying the group name. Order: groups in `cfg.Groups` order; URLs within a group in source order.

```go
type Repo struct {
    Name  string
    Group string  // new in this change
    Dir   string
    URL   string
    Path  string
}
```

#### Scenario: Repo carries Group field
- **GIVEN** a config with `default` containing `git@github.com:foo/bar.git`
- **WHEN** `FromConfig` runs
- **THEN** the resulting Repo has `Name=bar`, `Group=default`, `URL=git@github.com:foo/bar.git`, `Dir=$HOME/foo`, `Path=$HOME/foo/bar`

## Internal: Wrapper Boundaries

### Requirement: `internal/proc` Adds `RunForeground` for Inherited stdio
A new function in `internal/proc` SHALL support exec-in-context for `hop -C`:

```go
// RunForeground invokes name+args with Dir set to dir and stdin/stdout/stderr
// inherited from the parent. The exit code of the subprocess is returned via
// the (code, error) pair: when the subprocess runs to completion, code is its
// exit code and error is nil. When exec fails before the subprocess starts
// (binary not found, dir doesn't exist), code is -1 and error is non-nil.
func RunForeground(ctx context.Context, dir, name string, args ...string) (int, error)
```

This SHALL use `exec.CommandContext(ctx, name, args...)` with `cmd.Dir = dir`, `cmd.Stdin = os.Stdin`, `cmd.Stdout = os.Stdout`, `cmd.Stderr = os.Stderr`. No package outside `internal/proc` SHALL invoke `os/exec` directly.

#### Scenario: RunForeground inherits stdio
- **GIVEN** `proc.RunForeground(ctx, "/tmp", "echo", "hello")`
- **WHEN** invoked from a parent that has stdout connected to a buffer
- **THEN** the buffer receives `hello\n` (subprocess writes to inherited stdout)

#### Scenario: RunForeground propagates child exit code
- **GIVEN** `proc.RunForeground(ctx, "/tmp", "false")`
- **WHEN** invoked
- **THEN** code is 1
- **AND** error is nil

#### Scenario: RunForeground errors when binary missing
- **GIVEN** `proc.RunForeground(ctx, "/tmp", "definitely-not-a-real-binary")`
- **WHEN** invoked
- **THEN** code is -1
- **AND** errors.Is(err, proc.ErrNotFound) is true

#### Scenario: RunForeground errors when dir does not exist
- **GIVEN** `proc.RunForeground(ctx, "/no/such/dir", "echo", "hi")`
- **WHEN** invoked
- **THEN** code is -1
- **AND** error is non-nil (chdir error from the runtime)

### Requirement: `os/exec` Audit Continues to Pass
After this change, the audit `grep --include='*.go' --exclude='*_test.go' -rn '"os/exec"' src/internal/ src/cmd/` SHALL continue to match only `src/internal/proc/`. New code in `cmd/hop/`, `internal/yamled/`, etc. MUST NOT import `os/exec` directly.

## Architecture: Package Layout

### Requirement: `cmd/repo/` Renamed to `cmd/hop/`
The directory `src/cmd/repo/` SHALL be moved to `src/cmd/hop/`. All file names SHALL stay the same (`main.go`, `root.go`, `path.go` → `where.go` (renamed), `cd.go`, `clone.go`, `code.go`, `open.go`, `ls.go`, `shell_init.go`, `config.go`, plus tests). Imports referencing the old path SHALL be updated. Module path SHALL stay `github.com/sahil87/repo`.

#### Scenario: cmd/hop builds
- **GIVEN** the renamed tree
- **WHEN** `cd src && go build ./cmd/hop` runs
- **THEN** the build succeeds and produces a binary

#### Scenario: cmd/repo no longer exists
- **WHEN** `ls src/cmd/repo` runs
- **THEN** the directory does not exist

### Requirement: Filename `path.go` Renamed to `where.go`
The file `src/cmd/hop/path.go` SHALL be renamed to `src/cmd/hop/where.go`. The constructor function `newPathCmd` SHALL be renamed to `newWhereCmd`. The shared helpers (`loadRepos`, `resolveOne`, `resolveAndPrint`, the sentinel errors) SHALL stay in this file.

### Requirement: New `internal/yamled/` Package
A new package SHALL be created at `src/internal/yamled/` with `yamled.go` and `yamled_test.go`. Package doc string explains its purpose: comment-preserving YAML node-level edits.

## Build: Justfile and Scripts

### Requirement: Justfile, build.sh, install.sh Updated for `hop`
The justfile recipes SHALL stay one-liners. `scripts/build.sh` SHALL produce `bin/hop` (output `built: bin/hop (version: ...)`). `scripts/install.sh` SHALL copy to `~/.local/bin/hop` (output `installed: ~/.local/bin/hop`). The cobra `Use:` strings, error prefixes, and help text in the source code drive most user-visible output; the scripts only need to update file names.

#### Scenario: just build outputs hop
- **WHEN** `just build` runs
- **THEN** `bin/hop` exists at the repo root
- **AND** stdout contains `built: bin/hop (version: ...)`

#### Scenario: just install outputs hop
- **WHEN** `just install` runs (after build succeeds)
- **THEN** `~/.local/bin/hop` exists
- **AND** stdout contains `installed: <home>/.local/bin/hop`

## Tests

### Requirement: Existing Test Files Updated for Rename
All existing `*_test.go` files referencing `repo` SHALL be updated:
- File paths under `cmd/repo/` move to `cmd/hop/`.
- Test fixtures and golden strings reference `hop`/`hop.yaml`/`$HOP_CONFIG`.
- Schema-parser tests in `internal/config/*_test.go` and `internal/repos/repos_test.go` SHALL be rewritten to cover the grouped form (flat list, map shape, validation cases above).
- Test fixtures in `internal/config/testdata/` SHALL be updated to grouped-form YAML or supplemented with new fixtures for both shapes.

### Requirement: New Tests for New Behavior
New tests SHALL cover:
- Bare-name `hop where` and `hop cd` shim paths (where automatable; the shim itself is documented as manually verified if shell-test infrastructure isn't introduced — see Open Questions).
- `hop -C <name> <cmd>...` resolution + exec-in-context (using a small fixture binary or `pwd`).
- `hop clone <url>` ad-hoc with auto-registration: clone to disk, YAML write-back, comment preservation. Use a local temp git repo as the URL target where possible (avoid network).
- `hop clone <url> --no-add`, `--no-cd`, `--name`, `--group`.
- `internal/yamled.AppendURL` for both group shapes and the error cases.
- URL parsing: SSH, HTTPS, nested GitLab, no-`.git` suffix.
- Schema validation: invalid group name, unknown top-level field, unknown group field, URL collision across groups, URL duplication within group.
- `$HOP_CONFIG` set-but-missing hard error; no fallback to `$REPOS_YAML`.
- `internal/proc.RunForeground` test seam (use a fake invocation to assert argv composition; test happy path with a small program like `echo`/`pwd`).

### Requirement: Cross-Platform Builds Pass
After the change, `cd src && GOOS=darwin GOARCH=arm64 go build ./...` and `cd src && GOOS=linux GOARCH=amd64 go build ./...` SHALL both succeed.

## Deprecated Requirements

### `repos.yaml` schema (flat directory→URLs map)
**Reason**: Replaced by grouped schema (Option 4) — see Config: Schema requirements.
**Migration**: N/A (no backward compat; users `cp repos.yaml ~/.config/hop/hop.yaml` and edit by hand).

### `$REPOS_YAML` env var
**Reason**: Replaced by `$HOP_CONFIG`. No fallback.
**Migration**: N/A.

### `repo` binary name and `repo`-prefixed error messages
**Reason**: Replaced by `hop`.
**Migration**: N/A.

### `repo path` and `repo config path` subcommands
**Reason**: Renamed to `where` for voice-fit with `hop`.
**Migration**: N/A (no aliases).

### `~/.config/repo/repos.yaml` and `$XDG_CONFIG_HOME/repo/repos.yaml`
**Reason**: Replaced by the `hop`-prefixed search paths.
**Migration**: N/A.

## Design Decisions

1. **`-C` is implemented via pre-Execute argv inspection, not cobra `PersistentPreRunE`.**
   - *Why*: Cobra's flag parser tries to dispatch `<cmd...>` after `-C <name>` as a subcommand. PersistentPreRunE fires after parsing — too late. Pre-Execute inspection of `os.Args` lets us split argv before cobra sees it.
   - *Rejected*: A custom cobra `Args` validator that swallows everything after `<name>` — works but spreads `-C` logic across multiple cobra hooks, harder to maintain. The pre-Execute approach is one well-isolated function.

2. **Long form for `-C` is omitted (no `--in <name>` alias).**
   - *Why*: `git -C` and `make -C` both use the short form only; users learn it once. Adding `--in` doubles the surface for no clarity gain.
   - *Rejected*: `--in <name>` — viable, but unnecessary.

3. **Group display only when names collide (not always).**
   - *Why*: The fzf line is short; adding `[group]` to every entry adds noise. Most users have ~30 repos with unique names, so most picker lines stay clean.
   - *Rejected*: Always show `name [group]\tpath\turl` — uniform but cluttered.

4. **Atomic YAML write via temp + rename.**
   - *Why*: A crash mid-write must not leave a corrupt `hop.yaml`. Temp + rename is the standard idiom; same dir ensures rename is atomic on the same filesystem.
   - *Rejected*: In-place rewrite — risks data loss.

5. **`internal/yamled` is a new dedicated package, not folded into `internal/config`.**
   - *Why*: `config` is the schema validator; `yamled` is a node-level mutator. Keeping them separate respects responsibility boundaries — `config` validates and consumes; `yamled` produces a node tree, navigates, mutates, writes. Either could be tested independently.
   - *Rejected*: Inline write-back inside `config` — would mix two concerns and grow `config.go`.

6. **`hop config init` does NOT add a migration step from `repos.yaml`.**
   - *Why*: User base is one (the author). Migration cost is `cp repos.yaml hop.yaml` plus minor edits. A migration helper would be permanent code complexity for a one-time use.
   - *Rejected*: A `hop config migrate <old-path>` subcommand — unnecessary complexity.

7. **`shell-init zsh` embeds the cobra-generated completion via a build-time generation step.**
   - *Why*: The existing `_repo() { _files }` placeholder offered no real completion. Cobra's `GenZshCompletion` knows the subcommand surface for free. Embedding it into the shim's emitted output means a single `eval "$(hop shell-init zsh)"` gives subcommand + flag completion. (Repo-name completion remains a v2 enhancement requiring `ValidArgsFunction` callbacks.)
   - *Rejected*: Keep the placeholder — wastes the affordance the rename provides.

8. **`Repo.Group` is added to the existing struct rather than building a separate `GroupedRepo` type.**
   - *Why*: All consumers (`hop ls`, fzf picker, match) eventually want a flat list. Carrying group context as a field on `Repo` is simpler than two parallel types.
   - *Rejected*: A separate `GroupedRepo` wrapper — adds boilerplate without enabling a new use case.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Binary renamed `repo` → `hop` | Confirmed from intake #1 — user explicit choice over `forge` | S:95 R:60 A:95 D:95 |
| 2 | Certain | Single-letter alias is `h` (with `hi` for un-shadowed bare invocation) | Confirmed from intake #2 — `r` shadows zsh builtin | S:95 R:80 A:95 D:95 |
| 3 | Certain | Subcommand rename `path` → `where`, `config path` → `config where`, no aliases | Confirmed from intake #3, #26, #27 | S:95 R:90 A:95 D:95 |
| 4 | Certain | Config file `hop.yaml`, env `$HOP_CONFIG`, search `~/.config/hop/hop.yaml` | Confirmed from intake #4 | S:95 R:60 A:95 D:95 |
| 5 | Certain | No backward compatibility, no migration command, no fallback to legacy paths | Confirmed from intake #5 — user explicit | S:100 R:30 A:100 D:100 |
| 6 | Certain | Schema is Option 4 (named groups) with `config:` and `repos:` top-level keys | Confirmed from intake #6 | S:95 R:50 A:95 D:95 |
| 7 | Certain | `config.code_root` defaults to `~`, may be absolute / `~`-prefixed / relative; relative resolves to `$HOME` | Confirmed from intake #7; relative-`code_root` semantics extended at spec stage as a parallel of relative-`dir` rule (assumption #22) | S:90 R:80 A:90 D:85 |
| 8 | Certain | `hop clone <url>` appends to end of group's `urls:` list; flat-list group also gets append | Confirmed from intake #8 | S:95 R:90 A:100 D:100 |
| 9 | Certain | YAML write-back preserves user comments via `yaml.Node` and atomic temp+rename | Confirmed from intake #9; atomic-write detail added at spec stage | S:90 R:60 A:90 D:90 |
| 10 | Certain | Group with `dir:` set bypasses convention; path = `<dir>/<name>` | Confirmed from intake #10 | S:95 R:80 A:95 D:95 |
| 11 | Certain | Bare-name dispatch lives in shell shim, not binary | Confirmed from intake #11 | S:95 R:85 A:95 D:90 |
| 12 | Certain | `hop clone <url>` defaults to `default` group | Confirmed from intake #12 | S:95 R:80 A:90 D:90 |
| 13 | Certain | Missing target group on `hop clone <url>` → exit 1 with explicit error | Confirmed from intake #13 | S:95 R:85 A:95 D:90 |
| 14 | Certain | Group name regex `^[a-z][a-z0-9_-]*$` enforced at load time | Upgraded from intake Confident — adopted as the validation rule with a specific error message; no remaining ambiguity given the spec's schema-validation contract | S:90 R:75 A:90 D:90 |
| 15 | Certain | Same URL in two groups → load error; same URL twice in one group → load error | Confirmed from intake #15; intra-group duplication added at spec stage as a natural extension | S:95 R:85 A:95 D:90 |
| 16 | Certain | URL parsing: strip `.git`, last segment is name, everything before is org (preserving nested groups) | Upgraded from intake Confident — aligns with v0.0.1 `deriveName` and extends naturally; spec defines exact algorithm | S:90 R:70 A:90 D:85 |
| 17 | Certain | URL detection: contains `://` OR (contains `@` AND contains `:`) → URL | Upgraded from intake Confident — covers SSH and HTTPS; misclassification of names with `:` is rare and acceptable | S:85 R:80 A:90 D:80 |
| 18 | Certain | `hi` is a separate alias function | Confirmed from intake #18 | S:90 R:90 A:90 D:90 |
| 19 | Certain | Cobra-generated zsh completion replaces the `_files` placeholder, embedded in `shell-init zsh` output | Confirmed from intake #19 | S:90 R:95 A:95 D:90 |
| 20 | Certain | Empty groups (no `urls`) are valid; placeholder for future repos | Confirmed from intake #20 | S:90 R:95 A:95 D:90 |
| 21 | Certain | Group with `dir` but no `urls` is valid; can be a `--group` target for ad-hoc clone | Confirmed from intake #21; the "valid `--group` target" extension added at spec stage as a natural consequence | S:90 R:90 A:90 D:90 |
| 22 | Certain | Relative `dir:` (no `/`, no `~`) resolves relative to `code_root` | Confirmed from intake #22 | S:95 R:80 A:90 D:90 |
| 23 | Certain | Nested-group GitLab URLs map to nested directory structure on disk | Confirmed from intake #23 | S:95 R:75 A:90 D:90 |
| 24 | Certain | `--no-cd` is interpreted by the shim (suppress shell `cd`); binary's `--no-cd` flag suppresses stdout path | Upgraded from intake Confident — spec defines explicit behavior on both sides | S:90 R:80 A:90 D:85 |
| 25 | Certain | `-C` flag uses pre-Execute argv inspection, not cobra's PersistentPreRunE | Upgraded from intake Confident — concrete technique pinned at spec stage | S:90 R:75 A:85 D:85 |
| 26 | Certain | `--name` overrides on-disk path component but YAML stores URL verbatim (no per-entry `name` field added to schema) | New at spec stage — derived from "no per-entry metadata" schema rule and `--name` semantics | S:90 R:75 A:90 D:85 |
| 27 | Certain | `stateAlreadyCloned` for ad-hoc `hop clone <url>` still appends to YAML; `statePathExistsNotGit` does not | New at spec stage — clarifies edge cases of auto-registration | S:90 R:80 A:90 D:85 |
| 28 | Certain | `hop -C` propagates child exit code; missing name resolves before exec; missing command (no argv after `<name>`) → exit 2 with usage hint | New at spec stage — required for predictable composition | S:95 R:85 A:95 D:90 |
| 29 | Certain | Group display in fzf picker shows `[group]` only when names collide (not always) | New at spec stage — Design Decision #3 | S:90 R:90 A:90 D:85 |
| 30 | Certain | New `internal/yamled` package owns node-level YAML edits; `internal/config` owns schema validation | New at spec stage — Design Decision #5 | S:95 R:85 A:95 D:90 |
| 31 | Certain | `internal/proc.RunForeground` is added for stdio-inheriting exec; `os/exec` audit continues to scope to `internal/proc/` only | New at spec stage — required for `-C` and respects Constitution Principle I | S:95 R:90 A:95 D:90 |
| 32 | Certain | Map-shaped group missing `urls:` field → `yamled.AppendURL` errors; v1 does not add `urls:` automatically | New at spec stage — strict simplicity for v1 | S:90 R:80 A:90 D:85 |
| 33 | Certain | Atomic YAML write (temp + rename in same directory) | New at spec stage — Design Decision #4 | S:95 R:85 A:95 D:90 |
| 34 | Certain | Top-level YAML keys other than `config` and `repos` → load error | New at spec stage — closes schema | S:95 R:85 A:95 D:90 |
| 35 | Certain | Map-shaped group keys other than `dir` and `urls` → load error | New at spec stage — closes schema | S:95 R:85 A:95 D:90 |
| 36 | Certain | Loader uses `yaml.Node` to preserve group order in `cfg.Groups`; `repos.FromConfig` outputs in this order | New at spec stage — required to round-trip and to make `hop ls` deterministic against source | S:95 R:80 A:90 D:90 |
| 37 | Certain | `Repo` struct gains a `Group` field | New at spec stage — Design Decision #8 | S:95 R:85 A:95 D:90 |
| 38 | Certain | `hop config init`'s embedded starter is the grouped-form starter (no flat-map fallback) | New at spec stage — direct consequence of intake §8 (new starter content) | S:95 R:85 A:95 D:90 |
| 39 | Certain | Existing `cmd/repo/`-level error prefixes (`repo: ...`, `repo clone: ...`, `repo open: ...`, etc.) are rewritten to use `hop:` / `hop clone:` / `hop open:` | New at spec stage — required for consistency | S:95 R:90 A:95 D:90 |
| 40 | Certain | Go module path `github.com/sahil87/repo` stays unchanged for v1 | Upgraded from intake Confident — explicitly out of scope per Impact section; user agreed | S:90 R:60 A:90 D:90 |

40 assumptions (40 certain, 0 confident, 0 tentative, 0 unresolved).
