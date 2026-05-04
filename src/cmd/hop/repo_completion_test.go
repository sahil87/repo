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

	// where, cd, code, open, clone all share completeRepoNames; verify each
	// surfaces repo names. clone uses completeCloneArg, which delegates to
	// completeRepoNames for non-URL prefixes.
	for _, sub := range []string{"where", "cd", "code", "open", "clone"} {
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
