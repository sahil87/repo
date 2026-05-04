package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigInitWritesStarter(t *testing.T) {
	clearConfigEnv(t)
	dir := t.TempDir()
	target := filepath.Join(dir, "hop.yaml")
	t.Setenv("HOP_CONFIG", target)

	stdout, _, err := runArgs(t, "config", "init")
	if err != nil {
		t.Fatalf("config init: %v", err)
	}
	if !strings.Contains(stdout.String(), "Created "+target) {
		t.Fatalf("expected 'Created %s' on stdout, got %q", target, stdout.String())
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
	clearConfigEnv(t)
	dir := t.TempDir()
	target := filepath.Join(dir, "hop.yaml")
	if err := os.WriteFile(target, []byte("existing\n"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	t.Setenv("HOP_CONFIG", target)

	_, _, err := runArgs(t, "config", "init")
	if err == nil {
		t.Fatalf("expected refusal, got nil")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected 'already exists' message, got %q", err.Error())
	}
}

func TestConfigWherePrintsResolvedPath(t *testing.T) {
	clearConfigEnv(t)
	dir := t.TempDir()
	target := filepath.Join(dir, "hop.yaml")
	t.Setenv("HOP_CONFIG", target)

	stdout, _, err := runArgs(t, "config", "where")
	if err != nil {
		t.Fatalf("config where: %v", err)
	}
	if got := strings.TrimSpace(stdout.String()); got != target {
		t.Fatalf("expected %q, got %q", target, got)
	}
}

func TestConfigWhereDoesNotErrorOnMissingFile(t *testing.T) {
	clearConfigEnv(t)
	missing := "/tmp/no-such-file-xyz123.yaml"
	t.Setenv("HOP_CONFIG", missing)

	stdout, _, err := runArgs(t, "config", "where")
	if err != nil {
		t.Fatalf("config where on missing file: %v", err)
	}
	if got := strings.TrimSpace(stdout.String()); got != missing {
		t.Fatalf("expected %q, got %q", missing, got)
	}
}

func TestConfigPathSubcommandRemoved(t *testing.T) {
	clearConfigEnv(t)
	target := "/tmp/whatever-test-xyz.yaml"
	t.Setenv("HOP_CONFIG", target)
	stdout, _, _ := runArgs(t, "config", "path")
	// The old handler would have printed the resolved write target on stdout.
	// We assert the new behavior: stdout MUST NOT be just the resolved path.
	if strings.TrimSpace(stdout.String()) == target {
		t.Fatalf("config path appears to still call the old handler (stdout = %q)", stdout.String())
	}
}
