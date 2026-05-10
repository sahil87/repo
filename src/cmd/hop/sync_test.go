package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSyncUsageErrorWhenNoArgsAndNoAll(t *testing.T) {
	_, _, _ = pullSyncYAMLFixture(t)

	_, _, err := runArgs(t, "sync")
	if err == nil {
		t.Fatalf("expected usage error")
	}
	if !strings.Contains(err.Error(), "missing <name-or-group>") {
		t.Fatalf("expected missing-arg hint, got %q", err.Error())
	}
}

func TestSyncUsageErrorWhenAllAndPositional(t *testing.T) {
	_, _, _ = pullSyncYAMLFixture(t)

	_, _, err := runArgs(t, "sync", "alpha", "--all")
	if err == nil {
		t.Fatalf("expected usage error for --all + positional")
	}
	if !strings.Contains(err.Error(), "--all conflicts with positional") {
		t.Fatalf("expected conflict hint, got %q", err.Error())
	}
}

func TestSyncSingleNotClonedExitsWithSkipMessage(t *testing.T) {
	_, _, _ = pullSyncYAMLFixture(t)

	_, stderr, err := runArgs(t, "sync", "alpha")
	if err == nil {
		t.Fatalf("expected error for not-cloned single repo")
	}
	if !strings.Contains(stderr.String(), "skip: alpha not cloned") {
		t.Fatalf("expected skip line, got stderr=%q", stderr.String())
	}
}

func TestSyncBatchGroupSkipsAllNotClonedAndReportsSummary(t *testing.T) {
	_, _, _ = pullSyncYAMLFixture(t)

	_, stderr, err := runArgs(t, "sync", "default")
	if err != nil {
		t.Fatalf("expected nil err for all-skipped batch, got %v", err)
	}
	got := stderr.String()
	if !strings.Contains(got, "skip: alpha not cloned") {
		t.Errorf("expected skip alpha, got: %s", got)
	}
	if !strings.Contains(got, "skip: beta not cloned") {
		t.Errorf("expected skip beta, got: %s", got)
	}
	if !strings.Contains(got, "summary: synced=0 skipped=2 failed=0") {
		t.Errorf("expected sync summary line, got: %s", got)
	}
}

func TestSyncStdoutIsEmpty(t *testing.T) {
	_, _, _ = pullSyncYAMLFixture(t)

	stdout, _, _ := runArgs(t, "sync", "--all")
	if got := stdout.String(); got != "" {
		t.Fatalf("expected empty stdout, got %q", got)
	}
}

func TestSyncCobraRejectsTwoPositionals(t *testing.T) {
	_, _, _ = pullSyncYAMLFixture(t)

	_, _, err := runArgs(t, "sync", "alpha", "beta")
	if err == nil {
		t.Fatalf("expected cobra to reject 2 positionals")
	}
	if !strings.Contains(err.Error(), "accepts at most 1 arg") {
		t.Fatalf("expected cobra MaximumNArgs error, got: %v", err)
	}
}

func TestMentionsConflictDetectsRebaseMarker(t *testing.T) {
	if !mentionsConflict("error: could not apply 1234... CONFLICT (content)", "", nil) {
		t.Errorf("expected stdout CONFLICT to be detected")
	}
	if !mentionsConflict("", "CONFLICT (content): Merge conflict in foo.txt", nil) {
		t.Errorf("expected stderr CONFLICT to be detected (git emits CONFLICT on stderr)")
	}
	if mentionsConflict("Already up to date.", "", nil) {
		t.Errorf("did not expect false positive on clean output")
	}
}

