# Tasks: Rename to `hop` and Adopt Grouped Schema

**Change**: 260503-us8o-hop-rename-and-grouped-schema
**Spec**: `spec.md`
**Intake**: `intake.md`

## Phase 1: Setup

- [ ] T001 Move directory `src/cmd/repo/` to `src/cmd/hop/`. Use `git mv` to preserve history. After the move, files `main.go`, `root.go`, `path.go`, `cd.go`, `clone.go`, `code.go`, `open.go`, `ls.go`, `shell_init.go`, `config.go`, plus all `*_test.go` files, live under `src/cmd/hop/`. The package declaration in every file in this directory stays `package main`.
- [ ] T002 Rename `src/cmd/hop/path.go` to `src/cmd/hop/where.go` (use `git mv`). Inside the file, rename the constructor `newPathCmd` to `newWhereCmd`. Update the cobra `Use:` string from `path <name>` to `where <name>`. Update the `Short:` to `echo absolute path of matching repo`. Leave the shared helpers (`loadRepos`, `resolveOne`, `resolveAndPrint`, `errFzfCancelled`, `errSilent`, `fzfMissingHint`) in place — they are used by other files.
- [ ] T003 Update `src/cmd/hop/path_test.go` to `src/cmd/hop/where_test.go` (use `git mv`). Update test names referencing the old subcommand name from `Path` → `Where` (e.g., `TestPathCommand` → `TestWhereCommand`).
- [ ] T004 [P] Create `src/internal/yamled/` directory with `yamled.go` (skeleton only — exported `AppendURL(path, group, url string) error` returning `errors.New("not implemented")`) and a placeholder `yamled_test.go` so `go build ./...` compiles cleanly. Package doc string: "Package yamled provides comment-preserving YAML node-level edits — used by hop clone <url> to append a URL to a group's URL list without rewriting the user's formatting."

## Phase 2: Core Implementation

### Naming sweep

- [ ] T005 [P] Update binary name and module references: in `src/cmd/hop/main.go` change the package-comment `// Command repo ...` → `// Command hop ...`. In `src/cmd/hop/root.go` change `Use: "repo"` → `Use: "hop"`, the entire `rootLong` constant from "repo —" / "Usage:" block / "Notes:" block to use `hop` and `hop.yaml` and `$HOP_CONFIG` per the spec's Help text section, the `Short:` field, and any inline comments. Reuse the help-text format from `docs/specs/cli-surface.md` adapted with renames per the new spec.
- [ ] T006 [P] Update error prefixes in `src/cmd/hop/path.go` (now `where.go`): `fzfMissingHint` from `"repo: fzf is not installed. ..."` → `"hop: fzf is not installed. ..."`. Inside `resolveOne`, error wrappers `"repo: fzf failed: %w"`, `"repo: malformed fzf selection %q"`, `"repo: selection %q not found in repo list"` change `repo` → `hop`.
- [ ] T007 [P] Update `src/cmd/hop/cd.go`: `cdHint` constant from `"repo: 'cd' is shell-only. Add 'eval \"$(repo shell-init zsh)\"' to your zshrc, or use: cd \"$(repo path \"<name>\")\""` → `"hop: 'cd' is shell-only. Add 'eval \"$(hop shell-init zsh)\"' to your zshrc, or use: cd \"$(hop where \"<name>\")\""`. The `Short:` field references `hop shell-init zsh` instead of `repo shell-init zsh`.
- [ ] T008 [P] Update `src/cmd/hop/code.go`: `codeMissingHint` from `"repo code: 'code' command not found. ..."` → `"hop code: 'code' command not found. ..."`.
- [ ] T009 [P] Update `src/cmd/hop/open.go`: format strings using `"repo open: ..."` → `"hop open: ..."`.
- [ ] T010 [P] Update `src/cmd/hop/clone.go`: `gitMissingHint` from `"repo: git is not installed."` → `"hop: git is not installed."`. Format strings using `"repo clone: ..."` → `"hop clone: ..."`.
- [ ] T011 [P] Update `src/cmd/hop/shell_init.go`: error messages from `"repo shell-init: ..."` → `"hop shell-init: ..."`. The `zshInit` constant gets a full rewrite — see T026.
- [ ] T012 [P] Update `src/cmd/hop/config.go`: error messages from `"repo config init: ..."` → `"hop config init: ..."`. The `Use:` of the renamed inner subcommand changes from `path` → `where`. Stderr "tip" line from `"... set $REPOS_YAML in your shell rc ..."` → `"... set $HOP_CONFIG in your shell rc ..."`.
- [ ] T013 [P] Update `src/internal/config/config.go`: error message wrappers `"repo: read %s: %w"` → `"hop: read %s: %w"`, `"repo: parse %s: %w"` → `"hop: parse %s: %w"`. (Will be further rewritten in T021.)
- [ ] T014 [P] Update `src/internal/config/resolve.go`: rename env var lookup from `REPOS_YAML` → `HOP_CONFIG`. Update error messages: `"repo: $REPOS_YAML points to %s, ..."` → `"hop: $HOP_CONFIG points to %s, ..."`; `"repo: stat $REPOS_YAML (%s): %w"` → `"hop: stat $HOP_CONFIG (%s): %w"`; `"repo: no repos.yaml found. Set $REPOS_YAML ..."` → `"hop: no hop.yaml found. Set $HOP_CONFIG to a tracked file ..., or run 'hop config init' to bootstrap one at $XDG_CONFIG_HOME/hop/hop.yaml."`. Update XDG/HOME path joins from `"repo", "repos.yaml"` → `"hop", "hop.yaml"`. Update `ResolveWriteTarget` similarly. Update final error message `"repo: no config path resolvable. ..."` → `"hop: no config path resolvable. Set $HOP_CONFIG or ..."`.

