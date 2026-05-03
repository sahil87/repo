package main

import (
	"runtime"
	"strings"
	"testing"
)

func TestOpenMissingTool(t *testing.T) {
	writeReposFixture(t, singleRepoYAML)
	t.Setenv("PATH", "/nonexistent")

	_, stderr, err := runArgs(t, "open", "repo")
	if err == nil {
		t.Fatalf("expected error when open/xdg-open is missing")
	}
	tool := "xdg-open"
	if runtime.GOOS == "darwin" {
		tool = "open"
	}
	want := "repo open: '" + tool + "' not found."
	if !strings.Contains(stderr.String(), want) {
		t.Fatalf("expected stderr to contain %q (single-quoted tool name per cli-surface.md §External Tool Availability), got %q", want, stderr.String())
	}
}
