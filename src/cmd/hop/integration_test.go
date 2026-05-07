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

// writeFakeGitShim builds a single-shot fake `git` shell script that returns
// canned `remote` / `remote get-url <name>` output keyed by the cwd ($PWD).
// The script is created in a fresh dir which the caller prepends to PATH so
// the real git is shadowed.
//
// Returns the dir containing the shim (caller prepends to PATH).
func writeFakeGitShim(t *testing.T, urlByDir map[string]string) string {
	t.Helper()
	binDir := t.TempDir()
	gitPath := filepath.Join(binDir, "git")

	var sb strings.Builder
	sb.WriteString("#!/usr/bin/env bash\n")
	sb.WriteString("# fake git for integration test — driven by $PWD\n")
	sb.WriteString("dir=\"$PWD\"\n")
	sb.WriteString("if [[ \"$1\" == \"remote\" && \"$#\" -eq 1 ]]; then\n")
	for d := range urlByDir {
		sb.WriteString("  if [[ \"$dir\" == \"" + d + "\" ]]; then echo origin; exit 0; fi\n")
	}
	sb.WriteString("  exit 0\n")
	sb.WriteString("fi\n")
	sb.WriteString("if [[ \"$1\" == \"remote\" && \"$2\" == \"get-url\" && \"$3\" == \"origin\" ]]; then\n")
	for d, u := range urlByDir {
		sb.WriteString("  if [[ \"$dir\" == \"" + d + "\" ]]; then echo \"" + u + "\"; exit 0; fi\n")
	}
	sb.WriteString("  exit 1\n")
	sb.WriteString("fi\n")
	sb.WriteString("exit 0\n")

	if err := os.WriteFile(gitPath, []byte(sb.String()), 0o755); err != nil {
		t.Fatalf("write fake git: %v", err)
	}
	return binDir
}

// TestIntegrationConfigScanPrintMode builds the binary, synthesizes a tree
// containing convention-match and non-convention repos plus a worktree and a
// bare repo, and asserts both stdout YAML shape and stderr summary lines.
// `git` is shadowed by the fake shim in writeFakeGitShim so the test is
// deterministic across machines.
func TestIntegrationConfigScanPrintMode(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash required for fake git shim")
	}

	bin := buildBinary(t)
	home := t.TempDir()
	hopYaml := filepath.Join(home, ".config", "hop", "hop.yaml")
	if err := os.MkdirAll(filepath.Dir(hopYaml), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	original := "config:\n  code_root: ~/code\nrepos:\n  default: []\n"
	if err := os.WriteFile(hopYaml, []byte(original), 0o644); err != nil {
		t.Fatalf("write yaml: %v", err)
	}

	scanRoot := filepath.Join(home, "code")
	conv := filepath.Join(scanRoot, "sahil87", "hop")
	nonConv := filepath.Join(home, "vendor", "forks", "tool")
	worktree := filepath.Join(scanRoot, "wt-feature")
	bare := filepath.Join(scanRoot, "mirror.git")

	for _, d := range []string{conv, nonConv} {
		if err := os.MkdirAll(filepath.Join(d, ".git"), 0o755); err != nil {
			t.Fatalf("mkdir repo: %v", err)
		}
	}
	// Worktree: directory with `.git` as a regular file.
	if err := os.MkdirAll(worktree, 0o755); err != nil {
		t.Fatalf("mkdir worktree: %v", err)
	}
	if err := os.WriteFile(filepath.Join(worktree, ".git"), []byte("gitdir: /elsewhere\n"), 0o644); err != nil {
		t.Fatalf("write worktree marker: %v", err)
	}
	// Bare layout.
	if err := os.MkdirAll(filepath.Join(bare, "objects"), 0o755); err != nil {
		t.Fatalf("mkdir bare: %v", err)
	}
	if err := os.WriteFile(filepath.Join(bare, "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatalf("write bare HEAD: %v", err)
	}
	if err := os.WriteFile(filepath.Join(bare, "config"), []byte("[core]\n"), 0o644); err != nil {
		t.Fatalf("write bare config: %v", err)
	}

	canonConv, _ := filepath.EvalSymlinks(conv)
	canonNonConv, _ := filepath.EvalSymlinks(nonConv)
	binDir := writeFakeGitShim(t, map[string]string{
		canonConv:    "git@github.com:sahil87/hop.git",
		canonNonConv: "git@github.com:other/tool.git",
	})

	// Run scan in print mode against ~/code (convention root). Capture both
	// possible UTC dates around the run to avoid a midnight-edge race: the
	// header is stamped during the subprocess, so if the UTC day rolls between
	// capture and assertion the test would flake.
	dateBefore := time.Now().UTC().Format("2006-01-02")
	cmd := exec.Command(bin, "config", "scan", scanRoot)
	cmd.Env = append(os.Environ(),
		"HOME="+home,
		"HOP_CONFIG="+hopYaml,
		"PATH="+binDir+":"+os.Getenv("PATH"),
	)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("config scan: %v\nstderr: %s", err, stderr.String())
	}
	dateAfter := time.Now().UTC().Format("2006-01-02")

	gotOut := stdout.String()
	gotErr := stderr.String()

	// Header line. Accept either of the two adjacent UTC dates spanning the run.
	if !strings.Contains(gotOut, dateBefore+" (UTC).") &&
		!strings.Contains(gotOut, dateAfter+" (UTC).") {
		t.Errorf("missing UTC header date %q or %q; stdout=%q", dateBefore, dateAfter, gotOut)
	}
	// Convention-match URL is in the rendered YAML.
	if !strings.Contains(gotOut, "git@github.com:sahil87/hop.git") {
		t.Errorf("convention URL missing in stdout; stdout=%q", gotOut)
	}
	// Worktree skip in stderr summary.
	if !strings.Contains(gotErr, "1 worktree") {
		t.Errorf("worktree skip missing in summary; stderr=%q", gotErr)
	}
	if !strings.Contains(gotErr, "1 bare repo") {
		t.Errorf("bare-repo skip missing in summary; stderr=%q", gotErr)
	}
	// Original file unchanged in print mode.
	got, err := os.ReadFile(hopYaml)
	if err != nil {
		t.Fatalf("read yaml: %v", err)
	}
	if string(got) != original {
		t.Errorf("hop.yaml mutated in print mode:\nbefore:\n%s\nafter:\n%s", original, got)
	}
}

