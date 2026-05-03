# Config Resolution

> How `repos.yaml` is found, parsed, validated, and bootstrapped.

## Search Order

Resolved at every invocation — no caching, per Constitution Principle II ("No Database").

1. **`$REPOS_YAML`** if set
2. **`$XDG_CONFIG_HOME/repo/repos.yaml`** if `$XDG_CONFIG_HOME` is set
3. **`$HOME/.config/repo/repos.yaml`** (XDG fallback for systems where `$XDG_CONFIG_HOME` is unset)

The first candidate that resolves is used. **Resolution is *not* a fallthrough chain on existence** — if `$REPOS_YAML` is set, candidates 2 and 3 are not consulted (see "Hard error on `$REPOS_YAML` set but file missing" below).

### Removed from search order (vs. bash script)

The bash script also checked `$DOTFILES_DIR/repos.yaml` and `$HOME/code/bootstrap/dotfiles/repos.yaml`. Both are removed in v0.0.1:

- They are dotfiles-specific paths leaking into the public binary.
- Users who want them back can `export REPOS_YAML=$DOTFILES_DIR/repos.yaml` in their shell rc.

### Hard error on `$REPOS_YAML` set but file missing

If `$REPOS_YAML` is set, the user has declared their intent. Falling through to the next candidate would silently mask config bugs (a typo in the path, a deleted file, a broken Dropbox sync).

> **GIVEN** `$REPOS_YAML=/nonexistent/path.yaml`
> **WHEN** any subcommand needing config runs
> **THEN** stderr shows: `repo: $REPOS_YAML points to /nonexistent/path.yaml, which does not exist. Set $REPOS_YAML to an existing file or unset it.`
> **AND** exit code is 1
> **AND** candidates 2 and 3 are NOT consulted

Other resolution scenarios:

> **GIVEN** `$REPOS_YAML` is unset, `$XDG_CONFIG_HOME=/Users/sahil/.config`, and `/Users/sahil/.config/repo/repos.yaml` exists
> **WHEN** any subcommand needing config runs
> **THEN** that file is loaded
> **AND** candidate 3 is not consulted

> **GIVEN** all of `$REPOS_YAML`, `$XDG_CONFIG_HOME` are unset, but `~/.config/repo/repos.yaml` exists
> **WHEN** any subcommand needing config runs
> **THEN** `~/.config/repo/repos.yaml` is loaded

> **GIVEN** all three candidates resolve to nothing (no env vars, no file at `~/.config/repo/repos.yaml`)
> **WHEN** any subcommand needing config runs
> **THEN** stderr shows: `repo: no repos.yaml found. Set $REPOS_YAML to a tracked file (e.g., a Dropbox path or a git-tracked dotfile), or run 'repo config init' to bootstrap one at $XDG_CONFIG_HOME/repo/repos.yaml.`
> **AND** exit code is 1

> **NOTE**: `repo config init` and `repo config path` do **not** require an existing `repos.yaml`. They run even when the file is absent. All other subcommands require a loadable file.

## YAML Schema

```yaml
# Repositories to clone, grouped by parent directory.
# Each key is a directory (~ is expanded). Values are git clone URLs.

~/code/sahil87:
  - git@github.com:sahil87/repo.git
  - git@github.com:sahil87/wt.git

~/code/wvrdz:
  - git@github.com:wvrdz/dev-shell.git
```

### Structure

- **Top level**: a YAML map of `directory → list of git URLs`.
- **Directory keys**: strings. The literal `~` prefix is expanded to `$HOME` at load time. Any other path is used verbatim.
- **URLs**: strings. Both SSH (`git@host:owner/name.git`) and HTTPS (`https://host/owner/name.git`) forms are accepted; the binary does not parse or validate the URL beyond extracting the repo name.

### Derived fields

- **Repo name** = last `/`-separated component of the URL, with trailing `.git` stripped.
  - `git@github.com:sahil87/repo.git` → `repo`
  - `https://github.com/wvrdz/loom.git` → `loom`
  - `git@gitlab.com:org/group/sub/proj.git` → `proj`
- **Repo path** = `<expanded-dir> + "/" + <name>`.
  - `~/code/sahil87` + `repo` → `/Users/sahil/code/sahil87/repo`

### Loading semantics

