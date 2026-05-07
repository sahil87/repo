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
