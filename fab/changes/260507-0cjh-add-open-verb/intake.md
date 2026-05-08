# Intake: Add `hop <name> open` verb

**Change**: 260507-0cjh-add-open-verb
**Created**: 2026-05-07
**Status**: Draft

## Origin

> Modfify "hop <repo-name> open" to work like wt's open command that is context aware. You can check wt at "hop wt where"

Conversational `/fab-discuss` session preceded this `/fab-new`. The user landed on a clear design through several rounds of clarifying questions:

- **Reuse strategy**: hop shells out to `wt open` rather than reimplementing app detection or extracting a shared module. wt at `/home/sahil/code/sahil87/wt` already has a 330-line `apps.go` covering VSCode, Cursor, Ghostty, iTerm2, Terminal.app, GNOME Terminal, Konsole, Finder, Nautilus, Dolphin, clipboard copy, byobu/tmux tabs and sessions, plus default-app detection. Constitution Principle IV ("Wrap, Don't Reinvent") points at this answer.
- **Scope**: Only the explicit form `hop <name> open`. No no-arg variants (no `hop open` from cwd, no fzf picker). Keeps the surface area minimal — Principle VI.
- **Cwd handling**: Binary chdirs into the resolved repo path before exec'ing wt, so wt's `ValidateGitRepo()` cwd-check passes naturally without modifying wt.
- **"Open here" cd round-trip**: When the user picks "Open here" from wt's menu, the parent shell must `cd` into the repo. Mechanism: hop sets `WT_CD_FILE` to a temp path before exec'ing wt; after wt exits, hop reads the temp file. If non-empty, hop re-emits the path on stdout. Hop's existing shell shim wraps `open` like it wraps `cd`/`clone` — captures stdout, `cd`s if non-empty, otherwise does nothing. For all other apps (VSCode, terminals, file managers), no stdout output → no cd → terminal cwd untouched.
- **wt's hints leak through**: When wt prints `hint: "Open here" requires the shell wrapper... eval "$(wt shell-setup)"` to stderr, that hint reaches the user as-is. Acceptable — users adopting hop's `open` verb who want "Open here" install both shims. (Hop's own shim handles the cd; wt's hint is a no-op for the actual cd path because hop intercepts via `WT_CD_FILE`.)
- **Verb collision resolution**: Today `hop <name> open` works on Darwin via tool-form sugar — the shim rewrites to `command hop -R <name> open` and execs `/usr/bin/open` (Finder) inside the repo. Adding `open` as a recognized $2 verb intercepts this and replaces it with the wt menu. The Darwin one-liner for "open repo dir in Finder" is replaced by selecting "Finder" from wt's menu. Documented as a behavior change.
- **Prior removal**: `cli/subcommands.md:40` notes the `open` subcommand was removed when `internal/platform` was deleted. New `open` differs in scope — hop does not own platform abstraction; it delegates to wt. The memory note will be replaced at hydrate time.
- **wt as Homebrew dependency, not runtime check**: wt is declared as a Homebrew formula dependency (`depends_on "sahil87/tap/wt"` in `Formula/hop.rb`), so `brew install sahil87/tap/hop` pulls wt automatically. No runtime `LookPath("wt")` check, no missing-wt install hint, no External tool failure messages row. If `wt` is somehow absent (manual removal, non-brew install path), `proc.RunForeground` returns its natural `proc.ErrNotFound` — terse, consistent with how `-R` handles missing tools.

## Why

**Problem.** Today, opening a hop-managed repo in an editor or terminal is awkward:
- `hop <name> code` (tool-form) execs `code` inside the repo dir — works but only for one app, requires the user to remember the binary name (`code` vs `cursor` vs `ghostty`).
- `cd "$(hop <name> where)"` puts you in the repo's directory but doesn't open anything. You then run another command.
- There is no "menu of available apps" UX. The user has to know exactly what they want.