### Schema redesign

- [ ] T015 Update `src/internal/config/starter.yaml`: replace flat-map content with the grouped-form starter from spec `Requirement: hop config init Writes Embedded Starter`. Verbatim:
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
- [ ] T016 In `src/internal/repos/repos.go` add a `Group string` field to the `Repo` struct (between `Name` and `Dir`). Update the doc comment on `Repo` to mention `Group`. Tests in T024 will exercise this.
- [ ] T017 In `src/internal/config/config.go`, redesign the `Config` type and `Load` function. Replace `Config.Entries map[string][]string` with:
  ```go
  type Config struct {
      CodeRoot string
      Groups   []Group
  }

  type Group struct {
      Name string
      Dir  string
      URLs []string
  }
  ```
  Implement `Load(path string) (*Config, error)` to:
  1. Read file. Empty file (zero bytes) → `&Config{CodeRoot: "~", Groups: nil}`, nil error.
  2. `yaml.Unmarshal(data, &root)` where `root` is `*yaml.Node`.
  3. Validate: top-level must be a mapping node. Walk the top-level mapping and only accept keys `config` and `repos`. Unknown top-level → error `hop: parse <path>: unknown top-level field '<name>'. Valid: 'config', 'repos'.`. Missing `repos` → error `hop: parse <path>: missing required field 'repos'`.
  4. If `config:` is present: it must be a mapping with optional `code_root`. Other keys → error `hop: parse <path>: unknown config field '<name>'`. Set `cfg.CodeRoot` to its scalar value or default `"~"`.
  5. Walk `repos` mapping (in source order — yaml.Node preserves order via the `Content` slice). For each entry:
     a. Validate the group name against `^[a-z][a-z0-9_-]*$`. Mismatch → error `hop: parse <path>: invalid group name '<name>'. Group names must match ^[a-z][a-z0-9_-]*$`.
     b. If the value node is a sequence: collect URL strings. Build `Group{Name, Dir: "", URLs: ...}`.
     c. If the value node is a mapping: keys must be subset of `{dir, urls}`. Unknown → `hop: parse <path>: group '<name>' has unknown field '<key>'. Valid: 'dir', 'urls'.`. Empty `dir` (`""`) → `hop: parse <path>: group '<name>' has empty 'dir'`. Missing `urls` (or `urls: []`) → valid; URLs slice is empty.
     d. Other shapes → `hop: parse <path>: group '<name>' must be a list of URLs or a map with 'dir' and 'urls'.`.
  6. After collecting all groups, validate URL uniqueness:
     a. Within each group: duplicate → `hop: parse <path>: URL '<url>' is listed twice in group '<name>'.`.
     b. Across groups: same URL in two groups → `hop: parse <path>: URL '<url>' appears in groups '<a>' and '<b>'; a URL must belong to exactly one group.`.
  7. Return the populated `*Config`.

  Use `gopkg.in/yaml.v3` exclusively. Helper functions for node navigation can live in this file (or a new private helper file `nodes.go` if it gets large).
