package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sahil87/hop/internal/proc"
	"github.com/sahil87/hop/internal/repos"
)

// fixture used by several tests; using an absolute dir for the group's `dir`
// keeps assertions simple (no $HOME expansion needed).
const singleRepoYAML = `repos:
  default:
    dir: /tmp/test-repos
    urls:
      - git@github.com:sahil87/hop.git
`

func TestPathSubcommandRemoved(t *testing.T) {
	writeReposFixture(t, singleRepoYAML)

	_, _, err := runArgs(t, "path", "hop")
	if err == nil {
		t.Fatalf("expected error for removed `path` subcommand")
	}
}

func TestWhereSubcommandRemoved(t *testing.T) {
	writeReposFixture(t, singleRepoYAML)

	// `hop where <name>` was removed in favor of `hop <name> where` (repo-verb grammar).
	// The legacy 2-arg form `hop where hop` is now interpreted as $1="where" (treated
	// as a repo name since `where` is no longer a known subcommand) and $2="hop"
	// (which is neither `cd`, `where`, nor `-R`), so RunE's tool-form branch fires
	// with the "not a hop verb" hint.
	_, _, err := runArgs(t, "where", "hop")
	if err == nil {
		t.Fatalf("expected error for removed `where` subcommand form")
	}
	if !strings.Contains(err.Error(), "is not a hop verb") {
		t.Fatalf("expected tool-form-hint error, got %q (type %T)", err.Error(), err)
	}
}

func TestCdSubcommandRemoved(t *testing.T) {
	writeReposFixture(t, singleRepoYAML)

	// `hop cd <name>` was removed. Same grammar — $1=cd, $2=hop, otherwise → tool-form hint.
	_, _, err := runArgs(t, "cd", "hop")
	if err == nil {
		t.Fatalf("expected error for removed `cd` subcommand form")
	}
	if !strings.Contains(err.Error(), "is not a hop verb") {
		t.Fatalf("expected tool-form-hint error, got %q", err.Error())
	}
}

func TestBuildPickerLinesGroupSuffixOnCollision(t *testing.T) {
	rs := repos.Repos{
		{Name: "widget", Group: "default", Path: "/d/widget", URL: "git@h:o/widget.git"},
		{Name: "widget", Group: "vendor", Path: "/v/widget", URL: "git@h:v/widget.git"},
		{Name: "lone", Group: "default", Path: "/d/lone", URL: "git@h:o/lone.git"},
	}
	got := buildPickerLines(rs)
	if len(got) != 3 {
		t.Fatalf("got %d lines, want 3", len(got))
	}

	// Colliding names → "[group]" suffix.
	if !strings.HasPrefix(got[0], "widget [default]\t") {
		t.Errorf("got[0] = %q, want widget [default] prefix", got[0])
	}
	if !strings.HasPrefix(got[1], "widget [vendor]\t") {
		t.Errorf("got[1] = %q, want widget [vendor] prefix", got[1])
	}

	// Unique name → no suffix.
	if !strings.HasPrefix(got[2], "lone\t") || strings.HasPrefix(got[2], "lone [") {
		t.Errorf("got[2] = %q, want plain 'lone' prefix (no [group] suffix)", got[2])
	}

	// Match-back path is the second tab-separated column.
	for i, line := range got {
		parts := strings.Split(line, "\t")
		if len(parts) < 3 {
			t.Errorf("line %d not 3-column: %q", i, line)
		}
		if parts[1] != rs[i].Path {
			t.Errorf("line %d path column = %q, want %q", i, parts[1], rs[i].Path)
		}
	}
}

// resolveTargetsYAML defines a default group with two repos and a vendor
// group with one repo. Used by the resolveTargets unit tests.
const resolveTargetsYAML = `repos:
  default:
    dir: /tmp/test-resolve-targets
    urls:
      - git@github.com:sahil87/alpha.git
      - git@github.com:sahil87/beta.git
  vendor:
    dir: /tmp/test-resolve-targets-vendor
    urls:
      - git@github.com:vendor/gamma.git
`

