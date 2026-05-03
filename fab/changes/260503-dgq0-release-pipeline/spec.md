# Spec: Cross-platform release pipeline

**Change**: 260503-dgq0-release-pipeline
**Created**: 2026-05-03
**Affected memory**: `docs/memory/build/release-pipeline.md` (new), `docs/memory/build/local.md` (modify)

## Non-Goals

- macOS code signing and notarization — explicitly out of scope; binaries ship unsigned and trip Gatekeeper on first run. Documented workaround in README.
- Linux native packaging (`.deb`, `.rpm`, custom apt/dnf repos) — Linux users install via brew-on-Linux or direct tar.gz download.
- Prerelease tags (`v0.0.1-rc.1`, etc.) — `release.sh` accepts only `patch|minor|major`.
- A `VERSION` file at the repo root — git tags are the version source of truth (single-binary repo, run-kit's multi-binary `VERSION`-file rationale doesn't apply).
- Goreleaser — rejected in favor of mirroring run-kit's hand-rolled workflow (see Design Decisions).
- Modifying `src/cmd/repo/main.go` — the parent change already wired `var version = "dev"` and `rootCmd.Version = version`; cobra auto-wires `--version`, `-v`, and the `version` subcommand.
- Modifying `scripts/build.sh` — already uses `git describe --tags --always`, which is the desired behavior.
- Adding a LICENSE file to the repo — formula's `license "MIT"` line is informational only; brew does not enforce it. Adding LICENSE is a separate concern.
- Updating dependents that reference this repo (e.g., the `repos.yaml` starter that ships embedded in the binary) — its content is unchanged by this change.

## Build & Release: Version Source

### Requirement: Tag-driven version source

The release pipeline SHALL use the git tag as the single source of truth for the binary's version string. There SHALL NOT be a `VERSION` file at the repo root.

#### Scenario: Local build on a tagged commit

- **GIVEN** the working tree is at a commit pointed to by tag `v0.0.1`
- **WHEN** `just build` runs
- **THEN** `./bin/repo --version` prints `repo version v0.0.1`

#### Scenario: Local build on a post-tag commit

- **GIVEN** the working tree is 2 commits past tag `v0.0.1`
- **WHEN** `just build` runs
- **THEN** `./bin/repo --version` prints `repo version v0.0.1-2-g<short-sha>`

#### Scenario: Local build with no tags

- **GIVEN** the working tree has no tags
- **WHEN** `just build` runs
- **THEN** `./bin/repo --version` prints `repo version <short-sha>` (from `git describe --always`)

#### Scenario: Local build with no git history

- **GIVEN** a source tarball without `.git/`
- **WHEN** `just build` runs
- **THEN** `./bin/repo --version` prints `repo version dev`

#### Scenario: Build with no ldflags injection

- **GIVEN** a build invoked directly via `go build ./src/cmd/repo` (no `-ldflags`)
- **WHEN** the resulting binary is invoked
- **THEN** `./repo --version` prints `repo version dev` (the `var version = "dev"` default in `src/cmd/repo/main.go`)

### Requirement: Workflow extracts version from pushed tag

The release workflow SHALL read the version from `${GITHUB_REF#refs/tags/}`. It SHALL inject the version into the Go binary via `-ldflags "-X main.version=<tag>"`. The injected string SHALL retain the leading `v` prefix.

#### Scenario: Tag push triggers release

- **GIVEN** tag `v0.0.1` is pushed to `origin`
- **WHEN** the release workflow runs
- **THEN** the workflow extracts `tag=v0.0.1` and `version=0.0.1`
- **AND** each cross-compiled binary is built with `-ldflags "-X main.version=v0.0.1"`
- **AND** invoking any released binary prints `repo version v0.0.1`

### Requirement: Version printing surface

The binary SHALL expose its version string via `repo --version` and `repo -v`. Both SHALL produce identical output. This requirement is satisfied by the parent change's existing cobra wiring; no additional code changes apply in this change.

> Note: cobra also auto-wires a `repo version` subcommand from `rootCmd.Version`, but at runtime it is shadowed by the parent change's positional `repo <name>` handler, so `repo version` triggers an fzf lookup for a repo named "version" rather than a version print. The flag forms (`--version`, `-v`) are the documented public surface; `repo version` is not.

#### Scenario: Flag forms produce identical output

- **GIVEN** a binary built with `main.version=v0.0.1`
- **WHEN** the user runs `repo --version` or `repo -v` (separate invocations)
- **THEN** each invocation prints `repo version v0.0.1`
- **AND** each exits 0

## Build & Release: Release Script

### Requirement: `scripts/release.sh` accepts a bump-type argument

