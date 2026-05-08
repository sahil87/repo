package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// push tests reuse pullSyncYAMLFixture (defined in pull_test.go) — push shares
// the same resolver/runBatch pipeline as pull and sync, so the registry shape
// is identical. Keeping a single fixture avoids drift if the test registry
// shape ever changes.

func TestPushUsageErrorWhenNoArgsAndNoAll(t *testing.T) {
	_, _, _ = pullSyncYAMLFixture(t)

	_, _, err := runArgs(t, "push")
	if err == nil {
		t.Fatalf("expected usage error")
	}
	if !strings.Contains(err.Error(), "missing <name-or-group>") {
		t.Fatalf("expected missing arg hint, got %q", err.Error())
	}
}

func TestPushUsageErrorWhenAllAndPositional(t *testing.T) {
	_, _, _ = pullSyncYAMLFixture(t)

	_, _, err := runArgs(t, "push", "alpha", "--all")
	if err == nil {
		t.Fatalf("expected usage error for --all + positional")
	}
	if !strings.Contains(err.Error(), "--all conflicts with positional") {
		t.Fatalf("expected conflict hint, got %q", err.Error())
	}
}

func TestPushSingleNotClonedExitsWithSkipMessage(t *testing.T) {
	_, defaultDir, _ := pullSyncYAMLFixture(t)
	// Don't pre-create .git for alpha — it's not cloned.
	_ = defaultDir

	_, stderr, err := runArgs(t, "push", "alpha")
	if err == nil {
		t.Fatalf("expected error for not-cloned single repo")
	}
	if !strings.Contains(stderr.String(), "skip: alpha not cloned") {
		t.Fatalf("expected skip line, got stderr=%q", stderr.String())
	}
}

func TestPushBatchGroupSkipsNotClonedAndReportsSummary(t *testing.T) {
	_, defaultDir, _ := pullSyncYAMLFixture(t)
	// Neither alpha nor beta has a .git dir — both should be skipped.
	_ = defaultDir

	_, stderr, err := runArgs(t, "push", "default")
	if err != nil {
		t.Fatalf("expected nil err for batch with all-skipped, got %v", err)
	}
	got := stderr.String()
	if !strings.Contains(got, "skip: alpha not cloned") {
		t.Errorf("expected skip alpha line, got: %s", got)
	}
	if !strings.Contains(got, "skip: beta not cloned") {
		t.Errorf("expected skip beta line, got: %s", got)
	}
	if !strings.Contains(got, "summary: pushed=0 skipped=2 failed=0") {
		t.Errorf("expected summary line, got: %s", got)
	}
}

func TestPushBatchAllIteratesAllRepos(t *testing.T) {
	_, _, _ = pullSyncYAMLFixture(t)

	_, stderr, err := runArgs(t, "push", "--all")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	got := stderr.String()
	for _, name := range []string{"alpha", "beta", "gamma"} {
		if !strings.Contains(got, "skip: "+name+" not cloned") {
			t.Errorf("expected skip for %s, got: %s", name, got)
		}
	}
	if !strings.Contains(got, "summary: pushed=0 skipped=3 failed=0") {
		t.Errorf("expected summary line, got: %s", got)
	}
}

func TestPushBatchOutputOrderMatchesYAMLSourceOrder(t *testing.T) {
	_, _, _ = pullSyncYAMLFixture(t)

	_, stderr, _ := runArgs(t, "push", "--all")
	out := stderr.String()
	idxAlpha := strings.Index(out, "alpha")
	idxBeta := strings.Index(out, "beta")
	idxGamma := strings.Index(out, "gamma")
	if !(idxAlpha < idxBeta && idxBeta < idxGamma) {
		t.Fatalf("expected order alpha < beta < gamma in stderr; out=%q", out)
	}
}

func TestPushBatchGroupOnlyIncludesGroupMembers(t *testing.T) {
	_, _, _ = pullSyncYAMLFixture(t)

	_, stderr, _ := runArgs(t, "push", "vendor")
	out := stderr.String()
	if !strings.Contains(out, "skip: gamma not cloned") {
		t.Errorf("expected gamma in vendor batch, got: %s", out)
	}
	if strings.Contains(out, "alpha") || strings.Contains(out, "beta") {
		t.Errorf("default-group repos must not appear in vendor batch, got: %s", out)
	}
	if !strings.Contains(out, "summary: pushed=0 skipped=1 failed=0") {
		t.Errorf("expected summary, got: %s", out)
	}
}

func TestPushStdoutIsEmpty(t *testing.T) {
	_, _, _ = pullSyncYAMLFixture(t)

	stdout, _, _ := runArgs(t, "push", "--all")
	if got := stdout.String(); got != "" {
		t.Fatalf("expected empty stdout, got %q", got)
	}
}

func TestPushCobraRejectsTwoPositionals(t *testing.T) {
	_, _, _ = pullSyncYAMLFixture(t)

	_, _, err := runArgs(t, "push", "alpha", "beta")
	if err == nil {
		t.Fatalf("expected cobra to reject 2 positionals")
	}
	if !strings.Contains(err.Error(), "accepts at most 1 arg") {
		t.Fatalf("expected cobra MaximumNArgs error, got: %v", err)
	}
}

// TestPushSingleHappyPathAgainstRealGit exercises the success path against an
// actual local bare repo with one commit. After cloning, we make a local
// (allow-empty) commit and verify `hop push <name>` succeeds — exit 0, the
// per-repo `push: <name> ✓` status line on stderr, and stdout empty.
func TestPushSingleHappyPathAgainstRealGit(t *testing.T) {
	tmp := t.TempDir()
	url, _ := initBareRepoWithCommit(t, tmp)
	_, defaultDir := fixtureGroup(t, "default", true)

	target := filepath.Join(defaultDir, "source")
	if _, _, err := runArgs(t, "clone", url); err != nil {
		t.Fatalf("setup clone: %v", err)
	}

	// Make a local commit so push has something to publish (without it, push
	// emits "Everything up-to-date." which is also a successful exit-0 path —
	// either outcome verifies the wiring, but keeping the commit makes the
	// scenario realistic).
	commits := [][]string{
		{"git", "-C", target, "-c", "user.email=test@example.com", "-c", "user.name=test", "commit", "--allow-empty", "-m", "local-commit"},
	}
	for _, args := range commits {
		c := execCommand(args[0], args[1:]...)
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("setup commit %v: %v\noutput: %s", args, err, out)
		}
	}

	stdout, stderr, err := runArgs(t, "push", "source")
	if err != nil {
		t.Fatalf("hop push source: %v\nstderr: %s", err, stderr.String())
	}
	got := stderr.String()
	if !strings.Contains(got, "push: source ✓") {
		t.Errorf("expected success status line, got: %s", got)
	}
	if got := stdout.String(); got != "" {
		t.Errorf("expected empty stdout, got %q", got)
	}
	if _, err := os.Stat(filepath.Join(target, ".git")); err != nil {
		t.Errorf("expected cloned repo intact: %v", err)
	}
}
