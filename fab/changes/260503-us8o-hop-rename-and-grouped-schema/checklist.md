# Quality Checklist: Rename to `hop` and Adopt Grouped Schema

**Change**: 260503-us8o-hop-rename-and-grouped-schema
**Generated**: 2026-05-04
**Spec**: `spec.md`

## Functional Completeness

- [ ] CHK-001 Binary renamed to `hop`: `bin/hop` is produced by `just build`; `bin/repo` is NOT produced.
- [ ] CHK-002 All error messages, help text, comments use `hop`: zero occurrences of `repo:` (as a prefix) or `repo` as a binary reference in user-visible strings (`grep -rn '"repo:' src/cmd/hop/ src/internal/` returns no hits).
- [ ] CHK-003 Subcommand `path` → `where`: `hop where <name>` works; `hop path <name>` produces "unknown command" cobra error.
- [ ] CHK-004 Subcommand `config path` → `config where`: `hop config where` prints write target; `hop config path` produces "unknown command".
- [ ] CHK-005 Other subcommands preserved: `cd`, `code`, `open`, `clone`, `ls`, `shell-init`, `config init` all work and have not been renamed.
- [ ] CHK-006 New `-C` flag implemented: `hop -C <name> <cmd>...` resolves the name and execs the command in the resolved directory.
- [ ] CHK-007 `hop clone <url>` ad-hoc with auto-registration: clones the URL, appends to the target group's URL list in `hop.yaml`, prints the resolved path on stdout.
- [ ] CHK-008 `hop clone <url>` flags: `--no-add`, `--no-cd`, `--name`, `--group` all behave as specified.
- [ ] CHK-009 `hop clone <name>` (registry-driven) and `hop clone --all` preserved.
- [ ] CHK-010 Match algorithm preserved: case-insensitive substring on Name; exact-1 short-circuits fzf.
- [ ] CHK-011 Multi-group picker shows `[group]` suffix when names collide, and only then.
- [ ] CHK-012 YAML group order preserved: `cfg.Groups` reflects source order; `hop ls` output matches.
- [ ] CHK-013 Grouped schema parses: top-level `config:` and `repos:`, group bodies as flat list or map with `dir`/`urls`.
- [ ] CHK-014 `config.code_root` defaults to `~`; absolute, `~`-prefixed, and relative values all expand correctly.
- [ ] CHK-015 Group bodies validate: invalid group names rejected, unknown top-level fields rejected, unknown group fields rejected, empty `dir` rejected, missing `repos` rejected.
- [ ] CHK-016 Group name regex `^[a-z][a-z0-9_-]*$` enforced.
- [ ] CHK-017 URL uniqueness across groups enforced; URL duplication within a group enforced.
- [ ] CHK-018 Two groups may share `dir` value (no error).
- [ ] CHK-019 Empty groups (no `urls` or `urls: []`) accepted.
- [ ] CHK-020 Group with `dir` but no `urls` accepted; valid `--group` target for ad-hoc clone.
- [ ] CHK-021 Relative `dir:` resolves relative to `code_root`.
- [ ] CHK-022 Nested-group GitLab URLs map to nested directory structure on disk.
- [ ] CHK-023 URL parsing: `.git` stripped; SSH and HTTPS forms produce same `name`/`org`.
- [ ] CHK-024 Path resolution: flat group → `<code_root>/<org>/<name>`; map group → `<dir>/<name>`; empty `org` → drop component.
- [ ] CHK-025 Config file is `hop.yaml`; env var is `$HOP_CONFIG`; search paths use `hop` directory.
- [ ] CHK-026 `$REPOS_YAML` is ignored; no fallback to `repos.yaml` paths.
- [ ] CHK-027 `$HOP_CONFIG` set-but-missing produces explicit hard error mentioning `$HOP_CONFIG`.
- [ ] CHK-028 `hop config init` writes the new grouped-form starter content.
- [ ] CHK-029 `hop config init` refuses overwrite (existing file untouched).
- [ ] CHK-030 `hop shell-init zsh` emits new shim with bare-name dispatch, `_hop_dispatch`, `h()`, `hi()`, and cobra-generated zsh completion.
- [ ] CHK-031 Bare-name dispatch lives only in shim; binary's bare form (`hop` with one positional `<name>`) continues to behave as `hop where <name>`.
- [ ] CHK-032 `hop cd` from binary still errors with shell-only hint, updated to reference `hop shell-init zsh` and `hop where`.
- [ ] CHK-033 `internal/yamled.AppendURL` package exists, handles flat-list and map-shaped groups, errors on missing group, errors on map-shape with no `urls` field, writes atomically.
- [ ] CHK-034 `internal/yamled` write preserves comments in unmodified portions of the file. (Indentation normalization to yaml.v3 defaults is acceptable; comment preservation is the only contract.)
- [ ] CHK-035 `internal/proc.RunForeground` exists with the specified signature; inherits stdio; propagates child exit code; errors when binary missing or dir doesn't exist.
- [ ] CHK-036 `os/exec` audit passes: only `src/internal/proc/` imports `os/exec`; only `exec.CommandContext` is used.
- [ ] CHK-037 `cmd/repo/` directory no longer exists; `cmd/hop/` is the entrypoint directory.
- [ ] CHK-038 File `path.go` renamed to `where.go`; `newPathCmd` renamed to `newWhereCmd`.
- [ ] CHK-039 `internal/yamled/` package directory exists with `yamled.go` and `yamled_test.go`.
- [ ] CHK-040 Build scripts updated: `scripts/build.sh` produces `bin/hop`; `scripts/install.sh` copies to `~/.local/bin/hop`.
- [ ] CHK-041 Cross-platform builds pass for darwin-arm64 and linux-amd64.

