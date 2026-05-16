# Quality Checklist: Bootstrap Go binary v0.0.1

**Change**: 260503-iu93-bootstrap-go-binary
**Generated**: 2026-05-03
**Spec**: `spec.md`

## Functional Completeness

- [ ] CHK-001 Subcommand contract conformance: All 11 subcommands from `docs/specs/cli-surface.md` §"Subcommand Inventory (v0.0.1)" exist with the specified arg shapes, exit codes, and stdout/stderr conventions; verified by `repo --help` listing them and by the integration test.
- [ ] CHK-002 Match resolution algorithm parity: Implementation in `src/cmd/repo/path.go`'s shared resolver follows `cli-surface.md` §"Match Resolution Algorithm" exactly — case-insensitive substring on Name only, exactly-1 short-circuit, fzf with `--query`/`--select-1`/`--height 40%`/`--reverse`/`--with-nth 1`/`--delimiter '\t'` flags.
- [ ] CHK-003 `repo cd` binary form prints exact hint and exits 2: stderr text byte-matches the spec scenario; verified by integration test.
- [ ] CHK-004 `-v` and `--version` work, `version` subcommand tolerated: Both flag forms print non-empty version; verified by smoke test.
- [ ] CHK-005 Lazy external-tool checks: `repo ls`, `repo shell-init zsh`, `repo config init`, `repo config path`, and exact-match resolution paths do not preempt fzf/git/code/open availability; verified by tests that disable fzf/git on the test path.
- [ ] CHK-006 `repo shell-init zsh` emits a working zsh integration: emitted text contains a `repo()` function intercepting `cd` and a `compdef _repo repo` line; runtime `eval` (when zsh available) defines `repo` as a function.
- [ ] CHK-007 Search order conformance: `internal/config.Resolve()` checks `$REPOS_YAML` → `$XDG_CONFIG_HOME/repo/repos.yaml` → `$HOME/.config/repo/repos.yaml` in order; bash-script paths (`$DOTFILES_DIR`, hardcoded dotfiles path) are absent.
- [ ] CHK-008 YAML schema conformance: Loader uses `gopkg.in/yaml.v3`; Name derivation strips `.git`; `~` directory keys expand to `$HOME`; `Path = Dir + "/" + Name`.
- [ ] CHK-009 `repo config init` writes mode 0644 and refuses overwrite: file mode confirmed via `os.Stat` in test; existing file produces exact "already exists" stderr and exit 1.
- [ ] CHK-010 `repo config path` is non-load and never errors on missing file: prints resolved path (regardless of file existence), does not trigger the missing-file hard error.
- [ ] CHK-011 Source tree matches reference layout: `src/go.mod` is at `src/`, module path is `github.com/sahil87/repo`, `cmd/repo/<verb>.go` exists, flat `internal/<pkg>/` (config, repos, fzf, proc, platform), tests adjacent, `testdata/` per-package.
- [ ] CHK-012 Centralized exec via `internal/proc`: production code (`src/cmd`, `src/internal/{config,repos,fzf,platform}`) does not import `os/exec` directly; only `internal/proc/proc.go` does.
- [ ] CHK-013 Cross-platform isolation via build tags: `open_darwin.go` and `open_linux.go` carry the correct build tags; `cd src && GOOS=darwin GOARCH=arm64 go build ./...` and `cd src && GOOS=linux GOARCH=amd64 go build ./...` both succeed.
- [ ] CHK-014 Cobra wiring conventions: each subcommand is a `func newXxxCmd() *cobra.Command` factory in its own file; root sets `SilenceUsage = true` and `SilenceErrors = true`; bare-form delegates to the path handler.
- [ ] CHK-015 Justfile is one-line recipes: `just --list` shows `default`, `build`, `install`, `test`; no `release` recipe in v0.0.1; recipes delegate to `scripts/`.
- [ ] CHK-016 build.sh injects version from git describe: script byte-matches spec; built binary's `--version` reports the `git describe` output.
- [ ] CHK-017 install.sh copies to `~/.local/bin/repo`: installed binary is byte-identical to `./bin/repo`; idempotent re-install overwrites.
- [ ] CHK-018 `bin/` is gitignored: `git check-ignore bin/` returns success; `git status --porcelain` after `just build` shows no `bin/` lines.

## Behavioral Correctness

- [ ] CHK-019 `$REPOS_YAML` set but file missing → hard error: stderr matches `repo: $REPOS_YAML points to <path>, which does not exist. Set $REPOS_YAML to an existing file or unset it.`; exit 1; candidates 2 and 3 are NOT consulted.
- [ ] CHK-020 No fallback paths: dropped `$DOTFILES_DIR/repos.yaml` and `$HOME/code/bootstrap/dotfiles/repos.yaml` from search order — no occurrence in `internal/config/resolve.go`.
- [ ] CHK-021 Bare-form delegates to path handler: `repo foo` produces same stdout and exit code as `repo path foo`.
- [ ] CHK-022 Empty `repos.yaml` loads as zero repos: no error; `repo ls` prints nothing and exits 0.
- [ ] CHK-023 Malformed YAML produces parse error with file context: stderr contains the file path and a line number; exit 1.

## Scenario Coverage