`scripts/release.sh` SHALL accept exactly one positional argument: `patch`, `minor`, or `major`. It SHALL exit 1 with a usage message for unknown values or multiple bump-type arguments. Bare invocation (no arguments) SHALL print usage and exit 0 (informational, mirroring run-kit) — this is documented as a separate scenario below.

#### Scenario: Valid bump type

- **GIVEN** a clean working tree on a branch, with no existing tags
- **WHEN** `scripts/release.sh patch` runs
- **THEN** the script computes `current=v0.0.0` (fallback for tagless repo)
- **AND** computes `new=v0.0.1`
- **AND** creates tag `v0.0.1` locally
- **AND** pushes the tag to `origin`
- **AND** prints `Done — v0.0.1 pushed. CI will cross-compile, create the GitHub Release, and update the Homebrew tap.`
- **AND** exits 0

#### Scenario: Unknown bump type

- **GIVEN** any working tree state
- **WHEN** `scripts/release.sh foo` runs
- **THEN** the script prints a usage message to stderr
- **AND** exits 1

#### Scenario: No argument

- **GIVEN** any working tree state
- **WHEN** `scripts/release.sh` runs with no arguments
- **THEN** the script prints a usage message
- **AND** exits 0 (printing usage on bare invocation is informational, not error — matches run-kit)

#### Scenario: Multiple bump-type arguments

- **GIVEN** any working tree state
- **WHEN** `scripts/release.sh patch minor` runs
- **THEN** the script prints an error message naming the conflict
- **AND** exits 1

### Requirement: Release script pre-flight checks

`scripts/release.sh` SHALL refuse to proceed when (a) the working tree is dirty, or (b) HEAD is detached. Both checks run before any tag computation or push.

#### Scenario: Dirty working tree

- **GIVEN** a branch with uncommitted changes (`git status --porcelain` returns non-empty)
- **WHEN** `scripts/release.sh patch` runs
- **THEN** the script prints `ERROR: Working tree not clean. Commit or stash changes first.`
- **AND** exits 1
- **AND** does not create or push any tag

#### Scenario: Detached HEAD

- **GIVEN** a checkout in detached-HEAD state (`git branch --show-current` returns empty)
- **WHEN** `scripts/release.sh patch` runs
- **THEN** the script prints `ERROR: Not on a branch (detached HEAD). Check out a branch before releasing.`
- **AND** exits 1

### Requirement: Bump arithmetic

The release script SHALL compute the next version by parsing the current `major.minor.patch` (after stripping any leading `v`) and incrementing per the bump type:

- `patch`: `major.minor.(patch+1)`
- `minor`: `major.(minor+1).0`
- `major`: `(major+1).0.0`

#### Scenario: Patch bump

- **GIVEN** the latest tag is `v0.1.5`
- **WHEN** `scripts/release.sh patch` runs
- **THEN** the new tag is `v0.1.6`

#### Scenario: Minor bump

- **GIVEN** the latest tag is `v0.1.5`
- **WHEN** `scripts/release.sh minor` runs
- **THEN** the new tag is `v0.2.0`

#### Scenario: Major bump

- **GIVEN** the latest tag is `v0.1.5`
- **WHEN** `scripts/release.sh major` runs
- **THEN** the new tag is `v1.0.0`

#### Scenario: First release (no tags)

- **GIVEN** no tags exist on the repo
- **WHEN** `scripts/release.sh patch` runs
- **THEN** `git describe --tags --abbrev=0` exits non-zero
- **AND** the script falls back to `current=v0.0.0`
- **AND** the new tag is `v0.0.1`

### Requirement: Release script does not modify tracked files

`scripts/release.sh` SHALL NOT add, modify, or delete any tracked files. It SHALL NOT create commits. The only repository state changes it makes are creating a local annotated/lightweight tag and pushing it.

#### Scenario: Run on clean working tree

- **GIVEN** a clean working tree on a branch
- **WHEN** `scripts/release.sh patch` runs to completion
- **THEN** `git status --porcelain` is still empty
- **AND** `git log --oneline -1` shows the same commit as before
- **AND** the only state change is one new tag plus the corresponding remote ref

## Build & Release: Justfile

### Requirement: `release` recipe

The `justfile` SHALL include a `release` recipe that delegates to `scripts/release.sh`. The recipe SHALL accept a single argument with a default of `patch`.

#### Scenario: Default invocation

- **GIVEN** the justfile contains the `release` recipe
- **WHEN** the user runs `just release` (no argument)
- **THEN** the recipe invokes `scripts/release.sh patch`

#### Scenario: Explicit bump

