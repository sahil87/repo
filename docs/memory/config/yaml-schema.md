# repos.yaml Schema

How `repos.yaml` is structured and how Repo entries are derived. Schema is parsed in `src/internal/config/config.go`; field derivation lives in `src/internal/repos/repos.go`.

## Schema

Top-level YAML map of `directory → list of git URLs`:

```yaml
~/code/sahil87:
  - git@github.com:sahil87/repo.git
  - git@github.com:sahil87/wt.git

~/code/wvrdz:
  - git@github.com:wvrdz/dev-shell.git
```

Parsed with `gopkg.in/yaml.v3` into `Config.Entries map[string][]string`. No nested structure, no metadata — just dirs and URLs.

## Loading semantics

`config.Load(path) (*Config, error)`:

- Reads file via `os.ReadFile` — wraps `not found` etc. as `repo: read <path>: <wrapped>`.
- Empty file (zero bytes) → returns `&Config{Entries: map[string][]string{}}`, no error. `repo ls` then prints nothing; matchers return zero candidates.
- Malformed YAML → `repo: parse <path>: <yaml.v3 error with line>`. yaml.v3 errors carry line numbers natively, so they appear in stderr automatically.
- Nil map after unmarshal (e.g., file is just comments) → coerced to empty map.

## Derived fields (`internal/repos`)

`repos.FromConfig(cfg) Repos` walks `cfg.Entries` and produces `Repo{Name, Dir, URL, Path}`:

| Field | Derivation |
|---|---|
| `Dir` | The directory key with leading `~` expanded to `$HOME` (only when `~` is the literal first char followed by `/` or end-of-string). Other paths are verbatim. |
| `URL` | The URL string from the YAML, used verbatim — neither parsed nor validated. |
| `Name` | Last `/`-separated component of the URL with trailing `.git` stripped. SSH (`git@github.com:user/foo.git`) and HTTPS (`https://host/owner/foo.git`) both yield `foo`. `git@gitlab.com:org/group/sub/proj.git` yields `proj`. |
| `Path` | `filepath.Join(Dir, Name)` — i.e., `<expanded-dir>/<name>`. |

## Sort order

`FromConfig` sorts directory keys (lexicographic), then URLs within each directory (lexicographic). Required because yaml.v3 unmarshal into a `map` discards source order — deterministic sort is the next-best stable contract for `repo ls` and the bare-form picker.

## Permissive on URL contents

Anything is accepted as a URL string. A YAML-valid but semantically odd entry like `- not a url` parses fine; `Name` becomes `not a url` (last `/`-component, no `.git` to strip). Errors surface at git-invocation time (`repo clone`), not load time. This matches the bash original's `yq`-then-`git` flow.

## Tilde expansion edge cases (`expandTilde`)

- `~` alone → `$HOME`
- `~/foo` → `$HOME/foo`
- `~user/foo` → verbatim (no Linux-style user lookup)
- `/etc/~weird` → verbatim (only leading `~` is special)
- If `$HOME` is unset and the dir is `~` or `~/...` → returned verbatim (best-effort).