## Behavioral Correctness

- [ ] CHK-042 `hop where <name>` reuses the same handler as v0.0.1's `repo path <name>` — substring match, fzf fallback, stdout-only path.
- [ ] CHK-043 `hop -C` execs without invoking cobra subcommand parsing on the post-`<name>` argv (i.e., `hop -C name git status` works without "unknown command 'git'" error).
- [ ] CHK-044 `hop -C` propagates child non-zero exit code as the parent's exit code (e.g., `hop -C name false` exits 1).
- [ ] CHK-045 `hop clone <url>` URL-detection: `://` triggers URL path; `@` AND `:` triggers URL path; otherwise `<name>` path.
- [ ] CHK-046 `hop clone <url> --no-cd` writes YAML and clones but suppresses stdout path; `--no-add` clones and prints path but does NOT modify YAML.
- [ ] CHK-047 `hop clone <url>` with `stateAlreadyCloned` still appends to YAML (registers existing checkout), prints path.
- [ ] CHK-048 `hop clone <url>` with `statePathExistsNotGit` fails without modifying YAML.
- [ ] CHK-049 `hop clone <url>` with URL already in target group's urls is a silent skip (stderr note, no YAML write).
- [ ] CHK-050 `Repo.Group` field is populated correctly from the source group.

## Removal Verification

- [ ] CHK-051 No code path reads `$REPOS_YAML`: `grep -rn 'REPOS_YAML' src/` returns no production hits (test-only or fixture-only OK if explicit).
- [ ] CHK-052 No code path reads `repos.yaml` filename: `grep -rn 'repos.yaml' src/` returns hits only inside test data describing the legacy schema (and only when needed to assert non-fallback).
- [ ] CHK-053 Old `Config.Entries` field removed; no callers reference it.
- [ ] CHK-054 Old flat-map starter content removed from `src/internal/config/starter.yaml`.
- [ ] CHK-055 No `path` subcommand registration; `hop path` produces unknown-command error.
- [ ] CHK-056 No `config path` subcommand registration.

## Scenario Coverage

- [ ] CHK-057 "Build produces hop binary": `just build` produces `bin/hop`, no `bin/repo` (verified manually + via `scripts/build.sh` content).
- [ ] CHK-058 "Error prefixes use hop" (fzf missing): test asserts the `hop:` prefix.
- [ ] CHK-059 "hop where <name> resolves and prints" scenarios: tests in `where_test.go`.
- [ ] CHK-060 "hop config where" prints write target: test in `config_test.go`.
- [ ] CHK-061 "hop -C executes child command" scenarios (5 in spec): tests in `dashc_test.go`.
- [ ] CHK-062 "Ad-hoc URL clone" scenarios (8 in spec): tests in `clone_test.go`.
- [ ] CHK-063 "Disambiguation when names collide": fzf picker test verifies `[group]` suffix appears.
- [ ] CHK-064 Config schema scenarios (12+ in spec): tests in `config_test.go` covering valid and invalid YAML fixtures.
- [ ] CHK-065 "$HOP_CONFIG set but missing" hard error: test in `resolve_test.go`.
- [ ] CHK-066 "$REPOS_YAML is ignored" scenario: test in `resolve_test.go`.
- [ ] CHK-067 "hop config init in fresh environment" scenarios: tests in `config_test.go`.
- [ ] CHK-068 "shell-init zsh emits expected functions" scenario: test in `shell_init_test.go` asserts presence of `hop()`, `_hop_dispatch`, `h()`, `hi()`, completion `_hop` function.
- [ ] CHK-069 `yamled.AppendURL` scenarios (6 in spec): tests in `yamled_test.go`.
- [ ] CHK-070 `proc.RunForeground` scenarios (4 in spec): tests in `proc_test.go`.