- **GIVEN** the justfile contains the `release` recipe
- **WHEN** the user runs `just release minor`
- **THEN** the recipe invokes `scripts/release.sh minor`

### Requirement: Existing recipes preserved

The `release` recipe SHALL be added without modifying or removing the existing `default`, `build`, `install`, or `test` recipes (created by the parent change).

#### Scenario: Existing recipes intact

- **GIVEN** the justfile after this change
- **WHEN** the user runs `just build`, `just install`, or `just test`
- **THEN** each recipe behaves identically to the parent change's behavior
- **AND** `just --list` shows all five recipes (`default`, `build`, `install`, `test`, `release`)

## Build & Release: GitHub Actions Workflow

### Requirement: Workflow trigger

`.github/workflows/release.yml` SHALL trigger only on tag pushes matching the pattern `v*`. It SHALL NOT trigger on pushes to branches, pull requests, schedules, or manual dispatch.

#### Scenario: Tag push triggers workflow

- **GIVEN** the workflow is committed to the default branch
- **WHEN** `git push origin v0.0.1` runs
- **THEN** the `release` workflow run starts on the GitHub-hosted ubuntu-latest runner

#### Scenario: Branch push does not trigger workflow

- **GIVEN** the workflow is committed to the default branch
- **WHEN** `git push origin main` runs (without a tag)
- **THEN** no workflow run is started

### Requirement: Workflow permissions

The workflow SHALL declare `permissions: contents: write` at the job level (or workflow level). It SHALL NOT request any other permission scopes.

#### Scenario: Permissions declared

- **GIVEN** the workflow file
- **WHEN** parsed
- **THEN** the `permissions:` block contains exactly `contents: write` (no `pull-requests`, no `packages`, no `id-token`, etc.)

### Requirement: Setup steps

The workflow SHALL check out the repository with `fetch-depth: 0` and SHALL set up Go using `actions/setup-go@v5` with `go-version-file: src/go.mod`. Both action references SHALL be pinned to commit SHAs (with version comments).

#### Scenario: Full history available

- **GIVEN** a workflow run on tag push
- **WHEN** the checkout step completes
- **THEN** all tags and commit history are available (needed for the previous-tag-base computation)

#### Scenario: Go version source

- **GIVEN** `src/go.mod` declares `go 1.22` (or whatever the parent change set)
- **WHEN** the setup-go step runs
- **THEN** the runner has the same Go version installed as declared in `src/go.mod`

### Requirement: Cross-compile matrix

The workflow SHALL cross-compile the binary for exactly four `GOOS/GOARCH` targets: `darwin/arm64`, `darwin/amd64`, `linux/arm64`, `linux/amd64`. Each build SHALL set `CGO_ENABLED=0` and inject the version via ldflags.

#### Scenario: Four binaries produced

- **GIVEN** a workflow run on tag `v0.0.1`
- **WHEN** the cross-compile step completes
- **THEN** four directories exist: `dist/repo-darwin-arm64/`, `dist/repo-darwin-amd64/`, `dist/repo-linux-arm64/`, `dist/repo-linux-amd64/`
- **AND** each contains a single executable file named `repo`
- **AND** four tar.gz archives exist: `dist/repo-darwin-arm64.tar.gz`, `dist/repo-darwin-amd64.tar.gz`, `dist/repo-linux-arm64.tar.gz`, `dist/repo-linux-amd64.tar.gz`
- **AND** each tar.gz contains exactly one entry: the `repo` binary

#### Scenario: Version injected via ldflags

- **GIVEN** a workflow run on tag `v0.0.1`
- **WHEN** any of the four binaries is invoked
- **THEN** `repo --version` prints `repo version v0.0.1`

### Requirement: Previous-tag base computation

For minor-version bumps (where the patch component of the current tag is `0`), the workflow SHALL compute a `base_tag` set to the earliest tag matching `v{major}.{minor-1}.*`, sorted by `version:refname`. For patch bumps (patch != 0) and major bumps, the workflow SHALL leave `base_tag` empty (default GitHub behavior: compare to the immediate previous tag).

#### Scenario: Patch bump uses default base

- **GIVEN** a tag push of `v0.1.3` and a previous tag `v0.1.2` exists
- **WHEN** the workflow's previous-tag-base step runs
- **THEN** `base_tag` output is empty
- **AND** the GitHub Release notes compare `v0.1.3` against `v0.1.2`

#### Scenario: Minor bump uses previous-minor's first tag

- **GIVEN** a tag push of `v0.2.0` and prior tags `v0.1.0`, `v0.1.1`, `v0.1.2`, `v0.1.3` exist
- **WHEN** the workflow's previous-tag-base step runs
- **THEN** `base_tag` output is `v0.1.0`
- **AND** the GitHub Release notes for `v0.2.0` span the entire `0.1.x` series (commits/PRs from `v0.1.0..v0.2.0`)

