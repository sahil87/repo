package scan

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sahil87/hop/internal/proc"
)

// fakeRunner is a deterministic in-memory GitRunner for tests. Keyed by
// `dir + "\x00" + strings.Join(args, " ")`.
type fakeRunner struct {
	responses map[string]fakeResp
	calls     []fakeCall
}

type fakeResp struct {
	out []byte
	err error
}

type fakeCall struct {
	dir  string
	args []string
}

func newFakeRunner(responses map[string]fakeResp) *fakeRunner {
	return &fakeRunner{responses: responses}
}

func (r *fakeRunner) Run(ctx context.Context, dir string, args ...string) ([]byte, error) {
	r.calls = append(r.calls, fakeCall{dir: dir, args: append([]string(nil), args...)})
	key := dir + "\x00" + strings.Join(args, " ")
	if resp, ok := r.responses[key]; ok {
		return resp.out, resp.err
	}
	return nil, errors.New("fakeRunner: no response configured for " + key)
}

// makeRepo creates dir/.git as a directory (a "normal" git repo on the
// classifier's terms — it doesn't have to be a real git repo for tests).
func makeRepo(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatalf("makeRepo: %v", err)
	}
}

// makeWorktree creates dir with a `.git` file (the worktree marker).
func makeWorktree(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("makeWorktree mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: /elsewhere\n"), 0o644); err != nil {
		t.Fatalf("makeWorktree write: %v", err)
	}
}

// makeBare creates dir with HEAD + config + objects/, no .git subdir.
func makeBare(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, "objects"), 0o755); err != nil {
		t.Fatalf("makeBare mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatalf("makeBare HEAD: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config"), []byte("[core]\n"), 0o644); err != nil {
		t.Fatalf("makeBare config: %v", err)
	}
}

// remoteResp builds a fakeResp for `git remote` listing the given names, one
// per line (matches `git remote`'s actual output shape).
func remoteResp(names ...string) fakeResp {
	return fakeResp{out: []byte(strings.Join(names, "\n") + "\n")}
}

// urlResp builds a fakeResp for `git remote get-url <name>`.
func urlResp(url string) fakeResp {
	return fakeResp{out: []byte(url + "\n")}
}

// keyRemote builds the lookup key used by fakeRunner for `git remote`.
func keyRemote(dir string) string {
	return dir + "\x00" + "remote"
}

// keyGetURL builds the lookup key for `git remote get-url <name>`.
func keyGetURL(dir, name string) string {
	return dir + "\x00" + "remote get-url " + name
}

// canon returns the EvalSymlinks-resolved version of path (matches what Walk
// stores in Found.Path). Tests on macOS need this because /var/folders/...
// resolves through /private/var/folders/... .
func canon(t *testing.T, p string) string {
	t.Helper()
	c, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatalf("EvalSymlinks %s: %v", p, err)
	}
	return c
}

func TestWalkFindsConventionLayout(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "sahil87", "hop")
	makeRepo(t, repoDir)
	canonRepo := canon(t, repoDir)

	runner := newFakeRunner(map[string]fakeResp{
		keyRemote(canonRepo):                 remoteResp("origin"),
		keyGetURL(canonRepo, "origin"):       urlResp("git@github.com:sahil87/hop.git"),
	})

	found, skips, err := Walk(context.Background(), root, Options{Depth: 3, GitRunner: runner.Run})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if len(found) != 1 {
		t.Fatalf("expected 1 found, got %d: %#v", len(found), found)
	}
	if found[0].URL != "git@github.com:sahil87/hop.git" {
		t.Fatalf("URL = %q", found[0].URL)
	}
	if found[0].Path != canonRepo {
		t.Fatalf("Path = %q, want %q", found[0].Path, canonRepo)
	}
	if len(skips) != 0 {
		t.Fatalf("expected 0 skips, got %#v", skips)
	}
}

