package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// installFakeWt writes a fake `wt` shell script into a temp dir and prepends
// that dir to PATH. The script's behavior is controlled by FAKE_WT_MODE:
//
//	noop  → exit 0 (simulates user picking any non-failing menu option)
//	fail  → exit 7 (simulates wt internal error)
//
// The script also writes a side-channel file at $FAKE_WT_LOG recording the
// argv it received and the cwd it was launched from, so tests can assert
// hop's invocation contract.
//
// Returns the path to the side-channel log.
func installFakeWt(t *testing.T, mode string) string {
	t.Helper()
	dir := t.TempDir()
	wtPath := filepath.Join(dir, "wt")
	logPath := filepath.Join(dir, "wt-log")

	script := `#!/bin/sh
{
  echo "PWD=$PWD"
  echo "ARGC=$#"
  i=1
  for a in "$@"; do
    echo "ARG${i}=${a}"
    i=$((i+1))
  done
} > "$FAKE_WT_LOG"
case "$FAKE_WT_MODE" in
  noop)
    exit 0
    ;;
  fail)
    exit 7
    ;;
  *)
    echo "fake-wt: unknown mode $FAKE_WT_MODE" 1>&2
    exit 99
    ;;
esac
`
	if err := os.WriteFile(wtPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake wt: %v", err)
	}

	t.Setenv("FAKE_WT_MODE", mode)
	t.Setenv("FAKE_WT_LOG", logPath)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return logPath
}

// writeRepoFixture creates a hop.yaml fixture with a single repo named `name`
// whose dir is a real, existing directory (so wt's stat-the-arg branch
// succeeds). Returns the resolved repo path.
func writeRepoFixture(t *testing.T, name string) string {
	t.Helper()
	repoDir := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}
	yaml := fmt.Sprintf(`repos:
  default:
    dir: %s
    urls:
      - git@github.com:sahil87/%s.git
`, filepath.Dir(repoDir), name)
	writeReposFixture(t, yaml)
	return repoDir
}

// TestOpenPassesRepoPathAsArgToWt asserts the binary execs `wt open <path>`
// with the resolved repo path as a positional arg (not via cmd.Dir). This
// is what makes wt take its path-first branch (showing the app menu) rather
// than the worktree-selection menu it would show for a main-repo cwd with
// no arg.
func TestOpenPassesRepoPathAsArgToWt(t *testing.T) {
	repoDir := writeRepoFixture(t, "outbox")
	logPath := installFakeWt(t, "noop")

	stdout, stderr, err := runArgs(t, "outbox", "open")
	if err != nil {
		t.Fatalf("hop outbox open: %v\nstderr: %s", err, stderr.String())
	}
	if got := stdout.String(); got != "" {
		t.Errorf("expected empty stdout (binary is a passthrough), got %q", got)
	}

	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read fake-wt log: %v", err)
	}
	log := string(logBytes)
	if !strings.Contains(log, "ARGC=2\n") {
		t.Errorf("expected wt to receive 2 args (open <path>), log:\n%s", log)
	}
	if !strings.Contains(log, "ARG1=open\n") {
		t.Errorf("expected ARG1=open, log:\n%s", log)
	}
	if !strings.Contains(log, "ARG2="+repoDir+"\n") {
		t.Errorf("expected ARG2=%q (resolved repo path), log:\n%s", repoDir, log)
	}
}

// TestOpenPropagatesNonZeroExitCode asserts hop's exit code matches wt's
// when wt exits non-zero.
func TestOpenPropagatesNonZeroExitCode(t *testing.T) {
	writeRepoFixture(t, "outbox")
	installFakeWt(t, "fail")

	_, _, err := runArgs(t, "outbox", "open")
	if err == nil {
		t.Fatalf("expected non-nil error from wt fail mode")
	}
	var withCode *errExitCode
	if !errors.As(err, &withCode) {
		t.Fatalf("expected *errExitCode, got %T: %v", err, err)
	}
	if withCode.code != 7 {
		t.Fatalf("expected exit 7 propagated from fake wt, got %d", withCode.code)
	}
}

// TestOpenWtMissingExitsSilent asserts that when wt is not on PATH, hop
// emits a clean stderr hint and exits 1 via errSilent (no traceback, no
// crash).
func TestOpenWtMissingExitsSilent(t *testing.T) {
	writeRepoFixture(t, "outbox")
	emptyDir := t.TempDir()
	t.Setenv("PATH", emptyDir)

	_, stderr, err := runArgs(t, "outbox", "open")
	if !errors.Is(err, errSilent) {
		t.Fatalf("expected errSilent for missing wt, got %v", err)
	}
	if !strings.Contains(stderr.String(), "wt") {
		t.Errorf("expected stderr to mention `wt`, got: %s", stderr.String())
	}
}

// TestOpenSilentOnSuccess asserts hop emits no stdout regardless of whether
// wt exits 0 or non-zero. The cd-handoff is owned by the shim (via
// WT_CD_FILE), not the binary — the binary is a transparent passthrough.
func TestOpenSilentOnSuccess(t *testing.T) {
	writeRepoFixture(t, "outbox")
	installFakeWt(t, "noop")

	stdout, _, err := runArgs(t, "outbox", "open")
	if err != nil {
		t.Fatalf("hop outbox open: %v", err)
	}
	if got := stdout.String(); got != "" {
		t.Fatalf("expected empty stdout (binary passthrough), got %q", got)
	}
}