// configureGitIdentity sets local user.name and user.email on a clone so
// `git commit` (invoked by hop sync against the clone) succeeds without
// inheriting an unset global identity. Mirrors the test scaffolding pattern
// used by `initBareRepoWithCommit` (which uses `-c user.email=... -c user.name=...`
// per-command) — local config keeps subsequent commits hop drives identity-clean.
func configureGitIdentity(t *testing.T, repoPath string) {
	t.Helper()
	cfgs := [][]string{
		{"git", "-C", repoPath, "config", "user.email", "test@example.com"},
		{"git", "-C", repoPath, "config", "user.name", "test"},
	}
	for _, args := range cfgs {
		c := execCommand(args[0], args[1:]...)
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("setup %v: %v\noutput: %s", args, err, out)
		}
	}
}

// stageDirtyTracked overwrites a tracked file in repoPath with new content,
// leaving `git status --porcelain` non-empty (one " M" entry).
func stageDirtyTracked(t *testing.T, repoPath, name string) {
	t.Helper()
	path := filepath.Join(repoPath, name)
	if err := os.WriteFile(path, []byte("modified\n"), 0o644); err != nil {
		t.Fatalf("stage dirty %s: %v", path, err)
	}
}

// stageUntracked drops a fresh untracked file under repoPath.
func stageUntracked(t *testing.T, repoPath, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repoPath, name), []byte(body), 0o644); err != nil {
		t.Fatalf("stage untracked %s: %v", name, err)
	}
}

// gitOutput runs git with args in repoPath via execCommand (test scaffolding
// is exempt from Constitution I) and returns trimmed stdout; fails the test
// on non-zero exit.
func gitOutput(t *testing.T, repoPath string, args ...string) string {
	t.Helper()
	full := append([]string{"-C", repoPath}, args...)
	c := execCommand("git", full...)
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\noutput: %s", full, err, out)
	}
	return strings.TrimSpace(string(out))
}

// stageInitialTrackedFile creates and commits a tracked file in repoPath, then
// pushes the commit to origin so subsequent test edits produce a real "tracked
// file modification" working-tree dirty state.
func stageInitialTrackedFile(t *testing.T, repoPath, name string) {
	t.Helper()
	path := filepath.Join(repoPath, name)
	if err := os.WriteFile(path, []byte("initial\n"), 0o644); err != nil {
		t.Fatalf("stage initial %s: %v", path, err)
	}
	cmds := [][]string{
		{"git", "-C", repoPath, "add", name},
		{"git", "-C", repoPath, "commit", "-m", "add " + name},
		{"git", "-C", repoPath, "push", "origin", "HEAD"},
	}
	for _, args := range cmds {
		c := execCommand(args[0], args[1:]...)
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("setup %v: %v\noutput: %s", args, err, out)
		}
	}
}

// TestSyncCleanTreeRegressionEmitsTodaysLine verifies the clean-tree path is
// unchanged: no `committed,` token, no commas between summaries (spec A2, A8).
func TestSyncCleanTreeRegressionEmitsTodaysLine(t *testing.T) {
	tmp := t.TempDir()
	url, _ := initBareRepoWithCommit(t, tmp)
	_, defaultDir := fixtureGroup(t, "default", true)

	target := filepath.Join(defaultDir, "source")
	if _, _, err := runArgs(t, "clone", url); err != nil {
		t.Fatalf("setup clone: %v", err)
	}
	configureGitIdentity(t, target)

	_, stderr, err := runArgs(t, "sync", "source")
	if err != nil {
		t.Fatalf("hop sync source: %v\nstderr: %s", err, stderr.String())
	}
	got := stderr.String()
	if !strings.Contains(got, "sync: source ✓") {
		t.Fatalf("expected success status line, got: %s", got)
	}
	if strings.Contains(got, "committed,") {
		t.Fatalf("clean tree must not produce committed token, got: %s", got)
	}
}

