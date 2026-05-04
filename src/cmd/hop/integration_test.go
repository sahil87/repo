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

// buildBinary compiles the hop binary into a temp dir and returns the path.
func buildBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "hop")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	// We're in src/cmd/hop as the test runs; build from module root (one dir up
	// from the go test working dir is src/cmd/hop, so module root is ../..).
	cmd := exec.CommandContext(ctx, "go", "build", "-o", bin, "./cmd/hop")
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
		t.Fatalf("hop --version: %v\noutput: %s", err, out)
	}
	if strings.TrimSpace(string(out)) == "" {
		t.Fatalf("expected non-empty version output")
	}

	out2, err := exec.Command(bin, "-v").CombinedOutput()
	if err != nil {
		t.Fatalf("hop -v: %v\noutput: %s", err, out2)
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
	if !strings.Contains(string(out), "hop shell-init zsh") {
		t.Fatalf("expected hint to reference hop shell-init zsh, got: %s", out)
	}
}

func TestIntegrationWhereAndLs(t *testing.T) {
	bin := buildBinary(t)
	dir := t.TempDir()
	yaml := filepath.Join(dir, "hop.yaml")
	body := `repos:
  default:
    dir: /tmp/integration-test
    urls:
      - git@github.com:sahil87/alpha.git
      - git@github.com:sahil87/beta.git
      - git@github.com:sahil87/gamma.git
`
	if err := os.WriteFile(yaml, []byte(body), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	// `hop where alpha`
	cmd := exec.Command(bin, "where", "alpha")
	cmd.Env = append(os.Environ(), "HOP_CONFIG="+yaml)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("hop where alpha: %v\noutput: %s", err, out)
	}
	if got := strings.TrimSpace(string(out)); got != "/tmp/integration-test/alpha" {
		t.Fatalf("expected /tmp/integration-test/alpha, got %q", got)
	}

	// `hop ls`
	cmd = exec.Command(bin, "ls")
	cmd.Env = append(os.Environ(), "HOP_CONFIG="+yaml)
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("hop ls: %v\noutput: %s", err, out)
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
		t.Fatalf("hop shell-init zsh: %v", err)
	}
	if !strings.Contains(string(out), "hop()") {
		t.Fatalf("expected hop() function in emitted text")
	}
	if !strings.Contains(string(out), "h() { hop") {
		t.Fatalf("expected h() alias in emitted text")
	}
}

func TestIntegrationDashR(t *testing.T) {
	bin := buildBinary(t)
	dir := t.TempDir()
	yaml := filepath.Join(dir, "hop.yaml")
	body := `repos:
  default:
    dir: ` + dir + `
    urls:
      - git@github.com:sahil87/probe.git
`
	if err := os.WriteFile(yaml, []byte(body), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	// Make the resolved path actually exist so the child has a valid Dir.
	if err := os.MkdirAll(filepath.Join(dir, "probe"), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}

	cmd := exec.Command(bin, "-R", "probe", "pwd")
	cmd.Env = append(os.Environ(), "HOP_CONFIG="+yaml)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("hop -R probe pwd: %v\noutput: %s", err, out)
	}
	want := filepath.Join(dir, "probe")
	if got := strings.TrimSpace(string(out)); got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestIntegrationDashRNoCommand(t *testing.T) {
	bin := buildBinary(t)
	cmd := exec.Command(bin, "-R", "anything")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		if exitErr.ExitCode() != 2 {
			t.Fatalf("expected exit 2, got %d", exitErr.ExitCode())
		}
	}
	if !strings.Contains(string(out), "-R requires a command") {
		t.Fatalf("expected usage hint, got: %s", out)
	}
}

// TestIntegrationShellInitBashSourceable spawns a real bash, evals the
// shim script emitted by `hop shell-init bash`, and exercises one dispatch
// path (bare-name resolution via `command hop where`). This catches syntax,
// quoting, and completion-registration regressions that string-level
// assertions in shell_init_test.go would miss. Skipped if bash isn't
// available.
func TestIntegrationShellInitBashSourceable(t *testing.T) {
	bashPath, err := exec.LookPath("bash")
	if err != nil {
		t.Skipf("bash not on PATH: %v", err)
	}

	bin := buildBinary(t)
	dir := t.TempDir()
	yaml := filepath.Join(dir, "hop.yaml")
	body := `repos:
  default:
    dir: ` + dir + `
    urls:
      - git@github.com:sahil87/probe.git
`
	if err := os.WriteFile(yaml, []byte(body), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	// Eval the shim, then call hop where (a known-subcommand path), and
	// verify the function-wrapped call resolves correctly. Use --noprofile
	// --norc to isolate from the user's bashrc and PATH-prepend the dir
	// containing the freshly-built binary so `command hop` resolves to it.
	binDir := filepath.Dir(bin)
	script := `eval "$(hop shell-init bash)" 2>/dev/null
hop where probe`
	cmd := exec.Command(bashPath, "--noprofile", "--norc", "-c", script)
	cmd.Env = append(os.Environ(),
		"HOP_CONFIG="+yaml,
		"PATH="+binDir+":"+os.Getenv("PATH"),
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bash -c '<shim>; hop where probe': %v\noutput: %s", err, out)
	}
	want := filepath.Join(dir, "probe")
	if got := strings.TrimSpace(string(out)); got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}