func TestResolveTargetsAllReturnsBatchOverFullRegistry(t *testing.T) {
	writeReposFixture(t, resolveTargetsYAML)

	rs, mode, err := resolveTargets("", true)
	if err != nil {
		t.Fatalf("resolveTargets all: %v", err)
	}
	if mode != modeBatch {
		t.Fatalf("expected modeBatch, got %v", mode)
	}
	if len(rs) != 3 {
		t.Fatalf("expected 3 repos, got %d", len(rs))
	}
	// Source order: alpha, beta, gamma.
	if rs[0].Name != "alpha" || rs[1].Name != "beta" || rs[2].Name != "gamma" {
		t.Fatalf("expected alpha,beta,gamma order; got %s,%s,%s", rs[0].Name, rs[1].Name, rs[2].Name)
	}
}

func TestResolveTargetsExactGroupMatchReturnsBatchOfGroup(t *testing.T) {
	writeReposFixture(t, resolveTargetsYAML)

	rs, mode, err := resolveTargets("vendor", false)
	if err != nil {
		t.Fatalf("resolveTargets vendor: %v", err)
	}
	if mode != modeBatch {
		t.Fatalf("expected modeBatch, got %v", mode)
	}
	if len(rs) != 1 {
		t.Fatalf("expected 1 repo (gamma) in vendor batch, got %d", len(rs))
	}
	if rs[0].Name != "gamma" {
		t.Fatalf("expected gamma, got %s", rs[0].Name)
	}
}

func TestResolveTargetsSubstringMatchFallsThroughToSingle(t *testing.T) {
	writeReposFixture(t, resolveTargetsYAML)

	// "alph" matches alpha by substring; not a group name.
	rs, mode, err := resolveTargets("alph", false)
	if err != nil {
		t.Fatalf("resolveTargets alph: %v", err)
	}
	if mode != modeSingle {
		t.Fatalf("expected modeSingle, got %v", mode)
	}
	if len(rs) != 1 || rs[0].Name != "alpha" {
		t.Fatalf("expected single alpha, got %v", rs)
	}
}

func TestResolveTargetsGroupNameWinsOverRepoSubstring(t *testing.T) {
	// Group "alpha" exists AND a repo whose name substring-matches "alpha"
	// also exists in another group → group wins (rule 2 fires before rule 3).
	yaml := `repos:
  alpha:
    dir: /tmp/test-collision-alpha
    urls:
      - git@github.com:org/foo.git
      - git@github.com:org/bar.git
  vendor:
    dir: /tmp/test-collision-vendor
    urls:
      - git@github.com:vendor/alpha-shared.git
`
	writeReposFixture(t, yaml)

	rs, mode, err := resolveTargets("alpha", false)
	if err != nil {
		t.Fatalf("resolveTargets: %v", err)
	}
	if mode != modeBatch {
		t.Fatalf("expected modeBatch (group win), got %v", mode)
	}
	if len(rs) != 2 {
		t.Fatalf("expected 2 repos (alpha-group members), got %d", len(rs))
	}
	for _, r := range rs {
		if r.Group != "alpha" {
			t.Errorf("expected alpha-group member, got group %s", r.Group)
		}
	}
}

func TestResolveTargetsEmptyGroupResolvesAsEmptyBatch(t *testing.T) {
	// A group declared in hop.yaml with `urls:` null/empty must still be
	// recognized as a group (rule 2), not fall through to repo-name matching.
	yaml := `repos:
  empty:
    dir: /tmp/test-empty-group
    urls:
  populated:
    dir: /tmp/test-empty-group-populated
    urls:
      - git@github.com:org/foo.git
`
	writeReposFixture(t, yaml)

	rs, mode, err := resolveTargets("empty", false)
	if err != nil {
		t.Fatalf("resolveTargets empty: %v", err)
	}
	if mode != modeBatch {
		t.Fatalf("expected modeBatch for empty group, got %v", mode)
	}
	if len(rs) != 0 {
		t.Fatalf("expected 0 repos in empty batch, got %d (%v)", len(rs), rs)
	}
}