// TestSyncDirtyTreeAutoCommitsWithDefaultMessage covers spec A1, A3 (default),
// A8 (dirty success line), and A11 (per-call timeouts via independent
// invocations — verified indirectly: the test runs to completion).
func TestSyncDirtyTreeAutoCommitsWithDefaultMessage(t *testing.T) {
	tmp := t.TempDir()
	url, _ := initBareRepoWithCommit(t, tmp)
	_, defaultDir := fixtureGroup(t, "default", true)

	target := filepath.Join(defaultDir, "source")
	if _, _, err := runArgs(t, "clone", url); err != nil {
		t.Fatalf("setup clone: %v", err)
	}
	configureGitIdentity(t, target)
	stageInitialTrackedFile(t, target, "tracked.txt")
	stageDirtyTracked(t, target, "tracked.txt")

	_, stderr, err := runArgs(t, "sync", "source")
	if err != nil {
		t.Fatalf("hop sync source: %v\nstderr: %s", err, stderr.String())
	}
	got := stderr.String()
	if !strings.Contains(got, "sync: source ✓ committed,") {
		t.Fatalf("expected dirty-tree committed line, got: %s", got)
	}

	// Default message must appear verbatim in the latest commit.
	if msg := gitOutput(t, target, "log", "-1", "--pretty=%B"); !strings.Contains(msg, "chore: sync via hop") {
		t.Fatalf("expected default commit message in HEAD, got: %s", msg)
	}

	// Working tree clean post-sync.
	if status := gitOutput(t, target, "status", "--porcelain"); status != "" {
		t.Fatalf("expected clean tree post-sync, got: %s", status)
	}
}

// TestSyncDirtyTreeIncludesUntrackedFiles covers spec A1 (git add --all) and
// asserts xpush parity (assumption #4).
func TestSyncDirtyTreeIncludesUntrackedFiles(t *testing.T) {
	tmp := t.TempDir()
	url, _ := initBareRepoWithCommit(t, tmp)
	_, defaultDir := fixtureGroup(t, "default", true)

	target := filepath.Join(defaultDir, "source")
	if _, _, err := runArgs(t, "clone", url); err != nil {
		t.Fatalf("setup clone: %v", err)
	}
	configureGitIdentity(t, target)
	stageInitialTrackedFile(t, target, "tracked.txt")
	stageDirtyTracked(t, target, "tracked.txt")
	stageUntracked(t, target, "untracked.txt", "fresh\n")

	_, stderr, err := runArgs(t, "sync", "source")
	if err != nil {
		t.Fatalf("hop sync source: %v\nstderr: %s", err, stderr.String())
	}
	if !strings.Contains(stderr.String(), "sync: source ✓ committed,") {
		t.Fatalf("expected committed token, got: %s", stderr.String())
	}

	// Both files must appear in the latest commit's tree.
	files := gitOutput(t, target, "log", "-1", "--name-only", "--pretty=")
	if !strings.Contains(files, "tracked.txt") {
		t.Errorf("expected tracked.txt in HEAD commit, got: %s", files)
	}
	if !strings.Contains(files, "untracked.txt") {
		t.Errorf("expected untracked.txt in HEAD commit, got: %s", files)
	}
}

// TestSyncDirtyTreeCustomMessageOverridesDefault covers spec A3.
func TestSyncDirtyTreeCustomMessageOverridesDefault(t *testing.T) {
	tmp := t.TempDir()
	url, _ := initBareRepoWithCommit(t, tmp)
	_, defaultDir := fixtureGroup(t, "default", true)

	target := filepath.Join(defaultDir, "source")
	if _, _, err := runArgs(t, "clone", url); err != nil {
		t.Fatalf("setup clone: %v", err)
	}
	configureGitIdentity(t, target)
	stageInitialTrackedFile(t, target, "tracked.txt")
	stageDirtyTracked(t, target, "tracked.txt")

	custom := "fix(zsh): reload prompt on chpwd"
	_, stderr, err := runArgs(t, "sync", "source", "-m", custom)
	if err != nil {
		t.Fatalf("hop sync source -m: %v\nstderr: %s", err, stderr.String())
	}
	if !strings.Contains(stderr.String(), "sync: source ✓ committed,") {
		t.Fatalf("expected committed token, got: %s", stderr.String())
	}

	msg := gitOutput(t, target, "log", "-1", "--pretty=%B")
	if !strings.Contains(msg, custom) {
		t.Fatalf("expected custom message in HEAD, got: %s", msg)
	}
	if strings.Contains(msg, "chore: sync via hop") {
		t.Fatalf("default message must not appear when -m is passed, got: %s", msg)
	}
}

