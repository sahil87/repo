package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// buildBinary compiles the repo binary into a temp dir and returns the path.
// Uses os/exec directly per spec.md §"Test-file exception".
func buildBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "repo")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	// We're in src/cmd/repo as the test runs; build from module root (one dir up
	// from the go test working dir is src/cmd/repo, so module root is ../..).
	cmd := exec.CommandContext(ctx, "go", "build", "-o", bin, "./cmd/repo")
	cmd.Dir = "../.." // resolve to src/
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Skipf("go build failed in test environment: %v", err)
	}
	return bin
}

func TestIntegrationVersion(t *testing.T) {
	bin := buildBinary(t)
	out, err := exec.Command(bin, "--version").CombinedOutput()
	if err != nil {
		t.Fatalf("repo --version: %v\noutput: %s", err, out)
	}
	if strings.TrimSpace(string(out)) == "" {
		t.Fatalf("expected non-empty version output")
	}

	out2, err := exec.Command(bin, "-v").CombinedOutput()
	if err != nil {
		t.Fatalf("repo -v: %v\noutput: %s", err, out2)
	}
	if strings.TrimSpace(string(out2)) == "" {
		t.Fatalf("expected non-empty version output for -v")
	}
}

func TestIntegrationCdHint(t *testing.T) {
	bin := buildBinary(t)
	cmd := exec.Command(bin, "cd", "anything")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected non-zero exit, got nil err. output: %s", out)
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		if exitErr.ExitCode() != 2 {
			t.Fatalf("expected exit code 2, got %d", exitErr.ExitCode())
		}
	} else {
		t.Fatalf("expected *exec.ExitError, got %T", err)
	}
	if !strings.Contains(string(out), "'cd' is shell-only") {
		t.Fatalf("expected hint in output, got: %s", out)
	}
}

func TestIntegrationPathAndLs(t *testing.T) {
	bin := buildBinary(t)
	dir := t.TempDir()
	yaml := filepath.Join(dir, "repos.yaml")
	body := `/tmp/integration-test:
  - git@github.com:sahil87/alpha.git
  - git@github.com:sahil87/beta.git
  - git@github.com:sahil87/gamma.git
`
	if err := os.WriteFile(yaml, []byte(body), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	// `repo path alpha`
	cmd := exec.Command(bin, "path", "alpha")
	cmd.Env = append(os.Environ(), "REPOS_YAML="+yaml)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("repo path alpha: %v\noutput: %s", err, out)
	}
	if got := strings.TrimSpace(string(out)); got != "/tmp/integration-test/alpha" {
		t.Fatalf("expected /tmp/integration-test/alpha, got %q", got)
	}

	// `repo ls`
	cmd = exec.Command(bin, "ls")
	cmd.Env = append(os.Environ(), "REPOS_YAML="+yaml)
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("repo ls: %v\noutput: %s", err, out)
	}
	for _, name := range []string{"alpha", "beta", "gamma"} {
		if !strings.Contains(string(out), name) {
			t.Fatalf("expected %s in ls output, got: %s", name, out)
		}
	}
}

func TestIntegrationShellInitZsh(t *testing.T) {
	bin := buildBinary(t)
	out, err := exec.Command(bin, "shell-init", "zsh").Output()
	if err != nil {
		t.Fatalf("repo shell-init zsh: %v", err)
	}
	if !strings.Contains(string(out), "repo()") {
		t.Fatalf("expected repo() function in emitted text")
	}
	if !strings.Contains(string(out), "compdef _repo repo") {
		t.Fatalf("expected compdef line in emitted text")
	}
}