func TestResolveTargetsGroupLookupIsCaseSensitive(t *testing.T) {
	writeReposFixture(t, resolveTargetsYAML)

	// "Default" (uppercase D) is not an exact match for "default"; rule 2
	// fails, rule 3 falls through. With no substring repo match for
	// "Default", resolveByName triggers fzf — which we want to avoid in tests.
	// Instead, exercise the case-sensitivity by asserting hasConfiguredGroup's
	// behavior directly.
	rs, _, err := resolveTargets("vendor", false)
	if err != nil {
		t.Fatalf("sanity vendor: %v", err)
	}
	if len(rs) != 1 {
		t.Fatalf("sanity: expected 1 vendor repo, got %d", len(rs))
	}
	// Now verify that an uppercased query does NOT match the group via
	// hasConfiguredGroup directly (avoids fzf invocation).
	cfg := loadConfigForTest(t)
	if hasConfiguredGroup(cfg, "Vendor") {
		t.Fatalf("hasConfiguredGroup must be case-sensitive: 'Vendor' should not match group 'vendor'")
	}
	if !hasConfiguredGroup(cfg, "vendor") {
		t.Fatalf("hasConfiguredGroup: 'vendor' should match group 'vendor'")
	}
}

func TestBuildPickerLinesNoCollision(t *testing.T) {
	rs := repos.Repos{
		{Name: "alpha", Group: "default", Path: "/d/alpha", URL: "git@h:o/alpha.git"},
		{Name: "beta", Group: "default", Path: "/d/beta", URL: "git@h:o/beta.git"},
	}
	got := buildPickerLines(rs)
	for i, line := range got {
		first := strings.SplitN(line, "\t", 2)[0]
		if strings.Contains(first, "[") {
			t.Errorf("line %d unexpectedly has group suffix: %q", i, line)
		}
		if first != rs[i].Name {
			t.Errorf("line %d first column = %q, want %q", i, first, rs[i].Name)
		}
	}
}

// makeClonedRepoFixture writes a hop.yaml with a single repo named `name`
// rooted at a fresh temp dir, then creates the repo's main checkout AND a
// `.git` directory inside it so cloneState reports stateAlreadyCloned.
// Returns the resolved repo path.
func makeClonedRepoFixture(t *testing.T, name string) string {
	t.Helper()
	parent := t.TempDir()
	repoDir := filepath.Join(parent, name)
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}
	yaml := fmt.Sprintf(`repos:
  default:
    dir: %s
    urls:
      - git@github.com:sahil87/%s.git
`, parent, name)
	writeReposFixture(t, yaml)
	return repoDir
}

func TestResolveByNameSplitsOnFirstSlashAndReplacesPath(t *testing.T) {
	repoDir := makeClonedRepoFixture(t, "outbox")
	wantPath := repoDir + ".worktrees/feat-x"

	withListWorktrees(t, func(ctx context.Context, repoPath string) ([]WtEntry, error) {
		if repoPath != repoDir {
			t.Errorf("listWorktrees called with %q, want %q", repoPath, repoDir)
		}
		return []WtEntry{
			{Name: "main", Path: repoDir, IsMain: true},
			{Name: "feat-x", Path: wantPath, Dirty: true},
		}, nil
	})

	got, err := resolveByName("outbox/feat-x")
	if err != nil {
		t.Fatalf("resolveByName: %v", err)
	}
	if got.Path != wantPath {
		t.Errorf("got.Path = %q, want %q", got.Path, wantPath)
	}
	// Name/Group/URL preserved from the registry entry.
	if got.Name != "outbox" {
		t.Errorf("got.Name = %q, want outbox", got.Name)
	}
	if got.Group != "default" {
		t.Errorf("got.Group = %q, want default", got.Group)
	}
	if got.URL == "" {
		t.Errorf("got.URL is empty, want preserved from registry")
	}
}

func TestResolveByNameWorktreeMainReturnsMainPath(t *testing.T) {
	repoDir := makeClonedRepoFixture(t, "outbox")
	withListWorktrees(t, func(ctx context.Context, repoPath string) ([]WtEntry, error) {
		return []WtEntry{
			{Name: "main", Path: repoDir, IsMain: true},
			{Name: "feat-x", Path: repoDir + ".worktrees/feat-x"},
		}, nil
	})

	got, err := resolveByName("outbox/main")
	if err != nil {
		t.Fatalf("resolveByName outbox/main: %v", err)
	}
	if got.Path != repoDir {
		t.Errorf("got.Path = %q, want main checkout %q", got.Path, repoDir)
	}
}

