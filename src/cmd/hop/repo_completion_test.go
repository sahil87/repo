package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/sahil87/hop/internal/proc"
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

// makeCompletionFixture builds a hop.yaml + on-disk repo named `name` and
// returns the resolved repo path.
func makeCompletionFixture(t *testing.T, name string, cloned bool) string {
	t.Helper()
	parent := t.TempDir()
	repoDir := filepath.Join(parent, name)
	if cloned {
		if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	yaml := fmt.Sprintf(`repos:
  default:
    dir: %s
    urls:
      - git@github.com:sahil87/%s.git
`, parent, name)
	writeReposFixture(t, yaml)
	return repoDir
}

// TestCompletionWorktreeSlashOffersCandidates exercises the new worktree
// prefix branch. With `toComplete = "outbox/"`, completion must surface each
// worktree name prefixed with `outbox/` so cobra's prefix-match-on-toComplete
// substitutes correctly.
func TestCompletionWorktreeSlashOffersCandidates(t *testing.T) {
	repoDir := makeCompletionFixture(t, "outbox", true)
	withListWorktrees(t, func(ctx context.Context, repoPath string) ([]WtEntry, error) {
		if repoPath != repoDir {
			t.Errorf("listWorktrees called with %q, want %q", repoPath, repoDir)
		}
		return []WtEntry{
			{Name: "main", Path: repoDir, IsMain: true},
			{Name: "feat-x", Path: repoDir + ".worktrees/feat-x"},
			{Name: "hotfix", Path: repoDir + ".worktrees/hotfix"},
		}, nil
	})

	stdout, _, err := runArgs(t, cobra.ShellCompRequestCmd, "outbox/")
	if err != nil {
		t.Fatalf("__complete outbox/: %v", err)
	}
	cands := candidatesFrom(stdout.String())
	want := []string{"outbox/main", "outbox/feat-x", "outbox/hotfix"}
	for _, w := range want {
		found := false
		for _, c := range cands {
			if c == w {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing candidate %q in %v", w, cands)
		}
	}
}

// TestCompletionWorktreePartialAfterSlash verifies the prefix branch fires
// for `outbox/feat` (partial wt name), returning the full set prefixed —
// cobra's prefix-match narrows the visible list.
func TestCompletionWorktreePartialAfterSlash(t *testing.T) {
	repoDir := makeCompletionFixture(t, "outbox", true)
	withListWorktrees(t, func(ctx context.Context, repoPath string) ([]WtEntry, error) {
		return []WtEntry{
			{Name: "main", Path: repoDir},
			{Name: "feat-x", Path: repoDir + ".worktrees/feat-x"},
			{Name: "feat-y", Path: repoDir + ".worktrees/feat-y"},
		}, nil
	})

	stdout, _, err := runArgs(t, cobra.ShellCompRequestCmd, "outbox/feat")
	if err != nil {
		t.Fatalf("__complete outbox/feat: %v", err)
	}
	cands := candidatesFrom(stdout.String())
	for _, want := range []string{"outbox/feat-x", "outbox/feat-y"} {
		found := false
		for _, c := range cands {
			if c == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing candidate %q in %v", want, cands)
		}
	}
}

// TestCompletionWorktreeUnclonedSilent verifies that a `/`-prefixed completion
// against an uncloned repo returns no candidates and writes nothing to stderr.
func TestCompletionWorktreeUnclonedSilent(t *testing.T) {
	makeCompletionFixture(t, "loom", false)
	withListWorktrees(t, func(ctx context.Context, repoPath string) ([]WtEntry, error) {
		t.Fatalf("listWorktrees called against uncloned repo; want NOT called")
		return nil, nil
	})

	stdout, stderr, err := runArgs(t, cobra.ShellCompRequestCmd, "loom/")
	if err != nil {
		t.Fatalf("__complete loom/: %v", err)
	}
	if got := candidatesFrom(stdout.String()); len(got) > 0 {
		t.Errorf("expected no candidates for uncloned repo, got %v", got)
	}
	// Cobra's __complete always writes a "Completion ended with directive:"
	// line to stderr — that's expected output, not a hop-level error. We just
	// want to assert hop itself did not emit any error wording.
	if strings.Contains(stderr.String(), "hop:") {
		t.Errorf("expected no hop-level stderr output, got %q", stderr.String())
	}
}

// TestCompletionWorktreeMissingWtSilent verifies that when wt is not on PATH,
// `/`-prefix completion returns no candidates silently — no `not found`
// stderr line during TAB.
func TestCompletionWorktreeMissingWtSilent(t *testing.T) {
	makeCompletionFixture(t, "outbox", true)
	withListWorktrees(t, func(ctx context.Context, repoPath string) ([]WtEntry, error) {
		return nil, proc.ErrNotFound
	})

	stdout, stderr, err := runArgs(t, cobra.ShellCompRequestCmd, "outbox/")
	if err != nil {
		t.Fatalf("__complete outbox/: %v", err)
	}
	if got := candidatesFrom(stdout.String()); len(got) > 0 {
		t.Errorf("expected no candidates when wt missing, got %v", got)
	}
	if strings.Contains(stderr.String(), "wt") {
		t.Errorf("expected silent failure (no `wt` mention on stderr), got %q", stderr.String())
	}
}

// TestCompletionVerbPositionUnaffectedByWorktreeBranch verifies the existing
// verb-position completion (`hop <name> <TAB>` → cd/where/open) is unchanged
// by the worktree-prefix branch — the `/`-detection only operates on args[0]
// / the toComplete slot.
func TestCompletionVerbPositionUnaffectedByWorktreeBranch(t *testing.T) {
	makeCompletionFixture(t, "outbox", true)
	withListWorktrees(t, func(ctx context.Context, repoPath string) ([]WtEntry, error) {
		t.Fatalf("listWorktrees called at $2 verb position; want NOT called")
		return nil, nil
	})

	stdout, _, err := runArgs(t, cobra.ShellCompRequestCmd, "outbox", "")
	if err != nil {
		t.Fatalf("__complete outbox <empty>: %v", err)
	}
	cands := candidatesFrom(stdout.String())
	wantVerbs := map[string]bool{"cd": false, "where": false, "open": false}
	for _, c := range cands {
		if _, ok := wantVerbs[c]; ok {
			wantVerbs[c] = true
		}
	}
	for verb, found := range wantVerbs {
		if !found {
			t.Errorf("expected verb %q at $2, got: %v", verb, cands)
		}
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

// TestCompletionEagerWorktreeExpansion exercises the pre-slash eager-expansion
// branch added to `completeRepoNames`. The five cases below match the spec's
// decision matrix in change 260516-odle-eager-worktree-completion: when a
// unique cloned repo has >=2 worktrees, the candidate list gains worktree
// suffixes and the directive flips on NoSpace. All other shapes fall back to
// today's behavior.
func TestCompletionEagerWorktreeExpansion(t *testing.T) {
	cases := []struct {
		name           string
		toComplete     string
		repos          []string // names to seed in hop.yaml (single group "default")
		clonedRepos    []string // subset of repos with a `.git` dir
		wtEntries      []WtEntry
		wtErr          error
		wtNotCalled    bool // if true, listWorktrees must NOT be invoked
		wantCandidates []string
		wantDirective  cobra.ShellCompDirective
	}{
		{
			name:           "a unique match with 1 worktree falls back",
			toComplete:     "outb",
			repos:          []string{"outbox"},
			clonedRepos:    []string{"outbox"},
			wtEntries:      []WtEntry{{Name: "main", IsMain: true}},
			wantCandidates: []string{"outbox"},
			wantDirective:  cobra.ShellCompDirectiveNoFileComp,
		},
		{
			name:        "b unique match with multiple worktrees fires eager expansion",
			toComplete:  "outb",
			repos:       []string{"outbox"},
			clonedRepos: []string{"outbox"},
			wtEntries: []WtEntry{
				{Name: "main", IsMain: true},
				{Name: "feat-x"},
				{Name: "bugfix-y"},
			},
			wantCandidates: []string{"outbox", "outbox/main", "outbox/feat-x", "outbox/bugfix-y"},
			wantDirective:  cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace,
		},
		{
			name:           "c unique match uncloned falls back silently",
			toComplete:     "outb",
			repos:          []string{"outbox"},
			clonedRepos:    nil, // uncloned
			wtNotCalled:    true,
			wantCandidates: []string{"outbox"},
			wantDirective:  cobra.ShellCompDirectiveNoFileComp,
		},
		{
			name:           "d unique match with listWorktrees error falls back silently",
			toComplete:     "outb",
			repos:          []string{"outbox"},
			clonedRepos:    []string{"outbox"},
			wtErr:          errors.New("wt list: malformed JSON"),
			wantCandidates: []string{"outbox"},
			wantDirective:  cobra.ShellCompDirectiveNoFileComp,
		},
		{
			name:           "e ambiguous prefix bypasses eager branch",
			toComplete:     "co",
			repos:          []string{"code", "colors", "coredns"},
			clonedRepos:    []string{"code", "colors", "coredns"}, // would fire if guard absent
			wtNotCalled:    true,
			wantCandidates: []string{"code", "colors", "coredns"},
			wantDirective:  cobra.ShellCompDirectiveNoFileComp,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			parent := t.TempDir()
			cloned := make(map[string]bool, len(tc.clonedRepos))
			for _, n := range tc.clonedRepos {
				cloned[n] = true
			}
			var urls strings.Builder
			for _, n := range tc.repos {
				fmt.Fprintf(&urls, "      - git@github.com:sahil87/%s.git\n", n)
				if cloned[n] {
					if err := os.MkdirAll(filepath.Join(parent, n, ".git"), 0o755); err != nil {
						t.Fatalf("mkdir %s/.git: %v", n, err)
					}
				}
			}
			yaml := fmt.Sprintf("repos:\n  default:\n    dir: %s\n    urls:\n%s", parent, urls.String())
			writeReposFixture(t, yaml)

			called := false
			withListWorktrees(t, func(ctx context.Context, repoPath string) ([]WtEntry, error) {
				called = true
				if tc.wtNotCalled {
					t.Fatalf("listWorktrees called unexpectedly with %q", repoPath)
				}
				return tc.wtEntries, tc.wtErr
			})

			gotCands, gotDir := completeRepoNames(newRootCmd(), nil, tc.toComplete)
			if tc.wtNotCalled && called {
				t.Fatalf("listWorktrees was invoked but should not have been")
			}
			if !reflect.DeepEqual(gotCands, tc.wantCandidates) {
				t.Fatalf("candidates mismatch:\n  got:  %v\n  want: %v", gotCands, tc.wantCandidates)
			}
			if gotDir != tc.wantDirective {
				t.Fatalf("directive mismatch: got %d, want %d", gotDir, tc.wantDirective)
			}
		})
	}
}
