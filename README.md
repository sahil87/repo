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
- **Plays nicely with [`wt`](https://github.com/sahil87/wt)** — `hop <name> open` delegates to wt's app menu, so you get the same "open in editor / terminal / file manager / cd here" experience for every repo in the registry.

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

> **macOS Gatekeeper note**: tarball releases are not signed. Run `xattr -d com.apple.quarantine /path/to/hop` once after extracting, or use the Homebrew install above (brew strips the quarantine attribute).

## Shell integration

The shell shim is what makes `hop <name>` actually `cd` your shell. Install it once:

```sh
eval "$(hop shell-init zsh)"   # in ~/.zshrc
eval "$(hop shell-init bash)"  # in ~/.bashrc
```

This installs the `hop` shell function, the `h` and `hi` aliases, the bare-name dispatcher, the `hop <name> <tool>` tool-form sugar, and tab completion. Without it, the binary still works — it just can't change your parent shell's directory (a Unix constraint, not a hop limitation). See [Gotchas](#gotchas) for the shim-vs-binary details.

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

$ hop                         # bare → fzf picker over all repos, prints selection
```

`h` is the single-letter alias. `hi <name>` (also installed by the shim) bypasses the shim and invokes the binary directly — useful when you want the path printed instead of `cd`'d.

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
hop dotfiles cursor                  # open dotfiles in Cursor
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

Tab completion knows which slot you're in: `hop <TAB>` offers subcommands + repo names; `hop outbox <TAB>` offers verbs + tools.

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

## Reference

- `hop --help` — full subcommand listing
- [`docs/specs/cli-surface.md`](docs/specs/cli-surface.md) — canonical CLI contract (every subcommand, exit codes, stdout/stderr conventions, every behavioral scenario)
- [`docs/specs/config-resolution.md`](docs/specs/config-resolution.md) — config search order and `hop.yaml` schema
