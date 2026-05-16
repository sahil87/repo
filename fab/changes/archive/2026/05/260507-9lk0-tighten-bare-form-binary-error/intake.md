# Intake: Unify repo-verb grammar — `hop <repo> <verb>`, drop `cd`/`where` subcommands

**Change**: 260507-9lk0-tighten-bare-form-binary-error
**Created**: 2026-05-07
**Status**: Draft

## Origin

> tighten bare-form: binary errors on `hop <name>` (1 arg), shim cd's. Enforce the rule that any given `hop <subform>` either errors in the binary or works in both — never one effect in the binary and a different effect in the shim. Mirror `hop cd`'s existing pattern. Also fix `hop -h` drift...
>
> Then think — what if we change the cd and where from to 'hop <repo> <command>'
>
> Full unification. But "hop <reponame>" - choose B2. Also update help.
>
> No need of backward compatibility.

Mode: conversational. The change started as a help-text drift report and an Option A scope (binary errors on `hop <name>`, shim cd's, mirror `hop cd`'s existing pattern). Through discussion, the user pushed further: rather than patching the binary-vs-shim asymmetry on one form (`hop <name>`), unify the entire repo-verb grammar. Drop `hop cd <name>` and `hop where <name>` as top-level subcommands. Replace with `hop <repo> <verb>` — `cd` and `where` become verbs at $2, joining the existing `-R` and tool-form sugar. The bare `hop <name>` (1 arg) is shorthand for `hop <name> cd` per option B2: cd in the shim, error in the binary. No backwards compat — clean v0.x break.

## Why

**The pain point.** Today, hop has three different shapes for "do thing X with repo Y":

1. `hop cd <name>` — `cd` is at $1 (subcommand position).
2. `hop where <name>` — `where` is at $1 (subcommand position).
3. `hop <name> -R <cmd>...` and `hop <name> <tool>...` — repo at $1, action at $2.

