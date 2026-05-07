package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// execCommand is a tiny indirection so test scaffolding doesn't have to import
// os/exec at every call site. Tests are exempt from Constitution I — only
// production code under cmd/hop and internal/* is bound by it.
var execCommand = exec.Command

// makeClonedRepoDirs pre-creates `<groupDir>/<name>/.git` for each name so
// pull/sync's cloneState check returns stateAlreadyCloned without invoking
// real git.
func makeClonedRepoDirs(t *testing.T, groupDir string, names ...string) {
	t.Helper()
	for _, n := range names {
		if err := os.MkdirAll(filepath.Join(groupDir, n, ".git"), 0o755); err != nil {
			t.Fatalf("setup %s: %v", n, err)
		}
	}
}

// pullSyncYAMLBuilder writes a hop.yaml with a default and a vendor group
// pointing at temp dirs and returns the dirs so callers can pre-stage clones.
func pullSyncYAMLFixture(t *testing.T) (configPath, defaultDir, vendorDir string) {
	t.Helper()
	defaultDir = t.TempDir()
	vendorDir = t.TempDir()
	yaml := "repos:\n" +
		"  default:\n" +
		"    dir: " + defaultDir + "\n" +
		"    urls:\n" +
		"      - git@github.com:sahil87/alpha.git\n" +
		"      - git@github.com:sahil87/beta.git\n" +
		"  vendor:\n" +
		"    dir: " + vendorDir + "\n" +
		"    urls:\n" +
		"      - git@github.com:vendor/gamma.git\n"
	configPath = writeReposFixture(t, yaml)
	return configPath, defaultDir, vendorDir
}

func TestPullUsageErrorWhenNoArgsAndNoAll(t *testing.T) {
	_, _, _ = pullSyncYAMLFixture(t)

	_, _, err := runArgs(t, "pull")
	if err == nil {
		t.Fatalf("expected usage error")
	}
	if !strings.Contains(err.Error(), "missing <name-or-group>") {
		t.Fatalf("expected missing arg hint, got %q", err.Error())
	}
}

func TestPullUsageErrorWhenAllAndPositional(t *testing.T) {
	_, _, _ = pullSyncYAMLFixture(t)

	_, _, err := runArgs(t, "pull", "alpha", "--all")
	if err == nil {
		t.Fatalf("expected usage error for --all + positional")
	}
	if !strings.Contains(err.Error(), "--all conflicts with positional") {
		t.Fatalf("expected conflict hint, got %q", err.Error())
	}
}

func TestPullSingleNotClonedExitsWithSkipMessage(t *testing.T) {
	_, defaultDir, _ := pullSyncYAMLFixture(t)
	// Don't pre-create .git for alpha — it's not cloned.
	_ = defaultDir

	_, stderr, err := runArgs(t, "pull", "alpha")
	if err == nil {
		t.Fatalf("expected error for not-cloned single repo")
	}
	if !strings.Contains(stderr.String(), "skip: alpha not cloned") {
		t.Fatalf("expected skip line, got stderr=%q", stderr.String())
	}
}

func TestPullBatchGroupSkipsNotClonedAndReportsSummary(t *testing.T) {
	_, defaultDir, _ := pullSyncYAMLFixture(t)
	// Neither alpha nor beta has a .git dir — both should be skipped.
	_ = defaultDir

	_, stderr, err := runArgs(t, "pull", "default")
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
	if !strings.Contains(got, "summary: pulled=0 skipped=2 failed=0") {
		t.Errorf("expected summary line, got: %s", got)
	}
}

func TestPullBatchAllIteratesAllRepos(t *testing.T) {
	_, _, _ = pullSyncYAMLFixture(t)

	_, stderr, err := runArgs(t, "pull", "--all")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	got := stderr.String()
	for _, name := range []string{"alpha", "beta", "gamma"} {
		if !strings.Contains(got, "skip: "+name+" not cloned") {
			t.Errorf("expected skip for %s, got: %s", name, got)
		}
	}
	if !strings.Contains(got, "summary: pulled=0 skipped=3 failed=0") {
		t.Errorf("expected summary line, got: %s", got)
	}
}

