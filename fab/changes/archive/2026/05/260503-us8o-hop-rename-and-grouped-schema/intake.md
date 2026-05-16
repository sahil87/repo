# Intake: Rename to `hop` and Adopt Grouped Schema

**Change**: 260503-us8o-hop-rename-and-grouped-schema
**Created**: 2026-05-04
**Status**: Draft

## Origin

This change emerged from an extended `/fab-discuss` exploration of the `repo` binary's developer experience. The conversation walked through:

1. **Argument order** (`repo cd outbox` vs `repo outbox cd`) — concluded verb-first matches Unix convention and unlocks clean autocomplete.
2. **Composability** — `rk riff`-style use cases inside another repo motivated `-C` (exec-in-context) and stdout-as-path primitives.
3. **Lessons from zoxide / autojump** — single-letter alias as the killer hot path; bare-name dispatch as the convention.
4. **Single-character alias selection** — explored every letter; `r` shadows zsh's "repeat last command" builtin (confirmed with a real failure: `r` printed `rk riff` then errored on tmux pane 0). Settled on `h` after agreeing to rename the binary.
5. **Binary rename brainstorm** — `forge` (industry vocabulary for code hosts) vs `hop` (verb-as-name, foregrounds navigation). User chose `hop` because the dominant cognitive model is navigation, not "code forge."
6. **Clone flow** — distinguished registry-driven (`hop clone outbox`) from ad-hoc (`hop clone <url>`). Established that `hop clone <url>` auto-registers and lands the user (the name reinforces the design).
7. **Schema redesign** — explored 4 options (flat keys, URL-derived, hybrid string-or-map, named groups). User chose Option 4 (named groups) because Option 3's per-entry `dir` overrides cause repetition for groups of vendor / experimental repos.
8. **Defaults** — `code_root` defaults to `~`; comments preserved on YAML write-back; no migration command from old `repos.yaml`; no backward compatibility.

The change replaces the binary name, the config schema, the env var, the search paths, the shell shim, and one subcommand name (`path` → `where`). It also introduces three new concepts: bare-name dispatch, the `-C` exec-in-context flag, and ad-hoc URL clone with auto-registration. It is foundational — `sync`, `autosync`, and `features` are deferred but the schema and primitives leave room for them.

> Rename `repo` binary to `hop`, restructure config schema to grouped form, add ad-hoc URL clone with auto-registration, and lay foundation for future sync/autosync features.

## Why

**Problem.** The current `repo` tool is functionally complete for v0.0.1 but its DX has three structural limits that block its evolution into a daily-driver multitool:

1. **No hot path.** Every command is `repo <verb> <name>` — minimum 5 keystrokes before completion can help. The frequency-weighted ergonomic argument (cd is the dominant action) isn't honored.
2. **No composability primitive.** The binary prints paths to stdout (`repo path <name>`), but there's no `-C`-style flag to exec a child command in the resolved dir. Every composition (`rk riff` inside `outbox`, `just test` inside `fab-kit`) requires a manual two-step or shell substitution.
3. **Schema doesn't express the user's mental model.** The current flat-map (`directory → URLs`) forces the user to spell out parent directories that are derivable from URL conventions. For a user with a `~/code/<org>/<name>` layout, the file is redundant relative to how they think.

**Consequence of leaving it alone.** Future features (`sync`, `autosync`, cross-repo search) compound on a foundation that's already friction-heavy. Every new verb extends a verbose surface. The single-letter alias is impossible because `r` shadows a zsh builtin (proven empirically — see Origin §4). Composability stays a manual step. Each new feature pays the same DX tax.

**Why this approach.** The rename is in service of two primitives that everything else builds on: `hop where <name>` (path resolver) and `hop -C <name> <cmd>` (exec-in-context). Once those exist, `sync`, `autosync`, `features` become thin wrappers — the surface stays small as capabilities grow. Named groups give per-group operations a natural home (`hop sync vendor`) without polluting individual entries with metadata.

**Why now.** v0.0.1 just shipped. The user base is one (the author). The cost of a breaking rename is at its lifetime minimum. Every release that ships under `repo` raises the future cost of the rename and ossifies the schema choice.