#### Scenario: Minor bump with no prior minor

- **GIVEN** a tag push of `v0.2.0` and no `v0.1.*` tags exist
- **WHEN** the workflow's previous-tag-base step runs
- **THEN** `base_tag` output is empty
- **AND** the workflow proceeds without error

#### Scenario: First release (v0.0.1)

- **GIVEN** a tag push of `v0.0.1` with no prior tags
- **WHEN** the workflow runs
- **THEN** `base_tag` output is empty (patch=1, not zero, so the minor-bump path is not taken)
- **AND** GitHub's auto-generate falls back to "all commits since repository creation"

### Requirement: GitHub Release publication

The workflow SHALL publish a GitHub Release using `softprops/action-gh-release@v2` (pinned to a commit SHA). It SHALL upload all four `dist/*.tar.gz` files. It SHALL set `generate_release_notes: true` and pass the computed `previous_tag` parameter.

#### Scenario: Release page populated

- **GIVEN** a successful workflow run on tag `v0.0.1`
- **WHEN** the user navigates to `https://github.com/sahil87/repo/releases/tag/v0.0.1`
- **THEN** the page shows four assets (the four tar.gz files)
- **AND** the release notes section contains GitHub's auto-generated PR-based notes
- **AND** there is no `checksums.txt` asset (checksums are inline-only, used for the formula update)

### Requirement: Homebrew tap update

The workflow SHALL update `sahil87/homebrew-tap` by:

1. Computing SHA256 checksums for each of the four tar.gz archives via `sha256sum`.
2. Cloning the tap repository using `https://x-access-token:${HOMEBREW_TAP_TOKEN}@github.com/sahil87/homebrew-tap.git`.
3. Running `sed` against `.github/formula-template.rb` (in the source repo) to substitute the placeholders `VERSION_PLACEHOLDER`, `SHA_DARWIN_ARM64`, `SHA_DARWIN_AMD64`, `SHA_LINUX_ARM64`, `SHA_LINUX_AMD64`.
4. Writing the result to `Formula/repo.rb` in the cloned tap.
5. Committing the change as `github-actions[bot]` with the message `repo v<version>`.
6. Pushing directly to the tap's default branch (no pull request).

The `<version>` substituted into the formula's `version "VERSION_PLACEHOLDER"` field SHALL be the bare version (no `v` prefix), since the formula's URL string `releases/download/v#{version}/...` already adds the `v` back.

#### Scenario: Tap commit lands

- **GIVEN** a successful workflow run on tag `v0.0.1` and a valid `HOMEBREW_TAP_TOKEN` secret
- **WHEN** the tap-update step completes
- **THEN** `sahil87/homebrew-tap` has a new commit on its default branch
- **AND** the commit message is `repo v0.0.1`
- **AND** the commit author is `github-actions[bot]`
- **AND** the commit modifies (or creates) `Formula/repo.rb`

#### Scenario: Formula has correct content

- **GIVEN** the tap commit from the previous scenario
- **WHEN** the user inspects `Formula/repo.rb`
- **THEN** the file contains `version "0.0.1"` (no `v` prefix)
- **AND** four URL lines reference `releases/download/v0.0.1/repo-{darwin,linux}-{arm64,amd64}.tar.gz`
- **AND** four `sha256` lines match the SHAs of the four tar.gz files in the release

#### Scenario: Tap update fails on missing token

- **GIVEN** the workflow runs but `HOMEBREW_TAP_TOKEN` is not provisioned
- **WHEN** the tap-update step runs
- **THEN** the `git clone` step fails with an authentication error
- **AND** the workflow run fails
- **AND** the GitHub Release (from the previous step) remains published — only the tap update is missing
- **AND** the user can re-run the workflow after provisioning the secret, but typically just provisions and re-tags (e.g., `v0.0.2`)

### Requirement: Action SHA pinning

All third-party action references (`actions/checkout`, `actions/setup-go`, `softprops/action-gh-release`) SHALL be pinned to specific commit SHAs (full 40-character SHAs) with version comments. The SHAs SHOULD match those used in `~/code/sahil87/run-kit/.github/workflows/release.yml` at apply time, so the two workflows stay in lockstep.

#### Scenario: All actions pinned

- **GIVEN** the workflow file
- **WHEN** parsed
- **THEN** every `uses:` line outside the `actions/` namespace is pinned to a 40-character SHA
- **AND** every `uses:` line in the `actions/` namespace is also pinned to a SHA (not `@v4`, `@v5`, etc.)
- **AND** each pinned SHA is followed by a comment naming the version (e.g., `# v4`)

