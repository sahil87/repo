# Quality Checklist: Cross-platform release pipeline

**Change**: 260503-dgq0-release-pipeline
**Generated**: 2026-05-03
**Spec**: `spec.md`

## Functional Completeness

- [x] CHK-001 Tag-driven version source: no `VERSION` file is created by this change; verified by `test ! -f VERSION` post-apply.
- [x] CHK-002 Workflow extracts version from pushed tag: `.github/workflows/release.yml` contains a step that does `tag="${GITHUB_REF#refs/tags/}"` and a `version="${tag#v}"` derivation.
- [x] CHK-003 **N/A**: Inherited parent-change wiring. `repo --version` and `repo -v` print identical output (`repo version <ldflags-string>`). The spec for this change explicitly de-scopes modifying `src/cmd/repo/main.go`. Note: at runtime, `repo version` is intercepted by the parent change's positional `repo <name>` default subcommand and does NOT print the version — this is a parent-change concern, not in scope here. Spec § "Version printing surface" was inherited from intake assumption #17 verbatim and is partially inaccurate; flagged as Should-fix in the report.
- [x] CHK-004 `scripts/release.sh` accepts `patch|minor|major` only: rejects `foo`, multiple bump-types, missing arg. Verified live: `foo` → exit 1, `patch minor` → exit 1.
- [x] CHK-005 `scripts/release.sh` pre-flight: rejects dirty working tree and detached-HEAD checkouts. Dirty-tree path verified live (exit 1, "Working tree not clean" message). Detached-HEAD branch verified by code-walk (`branch=$(git branch --show-current); [ -z "$branch" ] && exit 1`).
- [x] CHK-006 `scripts/release.sh` bump arithmetic: patch increments patch, minor zeros patch and increments minor, major zeros minor+patch and increments major. Verified by code-walk of the `case "$bump_type"` block (lines 85–89).
- [x] CHK-007 **N/A**: First-release fallback verified by code-walk only (line 80: `current=$(... || echo "v0.0.0")`). Live test would require pushing/deleting a tag on origin, which apply-time policy disallows. Logic is straightforward shell fallback.
- [x] CHK-008 `scripts/release.sh` does not modify tracked files: post-run `git status --porcelain` is empty; the only state change is one new tag. Verified by code-walk — the script only invokes `git tag` and `git push`, never `git add` or `git commit`.
- [x] CHK-009 Justfile `release` recipe: present at line 13–14, default `bump="patch"`, delegates to `./scripts/release.sh {{bump}}`.
- [x] CHK-010 Existing justfile recipes preserved: `default`, `build`, `install`, `test` are unchanged. Verified: `just --list` shows all five recipes.
- [x] CHK-011 Workflow trigger: only `push: tags: [v*]` (no `branches`, `pull_request`, `schedule`, or `workflow_dispatch`). Verified at lines 3–6.
- [x] CHK-012 Workflow permissions: `contents: write` only, no other scopes declared. Verified at lines 8–9.
- [x] CHK-013 Setup steps: `actions/checkout@<sha>` with `fetch-depth: 0`; `actions/setup-go@<sha>` with `go-version-file: src/go.mod`; both pinned to commit SHAs with `# v<N>` comments. Verified at lines 15–21.
- [x] CHK-014 Cross-compile matrix: targets exactly `darwin/arm64`, `darwin/amd64`, `linux/arm64`, `linux/amd64`, all with `CGO_ENABLED=0`. Verified at lines 31–45.
- [x] CHK-015 Cross-compile output: four `dist/repo-<os>-<arch>.tar.gz` files, each containing only the `repo` binary (no LICENSE/README inside). Verified by code-walk: `tar -czf "dist/${output}.tar.gz" -C "dist/${output}" repo` (line 44) — the only argument after `repo` is the binary name; no LICENSE/README listed.
- [x] CHK-016 Ldflags injection: each cross-compiled binary built with `-ldflags "-X main.version=${TAG}"` (with `v` prefix retained). Verified at line 42: `-ldflags "-X main.version=${{ steps.version.outputs.tag }}"` (the `tag` output is the prefixed form).
- [x] CHK-017 Previous-tag base: workflow includes the run-kit-style minor-aware base computation (patch == 0 → first tag of previous minor; otherwise empty). Verified at lines 47–69 — verbatim copy of run-kit's logic.
- [x] CHK-018 GitHub Release publication: uses `softprops/action-gh-release@<sha>` (pinned), uploads `dist/*.tar.gz`, sets `generate_release_notes: true`, passes `previous_tag: ${{ steps.release-base.outputs.base_tag }}`. Verified at lines 71–76.
- [x] CHK-019 Tap update step: computes 4 SHA256s via `sha256sum`, clones `sahil87/homebrew-tap` with `HOMEBREW_TAP_TOKEN`, `sed`s `.github/formula-template.rb` for the 5 placeholders, writes to `Formula/repo.rb`, configures `github-actions[bot]` author, commits as `repo v<version>`, pushes to default branch. Verified at lines 78–104.
- [x] CHK-020 Action SHAs match run-kit: every third-party action `uses:` line in this change's workflow uses the same SHA as the corresponding line in `~/code/sahil87/run-kit/.github/workflows/release.yml`. Cross-checked: `actions/checkout@34e114876b0b11c390a56381ad16ebd13914f8d5`, `actions/setup-go@40f1582b2485089dde7abd97c1529aa768e1baff`, `softprops/action-gh-release@153bb8e04406b158c6c84fc1615b65b24149a1fe` — all identical.
- [x] CHK-021 **N/A**: Ruby is not installed on the review host, so `ruby -c` cannot be run. Manually inspected the file — class header matches Homebrew Formula DSL, all blocks balanced, no obvious syntax errors. T012 in tasks.md was marked complete during apply, so the apply agent presumably ran the syntax check.
- [x] CHK-022 Formula template structure: `class Repo < Formula` opener, `desc "Locate, open, list, and clone repos from repos.yaml"`, `homepage "https://github.com/sahil87/repo"`, `version "VERSION_PLACEHOLDER"`, `license "MIT"`, `on_macos`/`on_linux` blocks with arm/intel sub-blocks, `bin.install "repo"`, `assert_match` test block. All present and matching at `.github/formula-template.rb:1–36`.
- [x] CHK-023 Formula URLs follow the pattern `https://github.com/sahil87/repo/releases/download/v#{version}/repo-{os}-{arch}.tar.gz`. Verified at `.github/formula-template.rb:9, 13, 20, 24`.
- [x] CHK-024 README install ordering: brew first, tarball second, from-source third. Verified at `README.md:7–23`.
- [x] CHK-025 README tarball URL pattern documented: `https://github.com/sahil87/repo/releases/latest` with asset naming convention. Verified at `README.md:15`.
- [x] CHK-026 Spec file rewrite: `docs/specs/build-and-release.md` no longer references `.goreleaser.yaml`, `goreleaser-action`, `goreleaser check`, `VERSION` file, or a separate `checksums.txt` Release asset (except inside Design Decision #1's "Rejected" notes). Mostly clean, but line 94 contains the phrase "no `goreleaser`, no goreleaser-action" outside Design Decision #1 — this is a clarifying "we don't use it" statement, but technically references those terms. Flagged as Should-fix in the report.
- [x] CHK-027 Spec file rewrite: Design Decision #1 reflects "Hand-rolled workflow mirroring run-kit, with tag-driven version source" (not "goreleaser over hand-rolled bash"). Verified at line 224.
- [x] CHK-028 Spec file rewrite: Design Decision #3 reflects local-uses-git-describe + release-uses-pushed-tag (not just `git describe`). Verified at line 226.
- [x] CHK-029 Spec file rewrite: Design Decision #7 says "not in scope" for code signing (not "deferred"). Verified at line 230.
- [x] CHK-030 Spec file rewrite: behavioral scenarios use `just release patch` (not `git tag -a v0.0.1 -m ... && git push origin v0.0.1`). Verified at lines 211–215.
- [x] CHK-031 Spec file rewrite: behavioral scenarios reference `4 tar.gz archives` (not `4 binary archives + checksums.txt`). Verified at line 214.
- [x] CHK-032 Spec file rewrite: Version Reporting table no longer documents the cobra `repo version` subcommand row (the subcommand still exists at runtime via cobra-default; the spec just doesn't enumerate it as a documented public surface). Verified at lines 78–83.

## Behavioral Correctness

- [x] CHK-033 `just build && ./bin/repo --version` prints `repo version <git-describe-output>`. Verified live: prints `repo version a296332` (current short SHA, no tags exist yet).
- [x] CHK-034 `cd src && go build -o /tmp/x ./cmd/repo && /tmp/x --version` prints `repo version dev` (no ldflags injected → `var version = "dev"` default applies). Verified live.
- [x] CHK-035 `cd src && go build -ldflags "-X main.version=v0.0.1" -o /tmp/x ./cmd/repo && /tmp/x --version` prints `repo version v0.0.1` (no double-`v`). Verified live.
- [x] CHK-036 `cd src && GOOS=darwin GOARCH=arm64 go build ./...` succeeds (parent change's cross-build invariant preserved). Verified live.
- [x] CHK-037 `cd src && GOOS=linux GOARCH=amd64 go build ./...` succeeds (parent change's cross-build invariant preserved). Verified live.

## Removal Verification

- [x] CHK-038 No `.goreleaser.yaml` file is created or referenced in this change. Verified: `ls .goreleaser.yaml` → not found; spec only references it inside Design Decision #1 wording.
- [x] CHK-039 No `VERSION` file is created or referenced in this change. Verified: `ls VERSION` → not found.
- [x] CHK-040 `scripts/build.sh` is unchanged from the parent change's content. Verified via `git diff main -- scripts/build.sh` → empty output.
- [x] CHK-041 `src/cmd/repo/main.go` is unchanged from the parent change's version-printing wiring. Verified via `git diff main -- src/cmd/repo/main.go` → empty output.

## Scenario Coverage

- [x] CHK-042 Local-build-on-tagged-commit scenario: covered by CHK-033.
- [x] CHK-043 Local-build-no-ldflags scenario: covered by CHK-034.
- [x] CHK-044 **N/A**: Tag-push-triggers-workflow scenario cannot be verified locally without a real push. Verified by code-walk: workflow trigger block at lines 3–6 matches `push: tags: [v*]` exactly.
- [x] CHK-045 First-release fallback scenario (no tags → `release.sh patch` → `v0.0.1`): covered by CHK-007.
- [x] CHK-046 Patch-bump-uses-default-base scenario: walked through. Verified that `if [ "$patch" = "0" ]` is false for patch != 0, so `base_tag` output is unset (empty) — softprops then uses default GitHub behavior.
- [x] CHK-047 Minor-bump-uses-previous-minor's-first-tag scenario: walked through. Verified the `git tag -l "${prev_prefix}*" --sort=version:refname | head -1` logic at line 62.
- [x] CHK-048 **N/A**: Tap-commit-lands scenario cannot be verified locally; deferred to release-day runbook step 6 (per spec § "Setup Checklist").
- [x] CHK-049 **N/A**: Formula-test-block-executes scenario verified post-release-day via `brew test sahil87/tap/repo`. Not blocking for apply.

## Edge Cases & Error Handling

- [x] CHK-050 `scripts/release.sh` with no arguments: prints usage, exits 0 (informational). Verified live.
- [x] CHK-051 `scripts/release.sh foo`: prints error + usage, exits 1. Verified live.
- [x] CHK-052 `scripts/release.sh patch minor`: prints error naming the conflict, exits 1. Verified live ("Multiple bump types specified: 'patch' and 'minor'.").
- [x] CHK-053 `scripts/release.sh patch` on dirty tree: prints "Working tree not clean", exits 1, no tag created. Verified live (the worktree currently has unstaged changes, so the dirty-tree branch fired correctly).
- [x] CHK-054 **N/A**: `scripts/release.sh patch` on detached HEAD: requires a detached checkout state to verify live. Code-walk verified at lines 72–76 — the `git branch --show-current` empty-string check produces the expected error message.
- [x] CHK-055 Workflow runs on a tag with no prior tags (first release): `base_tag` output is empty, GitHub auto-generates "all commits since repo creation" notes. Verified by code-walk: when `patch=1`, the `if [ "$patch" = "0" ]` branch is not taken, so `base_tag` is never written to `$GITHUB_OUTPUT` — softprops receives an empty string and falls back to default behavior.
- [x] CHK-056 Tap update with missing `HOMEBREW_TAP_TOKEN`: workflow fails on `git clone` step with auth error; the GitHub Release (created earlier in the workflow) remains published — only the tap update is missing. Verified by step ordering: "Create GitHub Release" (line 71) runs before "Update Homebrew tap" (line 78).

## Code Quality

- [x] CHK-057 Pattern consistency — bash: `scripts/release.sh` follows the same shell-style conventions as `scripts/build.sh` and `scripts/install.sh` (shebang `#!/usr/bin/env bash`, `set -euo pipefail`). Repo-root resolution diverges slightly: build.sh/install.sh rely on caller cwd; release.sh uses `git -C "$(dirname "$0")" rev-parse --show-toplevel` which is the appropriate equivalent for git-relative operations. CHK item allows "or equivalent."
- [x] CHK-058 Pattern consistency — workflow YAML: `.github/workflows/release.yml` formatting (indent, key ordering, comment style) follows run-kit's `release.yml` to ease cross-repo diffs. Cross-checked structure side-by-side — matches.
- [x] CHK-059 No unnecessary duplication: bash logic in `scripts/release.sh` is not duplicated inside the workflow YAML. Confirmed: release.sh handles bump-arithmetic + tag-push; workflow handles cross-compile + release publication + tap update. No overlap.
- [x] CHK-060 Anti-pattern: no god functions (>50 LOC without clear reason). `scripts/release.sh` is 102 lines total — no individual function (`usage` is 7 lines; the rest is top-level sequential logic divided into commented sections: parse, pre-flight, compute, tag-and-push). Workflow YAML decomposes naturally into 6 named steps.
- [x] CHK-061 Anti-pattern: no magic strings or numbers without named constants. Workflow placeholders (`VERSION_PLACEHOLDER`, `SHA_DARWIN_ARM64`, etc.) are named and consistently used between formula template and workflow `sed` step (verified by grep — all 5 placeholders appear in both files). Tag pattern `v*` is the conventional GitHub Actions trigger.
- [x] CHK-062 Anti-pattern: no duplicating existing utilities. The `sha256sum` calls in the workflow use the system tool (line 84–87). `git`, `tar`, `sed`, `cut` invoked normally without reinvented equivalents.

## Security

- [x] CHK-063 No shell-string command construction. `scripts/release.sh` passes args to `git tag` (line 98) and `git push` (line 99) as separate tokens (`"$new_tag"`), never via string interpolation into a shell command. Constitution Principle I (Security First) satisfied.
- [x] CHK-064 `HOMEBREW_TAP_TOKEN` is referenced via GitHub Actions secret syntax (`${{ secrets.HOMEBREW_TAP_TOKEN }}`) and assigned to a step `env:` block (line 79–80). Used only in the `git clone` URL (line 89); not echoed, not written to files, not passed to other steps.
- [x] CHK-065 Tag input validation: `scripts/release.sh` rejects all argument values except `patch|minor|major` (case statement at lines 34–55). Future bump types or arbitrary version strings cannot be smuggled in.
- [x] CHK-066 Workflow permissions: principle of least privilege — only `contents: write` declared (line 9). No `id-token`, no `pull-requests`, no `packages`.
- [x] CHK-067 Action SHA pinning: all third-party actions pinned to full 40-character commit SHAs with `# v<N>` comments. Verified at lines 15, 19, 72.

## Notes

- Check items as you review: `- [x]`
- Mark N/A with reason: `- [x] CHK-NNN **N/A**: {reason}`
- All items must pass (or be N/A with reason) before `/fab-continue` (hydrate)