// TestSyncCleanTreeWithMessageHasNoEffect covers spec A4.
func TestSyncCleanTreeWithMessageHasNoEffect(t *testing.T) {
	tmp := t.TempDir()
	url, _ := initBareRepoWithCommit(t, tmp)
	_, defaultDir := fixtureGroup(t, "default", true)

	target := filepath.Join(defaultDir, "source")
	if _, _, err := runArgs(t, "clone", url); err != nil {
		t.Fatalf("setup clone: %v", err)
	}
	configureGitIdentity(t, target)

	beforeSHA := gitOutput(t, target, "rev-parse", "HEAD")

	_, stderr, err := runArgs(t, "sync", "source", "-m", "would-be message")
	if err != nil {
		t.Fatalf("hop sync source -m: %v\nstderr: %s", err, stderr.String())
	}
	got := stderr.String()
	if strings.Contains(got, "committed,") {
		t.Fatalf("clean tree with -m must not produce committed token, got: %s", got)
	}
	if !strings.Contains(got, "sync: source ✓") {
		t.Fatalf("expected success line, got: %s", got)
	}

	afterSHA := gitOutput(t, target, "rev-parse", "HEAD")
	if beforeSHA != afterSHA {
		t.Fatalf("HEAD must not advance on clean tree, before=%s after=%s", beforeSHA, afterSHA)
	}
}

// TestSyncDirtyTreeMultiLineMessagePassesThrough covers spec scenario
// "Multi-line `-m` value passes through to git" (assumption #20).
func TestSyncDirtyTreeMultiLineMessagePassesThrough(t *testing.T) {
	tmp := t.TempDir()
	url, _ := initBareRepoWithCommit(t, tmp)
	_, defaultDir := fixtureGroup(t, "default", true)

	target := filepath.Join(defaultDir, "source")
	if _, _, err := runArgs(t, "clone", url); err != nil {
		t.Fatalf("setup clone: %v", err)
	}
	configureGitIdentity(t, target)
	stageInitialTrackedFile(t, target, "tracked.txt")
	stageDirtyTracked(t, target, "tracked.txt")

	multi := "subject\n\nbody line 1\nbody line 2"
	_, _, err := runArgs(t, "sync", "source", "-m", multi)
	if err != nil {
		t.Fatalf("hop sync source -m multi-line: %v", err)
	}

	subject := gitOutput(t, target, "log", "-1", "--pretty=%s")
	if subject != "subject" {
		t.Errorf("expected subject 'subject', got: %q", subject)
	}
	body := gitOutput(t, target, "log", "-1", "--pretty=%b")
	if !strings.Contains(body, "body line 1") || !strings.Contains(body, "body line 2") {
		t.Errorf("expected both body lines, got: %q", body)
	}
}

// TestSyncDirtyTreeEmptyMessageFailsViaGit covers spec scenario "Empty `-m`
// value falls back to git's own validation" (assumption #20). Git aborts the
// commit; hop emits `commit failed:` and skips the rebase/push.
func TestSyncDirtyTreeEmptyMessageFailsViaGit(t *testing.T) {
	tmp := t.TempDir()
	url, _ := initBareRepoWithCommit(t, tmp)
	_, defaultDir := fixtureGroup(t, "default", true)

	target := filepath.Join(defaultDir, "source")
	if _, _, err := runArgs(t, "clone", url); err != nil {
		t.Fatalf("setup clone: %v", err)
	}
	configureGitIdentity(t, target)
	stageInitialTrackedFile(t, target, "tracked.txt")
	stageDirtyTracked(t, target, "tracked.txt")

	_, stderr, err := runArgs(t, "sync", "source", "-m", "")
	if err == nil {
		t.Fatalf("expected non-nil err on empty commit message")
	}
	got := stderr.String()
	if !strings.Contains(got, "sync: source ✗ commit failed:") {
		t.Fatalf("expected commit-failed line, got: %s", got)
	}
	// Sanity: the existing rebase-conflict / push-failed lines must NOT appear.
	if strings.Contains(got, "rebase conflict") || strings.Contains(got, "push failed:") {
		t.Fatalf("commit-step failure must not surface downstream errors, got: %s", got)
	}
}

