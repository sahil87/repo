# Spec: Add `hop config print` subcommand

**Change**: 260507-q437-config-print-subcommand
**Created**: 2026-05-07
**Affected memory**: `docs/memory/cli/subcommands.md`

## Non-Goals

- Pretty-printing, colorization, or paginated TUI display — `print` streams raw bytes; users compose with `bat`, `less`, `yq`, etc. as they would for any other text source.
- Emitting a normalized/effective config (defaults applied, fields canonicalized). The user typed *current* config — interpreted as "what's on disk", not "what hop sees after parsing". Adding a separate `--effective` flag is a future change, not this one.
- Validating the YAML before emission. `hop config print` SHALL be a transparent reader; if `hop.yaml` has a parse error, that surfaces only when other subcommands try to load it. Treating the file as opaque bytes preserves that boundary.
- Changing `hop config where`. The two verbs answer different questions ("which file?" vs. "what's in it?") and remain orthogonal.
- Shim ladder changes. `print` is at `$2` of `hop config`, not at `$1`, so the shim's known-subcommand list (which contains `config`) routes the call unchanged.

## CLI: `hop config print`

### Requirement: Subcommand registration
The binary SHALL expose `hop config print` as a sibling of `hop config init`, `hop config where`, and `hop config scan`. The cobra factory SHALL be named `newConfigPrintCmd` and registered alongside the existing factories in `newConfigCmd()` (`src/cmd/hop/config.go`). `newConfigCmd()`'s `Short` field SHALL be updated from `"config helpers (init, where, scan)"` to `"config helpers (init, where, scan, print)"` so `hop --help` and `hop config --help` list the new verb.

#### Scenario: Subcommand appears under `hop config --help`
- **GIVEN** the binary is built from this change
- **WHEN** I run `hop config --help`
- **THEN** stdout lists `print` alongside `init`, `where`, and `scan`
- **AND** the `Short` line for `print` reads `print the resolved hop.yaml contents to stdout`

#### Scenario: Subcommand appears in `hop --help` (root)
- **GIVEN** the binary is built from this change
- **WHEN** I run `hop --help`
- **THEN** the `config` subcommand's short-line lists `print` (the value comes from `newConfigCmd().Short`)

### Requirement: Argument and flag surface
`hop config print` SHALL accept zero positional arguments (`cobra.NoArgs`). It SHALL define no flags. Extra positionals or unknown flags MUST be rejected by cobra with a non-zero exit before `RunE` runs.

#### Scenario: Extra positional is rejected
- **GIVEN** the user runs `hop config print foo`
- **WHEN** the cobra parser inspects argv
- **THEN** cobra returns an `accepts 0 arg(s), received 1` error
- **AND** exit code is non-zero (cobra default — surfaced by `translateExit` as exit 1)

#### Scenario: Unknown flag is rejected
- **GIVEN** the user runs `hop config print --content`
- **WHEN** cobra parses the flag
- **THEN** cobra returns an unknown-flag error
- **AND** exit code is non-zero

### Requirement: Path resolution via `config.Resolve()`
`hop config print` SHALL resolve the config path using `internal/config.Resolve()` — the same resolver used by `hop` (bare picker), `hop ls`, `hop clone`, etc. It SHALL NOT use `config.ResolveWriteTarget()` (which is the writer-target query reserved for `hop config init` and `hop config where`). The semantic distinction matters: `Resolve()` returns an existing-file path or an error; `ResolveWriteTarget()` returns the path that *would* be used regardless of file existence. `print` is a reader and follows the reader contract.

#### Scenario: `$HOP_CONFIG` set, file exists
- **GIVEN** `$HOP_CONFIG` is set to `/tmp/foo/hop.yaml` and that file exists with content `repos:\n  default: []\n`
- **WHEN** I run `hop config print`
- **THEN** `Resolve()` returns `/tmp/foo/hop.yaml`
- **AND** the file's bytes go to stdout verbatim
- **AND** exit code is 0

