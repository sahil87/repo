package main

import (
	"strings"
	"testing"
)

// fixture used by several tests; ~ is expanded to $HOME at load time, but we use
// absolute dirs here to keep assertions simple.
const singleRepoYAML = `/tmp/test-repos:
  - git@github.com:sahil87/repo.git
`

func TestPathExactMatch(t *testing.T) {
	writeReposFixture(t, singleRepoYAML)

	stdout, _, err := runArgs(t, "path", "repo")
	if err != nil {
		t.Fatalf("path repo: %v", err)
	}
	got := strings.TrimSpace(stdout.String())
	if got != "/tmp/test-repos/repo" {
		t.Fatalf("expected /tmp/test-repos/repo, got %q", got)
	}
}

func TestBareSingleArgDelegatesToPath(t *testing.T) {
	writeReposFixture(t, singleRepoYAML)

	stdout, _, err := runArgs(t, "repo")
	if err != nil {
		t.Fatalf("bare repo: %v", err)
	}
	// With one repo and a single positional that uniquely matches, the bare form
	// short-circuits fzf and prints the path.
	got := strings.TrimSpace(stdout.String())
	if got != "/tmp/test-repos/repo" {
		t.Fatalf("expected /tmp/test-repos/repo, got %q", got)
	}
}

func TestPathRequiresArg(t *testing.T) {
	writeReposFixture(t, singleRepoYAML)

	_, _, err := runArgs(t, "path")
	if err == nil {
		t.Fatalf("expected error from `path` with no args")
	}
}

func TestPathConfigMissingError(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("REPOS_YAML", "/this/does/not/exist.yaml")

	_, _, err := runArgs(t, "path", "repo")
	if err == nil {
		t.Fatalf("expected error for missing $REPOS_YAML target")
	}
	if !strings.Contains(err.Error(), "$REPOS_YAML points to") {
		t.Fatalf("expected hard-error message, got %q", err.Error())
	}
}
