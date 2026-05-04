package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// clearConfigEnv unsets the env vars that drive Resolve so each test starts clean.
func clearConfigEnv(t *testing.T) {
	t.Helper()
	t.Setenv("HOP_CONFIG", "")
	t.Setenv("REPOS_YAML", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "")
	os.Unsetenv("HOP_CONFIG")
	os.Unsetenv("REPOS_YAML")
	os.Unsetenv("XDG_CONFIG_HOME")
	os.Unsetenv("HOME")
}

func TestResolveHopConfigSet(t *testing.T) {
	clearConfigEnv(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "custom.yaml")
	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	t.Setenv("HOP_CONFIG", path)

	got, err := Resolve()
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got != path {
		t.Fatalf("expected %q, got %q", path, got)
	}
}

func TestResolveHopConfigMissing(t *testing.T) {
	clearConfigEnv(t)
	missing := "/nonexistent/path-xyz.yaml"
	t.Setenv("HOP_CONFIG", missing)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "/tmp")

	_, err := Resolve()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	want := "hop: $HOP_CONFIG points to /nonexistent/path-xyz.yaml, which does not exist. Set $HOP_CONFIG to an existing file or unset it."
	if err.Error() != want {
		t.Fatalf("error mismatch:\n  want: %s\n  got:  %s", want, err.Error())
	}
}

func TestResolveXDGConfig(t *testing.T) {
	clearConfigEnv(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "hop", "hop.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	t.Setenv("XDG_CONFIG_HOME", dir)

	got, err := Resolve()
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got != path {
		t.Fatalf("expected %q, got %q", path, got)
	}
}

func TestResolveHomeFallback(t *testing.T) {
	clearConfigEnv(t)
	dir := t.TempDir()
	path := filepath.Join(dir, ".config", "hop", "hop.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	t.Setenv("HOME", dir)

	got, err := Resolve()
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got != path {
		t.Fatalf("expected %q, got %q", path, got)
	}
}

func TestResolveAllUnset(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("HOME", "/this/dir/does/not/exist-xyz")

	_, err := Resolve()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	want := "hop: no hop.yaml found. Set $HOP_CONFIG to a tracked file (e.g., a Dropbox path or a git-tracked dotfile), or run 'hop config init' to bootstrap one at /this/dir/does/not/exist-xyz/.config/hop/hop.yaml."
	if err.Error() != want {
		t.Fatalf("error mismatch:\n  want: %s\n  got:  %s", want, err.Error())
	}
}

func TestResolveAllUnsetXDGSet(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("XDG_CONFIG_HOME", "/custom/xdg")
	t.Setenv("HOME", "/this/dir/does/not/exist-xyz")

	_, err := Resolve()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	want := "hop: no hop.yaml found. Set $HOP_CONFIG to a tracked file (e.g., a Dropbox path or a git-tracked dotfile), or run 'hop config init' to bootstrap one at /custom/xdg/hop/hop.yaml."
	if err.Error() != want {
		t.Fatalf("error mismatch:\n  want: %s\n  got:  %s", want, err.Error())
	}
}

// TestResolveReposYamlIgnored verifies that $REPOS_YAML is no longer consulted
// — the rename is a clean break with no fallback.
func TestResolveReposYamlIgnored(t *testing.T) {
	clearConfigEnv(t)
	dir := t.TempDir()
	reposYaml := filepath.Join(dir, "repos.yaml")
	if err := os.WriteFile(reposYaml, []byte(""), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	t.Setenv("REPOS_YAML", reposYaml)
	t.Setenv("HOME", "/no-such-home-xyz")

	_, err := Resolve()
	if err == nil {
		t.Fatalf("expected error (REPOS_YAML should be ignored), got nil")
	}
	if !strings.Contains(err.Error(), "no hop.yaml found") {
		t.Fatalf("expected 'no hop.yaml found' (no REPOS_YAML fallback); got: %v", err)
	}
}

func TestResolveWriteTargetHopConfigMissing(t *testing.T) {
	clearConfigEnv(t)
	missing := "/tmp/does-not-exist-xyz.yaml"
	t.Setenv("HOP_CONFIG", missing)

	got, err := ResolveWriteTarget()
	if err != nil {
		t.Fatalf("ResolveWriteTarget: %v", err)
	}
	if got != missing {
		t.Fatalf("expected %q, got %q", missing, got)
	}
}

func TestResolveWriteTargetXDG(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("XDG_CONFIG_HOME", "/custom/xdg")

	got, err := ResolveWriteTarget()
	if err != nil {
		t.Fatalf("ResolveWriteTarget: %v", err)
	}
	if !strings.HasSuffix(got, "/custom/xdg/hop/hop.yaml") {
		t.Fatalf("expected XDG-rooted path, got %q", got)
	}
}

func TestResolveWriteTargetHome(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("HOME", "/home/test-user")

	got, err := ResolveWriteTarget()
	if err != nil {
		t.Fatalf("ResolveWriteTarget: %v", err)
	}
	if got != "/home/test-user/.config/hop/hop.yaml" {
		t.Fatalf("expected /home/test-user/.config/hop/hop.yaml, got %q", got)
	}
}