func TestWalkOriginPreferredOverFirst(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "a", "repo")
	makeRepo(t, repoDir)
	canonRepo := canon(t, repoDir)

	runner := newFakeRunner(map[string]fakeResp{
		keyRemote(canonRepo):           remoteResp("upstream", "origin", "fork"),
		keyGetURL(canonRepo, "origin"): urlResp("git@github.com:owner/origin-url.git"),
	})

	found, _, err := Walk(context.Background(), root, Options{Depth: 3, GitRunner: runner.Run})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if len(found) != 1 || found[0].URL != "git@github.com:owner/origin-url.git" {
		t.Fatalf("expected origin URL, got %#v", found)
	}
}

func TestWalkNonOriginFirstRemote(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "x")
	makeRepo(t, repoDir)
	canonRepo := canon(t, repoDir)

	runner := newFakeRunner(map[string]fakeResp{
		keyRemote(canonRepo):            remoteResp("gitlab", "fork"),
		keyGetURL(canonRepo, "gitlab"):  urlResp("git@gitlab.com:owner/x.git"),
	})

	found, _, err := Walk(context.Background(), root, Options{Depth: 3, GitRunner: runner.Run})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if len(found) != 1 || found[0].URL != "git@gitlab.com:owner/x.git" {
		t.Fatalf("expected gitlab URL, got %#v", found)
	}
}

func TestWalkNoRemote(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "scratch")
	makeRepo(t, repoDir)
	canonRepo := canon(t, repoDir)

	runner := newFakeRunner(map[string]fakeResp{
		keyRemote(canonRepo): {out: []byte("")},
	})

	found, skips, err := Walk(context.Background(), root, Options{Depth: 3, GitRunner: runner.Run})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if len(found) != 0 {
		t.Fatalf("expected 0 found, got %#v", found)
	}
	if len(skips) != 1 || skips[0].Reason != ReasonNoRemote {
		t.Fatalf("expected no-remote skip, got %#v", skips)
	}
	if skips[0].Path != canonRepo {
		t.Fatalf("skip path = %q, want %q", skips[0].Path, canonRepo)
	}
}

func TestWalkWorktreeSkipped(t *testing.T) {
	root := t.TempDir()
	wt := filepath.Join(root, "feature")
	makeWorktree(t, wt)

	runner := newFakeRunner(nil) // should never be called

	found, skips, err := Walk(context.Background(), root, Options{Depth: 3, GitRunner: runner.Run})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if len(found) != 0 {
		t.Fatalf("expected 0 found, got %#v", found)
	}
	if len(skips) != 1 || skips[0].Reason != ReasonWorktree {
		t.Fatalf("expected worktree skip, got %#v", skips)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("git should not be invoked for worktrees, got %d calls", len(runner.calls))
	}
}

func TestWalkBareRepoSkipped(t *testing.T) {
	root := t.TempDir()
	bare := filepath.Join(root, "old-mirror.git")
	makeBare(t, bare)

	runner := newFakeRunner(nil) // should never be called

	found, skips, err := Walk(context.Background(), root, Options{Depth: 3, GitRunner: runner.Run})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if len(found) != 0 {
		t.Fatalf("expected 0 found, got %#v", found)
	}
	if len(skips) != 1 || skips[0].Reason != ReasonBareRepo {
		t.Fatalf("expected bare-repo skip, got %#v", skips)
	}
}

// TestWalkRegisteredRepoNotDescended pins the no-descent invariant: once a
// repo is registered, its children are not visited, so a submodule under it
// is silently never seen and never produces a Skip entry. This is the sole
// submodule guard (per spec assumption #17 and the dirClass note in scan.go).
func TestWalkRegisteredRepoNotDescended(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Join(root, "a")
	makeRepo(t, parent)
	// Submodule under parent — would otherwise classify as a repo.
	submodule := filepath.Join(parent, "vendor", "lib")
	makeRepo(t, submodule)
	canonParent := canon(t, parent)

	runner := newFakeRunner(map[string]fakeResp{
		keyRemote(canonParent):           remoteResp("origin"),
		keyGetURL(canonParent, "origin"): urlResp("git@github.com:owner/a.git"),
	})

	found, skips, err := Walk(context.Background(), root, Options{Depth: 5, GitRunner: runner.Run})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if len(found) != 1 || found[0].Path != canonParent {
		t.Fatalf("expected single Found at parent, got %#v", found)
	}
	// Submodule is silently skipped (no Skip entry per the no-descent
	// invariant) — assumption #17 documents this is acceptable.
	for _, s := range skips {
		if s.Reason == ReasonSubmodule {
			t.Fatalf("did not expect explicit submodule skip under no-descent invariant; got %#v", s)
		}
	}
}

