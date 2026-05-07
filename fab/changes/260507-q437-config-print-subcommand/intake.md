# Intake: Add `hop config print` subcommand

**Change**: 260507-q437-config-print-subcommand
**Created**: 2026-05-07
**Status**: Draft

## Origin

> Add a "hop config print" that prints the current config

One-shot natural-language input via `/fab-new`. No prior conversational context.

The user already has `hop config where` (prints the resolved path) and `hop config init` / `hop config scan` (writers). Today, to view the active config, a user runs `cat "$(hop config where)"` or pipes through `bat`. This change adds a single-purpose verb so the action is one command instead of a shell composition, and so the binary owns the "show me my config" affordance directly.

## Why

1. **The pain point.** Users who want to inspect their active `hop.yaml` (to confirm what got loaded, to share it in a bug report, to diff it against another machine) currently need to stitch two commands: `cat "$(hop config where)"`. That requires knowing both subcommands and shell-quoting the substitution. It also fails silently in non-obvious ways — e.g., if `$HOP_CONFIG` points at a missing file, `hop config where` happily prints the (missing) path and `cat` fails downstream with a generic "No such file or directory".

2. **What happens without it.** Nothing breaks; users keep using `cat` + command substitution. But `hop config print` is the kind of small affordance that a locator CLI is *for* — it converts a multi-step shell idiom into a single verb in the same namespace as the other config helpers. Discoverability matters: `hop config -h` should list every config-related action.

3. **Why this approach.** A new sibling subcommand (`init` | `where` | `scan` | `print`) is the natural extension point. The alternatives all fail one of the constitutional principles or established conventions:
   - **Add a flag to `where`** (`hop config where --content`) — overloads a single-purpose subcommand and conflates "show path" with "show contents". Fails Principle VI minimum-surface only superficially (one new verb vs. one new flag), but pays for itself in clarity.
   - **Pipe-style alias** (e.g., `hop config cat`) — `print` is the term used elsewhere in the Go CLI ecosystem (e.g., `kubectl config view`, `git config --list`); `cat` would be the only Unix-tool name in the subcommand inventory.
   - **Implicit `hop config` with no subcommand** — would conflict with cobra's "subcommand required" pattern used by `hop config` today and break `hop config -h` discoverability.

## What Changes

### New subcommand: `hop config print`

**Synopsis:**

```
hop config print
```

**Args**: none (`cobra.NoArgs`).
**Flags**: none.
**Behavior**: Resolves the active `hop.yaml` path via `config.Resolve()` (the same resolver used by `hop`, `hop ls`, `hop clone`, etc. — *not* `ResolveWriteTarget()` like `hop config where` uses). Reads the file as raw bytes. Writes the bytes verbatim to stdout. Preserves comments, formatting, and trailing newline exactly as they appear on disk.

**Why raw bytes (not parsed-and-re-emitted YAML):** The user's `hop.yaml` likely contains comments (the embedded starter ships with several) and curated formatting. Round-tripping through `yaml.Unmarshal` + `yaml.Marshal` would strip comments and re-order keys; round-tripping through `yaml.Node` would preserve them but adds complexity for no user benefit. Raw stream is simpler and round-trips perfectly.

**Stdout / stderr discipline:** Bytes go to stdout. No framing, no `# Source:` header — keeps output pipeable into `yq`, `grep`, `wc`, etc. Errors go to stderr via the standard cobra/`translateExit` path.

**Exit codes** (per `main.go::translateExit`):
- `0` — file resolved and printed.
- `1` — `config.Resolve()` returned `ErrNoConfig` (no file in search order); or `$HOP_CONFIG` points to a missing file (the hard-error branch); or read error.
- `2` — N/A (no usage errors possible — `cobra.NoArgs` rejects extra positionals before `RunE`).

**Help line** (`Short`): `print the resolved hop.yaml contents to stdout`.

### Code touch points

- **`src/cmd/hop/config.go`** — add `newConfigPrintCmd()` factory matching the shape of `newConfigWhereCmd()`. Register it in `newConfigCmd()` alongside `newConfigInitCmd, newConfigWhereCmd, newConfigScanCmd`. Update the `Short` on `newConfigCmd` from `"config helpers (init, where, scan)"` to `"config helpers (init, where, scan, print)"`.
- **`src/cmd/hop/config_test.go`** — add a small unit test that writes a fixture `hop.yaml`, sets `$HOP_CONFIG` to it, runs `hop config print`, and asserts stdout matches the file bytes. Also a missing-file negative case (`$HOP_CONFIG` set to a non-existent path → exit 1, stderr matches the existing `Resolve()` error message).
- **No new internal package.** Reuses `config.Resolve()` and `os.ReadFile`. No new external dependencies.
- **Shim ladder** (`shell_init.go::posixInit`) does NOT need a change. `print` is a verb at `$2` of `hop config print`, not at `$1`. The known-subcommand list in the shim is `clone|ls|shell-init|config|update|help|...`; `config` is already there, and the shim delegates to `_hop_dispatch` (which calls `command hop "$@"`), so `hop config print` flows through the binary unchanged.
- **Cobra completion** is auto-generated from the subcommand tree, so adding `print` to the `hop config` group adds tab-completion for free.