- [ ] T018 In `src/internal/repos/repos.go`, rewrite `FromConfig(cfg *config.Config) (Repos, error)` for the new schema. For each group in `cfg.Groups` (in order), for each URL in the group's `URLs` (in order):
  - Compute `name` and `org` from URL via `deriveName` and a new `deriveOrg(url string) string` helper (exported package-private). `deriveOrg` algorithm: strip `.git`, then for SSH `git@host:path` take everything after `:` and before the last `/` (or empty); for HTTPS `https://host/path` take everything after `host/` and before the last `/`; otherwise empty.
  - Resolve path:
    - If group has `Dir != ""`: `Path = filepath.Join(expandDir(group.Dir, cfg.CodeRoot), name)` where `expandDir` handles absolute, `~`-prefixed, and relative-to-`code_root` cases.
    - Else (flat group): `Path = filepath.Join(expand(cfg.CodeRoot), org, name)`. If `org == ""`, drop the empty path component.
  - Append `Repo{Name: name, Group: group.Name, Dir: <expanded dir or code_root component>, URL: url, Path: ...}`.

  Update `expandTilde` (or rename to `expand`) to support relative paths (no `~`, no `/`) by joining with `$HOME`, per spec's Path Resolution requirement. Preserve the existing `~` and `~/` handling.

  Update the doc comment on `FromConfig` to reflect the new behavior.
- [ ] T019 Update `src/cmd/hop/path.go` (now `where.go`)'s fzf picker line construction. In `resolveOne`, before building `pickerLines`, build a map `nameCount[name]int` counting how many repos share each `Name`. When constructing the line for a repo, if `nameCount[r.Name] > 1`, prefix the displayed column with `r.Name + " [" + r.Group + "]"`; else just `r.Name`. The tab-separated suffix `\tpath\turl` stays the same. Match-back logic (lookup by `Path` in `rs`) stays unchanged because path is unique.
- [ ] T020 Update `src/cmd/hop/ls.go` to use the new `Repos` and `Repo.Group` shape. The aligned `name<spaces>path` output format stays the same. (`Group` is not displayed; spec only requires the new field for fzf disambiguation.)

### YAML write-back

- [ ] T021 Implement `src/internal/yamled/yamled.go` `AppendURL(path, group, url string) error`:
  1. Read file with `os.ReadFile`. Error → wrap as `yamled: read <path>: %w`.
  2. Parse with `yaml.Unmarshal` into `*yaml.Node` (root document).
  3. Navigate: root.Content[0] (document → mapping); find `repos` key; get its value node; find `<group>` key; get its value node.
  4. Group value node:
     - Sequence node: append a new scalar node `&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: url}` to `Content`. The scalar's Style should default (no quotes); yaml.v3 picks the appropriate style.
     - Mapping node: find the `urls` key. If absent → return `yamled: group '<group>' is map-shaped but has no 'urls' field; cannot append`. If present, its value must be a sequence node — append a scalar to it (same as above).
     - Other shape → `yamled: group '<group>' has unexpected shape; cannot append`.
  5. If `repos` not found or `<group>` not found → return `yamled: group '<group>' not found in <path>`.
  6. Marshal: `out, err := yaml.Marshal(&root)`. (Pass the document node; yaml.v3 round-trips it.)
  7. Atomic write: `tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp.*")`; write bytes; `tmp.Sync()`; `tmp.Close()`; `os.Rename(tmp.Name(), path)`. On error after `CreateTemp`, `os.Remove(tmp.Name())` (best-effort) and return the error.
