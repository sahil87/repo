# hop

> Part of [@sahil87's open source toolkit](https://ai.shll.in) — see all projects there.

[![Latest release](https://img.shields.io/github/v/release/sahil87/hop)](https://github.com/sahil87/hop/releases) [![Downloads](https://img.shields.io/github/downloads/sahil87/hop/total)](https://github.com/sahil87/hop/releases) [![Stars](https://img.shields.io/github/stars/sahil87/hop?style=social)](https://github.com/sahil87/hop/stargazers)

A small Go CLI that turns one config file (`hop.yaml`) into a personal directory of all your git repos — navigate, clone, run commands, and batch-update them from any directory.

## Why hop?

- **One config, every machine** — `hop.yaml` lists every repo you care about (with groups). Drop it in Dropbox, dotfiles, or `$HOP_CONFIG`, and your repo directory follows you between laptops.
- **Fuzzy navigation** — `h ot<TAB>` or `hop ot` fuzzy-matches `outbox` and `cd`s your shell straight there. No more `cd ~/code/sahil87/outbox`.
- **Run anything inside a repo, from anywhere** — `hop dotfiles cursor` opens your dotfiles in Cursor without changing your cwd. Works for any tool: `hop outbox git status`, `hop infra-tf terraform plan`, `hop loom npm test`.
- **Batch git ops over groups** — `hop pull --all` pulls every cloned repo. `hop sync work` rebases-and-pushes every repo in the `work` group. Group-level fan-out built in.
- **Bootstrap from disk, not yaml-by-hand** — `hop config scan ~/code` walks your existing clones, reads `git remote`, and populates `hop.yaml` for you. Comment-preserving merges, idempotent re-runs.
- **Plays nicely with [`wt`](https://github.com/sahil87/wt)** — `hop <name> open` delegates to wt's app menu, so you get the same "open in editor / terminal / file manager / cd here" experience for every repo in the registry. The `hop <name>/<wt-name>` suffix lands you straight inside a worktree (`h outbox/feat-x` cds you there, `hop outbox/feat-x git status` runs git in it), and `hop ls --trees` shows worktree state across every repo at a glance.

## The mental model

Three blocks. The repo name is the anchor — same color in both rows, so you can see it move between forms.

> **Colored half** = what you type · **grey half** = what the `h` alias or tab completion fills in.

**General form:**

![hop](https://img.shields.io/badge/h-op-9ca3af?labelColor=1f6feb&style=for-the-badge) ![arrow](https://img.shields.io/badge/-%E2%86%92-lightgrey?style=for-the-badge) ![repo](https://img.shields.io/badge/re-po--name-9ca3af?labelColor=7c3aed&style=for-the-badge) ![arrow](https://img.shields.io/badge/-%E2%86%92-lightgrey?style=for-the-badge) ![cmd](https://img.shields.io/badge/command-059669?style=for-the-badge)

```sh
h  out<TAB>     cursor .            # → hop outbox cursor .
h  dot<TAB>     ls -la              # → hop dotfiles ls -la
```

**Git form** — git commands come to mind first, so the repo name moves to the end:

![hop](https://img.shields.io/badge/h-op-9ca3af?labelColor=1f6feb&style=for-the-badge) ![arrow](https://img.shields.io/badge/-%E2%86%92-lightgrey?style=for-the-badge) ![git](https://img.shields.io/badge/git--command-d97706?style=for-the-badge) ![arrow](https://img.shields.io/badge/-%E2%86%92-lightgrey?style=for-the-badge) ![repo](https://img.shields.io/badge/re-po--name-9ca3af?labelColor=7c3aed&style=for-the-badge)

```sh
hop  pull   out<TAB>                # → hop pull outbox
hop  push   --all
hop  sync   work                    # 'work' is a group
hop  clone  out<TAB>
```

Same hop, same repo — just an ergonomic reorder for the case where the git verb is the first thing you think of.

**Worktree form** — the repo slot accepts an optional `/<wt-name>` suffix. Everything else is unchanged; every verb (cd, where, open, tool-form, -R) inherits it:

![hop](https://img.shields.io/badge/h-op-9ca3af?labelColor=1f6feb&style=for-the-badge) ![arrow](https://img.shields.io/badge/-%E2%86%92-lightgrey?style=for-the-badge) ![repo](https://img.shields.io/badge/re-po--name-9ca3af?labelColor=7c3aed&style=for-the-badge) ![sep](https://img.shields.io/badge/-%2F-lightgrey?style=for-the-badge) ![wt](https://img.shields.io/badge/fe-at--x-9ca3af?labelColor=0d9488&style=for-the-badge) ![arrow](https://img.shields.io/badge/-%E2%86%92-lightgrey?style=for-the-badge) ![cmd](https://img.shields.io/badge/command-059669?style=for-the-badge)

```sh
h  out<TAB>/fe<TAB>     git status              # → hop outbox/feat-x git status
h  out<TAB>/<TAB>       cursor .                # → TAB after / lists worktrees of outbox
hop  out<TAB>/main      where                   # → main-worktree path (same as bare `hop outbox where`)
```

The `/<wt-name>` resolves via `wt list --json`, so wt must be on `PATH` for any `/`-suffixed query — bare `hop outbox` is unaffected. The full grammar (subcommands, verbs, flags) is in [Grammar at a glance](#grammar-at-a-glance) below.

## Install

### Homebrew (macOS and Linux)

```sh
brew install sahil87/tap/hop
```

To upgrade later, run `hop update` — self-upgrades via Homebrew. When `hop` was installed from source or a release tarball, it prints a hint and exits without invoking brew.

### From source

```sh
git clone https://github.com/sahil87/hop.git
cd hop
just install
```

Builds the binary and copies it to `~/.local/bin/hop`. Make sure that directory is on your `$PATH`.

## Shell integration

The shell shim is what makes `hop <name>` actually `cd` your shell. Install it once:

```sh
eval "$(hop shell-init zsh)"   # in ~/.zshrc
eval "$(hop shell-init bash)"  # in ~/.bashrc
```

This installs the `hop` shell function, the `h` and `hi` aliases, the bare-name dispatcher, the `hop <name> <tool>` tool-form sugar, and tab completion. Without it, the binary still works — it just can't change your parent shell's directory (a Unix constraint, not a hop limitation). See [Gotchas](#gotchas) for the shim-vs-binary details.

> 💡 Have other sahil87 tools? [`shll shell-install`](https://github.com/sahil87/shll#shll-shell-install--wire-the-rc-file-recommended) handles all of their shell integrations and autocompletions at once.

## First run

Bootstrap a starter `hop.yaml`:

```sh
hop config init
hop config where   # show where it lives
```

By default the file lives at `$XDG_CONFIG_HOME/hop/hop.yaml` (or `~/.config/hop/hop.yaml`). Set `$HOP_CONFIG` in your shell rc to point at a tracked location (Dropbox, a git-tracked dotfile, etc.) so the config moves with you.

If you already have repos cloned somewhere, let hop discover them:

```sh
hop config scan ~/code              # preview: prints what it would write
hop config scan ~/code --write      # merge the result into hop.yaml (comments preserved)
```

Scan walks the directory (default depth 3, `--depth N` to override), inspects each git repo's `origin` remote, and auto-derives groups: repos whose on-disk path matches the `<code_root>/<org>/<name>` convention land in `default`; repos in non-convention layouts get a group named after their parent directory. Worktrees, submodules, bare repos, and repos with no remote are skipped.

## Quick tour

Three things hop does. Each is a complete mental model on its own.

### 1. Navigate

```text
$ hop ls
prompt-pantry  /Users/sahil/code/sahil-weaver/prompt-pantry
outbox         /Users/sahil/code/sahil87/outbox
fab-kit        /Users/sahil/code/sahil87/fab-kit
wt             /Users/sahil/code/sahil87/wt
idea           /Users/sahil/code/sahil87/idea
…

$ hop outbox where
/Users/sahil/code/sahil87/outbox

$ h out                       # fuzzy substring match → cd into outbox
$ pwd
/Users/sahil/code/sahil87/outbox

$ h outbox/feat-x             # cd into the feat-x worktree of outbox (via wt list --json)
$ pwd
/Users/sahil/code/sahil87/outbox.worktrees/feat-x

$ hop ls --trees              # one-shot worktree status across every cloned repo
outbox    3 trees  (main, feat-x*, hotfix↑2)
dotfiles  1 tree   (main)
hop       2 trees  (main, refactor-resolve)
loom      (not cloned)

$ hop                         # bare → fzf picker over all repos, prints selection
```

`h` is the single-letter alias. `hi <name>` (also installed by the shim) bypasses the shim and invokes the binary directly — useful when you want the path printed instead of `cd`'d.

The `/<wt-name>` suffix is optional, completes after `<TAB>` (so `h outbox/<TAB>` lists worktrees), and every verb inherits it — `hop outbox/feat-x where`, `hop outbox/feat-x open`, `hop outbox/feat-x git status`, all work. Bare `hop outbox` is unchanged. `wt` must be on `PATH` for any `/<wt>`-suffixed query.

### 2. Run anything inside a repo

The tool-form `hop <name> <tool> [args...]` runs *anything* with cwd set to that repo, then returns you to where you started. The repo name is the first arg; the rest is forwarded verbatim to the child.

```text
$ pwd
/tmp/scratch

$ hop outbox git status
On branch main
Your branch is up to date with 'origin/main'.

nothing to commit, working tree clean

$ pwd
/tmp/scratch                  # cwd unchanged
```

A few useful variants:

```sh
hop dotfiles cursor .                # open dotfiles in Cursor (the trailing . matters — see Gotchas)
hop infra-tf terraform plan          # run terraform inside infra-tf
hop outbox -R jq '.foo' file.json    # explicit -R form: same effect, clearer in scripts
hop outbox open                      # delegates to wt's app menu (editor / terminal / cd here)
```

### 3. Batch git ops

`pull`, `push`, and `sync` accept a repo name, a group name, or `--all`:

```sh
hop pull outbox                # pull a single repo
hop pull default               # pull every cloned repo in the `default` group
hop pull --all                 # pull every cloned repo in hop.yaml
hop sync work                  # `git pull --rebase` then `git push` for every repo in `work`
hop sync --all                 # same, every repo
```

Each command emits a per-repo `✓` / `✗` / `skip` line on stderr and a final `summary: pulled=N skipped=M failed=K`. `sync` skips the push when rebase hits a conflict and prints a `git -C <path> rebase --continue` hint. Uncloned repos are silently skipped — `hop clone --all` first if you want to materialize them.

## Grammar at a glance

The shim's rule is simpler than the spec makes it sound: **the first positional is either a subcommand or a repo name — never both.** Once you internalize that, everything else falls out.

| You type | Shim does |
|----------|-----------|
| `hop` | Bare fzf picker over all repos. |
| `hop <subcommand>` (`ls`, `clone`, `pull`, `sync`, `config`, `update`, …) | Routes to the subcommand. |
| `hop <name>` | `cd` into the repo (shim-rewrites to `cd "$(hop <name> where)"`). |
| `hop <name> cd` | Same — explicit verb form. |
| `hop <name> where` | Prints the absolute path. |
| `hop <name> open` | Delegates to wt's app menu. |
| `hop <name> -R <cmd> ...` | Runs `<cmd> ...` with cwd = repo. |
| `hop <name> <tool> ...` | Sugar — shim rewrites to `hop -R <name> <tool> ...`. |
| `hop <name>/<wt>` | Same as `hop <name>` but lands in the named worktree (resolved via `wt list --json`). All verbs above accept the suffix — `<name>/<wt> where`, `<name>/<wt> open`, `<name>/<wt> <tool> ...`, etc. |
| `hop ls --trees` | Per-repo worktree summary across the registry (`*` = dirty, `↑N` = unpushed commits). |

Tab completion knows which slot you're in: `hop <TAB>` offers subcommands + repo names; `hop outbox <TAB>` offers verbs + tools; `hop outbox/<TAB>` offers worktree names for that repo.

## Config schema

`hop.yaml` is grouped by named sections under `repos:`, with optional global `config:` fields:

```yaml
config:
  code_root: ~/code   # optional; defaults to ~

repos:
  default:
    - git@github.com:sahil87/hop.git
    - git@github.com:sahil87/wt.git

  vendor:
    dir: ~/vendor
    urls:
      - git@github.com:some-vendor/their-tool.git
```

A flat list (`default` above) uses convention: each URL lands at `<code_root>/<org>/<name>`. A map with `dir:` (`vendor` above) overrides convention: each URL lands at `<dir>/<name>`, with `org` ignored. Group names match `^[a-z][a-z0-9_-]*$`.

## Gotchas

- **`hop <name>` and `h <name>` need the shell shim.** A binary can't change its parent shell's cwd — that's the same Unix constraint wt hits with its "Open here" menu option. Without the shim, the binary prints a hint pointing at `eval "$(hop shell-init zsh)"` or the workaround `cd "$(hop <name> where)"`.
- **Tool-form (`hop <name> <tool>`) is shim-only too.** The shim rewrites it to `hop -R <name> <tool>` before the binary sees it. In scripts and CI, use the binary-direct form `hop -R <name> <tool> ...` (and `hop <name> where` for path resolution — that one's handled by the binary).
- **Substring match is on the repo name only.** Not URL, not path, not group. `hop ot` matches `outbox` but not the URL `git@github.com:org/outbox.git`. When two repos in different groups share a name, the picker shows `name [group]` to disambiguate.
- **No `--force` on `push` or `sync`.** Intentional — for nuanced single-repo cases, reach for `hop <name> -R git push --force` and you'll get the full git output. The batch wrappers stay safe by default.
- **`hop <name> cursor` / `code` need a trailing `.`** — e.g. `hop dotfiles cursor .`. Not a hop quirk: both editors take `[paths...]` as positional args and, when invoked with none, restore the previously open folder instead of opening the cwd. The `.` is what tells them "open *this* directory." Tools that operate on cwd by default (`git status`, `terraform plan`, `ls`, `npm test`) don't need it.
- **The `<name>/<wt>` suffix needs `wt` on `PATH`.** Hop shells out to `wt list --json` to resolve the worktree name (no state cached in `hop.yaml` — worktrees are wt's domain). Bare `hop <name>` queries never invoke wt. The Homebrew formula pulls wt in as a dependency; for non-brew installs, `brew install sahil87/tap/wt` or build from source.

## Reference

- `hop --help` — full subcommand listing
- [`docs/specs/cli-surface.md`](docs/specs/cli-surface.md) — canonical CLI contract (every subcommand, exit codes, stdout/stderr conventions, every behavioral scenario)
- [`docs/specs/config-resolution.md`](docs/specs/config-resolution.md) — config search order and `hop.yaml` schema
