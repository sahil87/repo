# Intake: Flip to repo-first grammar

**Change**: 260506-koa2-flip-to-repo-first-grammar
**Created**: 2026-05-06
**Status**: Draft

## Origin

> I am thinking of simplifying the hop command. How is this idea? The first argument is always the repo. So we switch from "hop code outbox" to "hop outbox code" -> does this simplify things?

Initiated as a `/fab-discuss` → `/fab-new` flow. The user proposed flipping the two-arg grammar so the **repo name always lives in `$1`** instead of `$2`. We walked the surface in `docs/specs/cli-surface.md`, the shim's 9-step precedence ladder in `src/cmd/hop/shell_init.go::posixInit`, and the `extractDashR` pre-cobra interceptor in `src/cmd/hop/main.go`. We also examined the recently-merged shim sugar (PR #9, commits `6f02e16` and `c148060`) which added `hop <tool> <name>` tool-form and the in-flight `260504-yr9l-complete-repo-names-second-arg` change which adds `$2` repo-name completion. The user explicitly **rejects yr9l in favour of this flip** — yr9l's premise (completion in `$2`) is dissolved by the flip (repo lives in `$1`, where completion already works).

Key design decisions made during the discussion:

- **Pure flip (Option A)** chosen over additive flip (both forms work) or partial flip (selective verbs only). Rationale: this is a cleanup change whose entire value proposition is removing the 3-way overload of `$1` (subcommand|repo|tool); preserving the old form would re-introduce the overload.
- **`hop open` is removed entirely.** Cross-platform abstraction (`open` on Darwin / `xdg-open` on Linux) is dropped. Users invoke the platform-native binary directly via tool-form: `hop outbox open` (Darwin), `hop outbox xdg-open` (Linux). Per Constitution Principle VI ("Minimal Surface Area") and Principle IV ("Wrap, Don't Reinvent"), once tool-form is the canonical way to run a binary in a repo, the `open` subcommand is redundant special-casing.
- **No verb-on-repo sugar (no step 5d).** `hop outbox where` does NOT route to `hop where outbox`. Subcommands stay strictly verb-first. Users who want `hop where outbox` (path lookup) type that exact form. Adding sugar later is non-breaking; removing it would be breaking — start tight.
- **`hop outbox pwd` is allowed to "just work."** It execs `/bin/pwd` with `cwd=<outbox-path>`, which prints the absolute path. This is a happy accident of the simpler grammar and is functionally equivalent to `hop outbox` (bare-name) and `hop where outbox`. No special handling — the grammar earns its simplicity by accepting the redundancy.
- **Clean break, no compatibility shim.** Consistent with the v0.x policy already in effect (`path` → `where`, `config path` → `config where`, `hop code` → tool-form, all without aliases). Users who have `hop code outbox` muscle memory get a cobra error; the muscle memory rebuilds in days.

## Why

### 1. The pain point

The hop binary has a deceptively simple grammar — but the shim that fronts it is hiding a 3-way overload of `$1`:

- `$1` is a **flag** (`-R`, `-h`, `--version`, ...) — forwarded to the binary
- `$1` is a **known subcommand** (`cd`, `clone`, `where`, `ls`, `open`, `shell-init`, `config`, `update`, `help`, `completion`) — dispatched
- `$1` is a **repo name** (1-arg form: `hop outbox` → `cd outbox`)
- `$1` is a **tool on PATH** (2-arg form: `hop cursor dotfiles` → `command hop -R dotfiles cursor`)

The last two compete: `hop cursor` could mean "cd into the cursor repo" or "run the cursor binary." The shim resolves it via a 9-step precedence ladder (`shell_init.go:14-31`, `cli-surface.md:156-202`) with explicit "repo wins over tool with 1 arg" and "subcommand wins over tool" rules, plus two cheerful-error escape hatches for builtin/keyword detection (`pwd` is a shell builtin, not a binary) and tool-form typo detection (`hop nonexistent dotfiles`). Design Decisions #10, #11, and #12 in `cli-surface.md` exist solely to justify these workarounds.

Concretely, the shim's `hop()` function is **~100 lines** (`shell_init.go:35-104`) of which ~50 lines exist purely to disambiguate the overload. The spec in `cli-surface.md` dedicates ~60 lines (lines 144-202) to documenting the precedence ladder and edge cases. Tab completion is the secondary symptom: yr9l identified that today's `completeRepoNames` bails when `len(args) > 0`, so `hop -R <TAB>` and `hop cursor <TAB>` don't complete repos — both shapes put the repo in `$2`, where completion never fires.

