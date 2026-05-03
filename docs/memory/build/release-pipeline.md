# Release Pipeline

How `repo` cuts a release. Hand-rolled GitHub Actions workflow mirroring `~/code/sahil87/run-kit`'s shape, with a tag-driven version source (no `VERSION` file — `repo` is single-binary, so the git tag itself is the source of truth).

## Trigger

A release is triggered by pushing a tag matching `v*` to the origin remote. In practice this happens via:

```
just release [patch|minor|major]   # default: patch
```

which delegates to `scripts/release.sh`. That script computes the next tag, creates it locally, and pushes it. The push fires `.github/workflows/release.yml`, and CI takes over.

There are no other release triggers — no `workflow_dispatch`, no branch-push, no schedule.

## `scripts/release.sh`

Computes the next semver tag from the current latest tag and pushes it. It does **not** modify any tracked files (no `VERSION` file write, no commit step) — this is a deliberate divergence from run-kit, which uses a `VERSION` file because it's a multi-binary monorepo.

Behavior:

- Accepts exactly one of `patch | minor | major` (or `-h`/`--help`). Multiple bump types or unknown values exit 1 with a usage message. Bare invocation prints usage and exits 0.
- Pre-flight: rejects dirty working tree (`git status --porcelain` non-empty) and detached HEAD. Both exit 1.
- Computes current tag via `git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0"`. The `v0.0.0` fallback handles the first-release case (no tags yet) — `release.sh patch` produces `v0.0.1`, `minor` produces `v0.1.0`, `major` produces `v1.0.0`.
- Bump arithmetic:

  ```sh
  case "$bump_type" in
    patch) patch=$((patch + 1)) ;;
    minor) minor=$((minor + 1)); patch=0 ;;
    major) major=$((major + 1)); minor=0; patch=0 ;;
  esac
  ```

- Creates the tag with `git tag "$new_tag"` and pushes with `git push origin "$new_tag"`.
- No `--force` flag, no main-branch check — releases can happen from any branch (mirrors run-kit; intentional flexibility for hotfix flows).

## Workflow steps (`.github/workflows/release.yml`)

Single job (`release`) on `ubuntu-latest`, `permissions: contents: write` (no other scopes), six steps:

1. **Checkout** with `fetch-depth: 0` — needed for the previous-tag-base computation.
2. **Setup Go** with `go-version-file: src/go.mod` — keeps the CI Go version in lockstep with `go.mod`.
3. **Extract version from tag** — sets two outputs from `${GITHUB_REF#refs/tags/}`:
   - `tag` (with `v` prefix, e.g. `v0.0.1`) — used for ldflags injection.
   - `version` (without prefix, e.g. `0.0.1`) — used for `sed` substitution into the formula.
4. **Cross-compile** — loops over `darwin/arm64 darwin/amd64 linux/arm64 linux/amd64`, building with `CGO_ENABLED=0` and `-ldflags "-X main.version=${tag}"`. Each binary is tarred via `tar -czf "dist/${output}.tar.gz" -C "dist/${output}" repo` — archives contain only the `repo` binary (no LICENSE/README inside).
5. **Determine release notes base tag** — minor-aware logic: if the patch component is `0` (minor bump), `base_tag` is set to the earliest tag matching `v{major}.{minor-1}.*` (sorted by `version:refname`, head -1), so v0.2.0's notes span the entire 0.1.x series. For patch bumps and major bumps, `base_tag` is left unset (default behavior: compare against the immediate previous tag).
6. **Create GitHub Release** via `softprops/action-gh-release` with `files: dist/*.tar.gz`, `generate_release_notes: true`, and `previous_tag: ${{ steps.release-base.outputs.base_tag }}`.
7. **Update Homebrew tap** — see Formula template below.

## Action SHAs

All third-party actions are pinned to commit SHAs with `# v<N>` comments:

| Action | SHA | Tag |
|---|---|---|
| `actions/checkout` | `34e114876b0b11c390a56381ad16ebd13914f8d5` | v4 |
| `actions/setup-go` | `40f1582b2485089dde7abd97c1529aa768e1baff` | v5 |
| `softprops/action-gh-release` | `153bb8e04406b158c6c84fc1615b65b24149a1fe` | v2 |

**Policy**: SHAs match `~/code/sahil87/run-kit/.github/workflows/release.yml` at apply time. Deviations need explicit justification — the lockstep keeps both repos updateable via a single-source diff if a third-party action ever needs bumping.

## Formula template (`.github/formula-template.rb`)

