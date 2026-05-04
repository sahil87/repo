# hop.yaml Schema

How `hop.yaml` is structured and how Repo entries are derived. Schema is parsed in `src/internal/config/config.go`; field derivation lives in `src/internal/repos/repos.go`.

## Schema

Two top-level keys: optional `config:` (currently with one optional field, `code_root`) and required `repos:` (a map of `group_name → group_body`).

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
    dir: experiments
    urls:
      - git@github.com:sahil87/sandbox.git
```

Parsed via `gopkg.in/yaml.v3` at the `yaml.Node` level (not directly into a typed struct) so that group source order is preserved on `cfg.Groups`.

## Top-level

- `config:` *(optional)* — map of global config. Currently has one field, `code_root`. Unknown keys → load error: `hop: parse <path>: unknown config field '<name>'`.
- `repos:` *(required)* — map of `group_name → group_body`. Missing `repos:` → `hop: parse <path>: missing required field 'repos'`.
- Any other top-level key → `hop: parse <path>: unknown top-level field '<name>'. Valid: 'config', 'repos'.`

## `config.code_root`

Optional. Defaults to `~` (literal tilde, expanded to `$HOME` at use time). Accepted forms:

- Absolute (`/srv/code`) → used verbatim.
- `~`-prefixed (`~`, `~/code`) → `~` expands to `$HOME`.
- Relative (`code`) → resolved relative to `$HOME`.

## Group body (two shapes)

Each group's value is either:

1. **Flat list (convention-driven):** a YAML list of URL strings. Each URL resolves to `<code_root>/<org-from-url>/<name-from-url>`. When `org` is empty (rare — bare path, no slash), the org component is dropped: `<code_root>/<name>`.
2. **Map with `dir` and `urls`:** a YAML map with optional `dir` and optional `urls`. Each URL resolves to `<dir>/<name-from-url>`. `dir` may be absolute, `~`-prefixed, or relative to `code_root`. Empty `dir: ""` → `hop: parse <path>: group '<name>' has empty 'dir'`.

Other YAML shapes for a group body → `hop: parse <path>: group '<name>' must be a list of URLs or a map with 'dir' and 'urls'.`

Map-shaped groups with unknown keys → `hop: parse <path>: group '<name>' has unknown field '<key>'. Valid: 'dir', 'urls'.`

## Group name

Validated against `^[a-z][a-z0-9_-]*$`. Mismatches → `hop: parse <path>: invalid group name '<name>'. Group names must match ^[a-z][a-z0-9_-]*$`.

`default` is just a name — not magic, not auto-created. But `hop clone <url>` (with no `--group`) defaults to it; if `default` is absent, that command errors.

## URL uniqueness

- Same URL in two groups → load error: `hop: parse <path>: URL '<url>' appears in groups '<a>' and '<b>'; a URL must belong to exactly one group.`
- Same URL twice in one group → load error: `hop: parse <path>: URL '<url>' is listed twice in group '<name>'.`
- Two groups with the same `dir` value → valid (URL-uniqueness covers actual on-disk collisions).
- Two repos with the same derived `Name` in different groups → valid (each lives at a different path; fzf disambiguates with `[group]` suffix).

## Empty / placeholder groups

Both forms are valid:

- Flat list with no entries: `default: []`
- Map with `dir:` but no `urls:`: `experiments: { dir: ~/code/experiments }`

The latter is useful as a `--group` target for `hop clone <url>` before any URLs have been registered.

## Loading semantics

`config.Load(path)`:

- Empty file (zero bytes) → `&Config{CodeRoot: "~", Groups: nil}`, no error. `hop ls` prints nothing; matchers return zero candidates.
- File containing only comments / null content → same as empty.
- Malformed YAML → `hop: parse <path>: <yaml.v3 error with line>`.
- Non-mapping top-level → `hop: parse <path>: top-level must be a mapping`.

## Internal data model

```go
type Config struct {
    CodeRoot string  // resolved (default "~")
    Groups   []Group // ordered as they appear in YAML
}

type Group struct {
    Name string  // group key
    Dir  string  // empty = convention-driven
    URLs []string
}

type Repo struct {
    Name  string
    Group string  // which group it came from
    Dir   string  // resolved parent (group's expanded dir, OR <code_root>/<org>)
    URL   string
    Path  string  // filepath.Join(Dir, Name)
}
```

`FromConfig(cfg)` walks groups in `cfg.Groups` order, applies resolution, produces a flat `Repos` list. Match resolution stays substring-on-`Name`. Group context flows into fzf display lines via `cmd/hop/where.go::buildPickerLines`.

## URL parsing (`DeriveName`, `DeriveOrg`)

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
4. The last `/`-separated component is `name`.
5. Everything before that last `/` is `org` (may contain `/` for nested GitLab groups; preserved as nested directory structure on disk).

## Path resolution per repo

```go
// Group has Dir
Path = filepath.Join(repos.ExpandDir(g.Dir, cfg.CodeRoot), name)

// Flat group, org non-empty
Path = filepath.Join(expand(cfg.CodeRoot), org, name)

// Flat group, org empty
Path = filepath.Join(expand(cfg.CodeRoot), name)
```

`ExpandDir(dir, codeRootHint)` rules:

- `""` → `""` (caller decides default)
- `"~"` → `$HOME` (verbatim if `$HOME` unset)
- `"~/..."` → `$HOME/...`
- absolute (`/...`) → verbatim
- `~user...` → verbatim (no Linux-style user lookup)
- relative (no `/`, no `~`) + non-empty `codeRootHint` → joined with the *expanded* codeRootHint (recursive call with empty hint to break the loop)
- relative + empty hint → joined with `$HOME`

## Permissive on URL contents

Anything is accepted as a URL string. A YAML-valid but semantically odd entry like `- not a url` parses fine (subject to URL uniqueness validation); `Name` becomes `not a url`. Errors surface at `git clone` invocation time, not load time.

## Removed in this change

The flat directory→URLs schema (v0.0.1's `~/code/sahil87: [...]`) is gone. There is no migration path — users `cp repos.yaml ~/.config/hop/hop.yaml` and rewrite by hand. See [search-order](search-order.md) for env var and search-path changes.
