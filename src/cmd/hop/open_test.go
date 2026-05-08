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
// that dir to PATH. The script's behavior is controlled by FAKE_WT_MODE in the
// env (read by the script at runtime):
//
//	here  → write $PWD to $WT_CD_FILE, exit 0 (simulates user picking "Open here")
//	noop  → do nothing, exit 0 (simulates editor/terminal launch)
//	fail  → exit 7 (simulates wt internal error)
//
// The script also writes a side-channel file `$FAKE_WT_LOG` recording $PWD,
// $WT_CD_FILE, and $WT_WRAPPER so tests can assert what hop passed.
//
// Returns the path to the side-channel log; tests inspect it to verify hop's
// invocation contract.
func installFakeWt(t *testing.T, mode string) string {
	t.Helper()
	dir := t.TempDir()
	wtPath := filepath.Join(dir, "wt")
	logPath := filepath.Join(dir, "wt-log")

	script := `#!/bin/sh
{
  echo "PWD=$PWD"
  echo "WT_CD_FILE=$WT_CD_FILE"
  echo "WT_WRAPPER=$WT_WRAPPER"
} > "$FAKE_WT_LOG"
case "$FAKE_WT_MODE" in
  here)
    printf '%s' "$PWD" > "$WT_CD_FILE"
    exit 0
    ;;
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
// whose dir is a real, existing directory (so wt's chdir-to-repo succeeds).
// Returns the resolved repo path.
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

func TestOpenHereWritesPathToStdout(t *testing.T) {
	repoDir := writeRepoFixture(t, "outbox")
	logPath := installFakeWt(t, "here")
	t.Setenv("HOP_WRAPPER", "1") // simulate shim-loaded environment

	stdout, stderr, err := runArgs(t, "outbox", "open")
	if err != nil {
		t.Fatalf("hop outbox open: %v\nstderr: %s", err, stderr.String())
	}
	got := stdout.String()
	if got != repoDir {
		t.Fatalf("expected stdout = %q, got %q", repoDir, got)
	}
	// Verify hop passed the expected env to wt.
	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read fake-wt log: %v", err)
	}
	log := string(logBytes)
	if !strings.Contains(log, "PWD="+repoDir+"\n") {
		t.Errorf("expected fake wt cwd = %q, log:\n%s", repoDir, log)
	}
	if !strings.Contains(log, "WT_WRAPPER=1\n") {
		t.Errorf("expected WT_WRAPPER=1 in env, log:\n%s", log)
	}
	if !strings.Contains(log, "WT_CD_FILE=") {
		t.Errorf("expected WT_CD_FILE in env, log:\n%s", log)
	}
}

func TestOpenNoopDoesNotEmitStdout(t *testing.T) {
	writeRepoFixture(t, "outbox")
	installFakeWt(t, "noop")
	t.Setenv("HOP_WRAPPER", "1")

	stdout, stderr, err := runArgs(t, "outbox", "open")
	if err != nil {
		t.Fatalf("hop outbox open: %v\nstderr: %s", err, stderr.String())
	}
	if got := stdout.String(); got != "" {
		t.Fatalf("expected empty stdout (no Open-here), got %q", got)
	}
}

func TestOpenPropagatesNonZeroExitCode(t *testing.T) {
	writeRepoFixture(t, "outbox")
	installFakeWt(t, "fail")
	t.Setenv("HOP_WRAPPER", "1")

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

func TestOpenWtMissingExitsSilent(t *testing.T) {
	writeRepoFixture(t, "outbox")
	// Set PATH to a single empty dir so `wt` is not findable.
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

func TestOpenHereWithoutShimEmitsHint(t *testing.T) {
	repoDir := writeRepoFixture(t, "outbox")
	installFakeWt(t, "here")
	// Explicitly unset HOP_WRAPPER to simulate binary-direct invocation.
	os.Unsetenv("HOP_WRAPPER")

	stdout, stderr, err := runArgs(t, "outbox", "open")
	if err != nil {
		t.Fatalf("hop outbox open: %v\nstderr: %s", err, stderr.String())
	}
	if got := stdout.String(); got != repoDir {
		t.Fatalf("expected stdout = %q, got %q", repoDir, got)
	}
	if !strings.Contains(stderr.String(), "shell shim") {
		t.Errorf("expected no-shim hint in stderr, got: %s", stderr.String())
	}
}

func TestOpenHereWithShimSuppressesHint(t *testing.T) {
	writeRepoFixture(t, "outbox")
	installFakeWt(t, "here")
	t.Setenv("HOP_WRAPPER", "1")

	_, stderr, err := runArgs(t, "outbox", "open")
	if err != nil {
		t.Fatalf("hop outbox open: %v\nstderr: %s", err, stderr.String())
	}
	if strings.Contains(stderr.String(), "shell shim") {
		t.Errorf("expected NO no-shim hint when HOP_WRAPPER=1, got: %s", stderr.String())
	}
}

// TestOpenCleansUpTempFile asserts the WT_CD_FILE temp path is removed by
// runOpen's defer os.Remove. The fake wt records the path in its log; we
// extract it post-invocation and check the file no longer exists.
func TestOpenCleansUpTempFile(t *testing.T) {
	writeRepoFixture(t, "outbox")
	logPath := installFakeWt(t, "here")
	t.Setenv("HOP_WRAPPER", "1")

	if _, _, err := runArgs(t, "outbox", "open"); err != nil {
		t.Fatalf("hop outbox open: %v", err)
	}
	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read fake-wt log: %v", err)
	}
	// Extract the WT_CD_FILE value.
	var cdPath string
	for _, line := range strings.Split(string(logBytes), "\n") {
		if strings.HasPrefix(line, "WT_CD_FILE=") {
			cdPath = strings.TrimPrefix(line, "WT_CD_FILE=")
			break
		}
	}
	if cdPath == "" {
		t.Fatalf("no WT_CD_FILE recorded in fake-wt log:\n%s", string(logBytes))
	}
	if _, err := os.Stat(cdPath); !os.IsNotExist(err) {
		t.Fatalf("expected WT_CD_FILE %q to be cleaned up post-invocation; stat err=%v", cdPath, err)
	}
}