## Build & Release: Formula Template

### Requirement: Formula template file

`.github/formula-template.rb` SHALL be a syntactically valid Ruby file defining a `Formula` subclass. It SHALL contain placeholders that the workflow substitutes at release time.

#### Scenario: File exists and parses

- **GIVEN** the apply stage has completed
- **WHEN** `ruby -c .github/formula-template.rb` runs
- **THEN** the command exits 0 (syntax check passes)

### Requirement: Formula structure mirrors run-kit

The formula template SHALL declare:

- A `class Repo < Formula` opener.
- A `desc` line: `"Locate, open, list, and clone repos from repos.yaml"`.
- A `homepage` line: `"https://github.com/sahil87/repo"`.
- A `version "VERSION_PLACEHOLDER"` line.
- A `license "MIT"` line (informational).
- An `on_macos` block with nested `on_arm` and `on_intel` blocks, each declaring `url` and `sha256` lines for the appropriate darwin tar.gz.
- An `on_linux` block with the same structure for the linux tar.gz.
- An `install` block: `bin.install "repo"`.
- A `test` block: `assert_match version.to_s, shell_output("#{bin}/repo --version")`.

URLs SHALL follow the pattern `https://github.com/sahil87/repo/releases/download/v#{version}/repo-{os}-{arch}.tar.gz`.

#### Scenario: Substituted formula installs successfully

- **GIVEN** the formula template after the workflow's `sed` substitution for `v0.0.1` (with all four real SHAs)
- **WHEN** a user runs `brew install sahil87/tap/repo` on a supported platform
- **THEN** brew downloads the appropriate `repo-{os}-{arch}.tar.gz`
- **AND** verifies the SHA256
- **AND** extracts and installs the `repo` binary to `$(brew --prefix)/bin/repo`
- **AND** subsequent `repo --version` prints `repo version v0.0.1`

#### Scenario: Test block executes successfully

- **GIVEN** an installed formula
- **WHEN** the user runs `brew test sahil87/tap/repo`
- **THEN** brew invokes `repo --version`
- **AND** the output contains the version string declared in the formula

## Build & Release: README Distribution Section

### Requirement: README install instructions ordered by primacy

`README.md` SHALL list installation channels in this order: (1) Homebrew (macOS and Linux), (2) GitHub Release tarball, (3) From source (`git clone` + `just install`). Older language describing only "from source" install SHALL be removed or relocated.

#### Scenario: Brew listed first

- **GIVEN** the README after this change
- **WHEN** a reader scans the install section top-to-bottom
- **THEN** the first install command shown is `brew install sahil87/tap/repo`
- **AND** the from-source instructions appear after both brew and tarball

#### Scenario: Tarball download URL pattern

- **GIVEN** the README's tarball-download instructions
- **WHEN** a reader follows them
- **THEN** the URL pattern `https://github.com/sahil87/repo/releases/latest` is provided
- **AND** the asset naming pattern `repo-{os}-{arch}.tar.gz` is documented

## Build & Release: Spec File Update

### Requirement: `docs/specs/build-and-release.md` rewrite

`docs/specs/build-and-release.md` SHALL be rewritten in this change to reflect the run-kit-mirroring decision. The rewrite SHALL:

1. Update the header's "Scope note" to reference the run-kit-mirroring approach (drop `.goreleaser.yaml` mention).
2. Drop the Cobra `repo version` subcommand row from the **Version Reporting** table — keep only `repo --version` and `repo -v`. Note: the `version` subcommand still exists at runtime (cobra-default), but the spec no longer documents it as a public surface.
3. Replace the entire **`.goreleaser.yaml` (intent)** block with a description of the hand-rolled workflow.
4. Replace the **`.github/workflows/release.yml` (intent)** block with the new tag-driven workflow content.
5. Update the **Initial Release** section to reflect the tag-driven model (no `VERSION` file initialization).
6. Update the **Behavioral Scenarios > Release pipeline** scenarios: replace `git tag -a v0.0.1 -m "Initial release" && git push origin v0.0.1` with `just release patch`. Replace `4 binary archives + checksums.txt` with `4 tar.gz archives` (no separate checksums file).
7. Flip **Design Decision #1** from "goreleaser over hand-rolled bash" to "Hand-rolled workflow mirroring run-kit, with tag-driven version source."
8. Update **Design Decision #3** to reflect that local builds use `git describe` and release builds use the pushed tag.
9. Strengthen **Design Decision #7** from "deferred" to "not in scope" for code signing.