Meanwhile, `wt` (the worktree CLI in the same author's toolchain) has solved this exact problem for worktrees: a context-aware `wt open` that detects available apps (editors, terminals, file managers, multiplexer tabs, clipboard copy), shows a menu with a smart default based on `TERM_PROGRAM` and last-used cache, and supports an "Open here" choice that `cd`s the parent shell.

**Why this approach over alternatives.**
- *Reimplement app detection in hop.* Rejected. Duplicates ~330 lines of wt code; maintenance burden across two repos. Violates Principle IV.
- *Extract a shared `openin` Go module imported by both wt and hop.* Cleaner long-term, but cross-repo coordination cost is high for a feature that has one consumer in each repo. Worth revisiting if a third caller appears.
- *Shell out to `wt open`.* Chosen. Zero duplication. wt already handles every concern (detection, menu, default selection, cache, "Open here" via `WT_CD_FILE`). Only cost: hop gains a runtime dependency on `wt`. Acceptable because (a) hop already depends on `git`, `fzf`, and (in the binary's tool-form path) arbitrary user binaries; (b) wt is from the same author's toolchain; (c) the failure mode is a clean hint, not a crash.

**Consequence of not doing this.** Users who want a menu-driven open UX use `wt` for worktrees and ad-hoc `cd $(hop <name> where) && code .` invocations for hop repos. Two separate workflows for the same conceptual operation ("open this repo in some app"). Users who want "open repo in Finder" on Darwin currently rely on the tool-form `hop <name> open` — that path stays one-liner-friendly (pick "Finder" from the menu) but adds one keystroke.

**Justification for new verb (Principle VI — Minimal Surface Area).** Could this be a flag on an existing verb? No — `open` produces an interactive menu UX and a conditional `cd` round-trip. Neither fits `where` (path resolver, no UI, no shell mutation) nor `cd` (always cds, no menu, no app launching) nor `-R` (specifies the tool by name, no menu, no app detection). The verb is the right grammar slot.

## What Changes

### New `open` verb at args[1]

Recognized as a 2-arg form alongside `where` and `cd` in `src/cmd/hop/root.go::newRootCmd::RunE`:

```go
case 2:
    switch args[1] {
    case "where":
        return resolveAndPrint(cmd, args[0])
    case "cd":
        return &errExitCode{code: 2, msg: cdHint}
    case "open":  // NEW
        return runOpen(cmd, args[0])
    default:
        return &errExitCode{code: 2, msg: fmt.Sprintf(toolFormHintFmt, args[1])}
    }
```

The `runOpen` function (likely in a new `src/cmd/hop/open.go`):

1. Resolve `args[0]` to a path via `resolveByName` (same pattern as `where`-verb).
2. Create a temp file in `os.TempDir()` for `WT_CD_FILE`. Use `os.CreateTemp` with prefix `hop-open-cd-`.
3. Build env: copy parent env, set `WT_CD_FILE=<temp-path>`, set `WT_WRAPPER=1` (suppresses wt's "install the shell wrapper" hint since hop is acting as the wrapper).
4. Invoke `wt open` with `cmd.Dir = <resolved-path>`, env from step 3, stdin/stdout/stderr inherited from parent. Use `proc.RunForeground` (Constitution Principle I — no `os/exec` in `cmd/`). Hop runs from anywhere; chdir to the resolved repo path satisfies wt's `ValidateGitRepo()`. If `proc.RunForeground` returns `proc.ErrNotFound`, hop exits with the standard "binary not found" path (the brew-formula contract makes this rare; no special hint).
5. After wt exits (regardless of exit code), read the temp file. If contents are non-empty, write them to `cmd.OutOrStdout()` (this is the "Open here" → cd path; hop's shim picks up stdout and cds the parent shell). If empty, write nothing.
6. Clean up the temp file via `defer os.Remove`.
7. Propagate wt's exit code via `errExitCode{code: <wt-exit>}` if non-zero, so the user sees wt's error path correctly. Exit 0 on wt success.

### Shell shim — wrap `open` like `cd`/`clone`

In `src/cmd/hop/shell_init.go::posixInit`, the otherwise-branch's $2 dispatch grows a new arm:

```sh
elif [[ "$2" == "open" ]]; then
  _hop_dispatch open "$1"
```

placed alongside the existing `where`/`cd`/`-R` arms in rule 5. And in `_hop_dispatch`:

```sh
open)
  local target
  target="$(command hop "$2" open)" || return $?
  if [[ -n "$target" ]]; then
    cd -- "$target"
  fi
  ;;
```

Note: callers always pass `$1` (repo name) as the dispatch's `$2`, mirroring the `cd` arm convention.

The bare-name (1-arg) branch is **not** changed — `hop <name>` still defaults to `cd`, not `open`. Only the explicit `hop <name> open` form invokes the new verb.

### Binary-direct invocation

`hop <name> open` works without the shim too — the binary handles the verb itself. But "Open here" cannot mutate the parent shell's cwd from a child process. When invoked binary-direct (no shim) and the user picks "Open here", hop prints the path to stdout and emits a hint to stderr suggesting the user wire the shim or run `eval "$(wt shell-setup)"`. The hint format mirrors hop's existing patterns:

```
hop: 'Open here' requires the shell shim to cd. Add 'eval "$(hop shell-init zsh)"' to your zshrc, or use: cd "$(hop "<name>" open)"
```

Detection: hop checks for an env var the shim sets (e.g., `HOP_WRAPPER=1`, added to `posixInit` alongside the function definitions). When the binary sees `HOP_WRAPPER` unset, it knows it's running outside the shim and emits the hint.

### Help text update (`rootLong` in `root.go`)

Add a row to the Usage table:

```
  hop <name> open           open the repo in an app (delegates to wt's menu)
```

Update the Notes section to mention `open` alongside `cd` as shell-integration-dependent for the "Open here" cd path.

### Memory updates

- **`docs/memory/cli/subcommands.md`** (modify):
  - Inventory table: add a row for `hop <name> open` between the `cd`-verb row and the tool-form row.
  - "Removed subcommands" section: update or remove the line about the `open` subcommand removal — replaced by the new verb.
  - Tool-form dispatch table: update or remove the rows for `hop outbox open` (Darwin) and `hop outbox xdg-open` (Linux) since those examples are now superseded; on Darwin the new verb intercepts before tool-form rewrite.
  - "Binary-form hint texts" section: add the new "Open here without shim" hint constant.
  - `hop shell-init` section: update rule 5 of the resolution ladder to list `$2 == "open"` alongside `where`/`cd`/`-R`. Update `_hop_dispatch` description to mention the new `open)` arm.
- **`docs/memory/architecture/wrapper-boundaries.md`** (modify):
  - "What is NOT wrapped" table or add a new sub-section: document that `wt` is shelled out to via `proc.RunForeground` from `cmd/hop/open.go`, with `WT_CD_FILE` and `WT_WRAPPER` env contract spelled out. No `internal/wt` wrapper package — single operation, premature to abstract.
- **`docs/memory/build/release-pipeline.md`** (modify):
  - "Formula template" section: document the new `depends_on "sahil87/tap/wt"` line in the template (and the corresponding line added to the published `Formula/hop.rb`). `Formula/wt.rb` already exists in `sahil87/homebrew-tap`, so no separate tap-side work is needed.
- **`docs/memory/cli/match-resolution.md`** (no change — `open` reuses `resolveByName`).

### Spec updates

- **`docs/specs/cli-surface.md`**: list `open` as a $2 verb with its arguments, exit codes, stdout/stderr conventions, and the shim cd round-trip.

### No changes to

- `clone.go`, `ls.go`, `config*.go`, `update.go`, `repo_completion.go`, `shell_init_test.go`'s test fixtures for unrelated subcommands.
- `internal/proc`, `internal/fzf`, `internal/yamled`, `internal/scan`, `internal/update` — no API changes.
- `internal/repos`, `internal/config` — no schema changes.

## Affected Memory

- `cli/subcommands`: (modify) add `hop <name> open` to the inventory; remove or update the prior-removal note; update tool-form dispatch table to reflect that Darwin's `hop outbox open` no longer reaches the tool-form path; document the new `wt`-missing hint; update the shim's resolution ladder description.
- `architecture/wrapper-boundaries`: (modify) document the `wt` shell-out as a non-packaged external invocation; record the `WT_CD_FILE`/`WT_WRAPPER` env contract.

## Impact

**Code**:
- `src/cmd/hop/root.go` — add `case "open"` arm in 2-arg switch.
- `src/cmd/hop/open.go` — new file, ~40 lines: `runOpen` function plus the new "no shim" hint constant. (No missing-wt hint — wt is a brew formula dep.)
- `src/cmd/hop/shell_init.go` — extend `posixInit` rule-5 dispatch with `$2 == "open"` arm and add `open)` arm in `_hop_dispatch`. Set `HOP_WRAPPER=1` in the function so the binary can detect shim presence.
- `src/cmd/hop/open_test.go` — new file: integration test for the temp-file cd round-trip (fake `wt` shell script in a temp dir prepended to `PATH`, writes a path to `WT_CD_FILE`; assert hop re-emits on stdout) and the binary-direct hint path (no shim → "Open here" prints path + hint).
- `src/cmd/hop/shell_init_test.go` — extend golden files to include the new dispatch arms and `HOP_WRAPPER=1` export line.
- `src/cmd/hop/root.go` — update `rootLong` Usage table and Notes section.
- `src/cmd/hop/repo_completion.go` — add `"open"` to the verb-position completion list alongside `"where"` and `"cd"`.

**Build / release**:
- `.github/formula-template.rb` — already edited on this branch; adds `depends_on "sahil87/tap/wt"` after `license "MIT"`. Live tap (`Formula/hop.rb` in `sahil87/homebrew-tap`) is **not** touched in this change — the next tagged hop release rewrites `Formula/hop.rb` from the template via the existing release workflow's `sed` substitution, and that release is when users on `brew upgrade hop` first see the new dep.

**External dependencies**:
- New runtime dep: `wt` on PATH, declared via Homebrew formula `depends_on "sahil87/tap/wt"`. No runtime LookPath check; `proc.RunForeground` returns its natural `proc.ErrNotFound` if wt is somehow absent.

**Cross-platform**:
- Darwin: behavior change — `hop <name> open` no longer execs Finder via tool-form; it shows wt's menu (Finder is one option among many).
- Linux: clean addition — `hop <name> open` was a tool-form attempt against `/usr/bin/open` which doesn't exist on most Linux installs. The new verb gives Linux users a working menu UX for the first time.

**Tests**: existing tool-form tests for `hop <name> open` (if any) need updating to reflect the new verb-intercept path. Search for `"open"` in `*_test.go` to enumerate.

**No DB, no migrations** (Principle II).

## Open Questions

- Should hop check wt's version compatibility (e.g., minimum wt version that exposes `WT_CD_FILE`)? Or trust whatever wt is installed and let hop fail loudly if the contract is broken?
- Should the shim's `HOP_WRAPPER=1` env var be exported to children of the shim, or scoped only to the function call? (Affects whether subprocess-spawned hop calls also see it.)
- Should hop pass `--app default` to `wt open` to skip wt's menu entirely (using wt's last-app cache)? Trade-off: faster default path vs. losing the menu for cold-start users. Default behavior should be the menu (matches wt's own default); a future `--app` flag on `hop <name> open` could be added if demand emerges.
- Tab-completion: should `open` be added to the verb completions alongside `cd`/`where`? (Likely yes — tracked as a small follow-up in the spec.)

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Shell out to `wt open` instead of reimplementing app detection | Discussed — user chose this over module-extraction and code-duplication | S:95 R:80 A:90 D:90 |
| 2 | Certain | Scope limited to `hop <name> open` (no no-arg variants) | Discussed — user explicitly excluded no-arg forms in clarification | S:95 R:80 A:90 D:95 |
| 3 | Certain | "Open here" cd uses stdout convention (binary prints path; shim cds if non-empty) | Discussed — user chose Option 1 (stdout) over Option 2 (HOP_CD_FILE temp file). Binary internally uses WT_CD_FILE temp file to read wt's choice, then re-emits on stdout. | S:95 R:75 A:90 D:90 |
| 4 | Certain | Binary chdirs into resolved repo path before exec'ing wt | Discussed — solves wt's `ValidateGitRepo()` cwd-check without wt-side changes | S:95 R:90 A:95 D:95 |
| 5 | Certain | wt is a hard runtime dependency; missing-wt produces clear install hint | Discussed — user confirmed | S:95 R:85 A:95 D:95 |
| 6 | Certain | Replace existing Darwin tool-form behavior (`hop <name> open` → `/usr/bin/open`) with the new verb | Discussed — user explicitly chose "verb wins" over alternative names like `launch` | S:95 R:60 A:90 D:85 |
| 7 | Certain | wt's stderr hints (e.g., `wt shell-setup` install line) leak through to users | Discussed — user accepted this as fine | S:95 R:90 A:95 D:90 |
| 8 | Certain | Replace memory note about prior `open` subcommand removal with the new behavior | Discussed — user said "Replace the memory with the new functionality" | S:95 R:95 A:95 D:95 |
| 9 | Confident | Use `proc.RunForeground` (not `os/exec` directly) for the `wt` invocation | Constitution Principle I requires all subprocess execs go through `internal/proc`. `RunForeground` matches the pattern used by `-R` (cmd.Dir + inherited stdio). | S:90 R:85 A:95 D:90 |
| 10 | Confident | New file `src/cmd/hop/open.go` rather than inlining `runOpen` in `root.go` | Existing pattern: each verb/subcommand has its own file (`clone.go`, `ls.go`, `update.go`, `config.go`, `config_scan.go`, `resolve.go`). Verb-vs-subcommand distinction doesn't matter for file organization. | S:80 R:80 A:90 D:85 |
| 11 | Confident | Set `WT_WRAPPER=1` in env passed to wt to suppress wt's install-shim hint | wt's `apps.go:189` checks this env var to decide whether to print the `wt shell-setup` hint. Hop is acting as the wrapper, so suppression is correct. | S:90 R:90 A:90 D:90 |
| 12 | Confident | Binary detects shim absence via `HOP_WRAPPER=1` env var (set by the shim's `hop()` function) | Mirrors wt's `WT_WRAPPER` pattern. New convention for hop, but trivial to add to `posixInit`. | S:80 R:80 A:90 D:85 |
| 13 | Certain | wt is a Homebrew formula dependency, not a runtime LookPath check. No missing-wt hint. | Clarified — user said "we will make a brew dependency". `Formula/wt.rb` already exists in `sahil87/homebrew-tap`. Adding `depends_on "sahil87/tap/wt"` to `.github/formula-template.rb` makes future hop releases pull wt automatically. If wt is somehow missing at runtime, `proc.RunForeground` returns `proc.ErrNotFound` — same path as any other missing tool under `-R`. | S:95 R:90 A:95 D:95 |
| 14 | Certain | Tab completion for the new `open` verb is added in this change (one line in `repo_completion.go`). | Clarified — user confirmed in-scope. | S:90 R:90 A:90 D:95 |
| 15 | Certain | Test via fake `wt` shell script in temp dir prepended to PATH. | Clarified — user confirmed. Matches the existing pattern for `git`/`brew` tests in this repo. | S:90 R:90 A:90 D:95 |
| 16 | Certain | Live tap (`Formula/hop.rb` in `sahil87/homebrew-tap`) is NOT edited in this change. Only `.github/formula-template.rb` in the hop repo. | Clarified — user chose "Template only". The next tagged hop release rewrites `Formula/hop.rb` from the template via the release workflow's `sed` substitution. Cleaner semantics: dep ships with the binary version that needs it. | S:95 R:90 A:95 D:95 |

16 assumptions (12 certain, 4 confident, 0 tentative, 0 unresolved). Spec gate (3.0 for `feat`) likely clearable on first pass.
