# Intake: Cross-platform release pipeline

**Change**: 260503-dgq0-release-pipeline
**Created**: 2026-05-03
**Status**: Draft

## Origin

> Carved out from the parent change `260503-iu93-bootstrap-go-binary` (which builds the v0.0.1 Go binary in-session). The release pipeline was split out because it requires CI runs and `HOMEBREW_TAP_TOKEN` provisioning that can't happen inside a single conversational session — those need to happen between sessions, with the user provisioning the secret in GitHub repo settings.

This intake is **draft, not active**. It exists as the explicit follow-up to the binary-bootstrap change. The user explicitly asked for it ("you create another intake for the pipeline files").

## Why

### The problem

The bootstrap change ships a binary that compiles and runs locally, but it does not yet:
- Produce cross-platform release artifacts (4-target tar.gz binaries with SHA256 checksums)
- Publish those artifacts to GitHub Releases on tag push
- Update `homebrew-tap` (the `sahil87/homebrew-tap` repo) so users can `brew install sahil87/tap/repo`

Without these, distribution is "build from source" only. That's fine for Sahil but blocks any onboarding flow for other users (which was a stated goal during the `/fab-discuss` session).

### The consequences if we don't act

- Every user must clone the source and run `just install` themselves.
- Version pinning is informal — there are no published artifacts at named versions.
- The `repos.yaml` starter that ships embedded in the binary references this repo (`git@github.com:sahil87/repo.git`), making "install this tool" a chicken-and-egg problem (need the tool to install the tool).

### Why this approach (mirror run-kit's hand-rolled pipeline)

- **Sibling project precedent (workflow shape, not version source).** `~/code/sahil87/run-kit` already ships through `sahil87/homebrew-tap` using a hand-rolled GitHub Actions workflow (no goreleaser). We mirror its workflow shape: cross-compile loop, `softprops/action-gh-release`, hand-rolled tap update via formula template + `sed`. One mental model for both projects' release pipelines.
- **Tag-driven version source (diverges from run-kit).** Run-kit uses a `VERSION` file because it's a multi-binary monorepo (frontend + backend); a file disambiguates which version applies. `repo` is single-binary, so the git tag itself is the natural source of truth. `scripts/release.sh` computes the next tag from `git describe --tags --abbrev=0` (with a `v0.0.0` fallback for the first release), pushes the tag, and lets CI take over. The release workflow reads `${GITHUB_REF#refs/tags/}` and ldflags-injects it as `main.version`. The local `scripts/build.sh` (already in the repo from the parent change) keeps using `git describe --tags --always`, so it auto-includes commit-past-tag info (`v0.0.1-2-gabc123`) and pre-tag SHAs.
- **GitHub-native release notes.** `softprops/action-gh-release` with `generate_release_notes: true` produces PR-based notes. A small bash step computes the previous-tag base — for patch bumps, default behavior; for minor bumps (patch == 0), the base tag becomes the **first tag of the previous minor cycle**, so v0.2.0's notes span the entire 0.1.x series.
- **Cross-platform matrix matches the constitution.** `darwin/arm64 darwin/amd64 linux/arm64 linux/amd64` — the four targets the Constitution's Cross-Platform Behavior section names. Same matrix as run-kit.
- **Homebrew tap update via direct push.** A `.github/formula-template.rb` lives in this repo with `VERSION_PLACEHOLDER` and `SHA_*` markers. CI `sed`s the template, clones the tap with `HOMEBREW_TAP_TOKEN`, commits, and pushes. ~25 lines of workflow bash — readable top-to-bottom.

### Rejected alternatives