#### Scenario: `$HOP_CONFIG` set, file missing
- **GIVEN** `$HOP_CONFIG` is set to `/tmp/no-such.yaml` and that file does not exist
- **WHEN** I run `hop config print`
- **THEN** `Resolve()` returns the error `hop: $HOP_CONFIG points to /tmp/no-such.yaml, which does not exist. Set $HOP_CONFIG to an existing file or unset it.`
- **AND** that error is propagated to stderr via the standard cobra path
- **AND** exit code is 1

#### Scenario: No `$HOP_CONFIG`, falls through to `$XDG_CONFIG_HOME`
- **GIVEN** `$HOP_CONFIG` is unset, `$XDG_CONFIG_HOME` is set to `/tmp/x` and `/tmp/x/hop/hop.yaml` exists
- **WHEN** I run `hop config print`
- **THEN** `Resolve()` returns `/tmp/x/hop/hop.yaml`
- **AND** that file's bytes go to stdout

#### Scenario: No config in any search location
- **GIVEN** `$HOP_CONFIG`, `$XDG_CONFIG_HOME`, and `$HOME/.config/hop/hop.yaml` all yield no existing file
- **WHEN** I run `hop config print`
- **THEN** `Resolve()` returns `hop: no hop.yaml found. Set $HOP_CONFIG to a tracked file ..., or run 'hop config init' to bootstrap one at <ResolveWriteTarget>.`
- **AND** stderr shows that message via the standard cobra path
- **AND** exit code is 1

### Requirement: Raw byte output
`hop config print` SHALL read the resolved file with `os.ReadFile` and write the raw bytes to `cmd.OutOrStdout()` verbatim. It SHALL NOT parse the YAML, normalize key order, strip comments, re-format whitespace, or apply default values. The trailing newline (if any) on disk is preserved; no synthetic trailing newline is appended if the file lacks one. No `# Source: <path>` header, no `---` separator, no framing.

The bytes-not-parse choice means `print` is comment-preserving and lossless by construction, and is the natural complement to `init` (writes embedded starter bytes) and `scan --write` (uses `internal/yamled` for comment-preserving merge).

#### Scenario: Comments and formatting preserved
- **GIVEN** `hop.yaml` contains:
  ```yaml
  # top comment
  config:
    code_root: ~/code  # inline comment
  repos:
    default:
      - git@github.com:foo/bar.git
  ```
- **WHEN** I run `hop config print`
- **THEN** stdout matches the file exactly, byte-for-byte (top comment, inline comment, indentation, trailing newline preserved)

#### Scenario: Empty file is printed as empty stdout
- **GIVEN** `hop.yaml` exists but is zero bytes
- **WHEN** I run `hop config print`
- **THEN** stdout is empty (zero bytes written)
- **AND** exit code is 0
- **AND** no synthetic content is emitted

#### Scenario: Output is pipeable to a parser
- **GIVEN** a typical `hop.yaml` with `repos.default[0] = git@github.com:foo/bar.git`
- **WHEN** I run `hop config print | yq '.repos.default[0]'`
- **THEN** `yq` receives clean YAML on stdin (no framing, no header)
- **AND** outputs `git@github.com:foo/bar.git`

### Requirement: Read error surfacing
If `os.ReadFile` returns an error after `Resolve()` succeeded (e.g., the file was deleted between resolution and read; permission denied; I/O error), `hop config print` SHALL return a wrapped error of the form `hop config print: read <path>: <underlying error>`. The error SHALL propagate through the standard `translateExit` path (cobra surfaces it via stderr; exit code 1).

#### Scenario: Permission-denied read
- **GIVEN** `$HOP_CONFIG` points to a file the user cannot read (mode 000)
- **WHEN** I run `hop config print`
- **THEN** stderr shows `Error: hop config print: read <path>: open <path>: permission denied` (or the platform equivalent)
- **AND** exit code is 1

### Requirement: Stdout/stderr discipline
The file bytes SHALL go to stdout via `cmd.OutOrStdout()`. All errors SHALL go to stderr via cobra's standard return path (RunE returns the error; cobra writes it to `cmd.ErrOrStderr()`). The subcommand SHALL NOT write anything to stderr on the success path — no progress lines, no `# Reading from <path>` header, nothing. This matches `hop config where` (silent on success, only path on stdout) and the broader stdout-is-data convention documented in `docs/specs/cli-surface.md` § Stdout / stderr Conventions.