- [ ] T022 Add `proc.RunForeground(ctx context.Context, dir, name string, args ...string) (int, error)` to `src/internal/proc/proc.go`. Implementation:
  ```go
  func RunForeground(ctx context.Context, dir, name string, args ...string) (int, error) {
      cmd := exec.CommandContext(ctx, name, args...)
      cmd.Dir = dir
      cmd.Stdin = os.Stdin
      cmd.Stdout = os.Stdout
      cmd.Stderr = os.Stderr
      err := cmd.Run()
      if err != nil {
          if errors.Is(err, exec.ErrNotFound) {
              return -1, ErrNotFound
          }
          if code, ok := ExitCode(err); ok {
              return code, nil  // child ran and exited non-zero — propagate code, no error
          }
          return -1, err
      }
      return 0, nil
  }
  ```
  Add a doc comment matching the spec.

### -C flag and ad-hoc clone

- [ ] T023 Implement the `-C` flag in `src/cmd/hop/main.go`. Strategy: pre-Execute argv inspection. Add a function `extractDashC(args []string) (target string, childCmd []string, ok bool, err error)` that:
  1. Walks `args[1:]` (skip the binary name) looking for `-C` or `-C=...`.
  2. If found, captures the target (next arg or after `=`) and returns `args[2:]` after the target as `childCmd`. Args before `-C` (if any global flags) stay associated with hop and are filtered from the slice passed to cobra.
  3. Returns `ok=true` if `-C` is present, `ok=false` otherwise.
  4. Returns `err` if `-C` is present but malformed (e.g., `-C` at end of argv with no value, or `-C <name>` with no command after).

  In `main()`, after constructing `rootCmd` and BEFORE `rootCmd.Execute()`, check `extractDashC(os.Args)`. If `ok` and no error: resolve the target via the standard `loadRepos` + `MatchOne` path (or fall back to fzf — for now, use the same `resolveOne` helper. Since `resolveOne` lives in `where.go`, expose a helper `resolveByName(query string) (*repos.Repo, error)` that wraps the match-or-fzf logic without printing. Refactor `resolveOne` to call this helper. The pre-Execute path may need a minimal cobra `cmd` stand-in for stderr — pass `rootCmd` itself. After resolution, invoke `proc.RunForeground(ctx, repo.Path, childCmd[0], childCmd[1:]...)` with a `context.Background()`, then `os.Exit(code)`. Errors from extraction or resolution: write to stderr and `os.Exit(2)` for usage errors, `os.Exit(1)` for resolution errors.

  Error messages:
  - `-C` with no value: `hop: -C requires a value. Usage: hop -C <name> <cmd>...`, exit 2.
  - `-C <name>` with no command after: `hop: -C requires a command to execute. Usage: hop -C <name> <cmd>...`, exit 2.

  After `extractDashC` returns `ok=false`, proceed with the standard `rootCmd.Execute()` flow.
- [ ] T024 Refactor `resolveOne` in `src/cmd/hop/where.go` to extract the match logic into `resolveByName(query string) (*repos.Repo, error)` that returns the resolved repo without writing to stdout. `resolveOne` (which takes a `*cobra.Command` for stderr) calls `resolveByName` and on failure writes the install hint via `cmd.ErrOrStderr()`. The `-C` code path in T023 calls `resolveByName` directly and writes errors to `os.Stderr` itself.

  Adjust the function so the fzf install-hint path remains: `resolveByName` returns a typed error (e.g., `errFzfMissing`) which the caller maps to the appropriate stderr writer.
- [ ] T025 Implement `hop clone <url>` in `src/cmd/hop/clone.go`:
  - Add new flags to `newCloneCmd`: `--no-add bool`, `--no-cd bool`, `--name string`, `--group string`. Default `--group` to `default`.
  - Add a helper `looksLikeURL(arg string) bool` (in `clone.go` or a new helper file): `strings.Contains(arg, "://") || (strings.Contains(arg, "@") && strings.Contains(arg, ":"))`.
  - In `RunE`, after handling `--all`: if `len(args) == 1` AND `looksLikeURL(args[0])`, dispatch to a new `cloneURL(cmd, url string, group string, noAdd, noCD bool, nameOverride string) error`. Otherwise, the existing registry-driven path.
  - `cloneURL`:
    1. Load the config to verify the target group exists. If absent → `hop: no '<group>' group in <config-path>. Pass --group <existing-group> or add '<group>:' to your config.`, exit 1.
    2. Compute `name`: if `nameOverride != ""` use it, else `deriveName(url)`.
    3. Compute the on-disk `path` per the group's resolution rule. If the group has `Dir != ""`: `path = filepath.Join(expandDir(group.Dir, cfg.CodeRoot), name)`. Else: `org := deriveOrg(url)`; `path = filepath.Join(expand(cfg.CodeRoot), org, name)`; if `org == ""`, drop component.
    4. Determine `cloneState(path)`:
       - `stateMissing`: print `clone: <url> → <path>` to stderr; run `git clone <url> <path>` via `proc.Run` with the existing `cloneTimeout` context. On error: handle `proc.ErrNotFound` (git missing hint, errSilent); else return error. After successful clone:
         a. If `!noAdd`: call `yamled.AppendURL(configPath, group, url)`. Treat any error as warning to stderr (`hop clone: registered to <config-path> failed: <err>`) but still proceed (clone succeeded; YAML write is best-effort? — actually no, stronger: if write fails after successful clone, surface error and return errSilent. Decision: return errSilent so the user sees the failure; the on-disk clone is preserved).
         b. If `!noCD`: print `path` to stdout.
         Return nil.
       - `stateAlreadyCloned`: print `skip: already cloned at <path>` to stderr. Then:
         a. If `!noAdd`: `yamled.AppendURL(configPath, group, url)`. (May error if URL is already in YAML — note: URL uniqueness is enforced at load time, not at append time. If the URL is already in another group, the append still works; the next load will error. Decision: do not pre-check; let load-time validation catch dupes. v1 behavior: simple append.)

           BUT: if the URL is already in the **same** group's `urls` list (duplicate), the next load will error per spec. Decision: pre-check whether URL is already in the target group; if so, skip the append silently and emit `skip: <url> already registered in '<group>'` to stderr.
         b. If `!noCD`: print `path` to stdout.
         Return nil.
       - `statePathExistsNotGit`: print `hop clone: <path> exists but is not a git repo` to stderr. Return errSilent. Do NOT modify YAML, do NOT print stdout.
- [ ] T026 Rewrite `src/cmd/hop/shell_init.go` `zshInit` constant per the spec's `Requirement: hop shell-init zsh Emits New Shim Format`. The new content includes:
  - The header comment `# hop zsh integration ...`.
  - The `hop()` function with bare-name dispatch (per spec verbatim).
  - `_hop_dispatch()` helper handling `cd` and URL-detected `clone` paths.
  - `h() { hop "$@"; }` and `hi() { command hop "$@"; }`.
  - Cobra-generated zsh completion: at runtime, use `cobra.Command.GenZshCompletion(buf)` to render the completion script and embed it in the output. Implementation: in `newShellInitCmd`'s `RunE`, after writing `zshInit` (the static prefix), call `rootCmd.GenZshCompletion(cmd.OutOrStdout())`. Note: `rootCmd` reference — pass it into `newShellInitCmd(rootCmd *cobra.Command)` as a parameter, then forward through cobra wiring in `root.go`. (Alternative: a package-level `var rootForCompletion *cobra.Command` set in `main` — preferred for less plumbing.)

  The static `zshInit` constant content (everything before the completion script) is the function definitions. The completion script is appended at runtime.

## Phase 3: Integration & Edge Cases

### Test fixtures and updates

- [ ] T027 Update `src/internal/config/testdata/`: rename existing files (`valid.yaml`, `empty.yaml`, `malformed.yaml`) to grouped-form fixtures. Add new fixture files: `valid-flat.yaml` (flat list group), `valid-mixed.yaml` (one flat group, one map group), `valid-empty-group.yaml` (group with empty urls), `invalid-group-name.yaml` (group with uppercase name), `invalid-unknown-top.yaml` (top-level field other than config/repos), `invalid-url-collision.yaml` (same URL in two groups), `invalid-empty-dir.yaml` (group with empty dir), `invalid-unknown-group-key.yaml` (group with `sync_strategy:` field).
- [ ] T028 Update `src/internal/config/config_test.go` for the new schema. New test cases (each loading a fixture):
  - `TestLoadFlatGroup` — verifies flat list is parsed; one group with N URLs.
  - `TestLoadMapGroup` — verifies map shape with `dir:` and `urls:`.
  - `TestLoadMixedGroups` — verifies group order in `cfg.Groups` matches source order.
  - `TestLoadEmptyGroup` — verifies group with `urls: []` loads with zero URLs.
  - `TestLoadConfigCodeRoot` — verifies default `~` and explicit values.
  - `TestLoadInvalidGroupName` — verifies error `hop: parse <path>: invalid group name 'My Group'. ...`.
  - `TestLoadInvalidUnknownTop` — verifies unknown top-level field error.
  - `TestLoadInvalidURLCollision` — verifies URL-in-two-groups error.
  - `TestLoadInvalidEmptyDir` — verifies empty `dir` rejected.
  - `TestLoadInvalidUnknownGroupKey` — verifies unknown group field error.
  - `TestLoadMissingRepos` — verifies error when `repos:` is absent.
  - `TestLoadEmptyFile` — verifies `&Config{CodeRoot: "~", Groups: nil}` returned.
- [ ] T029 Update `src/internal/config/resolve_test.go` for `$HOP_CONFIG`:
  - Set/unset `HOP_CONFIG` (use `t.Setenv`).
  - Update file paths: `repo/repos.yaml` → `hop/hop.yaml`.
  - Update expected error strings.
  - Add a test verifying that `$REPOS_YAML` is ignored (set `REPOS_YAML` to a real file, expect `Resolve` to fall through to XDG/HOME).
- [ ] T030 Update `src/internal/repos/repos_test.go` for the new schema:
  - Replace existing fixtures using `Config.Entries` with new construction using `Config.Groups`.
  - `TestFromConfigFlatGroup` — flat group, default `code_root`, repos at `<HOME>/<org>/<name>`.
  - `TestFromConfigMapGroupAbsoluteDir` — map group with `dir: ~/vendor`, repo at `~/vendor/<name>`.
  - `TestFromConfigMapGroupRelativeDir` — map group with `dir: experiments`, repo at `<HOME>/<code_root>/experiments/<name>`.
  - `TestFromConfigGroupOrderPreserved` — three groups in source order; verify `out` reflects order.
  - `TestFromConfigRepoGroupField` — verify each `Repo` has the expected `Group` value.
  - `TestDeriveOrg` — table-driven test for `deriveOrg`: SSH, HTTPS, nested GitLab, no-`.git`, malformed.
  - `TestExpandDirRelative` — relative dir resolves to `code_root`.
- [ ] T031 [P] Create `src/internal/yamled/yamled_test.go`:
  - `TestAppendURLFlatList` — start fixture: flat list with comments. After append: comments preserved, new URL at end.
  - `TestAppendURLMapGroup` — start fixture: map group with `dir:` and `urls:` and comments. After append: comments preserved, URL appended to `urls` list.
  - `TestAppendURLMissingGroup` — error case.
  - `TestAppendURLMapGroupNoUrls` — map shape without `urls` field; expect error.
  - `TestAppendURLAtomic` — write to a temp file, verify atomicity by checking that on simulated rename failure (use a read-only target dir), original is unchanged. Use `t.TempDir` and chmod tricks.
  - `TestAppendURLPreservesIndentation` — fixture with non-default indentation (e.g., 4 spaces); verify output uses same indentation.
- [ ] T032 [P] Update `src/internal/proc/proc_test.go` to add tests for `RunForeground`:
  - `TestRunForegroundEcho` — invokes `echo hello` in `t.TempDir()`; capture stdout via setting `os.Stdout` to a pipe (use a small helper — see existing test patterns); verify code 0, output `hello\n`.
  - `TestRunForegroundFalse` — invokes `false`; verify code 1, error nil.
  - `TestRunForegroundMissing` — invokes a non-existent binary; verify code -1, errors.Is(err, ErrNotFound).
  - `TestRunForegroundDirChange` — invokes `pwd` in a temp dir; verify stdout contains the temp dir path.
- [ ] T033 [P] Update `src/cmd/hop/cd_test.go`: update expected `cdHint` string to use `hop` and `hop where`.
- [ ] T034 [P] Update `src/cmd/hop/where_test.go`: new tests verifying `where` subcommand resolves and prints (replacing old `path_test.go` cases). Update expected error strings to use `hop:` prefix.
- [ ] T035 [P] Update `src/cmd/hop/clone_test.go`:
  - Existing tests use registry-driven paths — update repo names and URLs to grouped-form fixtures.
  - New tests:
    - `TestCloneURLAdHoc` — uses a local file:// URL pointing at a temp git repo (init bare repo in t.TempDir, init source repo, push). Run `hop clone <url>`. Verify clone, YAML write-back (open file, parse, check URL is in default's urls), stdout path.
    - `TestCloneURLNoAdd` — same setup, with `--no-add`. Verify clone, stdout path, NO YAML modification.
    - `TestCloneURLNoCd` — same setup, with `--no-cd`. Verify clone, YAML written, stdout empty.
    - `TestCloneURLNameOverride` — with `--name foo`. Verify on-disk path uses `foo`.
    - `TestCloneURLGroupOverride` — config with `vendor` group; `hop clone --group vendor <url>`. Verify clone target uses `vendor`'s `dir`, YAML appended to `vendor.urls`.
    - `TestCloneURLMissingGroup` — config without `default`; `hop clone <url>`. Verify exit 1 with error message.
    - `TestCloneURLAlreadyCloned` — pre-existing target dir with `.git`. Verify `skip:` message, YAML appended (for register-existing-checkout flow), stdout path.
    - `TestCloneURLDuplicateInGroup` — URL already in target group's urls. Verify `skip: <url> already registered in 'default'` to stderr, no YAML modification.
    - `TestCloneURLPathConflict` — pre-existing non-git dir at target. Verify error, no YAML modification.
- [ ] T036 [P] Update `src/cmd/hop/code_test.go`, `src/cmd/hop/open_test.go`, `src/cmd/hop/ls_test.go`, `src/cmd/hop/shell_init_test.go`, `src/cmd/hop/config_test.go` for new error strings, fixtures, and renames.
- [ ] T037 [P] Add a new test file `src/cmd/hop/dashc_test.go` covering `-C`:
  - `TestDashCResolvesAndExecs` — config with one repo; `hop -C <name> echo hello`; verify echo runs in repo dir, stdout `hello`, exit 0. (Use the integration_test.go pattern that builds the binary and invokes it.)
  - `TestDashCMissingTarget` — `hop -C nonexistent echo hi`; verify stderr resolution error, exit 1.
  - `TestDashCNoCommand` — `hop -C name`; verify stderr usage error, exit 2.
  - `TestDashCNoTarget` — `hop -C`; verify stderr usage error, exit 2.
  - `TestDashCPropagatesExit` — `hop -C name false`; verify exit 1.
- [ ] T038 [P] Update `src/cmd/hop/integration_test.go`: rename binary references from `repo` → `hop`. Update fixture YAML paths to `hop.yaml`. Update env var sets from `REPOS_YAML` → `HOP_CONFIG`.
- [ ] T039 [P] Update `src/cmd/hop/testutil_test.go`: any helpers referencing `repo` or `repos.yaml` get renamed.

### Build pipeline

- [ ] T040 Update `scripts/build.sh`: `go build -ldflags "-X main.version=${VERSION}" -o ../bin/repo ./cmd/repo` → `... -o ../bin/hop ./cmd/hop`. Echo line `built: bin/repo` → `built: bin/hop`.
- [ ] T041 Update `scripts/install.sh`: `DEST="${HOME}/.local/bin/repo"` → `DEST="${HOME}/.local/bin/hop"`. Echo line `installed:` reflects the new path automatically.
- [ ] T042 [P] Update `justfile` if any internal references to `repo` exist (the recipes themselves are generic).
- [ ] T043 [P] Update `.gitignore` if it references `bin/repo` specifically; otherwise no change (a glob `bin/` covers both).

### Documentation

- [ ] T044 Update `README.md` to reference `hop` throughout: binary name, install instructions, examples, `$HOP_CONFIG`, `hop.yaml`. Add sections describing the new schema, `-C`, ad-hoc URL clone, single-letter alias, bare-name dispatch.

## Phase 4: Polish

- [ ] T045 Run `cd src && go vet ./...` and fix any reported issues.
- [ ] T046 Run `cd src && gofmt -w ./...` to ensure formatting consistency.
- [ ] T047 Run cross-platform build verification: `cd src && GOOS=darwin GOARCH=arm64 go build ./...` and `cd src && GOOS=linux GOARCH=amd64 go build ./...`. Both must succeed.
- [ ] T048 Run `cd src && go test ./...` (full suite). Address any test failures by tracing back to the implementation; never modify implementation just to make tests pass.
- [ ] T049 Audit: `grep --include='*.go' --exclude='*_test.go' -rn '"os/exec"' src/internal/ src/cmd/` MUST match only `src/internal/proc/`. `grep --include='*.go' --exclude='*_test.go' -rn 'exec\.Command\b' src/` MUST match zero. (The audit excludes `exec.CommandContext`, which is permitted.)
- [ ] T050 Run `just build && just install` end-to-end. Manually verify `hop --version`, `hop ls`, `hop config init` (in a tempdir HOME), `hop where <name>`, and confirm no `repo` references appear in any user-visible output.

---

## Execution Order

### Phase 1 (sequential setup)
- T001 → T002 → T003 (file moves and renames must happen in order; tests follow source).
- T004 is independent and may run in parallel with T001-T003.

### Phase 2 — Naming sweep (parallel within group)
- T005-T014 are all `[P]` — different files, no cross-dependencies. They can run in parallel after T001-T003 complete.

### Phase 2 — Schema redesign (sequential)
- T015 (starter content) is independent.
- T016 (`Repo.Group` field) blocks T018 and T019.
- T017 (`config.Load` rewrite) blocks T018, T019, T020, T028.
- T018 (`FromConfig` rewrite) blocks T019, T020.
- T019 (fzf picker disambiguation) and T020 (`ls` adaptation) come last in this group.

### Phase 2 — YAML write-back (sequential)
- T021 (yamled implementation) blocks T031, T035.
- T022 (`RunForeground`) blocks T023, T032, T037.

### Phase 2 — `-C` and ad-hoc clone (sequential)
- T024 (refactor `resolveByName`) blocks T023.
- T023 (`-C` extraction) blocks T037.
- T025 (`hop clone <url>`) requires T021, T017, T018, T024 done.
- T026 (`shell-init` rewrite) is independent of T017-T025; can run any time after T011.

### Phase 3 (parallel where marked)
- T027 (fixtures) blocks T028, T030.
- T031, T032, T033, T034, T035, T036, T037, T038, T039 are independent of each other (`[P]`).
- T040, T041, T042, T043 are independent build-pipeline updates.
- T044 (README) can run any time after Phase 2 is complete.

### Phase 4 (sequential polish)
- T045 → T046 → T047 → T048 → T049 → T050.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Phase ordering matches the natural dependency graph (rename → schema → write-back → `-C` → tests) | Derived from spec's Internal: requirements; unambiguous | S:95 R:90 A:95 D:90 |
| 2 | Certain | `git mv` is preferred over `mv` for directory renames to preserve history | Standard practice; spec implies it via the rename language | S:95 R:90 A:95 D:90 |
| 3 | Certain | `Repo.Group` field added between `Name` and `Dir` for natural reading order | Aesthetic; doesn't affect behavior | S:90 R:95 A:90 D:80 |
| 4 | Certain | URL-already-in-same-group on ad-hoc clone is a silent skip with stderr note, not an error | Avoids surfacing a duplicate-write error for a no-op user action; matches spec scenarios for AlreadyCloned | S:85 R:85 A:90 D:85 |
| 5 | Certain | `cloneURL`'s `--no-cd` suppresses stdout but not stderr status messages (`clone:`, `skip:`) | Spec explicitly: "stdout is empty" but stderr still has status. Mirrors how today's clone uses stderr for status. | S:95 R:90 A:95 D:90 |
| 6 | Certain | Cobra completion is regenerated at every `shell-init zsh` invocation (no embedding at compile time) | Avoids generate-time complexity; latency is trivial (microseconds for the in-process generator) | S:90 R:95 A:90 D:85 |
| 7 | Certain | `rootCmd` for completion is captured via package-level var set in `main()` rather than threaded through factory functions | Simplest plumbing; one-liner change in `main()` | S:90 R:95 A:90 D:85 |

7 assumptions (7 certain, 0 confident, 0 tentative, 0 unresolved).