- **Goreleaser**: Rejected. Smaller config (~30 LOC vs. ~80) and built-in tap update are real advantages, but the **minor-aware base-tag logic for release notes is awkward in goreleaser** (requires disabling its changelog and post-hoc `gh release edit` via `gh api`). The hand-rolled pattern handles it natively via `softprops/action-gh-release`'s `previous_tag` parameter. Combined with the consistency win of mirroring run-kit (same author, same tap, same target platforms), goreleaser's leverage doesn't compound for a single-binary CLI.
- **Manual releases (build locally, upload to GitHub by hand)**: Rejected. Doesn't scale; introduces drift between what's tagged and what's published.
- **No homebrew-tap, just GitHub Releases**: Rejected. `brew install` is the canonical Mac install path; without the tap, Mac onboarding is "download a tar.gz and unpack it manually."

## What Changes

This change implements the release pipeline by mirroring `~/code/sahil87/run-kit`'s hand-rolled workflow shape, with a key divergence: **the git tag is the version source of truth**, not a `VERSION` file (run-kit uses one because it's multi-binary; `repo` doesn't need it). `scripts/release.sh` computes the next tag, pushes it, and CI takes over: cross-compiles, publishes a GitHub Release with PR-based notes, and updates `sahil87/homebrew-tap` via direct push.

`docs/specs/build-and-release.md` (the "Cross-Platform Release Pipeline (deferred to follow-up change)" section) was authored when goreleaser was the planned tool. The spec is now **partially superseded** by this intake on tooling specifics — the platform matrix, distribution targets, and setup checklist still hold; the `.goreleaser.yaml`/workflow content blocks do not. The spec will be updated during this change's hydrate stage to reflect the run-kit-mirroring decision.

Outcome: a working release pipeline that publishes a v0.0.1 GitHub Release with 4 binaries (darwin-arm64, darwin-amd64, linux-arm64, linux-amd64) and updates `sahil87/homebrew-tap` so `brew install sahil87/tap/repo` succeeds.

### Files to create / modify

| File | Action | Source of truth |
|---|---|---|
| `.github/workflows/release.yml` | Create | Mirror `~/code/sahil87/run-kit/.github/workflows/release.yml`, adapted for one Go binary (no frontend build, no embed step) and tag-driven version source |
| `.github/formula-template.rb` | Create | Mirror `~/code/sahil87/run-kit/.github/formula-template.rb`, adapted: class name `Repo`, binary `repo`, description, repo URL |
| `scripts/release.sh` | Create | See "Release script behavior" below |
| `src/cmd/repo/main.go` | **Not modified** — parent change already wired `var version = "dev"` + `rootCmd.Version = version`. Cobra auto-wires `--version`, `-v`, and the `version` subcommand. See "Version printing" below | n/a |
| `justfile` | Modify — add `release` recipe | See "Justfile recipe" below |
| `README.md` | Modify — replace install section | `docs/specs/build-and-release.md` § Distribution Channels |
| `docs/specs/build-and-release.md` | Modify — rewrite the goreleaser-flavored sections to reflect the run-kit-mirroring decision; flip Design Decisions #1 and #3; update behavioral scenarios | This intake; see "Spec rewrite scope" below |

`scripts/build.sh` is **unchanged** — it already uses `git describe --tags --always 2>/dev/null || echo dev`, which produces sensible output across pre-tag, tagged, and post-tag commits.

### Version source of truth

The git tag is the source of truth (no `VERSION` file).

- **Local builds** (`scripts/build.sh`, unchanged): use `git describe --tags --always 2>/dev/null || echo dev`. Produces `v0.0.1` on tagged commits, `v0.0.1-2-gabc123` on commits past a tag, short SHA pre-tag, `dev` if no git history.
- **Release builds** (workflow): extract version from the pushed tag via `${GITHUB_REF#refs/tags/}`. Inject as `-X main.version=v0.0.1` (with `v` prefix preserved for output consistency).
- **Formula update** (workflow): strip the `v` prefix once for templating Ruby's `version "0.0.1"` field — the formula's URL string is `releases/download/v#{version}/...` which already adds the `v` back.
- **Print form**: `repo --version` always prints `repo <whatever-was-injected>`. Output examples:
  - Local tagged: `repo v0.0.1`
  - Local post-tag: `repo v0.0.1-2-gabc123`
  - Local pre-tag: `repo a08147d`
  - Local no-git: `repo dev`
  - Release: `repo v0.0.1`
  - Built without any ldflags (e.g., `go install ...` directly): `repo unknown`

