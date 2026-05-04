package main

import (
	"strings"
	"testing"
)

const lsYAML = `repos:
  default:
    dir: /tmp/test-ls
    urls:
      - git@github.com:sahil87/alpha.git
      - git@github.com:sahil87/beta.git
`

func TestLsListsAllRepos(t *testing.T) {
	writeReposFixture(t, lsYAML)

	stdout, _, err := runArgs(t, "ls")
	if err != nil {
		t.Fatalf("ls: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "alpha") || !strings.Contains(out, "beta") {
		t.Fatalf("expected alpha and beta in output, got %q", out)
	}
	if !strings.Contains(out, "/tmp/test-ls/alpha") {
		t.Fatalf("expected path in output, got %q", out)
	}
}

func TestLsEmptyConfig(t *testing.T) {
	writeReposFixture(t, "")

	stdout, _, err := runArgs(t, "ls")
	if err != nil {
		t.Fatalf("ls (empty): %v", err)
	}
	if got := strings.TrimSpace(stdout.String()); got != "" {
		t.Fatalf("expected empty output, got %q", got)
	}
}