- [ ] CHK-024 Bare picker scenario: integration or unit test asserting `repo` (no args) invokes fzf with all entries.
- [ ] CHK-025 Unique substring match short-circuits fzf: test asserts fzf is NOT invoked when name resolves to exactly one repo.
- [ ] CHK-026 Ambiguous substring match invokes fzf with --query and --select-1: argv assertion in fzf wrapper test.
- [ ] CHK-027 Zero substring match invokes fzf with --query: same path; user can clear query to see all repos.
- [ ] CHK-028 `repo cd` shell-function form (post-eval): zsh test (or grep on emitted text) confirms `repo()` definition calls `command repo path` and `cd --`.
- [ ] CHK-029 `repo clone` single (missing path): `clone:` line on stderr; `git clone` invoked with the URL and target path; exit code matches git's.
- [ ] CHK-030 `repo clone` single (already cloned): `skip: already cloned at <path>` on stderr; exit 0.
- [ ] CHK-031 `repo clone` single (path exists, not git): exact stderr `repo clone: <path> exists but is not a git repo`; exit 1.
- [ ] CHK-032 `repo clone --all`: per-repo lines printed; final summary `summary: cloned=<N> skipped=<M> failed=<F>`; exit 0 if `failed == 0` else non-zero.
- [ ] CHK-033 `repo ls`: aligned `name<spaces>path` columns for every repo; exit 0.
- [ ] CHK-034 `repo shell-init` missing arg / unsupported shell: exact stderr from spec; exit 2.
- [ ] CHK-035 `repo --version` / `-v`: both print a single non-empty line; exit 0.
- [ ] CHK-036 `repo config init` creates starter at XDG path with mode 0644.
- [ ] CHK-037 `repo config init` refuses to overwrite existing file.
- [ ] CHK-038 `repo config path` prints path even when file is missing (env set or HOME-based); exit 0.

## Edge Cases & Error Handling

- [ ] CHK-039 Ambiguous match without fzf produces install hint: `repo: fzf is not installed. Install it: brew install fzf (macOS) or apt install fzf (Debian).`; exit 1.
- [ ] CHK-040 `git` missing during clone: exact stderr `repo: git is not installed.`; exit 1.
- [ ] CHK-041 `code` missing during `repo code`: exact stderr per spec; exit 1.
- [ ] CHK-042 `open`/`xdg-open` missing during `repo open`: exact stderr per spec; exit 1.
- [ ] CHK-043 All three config candidates resolve to nothing: exact stderr `repo: no repos.yaml found. ...`; exit 1.
- [ ] CHK-044 `~` expansion only at directory key's leading position: `~/foo` expands; `/etc/~weird` does not. (Verified by `repos_test.go` assertion.)
- [ ] CHK-045 Empty `<query>` to fzf: invocation does NOT include `--query` flag (or includes `--query ""` if the implementation chooses; assert one or the other consistent with `internal/fzf/fzf.go`).

## Code Quality

- [ ] CHK-046 Pattern consistency: New code follows naming and structural patterns of surrounding code; cobra factories named `newXxxCmd`, files named after the verb (`path.go`, `code.go`, ...), test files adjacent.
- [ ] CHK-047 No unnecessary duplication: The shared resolver in `path.go` is used by `code.go`, `open.go`, `cd.go`, and `clone.go` (no copy-pasted resolution logic). Config loading uses `internal/config.Load` (not re-implemented).
- [ ] CHK-048 Readability over cleverness: helper functions stay single-purpose; control flow uses early returns; no clever tricks (per `code-quality.md` Principles).
- [ ] CHK-049 Existing project patterns followed: layout mirrors `~/code/sahil87/fab-kit/src/go/wt/` per Spec Assumption #6.
- [ ] CHK-050 No god functions: no function exceeds 50 lines without a clear reason (per `code-quality.md` Anti-Patterns).
- [ ] CHK-051 No duplicated utilities: resolver helpers, error message constants, and yaml-fixture writers exist in exactly one location.
- [ ] CHK-052 No magic strings/numbers: error messages and fzf flags are package-level constants where reused; subcommand names are consts in their respective files.

## Security

- [ ] CHK-053 All exec uses `exec.CommandContext` with explicit argument slices: `grep --include='*.go' --exclude='*_test.go' -rn 'exec\.Command\b' src/` returns zero matches.
- [ ] CHK-054 `os/exec` import audit: `grep --include='*.go' --exclude='*_test.go' -rn '"os/exec"' src/internal/ src/cmd/` returns matches restricted to `src/internal/proc/`.
- [ ] CHK-055 User input passed to subprocess as args (not shell strings): repo names from `repos.yaml`, search queries from CLI args — all passed via `proc.Run`/`proc.RunInteractive` arg slices; no `sh -c`, no `os/exec.Command(... shell ...)` anywhere.
- [ ] CHK-056 fzf input via stdin, not args: search queries are passed as `--query <q>` (a single arg) and the candidate list is piped via stdin — no shell injection path through fzf.
- [ ] CHK-057 Embedded starter content does not leak credentials: `internal/config/starter.yaml` contains only the public `git@github.com:sahil87/repo.git` URL — no tokens, no private paths.