// (TestClassifyDirSubmoduleViaAncestorFlag was removed when the dual
// submodule-detection logic was simplified — TestWalkRegisteredRepoNotDescended
// already pins the no-descent invariant that is the sole submodule guard.)

func TestWalkDepthLimitExcludesDeeperRepos(t *testing.T) {
	root := t.TempDir()
	// Repo at depth 4 from root: root/a/b/c/d/.git
	deep := filepath.Join(root, "a", "b", "c", "d")
	makeRepo(t, deep)

	runner := newFakeRunner(map[string]fakeResp{}) // never invoked

	found, _, err := Walk(context.Background(), root, Options{Depth: 3, GitRunner: runner.Run})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if len(found) != 0 {
		t.Fatalf("expected 0 found at depth 3, got %#v", found)
	}
}

func TestWalkDepthLimitIncludesDepth3Repo(t *testing.T) {
	root := t.TempDir()
	// Repo at depth 3 from root: root/a/b/c/.git
	repoDir := filepath.Join(root, "a", "b", "c")
	makeRepo(t, repoDir)
	canonRepo := canon(t, repoDir)

	runner := newFakeRunner(map[string]fakeResp{
		keyRemote(canonRepo):           remoteResp("origin"),
		keyGetURL(canonRepo, "origin"): urlResp("git@github.com:owner/c.git"),
	})

	found, _, err := Walk(context.Background(), root, Options{Depth: 3, GitRunner: runner.Run})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if len(found) != 1 {
		t.Fatalf("expected 1 found, got %#v", found)
	}
}

func TestWalkSymlinkLoopDedup(t *testing.T) {
	root := t.TempDir()
	// Create a self-referential loop: root/a → root/a (symlink to itself
	// indirectly). Simpler: root/a is a real dir, root/b is a symlink to a,
	// root/a/c is a symlink to root (loop). Walk should not infinite-loop.
	a := filepath.Join(root, "a")
	if err := os.MkdirAll(a, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Symlink(root, filepath.Join(a, "back-to-root")); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	runner := newFakeRunner(map[string]fakeResp{}) // never invoked

	// If dedup fails, Walk runs forever (the symlink loop has finite depth
	// in path-components but creates infinite (path, depth) frames). Run in
	// a goroutine and assert termination within a generous bound — a real
	// failure manifests as a hang the timeout converts into a clear failure
	// rather than the test runner's eventual hard kill.
	done := make(chan struct{})
	var err error
	go func() {
		_, _, err = Walk(context.Background(), root, Options{Depth: 5, GitRunner: runner.Run})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Walk did not terminate within 3s — symlink-loop dedup likely broken")
	}
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
}

func TestWalkSameRepoViaTwoPathsCanonical(t *testing.T) {
	root := t.TempDir()
	real := filepath.Join(root, "real-repo")
	makeRepo(t, real)
	canonReal := canon(t, real)

	// Symlink at root/alias → real-repo. Walk should register the repo
	// exactly once because (dev, ino) dedup catches it.
	if err := os.Symlink(real, filepath.Join(root, "alias")); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	runner := newFakeRunner(map[string]fakeResp{
		keyRemote(canonReal):           remoteResp("origin"),
		keyGetURL(canonReal, "origin"): urlResp("git@github.com:owner/real.git"),
	})

	found, _, err := Walk(context.Background(), root, Options{Depth: 3, GitRunner: runner.Run})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if len(found) != 1 {
		t.Fatalf("expected exactly 1 Found (dedup), got %d: %#v", len(found), found)
	}
	if found[0].Path != canonReal {
		t.Fatalf("Found.Path = %q, want canonical %q", found[0].Path, canonReal)
	}
}

func TestWalkGitMissingPropagates(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "r")
	makeRepo(t, repoDir)
	canonRepo := canon(t, repoDir)

	runner := newFakeRunner(map[string]fakeResp{
		keyRemote(canonRepo): {err: proc.ErrNotFound},
	})

	_, _, err := Walk(context.Background(), root, Options{Depth: 3, GitRunner: runner.Run})
	if err == nil {
		t.Fatal("expected error when git is missing, got nil")
	}
	if !errors.Is(err, proc.ErrNotFound) {
		t.Fatalf("expected errors.Is(err, proc.ErrNotFound), got %v", err)
	}
}

