# hop

> Part of [@sahil87's open source toolkit](https://ai.shll.in) — see all projects there.

A small Go CLI for locating, opening, and operating on git repositories listed in `hop.yaml`. The dominant use case is navigation: `hop <name>` prints a path; `h <name>` (the single-letter alias) `cd`s your shell into that repo via the bare-name dispatcher. To run a command inside a repo without changing your cwd, the binary supports `hop -R <name> <cmd>...`; once you've installed the shell integration (`eval "$(hop shell-init zsh)"` or `bash`), the friendlier `hop <name> <tool> [args...]` form (e.g. `hop dotfiles cursor`) becomes available — it's a shim-side rewrite to `hop -R`, so it does not work when invoking the binary directly from a script.

## Install

### Homebrew (macOS and Linux)

```sh
brew install sahil87/tap/hop
```

To update later:

```sh
hop update
```

`hop update` self-upgrades via Homebrew (`brew update` then `brew upgrade sahil87/tap/hop`). When `hop` was installed from source or a release tarball, `hop update` prints a hint and exits without invoking brew.

### GitHub Release tarball

Download the appropriate tar.gz for your platform from the [latest release](https://github.com/sahil87/hop/releases/latest) — assets are named `hop-{os}-{arch}.tar.gz` (where `{os}` is `darwin` or `linux` and `{arch}` is `arm64` or `amd64`). Extract and place the `hop` binary on your `$PATH`.

> **macOS Gatekeeper note**: The released binaries are not signed or notarized (out of scope for now — see `docs/specs/build-and-release.md`). On first run, macOS will block the binary with `"hop" cannot be opened because the developer cannot be verified`. To allow it, either run `xattr -d com.apple.quarantine /path/to/hop` once after extracting, or open System Settings → Privacy & Security and click "Allow Anyway" after the first blocked attempt. Homebrew installs typically don't trigger this since brew strips the quarantine attribute.

### From source

```sh
git clone https://github.com/sahil87/hop.git
cd hop
just install
```

Builds the binary and copies it to `~/.local/bin/hop`. Make sure that directory is on your `$PATH`.

## Shell integration

For the shell integration (bare-name `cd`, the `hop <name> <tool>` sugar, the `h` and `hi` aliases, and tab completion):

```sh
eval "$(hop shell-init zsh)"   # zsh
eval "$(hop shell-init bash)"  # bash
```

Add that line to your `~/.zshrc` or `~/.bashrc`.

## First run

Bootstrap a starter `hop.yaml`:

```sh
hop config init
hop config where   # show where it lives
```

By default the file lives at `$XDG_CONFIG_HOME/hop/hop.yaml` (or `~/.config/hop/hop.yaml`). Set `$HOP_CONFIG` in your shell rc to point at a tracked location (Dropbox, a git-tracked dotfile, etc.) so the config moves with you.

If you already have repos cloned somewhere, `hop config scan` populates `hop.yaml` from disk instead of asking you to type each URL by hand:

```sh
hop config scan ~/code              # preview: prints what it would write
hop config scan ~/code --write      # merge the result into hop.yaml (comments preserved)
```

Scan walks the directory (default depth 3, `--depth N` to override), inspects each git repo's `origin` remote, and auto-derives groups: repos whose on-disk path matches the `<code_root>/<org>/<name>` convention land in `default`; repos in non-convention layouts get a group named after their parent directory. Worktrees, submodules, bare repos, and repos with no remote are skipped.

Otherwise, edit `hop.yaml` by hand to list your repos.

## Quick tour

```sh
hop                       # fzf picker over all repos; prints selection's path
hop outbox where          # resolve "outbox" to its absolute path
hop ls                    # list every repo (name + path)
h outbox                  # cd into outbox (single-letter alias + bare-name dispatch)
hop outbox git status     # tool-form: run `git status` inside outbox; cwd unchanged
hop dotfiles cursor       # tool-form: open dotfiles in cursor
hop outbox -R git status  # canonical user-facing form (explicit; equivalent to the tool-form above)
hop -R outbox git status  # binary-direct form (works without the shim, e.g. in scripts)
hop clone outbox          # registry-driven: clone outbox if it isn't already on disk
hop clone git@github.com:foo/bar.git
                          # ad-hoc: clone the URL, register it in hop.yaml, cd into it
```

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

## Reference

- `hop --help` — full subcommand listing
- [`docs/specs/cli-surface.md`](docs/specs/cli-surface.md) — canonical CLI contract
- [`docs/specs/config-resolution.md`](docs/specs/config-resolution.md) — search order and `hop.yaml` schema
- [`docs/specs/architecture.md`](docs/specs/architecture.md) — package layout
- [`docs/specs/build-and-release.md`](docs/specs/build-and-release.md) — build and release plan