### Spec rewrite scope

`docs/specs/build-and-release.md` was authored with goreleaser as the planned tool and a Cobra-default `repo version` subcommand. As part of this change's apply stage, rewrite:

- **Header (lines 1-5)**: Update the "follow-up change" framing to reference run-kit-mirroring, drop `.goreleaser.yaml` mention.
- **Version Reporting section**: Drop the Cobra `repo version` subcommand row. `--version` flag (and optional `-v` short form) is the only public surface.
- **Cross-Platform Release Pipeline section**: Replace the entire `.goreleaser.yaml` block with a description of the run-kit-mirroring workflow. Replace the `.github/workflows/release.yml` block with the new tag-driven workflow.
- **Initial Release section**: Update to reflect tag-driven model (no `VERSION` file initialization).
- **Behavioral scenarios**: Replace `git tag -a v0.0.1 -m ...; git push origin v0.0.1` with `just release patch`. Replace `4 binary archives + checksums.txt` with `4 binary tar.gz archives` (no separate `checksums.txt` file is published).
- **Design Decisions**:
  - Flip #1 (was: "goreleaser over hand-rolled bash") to "Hand-rolled workflow mirroring run-kit, with tag-driven version source. Goreleaser rejected because the minor-aware base-tag logic for release notes is awkward there."
  - Update #3 (was: "Version injection via ldflags ... single source of truth: `git describe`") — keep the ldflags injection point, but expand: local builds use `git describe`, release builds use the pushed tag. Same string in both paths in the released case (`v0.0.1`).
  - Decision #7 (no code signing) — strengthen wording from "deferred" to "not in scope" per the explicit clarification in this change.

### Release script behavior (`scripts/release.sh`)

Mirror run-kit's `scripts/release.sh` UX, but tag-driven (no `VERSION` file write):

- Accept exactly one arg: `patch` | `minor` | `major`.
- Reject unknown args, missing args, or multiple bump-type args. Print usage and exit 1.
- Pre-flight:
  - Working tree clean (`git status --porcelain` empty). Exit 1 if not. (Prevents tagging a dirty state.)
  - On a branch (not detached HEAD). Exit 1 if not.
- Compute current version: `current=$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")` — falls back to `v0.0.0` when no tags exist (first release).
- Strip leading `v`, parse as `major.minor.patch`, bump per arg.
- Print confirmation line: `Releasing v$new_version ($current → v$new_version)`.
- `git tag "v$new_version"`.
- `git push origin "v$new_version"`.
- Print: `Done — v$new_version pushed. CI will cross-compile, create the GitHub Release, and update the Homebrew tap.`

No `--force` flag, no main-branch check (run-kit doesn't have one — releases happen from whichever branch the script is run on, intentional flexibility). No commit step (the script doesn't modify any tracked files).

First-release flow: `git describe --tags --abbrev=0` exits non-zero on a tagless repo. The `|| echo "v0.0.0"` fallback gives `release.sh patch` → `v0.0.1`, `release.sh minor` → `v0.1.0`, `release.sh major` → `v1.0.0`.

### Workflow behavior (`.github/workflows/release.yml`)

Mirror run-kit's workflow, simplified for a single Go binary (no frontend, no embed steps):

1. Trigger on `push: tags: [v*]`.
2. Permissions: `contents: write`.
3. Checkout with `fetch-depth: 0` (needed for the previous-tag-base computation).
4. `setup-go@v5` with `go-version-file: src/go.mod`.
5. Compute version from the pushed tag:
   ```
   tag="${GITHUB_REF#refs/tags/}"        # v0.0.1
   version="${tag#v}"                    # 0.0.1 (used for formula sed)
   echo "tag=$tag" >> "$GITHUB_OUTPUT"
   echo "version=$version" >> "$GITHUB_OUTPUT"
   ```