## Edge Cases & Error Handling

- [ ] CHK-071 `-C` with no value or no command: stderr usage error, exit 2.
- [ ] CHK-072 `-C` with unknown name: stderr resolution error, exit 1.
- [ ] CHK-073 `hop clone <url>` with missing target group: stderr error, exit 1, no YAML write, no clone.
- [ ] CHK-074 `hop clone <url>` with `default` group missing and no `--group`: errors with literal `'default'` in the error message.
- [ ] CHK-075 `yamled.AppendURL` rename failure leaves original file unchanged (atomic write verification).
- [ ] CHK-076 `yamled.AppendURL` for missing group does not modify the file.
- [ ] CHK-077 Empty `hop.yaml` (zero bytes) loads as `&Config{CodeRoot: "~", Groups: nil}`, not an error.
- [ ] CHK-078 Malformed YAML produces a parse error with file path and line.
- [ ] CHK-079 Group with `urls: []` loads as a valid group with zero repos.

## Code Quality

- [ ] CHK-080 Pattern consistency: New code (`internal/yamled`, `proc.RunForeground`, `-C` extraction in `main.go`, `cloneURL` in `clone.go`) follows the package-layout, error-handling, and naming conventions of surrounding code (`internal/proc`, existing `cmd/hop/*.go`).
- [ ] CHK-081 No unnecessary duplication: Existing helpers (`loadRepos`, `resolveOne`/`resolveByName`, `cloneState`, `proc.Run`, `expandTilde`, `deriveName`) are reused; new helpers (`deriveOrg`, `expandDir`, `looksLikeURL`) are introduced only where no existing helper applies.
- [ ] CHK-082 Readability over cleverness (Principles): Functions stay focused; control flow is straightforward; names are descriptive.
- [ ] CHK-083 Composition over inheritance (Principles): Go has no inheritance; verify struct embedding and interface composition are used appropriately for any new types (e.g., `Group` is a value type, not embedded; `Config.Groups` is a slice, not a wrapper).
- [ ] CHK-084 No god functions (Anti-pattern): No new function exceeds 50 lines without justification. Note: `config.Load` may approach this — split into helpers if it does.
- [ ] CHK-085 No utility duplication (Anti-pattern): `deriveOrg` is the only new URL-parsing helper; `expandDir` is the only new path-expansion helper; both have a single source of truth.
- [ ] CHK-086 No magic strings/numbers (Anti-pattern): All error message prefixes use named constants where they're reused (e.g., `gitMissingHint`, `fzfMissingHint`, the new `cdHint`); the group regex is a package-level constant; the `code_root` default `"~"` is a documented constant.

## Security

- [ ] CHK-087 `os/exec` audit: only `src/internal/proc/` imports `os/exec`; only `exec.CommandContext` is used. (Constitution Principle I.)
- [ ] CHK-088 User input validation before subprocess: URL strings reach `git clone` as positional args (not shell strings); `<name>` resolution does not interpolate into commands; `proc.RunForeground` passes argv as a slice.
- [ ] CHK-089 No `sh -c`, `bash -c`, or shell-string interpolation in production code.
- [ ] CHK-090 `yamled` writes use `os.CreateTemp` + `os.Rename` (no shell, no exec); file modes are explicit (`0644`).
- [ ] CHK-091 `hop config init` write mode remains `0644` (no theatrical 0600).

## Notes

- Check items as you review: `- [x]`
- All items must pass before `/fab-continue` (hydrate).
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] CHK-NNN **N/A**: {reason}`.