func TestIntegrationConfigScanWriteMode(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash required for fake git shim")
	}

	bin := buildBinary(t)
	home := t.TempDir()
	hopYaml := filepath.Join(home, ".config", "hop", "hop.yaml")
	if err := os.MkdirAll(filepath.Dir(hopYaml), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	original := "# top comment preserved\nconfig:\n  code_root: ~/code\nrepos:\n  default: []\n"
	if err := os.WriteFile(hopYaml, []byte(original), 0o644); err != nil {
		t.Fatalf("write yaml: %v", err)
	}

	scanRoot := filepath.Join(home, "code")
	conv := filepath.Join(scanRoot, "sahil87", "hop")
	if err := os.MkdirAll(filepath.Join(conv, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}
	canonConv, _ := filepath.EvalSymlinks(conv)
	binDir := writeFakeGitShim(t, map[string]string{
		canonConv: "git@github.com:sahil87/hop.git",
	})

	cmd := exec.Command(bin, "config", "scan", scanRoot, "--write")
	cmd.Env = append(os.Environ(),
		"HOME="+home,
		"HOP_CONFIG="+hopYaml,
		"PATH="+binDir+":"+os.Getenv("PATH"),
	)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("config scan --write: %v\nstderr: %s", err, stderr.String())
	}

	if stdout.Len() != 0 {
		t.Errorf("write mode stdout should be empty; got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "wrote: "+hopYaml) {
		t.Errorf("missing 'wrote:' trailer; stderr=%q", stderr.String())
	}
	got, _ := os.ReadFile(hopYaml)
	gotStr := string(got)
	if !strings.Contains(gotStr, "git@github.com:sahil87/hop.git") {
		t.Errorf("URL not merged; got:\n%s", gotStr)
	}
	if !strings.Contains(gotStr, "# top comment preserved") {
		t.Errorf("comments lost; got:\n%s", gotStr)
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