6. Cross-compile loop:
   ```
   for target in darwin/arm64 darwin/amd64 linux/arm64 linux/amd64; do
     CGO_ENABLED=0 GOOS=$os GOARCH=$arch \
       go build -ldflags "-X main.version=${TAG}" \
       -o "dist/repo-${os}-${arch}/repo" ./src/cmd/repo
     tar -czf "dist/repo-${os}-${arch}.tar.gz" -C "dist/repo-${os}-${arch}" repo
   done
   ```
   Note: `main.version` receives the **tag with `v` prefix** (e.g., `v0.0.1`) — same form `git describe` produces locally, so `repo --version` output is consistent across local and release builds. Each tar.gz contains **only the binary** (no LICENSE/README inside the archive — those live in the GitHub Release page and the source repo).
7. **Compute previous-tag base** (run-kit's logic):
   - Parse current tag: extract `major.minor.patch`.
   - If `patch == 0` (minor bump): find the earliest tag matching `v{major}.{minor-1}.*` via `git tag -l "${prev_prefix}*" --sort=version:refname | head -1`. Set `base_tag` to that.
   - Otherwise: leave `base_tag` empty (default behavior — compare to immediate previous tag).
8. Create GitHub Release via `softprops/action-gh-release@v2` with `files: dist/*.tar.gz`, `generate_release_notes: true`, `previous_tag: ${{ steps.release-base.outputs.base_tag }}`.
9. Update Homebrew tap:
   - Compute `sha256sum` for each of the 4 tar.gz files.
   - `git clone https://x-access-token:${TAP_TOKEN}@github.com/sahil87/homebrew-tap.git /tmp/homebrew-tap`.
   - `sed` `.github/formula-template.rb` replacing `VERSION_PLACEHOLDER`, `SHA_DARWIN_ARM64`, `SHA_DARWIN_AMD64`, `SHA_LINUX_ARM64`, `SHA_LINUX_AMD64`.
   - Write to `/tmp/homebrew-tap/Formula/repo.rb`.
   - Configure `git config user.name/email` to `github-actions[bot]`.
   - Commit `repo v$VERSION` and push.

All action SHAs SHOULD be pinned (run-kit pins them; e.g., `actions/checkout@34e114876b0b11c390a56381ad16ebd13914f8d5 # v4`). Use the same SHAs as run-kit's current workflow at apply time.

### Formula template (`.github/formula-template.rb`)

Mirror run-kit's `formula-template.rb`, with these substitutions:
- `class Rk` → `class Repo`
- `desc "Tmux session manager with web UI"` → `desc "Locate, open, list, and clone repos from repos.yaml"`
- `homepage "https://github.com/sahil87/run-kit"` → `homepage "https://github.com/sahil87/repo"`
- All URLs `run-kit/releases/download/v#{version}/rk-...` → `repo/releases/download/v#{version}/repo-...`
- `bin.install "rk"` → `bin.install "repo"`
- `test do` block: `assert_match "repo version", shell_output("#{bin}/repo --version")` (or simply `assert_match version.to_s, shell_output("#{bin}/repo --version")`)

License: copy whatever run-kit declares (MIT). If `repo` doesn't yet have a LICENSE file, that's a separate concern — the formula's `license "MIT"` line is informational only; brew doesn't enforce it. (Adding a LICENSE file is out of scope for this change; can be a follow-up.)

### Version printing (Go side)

**No code change required for this stage.** The parent change `260503-iu93-bootstrap-go-binary` already wired up version printing in `src/cmd/repo/main.go`:

```go
var version = "dev"

func main() {
    rootCmd := newRootCmd()
    rootCmd.Version = version
    if err := rootCmd.Execute(); err != nil {
        os.Exit(translateExit(err))
    }
}
```

Cobra's auto-wiring from `rootCmd.Version` produces:
- `repo --version` → prints `repo version <version>`
- `repo -v` → same as `--version`
- `repo version` → same (cobra-default subcommand)

**Output forms** (cobra's format is `repo version <string>`):
- Built without ldflags (e.g., `go install ...`): `repo version dev` (the `var version = "dev"` default)
- Local `just build` (uses `git describe --tags --always`): `repo version v0.0.1` / `repo version v0.0.1-2-gabc123` / `repo version a08147d` / `repo version dev` (no git history)
- Release workflow build: `repo version v0.0.1`

The `dev` default in the existing source is a more useful fallback than the intake originally proposed (`unknown`). No reason to change it.

**Why no `v` prefix is added in Go**: both `git describe` (local) and the workflow's `${GITHUB_REF#refs/tags/}` (release) produce strings that already include the `v` prefix on tagged commits (e.g., `v0.0.1`). Adding another `v` would yield `vv0.0.1`.

### Justfile recipe

Append to the existing `justfile`:

```just
release bump="patch":
    scripts/release.sh {{bump}}
```

Default to `patch`. Mirrors run-kit's recipe.

Existing recipes (`build`, `install`, `test`) stay unchanged.

### Apply-stage acceptance criteria

1. `cd src && go build ./...` still passes (no regression from parent change).
2. `repo --version` (after `just build`, which uses `git describe`) prints `repo version <string>` (cobra format) where `<string>` is `git describe --tags --always`'s output. On a tagless repo: `repo version <short-sha>` or `repo version dev`.
3. `repo --version` (built with no ldflags) prints `repo version dev` (the `var version = "dev"` default).
4. `repo --version` (built with explicit ldflags `-X main.version=v0.0.1`) prints `repo version v0.0.1` (no double-`v`).
5. `scripts/release.sh` is executable and:
   - `scripts/release.sh foo` exits 1 with usage message.
   - `scripts/release.sh patch minor` exits 1 (multiple bump types).
   - `scripts/release.sh patch` (with dirty working tree) exits 1.
   - `scripts/release.sh patch` (with clean working tree, on a branch, no existing tags) creates tag `v0.0.1` locally — but **don't actually push** during apply; delete the tag after testing via `git tag -d v0.0.1`.
6. `act -j release` (or equivalent local GitHub Actions runner) can lint the workflow YAML and reach the cross-compile step. Full end-to-end runs need real secrets and a real tag — those happen on release day.
7. `.github/formula-template.rb` is well-formed Ruby (`ruby -c .github/formula-template.rb` passes).
8. README's install section shows the `brew install sahil87/tap/repo` command first, "from source" second.
9. `docs/specs/build-and-release.md` is rewritten — the goreleaser block is gone, `.github/workflows/release.yml` block reflects the run-kit-mirroring tag-driven workflow, Design Decisions #1 and #3 are flipped, behavioral scenarios use `just release patch`.

### Release-day runbook (out of code; user-driven)

After apply + review pass:

1. **Provision `HOMEBREW_TAP_TOKEN`** — fine-grained PAT with `Contents: write` on `sahil87/homebrew-tap`. Add as a GitHub secret on `sahil87/repo`. Same token shape as run-kit uses.
2. **Verify `sahil87/homebrew-tap`** has a `Formula/` directory. (Already exists for `rk.rb`; should be fine.)
3. `just release patch` — `git describe --tags --abbrev=0` falls back to `v0.0.0` (no tags exist yet), script bumps to `v0.0.1`, creates tag, pushes.
4. Watch the workflow at `https://github.com/sahil87/repo/actions`.
5. Verify `https://github.com/sahil87/repo/releases/tag/v0.0.1` has 4 tar.gz binaries (no separate `checksums.txt` — checksums are computed inline by the workflow and only used for the formula update).
6. Verify `sahil87/homebrew-tap` got a new commit adding `Formula/repo.rb`.
7. Smoke test in a clean shell: `brew install sahil87/tap/repo && repo --version` prints `repo v0.0.1`.

Steps 1–2 cannot be done by the apply agent (they require GitHub UI and external repo state). Steps 3–7 happen *after* this change is merged — they are not part of apply.

## Affected Memory

- `build/release-pipeline` (new) — hand-rolled GitHub Actions release workflow, formula template, release script, homebrew-tap integration, tag-driven version source
- `build/local` (modify) — current "Out of scope for v0.0.1" section enumerates everything this change introduces. Replace with a "Cross-references release pipeline" pointer; update version-printing description if needed

## Impact

**Files created**:
- `.github/workflows/release.yml`
- `.github/formula-template.rb`
- `scripts/release.sh`

**Files modified**:
- `justfile` — add `release bump="patch"` recipe
- `README.md` — install section update (brew first, from-source second)
- `docs/specs/build-and-release.md` — rewrite goreleaser-flavored sections (see "Spec rewrite scope" above)

**Files NOT modified**:
- `scripts/build.sh` — already uses `git describe --tags --always`, which is the desired behavior
- `src/cmd/repo/main.go` — parent change already wired `var version = "dev"` + `rootCmd.Version`; cobra auto-wires `--version`/`-v`/`version` subcommand

**Out-of-repo state changes**:
- GitHub repo secret `HOMEBREW_TAP_TOKEN` provisioned by user (fine-grained PAT, `Contents: write` on `sahil87/homebrew-tap`)
- First `v0.0.1` tag pushed by `just release patch`, triggering first GitHub Release
- `sahil87/homebrew-tap` updated by the workflow with a new commit adding `Formula/repo.rb`

**Dependencies on parent change** (`260503-iu93-bootstrap-go-binary`):
- The `src/` layout must exist (the workflow's `go build ./src/cmd/repo` path).
- The `justfile` must already exist (this change *adds* a recipe, doesn't create).
- The `scripts/` directory must already exist.
- The README must already have placeholder content this change replaces.
- A `main.go` (or equivalent) must exist at the build path so `--version` wiring has a target.

This change MUST NOT be applied until the parent change is merged.

## Open Questions

All open items are tracked in the Assumptions table below as Confident or Tentative rows. No questions blocking this intake.

## Clarifications

### Session 2026-05-03

| # | Q | A |
|---|---|---|
| 12 | macOS code signing / notarization for v0.0.1? | Not in scope — remove the requirement entirely. Binaries ship unsigned; Gatekeeper warning is acceptable. |
| 1, 7-9, 13 | Release tooling: goreleaser or hand-rolled? | Mirror `~/code/sahil87/run-kit`'s hand-rolled workflow shape. Release notes via `softprops/action-gh-release` `generate_release_notes: true` with run-kit's minor-aware base-tag logic (patch == 0 → previous minor's first tag). `.github/formula-template.rb` in the source repo, sed-substituted at release time. Direct push to `homebrew-tap` (no PR). |
| n/a | Version printing: `--version` flag, `version` subcommand, or skip? | `--version` flag (lighter than a subcommand; respects Constitution Principle VI Minimal Surface Area). |
| 11, 16 | Version source of truth: `VERSION` file (run-kit pattern) or git tags? | Git tags. `repo` is single-binary, so the run-kit `VERSION`-file rationale (multi-binary monorepo) doesn't apply. Local builds use `git describe`, release builds use the pushed tag. `release.sh` computes the next tag from `git describe --tags --abbrev=0` with `v0.0.0` fallback. |
| 17 | `--version` output when no ldflags injected? | `repo unknown`. Strings produced by `git describe` and the workflow tag-extract already include the `v` prefix on tagged commits, so no prefix is added in Go. |
| 14 | Prerelease support (`v0.0.1-rc.N`) for the first release? | No. Mirror run-kit — `release.sh` accepts only `patch\|minor\|major`. Can be added later if iterative testing becomes valuable. |
| 15 | Linux native packaging (.deb / .rpm) for v0.0.1? | No — tar.gz only. Linux users install via brew-on-Linux or direct download. Native packaging is a follow-up if user demand surfaces. |
| 21 | `docs/specs/build-and-release.md` rewrite — in this change or follow-up? | In this change. Apply stage rewrites the goreleaser-flavored sections; acceptance criterion #9 verifies. |
| n/a | Change type? | `feat` (was incorrectly auto-classified as `fix`). |

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Release pipeline is hand-rolled (mirroring run-kit), not goreleaser | Clarified — user chose to mirror `~/code/sahil87/run-kit` for consistency, cleaner minor-aware base-tag logic, and one mental model across both projects | S:95 R:75 A:90 D:95 |
| 2 | Certain | Build matrix: darwin-arm64, darwin-amd64, linux-arm64, linux-amd64 (no Windows) | Constitution Cross-Platform Behavior section explicitly lists these four | S:100 R:90 A:100 D:100 |
| 3 | Certain | Homebrew tap target: `sahil87/homebrew-tap` | Already in user's repos.yaml (`~/code/bootstrap/dotfiles/repos.yaml` line 11), already hosts run-kit's formula | S:100 R:75 A:95 D:100 |
| 4 | Certain | First release version: v0.0.1 (no existing tags; `just release patch` falls back to `v0.0.0` baseline, bumps to `v0.0.1`) | Inherited from parent change — user explicitly chose 0.0.1; clarified-confirmed bump-from-baseline model | S:100 R:90 A:95 D:100 |
| 5 | Certain | CI trigger is git tag push matching `v*` | Mirrors run-kit's `on: push: tags: [v*]`, matches Constitution Principle V (release via tag) | S:95 R:80 A:95 D:95 |
| 6 | Certain | `HOMEBREW_TAP_TOKEN` is a manual setup step, not automated | GitHub PATs cannot be provisioned via code | S:100 R:60 A:100 D:100 |
| 7 | Certain | Archive format: tar.gz containing only the binary (no LICENSE/README inside the archive) | Mirrors run-kit's `tar -czf "...tar.gz" -C "...dir" rk` — directory contains only the binary; LICENSE/README live in repo and Release page | S:90 R:70 A:90 D:80 |
| 8 | Certain | Checksums: computed inline via `sha256sum` in the workflow, used only to template the formula. No `checksums.txt` published as a separate Release asset | Mirrors run-kit's pattern. Brew formula contains the SHAs; users who download directly can verify via the formula or recompute | S:85 R:75 A:90 D:80 |
| 9 | Certain | Hand-rolled cross-compile loop in workflow (not goreleaser-action). `softprops/action-gh-release@v2` for publishing | Mirrors run-kit's workflow exactly | S:90 R:75 A:90 D:90 |
| 10 | Confident | Workflow uses `actions/setup-go@v5` with `go-version-file: src/go.mod` | Standard pattern; mirrors run-kit (which uses `go-version-file: app/backend/go.mod` adapted to `src/go.mod` for this repo) | S:90 R:85 A:90 D:85 |
| 11 | Certain | `scripts/release.sh` takes `patch\|minor\|major`, computes current version from `git describe --tags --abbrev=0` (with `v0.0.0` fallback), creates tag, pushes tag. Does NOT modify any tracked files (no commit step) | Clarified — tag-driven instead of file-driven; no `VERSION` file. UX (`just release patch`) matches run-kit, internals diverge | S:95 R:80 A:90 D:90 |
| 12 | Certain | macOS code signing / notarization is **not in scope** — binaries are unsigned, Gatekeeper warning is acceptable | Clarified — user explicitly removed this requirement; no Apple Developer account, no notarization step in workflow | S:95 R:65 A:60 D:55 |
| 13 | Certain | Release notes use `softprops/action-gh-release@v2` with `generate_release_notes: true`. For minor bumps (patch == 0), the workflow computes a `previous_tag` set to the earliest tag of the previous minor cycle (`v{major}.{minor-1}.*` sorted version:refname, head -1). For patch bumps, default behavior (compare to immediate previous tag) | Clarified — mirror run-kit's release-notes-base-tag logic. Trivial in hand-rolled approach via `previous_tag:` parameter | S:95 R:80 A:90 D:90 |
| 14 | Certain | No prerelease support — `release.sh` accepts only `patch\|minor\|major`. The workflow does not handle `-rc.N` tags. Mirrors run-kit | Clarified — user confirmed; adds complexity for marginal benefit; can be added later if iterative testing becomes valuable | S:95 R:75 A:75 D:60 |
| 15 | Certain | Linux release artifacts are tar.gz only — no `.deb`, no `.rpm`, no apt/dnf repo. Linux users install via brew-on-Linux or direct tar.gz download. Mirrors run-kit | Clarified — user confirmed; native Linux packaging is a meaningful follow-up if/when user demand surfaces; out of scope for v0.0.1 | S:95 R:80 A:80 D:65 |
| 16 | Certain | Version source of truth is the git tag (no `VERSION` file). Local builds use `git describe --tags --always` (already in `scripts/build.sh`); release builds use `${GITHUB_REF#refs/tags/}` from the pushed tag | Clarified — diverges from run-kit (which uses `VERSION` because it's multi-binary). `repo` is single-binary; tag-driven is simpler. Strips one file from the design and avoids `VERSION`/tag drift | S:95 R:75 A:90 D:90 |
| 17 | Certain | Version printing is **already wired** by the parent change via cobra's `rootCmd.Version = version`. Cobra auto-provides `--version`, `-v`, and `version` subcommand. Default is `var version = "dev"`, overridden by `-ldflags "-X main.version=..."` at build. No Go code changes in this change | Clarified — verified against the on-disk `src/cmd/repo/main.go`. Removes the originally-proposed Go modification from scope | S:100 R:90 A:100 D:95 |
| 18 | Certain | `.github/formula-template.rb` lives in this repo (not in the tap repo). Workflow `sed`s placeholders (`VERSION_PLACEHOLDER`, `SHA_*`) and writes the result to `Formula/repo.rb` in the cloned tap | Clarified — mirror run-kit's template-and-sed pattern | S:95 R:75 A:90 D:90 |
| 19 | Confident | Tap update is a direct push to the tap's default branch (no PR), authored as `github-actions[bot]`. Commit message: `repo v<version>` | Mirrors run-kit's pattern. `pull_request: enabled: true` would slow releases for marginal benefit on a personal-tap repo | S:80 R:65 A:85 D:75 |
| 20 | Confident | All third-party action references are pinned to commit SHAs (with `# v<N>` comments) — same SHAs as run-kit's current workflow at apply time | Mirrors run-kit; supply-chain hardening | S:85 R:80 A:85 D:80 |
| 21 | Certain | `docs/specs/build-and-release.md` is rewritten as part of this change's apply stage (not deferred to a follow-up). Specific sections to rewrite are enumerated in "Spec rewrite scope" above. Acceptance criterion #9 verifies the rewrite | Clarified — user confirmed in-scope. Doing it inline keeps the spec coherent with the implementation at merge time | S:95 R:80 A:90 D:85 |
| 22 | Certain | `scripts/build.sh` is **not modified** by this change. It already uses `git describe --tags --always 2>/dev/null \|\| echo dev`, which produces the desired output across all build states (pre-tag, tagged, post-tag, no-git) | Clarified — verified by reading the on-disk file. Avoiding a modification keeps the change surface smaller | S:100 R:90 A:100 D:100 |

22 assumptions (19 certain, 3 confident, 0 tentative, 0 unresolved).
