# CLI Subcommands

What each subcommand of the `hop` binary actually does. Source files live in `src/cmd/hop/`.

The grammar is **`subcommand` xor `repo` at $1**. When $1 is a subcommand (`clone`, `pull`, `push`, `sync`, `ls`, `shell-init`, `config`, `update`), normal cobra dispatch applies. When $1 is a repo name, the root command's `RunE` switches on `len(args)` and `args[1]`: 1 arg → bare-name hint (shell-only); 2 args, `args[1] == "where"` → resolve and print; 2 args, `args[1] == "cd"` → cd-verb hint (shell-only); 2 args, `args[1] == "open"` → delegate to `wt open` (binary handles directly); 2 args, anything else → tool-form hint (shim-only). Cobra's `MaximumNArgs(2)` cap rejects 3+ positionals before RunE runs. The verbs `cd`, `where`, and `open` are NOT subcommands at $1 — they exist only at $2 in the repo-first form (the v0.x top-level `hop cd <name>` and `hop where <name>` were removed in the repo-verb grammar flip; `open` was removed as a subcommand and reinstated as a $2 verb that delegates to `wt`).

**Worktree-aware grammar extension** (change `7eab`): the `$1` repo positional accepts an optional `/<wt-name>` suffix — `hop <name>/<wt-name>` — that resolves to the absolute path of the named worktree. The split happens inside `resolveByName` (see [match-resolution](match-resolution.md)) and is invisible to cobra. Every form that already runs through `resolveByName` inherits the suffix for free: bare `hop <name>/<wt>` (shim cd), `hop <name>/<wt> where`, `hop <name>/<wt> open`, `hop <name>/<wt> -R <cmd>...`, and the shim's tool-form `hop <name>/<wt> <tool> [args...]`. The `/` is safe as a separator because `hop.yaml` repo names are URL basenames (single component, no `/`) — see [config/yaml-schema](../config/yaml-schema.md). Worktree resolution shells out to `wt list --json` (a second wrapped wt call alongside `wt open` — see [architecture/wrapper-boundaries](../architecture/wrapper-boundaries.md)).

