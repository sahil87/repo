# hop

A small Go CLI for locating, opening, and operating on git repositories listed in `hop.yaml`. The dominant use case is navigation: `hop <name>` prints a path; `h <name>` (the single-letter alias) `cd`s your shell into that repo via the bare-name dispatcher; `hop -C <name> <cmd>...` runs a command inside it without changing your cwd.

## Install

### Homebrew (macOS and Linux)

```sh
brew install sahil87/tap/hop
```

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

For the shell integration (bare-name `cd`, the `h` and `hi` aliases, and zsh tab completion):

```sh
eval "$(hop shell-init zsh)"
```

Add that line to your `~/.zshrc`.

## First run

Bootstrap a starter `hop.yaml`:

```sh
hop config init
hop config where   # show where it lives
```

Edit it to list your repos. By default the file lives at `$XDG_CONFIG_HOME/hop/hop.yaml` (or `~/.config/hop/hop.yaml`). Set `$HOP_CONFIG` in your shell rc to point at a tracked location (Dropbox, a git-tracked dotfile, etc.) so the config moves with you.

## Quick tour

```sh
hop                       # fzf picker over all repos; prints selection's path
hop where outbox          # resolve "outbox" to its absolute path
hop ls                    # list every repo (name + path)
h outbox                  # cd into outbox (single-letter alias + bare-name dispatch)
hop -C outbox git status  # run `git status` inside outbox; cwd unchanged
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
