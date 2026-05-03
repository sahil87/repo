package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigInitWritesStarter(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "repos.yaml")
	t.Setenv("REPOS_YAML", target)

	stdout, _, err := runArgs(t, "config", "init")
	if err != nil {
		t.Fatalf("config init: %v", err)
	}
	if !strings.Contains(stdout.String(), "Created "+target) {
		t.Fatalf("expected 'Created <path>' on stdout, got %q", stdout.String())
	}

	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Fatalf("expected mode 0644, got %o", info.Mode().Perm())
	}
}

func TestConfigInitRefusesOverwrite(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "repos.yaml")
	if err := os.WriteFile(target, []byte("existing\n"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	t.Setenv("REPOS_YAML", target)

	_, _, err := runArgs(t, "config", "init")
	if err == nil {
		t.Fatalf("expected refusal, got nil")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected 'already exists' message, got %q", err.Error())
	}
}

func TestConfigPathPrintsResolvedPath(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "repos.yaml")
	t.Setenv("REPOS_YAML", target)

	stdout, _, err := runArgs(t, "config", "path")
	if err != nil {
		t.Fatalf("config path: %v", err)
	}
	if got := strings.TrimSpace(stdout.String()); got != target {
		t.Fatalf("expected %q, got %q", target, got)
	}
}

func TestConfigPathDoesNotErrorOnMissingFile(t *testing.T) {
	missing := "/tmp/no-such-file-xyz123.yaml"
	t.Setenv("REPOS_YAML", missing)

	stdout, _, err := runArgs(t, "config", "path")
	if err != nil {
		t.Fatalf("config path on missing file: %v", err)
	}
	if got := strings.TrimSpace(stdout.String()); got != missing {
		t.Fatalf("expected %q, got %q", missing, got)
	}
}