// TestSyncDirtyTreePreCommitHookFailureSkipsPushPull covers spec A6 and A7:
// hook rejection ⇒ no rebase, no push, dedicated commit-failed line.
func TestSyncDirtyTreePreCommitHookFailureSkipsPushPull(t *testing.T) {
	tmp := t.TempDir()
	url, _ := initBareRepoWithCommit(t, tmp)
	_, defaultDir := fixtureGroup(t, "default", true)

	target := filepath.Join(defaultDir, "source")
	if _, _, err := runArgs(t, "clone", url); err != nil {
		t.Fatalf("setup clone: %v", err)
	}
	configureGitIdentity(t, target)
	stageInitialTrackedFile(t, target, "tracked.txt")
	stageDirtyTracked(t, target, "tracked.txt")

	// Install a pre-commit hook that always exits 1.
	hookPath := filepath.Join(target, ".git", "hooks", "pre-commit")
	hook := "#!/bin/sh\necho 'gofmt: bad formatting in foo.go' >&2\nexit 1\n"
	if err := os.WriteFile(hookPath, []byte(hook), 0o755); err != nil {
		t.Fatalf("install hook: %v", err)
	}

	beforeSHA := gitOutput(t, target, "rev-parse", "HEAD")

	_, stderr, err := runArgs(t, "sync", "source")
	if err == nil {
		t.Fatalf("expected non-nil err on hook failure")
	}
	got := stderr.String()
	if !strings.Contains(got, "sync: source ✗ commit failed:") {
		t.Fatalf("expected commit-failed line, got: %s", got)
	}

	// HEAD must not move (commit refused, push could not have run).
	afterSHA := gitOutput(t, target, "rev-parse", "HEAD")
	if beforeSHA != afterSHA {
		t.Fatalf("HEAD must not advance on hook-rejected commit; before=%s after=%s", beforeSHA, afterSHA)
	}
}