Same mental operation, two different argument orders. The shim already standardizes on repo-first for tool-form (`hop dotfiles cursor`); `cd` and `where` are the holdouts. New users learn two grammars; the code maintains two dispatch paths (cobra's subcommand routing for `cd`/`where`, the shim's repo-first ladder for everything else); tab completion is split (`completeRepoNames` runs at $1 *only* when `where`/`cd` aren't already there).

**On top of that, the same binary-vs-shim asymmetry the original Option A tackles.** `hop <name>` prints under the binary, cd's under the shim — two effects sharing one syntax. Option A fixes one form; full unification fixes the entire grammar.

**The consequence of leaving it.** The dual-grammar tax compounds with each new subcommand. The help block has already drifted (`config scan` missing from `Usage`; the `<name>` row is wrong under the shim; `where`'s "same, explicit form" is misleading). Constitution Principle VI (Minimal Surface Area) says new top-level subcommands need explicit justification; the inverse is also true — keeping ones that don't earn their slot is rot.

**Why this approach over alternatives.**

- **Stay with Option A only** (originally chosen): fixes the dichotomy for `hop <name>` but leaves `cd` and `where` at $1. Two grammars survive.
- **Drop `cd`, keep `where`**: collapses cd into bare-form; preserves `where` for scripts. One subcommand removed, but the grammar is still mixed.
- **Full unification (chosen)**: one grammar (`hop <repo> <verb>`), two fewer top-level subcommands, the shim's existing tool-form sugar pattern generalizes naturally. The cost: bigger break, slightly bigger code change, every script using `hop where <name>` migrates to `hop <name> where`. The user explicitly waived backwards-compat.

The new grammar is also more learnable: "first positional is a repo or a subcommand (mutually exclusive); when it's a repo, second positional is a verb (`cd`, `where`), `-R`, or a tool name." That sentence covers everything the user types in their daily flow.

## What Changes

### 1. New grammar surface

```
hop                          0 args → fzf picker, print selection (unchanged)
hop <name>                   1 arg, repo → cd (shim) / error (binary)        [B2]
hop <name> cd                2 args, $2=cd → cd (shim) / error (binary)
hop <name> where             2 args, $2=where → print path (both)
hop <name> -R <cmd>...       2+ args, $2=-R → exec child with cwd (canonical)
hop <name> <tool> [args]     2+ args, $2=any-other → shim sugar → -R rewrite
hop clone <name>             subcommand at $1 — unchanged
hop clone <url>              ad-hoc URL clone — unchanged
hop clone --all              bulk clone — unchanged
hop clone                    fzf picker + clone — unchanged
hop ls                       subcommand — unchanged
hop config init|where|scan   subcommands — unchanged (config keeps `where`)
hop shell-init <shell>       subcommand — unchanged
hop update                   subcommand — unchanged
hop -h | -v | --help         flags — unchanged
```

`hop config where` survives because it's `where` under `config`, a different namespace. The collapse only affects top-level `cd`/`where`.

### 2. Binary behavior at $2

The binary's rootCmd grows from `MaximumNArgs(1)` to `MaximumNArgs(2)`. Dispatch on the second positional:

- `args[1] == "where"` → resolve `args[0]` and print the path (binary form of `hop <name> where`).
- `args[1] == "cd"` → return `errExitCode{code: 2, msg: cdHint}`. The hint is updated to: `hop: 'cd' is shell-only. Add 'eval "$(hop shell-init zsh)"' to your zshrc, or use: cd "$(hop "<name>" where)"`.
- `args[1] == anything else` → return `errExitCode{code: 2, msg: toolFormHint}`. Tool-form is shim-only (Option X — keep it that way; do not absorb tool-form into the binary). Hint: `hop: tool-form '<tool>' is shell-only. Add 'eval "$(hop shell-init zsh)"' to your zshrc, or use: hop -R "<name>" <tool> [args...]`.

When `len(args) == 1` (bare-name, B2):
- Binary errors with `errExitCode{code: 2, msg: bareNameHint}`. Hint: `hop: bare-name dispatch is shell-only. Add 'eval "$(hop shell-init zsh)"' to your zshrc, or use: hop "<name>" where`.

When `len(args) == 0`: bare picker (unchanged).

### 3. File-level code changes

**Deleted:**
- `src/cmd/hop/cd.go` — `newCdCmd()` factory and `cdHint` constant.
- `src/cmd/hop/cd_test.go` — cobra-form tests for `hop cd`.

**Renamed:**
- `src/cmd/hop/where.go` → `src/cmd/hop/resolve.go`. Drop `newWhereCmd()` (the only cobra factory in the file). All resolution helpers (`loadRepos`, `resolveByName`, `buildPickerLines`, `resolveOne`, `resolveAndPrint`) stay — they're used by root.go and clone.go. The new filename describes what the file actually contains. Use `git mv` so history is preserved.
- `src/cmd/hop/where_test.go` → `src/cmd/hop/resolve_test.go`. Drop tests targeting `newWhereCmd`'s cobra surface; keep tests for the resolution helpers.

**Modified:**
- `src/cmd/hop/root.go`:
  - Drop `newCdCmd()` and `newWhereCmd()` from `cmd.AddCommand(...)`.
  - `Args: cobra.MaximumNArgs(2)`.
  - `RunE` extended for the new $2 dispatch (cases above) plus the bare-name B2 error.
  - `ValidArgsFunction` stays as `completeRepoNames` for $1; $2 completion punted (see Out of scope).
  - `rootLong` rewritten — see §4.

- `src/cmd/hop/shell_init.go`:
  - Drop `cd|where|` from the known-subcommand list at $1 (lines 46 / shell_init.go:46).
  - In the `*)` (repo-name) branch, add a 2-arg verb dispatch:
    - `$# == 1` → `_hop_dispatch cd "$1"` (existing — bare-name → cd).
    - `$# >= 2` and `$2 == cd` → `_hop_dispatch cd "$1"` (explicit cd verb).
    - `$# >= 2` and `$2 == where` → `command hop "$1" where` (binary handles directly).
    - `$# >= 2` and `$2 == -R` → existing canonical rewrite to `command hop -R "$1" "${@:3}"`.
    - else → existing tool-form sugar `command hop -R "$1" "$2" "${@:3}"`.
  - Update `_hop_dispatch cd`'s helper: it currently runs `command hop where "$2"` (where.go line in shim); change to `command hop "$2" where` since the top-level `where` subcommand is gone.

**Added:**
- `src/cmd/hop/bare_name_test.go` (new file — Tentative #12 in the prior intake, now Confident based on the cleaner separation): tests for the binary's error paths covering bare-name (1 arg), `cd` verb (2 args), tool-form attempts (2+ args, anything other than `where`/`-R`), and the happy path `hop <name> where` printing.

### 4. Help text rewrite (`rootLong` in `root.go`)

```
hop — locate, open, and operate on repos from hop.yaml.

Getting started:
  1. Run `hop config init` to create a starter hop.yaml.
  2. Edit it to list your repos (each entry: name + git URL + parent dir).
  3. Optional: set $HOP_CONFIG in your shell rc to point at a tracked file
     (git-tracked dotfile, Dropbox, etc.) so it follows you across machines.
  4. For interactive use, install the shim: eval "$(hop shell-init zsh)"

Usage:
  hop                       fzf picker, print selection
  hop <name>                cd into the repo (shell function — needs `eval "$(hop shell-init zsh)"`)
  hop <name> cd             same — explicit verb form
  hop <name> where          echo abs path of matching repo
  hop <name> -R <cmd>...    run <cmd>... with cwd = <name>'s repo dir
  hop <name> <tool>...      shim-only sugar for `hop <name> -R <tool> ...` (e.g., `hop dotfiles cursor`)
  hop clone <name>          git clone the repo if it isn't already on disk
  hop clone <url>           ad-hoc clone: clone the URL, register it in hop.yaml, print landed path
  hop clone --all           clone every repo from hop.yaml that isn't already on disk
  hop clone                 fzf picker, then clone if missing
  hop ls                    list all repos
  hop shell-init <shell>    emit shell integration (zsh or bash). Use: eval "$(hop shell-init zsh)"
  hop config init           bootstrap a starter hop.yaml
  hop config where          print the resolved hop.yaml path
  hop config scan <dir>     scan a directory for git repos and populate hop.yaml
  hop update                self-update the hop binary via Homebrew
  hop -h | --help           show this help
  hop -v | --version        print version

Notes:
  - `hop <name>` and `hop <name> cd` require shell integration (a binary can't change
    its parent shell's cwd). Without it, use:  cd "$(hop <name> where)"
  - On ambiguous or no-match queries, fzf opens prefilled with your query.
  - Grammar: first positional is a repo OR a subcommand (mutually exclusive). When it's
    a repo, second positional is a verb (`cd`, `where`), `-R`, or a tool name.
  - Config search order: $HOP_CONFIG, then $XDG_CONFIG_HOME/hop/hop.yaml, then $HOME/.config/hop/hop.yaml.
```

### 5. Spec rewrite

**`docs/specs/cli-surface.md`:**

- **Subcommand Inventory table** (line 8-26):
  - Drop the `hop where <name>` row (line 12) and `hop cd <name>` row (line 15).
  - Update the `hop <name>` row (line 11): "Binary form: print hint to stderr, exit 2. Shell-function form: cd into resolved path." (mirrors the deleted `hop cd` row).
  - Add new rows for `hop <name> cd` and `hop <name> where`.
  - The existing `hop <name> -R <cmd>...` and `hop <name> <tool> [args...]` rows stay.

- **Match Resolution Algorithm** (line 31):
  - Caller list update: was `hop, hop <name>, hop where, hop -R, hop cd, hop clone`. New: `hop, hop <name>, hop <name> where, hop <name> cd, hop -R, hop clone`.

- **GIVEN/WHEN/THEN scenarios**:
  - "Unique substring match" (line 64) and "Ambiguous substring match" (line 73): re-scope as `hop <name> where` scenarios (the path-printing form). Add binary-form `hop <name>` scenarios asserting exit 2 + stderr hint.
  - "`hop cd` binary form" / "`hop cd` shell-function form" (lines 95-108): rewrite as `hop <name> cd` scenarios (binary error + shim cd). Drop the `hop cd` framing.
  - "Bare-name dispatch (shell shim)" (line 110): keep, but tighten — the shim now routes both `hop <name>` (1 arg) and `hop <name> cd` (2 args) through `_hop_dispatch cd`. Note both forms.
  - Existing `hop <name> -R <cmd>...` and `hop <name> <tool>` scenarios (line 120+): unchanged — they already use the new grammar.

- **Design Decisions**:
  - **#1** (line 431, "`hop cd` is intentionally split"): generalize. New phrasing: "The `cd` verb at $2 is shell-only. The binary errors with a hint pointing at the shim install and `hop <name> where`. Same for the bare `hop <name>` (B2 — shorthand for `hop <name> cd`). Generalizes to: every form that needs the shim errors in the binary; every form that the binary can fulfill works in both layers."
  - **#2** (line 432, "Bare-name dispatch lives only in the shim"): rewrite. New phrasing: "Bare-name dispatch (`hop <name>` 1 arg) is shorthand for `hop <name> cd`. Both are shell-only — the binary errors with a hint. This enforces the invariant that any `hop <subform>` either errors in the binary or works in both — never two different effects sharing one syntax."
  - **#6** (line 437, "`hop where` and `hop config where` use the same verb"): rewrite. New phrasing: "The `where` verb is the explicit path-printer. Used as `hop <name> where` (top-level) and `hop config where` (config namespace). The top-level `where` subcommand was removed in v0.x — `hop <name> where` is the replacement; the verb survives, the subcommand position does not."
  - **#10** (line 440, subcommand-xor-repo grammar): reaffirm and extend. New phrasing: "Grammar is `subcommand` xor `repo` at $1. When $1 is a repo, $2 is a verb (`cd`, `where`), `-R`, or a tool name. The verbs `cd` and `where` are NOT subcommands at $1 — they exist only at $2 in the repo-first form. This keeps the shim's 4-step ladder simple (no need to special-case verb-vs-subcommand)."
  - **#11** (line 441, `hop open` removed): unchanged.
  - **#12** (line 442, `hop code` removed via tool-form): unchanged. Reinforced by the broader pattern.
  - **New Design Decision** (e.g., #13): "Tool-form is shim-only; the binary errors on `hop <name> <tool>`. The binary could absorb tool-form (`hop -R <name> <tool>` is its internal shape), but doing so blurs the binary's role as a path-printer + error-emitter. Keeping tool-form shim-only preserves the binary's narrow contract and matches the existing posture for `cd` and bare-name."

- **Help Text section** (line 389): replace with the new `rootLong` from §4.

**`docs/specs/architecture.md`:**

Pre-flight grep surfaced three references that must update:
- **Line 20-21** (source-tree diagram): drop the `cd.go` row; rename `where.go` row to `resolve.go` and update its description from `hop where, bare hop <name>, shared resolver helpers` to `bare hop <name>, hop <name> where, hop <name> cd dispatch, shared resolver helpers`.
- **Line 110** (`where.go` row in the file responsibilities table): same rename + description update; remove the `func newWhereCmd()` reference; the helpers list (`loadRepos`, `resolveByName`, `resolveOne`, `resolveAndPrint`, `buildPickerLines`) stays.
- **Line 214** (`hop where <name>` example in the discussion of resolver semantics): rewrite to `hop <name> where` and update the script example `cd "$(hop where outbox)"` → `cd "$(hop outbox where)"`.

**`docs/specs/config-resolution.md`:**

- **Line 251** (passing reference to `hop where` for voice-fit explanation of `hop config where`): unchanged — the voice-fit argument is now about the `where` verb generally, which still exists at $2 and under `config`. Reading verifies no edit needed; if the wording reads awkwardly post-change, light copy edit only.

### 6. README sweep

Audit `README.md` for any of these patterns and update:
- `hop where <name>` → `hop <name> where`.
- `hop cd <name>` → `hop <name>` or `hop <name> cd` (prefer the shorter bare form for primary examples).
- `cd "$(hop where <name>)"` → `cd "$(hop <name> where)"`.
- `cd "$(hop <name>)"` (if any exist) → `cd "$(hop <name> where)"`.

Verify count via grep in apply.

### 7. Tests

- Delete `cd_test.go` and the cobra-surface portions of `where_test.go` (the latter renamed to `resolve_test.go`; helper tests preserved).
- New `bare_name_test.go` covering:
  - `hop foo` (binary, 1 arg) → exit 2, stderr matches bare-name hint, no stdout.
  - `hop foo cd` (binary, 2 args) → exit 2, stderr matches cd hint (updated to point at `hop "<name>" where`), no stdout.
  - `hop foo where` (binary, 2 args) → exit 0, stdout is resolved path.
  - `hop foo somerandomtool` (binary, 2 args) → exit 2, stderr matches tool-form hint, no stdout.
  - `hop foo somerandomtool a b c` (binary, 4 args) → same as above (any 2+ args with non-`where`/non-`-R` verb).
- Update any existing tests in `integration_test.go` or elsewhere that invoke `hop where <name>` or `hop cd <name>` directly (grep pre-flight).

## Affected Memory

- `cli/subcommands`: (modify) — major rewrite. Drop the `cd` and `where` subcommand sections. Add `<name> cd` and `<name> where` as repo-verb forms. Update grammar overview.
- `cli/match-resolution`: (modify) — caller list updated; the algorithm itself is unchanged.
- `architecture/wrapper-boundaries`: (verify) — likely no change, but pre-flight grep to confirm `where`/`cd` aren't referenced in a way that breaks.

Memory updates happen in hydrate, not apply.

## Impact

**Code (apply scope):**
- `src/cmd/hop/cd.go` — deleted.
- `src/cmd/hop/cd_test.go` — deleted.
- `src/cmd/hop/where.go` → `src/cmd/hop/resolve.go` — `git mv` + drop `newWhereCmd`.
- `src/cmd/hop/where_test.go` → `src/cmd/hop/resolve_test.go` — `git mv` + drop cobra-surface tests.
- `src/cmd/hop/root.go` — `Args` bump, `RunE` rewrite, `rootLong` rewrite, `AddCommand` list trimmed.
- `src/cmd/hop/shell_init.go` — drop `cd`/`where` from known-subcommand list, add $2 verb dispatch, update `_hop_dispatch cd` helper to call `hop "$2" where`. Drop the no-$2 fallback branch (`command hop cd` at line 75) since `hop cd` is no longer a subcommand and the new shim never calls `_hop_dispatch cd` without a $2 — the only callers are bare-name (passes $1) and explicit `cd` verb (passes $1). Verify in apply that no path triggers the dropped branch.
- `src/cmd/hop/bare_name_test.go` — new.
- Existing tests touching `hop where <name>` / `hop cd <name>` — pre-flight grep concrete count: `integration_test.go` (6 sites: `hop where alpha`, `hop where probe`, comment about bare-name resolution via `command hop where`), `shell_init_test.go` (2 sites: literal substring assertions for `command hop where "$2"` → update to `command hop "$2" where`), `cd_test.go` (deleted), `where_test.go` (renamed to `resolve_test.go`; cobra-surface tests dropped, helper tests preserved). `main.go` and `where.go` have comment-only references; updated alongside.
- `docs/specs/cli-surface.md` — inventory, algorithm caller list, scenarios, design decisions, help text section.
- `README.md` — sweep + update.

**Code (hydrate scope):**
- `docs/memory/cli/subcommands.md` — major rewrite.
- `docs/memory/cli/match-resolution.md` — caller list.
- `docs/memory/architecture/wrapper-boundaries.md` — verify only.

**Out of scope:**
- **Tab completion at $2**: not added in this change. Today, $2 has no completion (the previous grammar didn't need it). Adding `cd`/`where` completion at $2 plus PATH-completing tools would be its own change; punt to a follow-up. Existing $1 completion (subcommand names + repo names, minus the dropped `cd`/`where`) is unaffected.
- **Binary absorbing tool-form** (Option Y from discussion): not done. Binary errors on `hop <name> <tool>`; tool-form remains shim-only. Justification in new Design Decision #13 above.
- **`hop` (0 args, picker)**: unchanged. Bare picker still prints in both binary and shim.
- **`hop clone`** (any form): unchanged. The `hop clone <url>` print contract (IPC with shim) is preserved per prior discussion.
- **`hop -R <name> <cmd>...`**: unchanged. `extractDashR` pre-cobra inspection in `main.go` unaffected.
- **Memory file moves/restructuring**: only content updates; no new domains.

**Backwards compat:** **None — clean v0.x break.** All four of these break:
1. `hop cd <name>` → cobra rejects "unknown command". Users learn from `hop -h` or muscle-memory shifts to `hop <name>` or `hop <name> cd`.
2. `hop where <name>` → cobra rejects "unknown command". Users shift to `hop <name> where`.
3. `cd "$(hop where <name>)"` → script breaks. Replacement: `cd "$(hop <name> where)"`.
4. `cd "$(hop <name>)"` (binary form) → already broken by Option A; same v0.x break.

User explicitly waived compat. No aliases, no deprecation period, no migration shim.

**Constitution alignment:**
- Principle I (Security First): no new subprocess paths.
- Principle VI (Minimal Surface Area): **two top-level subcommands removed**. This is the principle's strongest application yet — `cd` and `where` were always derivable from a repo + verb shape; collapsing them aligns with "could this be a flag on an existing subcommand, or a separate tool? must be answered no before adding one."
- Test Integrity: tests update to match new spec; spec is the source of truth.

## Open Questions

None — all decisions converged in discussion before this intake update.

## Clarifications

### Session 2026-05-07 (bulk confirm)

| # | Action | Detail |
|---|--------|--------|
| 11 | Confirmed | — |
| 12 | Confirmed | — |
| 13 | Confirmed | — |
| 14 | Confirmed | — |
| 15 | Confirmed | — |
| 16 | Confirmed | — |
| 17 | Confirmed | — |
| 18 | Confirmed | — |
| 19 | Confirmed | — |
| 20 | Confirmed | — |
| 21 | Confirmed | — |
| 22 | Confirmed | — |
| 23 | Confirmed | — |
| 24 | Confirmed | — |
| 25 | Confirmed | — |
| 26 | Confirmed | — |

User responded "all ok" — bulk-confirmed all 16 Confident assumptions.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Drop `hop cd <name>` and `hop where <name>` as top-level subcommands; replace with `hop <name> cd` and `hop <name> where` | Discussed — user picked full unification | S:95 R:50 A:90 D:95 |
| 2 | Certain | Bare `hop <name>` (1 arg) is shorthand for `hop <name> cd` per option B2: cd in shim, error in binary | Discussed — user explicitly chose B2 over B1 (no shorthand) and B3 (rollback Option A) | S:95 R:60 A:90 D:95 |
| 3 | Certain | Binary errors on `hop <name>` (1 arg) and `hop <name> cd` (2 args) with hint pointing at shim install + `hop <name> where` | Mirrors existing `hop cd` pattern; user agreed | S:95 R:80 A:95 D:95 |
| 4 | Certain | Exit code 2 for all binary error paths (bare-name, `cd` verb, tool-form attempts) | Matches `errExitCode{code: 2}` already used by `hop cd`; consistent with existing `translateExit` taxonomy | S:95 R:90 A:95 D:95 |
| 5 | Certain | No backwards compat — clean v0.x break for `hop cd <name>`, `hop where <name>`, `cd "$(hop where <name>)"`, `cd "$(hop <name>)"` | User explicitly stated "No need of backward compatibility" | S:100 R:50 A:95 D:100 |
| 6 | Certain | `hop config where` survives — different namespace, no collision | Self-evident from grammar; user noted this in discussion | S:95 R:95 A:95 D:95 |
| 7 | Certain | `hop` (0 args, picker) unchanged in both binary and shim | Discussed — user explicitly scoped this out | S:95 R:95 A:95 D:95 |
| 8 | Certain | `hop clone` (all forms) unchanged | Discussed — print is IPC contract with shim, not a divergence | S:95 R:90 A:90 D:95 |
| 9 | Certain | `hop -R <name> <cmd>...` and `extractDashR` unchanged | Outside the unification scope; tool-form sugar already routes through `-R` | S:95 R:95 A:95 D:95 |
| 10 | Certain | Change type: refactor | User direction; description has "refactor", "restructure", "consolidate"; matches inference rule | S:95 R:95 A:95 D:95 |
| 11 | Certain | Binary's $2 dispatch: only `where` works; `cd` errors with shell-only hint; anything else errors with tool-form-not-a-hop-verb hint (Option X, not Y) | Clarified — user confirmed | S:95 R:75 A:85 D:80 |
| 12 | Certain | `where.go` → `resolve.go` rename via `git mv` (preserve history); drop `newWhereCmd` factory in place; helpers stay | Clarified — user confirmed | S:95 R:90 A:95 D:85 |
| 13 | Certain | `cd.go` and `cd_test.go` deleted outright (no rename target) | Clarified — user confirmed | S:95 R:95 A:95 D:90 |
| 14 | Certain | New file `bare_name_test.go` for the new error paths and 2-arg happy path | Clarified — user confirmed | S:95 R:90 A:90 D:85 |
| 15 | Certain | Shim's `_hop_dispatch cd` helper updated: `command hop where "$2"` → `command hop "$2" where` | Clarified — user confirmed | S:95 R:90 A:95 D:95 |
| 16 | Certain | `cd` and `where` removed from the shim's known-subcommand list at $1 | Clarified — user confirmed | S:95 R:90 A:95 D:95 |
| 17 | Certain | Tab completion at $2 punted to a follow-up | Clarified — user confirmed | S:95 R:90 A:85 D:80 |
| 18 | Certain | Updated `cd` hint text changes from `cd "$(hop where "<name>")"` → `cd "$(hop "<name>" where)"` | Clarified — user confirmed | S:95 R:90 A:95 D:95 |
| 19 | Certain | New Design Decision #13 in spec: "Tool-form is shim-only; binary errors on `hop <name> <tool>`" | Clarified — user confirmed | S:95 R:80 A:85 D:80 |
| 20 | Certain | Memory updates deferred to hydrate per fab convention | Clarified — user confirmed | S:95 R:95 A:95 D:90 |
| 21 | Certain | README sweep targets `hop where <name>`, `hop cd <name>`, `cd "$(hop where <name>)"`, `cd "$(hop <name>)"` patterns | Clarified — user confirmed | S:95 R:90 A:90 D:85 |
| 22 | Certain | Tool-form error message wording: `hop: '<tool>' is not a hop verb (cd, where). For tool-form, install the shim: eval "$(hop shell-init zsh)", or use: hop -R "<name>" <tool> [args...]` | Clarified — user confirmed | S:95 R:80 A:85 D:85 |
| 23 | Certain | Concrete call-site count from pre-flight grep: 17 in-code touches across 8 files (`root.go` 4, `cd.go` 1 [deleted], `cd_test.go` 1 [deleted], `where.go` 1, `shell_init.go` 2, `shell_init_test.go` 2, `main.go` 2, `integration_test.go` 6), plus `architecture.md` (3 sites), `cli-surface.md` (~15 sites enumerated), `config-resolution.md` (1, no edit), README.md (1), 3 memory files (hydrate). All mechanical. | Clarified — user confirmed | S:95 R:90 A:95 D:95 |
| 24 | Certain | `docs/specs/architecture.md` updates added to spec-update scope: line 20-21 source tree (drop `cd.go`, rename `where.go` → `resolve.go`), line 110 file responsibilities row (same), line 214 example `cd "$(hop where outbox)"` → `cd "$(hop outbox where)"` | Clarified — user confirmed | S:95 R:90 A:95 D:90 |
| 25 | Certain | Shim's `_hop_dispatch cd` helper: drop the no-$2 fallback branch (`command hop cd` at shell_init.go:75) | Clarified — user confirmed | S:95 R:85 A:90 D:85 |
| 26 | Certain | `docs/specs/config-resolution.md` line 251 (passing reference to `hop where` for voice-fit) — no edit required | Clarified — user confirmed | S:95 R:95 A:90 D:80 |

26 assumptions (26 certain, 0 confident, 0 tentative, 0 unresolved).