### Example usage

```sh
# Inspect the active config
$ hop config print
config:
  code_root: ~
repos:
  default:
    - git@github.com:sahil87/hop.git

# Pipe to a parser
$ hop config print | yq '.repos.default'

# Diff against a backup
$ hop config print | diff - ~/Dropbox/hop.yaml.backup
```

## Affected Memory

- `cli/subcommands.md`: (modify) Add a row for `hop config print` to the Inventory table, between the existing `hop config where` and `hop config scan <dir>` rows. No new sections needed — the behavior is fully described by the standard Inventory columns.

## Impact

- **Code**: One new ~15-line cobra factory in `src/cmd/hop/config.go` plus tests. No changes to `internal/config` (existing `Resolve` is sufficient).
- **APIs / dependencies**: None — uses stdlib (`os.ReadFile`) and the existing `internal/config` package.
- **Cross-platform**: No platform-specific code.
- **Test surface**: Adds tests; does not modify existing fixtures or table-driven cases.
- **Docs**: One row added to `docs/memory/cli/subcommands.md` Inventory. `docs/specs/cli-surface.md` may need a parallel row in its subcommand table (verify during spec stage).
- **Backwards compat**: Pure addition. No removed/renamed/aliased surface.
- **Constitutional alignment**: Principle II (No Database) — N/A; Principle III (Convention Over Configuration) — no new flags, derives everything from existing resolver; Principle IV (Wrap Don't Reinvent) — uses stdlib I/O; Principle VI (Minimal Surface Area) — adds one *sub*-subcommand within the existing `config` group, not a top-level verb. The "could this be a flag on an existing subcommand?" answer: yes (e.g., `hop config where --content`), but the overload would muddy `where`'s single purpose.

## Open Questions

None. The behavior is well-defined by the existing config-resolution patterns in the codebase.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Confident | Print raw file bytes (not parsed-and-re-emitted YAML) — preserves comments, formatting, key order. | Two valid interpretations of "current config" (raw vs. effective/parsed), but raw is simpler, comment-preserving, and matches user mental model. Reversible (could add `--effective` flag later). | S:65 R:75 A:80 D:75 |
| 2 | Confident | Use `config.Resolve()` (errors on missing file) rather than `ResolveWriteTarget()` (never errors). | Semantically "print the *current* config" implies one exists; consistent with every other read path (`hop`, `hop ls`, `hop clone`). `where` is the only command using the bootstrap-target resolver, and that's because it's a writer-target query. | S:70 R:90 A:90 D:85 |
| 3 | Confident | Subcommand name `print` (not `cat`, not `show`, not `view`). | `print` is the conventional Go-CLI term for stdout emission (cobra docs, kubectl, gcloud). `cat` would be the only Unix-tool name in the inventory. `show`/`view` imply pagination/TUI, neither of which we want. Reversible via alias if user pushes back. | S:60 R:80 A:85 D:65 |
| 4 | Certain | No flags. | Constitution Principle III: flags only when convention can't suffice. Raw-bytes-to-stdout has no parameters worth surfacing. | S:90 R:85 A:95 D:90 |
| 5 | Certain | Output goes to stdout; errors to stderr; no framing/headers. | Existing convention across all hop subcommands (clone status → stderr, resolved paths → stdout). Pipeability is the explicit reason. | S:85 R:90 A:95 D:90 |
| 6 | Certain | Wire as a sibling of `init`/`where`/`scan` in `newConfigCmd()`. Update `Short` to mention `print`. | Direct pattern match against existing code in `src/cmd/hop/config.go`. No structural choice to make. | S:90 R:95 A:95 D:95 |
| 7 | Confident | Memory update: modify `cli/subcommands.md` Inventory table only. | The behavior fits the standard table columns (Subcommand, File, Args, Behavior). No new sub-section needed. Spec-stage will reconfirm whether `docs/specs/cli-surface.md` needs a parallel update. | S:70 R:80 A:80 D:75 |
| 8 | Certain | `cobra.NoArgs`. | No args to accept. Standard cobra idiom for nullary subcommands (matches `init` and `where`). | S:95 R:90 A:95 D:95 |

8 assumptions (4 certain, 4 confident, 0 tentative, 0 unresolved).