// TestSyncRebaseConflictAfterAutoCommit covers spec A9 (case 7): on a dirty
// tree, the auto-commit step succeeds, but the subsequent `git pull --rebase`
// hits a conflict against a divergent upstream commit. The existing
// rebase-conflict line MUST surface unchanged, push MUST NOT run, and the
// local commit produced by the auto-commit step MUST remain in the local repo
// (visible via reflog / ORIG_HEAD per rebase semantics — assumption #21: no
// rollback).
func TestSyncRebaseConflictAfterAutoCommit(t *testing.T) {
	tmp := t.TempDir()
	url, barePath := initBareRepoWithCommit(t, tmp)
	_, defaultDir := fixtureGroup(t, "default", true)

	target := filepath.Join(defaultDir, "source")
	if _, _, err := runArgs(t, "clone", url); err != nil {
		t.Fatalf("setup clone: %v", err)
	}
	configureGitIdentity(t, target)
	// Seed a tracked file in the upstream so both sides have a common base.
	stageInitialTrackedFile(t, target, "tracked.txt")

	// Push a divergent commit to the bare from a separate stage clone — same
	// file, different content so a `pull --rebase` of any local change to that
	// file produces a CONFLICT.
	divergent := filepath.Join(tmp, "divergent-stage")
	cmds := [][]string{
		{"git", "clone", barePath, divergent},
		{"git", "-C", divergent, "config", "user.email", "test@example.com"},
		{"git", "-C", divergent, "config", "user.name", "test"},
	}
	for _, args := range cmds {
		c := execCommand(args[0], args[1:]...)
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("setup %v: %v\noutput: %s", args, err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(divergent, "tracked.txt"), []byte("remote-content\n"), 0o644); err != nil {
		t.Fatalf("write divergent tracked.txt: %v", err)
	}
	cmds = [][]string{
		{"git", "-C", divergent, "add", "tracked.txt"},
		{"git", "-C", divergent, "commit", "-m", "remote-side change"},
		{"git", "-C", divergent, "push", "origin", "HEAD:refs/heads/main"},
	}
	for _, args := range cmds {
		c := execCommand(args[0], args[1:]...)
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("setup %v: %v\noutput: %s", args, err, out)
		}
	}

	// Capture the bare's HEAD ref at this point — the post-conflict assertion
	// confirms it has NOT moved (no push happened).
	bareHEADBefore := gitOutput(t, barePath, "rev-parse", "HEAD")

	// Make target dirty on the SAME file with conflicting content.
	if err := os.WriteFile(filepath.Join(target, "tracked.txt"), []byte("local-content\n"), 0o644); err != nil {
		t.Fatalf("dirty tracked.txt: %v", err)
	}

	_, stderr, err := runArgs(t, "sync", "source")
	if err == nil {
		t.Fatalf("expected non-nil err on rebase conflict")
	}
	got := stderr.String()

	// Existing rebase-conflict line MUST surface unchanged (spec A9).
	if !strings.Contains(got, "sync: source ✗ rebase conflict — resolve manually with: git -C ") {
		t.Fatalf("expected rebase-conflict line, got: %s", got)
	}
	if !strings.Contains(got, "rebase --continue") {
		t.Fatalf("expected rebase-continue hint, got: %s", got)
	}
	// commit-failed line MUST NOT appear (commit succeeded).
	if strings.Contains(got, "commit failed:") {
		t.Fatalf("commit succeeded; commit-failed line must not appear, got: %s", got)
	}
	// push-failed line MUST NOT appear (push must not run after a rebase
	// conflict).
	if strings.Contains(got, "push failed:") {
		t.Fatalf("push must not run after rebase conflict, got: %s", got)
	}

	// Push MUST NOT have run — bare's HEAD unchanged from the divergent
	// commit pushed pre-sync.
	bareHEADAfter := gitOutput(t, barePath, "rev-parse", "HEAD")
	if bareHEADBefore != bareHEADAfter {
		t.Fatalf("push must not run after rebase conflict; bare HEAD before=%s after=%s", bareHEADBefore, bareHEADAfter)
	}

	// The local auto-commit MUST remain — git rebase preserves the original
	// pre-rebase HEAD in ORIG_HEAD. Verify the commit message is present.
	origMsg := gitOutput(t, target, "log", "ORIG_HEAD", "-1", "--pretty=%B")
	if !strings.Contains(origMsg, "chore: sync via hop") {
		t.Fatalf("expected auto-commit preserved at ORIG_HEAD, got: %s", origMsg)
	}
	// `git add --all` succeeded — the (now staged into commit) modification
	// is no longer in the working tree as untracked/modified outside the
	// in-progress rebase. During the conflict, the conflicted file is staged
	// with conflict markers; status will show "UU" or similar — accept any
	// non-empty status as long as it does NOT show our pre-add " M" marker
	// for tracked.txt (i.e., the add+commit ran).
	postStatus := gitOutput(t, target, "status", "--porcelain")
	if strings.Contains(postStatus, " M tracked.txt") {
		t.Fatalf("git add --all did not run; tracked.txt still shows ' M' marker: %q", postStatus)
	}
}

