package main

import (
	"strings"
	"testing"
)

func TestCodeMissingTool(t *testing.T) {
	writeReposFixture(t, singleRepoYAML)
	// Hide `code` binary by clearing PATH for this test; the resolver succeeds
	// (single match) but the proc.Run("code") returns ErrNotFound.
	t.Setenv("PATH", "/nonexistent")

	_, stderr, err := runArgs(t, "code", "repo")
	if err == nil {
		t.Fatalf("expected error when 'code' is missing")
	}
	if !strings.Contains(stderr.String(), codeMissingHint) {
		t.Fatalf("expected stderr to contain code-missing hint, got %q", stderr.String())
	}
}
