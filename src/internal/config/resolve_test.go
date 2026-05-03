package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// clearConfigEnv unsets the three env vars that drive Resolve so each test starts clean.
func clearConfigEnv(t *testing.T) {
	t.Helper()
	t.Setenv("REPOS_YAML", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "")
	// Clear empty-string vs unset distinction by using os.Unsetenv directly,
	// since t.Setenv does not provide an unset; setting to "" emulates unset for
	// our LookupEnv/Getenv usage.
	os.Unsetenv("REPOS_YAML")
	os.Unsetenv("XDG_CONFIG_HOME")
	os.Unsetenv("HOME")
}

func TestResolveReposYamlSet(t *testing.T) {
	clearConfigEnv(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "custom.yaml")
	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	t.Setenv("REPOS_YAML", path)

	got, err := Resolve()
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got != path {
		t.Fatalf("expected %q, got %q", path, got)
	}
}

func TestResolveReposYamlMissing(t *testing.T) {
	clearConfigEnv(t)
	missing := "/nonexistent/path-xyz.yaml"
	t.Setenv("REPOS_YAML", missing)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "/tmp")

	_, err := Resolve()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	want := "repo: $REPOS_YAML points to /nonexistent/path-xyz.yaml, which does not exist. Set $REPOS_YAML to an existing file or unset it."
	if err.Error() != want {
		t.Fatalf("error mismatch:\n  want: %s\n  got:  %s", want, err.Error())
	}
}

func TestResolveXDGConfig(t *testing.T) {
	clearConfigEnv(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "repo", "repos.yaml")
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
	path := filepath.Join(dir, ".config", "repo", "repos.yaml")
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
	want := "repo: no repos.yaml found. Set $REPOS_YAML to a tracked file (e.g., a Dropbox path or a git-tracked dotfile), or run 'repo config init' to bootstrap one at $XDG_CONFIG_HOME/repo/repos.yaml."
	if err.Error() != want {
		t.Fatalf("error mismatch:\n  want: %s\n  got:  %s", want, err.Error())
	}
}

func TestResolveWriteTargetReposYamlMissing(t *testing.T) {
	clearConfigEnv(t)
	missing := "/tmp/does-not-exist-xyz.yaml"
	t.Setenv("REPOS_YAML", missing)

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
	if !strings.HasSuffix(got, "/custom/xdg/repo/repos.yaml") {
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
	if got != "/home/test-user/.config/repo/repos.yaml" {
		t.Fatalf("expected /home/test-user/.config/repo/repos.yaml, got %q", got)
	}
}