**Why `hop` over `forge`.** Discussion explored both. `forge` is industry vocabulary (Gitea/Forgejo) and stronger semantically, but `hop` foregrounds the dominant action (navigation) and reads naturally with `-C` (`hop -C outbox rk riff` = "hop into outbox and run rk riff"). User preference: `hop`.

**Why Option 4 (named groups) over Option 3 (string-or-map list).** Option 3 requires repeating `dir: ~/vendor` for every vendor entry. Option 4 declares it once per group. Option 4 also gives per-group operations a first-class name (`hop sync vendor`), which Option 3 cannot.

**Why no backward compatibility.** User base is one. Migration cost is `cp repos.yaml hop.yaml` plus minor schema edits. Carrying a fallback chain for `repos.yaml` and `$REPOS_YAML` is permanent code complexity for a one-time concern.

## What Changes

### 1. Binary and command-surface rename

**Binary:** `repo` → `hop`. Module path stays `github.com/sahil87/repo` for v1 (a separate concern; rename if/when Go module path becomes a friction point). The package and binary names move:

- `src/cmd/repo/` → `src/cmd/hop/`
- All cobra command `Use:` strings prefixed with `hop`
- All error messages (`repo: ...`) → `hop: ...`
- Help text, comments, README

**Subcommand renames (two):**

1. `path` → `where`. Same handler (resolves and prints absolute path to stdout). Renamed for voice-fit with the new binary name (`hop where outbox` reads as "hop, where is outbox?").
2. `config path` → `config where`. Same handler (prints `ResolveWriteTarget()` to stdout). Renamed for consistency with the locator's `path` → `where` rename: both subcommands now use `where` for "tell me a path" semantics.

**No other subcommand renames.** `cd`, `code`, `open`, `clone`, `ls`, `shell-init`, `config init` keep their names.

### 2. Bare-name dispatch (the hot-path enabler)

The shell shim routes `hop <not-a-known-subcommand>` to `cd <name>`:

```sh
hop() {
  if [[ $# -eq 0 ]]; then
    command hop                              # bare → fzf picker (today's behavior)
    return $?
  fi

  case "$1" in
    cd|clone|where|ls|code|open|shell-init|config|--help|-h|--version|completion)
      _hop_dispatch "$@"
      ;;
    -*)
      command hop "$@"                       # global flags pass through
      ;;
    *)
      _hop_dispatch cd "$1"                  # bare name → cd
      ;;
  esac
}
```

This is what makes `h outbox` (the single-letter alias) viable. Without bare-name dispatch, `h outbox` would error.

**Binary-side support:** The binary itself does NOT implement bare-name dispatch — the shim is the sole site. Reasoning: bare-name `cd` requires shell mutation; if the binary tried to handle it, users without the shim would hit `errExitCode 2: 'cd' is shell-only`. Cleaner to make the shim the only place this logic lives.

**Shadowing concern:** What if a user has a repo literally named `cd`, `clone`, or `where`? The shim's `case` statement consumes those names as subcommands. Users would have to use the long form (`hop cd cd` is awkward but works). Documented as a known limitation; mitigations not pursued in v1.

### 3. Single-letter alias `h` (and `hi`)

The shell shim emits two alias functions alongside the main `hop` function:

```sh
h() { hop "$@"; }
hi() { command hop "$@"; }   # interactive — bypasses bare-name dispatch, so bare `hi` always picks
```

`h outbox` → `cd outbox` (via bare-name dispatch).
`h -C outbox rk riff` → exec in outbox.
`hi` (no args) → fzf picker. Equivalent to bare `hop` today.

Choice rationale (from Origin §4): `r` shadows zsh's "repeat" builtin and silently runs the last command, producing confusing failures. `re` was rejected once we agreed to rename the binary entirely. `h` is mnemonic (`h` = `hop`), home-row left-hand index finger, no major collisions.

### 4. Composability primitives

**`hop where <name>`** — Renamed from `path`. Resolves `<name>` and prints the absolute path to stdout. Handler shared with bare `hop <name>` resolution. No behavior change beyond the rename.

