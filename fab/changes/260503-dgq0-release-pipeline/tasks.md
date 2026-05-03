# Tasks: Cross-platform release pipeline

**Change**: 260503-dgq0-release-pipeline
**Spec**: `spec.md`
**Intake**: `intake.md`

## Phase 1: Setup

- [x] T001 [P] Pull current pinned action SHAs from `~/code/sahil87/run-kit/.github/workflows/release.yml` to use verbatim — capture the full 40-character SHAs and version comments for `actions/checkout`, `actions/setup-go`, `softprops/action-gh-release`, plus any other shared actions, into a working note for use in T004.
- [x] T002 [P] Read `~/code/sahil87/run-kit/.github/formula-template.rb` and confirm the field set we'll mirror (class header, `desc`, `homepage`, `version`, `license`, `on_macos`/`on_linux` blocks, `install`, `test`). Note any divergences planned (e.g., `desc` text, URL hostname).

## Phase 2: Core Implementation

- [x] T003 Create `scripts/release.sh` with `patch|minor|major` argument parsing, dirty-tree + detached-HEAD pre-flight checks, `git describe --tags --abbrev=0` lookup with `v0.0.0` fallback, version bump arithmetic, `git tag` + `git push origin <tag>`, and the success message. Make it executable (`chmod +x`). Mirror run-kit's `scripts/release.sh` structure but drop the `VERSION`-file write and the commit step. Reference: spec § "Build & Release: Release Script" requirements 1–5.
- [x] T004 Create `.github/workflows/release.yml` mirroring `~/code/sahil87/run-kit/.github/workflows/release.yml` with adaptations: (a) replace the version-from-VERSION-file step with a tag-extract step (`tag="${GITHUB_REF#refs/tags/}"`, `version="${tag#v}"`); (b) drop the frontend build/embed steps; (c) cross-compile loop targets `./src/cmd/repo` (not `app/backend/...`); (d) tar.gz output names use `repo-<os>-<arch>.tar.gz` (not `rk-...`); (e) keep the previous-tag-base computation logic verbatim; (f) keep the `softprops/action-gh-release@<sha>` step with `generate_release_notes: true` and `previous_tag: ${{ steps.release-base.outputs.base_tag }}`; (g) tap update step uses `formula-template.rb` with placeholders `VERSION_PLACEHOLDER`, `SHA_DARWIN_ARM64`, `SHA_DARWIN_AMD64`, `SHA_LINUX_ARM64`, `SHA_LINUX_AMD64`; output path `Formula/repo.rb`; commit message `repo v${version}`. Use the SHAs captured in T001. Reference: spec § "Build & Release: GitHub Actions Workflow" requirements.
- [x] T005 [P] Create `.github/formula-template.rb` mirroring run-kit's structure with substitutions: `class Repo`, `desc "Locate, open, list, and clone repos from repos.yaml"`, `homepage "https://github.com/sahil87/repo"`, all `rk` → `repo` URL/binary references, `bin.install "repo"`, test block `assert_match version.to_s, shell_output("#{bin}/repo --version")`. Run `ruby -c .github/formula-template.rb` to confirm syntax. Reference: spec § "Build & Release: Formula Template" requirements.
- [x] T006 [P] Add `release` recipe to the existing `justfile`: `release bump="patch":\n    scripts/release.sh {{bump}}`. Verify existing `default`, `build`, `install`, `test` recipes are unchanged. Reference: spec § "Build & Release: Justfile" requirements.

## Phase 3: Integration & Edge Cases