A syntactically valid Homebrew Formula Ruby file with five placeholders that the workflow's tap-update step replaces via `sed`:

| Placeholder | Replacement |
|---|---|
| `VERSION_PLACEHOLDER` | bare version (no `v` prefix), e.g. `0.0.1` |
| `SHA_DARWIN_ARM64` | `sha256sum dist/repo-darwin-arm64.tar.gz` |
| `SHA_DARWIN_AMD64` | `sha256sum dist/repo-darwin-amd64.tar.gz` |
| `SHA_LINUX_ARM64`  | `sha256sum dist/repo-linux-arm64.tar.gz` |
| `SHA_LINUX_AMD64`  | `sha256sum dist/repo-linux-amd64.tar.gz` |

The substituted file is written to `Formula/repo.rb` in a clone of `sahil87/homebrew-tap`. The clone uses `https://x-access-token:${HOMEBREW_TAP_TOKEN}@github.com/sahil87/homebrew-tap.git`. The commit is authored as `github-actions[bot]` with message `repo v<version>` and pushed directly to the tap's default branch (no PR).

The published formula's structure:

- `class Repo < Formula` opener.
- `desc`, `homepage`, `version`, `license "MIT"` (informational — brew does not enforce).
- `on_macos` block with nested `on_arm` / `on_intel` blocks declaring `url` and `sha256` for the two darwin tar.gz files.
- `on_linux` block with the same shape for the two linux tar.gz files.
- URLs follow `https://github.com/sahil87/repo/releases/download/v#{version}/repo-{os}-{arch}.tar.gz` — note the `v` prefix is re-added in the URL, so `version "VERSION_PLACEHOLDER"` stores the bare form.
- `install` block: `bin.install "repo"`.
- `test` block: `assert_match version.to_s, shell_output("#{bin}/repo --version")`.

## Setup checklist

One-time setup per repo:

1. **Provision `HOMEBREW_TAP_TOKEN`** as a GitHub repository secret on `sahil87/repo`. It must be a fine-grained Personal Access Token with `Contents: write` permission scoped to `sahil87/homebrew-tap`. This step is manual (GitHub UI) and cannot be automated.
2. **Verify the tap repo** — `sahil87/homebrew-tap` must exist and the bot must have push access via the token. The `Formula/` directory already exists (it hosts `Formula/rk.rb` for run-kit).

## Release-day runbook

1. `just release [patch|minor|major]` (default `patch`) on a clean working tree, on a branch.
2. Watch the workflow at `https://github.com/sahil87/repo/actions`.
3. Verify the GitHub Release page shows four `repo-{os}-{arch}.tar.gz` assets (no separate `checksums.txt` is published).
4. Verify `sahil87/homebrew-tap` got a new commit adding/updating `Formula/repo.rb` authored by `github-actions[bot]`.
5. Smoke test in a clean shell: `brew install sahil87/tap/repo && repo --version` should print `repo version v<version>`.

If `HOMEBREW_TAP_TOKEN` is missing or invalid, the tap-update step fails on `git clone` with an auth error. The GitHub Release (created in the prior step) remains published — re-running typically means provisioning the secret and tagging again (e.g., `v0.0.2`).

## Out of scope

These are policy decisions, not deferrals:

- **Code signing / notarization** — binaries ship unsigned. macOS users see a Gatekeeper warning on first run for direct downloads; brew installs typically don't trip it as hard. An Apple Developer account ($99/yr) is not justified for personal-tooling CLIs.
- **Linux native packaging** (`.deb`, `.rpm`, custom apt/dnf repos) — Linux users install via brew-on-Linux or direct tar.gz download.
- **Prerelease tags** (`v0.0.1-rc.1`) — `release.sh` accepts only `patch|minor|major`. Adding RC support is ~30 LOC across the script and the workflow if/when iterative pipeline testing becomes valuable.
- **A `VERSION` file** — git tag is the single source of truth (single-binary repo; run-kit's multi-binary `VERSION`-file rationale doesn't apply).
- **Goreleaser** — the minor-aware base-tag logic for release notes is awkward in goreleaser (requires disabling its changelog and using post-hoc `gh release edit`); cleaner here via `softprops/action-gh-release`'s `previous_tag` parameter. Switching back is a one-evening rewrite if the repo grows multiple binaries or wants signing/Docker/Snap.

## Cross-references

- `docs/specs/build-and-release.md` — pre-implementation design intent and behavioral scenarios.
- `docs/memory/build/local.md` — `just build` / `just install` for local development.
- `docs/memory/cli/subcommands.md` — the binary being released, including its `--version` surface.