### 2. The consequence of doing nothing

The complexity is paid by every contributor reading the shim and every user who hits an edge case. Specifically:

- New top-level subcommands (Constitution Principle VI gates these) compete with the tool namespace forever. Every new subcommand requires updating the case-list in `shell_init.go:48` AND the `Notes:` block in `rootLong`.
- The cheerful-error escape hatches (`shell_init.go:81-96`) are dead weight — they exist because the grammar puts users into traps the grammar shouldn't have created.
- yr9l would add another ~30-50 lines to `completeRepoNames` and `main.go::main` to make completion work in `$2`, layering more code on top of a grammar that's the actual problem.
- The "repo wins over tool with 1 arg" rule (step 5 vs step 6 in the ladder) means `hop cursor` and `hop cursor dotfiles` interpret `$1` differently — the *same token in the same slot* changes meaning when a second arg is added. That's a real footgun.

### 3. Why this approach over alternatives

The flip eliminates the overload at its root. After the flip:

- `$1` is **either a known subcommand or a repo name** (2-way, not 3-way). The "is `$1` a tool on PATH?" check disappears entirely.
- `$2` (in the tool-form case) is **unambiguously a tool**. No competing interpretation, no precedence rule needed.
- Tab completion for the repo slot already works for `$1` — no new completion code needed.
- The shim's precedence ladder collapses from **9 steps to 4**:
  ```
  1. $# == 0                              → bare picker
  2. $1 is __complete*, flag, or subcmd   → forward to binary / dispatch
  3. $# == 1                              → bare-name cd ($1 is the repo)
  4. $# >= 2                              → tool-form: command hop -R "$1" "$2" "${@:3}"
                                            (special case: if $2 == "-R", use command hop -R "$1" "${@:3}")
  ```