// TestSyncPushFailAfterAutoCommit covers spec A9 (case 8): on a dirty tree,
// the auto-commit and pull --rebase both succeed, but the subsequent push is
// rejected by a pre-receive hook on the bare. The existing push-failed line
// MUST surface unchanged, exit non-zero, and the local commit MUST remain in
// HEAD (assumption #21: no rollback).
func TestSyncPushFailAfterAutoCommit(t *testing.T) {
	tmp := t.TempDir()
	url, barePath := initBareRepoWithCommit(t, tmp)
	_, defaultDir := fixtureGroup(t, "default", true)

	target := filepath.Join(defaultDir, "source")
	if _, _, err := runArgs(t, "clone", url); err != nil {
		t.Fatalf("setup clone: %v", err)
	}
	configureGitIdentity(t, target)
	stageInitialTrackedFile(t, target, "tracked.txt")
	stageDirtyTracked(t, target, "tracked.txt")

	// Install a pre-receive hook on the BARE that always rejects pushes. This
	// produces a clean push-failure path without needing to engineer a
	// non-fast-forward via concurrent commits.
	hookPath := filepath.Join(barePath, "hooks", "pre-receive")
	hook := "#!/bin/sh\necho 'remote: pushes are rejected by policy' >&2\nexit 1\n"
	if err := os.WriteFile(hookPath, []byte(hook), 0o755); err != nil {
		t.Fatalf("install pre-receive hook: %v", err)
	}

	beforeHEAD := gitOutput(t, target, "rev-parse", "HEAD")

	_, stderr, err := runArgs(t, "sync", "source")
	if err == nil {
		t.Fatalf("expected non-nil err on push failure")
	}
	got := stderr.String()

	// Existing push-failed line MUST surface unchanged (spec A9).
	if !strings.Contains(got, "sync: source ✗ push failed:") {
		t.Fatalf("expected push-failed line, got: %s", got)
	}
	// Commit and rebase MUST NOT have logged failure lines.
	if strings.Contains(got, "commit failed:") {
		t.Fatalf("commit succeeded; commit-failed line must not appear, got: %s", got)
	}
	if strings.Contains(got, "rebase conflict") {
		t.Fatalf("rebase succeeded; rebase-conflict line must not appear, got: %s", got)
	}

	// Auto-commit MUST be present in the local repo's HEAD (assumption #21:
	// no rollback after push failure).
	afterHEAD := gitOutput(t, target, "rev-parse", "HEAD")
	if beforeHEAD == afterHEAD {
		t.Fatalf("expected HEAD to advance via auto-commit; HEAD unchanged at %s", afterHEAD)
	}
	headMsg := gitOutput(t, target, "log", "-1", "--pretty=%B")
	if !strings.Contains(headMsg, "chore: sync via hop") {
		t.Fatalf("expected auto-commit at HEAD, got: %s", headMsg)
	}
}

// TestSyncFlagRejectedOnPush covers spec A5 (push side).
func TestSyncFlagRejectedOnPush(t *testing.T) {
	_, _, _ = pullSyncYAMLFixture(t)

	_, _, err := runArgs(t, "push", "alpha", "-m", "anything")
	if err == nil {
		t.Fatalf("expected cobra to reject -m on push")
	}
	if !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("expected unknown-flag error, got: %v", err)
	}
}

// TestSyncFlagRejectedOnPull covers spec A5 (pull side).
func TestSyncFlagRejectedOnPull(t *testing.T) {
	_, _, _ = pullSyncYAMLFixture(t)

	_, _, err := runArgs(t, "pull", "alpha", "-m", "anything")
	if err == nil {
		t.Fatalf("expected cobra to reject -m on pull")
	}
	if !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("expected unknown-flag error, got: %v", err)
	}
}