func TestResolveByNameNoSlashSkipsWtInvocation(t *testing.T) {
	repoDir := makeClonedRepoFixture(t, "outbox")
	called := false
	withListWorktrees(t, func(ctx context.Context, repoPath string) ([]WtEntry, error) {
		called = true
		return nil, nil
	})

	got, err := resolveByName("outbox")
	if err != nil {
		t.Fatalf("resolveByName outbox: %v", err)
	}
	if called {
		t.Errorf("listWorktrees called for /-less query; want NOT called")
	}
	if got.Path != repoDir {
		t.Errorf("got.Path = %q, want %q", got.Path, repoDir)
	}
}

func TestResolveByNameSplitsOnFirstSlashNotLast(t *testing.T) {
	repoDir := makeClonedRepoFixture(t, "outbox")
	wantPath := repoDir + ".worktrees/feat-x-sub"

	withListWorktrees(t, func(ctx context.Context, repoPath string) ([]WtEntry, error) {
		return []WtEntry{
			{Name: "feat-x/sub", Path: wantPath},
		}, nil
	})

	got, err := resolveByName("outbox/feat-x/sub")
	if err != nil {
		t.Fatalf("resolveByName: %v", err)
	}
	if got.Path != wantPath {
		t.Errorf("got.Path = %q, want %q (RHS must be %q, not split further)", got.Path, wantPath, "feat-x/sub")
	}
}

func TestResolveByNameEmptyRHSIsUsageError(t *testing.T) {
	makeClonedRepoFixture(t, "outbox")

	_, err := resolveByName("outbox/")
	var withCode *errExitCode
	if !errors.As(err, &withCode) {
		t.Fatalf("expected *errExitCode, got %T: %v", err, err)
	}
	if withCode.code != 2 {
		t.Errorf("expected exit code 2, got %d", withCode.code)
	}
	if !strings.Contains(withCode.msg, "empty worktree name after '/'") {
		t.Errorf("unexpected msg: %q", withCode.msg)
	}
}

func TestResolveByNameEmptyLHSIsUsageError(t *testing.T) {
	makeClonedRepoFixture(t, "outbox")

	_, err := resolveByName("/feat-x")
	var withCode *errExitCode
	if !errors.As(err, &withCode) {
		t.Fatalf("expected *errExitCode, got %T: %v", err, err)
	}
	if withCode.code != 2 {
		t.Errorf("expected exit code 2, got %d", withCode.code)
	}
	if !strings.Contains(withCode.msg, "empty repo name before '/'") {
		t.Errorf("unexpected msg: %q", withCode.msg)
	}
}

func TestResolveByNameUnknownWorktreeSurfacesError(t *testing.T) {
	repoDir := makeClonedRepoFixture(t, "outbox")
	withListWorktrees(t, func(ctx context.Context, repoPath string) ([]WtEntry, error) {
		return []WtEntry{{Name: "main", Path: repoDir, IsMain: true}}, nil
	})

	_, err := resolveByName("outbox/nonexistent")
	var withCode *errExitCode
	if !errors.As(err, &withCode) {
		t.Fatalf("expected *errExitCode, got %T: %v", err, err)
	}
	if withCode.code != 1 {
		t.Errorf("expected exit code 1, got %d", withCode.code)
	}
	wantSubstrs := []string{
		"hop: worktree 'nonexistent' not found in 'outbox'",
		"Try: wt list (in " + repoDir + ")",
		"hop ls --trees",
	}
	for _, want := range wantSubstrs {
		if !strings.Contains(withCode.msg, want) {
			t.Errorf("missing substring %q in msg: %q", want, withCode.msg)
		}
	}
}

func TestResolveByNameCaseSensitiveWorktreeMatch(t *testing.T) {
	repoDir := makeClonedRepoFixture(t, "outbox")
	withListWorktrees(t, func(ctx context.Context, repoPath string) ([]WtEntry, error) {
		// Worktree literally named "Feat-X" (uppercase F).
		return []WtEntry{{Name: "Feat-X", Path: repoDir + ".worktrees/Feat-X"}}, nil
	})

	_, err := resolveByName("outbox/feat-x")
	var withCode *errExitCode
	if !errors.As(err, &withCode) {
		t.Fatalf("expected *errExitCode (case-sensitive miss), got %T: %v", err, err)
	}
	if !strings.Contains(withCode.msg, "worktree 'feat-x' not found") {
		t.Errorf("expected no-such-worktree error for case mismatch, got: %q", withCode.msg)
	}
}