#### Scenario: Successful run produces no stderr
- **GIVEN** `$HOP_CONFIG` points to a valid `hop.yaml`
- **WHEN** I run `hop config print`
- **THEN** stdout is the file's bytes
- **AND** stderr is empty

### Requirement: No external tools
`hop config print` SHALL NOT invoke `git`, `fzf`, `code`, `open`, or any other external binary. It is a pure stdlib operation (`os.ReadFile` + write to stdout). The lazy-tool-check convention in `docs/specs/cli-surface.md` § External Tool Availability applies trivially — no entry needs to be added.

#### Scenario: Works in minimal environment
- **GIVEN** `fzf` and `git` are not installed
- **WHEN** I run `hop config print`
- **THEN** the command succeeds (no preflight fzf/git check)

### Requirement: Help text
The cobra command SHALL set `Short = "print the resolved hop.yaml contents to stdout"`. No `Long` is required (the behavior is fully captured by `Short` and the global `--help` rendering). `Args = cobra.NoArgs`.

#### Scenario: Short string visible in `hop config --help`
- **GIVEN** the binary is built
- **WHEN** I run `hop config --help`
- **THEN** the line for `print` reads exactly `print        print the resolved hop.yaml contents to stdout` (cobra's auto-aligned column format)

## CLI: Tests

### Requirement: Unit test coverage
The change SHALL add tests in `src/cmd/hop/config_test.go`:

1. **`TestConfigPrintEmitsFileBytes`** — write a fixture `hop.yaml` with comments and inline whitespace, set `$HOP_CONFIG`, run `hop config print`, assert stdout equals the on-disk bytes verbatim, assert stderr is empty, assert exit code 0.
2. **`TestConfigPrintMissingFileErrors`** — set `$HOP_CONFIG` to a non-existent path, run `hop config print`, assert exit error is non-nil, assert stderr contains `points to ... which does not exist`.
3. **`TestConfigPrintNoConfigErrors`** — clear `$HOP_CONFIG`, `$XDG_CONFIG_HOME`, set `$HOME` to a temp dir with no `~/.config/hop/hop.yaml`, run `hop config print`, assert error contains `no hop.yaml found`.
4. **`TestConfigPrintListedUnderConfigHelp`** — extend the existing `TestConfigScanListedUnderConfigHelp` (or add a new sibling) to assert `print` is also listed in `hop config --help` stdout.

Tests SHALL use the existing `runArgs` and `clearConfigEnv` helpers in `src/cmd/hop/testutil_test.go` to mirror the surrounding test idiom.

#### Scenario: Tests pass
- **GIVEN** the implementation is complete
- **WHEN** `go test ./src/cmd/hop/...` runs
- **THEN** the four new tests pass alongside all existing tests

## Documentation

### Requirement: Memory update
`docs/memory/cli/subcommands.md` SHALL gain one new row in the `## Inventory` table, between the existing `hop config where` row and the `hop config scan <dir>` row. The row SHALL follow the existing column convention (Subcommand, File, Args, Behavior). Behavior text SHALL be a single-paragraph summary mirroring the existing `hop config where` row's depth.

#### Scenario: Inventory row added in the right position
- **GIVEN** the hydrate stage runs after apply
- **WHEN** `docs/memory/cli/subcommands.md` is updated
- **THEN** the table contains a new row for `hop config print` immediately after the `hop config where` row and immediately before the `hop config scan <dir>` row

### Requirement: Spec update (cli-surface)
`docs/specs/cli-surface.md` SHALL gain a corresponding row in its `## Subcommand Inventory` table at the equivalent position (between `hop config where` and `hop config scan <dir>`), mirroring its existing column structure (Subcommand, Args, Behavior summary, Exit codes). The Help-text `Usage:` block enumeration listed in `docs/specs/cli-surface.md` § Help Text SHOULD include `hop config print` in the same relative position. (Spec is human-curated per the spec-index conventions; this is a SHOULD, not a MUST, since the test suite does not directly enforce the spec table.)

#### Scenario: Spec inventory row added
- **WHEN** the change is hydrated
- **THEN** `docs/specs/cli-surface.md` § Subcommand Inventory has a `hop config print` row with `Behavior summary` = "Print the resolved hop.yaml contents to stdout (raw bytes, comment-preserving)" and `Exit codes` = "0 success, 1 unresolvable / read error"

## Design Decisions

1. **Stream raw bytes, not parsed-and-re-emitted YAML.**
   - *Why*: User typed "current config" → "what's on disk". Comment-preserving by construction. Round-tripping through `yaml.Unmarshal` + `yaml.Marshal` would strip comments and re-order keys. Round-tripping through `yaml.Node` would preserve comments but at higher complexity for zero user benefit. Aligns with the comment-preserving discipline already established by `internal/yamled` (used by `scan --write` and `clone <url>` auto-registration).
   - *Rejected*: "Effective config" (parse + apply defaults like `code_root: ~`, re-emit). Loses comments, surprises users who expect to see what they wrote. If demand emerges, a future change can add `--effective` as an opt-in flag — it's reversible.

2. **Use `config.Resolve()`, not `config.ResolveWriteTarget()`.**
   - *Why*: Semantic alignment. Every read path (`hop`, `hop ls`, `hop clone`, `hop -R`) uses `Resolve()` and errors on missing file. `ResolveWriteTarget()` is the bootstrap-path query — its job is "give me a path to write to even if nothing's there yet" and `init`/`where` are the only legitimate users.
   - *Rejected*: `ResolveWriteTarget()` + `os.Stat` + custom error path. Reinvents `Resolve()`'s error surface (the existing `$HOP_CONFIG points to ... does not exist` and `no hop.yaml found ...` messages are well-tuned). Using `Resolve()` directly inherits those messages for free.

3. **Subcommand name is `print`, not `cat`/`show`/`view`/`dump`.**
   - *Why*: `print` is the conventional Go-CLI term for stdout emission (cobra docs, `kubectl get -o yaml`, `gcloud config config-helper`, `git config --list`). It pairs naturally with `init`/`where`/`scan` — none of those are Unix-tool names either.
   - *Rejected*: `cat` would be the only Unix-tool name in the inventory and feels colloquial. `show`/`view` imply pagination or a TUI. `dump` connotes verbose internals or debug output. None of those communicate "stream the file as-is" as cleanly as `print`.

4. **No flags.**
   - *Why*: Constitution Principle III (Convention Over Configuration). The action is parameter-free: read the resolved file, write its bytes. Anything users might want (filter, transform, paginate) is one Unix pipe away (`hop config print | yq`, `hop config print | bat`, `hop config print | grep -v '^#'`).
   - *Rejected*: `--path` (override `$HOP_CONFIG` ad-hoc) is what the env var already does. `--effective` (parsed mode) is out of scope per the Non-Goals section. `--no-comments` belongs in `yq` or `grep -v`, not in hop.

5. **`cobra.NoArgs` (not `cobra.MaximumNArgs(0)` or no constraint).**
   - *Why*: Idiomatic match against `init` and `where` (both use `cobra.NoArgs`). Surfaces `accepts 0 arg(s), received N` errors before `RunE`.
   - *Rejected*: Skipping the constraint and letting `RunE` ignore extras silently — bad UX, hides typos like `hop config print foo` (where the user might have meant `hop foo where`).

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Stream raw file bytes, not parsed-and-re-emitted YAML | Confirmed from intake #1 → upgraded to Certain. Spec-stage analysis: aligns with `internal/yamled` comment-preserving discipline; round-tripping through yaml.v3 has no upside for `print` (no merge, no normalization needed). Reversible via future `--effective` flag. | S:90 R:80 A:90 D:85 |
| 2 | Certain | Use `config.Resolve()` (errors on missing file), not `ResolveWriteTarget()` | Confirmed from intake #2 → upgraded to Certain. Every read path in the codebase uses `Resolve()`; `ResolveWriteTarget()` is reserved for `init`/`where` (writer-target queries). Inherits `Resolve()`'s well-tuned error messages for free. | S:90 R:90 A:95 D:90 |
| 3 | Certain | Subcommand name is `print` | Confirmed from intake #3 → upgraded to Certain after Design Decision review. Conventional in Go-CLI ecosystem (cobra docs, kubectl, gcloud, git config). Reversible by alias if user pushes back, but no evidence anyone prefers `cat`/`show`/`view`/`dump`. | S:80 R:80 A:90 D:85 |
| 4 | Certain | No flags | Confirmed from intake #4. Constitution Principle III. Pipe composition covers any transformation a user might want. | S:90 R:85 A:95 D:90 |
| 5 | Certain | Output → stdout via `cmd.OutOrStdout()`; errors → stderr via cobra; no framing/headers; silent on success | Confirmed from intake #5. Established hop convention; pipeability is the explicit reason. | S:90 R:90 A:95 D:90 |
| 6 | Certain | Wire as `newConfigPrintCmd()` sibling of `init`/`where`/`scan` in `newConfigCmd()`; update `Short` to mention `print` | Confirmed from intake #6. Direct pattern match against existing code. | S:95 R:95 A:95 D:95 |
| 7 | Confident | Memory: modify `cli/subcommands.md` Inventory only — also update `docs/specs/cli-surface.md` Inventory and `Usage:` block enumeration during apply | Upgraded scope from intake #7. Spec-stage discovery: `docs/specs/cli-surface.md` has its own Inventory table that should stay in sync (cli-surface is the canonical CLI contract). The spec is human-curated, but a SHOULD entry covers it. | S:80 R:75 A:80 D:80 |
| 8 | Certain | `cobra.NoArgs` | Confirmed from intake #8. Standard idiom (matches `init` and `where`). | S:95 R:90 A:95 D:95 |
| 9 | Certain | `Short = "print the resolved hop.yaml contents to stdout"` | New (spec-stage). Wording chosen for parallel structure with `where`'s `print the resolved hop.yaml path` Short. | S:85 R:85 A:90 D:85 |
| 10 | Certain | Empty file → empty stdout, exit 0 (no synthetic content) | New (spec-stage). Direct consequence of "stream raw bytes" — zero bytes in, zero bytes out. Matches `config.Load`'s zero-bytes-is-valid-empty-file handling. | S:85 R:90 A:95 D:90 |
| 11 | Confident | Read errors wrapped as `hop config print: read <path>: <underlying>` | New (spec-stage). Mirrors `config.Load`'s wrapping convention (`hop: read %s: %w`). Slight variation: prefix with subcommand name to disambiguate from loader errors elsewhere, since `print` is a thin wrapper that doesn't go through `config.Load`. | S:75 R:80 A:80 D:75 |
| 12 | Certain | No external tools (no fzf/git/etc. invocations) | New (spec-stage). Pure stdlib — `os.ReadFile` + write. Works in minimal environments. | S:95 R:95 A:95 D:95 |
| 13 | Confident | Test count: 4 new tests in `config_test.go` (file-bytes happy path, missing file, no config, help-listing) | New (spec-stage). Mirrors the depth of existing init/where tests (each has 2-3 tests covering happy path + error paths + help-listing). Could grow with more edge cases (permission denied, empty file) — judged additive, not blocking. | S:75 R:85 A:80 D:75 |
| 14 | Certain | Help-listing test extends or pairs with the existing `TestConfigScanListedUnderConfigHelp` pattern | New (spec-stage). Direct pattern match — that test already iterates over `[]string{"init", "where", "scan"}`; the natural change is to append `"print"` (or duplicate the test). | S:90 R:90 A:90 D:90 |
| 15 | Certain | Apply does NOT change the shim ladder in `shell_init.go` | New (spec-stage). `print` lives at `$2` of `hop config`; the shim's known-subcommand list at `$1` (`config|...`) routes the call unchanged. | S:95 R:95 A:95 D:95 |

15 assumptions (12 certain, 3 confident, 0 tentative, 0 unresolved).