#### Scenario: Spec is internally consistent post-rewrite

- **GIVEN** the rewritten `docs/specs/build-and-release.md`
- **WHEN** a reader follows it end-to-end
- **THEN** there are no references to `.goreleaser.yaml`, `goreleaser-action`, `goreleaser check`, or `VERSION` file
- **AND** the workflow code blocks reference `softprops/action-gh-release` and the cross-compile loop
- **AND** Design Decisions #1, #3, and #7 reflect the current decisions

## Build & Release: Out-of-Repo State

### Requirement: `HOMEBREW_TAP_TOKEN` provisioning is manual

`HOMEBREW_TAP_TOKEN` SHALL be provisioned by the user as a GitHub repository secret on `sahil87/repo`, via GitHub's UI. It SHALL be a fine-grained Personal Access Token with `Contents: write` permission scoped to `sahil87/homebrew-tap`. This step SHALL NOT be automated.

#### Scenario: Token provisioned before first release

- **GIVEN** a fine-grained PAT with `Contents: write` on `sahil87/homebrew-tap`
- **WHEN** the user adds it as `HOMEBREW_TAP_TOKEN` in `sahil87/repo` → Settings → Secrets and variables → Actions
- **THEN** the workflow can authenticate to push to the tap on subsequent tag pushes

### Requirement: Tap repo prerequisites

`sahil87/homebrew-tap` SHALL exist and contain a `Formula/` directory (creating it on first push if absent is acceptable, but the directory naming is fixed). The bot user `github-actions[bot]` SHALL have push access via the provisioned token.

#### Scenario: Tap exists with Formula directory