func TestPullBatchOutputOrderMatchesYAMLSourceOrder(t *testing.T) {
	_, _, _ = pullSyncYAMLFixture(t)

	_, stderr, _ := runArgs(t, "pull", "--all")
	out := stderr.String()
	idxAlpha := strings.Index(out, "alpha")
	idxBeta := strings.Index(out, "beta")
	idxGamma := strings.Index(out, "gamma")
	if !(idxAlpha < idxBeta && idxBeta < idxGamma) {
		t.Fatalf("expected order alpha < beta < gamma in stderr; out=%q", out)
	}
}

func TestPullBatchGroupOnlyIncludesGroupMembers(t *testing.T) {
	_, _, _ = pullSyncYAMLFixture(t)

	_, stderr, _ := runArgs(t, "pull", "vendor")
	out := stderr.String()
	if !strings.Contains(out, "skip: gamma not cloned") {
		t.Errorf("expected gamma in vendor batch, got: %s", out)
	}
	if strings.Contains(out, "alpha") || strings.Contains(out, "beta") {
		t.Errorf("default-group repos must not appear in vendor batch, got: %s", out)
	}
	if !strings.Contains(out, "summary: pulled=0 skipped=1 failed=0") {
		t.Errorf("expected summary, got: %s", out)
	}
}

func TestPullStdoutIsEmpty(t *testing.T) {
	_, _, _ = pullSyncYAMLFixture(t)

	stdout, _, _ := runArgs(t, "pull", "--all")
	if got := stdout.String(); got != "" {
		t.Fatalf("expected empty stdout, got %q", got)
	}
}

func TestPullCobraRejectsTwoPositionals(t *testing.T) {
	_, _, _ = pullSyncYAMLFixture(t)

	_, _, err := runArgs(t, "pull", "alpha", "beta")
	if err == nil {
		t.Fatalf("expected cobra to reject 2 positionals")
	}
	if !strings.Contains(err.Error(), "accepts at most 1 arg") {
		t.Fatalf("expected cobra MaximumNArgs error, got: %v", err)
	}
}

// initBareRepoWithCommit creates a bare repo at <dir>/source.git, then clones
// it into a temp working tree, makes one commit, and pushes it back so the
// bare repo has a default branch with at least one commit. Returns the bare
// repo's file:// URL and the bare path. Used for pull/sync tests where the
// upstream needs to have content (an empty bare upstream causes `git pull` to
// fail with "no such ref was fetched").
func initBareRepoWithCommit(t *testing.T, dir string) (url, srcPath string) {
	t.Helper()
	url, srcPath = initBareRepo(t, dir)

	// Clone, commit, push back. Use exec directly here (not internal/proc) —
	// this is test scaffolding only, not production code under Constitution I.
	stage := filepath.Join(dir, "stage")
	cmds := [][]string{
		{"git", "clone", srcPath, stage},
		{"git", "-C", stage, "-c", "user.email=test@example.com", "-c", "user.name=test", "commit", "--allow-empty", "-m", "init"},
		{"git", "-C", stage, "push", "origin", "HEAD:refs/heads/main"},
	}
	for _, args := range cmds {
		c := execCommand(args[0], args[1:]...)
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("setup %v: %v\noutput: %s", args, err, out)
		}
	}
	return url, srcPath
}

// TestPullSingleHappyPathAgainstRealGit exercises the success path against an
// actual local bare repo with one commit. Verifies the per-repo status line,
// exit 0, and that proc.RunCapture is wired correctly.
func TestPullSingleHappyPathAgainstRealGit(t *testing.T) {
	tmp := t.TempDir()
	url, _ := initBareRepoWithCommit(t, tmp)
	_, defaultDir := fixtureGroup(t, "default", true)

	target := filepath.Join(defaultDir, "source")
	if _, _, err := runArgs(t, "clone", url); err != nil {
		t.Fatalf("setup clone: %v", err)
	}

	_, stderr, err := runArgs(t, "pull", "source")
	if err != nil {
		t.Fatalf("hop pull source: %v\nstderr: %s", err, stderr.String())
	}
	got := stderr.String()
	if !strings.Contains(got, "pull: source ✓") {
		t.Errorf("expected success status line, got: %s", got)
	}
	if !strings.Contains(got, "Already up to date.") {
		t.Errorf("expected 'Already up to date.' summary, got: %s", got)
	}
	if _, err := os.Stat(filepath.Join(target, ".git")); err != nil {
		t.Errorf("expected cloned repo intact: %v", err)
	}
}