Match resolution algorithm used by `hop` (bare picker), `hop <name> where`, `hop <name> cd` (via the shim's `_hop_dispatch cd`), `hop clone`, `hop -R` is documented separately in [match-resolution](match-resolution.md). The name-or-group resolver used by `hop pull`, `hop push`, and `hop sync` (which adds an exact group-match step in front of the existing substring repo match) is described in the same file under [Name-or-Group Resolution](match-resolution.md#name-or-group-resolution).

## Inventory

| Subcommand | File | Args | Behavior |
|---|---|---|---|
| `hop` (bare picker) | `root.go` | 0 positional | Resolves via match-or-fzf, prints abs path on stdout |
| `hop <name>` | `root.go` (RunE 1-arg) | exactly 1 positional, not a known subcommand | Binary: prints `bareNameHint` (`hop: bare-name dispatch is shell-only. Add 'eval "$(hop shell-init zsh)"' to your zshrc, or use: hop "<name>" where`) to stderr, exits 2. Shim: routes through `_hop_dispatch cd "$1"` (1-arg shorthand for `hop <name> cd`). The `<name>` positional accepts an optional `/<wt-name>` suffix (`hop <name>/<wt-name>`) that resolves through `wt list --json` — the shim's `cd` path inherits worktree-awareness via `resolveByName`'s `/`-split (see [match-resolution](match-resolution.md)) |
| `hop <name> where` | `root.go` (RunE 2-arg, `args[1] == "where"`) | exactly 2 positionals | Resolves `args[0]` via match-or-fzf and prints abs path on stdout. Replaced v0.x's top-level `hop where <name>` subcommand. With a `/<wt-name>` suffix on `args[0]` (`hop <name>/<wt> where`), prints the worktree's absolute path instead of the main checkout's — same `resolveByName` seam, additional `wt list --json` round-trip (see [match-resolution](match-resolution.md)) |
| `hop <name> cd` | `root.go` (RunE 2-arg, `args[1] == "cd"`) | exactly 2 positionals | Binary: prints `cdHint` (`hop: 'cd' is shell-only. Add 'eval "$(hop shell-init zsh)"' to your zshrc, or use: cd "$(hop "<name>" where)"`) to stderr, exits 2. Shim: `$2 == "cd"` branch routes through `_hop_dispatch cd "$1"` and `cd`s into the resolved repo. Replaced v0.x's top-level `hop cd <name>` subcommand |
| `hop <name> open` | `open.go` (RunE 2-arg, `args[1] == "open"`) | exactly 2 positionals | Resolves `args[0]` to a path, then execs `wt open <path>` via `proc.RunForeground` (no `cmd.Dir`, no env override — fully transparent passthrough with stdio inherited). Passing the path as a positional arg makes wt take its "path-first" branch and show the **app menu**; without the arg, wt would chdir-detect a git repo and show the **worktree-selection menu** instead. Propagates wt's exit code via `errExitCode`; missing wt emits `hop: wt: not found on PATH.` to stderr and exits 1 (`errSilent`). The cd-handoff for "Open here" is owned by the shim, not the binary — see `_hop_dispatch open)` below. Shim: `$2 == "open"` routes through `_hop_dispatch open "$1"`, which creates a temp file, exports `WT_CD_FILE`/`WT_WRAPPER`, invokes the binary, and reads the temp file to cd if non-empty. Replaces the v0.x tool-form `hop <name> open` (Darwin Finder via `/usr/bin/open`) — that path now reaches wt's menu instead, where "Finder" is one of the offered apps. The `<name>` positional accepts the `/<wt-name>` suffix; `resolveByName` returns a `*repos.Repo` whose `Path` is the worktree's, and wt opens its app menu targeting that path |
| `hop <name> <tool> [args...]` | `root.go` (RunE 2-arg default) for the binary; `shell_init.go::posixInit` for the shim | 2 positionals (binary cap), 2+ in the shim | Binary: prints `fmt.Sprintf(toolFormHintFmt, args[1])` (`hop: '<tool>' is not a hop verb (cd, where, open). For tool-form, install the shim: eval "$(hop shell-init zsh)", or use: hop -R "<name>" <tool> [args...]`) to stderr, exits 2. Shim: rewrites to `command hop -R "$1" "$2" "${@:3}"` (tool-form sugar). See [Tool-form dispatch](#tool-form-dispatch) below. The shim-rewritten form `hop -R <name>/<wt> <tool>...` reaches `runDashR`, which calls `resolveByName` and inherits the `/`-suffix worktree resolution — every tool-form invocation gains worktree-awareness without per-verb code |
| `hop clone [<name>]` / `--all` | `clone.go` | 0 or 1, plus `--all` flag | Single resolves via match-or-fzf; `--all` iterates the full list and prints a summary |
| `hop clone <url>` | `clone.go` | 1 (URL form) | Ad-hoc clone with auto-registration. Detects URL via `looksLikeURL`. Supports `--group`, `--no-add`, `--no-cd`, `--name` flags. See [Ad-hoc URL clone](#ad-hoc-url-clone) below |
| `hop pull [<name-or-group>]` / `--all` | `pull.go` | 0 or 1, plus `--all` flag (`cobra.MaximumNArgs(1)`) | Wraps `git pull` over a single repo (substring match on `Name`), every cloned repo in a named group (exact group match), or every cloned repo with `--all`. Routes through `internal/proc.RunCapture` with the same 10-minute `cloneTimeout`. stdout empty; per-repo `pull: <name> ✓ <last-line>` / `pull: <name> ✗ <err>` and `skip: <name> not cloned` go to stderr. Batch mode emits a final `summary: pulled=N skipped=M failed=K`; exit 0 if `failed == 0`, else 1 (`errSilent`). Single-repo `not cloned` exits 1; usage errors (missing positional + missing `--all`, or `--all` combined with positional) exit 2 with a hop-emitted message; fzf cancel exits 130. `git` missing on PATH emits `gitMissingHint` once and aborts the batch. Resolution rules and tiebreaker live in [match-resolution](match-resolution.md#name-or-group-resolution) |
| `hop push [<name-or-group>]` / `--all` | `push.go` | 0 or 1, plus `--all` flag (`cobra.MaximumNArgs(1)`) | Wraps `git push` over a single repo (substring match on `Name`), every cloned repo in a named group (exact group match), or every cloned repo with `--all`. Same signature, flag set, and resolution rules as `hop pull` (delegates to the shared `resolveTargets` resolver). Each invocation routes through `internal/proc.RunCapture` with the 10-minute `cloneTimeout`. stdout empty; per-repo `push: <name> ✓ <last-line>` / `push: <name> ✗ <err>` and `skip: <name> not cloned` go to stderr. Batch mode emits a final `summary: pushed=N skipped=M failed=K`; exit 0 if `failed == 0`, else 1 (`errSilent`). Single-repo `not cloned` exits 1; usage errors (missing positional + missing `--all`, or `--all` combined with positional) exit 2 with a hop-emitted message; fzf cancel exits 130. `git` missing on PATH emits `gitMissingHint` once and aborts the batch. No `--force`, no `--set-upstream` — Constitution III; users wanting them reach for `hop -R <name> git push --force` |
| `hop sync [<name-or-group>]` / `--all` | `sync.go` | 0 or 1, plus `--all` and `-m / --message <msg>` flags (`cobra.MaximumNArgs(1)`) | Wraps an optional auto-commit (when the working tree is dirty) and then `git pull --rebase` then `git push` per target. Same signature and resolution rules as `hop pull` (delegates to the shared `resolveTargets` resolver). Per-repo flow: `git status --porcelain` → if non-empty, `git add --all` then `git commit -m <msg>` (default `chore: sync via hop`, override via `-m`/`--message`) → `git pull --rebase` → `git push`. Hooks are respected (no `--no-verify`). On commit failure (e.g., `pre-commit` rejection), emits `sync: <name> ✗ commit failed: <err>` and skips both rebase and push. On rebase failure with `CONFLICT` substring in git's stdout/stderr, emits `sync: <name> ✗ rebase conflict — resolve manually with: git -C <path> rebase --continue` and skips push; on other rebase failure forwards git's stderr verbatim and skips push; on push failure emits `sync: <name> ✗ push failed: <err>`. Each git invocation gets an independent 10-minute timeout (per-call, not per-batch). No auto-stash, no auto-resolve, no force-push (clean trees still have no auto-stash; dirty trees auto-commit). Batch summary: `summary: synced=N skipped=M failed=K`. Exit codes match `pull`. See [Sync auto-commit](#sync-auto-commit) below |
| `hop ls` | `ls.go` | none (`cobra.NoArgs`); `--trees` bool flag (default false) | Default (no flag): prints aligned `name<spaces>path` rows; empty list prints nothing. With `--trees`: fans `wt list --json` across configured repos in YAML source order, emitting per-row summaries `name<spaces>{N} tree(s)  (<wt-list>)` where each wt is rendered as `name[*][↑N]` (`*` if `Dirty`, `↑N` if `Unpushed > 0`). Non-cloned repos surface `name<spaces>(not cloned)` without invoking wt. Per-row `wt list` failure (corrupt `.git`, malformed JSON) degrades gracefully as `name<spaces>(wt list failed: <err>)` — the table is never aborted. Exception: the FIRST `wt list` invocation hitting `proc.ErrNotFound` fails fast with `hop: wt: not found on PATH.` to stderr and exit 1 (`errSilent`); subsequent invocations within the same run never hit `ErrNotFound` because we abort on the first. Singular `tree` when N == 1, `trees` otherwise. Glyph constants `wtDirtyGlyph = "*"` and `wtUnpushedGlyph = "↑"` live at the top of `ls.go` |
| `hop shell-init <shell>` | `shell_init.go` | exactly 1 | `zsh` → emits `posixInit` prefix + cobra-generated `_hop` zsh completion + `compdef _hop h hi`; `bash` → `posixInit` + cobra-generated `__start_hop` bash completion + `complete -o default -F __start_hop h hi`; missing or other → exit 2 with exact stderr |
| `hop config init` | `config.go` | none | Writes embedded `starter.yaml` to `ResolveWriteTarget()`. Post-write stderr tip points users at `hop config scan <dir>` (and the `$HOP_CONFIG` portability tip) — see [config/init-bootstrap](../config/init-bootstrap.md) |
| `hop config where` | `config.go` | none | Prints `ResolveWriteTarget()` to stdout (never errors on missing file). Renamed from v0.0.1's `config path` for voice-fit consistency with the `where` verb (used as `hop <name> where` at the top level and `hop config where` here). Different namespace from the top-level `where` verb — no collision |
| `hop config print` | `config.go` | none | Reads the resolved `hop.yaml` (via `config.Resolve()` — same reader-contract resolver used by `hop`, `hop ls`, `hop clone`) and writes its raw bytes verbatim to stdout via `os.ReadFile`. No parsing, no normalization, no synthetic header — comments and formatting preserved by construction. Errors from `Resolve()` propagate unchanged (`$HOP_CONFIG points to ... does not exist`, `no hop.yaml found ...`); read errors after a successful resolve are wrapped as `hop config print: read <path>: <err>`. `Args: cobra.NoArgs`, no flags. Wired alongside `init`/`where`/`scan` in `newConfigCmd().AddCommand(...)` |
| `hop config scan <dir>` | `config_scan.go` | exactly 1 dir; `--write` (bool, default false), `--depth N` (int, default 3, must be >= 1) | Walk `<dir>` for git repos (DFS, depth-bounded, symlink-following with (dev,inode) loop dedup), auto-derive groups (convention match → `default`; non-match → invent group from parent-dir basename, slugified), render YAML to stdout (default) or merge into the resolved `hop.yaml` (`--write`, atomic + comment-preserving via `internal/yamled.MergeScan`). Skips worktrees, bare repos, no-remote repos; submodules excluded by the no-descent invariant. Exit codes: 0 success (any repo count, including zero); 1 hop.yaml missing, load error, or write/merge failure; 2 usage error (missing `<dir>`, dir validation failure, `--depth < 1`). See [config/scan](../config/scan.md) |
| `hop update` | `update.go` | none | Self-update via Homebrew. Detects brew install via `os.Executable` + `EvalSymlinks` + `/Cellar/` substring; non-brew installs exit 0 with a manual-update hint. Logic lives in `internal/update`; subprocess calls go through `internal/proc` |
| `hop <name> -R <cmd>...` | `main.go` | repo name + flag + child argv | Resolves `<name>` to a path, then execs `<cmd>...` with `Dir=<path>` and inherited stdio. User-facing form puts the repo first; the shim rewrites it to the binary's internal shape `hop -R <name> <cmd>...` before exec. Implemented via pre-Execute argv inspection in `main.go::extractDashR`, which scans for `-R` and treats the next token as `<name>` and everything after as `<cmd>...`. Bypasses cobra subcommand parsing for the post-`<name>` argv. Spelled `-R` (not `-C`) because hop is repo-scoped, not directory-scoped |
| `hop -v` / `hop --version` | cobra | — | Auto-wired by cobra when `rootCmd.Version` is set; output is the `var version` value (default `dev`, overridden via `-ldflags "-X main.version=..."`) |
| `hop help` / `-h` / `--help` | cobra | — | Cobra-rendered help, with `rootLong` providing the `Usage:` table and `Notes:` block from `root.go` |

## Removed subcommands

The `path` subcommand has been removed (no alias). Use `hop <name> where` or the bare form `hop <name>` (shim only).

The top-level `where` and `cd` subcommands were removed in the v0.x repo-verb grammar flip (no aliases). Use `hop <name> where` and `hop <name> cd` (or the bare `hop <name>` shorthand for the shim's `cd`). The verb position migrated from $1 (subcommand) to $2 (verb following a repo). `hop config where` survives unchanged (different namespace).

The `config path` subcommand has been removed (no alias). Use `hop config where`.

The `code` subcommand has been removed (no alias). Use the shim's tool-form: `hop <name> code` (if `code` is on PATH) — the shim rewrites this to `command hop -R <name> code`. Or invoke the binary directly: `hop -R <name> code`.

The `open` subcommand was removed in v0.x (along with the `internal/platform` cross-platform abstraction) and later **reinstated as a $2 verb** in `hop <name> open`. The new verb does not own platform abstraction itself — it shells out to `wt open`, which handles app detection, menu selection, and launching for editors, terminals, file managers, multiplexer tabs, and the "Open here" cd path. See the inventory row above. The Darwin one-liner for "open repo dir in Finder" is preserved by selecting "Finder" from wt's menu (one extra keystroke vs. the old tool-form). On Linux, the new verb gives a working menu UX where the prior tool-form `hop <name> xdg-open` was the only option.

## Ad-hoc URL clone

`hop clone <url>` (URL form, detected by `looksLikeURL`: contains `://` OR (`@` AND `:`)) clones a URL not in `hop.yaml`, then registers it.

Flow:
1. Resolve target group (`--group <name>`, default `default`). Missing group → exit 1 with `hop: no '<group>' group in <config-path>. ...`.
2. Compute landing path. For map-shaped groups (`dir:` set): `<dir>/<name>`. For flat groups: `<code_root>/<org>/<name>` (org dropped if URL has none). `--name` overrides the URL-derived name.
3. Classify on-disk state:
   - `stateMissing` → `git clone <url> <path>`, then append URL to `hop.yaml` (unless `--no-add`), print path on stdout (unless `--no-cd`).
   - `stateAlreadyCloned` → emit `skip: already cloned at <path>` to stderr; still appends YAML and prints path (registers existing checkout).
   - `statePathExistsNotGit` → emit `hop clone: <path> exists but is not a git repo`; exit 1; no YAML write, no stdout.
4. URL already in target group's `urls` list → emit `skip: <url> already registered in '<group>'` to stderr, no YAML write, but still print path (unless `--no-cd`).

YAML write goes through `internal/yamled.AppendURL` (comment-preserving, atomic temp+rename — see [architecture/package-layout](../architecture/package-layout.md#internalyamled)).

## Exit code convention

Defined in `main.go::translateExit`:

| Code | Trigger |
|---|---|
| 0 | Success |
| 1 | Application error (default for all unmatched errors); also `errSilent` (caller already wrote stderr) |
| 2 | `errExitCode{code: 2}` — used by `cd`, `shell-init`, and `-R` for usage errors |
| 130 | `errFzfCancelled` — fzf user cancellation (Esc / Ctrl-C) |

Cobra's `SilenceUsage: true` and `SilenceErrors: true` are set on `rootCmd`, so `translateExit` is the sole stderr/exit path for top-level errors.

The `-R` flag bypasses cobra entirely (pre-Execute argv inspection) and uses `os.Exit` directly with the child's exit code (or 2 for usage errors, 1 for resolution errors).

## Shared helpers (`where.go`)

- `loadRepos() (repos.Repos, error)` — `config.Resolve()` → `config.Load()` → `repos.FromConfig()`. Used by every subcommand that reads `hop.yaml`.
- `resolveByName(query string) (*repos.Repo, error)` — implements the match-or-fzf algorithm without writing to stderr; returns typed sentinels (`errFzfMissing`, `errFzfCancelled`) so callers control which stderr to write to. **Pre-step (change `7eab`)**: if `query` contains a `/`, the function splits on the FIRST `/` (repo names from `hop.yaml` are URL basenames with no `/`, so first-`/` split is unambiguous even when wt worktree names themselves contain `/`), validates that both halves are non-empty (empty LHS / RHS returns an `*errExitCode{code: 2, msg: ...}`), recurses on the LHS to resolve the repo, then calls `resolveWorktreePath` to invoke `wt list --json` in the resolved repo's main checkout and find the entry whose `Name == rhs` (exact, case-sensitive). The returned `*repos.Repo` is a shallow copy of the LHS-resolved repo with `Path` replaced by the worktree's absolute path; all other fields (`Name`, `Group`, `URL`, `Dir`) describe the registry entry and stay unchanged. Worktree-resolution errors (uncloned-with-`/`, wt missing, no-such-worktree, malformed JSON) surface as `*errExitCode{code: 1, msg: <pre-formatted stderr line>}` so `translateExit` prints them verbatim — see "External tool failure messages" below.
- `resolveWorktreePath(repo *repos.Repo, wtName string) (*repos.Repo, error)` — sub-step of `resolveByName` for the `/`-suffixed branch (change `7eab`, lives in `resolve.go`). Guards: (1) repo MUST be cloned (`cloneState(repo.Path) == stateAlreadyCloned`) — this guard applies ONLY to `/`-suffixed queries so bare `hop <name> where` against uncloned repos keeps its existing permissive behavior; (2) invokes `listWorktrees(ctx, repo.Path)` (the package-level seam in `wt_list.go`); (3) returns the matching entry's path or one of the four `errExitCode` lines. Helper kept inline in `cmd/hop/` (NOT promoted to `internal/wt/`) per the wrapper-boundaries "promote later" pattern.
- `resolveOne(cmd, query) (*repos.Repo, error)` — cobra-friendly wrapper that writes `fzfMissingHint` to `cmd.ErrOrStderr()` and returns `errSilent` on missing fzf.
- `resolveAndPrint(cmd, query) error` — wraps `resolveOne` and writes `repo.Path` to stdout.
- `buildPickerLines(rs) []string` — builds the tab-separated lines piped to fzf. When two repos share a `Name`, the displayed first column gets a `[<group>]` suffix. The path column (used for match-back) is unique per repo.
- `listWorktrees(ctx, repoPath) ([]WtEntry, error)` — package-level `var` seam in `wt_list.go` initialised to `defaultListWorktrees`. The default builds a 5-second `context.WithTimeout` and routes through `proc.RunCapture(ctx, repoPath, "wt", "list", "--json")`, then unmarshals into `[]WtEntry`. The seam pattern mirrors `internal/fzf/fzf.go::runInteractive` — tests inject fakes without spawning a real `wt`. Returns `proc.ErrNotFound` when `wt` is missing on PATH (callers match via `errors.Is` to produce the install hint). Three call sites: `resolveWorktreePath` (single-worktree lookup for `<repo>/<wt>` path resolution — change `7eab`), `ls.go::runLsTrees` (fan-out for `hop ls --trees` — change `7eab`), and tab completion in `repo_completion.go` (both the post-slash `completeWorktreeCandidates` from change `7eab` and the pre-slash eager branch in `completeRepoNames` from change `odle`). Still below the threshold for a dedicated `internal/wt/` package — all four needs are identical (single-shot `wt list --json` with a 5-second timeout).

## External tool failure messages

Lazy: only checked when the tool is actually invoked. Exact stderr lines:

| Tool | Constant / location | Message |
|---|---|---|
| `fzf` | `where.go::fzfMissingHint` | `hop: fzf is not installed. Install it: brew install fzf (macOS) or apt install fzf (Debian).` |
| `git` | `clone.go::gitMissingHint` | `hop: git is not installed.` (also reused by `hop config scan` — lazy-checked at the first `git remote` invocation; empty scan trees with zero `.git` discoveries succeed without invoking `git`. Also reused by `hop pull`, `hop push`, and `hop sync` — emitted once and aborts the batch immediately on the first `proc.ErrNotFound`) |
| `<cmd>` for `-R` / tool-form | `main.go::runDashR` (formatted) | `hop: -R: '<cmd>' not found.` (when `<cmd>` is missing on PATH at exec time). Covers tool-form invocations like `hop <name> xdg-open` since they rewrite to `-R`. `hop <name> open` is **not** in this category — `open` is now a recognized $2 verb that delegates to `wt`, with its own missing-tool message |
| `wt` | `open.go::runOpen`, `resolve.go::resolveWorktreePath` (`wtMissingHint` constant), `ls.go::runLsTrees` | `hop: wt: not found on PATH.` (when `wt` is missing at exec time). The exact wording lives once as the `wtMissingHint` constant in `resolve.go` and is reused by `open.go`, the worktree-resolution branch of `resolveByName`, and `runLsTrees` so the line stays consistent across all three surfaces. Mitigated: `wt` is declared as a Homebrew formula dependency (`depends_on "sahil87/tap/wt"` in `Formula/hop.rb`), so `brew install sahil87/tap/hop` pulls it automatically. The runtime hint covers non-brew installs and the rare case of manual removal |
| `wt list --json` | `resolve.go::resolveWorktreePath`, `ls.go::runLsTrees` | Three additional wt-related stderr lines surface from the worktree-resolution and `ls --trees` paths (change `7eab`): `hop: worktree '<wt>' not found in '<name>'. Try: wt list (in <path>) or hop ls --trees` (no-such-worktree, exit 1); `hop: wt list: <err>` (non-zero exit or malformed JSON from `wt list --json` — no silent fallback to the main path; unparseable wt output is a real failure, exit 1); per-row `(wt list failed: <err>)` inline in `ls --trees` output (one corrupt `.git` shouldn't blank the table). Empty LHS / RHS in a `/`-suffixed query exits 2: `hop: empty repo name before '/'` / `hop: empty worktree name after '/'`. `/`-suffixed queries against uncloned repos exit 1 via the existing not-cloned wording `hop: '<name>' is not cloned. Try: hop clone <name>` BEFORE `wt list` is invoked — the cloned-state guard applies ONLY to `/`-suffixed queries; bare queries retain their existing permissive behavior |
| `brew` | `internal/update` | `hop update: brew not found on PATH.` (only when binary is brew-installed) |

The fzf/git hints trigger `errSilent` (exit 1) directly — the subcommand writes the hint to `cmd.ErrOrStderr()` and returns `errSilent`. The `-R` path bypasses cobra and writes directly to `os.Stderr` via `runDashR`. `brew` follows a slightly different path: `internal/update.Run` writes the hint and returns `proc.ErrNotFound`; the cobra wrapper in `cmd/hop/update.go` then catches `proc.ErrNotFound` via `errors.Is` and converts it to `errSilent`. The user-visible behavior is identical (single hint line on stderr, exit 1) — the indirection exists so `internal/update` stays free of cobra-specific sentinels.

## Binary-form hint texts (constants in `root.go`)

The root command's `RunE` returns one of three exit-code-2 errExitCode constants depending on the args shape (`bareNameHint`, `cdHint`, `toolFormHintFmt` — all in `root.go` next to `rootLong`).

`bareNameHint` (1-arg form `hop <name>`):

```
hop: bare-name dispatch is shell-only. Add 'eval "$(hop shell-init zsh)"' to your zshrc, or use: hop "<name>" where
```

`cdHint` (2-arg `cd` verb form `hop <name> cd`):

```
hop: 'cd' is shell-only. Add 'eval "$(hop shell-init zsh)"' to your zshrc, or use: cd "$(hop "<name>" where)"
```

`toolFormHintFmt` (2-arg tool-form `hop <name> <tool>` — `%s` gets `args[1]`):

```
hop: '%s' is not a hop verb (cd, where, open). For tool-form, install the shim: eval "$(hop shell-init zsh)", or use: hop -R "<name>" <tool> [args...]
```

The `where`-verb and `open`-verb branches do not have a hint — they succeed and either print to stdout (where) or exec wt with stdio inherited (open).

When the user invokes `hop <name> open` directly (without the shim) and picks "Open here", the binary itself cannot mutate the parent shell; wt's own `wt shell-setup` hint surfaces from wt directly in that path, since the shim is the only caller that sets `WT_CD_FILE` / `WT_WRAPPER`. Hop intentionally does not own this hint — the cd-handoff is a shim concern, not a binary concern.

## `hop shell-init <shell>` emitted text

The shared portion (`shell_init.go::posixInit`) is identical for zsh and bash — both shells understand `[[ ]]`, `${@:N}` slicing, and `local`. Only the appended completion script differs.

The shared `posixInit` defines:

- `hop()` function with this 5-step resolution ladder (top-down, first match wins). The grammar is `subcommand` xor `repo` in `$1` — never a tool, never a verb — so `$1` has only two interpretations and the ladder needs no PATH inspection or builtin/keyword filtering:
  1. **No args** → `command hop` (bare picker).
  2. **`__complete*`** → `command hop "$@"`. Cobra's hidden completion entrypoint must reach the binary; without this branch the function would route `__complete` to the bare-name dispatcher and break tab completion.
  3. **Known subcommand** (`clone|pull|push|sync|ls|shell-init|config|update|help|--help|-h|--version|completion`) → `_hop_dispatch "$@"`. The `help` token is in this list because cobra auto-wires `hop help [subcommand]`. `pull`, `push`, and `sync` were added alongside the same-named subcommands; without their entries here the shim's rule 5 would misroute `hop push <name>` into tool-form (`command hop -R push <name>`) and the binary would fail with `-R: 'push' not found.` — same misroute story for `pull` and `sync`. The list does NOT include `cd` or `where` — those moved to $2 verbs in the repo-verb grammar flip; `hop cd ...` and `hop where ...` fall into the otherwise branch and are treated as repo names (so `hop where outbox` becomes a tool-form attempt against a non-existent `where` repo, which fails at the binary's resolveByName — the migration story is "rewrite legacy callers to `hop <name> where` / `hop <name> cd`").
  4. **Flag-prefixed (`-*`)** → `command hop "$@"`.
  5. **Otherwise** — `$1` is treated as a repo name; dispatch on `$2`:
     - **`$# == 1`** → `_hop_dispatch cd "$1"` (bare-name → `cd`, shorthand for `hop <name> cd`).
     - **`$2 == "cd"`** → `_hop_dispatch cd "$1"` (explicit `cd` verb — same dispatch as bare-name).
     - **`$2 == "where"`** → `command hop "$1" where` (binary's `where`-verb dispatch handles directly).
     - **`$2 == "open"`** → `_hop_dispatch open "$1"` (open-verb dispatch — captures binary's stdout and `cd`s the parent shell iff non-empty; the binary delegates to `wt open` and prints a path only when the user picks "Open here").
     - **`$2 == "-R"`** → `command hop -R "$1" "${@:3}"` (canonical exec form). The shim rewrites the user-facing `hop <name> -R <cmd>...` to the binary's internal shape `hop -R <name> <cmd>...` so `extractDashR` continues to see `-R` followed by `<name>` followed by `<cmd>...`.
     - **otherwise (`$# >= 2`, `$2` not `cd`/`where`/`open`/`-R`)** → `command hop -R "$1" "$2" "${@:3}"` (tool-form sugar). Missing tools surface via the binary's `hop: -R: '<cmd>' not found.` error — there is no shim-side PATH check or cheerful escape hatch.

The shim does NOT call `command -v` or `type` on `$1` or `$2`, and does NOT print cheerful errors. The pre-flip ladder needed those because `$1` could be a tool, repo, or subcommand; the post-flip grammar removes the overload at the source. Direct binary invocations of the tool-form (`/path/to/hop <name> <tool>`) hit the binary's `RunE` 2-arg default branch, which prints `fmt.Sprintf(toolFormHintFmt, args[1])` and exits 2 — the tool-form is shim-only. Direct binary invocations of `hop <name>` (1 arg) and `hop <name> cd` similarly hit `RunE`'s 1-arg and 2-arg-`cd` branches and exit 2 with their respective hints. `hop <name> where`, `hop <name> open`, and `hop <name> -R <cmd>...` work without the shim — `where` prints the path; `open` invokes `wt open <path>` with stdio inherited (interactive menu reaches the user); `-R` execs the child. The `open` verb's parent-shell `cd` (for the "Open here" choice) only takes effect when invoked through the shim — the binary is a transparent passthrough, and wt's own `wt shell-setup` hint surfaces directly when the user picks "Open here" without any wrapper.
- `_hop_dispatch()` helper — handles three shell-mutating paths: the `cd` path (`command hop "$2" where` then `cd --`), the URL-detected `clone` path (`cd --` to the printed path on success), and the `open` path (creates a temp file via `mktemp`, exports `WT_CD_FILE`/`WT_WRAPPER` prefix-style on the `command hop "$2" open` invocation, then reads the temp file with `[[ -s "$cdfile" ]]` and `cd -- "$target"` if non-empty; cleans up via `rm -f`). The `cd)` arm has only one external call (`command hop "$2" where`); both callers (1-arg bare-name and 2-arg explicit-cd) always pass `$1` as the dispatch's `$2`, so there is no no-`$2` fallback. The `open)` arm uses a temp file (not stdout capture) because wt's interactive menu reaches the user's terminal via stdout; capturing stdout with `$(...)` would swallow the menu and hang.
- `h() { hop "$@"; }` — single-letter alias.
- `hi() { command hop "$@"; }` — un-shadowed alias (calls the binary directly, bypassing the shim).

The cobra-generated completion is appended at runtime — `rootCmd.GenZshCompletion(out)` for zsh, `rootCmd.GenBashCompletionV2(out, true)` for bash. The `rootCmd` reference is captured in `main.go::rootForCompletion` (a package-level var set in `main()`). After the cobra completion, the shell-init emits the alias-completion line:
- zsh: `compdef _hop h hi`
- bash: `complete -o default -F __start_hop h hi`

so the `h` and `hi` aliases share the same completion logic — without this, tab completion would only work on `hop`, not on the aliases.

### Repo positional tab completion (`$1`)

The `ValidArgsFunction` for the root command's `$1` slot is `src/cmd/hop/repo_completion.go::completeRepoNames`. It runs three branches in order:

1. **Verb position (`len(args) == 1` on the root command)** → return `["cd", "where", "open"]` with `ShellCompDirectiveNoFileComp`. Non-root callers (e.g., `clone` via `completeCloneArg`) suppress completion past `$1`.
2. **Post-slash worktree branch (`toComplete` contains `/`)** → delegate to `completeWorktreeCandidates`, which splits on the first `/`, resolves the LHS via `rs.MatchOne`, invokes `listWorktrees(ctx, repo.Path)`, and returns `<lhs>/<wt>` strings (the full token the user is composing — cobra prefix-matches against `toComplete`, so bare wt names would mis-replace the LHS). Unchanged since change `7eab`.
3. **Pre-slash eager worktree branch (`toComplete` has no `/`)** — change `odle`. After collecting today's `names` slice (loaded repos minus the subcommand-collision filter), the function probes for an eager-expansion fire. All four guards must hold for the menu to surface:
   - `rs.MatchOne(toComplete)` returns exactly one repo. Ambiguous prefixes (0 or 2+ matches) bypass the branch entirely — the unique-match guard is the cost gate that prevents `wt list --json` fan-out across N repos. Empty `toComplete` falls into this case naturally because `MatchOne("")` returns every repo.
   - The matched repo's `Name` is NOT in `subNames` (subcommand collision). The collision filter runs BEFORE the eager check, so a repo literally named `clone` (which cobra would dispatch to the `hop clone` subcommand before the bare-form resolver ever sees it) is never offered with worktree suffixes.
   - `cloneState(repo.Path)` returns `stateAlreadyCloned`. Uncloned repos have no `.git` to query — matches `completeWorktreeCandidates`'s precedent.
   - `listWorktrees(context.Background(), repo.Path)` returns nil error AND `len(entries) >= 2`. The `>= 2` threshold means the menu only surfaces when worktree-vs-main is a real choice; a 1-worktree repo keeps today's auto-space-and-finalize behavior.

   When all four hold, the function returns `[<repo>, <repo>/<entries[0].Name>, <repo>/<entries[1].Name>, ...]` (bare repo at position 0, then `wt list --json` source order verbatim — matches `hop ls --trees` ordering, no alphabetical reordering) with directive `cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace`. `NoSpace` is essential — without it the shell auto-finalizes `$1` after the user picks the bare form, defeating the menu. The bitwise-OR composes cleanly with the existing `NoFileComp` flag.

   **Silent-fallback contract**: every failure mode (uncloned, `wt` missing, `wt list --json` non-zero exit, malformed JSON, context timeout, fewer than 2 entries) returns the original `names` list with today's `ShellCompDirectiveNoFileComp` only. The completion path never writes to stderr — enforced by the package-level comment block in `repo_completion.go` and consistent with `completeWorktreeCandidates`.

   **Shell-degradation note**: bash users (and zsh users without `menu select`) see the eager candidates as a flat list rather than arrow-key navigation. Candidates are still surfaced; further disambiguation is by typing more characters. The bare-cd cost — `h <name><TAB>` against a multi-worktree repo no longer auto-spaces, so the user pays one extra Space keystroke to commit "use main" — is the accepted price of discoverability.

   No caching across Tab presses: each eager-fire invocation runs `wt list --json` fresh (Constitution II — no database; measured 6.3-8.1ms median latency is below the perception threshold). The shared `listWorktrees` seam is documented in [match-resolution](match-resolution.md#worktree-resolution-sub-step-change-7eab).

## Tool-form dispatch

The shim's tool-form sugar (`hop <name> <tool> [args...]`) is a generalization of the removed `hop code` and `hop open` subcommands to any binary on PATH. The repo name lives in `$1` and the tool name lives in `$2` — never the other way around:

| User types | Shim behavior | Binary sees |
|---|---|---|
| `hop dotfiles cursor` | otherwise → `command hop -R dotfiles cursor` | argv `[hop, -R, dotfiles, cursor]` → `extractDashR` → exec `cursor` with `cwd = <dotfiles>` |
| `hop outbox git status` | otherwise → `command hop -R outbox git status` | argv `[hop, -R, outbox, git, status]` → exec `git status` with `cwd = <outbox>` |
| `hop dotfiles /bin/pwd` | otherwise → `command hop -R dotfiles /bin/pwd` | exec `/bin/pwd` with `cwd = <dotfiles>` |
| `hop outbox pwd` | otherwise → `command hop -R outbox pwd` | exec `/bin/pwd` (the on-PATH binary, not the builtin) with `cwd = <outbox>` — prints the path. No special handling; the simpler grammar earns this redundancy |
| `hop outbox xdg-open .` | otherwise → `command hop -R outbox xdg-open .` | exec `xdg-open .` with `cwd = <outbox>` (Linux). Note: `hop outbox xdg-open` (no positional after) and `hop outbox open` no longer reach this path — `open` is now a recognized $2 verb (see below) |
| `hop outbox notarealbinary` | otherwise (rule 5, default) → `command hop -R outbox notarealbinary` | `hop: -R: 'notarealbinary' not found.`, exit 1 |
| `hop outbox -R git status` | rule 5, `$2 == -R` → `command hop -R outbox git status` (canonical exec form) | same as `hop outbox git status` |
| `hop outbox where` | rule 5, `$2 == where` → `command hop "outbox" where` | binary's `where`-verb dispatch resolves and prints the path |
| `hop outbox cd` | rule 5, `$2 == cd` → `_hop_dispatch cd "outbox"` | (no binary call; `cd` happens in the parent shell via `command hop "$2" where` lookup + `cd --`) |
| `hop outbox open` | rule 5, `$2 == open` → `_hop_dispatch open "outbox"` | binary's `open`-verb dispatch resolves the path, chdirs there, and execs `wt open` with `WT_CD_FILE`/`WT_WRAPPER` set. Stdout is non-empty iff user picks "Open here" — the shim then `cd`s the parent. Replaces the prior tool-form for both `open` (Darwin → `/usr/bin/open`) and is the recommended path on Linux instead of `hop <name> xdg-open` |
| `hop outbox` (1 arg, outbox is a repo) | rule 5, `$# == 1` → `_hop_dispatch cd "outbox"` | (no binary call; `cd` happens in the parent shell — shorthand for `hop outbox cd`) |
| `hop where outbox` (legacy) | rule 5, `$2 == outbox` (anything-else) → `command hop -R "where" "outbox"` | binary's `resolveByName("where")` finds no match; tool-form path errors with no-match. Old scripts must migrate to `hop outbox where` |
| `hop cd outbox` (legacy) | rule 5, `$2 == outbox` (anything-else) → `command hop -R "cd" "outbox"` | same as above; migrate to `hop outbox cd` (or just `hop outbox`) |

The tool-form is **shim-only**: the binary does not interpret it. Direct binary invocations (`/path/to/hop dotfiles cursor`) hit the binary's `RunE` 2-arg default branch, which prints `fmt.Sprintf(toolFormHintFmt, "cursor")` (`hop: 'cursor' is not a hop verb (cd, where, open). For tool-form, install the shim: ..., or use: hop -R "<name>" <tool> [args...]`) and exits 2. Cobra's `MaximumNArgs(2)` cap rejects 3+ positionals before RunE runs (with cobra's `accepts at most 2 arg(s)` error). Scripts and CI jobs that bypass the shim must use `hop -R <name> <tool>` explicitly (or `hop <name> where` for path resolution).

## `hop clone` per-line output

All status lines go to **stderr** (stdout is reserved for resolved paths, used by the shell shim's `cd`). Formats:

- `clone: <url> → <path>` (before invoking `git clone`)
- `skip: already cloned at <path>` (when `<path>/.git` exists)
- `skip: <url> already registered in '<group>'` (URL form, when URL is already in the target group's `urls` list)
- `hop clone: <path> exists but is not a git repo` (path conflict, exits 1 single / counts toward `failed` for `--all`)
- `summary: cloned=<N> skipped=<M> failed=<F>` (only `--all`)

Clone uses a 10-minute timeout (`clone.go::cloneTimeout`).

## `hop pull` / `hop push` / `hop sync` per-line output

All status lines go to **stderr** (stdout is empty — the shim does NOT `cd` after these verbs). Formats:

- `pull: <name> ✓ <last-line>` / `push: <name> ✓ <last-line>` / `sync: <name> ✓ <pull-summary> <push-summary>` (success — `<last-line>` is the last non-empty line of git's stdout via `lastNonEmptyLine` in `pull.go`; for pull e.g. "Already up to date." / "Fast-forward"; for push e.g. "Everything up-to-date" / "<src> -> <dst>")
- `sync: <name> ✓ committed, <pull-summary>, <push-summary>` (sync success when the auto-commit fired — comma-separated, `committed` prefix signals hop made a commit on the user's behalf; absence of `committed` in the line means the tree was clean and behavior matched the pre-auto-commit `hop sync`)
- `pull: <name> ✗ <err>` / `push: <name> ✗ <err>` / `sync: <name> ✗ <err>` (failure — git's own stderr is forwarded verbatim by `proc.RunCapture`; the hop line summarizes for the per-repo log)
- `sync: <name> ✗ commit failed: <err>` (auto-commit failed — typically a `pre-commit` / `commit-msg` hook rejection; rebase and push are NOT invoked when this fires. Mirrors the `push failed: <err>` shape)
- `sync: <name> ✗ rebase conflict — resolve manually with: git -C <path> rebase --continue` (specialized hop-side hint emitted only when git's output contains a `CONFLICT` substring on rebase failure; replaces the verbatim git error line for that case — `git push` is NOT invoked when this fires; applies to both clean and dirty paths)
- `sync: <name> ✗ push failed: <err>` (rebase succeeded but push failed — non-fast-forward, network, etc.; applies to both clean and dirty paths)
- `skip: <name> not cloned` (`<path>/.git` missing — counts toward `skipped` in batch mode; exits 1 in single-repo mode; shared verbatim across pull, push, and sync)
- `summary: pulled=<N> skipped=<M> failed=<K>` for `pull` / `summary: pushed=<N> skipped=<M> failed=<K>` for `push` / `summary: synced=<N> skipped=<M> failed=<K>` for `sync` (only batch mode — group match or `--all`)

Each git invocation runs through `internal/proc.RunCapture` with an **independent** 10-minute timeout (reusing `clone.go::cloneTimeout`), not a shared batch budget. `pull` and `push` use one context per repo (one git invocation each); `sync` uses up to five independent contexts per repo — `git status --porcelain`, then (only when status is non-empty) `git add --all` and `git commit -m <msg>`, then `git pull --rebase`, and finally `git push`. `git` missing on PATH emits `gitMissingHint` (`hop: git is not installed.`) once and aborts a batch immediately (no further repos attempted, no summary line emitted) — mirrors `clone.go::cloneAll`'s `proc.ErrNotFound` early-return.

## Sync auto-commit

`hop sync` auto-commits dirty working trees before the existing rebase + push. The motivation is to fully replace ad-hoc shell helpers (e.g., `xpush`-style scripts) with a single hop verb driven by `hop.yaml` rather than a hardcoded directory list. Behavior is default-on for dirty trees; clean trees are unchanged.

Per-repo order of operations (single repo `<path>`):

1. `git -C <path> status --porcelain` — dirty detection.
2. If status output is non-empty:
   1. `git -C <path> add --all` — stages tracked modifications, deletions, AND untracked files (matches xpush's `git add --all :/`).
   2. `git -C <path> commit -m "<message>"` — default message `chore: sync via hop`, overridable via `-m / --message <msg>` (string flag, no toggle — `-m` only changes the message; auto-commit remains on for dirty trees).
3. `git -C <path> pull --rebase` (existing).
4. `git -C <path> push` (existing).

Any step's failure aborts that repo's sync and counts toward `failed` in batch mode. Rebase and push behavior (including the `CONFLICT` substring detection and the `rebase conflict` hint) is identical to the pre-auto-commit code path.

**Hooks are respected.** No `--no-verify` is passed. A failing `pre-commit` (or `commit-msg`) hook causes `git commit` to fail, hop emits `sync: <name> ✗ commit failed: <err>`, and rebase + push are skipped. Users who want to bypass hooks can drop to `hop -R <name> git commit --no-verify` (or run git directly).

**Default message**: `chore: sync via hop`. Chosen for Conventional Commits compatibility (`chore:` prefix), greppability (`git log --grep "via hop"`), brevity (19 chars), and honesty (commits are explicitly attributed to hop).

**`-m / --message` flag**: type `string`, default `chore: sync via hop`, no short alias other than `-m`. Applies to every dirty repo in a batch — there is no per-repo override path. Same flag is accepted in single, group, and `--all` invocations.

**`hop push` is intentionally NOT changed** — see the inventory row above. The asymmetry exists because pushing without rebasing is the riskier op; `hop sync` is the safe verb because it always rebases first.

Helper: `commitDirtyTree` lives inline in `sync.go` (not extracted to a shared file). It encapsulates the `status → add → commit` sequence above, distinguishes a clean tree from a real failure, and emits the `sync: <name> ✗ commit failed: <detail>` line on its own so callers don't duplicate it. If a future subcommand needs the same shape, extract then.

## `hop update` — self-update via Homebrew

Implementation in `internal/update`; the cobra factory in `cmd/hop/update.go` is a thin wrapper. The flow:

1. **Detect brew install**: walk `os.Executable()` through `filepath.EvalSymlinks` and check whether the resolved path contains `/Cellar/`. If not, print `hop v<X> was not installed via Homebrew.\nUpdate manually, or reinstall with: brew install sahil87/tap/hop` and exit 0.
2. **Refresh brew index**: `brew update --quiet` with a 30-second timeout (via `proc.Run` + `context.WithTimeout`). Failure exits 1.
3. **Query latest version**: `brew info --json=v2 sahil87/tap/hop` and parse `formulae[0].versions.stable`. The formula name is **fully qualified** (`sahil87/tap/hop`) to dodge a name collision with the Homebrew core `hop` cask (an HWP document viewer).
4. **Compare versions**: normalize both sides by stripping a leading `v` (binary reports `v0.0.3`, brew reports `0.0.3`). If equal, print `Already up to date (v<X>).` and exit 0.
5. **Upgrade**: `brew upgrade sahil87/tap/hop` with a 120-second timeout. Stream brew's stdout/stderr through to the user via `proc.RunForeground` so progress is visible. On success, print `Updated to v<new>.` and exit 0.

All `brew` invocations route through `internal/proc` (Constitution Principle I — no direct `os/exec` outside `internal/proc`). The package exposes `Run(currentVersion string, out, errOut io.Writer) error` as its single public entry point. The `out`/`errOut` writers receive only the wrapper messages this package emits; subprocess output from `brew` is routed by `internal/proc` to the parent's `os.Stdout`/`os.Stderr` directly. Production callers pass `os.Stdout` and `os.Stderr` (via `cmd.OutOrStdout()` / `cmd.ErrOrStderr()`) to keep both consistent. When `brew` is missing on PATH, `Run` returns `proc.ErrNotFound`; the cobra wrapper converts that to `errSilent` so `translateExit` doesn't print a second error line.
