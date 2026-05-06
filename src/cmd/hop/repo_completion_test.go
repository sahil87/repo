package main

import (
	"os/exec"
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

	// where, cd, open, clone all share completeRepoNames; verify each surfaces
	// repo names. clone uses completeCloneArg, which delegates to
	// completeRepoNames for non-URL prefixes.
	for _, sub := range []string{"where", "cd", "open", "clone"} {
		stdout, _, err := runArgs(t, cobra.ShellCompRequestCmd, sub, "")
		if err != nil {
			t.Fatalf("__complete %s: %v", sub, err)
		}
		out := stdout.String()
		if !strings.Contains(out, "alpha") {
			t.Fatalf("expected 'alpha' in `%s` completion, got:\n%s", sub, out)
		}
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
	// Use a subcommand (`where`) so there is no subcommand-list output to
	// confound the candidate-count assertion.
	stdout, _, err := runArgs(t, cobra.ShellCompRequestCmd, "where", "")
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
	writeReposFixture(t, `repos:
  default:
    dir: /tmp/test-collision
    urls:
      - git@github.com:sahil87/alpha.git
      - git@github.com:sahil87/clone.git
      - git@github.com:sahil87/where.git
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
		if c == "clone" || c == "where" {
			t.Fatalf("expected colliding name %q to be filtered from root completion candidates, got: %v", c, bare)
		}
	}
	if !foundAlpha {
		t.Fatalf("expected non-colliding 'alpha' in candidates, got: %v", bare)
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

// fakeToolOnPATH is a binary name that is virtually guaranteed to NOT exist
// on PATH on any developer machine or CI runner. Used for "tool missing"
// fixtures in the tool-form completion tests.
const fakeToolOnPATH = "hop-nonexistent-tool-xyzzy"

// TestShouldCompleteRepoForSecondArg unit-tests the shape detection helper
// directly with a fresh root command. This is the unit-level counterpart to
// the end-to-end tests below — a regression here pinpoints the issue without
// the indirection of cobra's __complete dispatch. Covers the tool-form
// branch only; `-R` completion is handled by cobra flag-completion via
// completeRepoNamesForFlag and is exercised separately by
// TestCompletionDashRReturnsRepoNames below.
func TestShouldCompleteRepoForSecondArg(t *testing.T) {
	cmd := newRootCmd()

	// args == [] → false. Only the len(args) == 1 shape is the `$2` slot.
	if shouldCompleteRepoForSecondArg(cmd, []string{}) {
		t.Errorf("shouldCompleteRepoForSecondArg(cmd, []) = true, want false")
	}

	// args == ["sh"] → true. sh is on PATH on every POSIX runner and is not
	// a hop subcommand. Skip if sh is somehow not on PATH (defensive).
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skipf("sh not on PATH; skipping tool-form positive case: %v", err)
	} else if !shouldCompleteRepoForSecondArg(cmd, []string{"sh"}) {
		t.Errorf("shouldCompleteRepoForSecondArg(cmd, [sh]) = false, want true")
	}

	// args == [fakeToolOnPATH] → false. Defensive guard: if the fake name
	// somehow resolves on this machine, skip rather than fail spuriously.
	if _, err := exec.LookPath(fakeToolOnPATH); err == nil {
		t.Skipf("%q unexpectedly on PATH; skipping tool-form negative case", fakeToolOnPATH)
	} else if shouldCompleteRepoForSecondArg(cmd, []string{fakeToolOnPATH}) {
		t.Errorf("shouldCompleteRepoForSecondArg(cmd, [%s]) = true, want false", fakeToolOnPATH)
	}

	// args == ["clone"] → false. clone is a hop subcommand; the subcommand
	// check fires before the PATH lookup. Verifies cobra introspection
	// rather than a hardcoded list.
	if shouldCompleteRepoForSecondArg(cmd, []string{"clone"}) {
		t.Errorf("shouldCompleteRepoForSecondArg(cmd, [clone]) = true, want false (subcommand wins)")
	}

	// args == ["sh", "name"] → false. len(args) != 1, so this is the third
	// position (child argv), not the repo slot.
	if shouldCompleteRepoForSecondArg(cmd, []string{"sh", "name"}) {
		t.Errorf("shouldCompleteRepoForSecondArg(cmd, [sh, name]) = true, want false")
	}
}

// TestCompletionDashRReturnsRepoNames is the end-to-end test for the
// `hop -R <TAB>` shape: cobra's __complete dispatcher routes the empty
// value-slot completion to completeRepoNamesForFlag (registered against
// the hidden `-R` flag in newRootCmd), which returns the repo names.
func TestCompletionDashRReturnsRepoNames(t *testing.T) {
	writeReposFixture(t, completionYAML)

	stdout, _, err := runArgs(t, cobra.ShellCompRequestCmd, "-R", "")
	if err != nil {
		t.Fatalf("__complete -R: %v", err)
	}
	out := stdout.String()
	for _, want := range []string{"alpha", "beta", "dotfiles"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in completion output, got:\n%s", want, out)
		}
	}
}

// TestCompletionDashRThirdPositionEmpty guards against accidentally
// completing repo names in the child-argv slot of `hop -R <name> <TAB>`.
// Cobra consumes `-R name` as a flag pair, so completeRepoNames is invoked
// with args=[] (looks like bare `$1`); the root R flag's Changed bit is the
// signal we use to suppress.
func TestCompletionDashRThirdPositionEmpty(t *testing.T) {
	writeReposFixture(t, completionYAML)

	stdout, _, err := runArgs(t, cobra.ShellCompRequestCmd, "-R", "name", "")
	if err != nil {
		t.Fatalf("__complete -R name: %v", err)
	}
	if got := candidatesFrom(stdout.String()); len(got) > 0 {
		t.Fatalf("expected no candidates at third position of -R, got: %v", got)
	}
}

// TestCompletionDashRSurfacesRepoNamedClone is the explicit guard for the
// design decision to NOT filter subcommand-named repos from -R completion:
// `hop -R clone <cmd>` is a valid invocation (run <cmd> in the repo named
// `clone`), so the completion should advertise it. Contrast with
// TestCompletionRootFiltersSubcommandCollisions for the bare-form `$1`
// case where the filter DOES apply.
func TestCompletionDashRSurfacesRepoNamedClone(t *testing.T) {
	writeReposFixture(t, `repos:
  default:
    dir: /tmp/test-dashr-clone
    urls:
      - git@github.com:sahil87/alpha.git
      - git@github.com:sahil87/clone.git
`)

	stdout, _, err := runArgs(t, cobra.ShellCompRequestCmd, "-R", "")
	if err != nil {
		t.Fatalf("__complete -R: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "clone") {
		t.Fatalf("expected 'clone' in -R completion (subcommand-named repos are valid -R targets), got:\n%s", out)
	}
}

// TestCompletionToolFormReturnsRepoNames verifies that `hop <tool> <TAB>`
// (tool-form sugar) completes repo names when <tool> is a binary on PATH
// and not a hop subcommand. Uses sh as the canonical PATH-resident binary.
func TestCompletionToolFormReturnsRepoNames(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skipf("sh not on PATH; skipping: %v", err)
	}
	writeReposFixture(t, completionYAML)

	stdout, _, err := runArgs(t, cobra.ShellCompRequestCmd, "sh", "")
	if err != nil {
		t.Fatalf("__complete sh: %v", err)
	}
	out := stdout.String()
	for _, want := range []string{"alpha", "beta", "dotfiles"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in completion output, got:\n%s", want, out)
		}
	}
}

// TestCompletionToolFormUnknownToolEmpty verifies that a name that does not
// resolve on PATH (and is not a subcommand) yields no candidates — the shim
// would print an error rather than dispatch as tool-form, so completion
// must not pretend otherwise.
func TestCompletionToolFormUnknownToolEmpty(t *testing.T) {
	if _, err := exec.LookPath(fakeToolOnPATH); err == nil {
		t.Skipf("%q unexpectedly on PATH; skipping", fakeToolOnPATH)
	}
	writeReposFixture(t, completionYAML)

	stdout, _, err := runArgs(t, cobra.ShellCompRequestCmd, fakeToolOnPATH, "")
	if err != nil {
		t.Fatalf("__complete %s: %v", fakeToolOnPATH, err)
	}
	if got := candidatesFrom(stdout.String()); len(got) > 0 {
		t.Fatalf("expected no candidates for unknown tool, got: %v", got)
	}
}

// TestCompletionToolFormThirdPositionEmpty guards against accidentally
// completing repo names in the child-argv slot of `hop <tool> <name> <TAB>`.
func TestCompletionToolFormThirdPositionEmpty(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skipf("sh not on PATH; skipping: %v", err)
	}
	writeReposFixture(t, completionYAML)

	stdout, _, err := runArgs(t, cobra.ShellCompRequestCmd, "sh", "name", "")
	if err != nil {
		t.Fatalf("__complete sh name: %v", err)
	}
	if got := candidatesFrom(stdout.String()); len(got) > 0 {
		t.Fatalf("expected no candidates at third position of tool-form, got: %v", got)
	}
}

// TestCompletionDashRThirdPositionSubcommandRouted is the regression guard
// for the case where cobra dispatches the post-`-R <name>` argv to a real
// subcommand (e.g. `hop -R alpha where <TAB>`). The hidden -R flag MUST be
// persistent so subcommands can still parse it; the suppression check in
// completeRepoNames MUST look at the root's flag (cmd.Root().Flag(...)) so
// the subcommand's invocation observes the Changed bit. Without these,
// cobra fails to parse `-R` at the subcommand level and falls back to file
// completion, leaking candidates for the child argv.
func TestCompletionDashRThirdPositionSubcommandRouted(t *testing.T) {
	writeReposFixture(t, completionYAML)

	stdout, _, err := runArgs(t, cobra.ShellCompRequestCmd, "-R", "alpha", "where", "")
	if err != nil {
		t.Fatalf("__complete -R alpha where: %v", err)
	}
	// `where` is a subcommand whose own ValidArgsFunction is
	// completeRepoNames. We expect zero candidates — the call originates
	// past `-R alpha`, the child argv slot. We use candidatesFrom (not
	// substring search) because `where`'s own completeRepoNames could
	// otherwise echo all repo names.
	if got := candidatesFrom(stdout.String()); len(got) > 0 {
		t.Fatalf("expected no candidates at child argv slot of `-R alpha where`, got: %v", got)
	}
}

// TestCompletionSubcommandSecondPositionalEmpty is the regression guard for
// the case where a non-root subcommand sees a 2nd positional (e.g.
// `hop where sh <TAB>`). `where` accepts at most 1 positional, so the
// completion should yield zero candidates. shouldCompleteRepoForSecondArg
// MUST gate on cmd.Parent() == nil to avoid leaking repo-name candidates
// into a subcommand's invalid argv slot.
func TestCompletionSubcommandSecondPositionalEmpty(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skipf("sh not on PATH; skipping: %v", err)
	}
	writeReposFixture(t, completionYAML)

	for _, sub := range []string{"where", "open", "cd"} {
		stdout, _, err := runArgs(t, cobra.ShellCompRequestCmd, sub, "sh", "")
		if err != nil {
			t.Fatalf("__complete %s sh: %v", sub, err)
		}
		if got := candidatesFrom(stdout.String()); len(got) > 0 {
			t.Fatalf("expected no candidates for `%s sh <TAB>` (subcommand has no 2nd positional), got: %v", sub, got)
		}
	}
}
