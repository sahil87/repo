package main

import (
	"strings"
	"testing"

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