// initNamedBareRepoWithCommit is a variant of initBareRepoWithCommit that
// names the bare repo (and hence the inferred clone name) per caller. Returns
// the file:// URL. Used by the batch-mode test to create three distinctly
// named clones in the same fixture group without collisions.
func initNamedBareRepoWithCommit(t *testing.T, dir, name string) string {
	t.Helper()
	// Mirror initBareRepo's guard so this helper degrades the same way when
	// git is absent (skip rather than fail) — keeps the test suite consistent
	// across all init* helpers.
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("setup tmp %s: %v", dir, err)
	}
	bare := filepath.Join(dir, name+".git")
	cmds := [][]string{
		{"git", "init", "--bare", bare},
	}
	for _, args := range cmds {
		c := execCommand(args[0], args[1:]...)
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("init bare %s: %v\noutput: %s", name, err, out)
		}
	}

	stage := filepath.Join(dir, name+"-stage")
	cmds = [][]string{
		{"git", "clone", bare, stage},
		{"git", "-C", stage, "-c", "user.email=test@example.com", "-c", "user.name=test", "commit", "--allow-empty", "-m", "init"},
		{"git", "-C", stage, "push", "origin", "HEAD:refs/heads/main"},
	}
	for _, args := range cmds {
		c := execCommand(args[0], args[1:]...)
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("setup %v: %v\noutput: %s", args, err, out)
		}
	}
	return "file://" + bare
}

// TestSyncBatchMixedCleanDirtyHookFailure covers spec A10 (batch summary
// across mixed outcomes) and the `committed,` token co-existing with the
// today-shape clean-tree line in the same batch.
func TestSyncBatchMixedCleanDirtyHookFailure(t *testing.T) {
	tmp := t.TempDir()
	urlAlpha := initNamedBareRepoWithCommit(t, tmp, "alpha")
	urlBeta := initNamedBareRepoWithCommit(t, tmp, "beta")
	urlGamma := initNamedBareRepoWithCommit(t, tmp, "gamma")

	groupDir := t.TempDir()
	yaml := "repos:\n" +
		"  default:\n" +
		"    dir: " + groupDir + "\n" +
		"    urls:\n" +
		"      - " + urlAlpha + "\n" +
		"      - " + urlBeta + "\n" +
		"      - " + urlGamma + "\n"
	writeReposFixture(t, yaml)

	// Clone all three.
	for _, u := range []string{urlAlpha, urlBeta, urlGamma} {
		if _, _, err := runArgs(t, "clone", u); err != nil {
			t.Fatalf("setup clone %s: %v", u, err)
		}
	}
	alphaDir := filepath.Join(groupDir, "alpha")
	betaDir := filepath.Join(groupDir, "beta")
	gammaDir := filepath.Join(groupDir, "gamma")
	for _, d := range []string{alphaDir, betaDir, gammaDir} {
		configureGitIdentity(t, d)
	}

	// alpha: dirty (commit + push expected to succeed).
	stageInitialTrackedFile(t, alphaDir, "tracked.txt")
	stageDirtyTracked(t, alphaDir, "tracked.txt")
	// beta: clean — no edits.
	// gamma: dirty + pre-commit hook that fails.
	stageInitialTrackedFile(t, gammaDir, "tracked.txt")
	stageDirtyTracked(t, gammaDir, "tracked.txt")
	hookPath := filepath.Join(gammaDir, ".git", "hooks", "pre-commit")
	hook := "#!/bin/sh\nexit 1\n"
	if err := os.WriteFile(hookPath, []byte(hook), 0o755); err != nil {
		t.Fatalf("install gamma hook: %v", err)
	}

	_, stderr, err := runArgs(t, "sync", "--all")
	if err == nil {
		t.Fatalf("expected errSilent for batch with one failed repo")
	}
	got := stderr.String()

	// alpha must succeed with committed token; beta clean-shape; gamma fails.
	if !strings.Contains(got, "sync: alpha ✓ committed,") {
		t.Errorf("expected alpha committed line, got: %s", got)
	}
	if !strings.Contains(got, "sync: beta ✓ ") || strings.Contains(got, "sync: beta ✓ committed,") {
		t.Errorf("expected beta clean-shape line (no committed token), got: %s", got)
	}
	if !strings.Contains(got, "sync: gamma ✗ commit failed:") {
		t.Errorf("expected gamma commit-failed line, got: %s", got)
	}
	if !strings.Contains(got, "summary: synced=2 skipped=0 failed=1") {
		t.Errorf("expected summary synced=2 skipped=0 failed=1, got: %s", got)
	}
}