**`hop -C <name> <cmd...>`** — New global flag. Resolves `<name>` to a path, then `os.Chdir`s into it and `exec`s `<cmd...>` (or, more cobra-idiomatic, runs `<cmd...>` as a subprocess with `Dir:` set on `exec.Cmd`). Conceptually equivalent to `git -C <path>` and `make -C <path>`.

**Implementation note (flagged risk #1):** Cobra's standard global flag handling expects the rest of the args to be parsed as subcommands. `-C` needs to short-circuit normal command dispatch — once `-C` is seen with a name, everything after is treated as the literal command to exec. Likely needs a custom `PreRun` or `cmd.Execute()`-level hook that detects `-C` early and bypasses subcommand parsing. To be resolved at spec stage.

**Examples:**

```
hop -C outbox rk riff               # rk riff inside outbox
hop -C fab-kit just test            # just test inside fab-kit
hop -C outbox git status            # git status inside outbox
```

**Why not just `cd "$(hop where outbox)" && rk riff`?** That works for one-off use but: (a) leaves cwd changed in the shell after the command, (b) requires a subshell `( ... )` to scope, (c) is more keystrokes. `hop -C` is the same primitive but ergonomic.

### 5. Clone modes

**Today's behavior preserved:**

- `hop clone <name>` — registry-driven. Resolves `<name>` against the loaded config, clones if missing. Same logic as `repo clone` today.
- `hop clone --all` — bulk-clone everything not on disk. Same as today.

**New: ad-hoc URL clone with auto-registration:**

- `hop clone <url>` — clones the URL, appends it to `hop.yaml` (default group), and prints the landed path on stdout (the shell shim then `cd`s there).
- `hop clone <url> --group <name>` — register under named group instead of `default`.
- `hop clone <url> --no-add` — clone but skip the `hop.yaml` write.
- `hop clone <url> --no-cd` — clone and register but don't print the path (no shell `cd`).
- `hop clone <url> --name <override>` — override the derived name.

**Argument disambiguation:**

```
contains "://" OR
contains "@" AND contains ":" with host:path shape
  → URL
otherwise
  → name
```

In Go this is roughly:

```go
func looksLikeURL(arg string) bool {
    if strings.Contains(arg, "://") {
        return true
    }
    // git@host:owner/name.git shape
    if strings.Contains(arg, "@") && strings.Contains(arg, ":") {
        return true
    }
    return false
}
```

Edge case: a name with `:` in it (extremely rare) could be misclassified. Acceptable — names are derived from URLs and `:` doesn't appear in URL basenames.

**Auto-registration target:** `default` group. If `default` doesn't exist in the config:

```
hop: no `default` group in $HOP_CONFIG. Pass `--group <name>` or add a `default:` group.
```

Exit 1.

**YAML write-back (flagged risk #3):** The write-back must preserve user comments and ordering. The current `gopkg.in/yaml.v3` `Marshal`/`Unmarshal` round-trip flattens comments. The fix: load the file as a `yaml.Node`, navigate to `repos.<group>.urls` (or `repos.<group>` if it's the flat list form — see schema §6), append a new scalar node, and marshal the root node back. Implementation cost is materially higher than the current marshal/unmarshal — but it's the right call. Users hand-edit `hop.yaml`; rewriting their formatting on every `hop clone` would erode trust.

**Append position:** End of the group's URL list. Don't sort, don't deduplicate (let load-time validation catch dupes).

**Append example:**

Before:
```yaml
config:
  code_root: ~/code

repos:
  default:
    - git@github.com:sahil87/repo.git    # the locator tool
    - git@github.com:sahil87/wt.git
```

After `hop clone git@github.com:sahil87/outbox.git`:
```yaml
config:
  code_root: ~/code

repos:
  default:
    - git@github.com:sahil87/repo.git    # the locator tool
    - git@github.com:sahil87/wt.git
    - git@github.com:sahil87/outbox.git
```

Comment preserved. Indentation preserved.

### 6. Config schema (Option 4 — named groups)

**File:** `hop.yaml` (was `repos.yaml`).
**Env var:** `$HOP_CONFIG` (was `$REPOS_YAML`).
**Search paths:**

1. `$HOP_CONFIG` if set and non-empty
2. `$XDG_CONFIG_HOME/hop/hop.yaml` if `$XDG_CONFIG_HOME` is set
3. `$HOME/.config/hop/hop.yaml`

Same hard-error semantics on `$HOP_CONFIG` set but missing. Same write-target semantics for `hop config init` / `hop config path`.

**No fallback to old paths.** No `repos.yaml` lookup. No `$REPOS_YAML` honored.

**Schema:**

```yaml
config:
  code_root: ~/code   # optional; defaults to ~

repos:
  default:
    - git@github.com:sahil87/repo.git
    - git@github.com:sahil87/wt.git
    - git@github.com:wvrdz/dev-shell.git

  vendor:
    dir: ~/vendor
    urls:
      - git@github.com:some-vendor/their-tool.git

  experiments:
    dir: ~/code/experiments
    urls:
      - git@github.com:sahil87/sandbox.git
```

**Top-level keys:**

- `config:` *(optional)* — map of global config. Currently has one field: `code_root`. More fields may be added later.
- `repos:` *(required)* — map of `group_name → group_body`.

**`config.code_root`** *(optional, default `~`)*: Base path for convention-driven groups. `~` is expanded to `$HOME` at load time. Required only when at least one group lacks a `dir` field. If absent and a flat-form group exists, defaults to `~`.

**Group body — two shapes:**

1. **Flat list (convention-driven):** the value is a list of URL strings. Each URL resolves to `<code_root>/<org-from-url>/<name-from-url>`.
2. **Map with `dir` and `urls` (override):** the value is a map with optional `dir` and required `urls`. Each URL resolves to `<dir>/<name-from-url>`. `dir` may be absolute (`~/vendor`) or relative-to-`code_root` (TBD; see Open Questions). Org is irrelevant when `dir` is set.

**Validation:**

- Group name regex: `^[a-z][a-z0-9_-]*$`. Names that don't match → load error. Names like `My Group` or `123foo` rejected.
- `default` is just a name. Not magic, not auto-created. But `hop clone <url>` defaults to it (see §5).
- Same URL appearing in two groups → load error: `hop: URL <url> is in groups <a> and <b>; a URL must belong to exactly one group.`
- Empty group (no `urls` field, or `urls: []`) → valid. Treated as having zero repos.
- Group with `dir` but no `urls` → valid. Placeholder for future repos. `hop clone <url> --group <empty-group>` works.
- Two groups with the same `dir` → valid. Two groups can share a directory. (No conflict at the path level unless two URLs resolve to the same path, which is the URL-collision check above.)
- Two repos with the same derived `Name` across groups → valid. Each lives at a different on-disk path. `hop cd <name>` returns multiple matches; fzf disambiguates with group context in the display.

**URL parsing for org/name:**

| URL | Org | Name |
|---|---|---|
| `git@github.com:sahil87/repo.git` | `sahil87` | `repo` |
| `https://github.com/sahil87/repo.git` | `sahil87` | `repo` |
| `git@gitlab.com:org/group/sub/proj.git` | `org/group/sub` | `proj` |
| `https://github.com/sahil87/repo` (no `.git`) | `sahil87` | `repo` |

Algorithm:

1. Strip trailing `.git` if present.
2. SSH form (`git@host:path`): split on `:`, take part after.
3. HTTPS form (`https://host/path`): split on `/`, take parts after `host/`.
4. Last path component → name.
5. Everything before the last component → org. (May contain `/` for nested GitLab groups; preserved as nested directory structure on disk.)

**Path resolution per repo:**

```go
func (g Group) Resolve(url string, codeRoot string) string {
    name := deriveName(url)
    if g.Dir != "" {
        return filepath.Join(expandTilde(g.Dir), name)
    }
    org := deriveOrg(url)
    return filepath.Join(expandTilde(codeRoot), org, name)
}
```

**Internal data model:**

```go
type Group struct {
    Name string    // group key
    Dir  string    // empty means convention-driven
    URLs []string
}

type Config struct {
    CodeRoot string  // default "~"
    Groups   []Group // ordered as they appear in YAML (best-effort)
}

type Repo struct {
    Name  string
    Group string  // new: which group it came from
    Dir   string  // resolved (group's dir, OR <code_root>/<org>)
    URL   string
    Path  string
}
```

`FromConfig` walks groups, applies resolution, produces a flat `Repos` list. Match resolution stays substring-on-name. Group context flows into fzf display lines.

### 7. Shell shim — emitted contents

`hop shell-init zsh` emits:

```sh
# hop zsh integration — emit via: eval "$(hop shell-init zsh)"
# Installs: hop function (with bare-name dispatch), h alias, hi alias, completion.

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

_hop_dispatch() {
  case "$1" in
    cd)
      if [[ -z "$2" ]]; then
        command hop cd
        return $?
      fi
      local target
      target="$(command hop where "$2")" || return $?
      cd -- "$target"
      ;;
    clone)
      # Detect URL form (contains :// or @host:path)
      if [[ "$2" == *"://"* ]] || [[ "$2" == *"@"*":"* ]]; then
        local target
        target="$(command hop clone "${@:2}")" || return $?
        if [[ -n "$target" ]]; then
          cd -- "$target"
        fi
      else
        command hop "$@"
      fi
      ;;
    *)
      command hop "$@"
      ;;
  esac
}

h() { hop "$@"; }
hi() { command hop "$@"; }

# Completion (cobra-generated zsh completion script embedded here)
# ... (output of `hop completion zsh` at build time)
```

**Completion replacement:** today's `_repo() { _files }` is a placeholder. Replace with cobra's auto-generated zsh completion (`hop completion zsh` output, embedded into the shim or emitted alongside it). Cobra knows the subcommand list, so completion-after-`hop` is free. Repo-name completion (after `hop cd`, `hop where`, `hop code`, `hop open`, `hop sync` once it exists) requires a custom `ValidArgsFunction` per command that calls `loadRepos()` and returns names.

### 8. Bootstrap content (`hop config init`)

Embedded starter `hop.yaml`:

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

The starter clones `hop` itself when run. Same self-bootstrapping principle as today.

### 9. Build / packaging

- `justfile` recipes: `build` builds `hop`, `install` installs `hop`. Internal references to `repo` updated.
- `scripts/build.sh` produces `bin/hop`.
- `goreleaser.yml` (when present) builds artifact `hop_<version>_<os>_<arch>` instead of `repo_*`.
- `homebrew-tap` formula renamed (deferred to release-pipeline change).
- README updated.

### 10. Tests

All test files reference `repo` in command lines, env vars, and config files. Sweep:

- `cmd/repo/*_test.go` → `cmd/hop/*_test.go`
- `internal/config/*_test.go` — env var name, search paths, file name
- `internal/repos/repos_test.go` — schema parser tests rewritten for grouped form
- `internal/proc/proc_test.go` — likely no change
- New test files for: bare-name dispatch (shim test if feasible, or document as manual), `-C` flag, ad-hoc URL clone with auto-registration, YAML round-trip preserving comments, group validation rules, URL collision detection.

## Affected Memory

- `cli/subcommands.md`: (modify) major rewrite — new commands (`where`, `-C`), bare-name dispatch, ad-hoc clone modes, single-letter alias.
- `cli/match-resolution.md`: (modify) minor — group context in fzf disambiguation, name collision across groups.
- `config/yaml-schema.md`: (modify) full rewrite — grouped schema, `config.code_root`, URL parsing for org, resolution rules.
- `config/search-order.md`: (modify) full rewrite — `$HOP_CONFIG`, `~/.config/hop/hop.yaml`.
- `config/init-bootstrap.md`: (modify) rewrite — new starter content, embedded grouped form.
- `architecture/package-layout.md`: (modify) `cmd/hop/` rename, `Group` type added in `internal/repos`, possibly `internal/yamled` (or similar) for comment-preserving YAML write-back.
- `architecture/wrapper-boundaries.md`: (modify) minor — composability primitives (`where`, `-C`).
- `build/local.md`: (modify) binary name in justfile recipes, install targets, build outputs.

## Impact

**Code areas touched (estimated):**

- `src/cmd/repo/` → `src/cmd/hop/` (full directory rename + content edits across ~10 files)
- `src/internal/config/` (resolve.go, config.go, starter.yaml — full rewrite of schema parsing; new node-level YAML write logic)
- `src/internal/repos/repos.go` (`FromConfig` rewritten for groups; `Group` type added; `Repo.Group` field added)
- New: `src/internal/yamled/` (or similar) for `yaml.Node`-level append operation. Contains the comment-preserving write-back logic.
- All `*_test.go` files mention `repo` somewhere; sweep required.
- `justfile`, `scripts/build.sh` — binary name.
- `README.md` — full rewrite of usage section.

**External tools:** No new dependencies. `gopkg.in/yaml.v3` already supports `yaml.Node`-level operations.

**Public-API surface:** Breaking. The binary name, env var, config file, and schema are all incompatible with v0.0.1. Treated as a v0.x → v0.y bump (no semver guarantees yet).

**Out of scope:**

- `hop sync` / `hop autosync` — separate change. This change builds the foundation (`-C`, `where`, groups) but ships no sync verb.
- `hop features` (cross-repo grep / search) — separate change.
- gh-style URL shorthand (`hop clone foo/bar` → `https://github.com/foo/bar`) — punted; deferred until evidence of need.
- Per-group config beyond `dir` (`sync_strategy`, `auto_pull`, `exclude`, etc.) — schema leaves room but no fields added in v1. YAGNI.
- Config migration command (`hop config migrate <old-repos.yaml>`) — explicitly skipped per user direction. User base is one; manual rewrite is acceptable.
- Go module path rename (`github.com/sahil87/repo` → `github.com/sahil87/hop`) — out of scope. Module path is internal; binary name is what users see. May rename in a follow-up if module-path friction emerges.
- `repo config` subcommand restructure — `config init` keeps its name. (`config path` → `config where` IS part of this change; see What Changes §1.)

## Open Questions

- Should a relative `dir:` value (e.g., `dir: vendor`, no leading `/` or `~`) be interpreted as relative to `code_root`, or rejected as ambiguous? Spec needs to pick one.
- For URL parsing, when a URL has nested groups (`git@gitlab.com:org/group/sub/proj.git`), should the on-disk path be `<code_root>/org/group/sub/proj` (nested) or `<code_root>/org/proj` (flattened)? The spec assumes nested; confirm at spec stage with a real GitLab user (TBD).
- `hop clone <url> --no-cd` — what happens with the printed-path stdout convention? Should `--no-cd` suppress the path print, or should the shim be smart enough not to `cd` when `--no-cd` is passed? Probably the latter (binary always prints the path; shim handles the flag's shell semantics). To be resolved at spec stage.
- Does `hi` need to be a separate function, or can the user just type `hop`? Difference: bare `hop` falls through bare-name dispatch (works), but in a context where the user has aliased `hop` to something, `hi` (which calls `command hop` directly) is the un-shadowed escape. Worth keeping; it's two lines.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Binary renamed `repo` → `hop` | Discussed — user explicitly chose `hop` over `forge` after weighing both | S:95 R:60 A:95 D:95 |
| 2 | Certain | Single-letter alias is `h` | Discussed — `r` shadows zsh builtin (proven); user chose `h` | S:95 R:80 A:95 D:95 |
| 3 | Certain | Subcommand rename `path` → `where` | Discussed — user agreed for voice-fit with `hop` | S:95 R:90 A:95 D:95 |
| 4 | Certain | Config file `repos.yaml` → `hop.yaml`, env `$REPOS_YAML` → `$HOP_CONFIG`, search `~/.config/hop/hop.yaml` | Discussed — user agreed full rename | S:95 R:60 A:95 D:95 |
| 5 | Certain | No backward compatibility, no migration command | Discussed — user explicitly said "Just rename" and "no migration" | S:100 R:30 A:100 D:100 |
| 6 | Certain | Schema is Option 4 (named groups) | Discussed — user chose Option 4 over Option 3 to avoid `dir:` repetition | S:95 R:50 A:95 D:95 |
| 7 | Certain | `config.code_root` defaults to `~` when unset | Discussed — user said "Code root can be optional with default ~" | S:95 R:90 A:100 D:100 |
| 8 | Certain | `hop clone <url>` ad-hoc mode appends URL to end of group's `urls:` list | Discussed — user agreed "ok end of list" | S:95 R:90 A:100 D:100 |
| 9 | Certain | YAML write-back preserves user comments | Discussed — user said "Try to not lose comments". Drives `yaml.Node`-level write-back implementation | S:95 R:60 A:90 D:90 |
| 10 | Certain | `dir:` set on a group means convention is bypassed entirely (path = `<dir>/<name>`, org irrelevant) | Discussed — user confirmed "ok confirmed" | S:95 R:80 A:95 D:95 |
| 11 | Certain | Bare-name dispatch lives in shell shim, not binary | Clarified — user confirmed. Binary handling would error for users without shim; shell mutation belongs in shell. | S:95 R:85 A:95 D:90 |
| 12 | Certain | `hop clone <url>` defaults to `default` group when `--group` is unset | Discussed and agreed during option-4 evaluation; convention is documented in intake | S:95 R:80 A:90 D:90 |
| 13 | Certain | If `default` group does not exist and `hop clone <url>` runs without `--group`, exit with error | Discussed — user agreed during option-4 evaluation | S:95 R:85 A:95 D:90 |
| 14 | Confident | Group name regex `^[a-z][a-z0-9_-]*$` | Standard CLI-arg-safe identifier shape; not explicitly discussed but follows convention | S:65 R:75 A:85 D:75 |
| 15 | Certain | Same URL in two groups → load error | Discussed during schema deep-dive — user agreed explicitly | S:95 R:85 A:95 D:90 |
| 16 | Confident | URL parsing — strip `.git`, last path segment is name, everything before is org (preserve nested groups) | Discussed; matches today's `deriveName` and extends naturally | S:75 R:70 A:85 D:75 |
| 17 | Confident | Argument disambiguation between `<name>` and `<url>`: contains `://` OR (`@` AND `:`) → URL | Discussed; covers SSH and HTTPS forms; misclassification of names with `:` is rare and acceptable | S:80 R:80 A:90 D:80 |
| 18 | Certain | `hi` is an alias function (`hi() { command hop "$@"; }`) for the un-shadowed bare-`hop` path | Discussed; modeled on zoxide's `z`/`zi` pattern; user did not push back | S:90 R:90 A:90 D:90 |
| 19 | Certain | Cobra's auto-generated zsh completion replaces today's `_files` placeholder | Discussed; cobra supports it natively; standard practice | S:90 R:95 A:95 D:90 |
| 20 | Certain | Empty groups (no `urls`) are valid; placeholder for future repos | Discussed during edge-case walkthrough — user agreed | S:90 R:95 A:95 D:90 |
| 21 | Certain | Group with `dir` but no `urls` is valid | Discussed during edge-case walkthrough — user agreed | S:90 R:95 A:95 D:90 |
| 22 | Certain | `dir:` with a relative value (no leading `/` or `~`) is interpreted as relative to `code_root` | Clarified — user confirmed | S:95 R:80 A:90 D:90 |
| 23 | Certain | Nested-group GitLab URLs map to nested directory structure on disk (`<code_root>/<org>/<group>/<sub>/<name>`) | Clarified — user confirmed | S:95 R:75 A:90 D:90 |
| 24 | Confident | `--no-cd` is interpreted by the shell shim, not the binary (binary always prints the path) | Clarified — user confirmed | S:90 R:80 A:85 D:80 |
| 25 | Confident | `-C` flag implementation: cobra-level short-circuit (PersistentPreRunE or pre-Execute parsing); exact technique chosen during spec spike | Clarified — user confirmed approach; technique deferred to spec | S:85 R:70 A:80 D:75 |
| 26 | Certain | `hop config path` is renamed to `hop config where` for voice-fit consistency with locator rename | Clarified — user agreed to rename if needed. Both subcommands now use "where" for "tell me a path" semantics. | S:95 R:85 A:90 D:90 |
| 27 | Certain | The new `hop where` subcommand exists alongside removing `hop path` (no alias) | Direct consequence of "no backward compatibility" (assumption #5) — clean rename, not addition | S:95 R:85 A:95 D:90 |
| 28 | Confident | Go module path stays `github.com/sahil87/repo` for v1 | Out-of-scope per Impact section; module path is internal, not user-facing; deferred until/unless friction emerges | S:80 R:60 A:85 D:80 |

28 assumptions (23 certain, 5 confident, 0 tentative, 0 unresolved).