func TestWalkEmptyTreeSucceedsWithoutGit(t *testing.T) {
	root := t.TempDir()
	// No .git anywhere — git is never invoked even if the runner would error.
	runner := newFakeRunner(map[string]fakeResp{
		"any\x00remote": {err: proc.ErrNotFound},
	})

	found, skips, err := Walk(context.Background(), root, Options{Depth: 3, GitRunner: runner.Run})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if len(found) != 0 || len(skips) != 0 {
		t.Fatalf("expected empty results, got found=%#v skips=%#v", found, skips)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("git should not be invoked for empty tree, got %d calls", len(runner.calls))
	}
}

func TestWalkDiscoveryOrderIsLexical(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"zeta", "alpha", "mango"} {
		makeRepo(t, filepath.Join(root, "owner", name))
	}

	runner := newFakeRunner(map[string]fakeResp{})
	for _, name := range []string{"zeta", "alpha", "mango"} {
		c := canon(t, filepath.Join(root, "owner", name))
		runner.responses[keyRemote(c)] = remoteResp("origin")
		runner.responses[keyGetURL(c, "origin")] = urlResp("git@github.com:owner/" + name + ".git")
	}

	found, _, err := Walk(context.Background(), root, Options{Depth: 3, GitRunner: runner.Run})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	got := make([]string, len(found))
	for i, f := range found {
		got[i] = filepath.Base(f.Path)
	}
	want := []string{"alpha", "mango", "zeta"}
	if !equalStrings(got, want) {
		t.Fatalf("discovery order = %v, want %v (lexical sort of subdirs)", got, want)
	}
}

func TestWalkDepthZeroOnlyRoot(t *testing.T) {
	root := t.TempDir()
	makeRepo(t, root) // root itself is a repo
	canonRoot := canon(t, root)

	runner := newFakeRunner(map[string]fakeResp{
		keyRemote(canonRoot):           remoteResp("origin"),
		keyGetURL(canonRoot, "origin"): urlResp("git@github.com:owner/root.git"),
	})

	found, _, err := Walk(context.Background(), root, Options{Depth: 0, GitRunner: runner.Run})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if len(found) != 1 || found[0].Path != canonRoot {
		t.Fatalf("expected root-only repo, got %#v", found)
	}
}

func TestWalkDefaultRunnerBindsProcRunCapture(t *testing.T) {
	// When Options.GitRunner is nil, Walk uses the default runner. We can't
	// easily exercise the actual subprocess here (would require git on PATH),
	// but we can confirm the default doesn't blow up on an empty tree (no git
	// invocations needed).
	root := t.TempDir()
	found, skips, err := Walk(context.Background(), root, Options{Depth: 3})
	if err != nil {
		t.Fatalf("Walk with nil runner on empty tree: %v", err)
	}
	if len(found) != 0 || len(skips) != 0 {
		t.Fatalf("expected empty results, got found=%#v skips=%#v", found, skips)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