func TestResolveByNameWtMissingOnPATH(t *testing.T) {
	makeClonedRepoFixture(t, "outbox")
	withListWorktrees(t, func(ctx context.Context, repoPath string) ([]WtEntry, error) {
		return nil, proc.ErrNotFound
	})

	_, err := resolveByName("outbox/feat-x")
	var withCode *errExitCode
	if !errors.As(err, &withCode) {
		t.Fatalf("expected *errExitCode, got %T: %v", err, err)
	}
	if withCode.code != 1 {
		t.Errorf("expected exit code 1, got %d", withCode.code)
	}
	if withCode.msg != wtMissingHint {
		t.Errorf("msg = %q, want verbatim %q", withCode.msg, wtMissingHint)
	}
}

func TestResolveByNameMalformedJSONSurfaces(t *testing.T) {
	makeClonedRepoFixture(t, "outbox")
	// listWorktrees returns the raw json.Unmarshal error verbatim (the
	// "wt list:" prefix is owned by this caller, not the seam) — see
	// unmarshalWtEntries' contract in wt_list.go.
	withListWorktrees(t, func(ctx context.Context, repoPath string) ([]WtEntry, error) {
		return nil, fmt.Errorf("invalid character 'n' looking for beginning of value")
	})

	_, err := resolveByName("outbox/feat-x")
	var withCode *errExitCode
	if !errors.As(err, &withCode) {
		t.Fatalf("expected *errExitCode, got %T: %v", err, err)
	}
	if withCode.code != 1 {
		t.Errorf("expected exit code 1, got %d", withCode.code)
	}
	if !strings.HasPrefix(withCode.msg, "hop: wt list:") {
		t.Errorf("expected prefix 'hop: wt list:', got %q", withCode.msg)
	}
	// Guard against prefix duplication regressing (the bug Copilot flagged):
	// the label "wt list:" must appear exactly once, not "hop: wt list: wt list: ...".
	if strings.Count(withCode.msg, "wt list:") != 1 {
		t.Errorf("expected single 'wt list:' label, got %q", withCode.msg)
	}
}

func TestResolveByNameUnclonedRepoShortCircuitsWithSlash(t *testing.T) {
	// Build a fixture pointing at a parent dir that does NOT contain the repo
	// subdir (so the resolved repo.Path doesn't exist → stateMissing).
	parent := t.TempDir()
	yaml := fmt.Sprintf(`repos:
  default:
    dir: %s
    urls:
      - git@github.com:sahil87/loom.git
`, parent)
	writeReposFixture(t, yaml)

	wtCalled := false
	withListWorktrees(t, func(ctx context.Context, repoPath string) ([]WtEntry, error) {
		wtCalled = true
		return nil, nil
	})

	_, err := resolveByName("loom/feat-x")
	var withCode *errExitCode
	if !errors.As(err, &withCode) {
		t.Fatalf("expected *errExitCode, got %T: %v", err, err)
	}
	if withCode.code != 1 {
		t.Errorf("expected exit code 1, got %d", withCode.code)
	}
	if !strings.Contains(withCode.msg, "'loom' is not cloned") {
		t.Errorf("expected uncloned hint, got %q", withCode.msg)
	}
	if wtCalled {
		t.Errorf("wt list invoked against uncloned repo; want NOT invoked")
	}
}

func TestResolveByNameUnclonedRepoBareQueryStillPermissive(t *testing.T) {
	// Same fixture as the uncloned-with-slash test, but bare query — must
	// retain pre-change permissive behavior (registry-derived path, no error).
	parent := t.TempDir()
	expected := filepath.Join(parent, "loom")
	yaml := fmt.Sprintf(`repos:
  default:
    dir: %s
    urls:
      - git@github.com:sahil87/loom.git
`, parent)
	writeReposFixture(t, yaml)

	got, err := resolveByName("loom")
	if err != nil {
		t.Fatalf("bare uncloned query: %v", err)
	}
	if got.Path != expected {
		t.Errorf("got.Path = %q, want %q (registry-derived)", got.Path, expected)
	}
}
