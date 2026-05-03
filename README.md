# repo

A small Go CLI for locating, opening, and cloning repositories listed in `repos.yaml`. Drives subcommands like `repo <name>` (print path), `repo code <name>` (open VSCode), `repo open <name>` (file manager), `repo cd <name>` (shell function), `repo clone --all`, and `repo ls`.

## Install

### Homebrew (macOS and Linux)

```sh
brew install sahil87/tap/repo
```

### GitHub Release tarball

Download the appropriate tar.gz for your platform from the [latest release](https://github.com/sahil87/repo/releases/latest) — assets are named `repo-{os}-{arch}.tar.gz` (where `{os}` is `darwin` or `linux` and `{arch}` is `arm64` or `amd64`). Extract and place the `repo` binary on your `$PATH`.

### From source

```sh
just install
```

Builds the binary and copies it to `~/.local/bin/repo`. Make sure that directory is on your `$PATH`.

## Shell integration

For the shell-function form of `repo cd` (and zsh tab completion):

```sh
eval "$(repo shell-init zsh)"
```

Add that line to your `~/.zshrc`.

## First run

Bootstrap a starter `repos.yaml`:

```sh
repo config init
repo config path   # show where it lives
```

Edit it to list your repos. By default the file lives at `$XDG_CONFIG_HOME/repo/repos.yaml` (or `~/.config/repo/repos.yaml`). Set `$REPOS_YAML` in your shell rc to point at a tracked location (Dropbox, a git-tracked dotfile, etc.) so the config moves with you.

## Reference

- `repo --help` — full subcommand listing
- [`docs/specs/cli-surface.md`](docs/specs/cli-surface.md) — canonical CLI contract
- [`docs/specs/config-resolution.md`](docs/specs/config-resolution.md) — search order and `repos.yaml` schema
- [`docs/specs/architecture.md`](docs/specs/architecture.md) — package layout
- [`docs/specs/build-and-release.md`](docs/specs/build-and-release.md) — build and release plan