- Builtin/keyword filtering (Decision #12) is deleted — `hop dotfiles pwd` execs `/bin/pwd` cleanly, no special handling needed.
- Cheerful-error escape hatches (steps 7 and 8) are deleted — the binary's `-R: '<cmd>' not found` error is good enough.
- The `hop open` subcommand and the `internal/platform` package are deleted — tool-form covers the use case generically.

Alternatives considered and rejected:

- **Additive flip (both forms work)**: shim has to detect "does `$1` look like a repo?" before tool-form. Adds complexity instead of removing it. Defeats the point.
- **Partial flip (selective verbs)**: worst of both worlds — two grammars, neither pure, more cognitive load.
- **Land yr9l first, then flip**: yr9l's code gets deleted by the flip. Wasteful; reject yr9l.

The chosen approach is the simplest possible: invert the argument order in the shim's tool-form rewrite and in `extractDashR`, delete the dead code, delete `hop open`, update the spec.

## What Changes

### Change 1 — Shim: collapse precedence ladder to 4 steps

`src/cmd/hop/shell_init.go::posixInit` (the body of the `hop()` shell function) is rewritten to the new ladder:

```sh
hop() {
  if [[ $# -eq 0 ]]; then
    command hop
    return $?
  fi
  case "$1" in
    __complete*)
      command hop "$@"
      ;;
    cd|clone|where|ls|shell-init|config|update|help|--help|-h|--version|completion)
      _hop_dispatch "$@"
      ;;
    -*)
      command hop "$@"
      ;;
    *)
      # $1 is a repo name (or will be treated as one).
      if [[ $# -eq 1 ]]; then
        _hop_dispatch cd "$1"
      elif [[ "$2" == "-R" ]]; then
        # Canonical exec form: hop <name> -R <cmd>...
        command hop -R "$1" "${@:3}"
      else
        # Tool-form sugar: hop <name> <tool> [args...]
        command hop -R "$1" "$2" "${@:3}"
      fi
      ;;
  esac
}
```

Note: the `open` token is removed from the known-subcommand case-list (Change 4 below removes the subcommand). Everything else in `posixInit` (the `_hop_dispatch` helper, the `h()` and `hi()` aliases, the comment header) is preserved with a rewritten comment block explaining the new 4-step ladder.

Deleted from today's posixInit:

- The leading-slash check on `command -v "$1"` (lines 64-66)
- The builtin/keyword/alias/function detection branch with `type "$1"` and the 3-line cheerful error (lines 70-88)
- The "not on PATH" cheerful-error branch (lines 89-96)
- The `$2`-is-flag fallback (lines 97-100)
- The "repo wins over tool" comment block (lines 27-31, 56-59)

Estimated deletion: ~50 lines of shim body, ~18 lines of comment header.

### Change 2 — `main.go::extractDashR`: scan for `-R` after the name, not before

`extractDashR` today scans argv for a leading `-R` global flag and treats the next token as `<name>`, then everything after as `<cmd>...`. After the flip, the canonical form becomes `hop <name> -R <cmd>...`, so the function flips to:

- Walk argv looking for `-R` (or `-R=<value>`, kept for the `=`-form)
- Take the **preceding** token as `<name>` (must exist; else usage error)
- Take **everything after** `-R` as `<cmd>...` (must be non-empty; else usage error)

Edge cases preserved:

- `hop -R` alone → usage error (no name, no cmd)
- `hop outbox -R` → usage error ("hop: -R requires a command to execute")
- `hop outbox -R=git status` → equivalent to `hop outbox -R git status`
- `hop outbox -R cmd` where `cmd` is not on PATH → exit 1 with `hop: -R: 'cmd' not found.`

`dashr_test.go` flips its argv inputs accordingly. The shim already passes the new shape via Change 1, so the binary and shim agree.

Help text in `cli-surface.md` and `rootLong` (in `root.go`) updates to show the new form.

### Change 3 — Subcommand list update

The shim's known-subcommand case-list in `posixInit` shrinks by one entry: `open` is removed. The remaining list:

```
cd|clone|where|ls|shell-init|config|update|help|--help|-h|--version|completion
```

`completion` stays in the list (cobra's auto-generated `hop completion bash/zsh/fish/...` subcommand — unrelated to our shell-init).

### Change 4 — Delete `hop open` and the `internal/platform` package

Files to delete:

- `src/cmd/hop/open.go` (entire file)
- `src/cmd/hop/open_test.go` (entire file)
- `src/internal/platform/platform.go`
- `src/internal/platform/open_darwin.go`
- `src/internal/platform/open_linux.go`
- `src/internal/platform/platform_test.go`

Files to update:

- `src/cmd/hop/root.go` — remove the `rootCmd.AddCommand(newOpenCmd())` line
- `src/cmd/hop/main.go` — remove any platform package imports if present
- `docs/specs/cli-surface.md` — remove `hop open` row, scenario, and external-tool-availability row
- `docs/specs/architecture.md` — remove the `internal/platform` and `open.go` rows from the layout tree
- `docs/memory/cli/subcommands.md` — remove the `hop open` row and any cross-references
- `docs/memory/architecture/package-layout.md` — remove the `internal/platform` entry
- `docs/memory/architecture/wrapper-boundaries.md` — remove platform/open references

### Change 5 — Spec rewrite (`docs/specs/cli-surface.md`)

Substantive edits:

- **Subcommand inventory table**: remove the `hop open` row; flip the `hop -R` row to `hop <name> -R <cmd>...`; flip the shim sugar row to `hop <name> <tool> [args...]`.
- **Behavioral scenarios**: flip the `-R` exec-in-context scenarios (lines 120-142) to use the new argv shape; flip the tool-form scenarios (lines 168-202) to use `hop <name> <tool>` and replace the cheerful-error scenarios (lines 184-202) with a single note that "if `<tool>` is not on PATH, the binary emits `hop: -R: '<tool>' not found`"; delete the `hop open` scenario.
- **Match resolution algorithm**: remove `hop open` from the list of subcommands using it.
- **Stdout/stderr conventions**: remove the `hop open` references.
- **External tool availability table**: remove the `open`/`xdg-open` row.
- **Design Decisions**: delete #10 (shim-only sugar precedence ladder), #11 (`hop code` removal — superseded by the broader flip), #12 (builtin filtering); rewrite the surrounding decisions to reflect the new grammar; renumber.
- Add a new design decision: **"Grammar is `subcommand` xor `repo`. The first positional is one or the other — never a tool. This collapses the shim's precedence ladder, eliminates builtin filtering, and makes tab completion work in the repo slot for free."**

### Change 6 — Memory updates

`docs/memory/cli/subcommands.md` and `docs/memory/cli/match-resolution.md` are rewritten to reflect the new grammar. Specifically:

- Drop the "Tool-form dispatch" section's precedence ladder (replaced with the 4-step description above).
- Drop the cheerful-error documentation.
- Remove the `hop open` row from the inventory.
- Add a "Removed subcommands" entry for `open`.
- Update the `hop -R` row to show the new argv shape.

`docs/memory/architecture/package-layout.md` and `docs/memory/architecture/wrapper-boundaries.md` lose the `internal/platform` entries.

### Change 7 — Help text in `rootLong`

Update `src/cmd/hop/root.go::rootLong` to:

- Remove the `hop open <name>` row from the Usage table.
- Update the `hop -R` row to `hop <name> -R <cmd>...`.
- Update the shim sugar row to `hop <name> <tool> [args...]`.
- Update the `Notes:` block to describe the new tool-form (remove the builtin-filtering note since it's no longer relevant).

### Change 8 — Reject yr9l

The change `260504-yr9l-complete-repo-names-second-arg` is rejected in favor of this flip. Its motivation (tab completion for repo names in `$2`) is dissolved because the flip moves the repo to `$1`, where completion already works. Action: archive yr9l via `fab archive yr9l` after this intake is created.

### Sample interactions (post-flip)

```
$ hop                          # fzf picker (unchanged)
$ hop outbox                   # cd into outbox (shim) / print path (binary) (unchanged)
$ hop where outbox             # print outbox path (unchanged)
$ hop ls                       # list repos (unchanged)
$ hop config init              # bootstrap config (unchanged)

# Flipped:
$ hop outbox cursor            # was: hop cursor outbox — run cursor in outbox
$ hop outbox git status        # was: hop -R outbox git status (canonical) or hop git outbox status (sugar — ambiguous, didn't work)
$ hop outbox -R git status     # was: hop -R outbox git status — canonical form (only needed when $2 is also a hop subcommand or flag)
$ hop dotfiles code            # was: hop code dotfiles — open dotfiles in VS Code

# Removed:
$ hop open outbox              # ERROR (subcommand removed) — use: hop outbox open (Darwin) or hop outbox xdg-open (Linux)

# "Just works" (intentional):
$ hop outbox pwd               # execs /bin/pwd with cwd=<outbox-path> — prints outbox's absolute path
```

## Affected Memory

- `cli/subcommands`: (modify) Inventory table flips `hop -R` and shim-sugar rows; `hop open` row removed; tool-form precedence ladder section rewritten; cheerful-error documentation removed.
- `cli/match-resolution`: (modify) Drop `hop open` from the list of subcommands using the algorithm.
- `architecture/package-layout`: (modify) Remove `internal/platform/` and `open.go`/`open_test.go` entries.
- `architecture/wrapper-boundaries`: (modify) Remove the platform/`open`+`xdg-open` wrapper boundary entry.

## Impact

**Source code (deletions):**

- `src/cmd/hop/open.go` — entire file deleted
- `src/cmd/hop/open_test.go` — entire file deleted
- `src/internal/platform/` — entire package deleted (4 files)
- `src/cmd/hop/shell_init.go::posixInit` — ~50 lines of shim body deleted, ~18 lines of comment header simplified
- `src/cmd/hop/main.go::extractDashR` — flipped (similar size, but logic inverted)

**Source code (modifications):**

- `src/cmd/hop/root.go` — remove `AddCommand(newOpenCmd())`, update `rootLong` help
- `src/cmd/hop/main.go` — flip `extractDashR`, drop platform import if present
- `src/cmd/hop/dashr_test.go` — flip argv inputs in tests
- `src/cmd/hop/shell_init.go` — rewrite `posixInit`, drop `open` from case-list
- `src/cmd/hop/shell_init_test.go` — update fixture expectations for the new shim body
- `src/cmd/hop/integration_test.go` — flip any `-R` invocations and tool-form invocations to the new shape

**Documentation:**

- `docs/specs/cli-surface.md` — substantial edits per Change 5
- `docs/specs/architecture.md` — remove platform layout entries
- `docs/memory/cli/subcommands.md` and `docs/memory/cli/match-resolution.md` — flip examples, remove `hop open`
- `docs/memory/architecture/package-layout.md` and `docs/memory/architecture/wrapper-boundaries.md` — remove platform entries

**External-facing breaking changes:**

- `hop -R outbox git status` → must be rewritten as `hop outbox -R git status`
- `hop code dotfiles`, `hop cursor outbox` → must be rewritten as `hop dotfiles code`, `hop outbox cursor`
- `hop open outbox` → removed; use `hop outbox open` (Darwin) or `hop outbox xdg-open` (Linux)

Per v0.x policy, no compatibility shim is added. Users update their shell history and any aliases.

**Estimated net deletion**: ~200+ lines of code/spec/memory removed; ~50 lines of replacement code added.

**Constitution alignment:**

- Principle I (Security First) — unchanged; `proc.RunForeground` still wraps all subprocess execution.
- Principle IV (Wrap, Don't Reinvent) — strengthened: removing `internal/platform` means `open`/`xdg-open` are no longer wrapped; users invoke them through tool-form, which already wraps via `internal/proc`.
- Principle VI (Minimal Surface Area) — strengthened: one fewer top-level subcommand (`open`); the binary's grammar is simpler.

## Open Questions

(None outstanding — the discussion resolved the major decisions: pure flip vs. additive, `hop open` removal, no verb-on-repo sugar, accept `hop outbox pwd` redundancy. Remaining detail is implementation-level, deferred to spec/tasks.)

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Pure flip (Option A): `hop <name> <tool>` and `hop <name> -R <cmd>...` replace the existing forms. No additive support. | Discussed — user chose Option A explicitly over additive (B) and partial (C). | S:95 R:60 A:90 D:95 |
| 2 | Certain | `hop open` subcommand is removed entirely. `internal/platform` package is deleted. | Discussed — user agreed to switch to `hop outbox open` (Darwin) / `hop outbox xdg-open` (Linux) directly. | S:95 R:55 A:90 D:90 |
| 3 | Certain | Clean break, no compatibility aliases for the old `-R` and tool-form arg orders. | Consistent with v0.x policy already documented in cli-surface.md (no aliases for `path`→`where`, `config path`→`config where`, `hop code` removal). | S:90 R:60 A:95 D:95 |
| 4 | Certain | yr9l is rejected. Its premise (completion in `$2`) is dissolved by the flip. yr9l is archived after this intake lands. | User explicitly rejected yr9l in favour of this flip during the conversation. | S:100 R:75 A:95 D:100 |
| 5 | Certain | No verb-on-repo sugar. `hop outbox where` does NOT route to `hop where outbox`. | Discussed — user agreed to start tight; can loosen later (non-breaking) if desired. | S:90 R:80 A:90 D:90 |
| 6 | Certain | `hop outbox pwd` is allowed to "just work" — execs `/bin/pwd`, prints outbox's path. No special handling. | Discussed — accepted as a happy accident of the simpler grammar; functionally equivalent to `hop outbox` and `hop where outbox`. | S:85 R:90 A:90 D:85 |
| 7 | Confident | The shim's precedence ladder collapses from 9 steps to 4 (`bare picker`, `forward subcmds/flags/__complete`, `bare-name cd`, `tool-form rewrite`). | Derived directly from the new grammar's invariants — no overload of `$1` means no precedence rule needed. Verified against today's `posixInit` body line-by-line. | S:80 R:65 A:80 D:75 |
| 8 | Confident | `extractDashR` flips: scan for `-R`, take preceding token as `<name>`, take following tokens as `<cmd>...`. | Symmetric inversion of today's logic; same edge cases (missing name, missing cmd, `-R=value`) apply. | S:80 R:60 A:85 D:80 |
| 9 | Confident | Builtin/keyword filtering (Design Decision #12) and the cheerful-error escape hatches (steps 7-8 in today's ladder) are deleted. | These exist only because today's grammar overloads `$1`. Once `$1` is unambiguously a repo, `hop dotfiles pwd` is a clean tool-form invocation; the binary's `-R: 'cmd' not found` covers genuine errors. | S:80 R:75 A:80 D:80 |
| 10 | Confident | Tab completion for the repo slot works after the flip with no new code — repos always live in `$1`, where `completeRepoNames` already fires. | Verified: today's completion works for `hop <TAB>`, `hop where <TAB>`, `hop cd <TAB>`, `hop open <TAB>`, `hop clone <TAB>`. The flip moves the tool-form repo from `$2` to `$1`, hitting the existing path. | S:85 R:80 A:85 D:85 |
| 11 | Confident | Estimated net deletion: ~200+ lines (code + spec + memory), replaced by ~50 lines. | Walked through `shell_init.go::posixInit` (~50 LOC of shim body), `cli-surface.md` lines 144-202 (~60 LOC of spec), Design Decisions #10-12 (~30 LOC), `internal/platform` package (~50-80 LOC), `open.go` + `open_test.go` (~50 LOC). | S:75 R:90 A:75 D:80 |
| 12 | Confident | Help text in `rootLong` (root.go) and `Notes:` block need updating to reflect new shapes; users discovering hop via `hop -h` see the new grammar. | Mechanical follow-on of Change 1 and 4. | S:90 R:85 A:90 D:85 |

12 assumptions (6 certain, 6 confident, 0 tentative, 0 unresolved).
