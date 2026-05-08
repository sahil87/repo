package main

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

const completionYAML = `repos:
  default:
    dir: /tmp/test-completion
    urls:
      - git@github.com:sahil87/alpha.git
      - git@github.com:sahil87/beta.git
      - git@github.com:bootstrap/dotfiles.git
`

// TestCompletionListsRepoNames exercises cobra's hidden __complete command,
// which is what the zsh completion script invokes at runtime. This is the
// integration-level test: if this passes, tab-completion works end-to-end.
func TestCompletionListsRepoNames(t *testing.T) {
	writeReposFixture(t, completionYAML)

	stdout, _, err := runArgs(t, cobra.ShellCompRequestCmd, "")
	if err != nil {
		t.Fatalf("__complete: %v", err)
	}
	out := stdout.String()
	for _, want := range []string{"alpha", "beta", "dotfiles"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in completion output, got:\n%s", want, out)
		}
	}
}

func TestCompletionReturnsAllNamesForShellFiltering(t *testing.T) {
	writeReposFixture(t, completionYAML)

	// Cobra's __complete returns the full candidate list and lets the shell
	// filter against toComplete (cobra prefix-matches in the generated
	// script, not server-side). So even with a "dotfil" prefix we expect all
	// names to come back — the shell narrows the visible set.
	stdout, _, err := runArgs(t, cobra.ShellCompRequestCmd, "dotfil")
	if err != nil {
		t.Fatalf("__complete dotfil: %v", err)
	}
	out := stdout.String()
	for _, want := range []string{"alpha", "beta", "dotfiles"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestCompletionForSubcommands(t *testing.T) {
	writeReposFixture(t, completionYAML)

	// `clone` shares completeRepoNames via completeCloneArg (which delegates to
	// completeRepoNames for non-URL prefixes). Verify it surfaces repo names.
	// `where` and `cd` were removed as top-level subcommands in the repo-verb
	// grammar flip — they're now $2 verbs, not $1 subcommands; tab completion
	// at $2 is punted to a follow-up.
	stdout, _, err := runArgs(t, cobra.ShellCompRequestCmd, "clone", "")
	if err != nil {
		t.Fatalf("__complete clone: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "alpha") {
		t.Fatalf("expected 'alpha' in `clone` completion, got:\n%s", out)
	}
}

func TestCompletionCloneSuppressesOnURLPrefix(t *testing.T) {
	writeReposFixture(t, completionYAML)

	// When the user has typed a URL-shaped prefix, repo-name completion is
	// suppressed (no candidates). This matches `clone <url>` semantics.
	stdout, _, err := runArgs(t, cobra.ShellCompRequestCmd, "clone", "git@github.com:foo/bar")
	if err != nil {
		t.Fatalf("__complete clone <url>: %v", err)
	}
	out := stdout.String()
	if strings.Contains(out, "alpha") || strings.Contains(out, "dotfiles") {
		t.Fatalf("expected no repo names for URL-shaped prefix, got:\n%s", out)
	}
}

func TestCompletionMissingConfigIsSilent(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("HOP_CONFIG", "/this/does/not/exist.yaml")

	// A missing config must NOT surface an error during completion AND must
	// not leak any positional candidates — tab-completion errors are
	// user-hostile and stray candidates would confuse autocompletion.
	// Use a subcommand (`clone`) so there is no subcommand-list output to
	// confound the candidate-count assertion.
	stdout, _, err := runArgs(t, cobra.ShellCompRequestCmd, "clone", "")
	if err != nil {
		t.Fatalf("__complete with missing config returned error: %v", err)
	}
	if got := candidatesFrom(stdout.String()); len(got) > 0 {
		t.Fatalf("expected no candidates for missing config, got: %v", got)
	}
}

// candidatesFrom parses cobra's __complete stdout and returns the bare
// (non-subcommand) candidate names — lines with no tab character. Cobra
// emits subcommand entries as "name\t<short>"; ValidArgsFunction candidates
// (our repo names) are emitted bare with no description, so a tab-free line
// is the signal of a positional candidate. The trailing `:<int>` directive
// line and human-readable summary are stripped.
func candidatesFrom(out string) []string {
	var candidates []string
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if line == "" || strings.HasPrefix(line, ":") || strings.HasPrefix(line, "Completion ended") {
			continue
		}
		if strings.Contains(line, "\t") {
			continue
		}
		candidates = append(candidates, line)
	}
	return candidates
}

func TestCompletionRootFiltersSubcommandCollisions(t *testing.T) {
	// A repo named after a subcommand can never reach the bare-form resolver
	// — cobra dispatches the first token to the subcommand. Advertising it
	// from the root would be misleading.
	//
	// Note: `where` and `cd` are NOT in this filter list anymore — they were
	// removed as top-level subcommands in the repo-verb grammar flip. A repo
	// named `where` or `cd` now surfaces from root completion (and cobra
	// routes `hop where` / `hop cd` to the new $1=repo, $2=anything-else
	// dispatch in the root's RunE, where `where` and `cd` as $1 fall into the
	// 1-arg bare-name case).
	writeReposFixture(t, `repos:
  default:
    dir: /tmp/test-collision
    urls:
      - git@github.com:sahil87/alpha.git
      - git@github.com:sahil87/clone.git
`)

	stdout, _, err := runArgs(t, cobra.ShellCompRequestCmd, "")
	if err != nil {
		t.Fatalf("__complete: %v", err)
	}
	bare := candidatesFrom(stdout.String())
	var foundAlpha bool
	for _, c := range bare {
		if c == "alpha" {
			foundAlpha = true
		}
		if c == "clone" {
			t.Fatalf("expected colliding name %q to be filtered from root completion candidates, got: %v", c, bare)
		}
	}
	if !foundAlpha {
		t.Fatalf("expected non-colliding 'alpha' in candidates, got: %v", bare)
	}
}

// TestCompletionRootSurfacesRepoNamedWhereOrCd asserts that since the `hop where`
// and `hop cd` subcommands were removed (replaced by the $2-verb grammar), repos
// named `where` or `cd` are no longer filtered from root completion. Guards
// against accidental re-introduction of these as subcommands or a hardcoded
// filter entry.
func TestCompletionRootSurfacesRepoNamedWhereOrCd(t *testing.T) {
	writeReposFixture(t, `repos:
  default:
    dir: /tmp/test-where-cd-uncollided
    urls:
      - git@github.com:sahil87/alpha.git
      - git@github.com:sahil87/where.git
      - git@github.com:sahil87/cd.git
`)

	stdout, _, err := runArgs(t, cobra.ShellCompRequestCmd, "")
	if err != nil {
		t.Fatalf("__complete: %v", err)
	}
	bare := candidatesFrom(stdout.String())
	foundWhere, foundCd := false, false
	for _, c := range bare {
		if c == "where" {
			foundWhere = true
		}
		if c == "cd" {
			foundCd = true
		}
	}
	if !foundWhere {
		t.Errorf("expected `where` repo to surface (cd/where are no longer subcommands), got: %v", bare)
	}
	if !foundCd {
		t.Errorf("expected `cd` repo to surface, got: %v", bare)
	}
}

// TestCompletionRootSurfacesRepoNamedCode asserts that since the `hop code`
// subcommand was removed (replaced by the shim's tool-form `hop code <repo>`),
// a repo named `code` is no longer filtered from root completion. Before
// removal, `code` would have been suppressed by completeRepoNames' subcommand
// collision filter; this test guards against accidental re-introduction of
// `code` as a subcommand or hard-coded filter entry.
func TestCompletionRootSurfacesRepoNamedCode(t *testing.T) {
	writeReposFixture(t, `repos:
  default:
    dir: /tmp/test-code-uncollided
    urls:
      - git@github.com:sahil87/alpha.git
      - git@github.com:sahil87/code.git
`)

	stdout, _, err := runArgs(t, cobra.ShellCompRequestCmd, "")
	if err != nil {
		t.Fatalf("__complete: %v", err)
	}
	bare := candidatesFrom(stdout.String())
	var foundCode bool
	for _, c := range bare {
		if c == "code" {
			foundCode = true
			break
		}
	}
	if !foundCode {
		t.Fatalf("expected 'code' in root completion candidates after subcommand removal, got: %v", bare)
	}
}

// TestCompletionPullSurfacesReposAndGroups exercises the new
// `completeRepoOrGroupNames` helper used by `hop pull` and `hop sync`. The
// helper returns both group names and repo names, deduplicated. We invoke the
// hidden __complete entry for `pull` and assert both kinds appear.
func TestCompletionPullSurfacesReposAndGroups(t *testing.T) {
	writeReposFixture(t, `repos:
  default:
    dir: /tmp/test-pull-completion-default
    urls:
      - git@github.com:sahil87/alpha.git
      - git@github.com:sahil87/beta.git
  vendor:
    dir: /tmp/test-pull-completion-vendor
    urls:
      - git@github.com:vendor/gamma.git
`)

	stdout, _, err := runArgs(t, cobra.ShellCompRequestCmd, "pull", "")
	if err != nil {
		t.Fatalf("__complete pull: %v", err)
	}
	out := stdout.String()
	for _, want := range []string{"default", "vendor", "alpha", "beta", "gamma"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in pull completion, got:\n%s", want, out)
		}
	}
}

// TestCompletionPullDeduplicatesGroupAndRepoCollisions asserts that when a
// name appears as BOTH a group and a repo (the rare collision case), it is
// emitted once. Group entries are listed first, so the repo with the same
// name is suppressed by the dedup pass.
func TestCompletionPullDeduplicatesGroupAndRepoCollisions(t *testing.T) {
	// "outbox" is both a group AND a repo name (in another group).
	writeReposFixture(t, `repos:
  outbox:
    dir: /tmp/test-pull-collision-outbox
    urls:
      - git@github.com:org/foo.git
  vendor:
    dir: /tmp/test-pull-collision-vendor
    urls:
      - git@github.com:vendor/outbox.git
`)

	stdout, _, err := runArgs(t, cobra.ShellCompRequestCmd, "pull", "")
	if err != nil {
		t.Fatalf("__complete pull: %v", err)
	}
	cands := candidatesFrom(stdout.String())
	count := 0
	for _, c := range cands {
		if c == "outbox" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected `outbox` once (dedup), got %d times in candidates: %v", count, cands)
	}
}

// TestCompletionPullSuppressesAfterPositional asserts that once the user has
// already typed one positional argument, the completion returns no further
// candidates (cobra's MaximumNArgs(1) means a second positional is invalid).
func TestCompletionPullSuppressesAfterPositional(t *testing.T) {
	writeReposFixture(t, completionYAML)

	stdout, _, err := runArgs(t, cobra.ShellCompRequestCmd, "pull", "alpha", "")
	if err != nil {
		t.Fatalf("__complete pull alpha: %v", err)
	}
	if got := candidatesFrom(stdout.String()); len(got) > 0 {
		t.Fatalf("expected no candidates after first positional, got: %v", got)
	}
}

// TestCompletionSyncSurfacesReposAndGroups mirrors the pull-completion test
// for sync — same helper, same expectation.
func TestCompletionSyncSurfacesReposAndGroups(t *testing.T) {
	writeReposFixture(t, `repos:
  default:
    dir: /tmp/test-sync-completion
    urls:
      - git@github.com:sahil87/alpha.git
`)

	stdout, _, err := runArgs(t, cobra.ShellCompRequestCmd, "sync", "")
	if err != nil {
		t.Fatalf("__complete sync: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "default") {
		t.Errorf("expected `default` in sync completion, got:\n%s", out)
	}
	if !strings.Contains(out, "alpha") {
		t.Errorf("expected `alpha` in sync completion, got:\n%s", out)
	}
}

// TestCompletionVerbPositionListsCdWhereOpen exercises completion at the $2
// verb position: `hop <name> <TAB>` should surface `cd`, `where`, and `open`.
// These are the recognized verbs at args[1] in the root command's RunE.
func TestCompletionVerbPositionListsCdWhereOpen(t *testing.T) {
	writeReposFixture(t, completionYAML)

	stdout, _, err := runArgs(t, cobra.ShellCompRequestCmd, "alpha", "")
	if err != nil {
		t.Fatalf("__complete alpha: %v", err)
	}
	cands := candidatesFrom(stdout.String())
	wantSet := map[string]bool{"cd": false, "where": false, "open": false}
	for _, c := range cands {
		if _, ok := wantSet[c]; ok {
			wantSet[c] = true
		}
	}
	for verb, found := range wantSet {
		if !found {
			t.Errorf("expected verb %q in candidates at $2 position, got: %v", verb, cands)
		}
	}
}

// TestCompletionVerbPositionDoesNotListRepoNames asserts that at the $2 position
// the completion returns ONLY the verb set, not repo names. Without this guard,
// a regression that returned repos at $2 would advertise nonsensical
// `hop outbox dotfiles` style commands.
func TestCompletionVerbPositionDoesNotListRepoNames(t *testing.T) {
	writeReposFixture(t, completionYAML)

	stdout, _, err := runArgs(t, cobra.ShellCompRequestCmd, "alpha", "")
	if err != nil {
		t.Fatalf("__complete alpha: %v", err)
	}
	cands := candidatesFrom(stdout.String())
	for _, c := range cands {
		if c == "alpha" || c == "beta" || c == "dotfiles" {
			t.Fatalf("expected no repo names at $2 position, got %q in candidates: %v", c, cands)
		}
	}
}

// TestCompletionThirdPositionalSuppressed asserts that beyond $2 there are no
// candidates. cobra's MaximumNArgs(2) would reject any further positional, so
// suggesting candidates would be misleading.
func TestCompletionThirdPositionalSuppressed(t *testing.T) {
	writeReposFixture(t, completionYAML)

	stdout, _, err := runArgs(t, cobra.ShellCompRequestCmd, "alpha", "open", "")
	if err != nil {
		t.Fatalf("__complete alpha open: %v", err)
	}
	if got := candidatesFrom(stdout.String()); len(got) > 0 {
		t.Fatalf("expected no candidates at position 3, got: %v", got)
	}
}

// TestCompletionCloneSuppressesAfterPositional asserts that `hop clone <name>
// <TAB>` returns no candidates. clone delegates to completeRepoNames via
// completeCloneArg, but the repo-verb candidates (cd/where/open) only apply to
// the root command's $2 — clone has at most one positional, so a second
// argument should suggest nothing rather than the verb set.
func TestCompletionCloneSuppressesAfterPositional(t *testing.T) {
	writeReposFixture(t, completionYAML)

	stdout, _, err := runArgs(t, cobra.ShellCompRequestCmd, "clone", "alpha", "")
	if err != nil {
		t.Fatalf("__complete clone alpha: %v", err)
	}
	if got := candidatesFrom(stdout.String()); len(got) > 0 {
		t.Fatalf("expected no candidates after first positional on clone, got: %v", got)
	}
}

func TestCompletionCloneSuppressesOnAllFlag(t *testing.T) {
	writeReposFixture(t, completionYAML)

	// `clone --all` ignores any positional argument, so completion should
	// not advertise repo names once --all is set.
	stdout, _, err := runArgs(t, cobra.ShellCompRequestCmd, "clone", "--all", "")
	if err != nil {
		t.Fatalf("__complete clone --all: %v", err)
	}
	if got := candidatesFrom(stdout.String()); len(got) > 0 {
		t.Fatalf("expected no candidates after --all, got: %v", got)
	}
}