- **GIVEN** `sahil87/homebrew-tap` already hosts `Formula/rk.rb` (run-kit's formula)
- **WHEN** the workflow's tap-update step runs for `repo`
- **THEN** the new `Formula/repo.rb` is added alongside `Formula/rk.rb`
- **AND** `Formula/rk.rb` is unchanged

## Design Decisions

1. **Hand-rolled workflow mirroring run-kit, with tag-driven version source**: We mirror run-kit's release workflow shape (cross-compile loop, `softprops/action-gh-release`, hand-rolled tap update via formula template + `sed`) but use git tags as the version source instead of run-kit's `VERSION` file.
   - *Why*: Mirroring run-kit gives one mental model across both projects, one formula-template idiom, one release-script UX (`just release patch`). The minor-aware base-tag logic for release notes is meaningfully cleaner in the hand-rolled approach (one `previous_tag:` parameter on `softprops/action-gh-release`) than in goreleaser (which requires disabling its changelog and using post-hoc `gh release edit`). The divergence on version source — git tags instead of `VERSION` file — is justified because `repo` is single-binary, while run-kit's `VERSION`-file rationale applies to its multi-binary monorepo (frontend + backend disambiguation).
   - *Rejected*: Goreleaser. ~25 LOC smaller config and free Homebrew tap update via the `brews:` block, but the tap advantage doesn't compound for a single-binary CLI, and the release-notes customization is awkward. Goreleaser pays back if/when this repo grows multiple binaries or wants signing/Docker/Snap; switching is a one-evening rewrite then.
   - *Rejected*: Manual releases (build locally, upload by hand). Doesn't scale; introduces drift between tagged commits and published artifacts.

2. **Tag push is the single trigger**: No manual workflow dispatch, no GUI clicks. `git push origin <tag>` (via `just release patch`) is the entire release-day action.
   - *Why*: One source of truth (the tag), one path to a release. Reduces operator error.
   - *Rejected*: `workflow_dispatch` button. Adds a manual GUI step; defeats the point of a tag-driven pipeline.

3. **`scripts/release.sh` does not commit**: It only creates and pushes a tag. No `VERSION` file to bump, no commit to the working tree.
   - *Why*: Tags are stable git refs; no commit churn means the release script is idempotent on the file tree. If the tag push fails, nothing local needs reverting.
   - *Rejected*: Bumping a `VERSION` file (run-kit's approach). Justified there by their multi-binary structure; not justified here.

4. **First-release fallback is `v0.0.0` baseline**: When no tags exist, `git describe --tags --abbrev=0` exits non-zero; the release script falls back to `v0.0.0` as the synthetic baseline so `release.sh patch` produces `v0.0.1`, `release.sh minor` produces `v0.1.0`, and `release.sh major` produces `v1.0.0`.
   - *Why*: Predictable first-release behavior without special-casing in the script.
   - *Rejected*: Special-case the first release as "always v0.0.1." Less general than the baseline-fallback approach.

5. **No `--force` flag, no `main`-only check on release.sh**: The script runs from whichever branch you're on, as long as the working tree is clean and HEAD is on a branch.
   - *Why*: Mirrors run-kit. Allows hotfix-branch releases without script changes.
   - *Rejected*: Hardcoded `main`-branch check. Would require a `--force` escape hatch and complicate hotfix flows.

6. **No code signing or notarization for v0.0.1 (or any future version, until decided otherwise)**: Binaries ship unsigned. macOS users see a Gatekeeper warning on first run; brew installs typically don't trip this as hard as direct downloads.
   - *Why*: Apple Developer accounts cost $99/year. The marginal UX win doesn't justify recurring cost for a personal-tooling CLI.
   - *Rejected*: Sign and notarize. Real but ongoing cost; can be added later if a third party demands it.

7. **No Linux native packaging**: Linux distribution is brew-on-Linux or direct tar.gz download.
   - *Why*: tar.gz covers brew-on-Linux. Adding `.deb`/`.rpm` requires `nfpm` or similar plus a separate distribution channel (custom apt/dnf repo, or release-page hosting that asks users to `dpkg -i`). Not worth it for v0.0.1.
   - *Rejected*: Add `.deb`. Real value if Linux users without brew show up, but speculative for now.

8. **No prerelease tag support**: `release.sh` accepts only `patch|minor|major`. The workflow does not handle `-rc.N` tags specially.
   - *Why*: First release; iterative testing of the pipeline can use one-off ad-hoc tags (e.g., `v0.0.0-test`) and manually clean up. Adding RC support is ~30 LOC across release.sh and the workflow.
   - *Rejected*: Add prerelease support up front. Speculative for v0.0.1.

9. **Action SHAs pinned, mirroring run-kit's exact SHAs at apply time**: Supply-chain hardening; also keeps both repos updateable via a single-source diff if action versions ever bump.
   - *Why*: Industry best practice for third-party GitHub Actions.
   - *Rejected*: `@v4`/`@v5` floating refs. Slightly less robust against malicious tag retargeting.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Release pipeline is hand-rolled (mirroring run-kit's workflow shape), not goreleaser | Confirmed from intake #1; run-kit precedent + cleaner minor-aware base-tag logic | S:100 R:75 A:90 D:95 |
| 2 | Certain | Build matrix: darwin-arm64, darwin-amd64, linux-arm64, linux-amd64 (no Windows) | Confirmed from intake #2; Constitution Cross-Platform Behavior section | S:100 R:90 A:100 D:100 |
| 3 | Certain | Homebrew tap target: `sahil87/homebrew-tap` | Confirmed from intake #3; already hosts `Formula/rk.rb` for run-kit | S:100 R:75 A:95 D:100 |
| 4 | Certain | First release version: v0.0.1 (no existing tags; baseline fallback `v0.0.0` + patch bump) | Confirmed from intake #4 | S:100 R:90 A:95 D:100 |
| 5 | Certain | Workflow trigger: tag push matching `v*` only | Confirmed from intake #5; matches run-kit's workflow trigger | S:100 R:80 A:95 D:100 |
| 6 | Certain | `HOMEBREW_TAP_TOKEN` provisioning is manual (out of code scope) | Confirmed from intake #6 | S:100 R:60 A:100 D:100 |
| 7 | Certain | Archive format: tar.gz containing only the binary (no LICENSE/README inside the archive) | Confirmed from intake #7; mirrors run-kit | S:95 R:70 A:90 D:85 |
| 8 | Certain | Checksums computed inline via `sha256sum`; not published as a separate `checksums.txt` Release asset | Confirmed from intake #8; mirrors run-kit | S:90 R:75 A:90 D:80 |
| 9 | Certain | Hand-rolled cross-compile loop in workflow; `softprops/action-gh-release@v2` for publishing | Confirmed from intake #9 | S:95 R:75 A:90 D:90 |
| 10 | Certain | Workflow uses `actions/setup-go@v5` with `go-version-file: src/go.mod` | Upgraded from intake Confident #10. Verified pattern matches run-kit; specific path adapts to this repo's `src/go.mod` location | S:95 R:85 A:95 D:90 |
| 11 | Certain | `scripts/release.sh` accepts `patch\|minor\|major`; computes current version from `git describe --tags --abbrev=0` (with `v0.0.0` fallback); creates and pushes a tag without modifying tracked files | Confirmed from intake #11 | S:100 R:80 A:95 D:95 |
| 12 | Certain | macOS code signing/notarization is not in scope — binaries ship unsigned, Gatekeeper warning is acceptable | Confirmed from intake #12; user explicitly removed the requirement | S:100 R:65 A:60 D:55 |
| 13 | Certain | Release notes via `softprops/action-gh-release@v2` with `generate_release_notes: true`. Minor-bump base-tag override (patch == 0 → first tag of previous minor cycle) | Confirmed from intake #13 | S:100 R:80 A:95 D:95 |
| 14 | Certain | No prerelease support — `release.sh` accepts only `patch\|minor\|major`; workflow does not handle `-rc.N` | Confirmed from intake #14 | S:95 R:75 A:75 D:60 |
| 15 | Certain | Linux release artifacts are tar.gz only — no `.deb`, `.rpm`, apt/dnf | Confirmed from intake #15 | S:95 R:80 A:80 D:65 |
| 16 | Certain | Version source of truth is the git tag (no `VERSION` file). Local: `git describe`. Release: `${GITHUB_REF#refs/tags/}`. Both inject ldflags retaining the `v` prefix | Confirmed from intake #16 | S:100 R:80 A:95 D:95 |
| 17 | Certain | Version printing is already wired by the parent change via cobra (`var version = "dev"` + `rootCmd.Version`). No Go code changes in this change. Output format is cobra's default: `repo version <string>` | Confirmed from intake #17; verified against the on-disk `src/cmd/repo/main.go` | S:100 R:90 A:100 D:95 |
| 18 | Certain | `.github/formula-template.rb` lives in this repo; workflow `sed`s placeholders and writes to the cloned tap's `Formula/repo.rb` | Confirmed from intake #18 | S:100 R:75 A:90 D:90 |
| 19 | Certain | Tap update is direct push to the tap's default branch (no PR), authored as `github-actions[bot]`, commit message `repo v<version>` | Upgraded from intake Confident #19. Spec-stage analysis: PR overhead is unjustified for a personal tap; matches run-kit | S:90 R:65 A:85 D:80 |
| 20 | Certain | All third-party action references pinned to commit SHAs with `# v<N>` comments; SHAs match run-kit's at apply time | Upgraded from intake Confident #20. Spec-stage analysis: keeping the SHAs in lockstep with run-kit is the natural baseline; deviations would need explicit justification | S:90 R:80 A:90 D:85 |
| 21 | Certain | `docs/specs/build-and-release.md` rewrite is in scope for apply (not deferred to a follow-up) | Confirmed from intake #21 | S:100 R:80 A:90 D:85 |
| 22 | Certain | `scripts/build.sh` is not modified — already uses `git describe --tags --always` | Confirmed from intake #22; verified against on-disk file | S:100 R:90 A:100 D:100 |
| 23 | Certain | Output format from cobra is `repo version <string>` (not `repo <string>` as an earlier version of the intake suggested). Acceptance criteria reflect this | Spec-stage discovery: cobra's `rootCmd.Version` produces the format `repo version <X>` automatically. Verified in cobra's source. Output examples in the intake updated to match | S:95 R:90 A:95 D:95 |
| 24 | Certain | Workflow's `version` step output (the bare-version form, no `v` prefix) is used only for `sed`-substituting the formula template's `version "VERSION_PLACEHOLDER"` field. The ldflags injection uses the prefixed form (`v0.0.1`) | Spec-stage clarification: avoiding double-`v` requires that the formula template's URL `releases/download/v#{version}/...` re-add the prefix, while the ldflags use the prefixed form directly | S:95 R:85 A:95 D:90 |
| 25 | Certain | `release.sh` with no arguments prints usage and exits 0 (informational), matching run-kit's exact behavior | Spec-stage decision: prevents a usability footgun where a typo (`just release` with no arg + no `default="patch"` in justfile) would otherwise error noisily. The justfile's `bump="patch"` default also prevents this in practice | S:90 R:85 A:90 D:80 |
| 26 | Confident | Sub-second precision is not needed in any output — the workflow run timing reported in the GitHub UI is sufficient. No `time` instrumentation in `release.sh` | Spec-stage decision: simple by default; no reason to instrument a script that runs in seconds | S:75 R:90 A:85 D:80 |
| 27 | Confident | The `test` block in the formula calls `repo --version` (not `repo version`). Cobra produces the same output for both, but `--version` is the spec'd public surface | Spec-stage decision: be explicit about which surface the formula tests | S:80 R:85 A:90 D:80 |
| 28 | Confident | The release-day runbook in the README/docs does NOT include "verify each tar.gz extracts and runs" — that's smoke-tested via `brew install ... && repo --version` (the simpler check that proves the entire chain) | Spec-stage decision: avoid runbook bloat. The brew-install smoke test exercises the full path | S:80 R:80 A:80 D:75 |

28 assumptions (25 certain, 3 confident, 0 tentative, 0 unresolved).