- [x] T007 [P] Update `README.md` install section: add Homebrew as the primary install method (`brew install sahil87/tap/repo`), then GitHub Release tarball download (with URL pattern `https://github.com/sahil87/repo/releases/latest` and asset naming `repo-{os}-{arch}.tar.gz`), then "From source" (existing `git clone` + `just install` flow). If the README currently has no install section, add one near the top. Reference: spec § "Build & Release: README Distribution Section".
- [x] T008 [P] Rewrite `docs/specs/build-and-release.md` per spec § "Build & Release: Spec File Update" — update the header scope note (drop `.goreleaser.yaml`), drop the cobra `repo version` subcommand row from the Version Reporting table (keep only `--version`/`-v`), replace the entire `.goreleaser.yaml` block with a description of the run-kit-mirroring workflow shape, replace the `.github/workflows/release.yml` block with the new tag-driven content, update the Initial Release section (no VERSION file), update Release-pipeline behavioral scenarios (`just release patch` instead of `git tag -a ... && git push`; `4 tar.gz archives` instead of `4 binary archives + checksums.txt`), flip Design Decisions #1 and #3, strengthen #7 from "deferred" to "not in scope".
- [x] T009 Verify acceptance criteria #1, #2, #3, #4 (build & version-printing) by hand-running the relevant commands in a scratch shell, after T003–T006 complete. Specifically: (a) `cd src && go build ./...` passes; (b) `just build && ./bin/repo --version` prints `repo version <git-describe-output>`; (c) `cd src && go build -o /tmp/repo-noflags ./cmd/repo && /tmp/repo-noflags --version` prints `repo version dev`; (d) `cd src && go build -ldflags "-X main.version=v0.0.1" -o /tmp/repo-flags ./cmd/repo && /tmp/repo-flags --version` prints `repo version v0.0.1`.
- [x] T010 Verify acceptance criterion #5 (`scripts/release.sh` behavior) — run each negative case (`scripts/release.sh foo`, `scripts/release.sh patch minor`, `scripts/release.sh patch` on a dirty tree) and confirm exit codes / messages. Run the positive case `scripts/release.sh patch` on a clean tree (it will create a tag locally), then immediately delete the tag via `git tag -d v<created>`. **Do not push during apply.**
- [x] T011 Verify acceptance criterion #6 (workflow YAML lints) — run `act -l -W .github/workflows/release.yml` (or `actionlint .github/workflows/release.yml` if available) to catch syntax errors. If neither tool is installed, fall back to a manual readthrough plus `python3 -c 'import yaml; yaml.safe_load(open(".github/workflows/release.yml"))'` for YAML well-formedness.
- [x] T012 Verify acceptance criterion #7 (formula syntax) — `ruby -c .github/formula-template.rb` exits 0. Note: this only verifies Ruby syntax, not the post-`sed` substituted formula. Ad-hoc validation of a substituted formula by running the `sed` command manually with sample values is in scope here as a smoke test.
- [x] T013 Verify acceptance criterion #8 (README order) — re-read `README.md` post-T007 and confirm the install-method ordering matches the spec.
- [x] T014 Verify acceptance criterion #9 (spec rewrite consistency) — re-read `docs/specs/build-and-release.md` post-T008 and grep for any stale references (`goreleaser`, `VERSION file`, `checksums.txt`, the cobra `repo version` subcommand row in the Version Reporting table). Each of those terms should appear zero times in the rewritten spec (except where the spec itself describes what was rejected, in Design Decision #1).

## Phase 4: Polish

<!-- No Phase 4 tasks. Polish for this change is the spec rewrite (T008) and runbook documentation (T007) which are core to the deliverable, not separable polish. -->

---

## Execution Order

- T001, T002 are research/preparation — both [P], independent of each other and of all later tasks. Run first.
- T003 (release.sh) is independent of T001/T002 and can run in parallel with them, but is foundational for T010, so should complete before T010.
- T004 (workflow.yml) depends on T001 (action SHAs) and T005 (formula template path/placeholder names must match what the workflow's `sed` step references). T004 cannot start until both T001 and T005 are complete.
- T005 (formula template) depends on T002 (run-kit field-set survey) but is otherwise independent — [P] with T003 and T006.
- T006 (justfile) is independent of all other Core tasks — [P].
- T007, T008 are doc-only and independent of all Core tasks — both [P].
- T009 depends on T003 + T006 (build script and justfile must exist for `just build`).
- T010 depends on T003 (the script must exist).
- T011 depends on T004 (the workflow must exist).
- T012 depends on T005 (the formula must exist).
- T013 depends on T007 (the README must be updated).
- T014 depends on T008 (the spec must be rewritten).

The natural parallel groupings:
- **Group A (Phase 1 prep)**: T001, T002 — both [P], no inter-dependency.
- **Group B (Phase 2 core)**: T003, T005, T006 — all [P]. T004 depends on T001 + T005; run after Group A and after T005.
- **Group C (Phase 3 docs)**: T007, T008 — both [P], independent of Group B's outcomes since they reference the spec, not the implemented files.
- **Group D (Phase 3 verifications)**: T009–T014 — each depends on its corresponding implementation task (per the dependency list above). T009/T010/T011/T012 can run in parallel after Group B + T004 complete; T013/T014 can run in parallel after Group C completes.