- The YAML is parsed using `gopkg.in/yaml.v3` (per Constitution Principle IV — Wrap, Don't Reinvent).
- Order of entries is preserved (yaml.v3 does this by default for `yaml.Node` round-trips, but for our use we accept whatever the library gives — `repo ls` output order matches yaml definition order in practice).
- Empty file → zero repos, no error. `repo ls` prints nothing; matching subcommands behave as if no repos exist (every match resolves to "no candidates" and either prompts fzf with empty list or errors).
- Malformed YAML → parse error with file path and line number on stderr; exit 1.

> **GIVEN** `repos.yaml` contains:
>
> ```yaml
> ~/code/sahil87:
>   - not a url   # invalid value type for our use, but valid YAML
> ```
>
> **WHEN** any subcommand loads it
> **THEN** the load succeeds (YAML is valid)
> **AND** the repo's name is derived as `not a url` (last token after split-on-`/`, no `.git` strip)
> **AND** clone-style operations on this repo will fail at git invocation time, not load time

This permissive behavior matches the bash script (`yq` extracts whatever the URL field contains; downstream `git clone` is what validates).

## `repo config init`

Bootstrap a starter `repos.yaml`.

### Write target

1. If `$REPOS_YAML` is set: write to that path. (User has declared intent.)
2. Else: write to `$XDG_CONFIG_HOME/repo/repos.yaml` (or `$HOME/.config/repo/repos.yaml` if `$XDG_CONFIG_HOME` is unset).

### Behavior

- If the target file already exists: refuse to overwrite. Print to stderr: `repo config init: <path> already exists. Delete it first or set $REPOS_YAML to a different path.` Exit 1.
- If the parent directory doesn't exist: create it (`os.MkdirAll`, mode 0755).
- Write the embedded starter content. **Mode 0644.**
- Print to stdout: `Created <path>`.
- Print to stderr: `Edit the file to add your repos. Tip: set $REPOS_YAML in your shell rc to point at a version-tracked location (a git-tracked dotfile, Dropbox, etc.) so this config moves with you across machines.`
- Exit 0.

### Embedded starter content

Stored at `src/internal/config/starter.yaml`, embedded via `//go:embed`. Content:

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

The starter is self-bootstrapping: a fresh user with this file can immediately `repo` (fzf picker shows one entry: `repo`) or `repo clone repo` (downloads this tool's source). It also serves as a copy-pasteable example of the schema.

### Scenarios

> **GIVEN** `$REPOS_YAML` is unset, `~/.config/repo/repos.yaml` does not exist
> **WHEN** I run `repo config init`
> **THEN** `~/.config/repo/repos.yaml` is created with mode 0644 containing the starter content
> **AND** stdout shows `Created /Users/sahil/.config/repo/repos.yaml`
> **AND** exit code is 0

> **GIVEN** the same setup but `$XDG_CONFIG_HOME=/Users/sahil/.cfg`
> **WHEN** I run `repo config init`
> **THEN** `/Users/sahil/.cfg/repo/repos.yaml` is created (parent dirs created if absent)

> **GIVEN** `$REPOS_YAML=/tmp/test.yaml` is set, `/tmp/test.yaml` does not exist
> **WHEN** I run `repo config init`
> **THEN** `/tmp/test.yaml` is created (parent `/tmp` already exists)

> **GIVEN** `~/.config/repo/repos.yaml` already exists
> **WHEN** I run `repo config init`
> **THEN** stderr shows the "already exists" message
> **AND** exit code is 1
> **AND** the existing file is unmodified

## `repo config path`

Print the resolved config path to stdout.

### Behavior

- Run the same search order as a normal load.
- Print the path that *would* be used to stdout (whether or not the file exists).
- If a path can be determined (env or HOME-based fallback resolves), exit 0.
- If no path can be resolved (e.g., `$HOME` is unset and no env vars are set — extremely rare), print to stderr: `repo: no config path resolvable. Set $REPOS_YAML or ensure $XDG_CONFIG_HOME or $HOME is set.` Exit 1.

> **NOTE**: This subcommand does NOT trigger the "$REPOS_YAML set but file missing" hard error — it's a debug aid, not a load. It always prints what *would* be used and exits 0 unless the path itself is unresolvable.

### Scenarios

> **GIVEN** `$REPOS_YAML=/tmp/foo.yaml`
> **WHEN** I run `repo config path`
> **THEN** stdout is `/tmp/foo.yaml`
> **AND** exit code is 0 (regardless of whether `/tmp/foo.yaml` exists)

> **GIVEN** `$REPOS_YAML` is unset, `$XDG_CONFIG_HOME=/Users/sahil/.config`
> **WHEN** I run `repo config path`
> **THEN** stdout is `/Users/sahil/.config/repo/repos.yaml`
> **AND** exit code is 0

> **GIVEN** all env vars unset, `$HOME=/Users/sahil`
> **WHEN** I run `repo config path`
> **THEN** stdout is `/Users/sahil/.config/repo/repos.yaml`
> **AND** exit code is 0

## Design Decisions

1. **No caching.** The YAML is re-read on every invocation. Per Constitution Principle II — "No Database." The file is small (typically <1KB); re-parsing is cheap.
2. **Hard error on `$REPOS_YAML` missing.** Setting an env var is intent. Silent fallthrough would mask config bugs. This diverges from the bash script (which falls through), and is intentional.
3. **Mode 0644 for the starter.** The file contains repo paths and git URLs — none are credentials. Treating it as sensitive (0600) would be theater. Users who want stricter perms can `chmod` after init.
4. **`$DOTFILES_DIR` and the hardcoded `~/code/bootstrap/dotfiles/repos.yaml` paths are gone.** They were Sahil's personal layout leaking into the binary. Users who relied on them set `REPOS_YAML` instead.
5. **Self-bootstrapping starter content.** Includes this repo's URL as a copy-pasteable example. Trade-off: hardcodes one URL into the binary; benefit: a fresh user runs `repo` immediately after init and sees something work.
