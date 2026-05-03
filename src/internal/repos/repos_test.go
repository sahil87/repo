package repos

import (
	"path/filepath"
	"testing"

	"github.com/sahil87/repo/internal/config"
)

func TestFromConfig(t *testing.T) {
	t.Setenv("HOME", "/home/test")
	cfg := &config.Config{Entries: map[string][]string{
		"~/code/sahil87": {
			"git@github.com:sahil87/repo.git",
			"git@github.com:sahil87/wt.git",
		},
		"/etc/foo": {
			"https://github.com/example/foo.git",
		},
	}}
	rs, err := FromConfig(cfg)
	if err != nil {
		t.Fatalf("FromConfig: %v", err)
	}
	if len(rs) != 3 {
		t.Fatalf("expected 3 repos, got %d", len(rs))
	}

	// Check that path joins are correct and ~ was expanded
	for _, r := range rs {
		switch r.Name {
		case "repo":
			if r.Dir != "/home/test/code/sahil87" {
				t.Fatalf("repo: expected ~/code/sahil87 expanded, got %q", r.Dir)
			}
			if r.Path != "/home/test/code/sahil87/repo" {
				t.Fatalf("repo: expected expanded path, got %q", r.Path)
			}
		case "wt":
			if r.Path != "/home/test/code/sahil87/wt" {
				t.Fatalf("wt: expected expanded path, got %q", r.Path)
			}
		case "foo":
			if r.Dir != "/etc/foo" {
				t.Fatalf("foo: expected literal /etc/foo, got %q", r.Dir)
			}
			if r.Path != filepath.Join("/etc/foo", "foo") {
				t.Fatalf("foo: expected /etc/foo/foo, got %q", r.Path)
			}
		default:
			t.Fatalf("unexpected repo name %q", r.Name)
		}
	}
}

func TestFromConfigDeterministicOrder(t *testing.T) {
	cfg := &config.Config{Entries: map[string][]string{
		"/zzz": {"git@example.com:z/c.git", "git@example.com:z/a.git", "git@example.com:z/b.git"},
		"/aaa": {"git@example.com:a/y.git", "git@example.com:a/x.git"},
		"/mmm": {"git@example.com:m/m.git"},
	}}
	first, _ := FromConfig(cfg)
	for i := 0; i < 25; i++ {
		got, _ := FromConfig(cfg)
		if len(got) != len(first) {
			t.Fatalf("iteration %d: length mismatch", i)
		}
		for j := range got {
			if got[j] != first[j] {
				t.Fatalf("iteration %d index %d: %+v != %+v", i, j, got[j], first[j])
			}
		}
	}
	if first[0].Dir != "/aaa" || first[len(first)-1].Dir != "/zzz" {
		t.Fatalf("expected dir-sorted output, got first=%q last=%q", first[0].Dir, first[len(first)-1].Dir)
	}
	// Within /aaa, URLs should sort alphabetically: a/x.git before a/y.git → names x before y
	if first[0].Name != "x" || first[1].Name != "y" {
		t.Fatalf("expected url-sorted within dir (x then y), got %q then %q", first[0].Name, first[1].Name)
	}
}

func TestDeriveName(t *testing.T) {
	tests := []struct {
		url, want string
	}{
		{"git@github.com:sahil87/repo.git", "repo"},
		{"https://github.com/wvrdz/loom.git", "loom"},
		{"git@gitlab.com:org/group/sub/proj.git", "proj"},
		{"https://example.com/no-git-suffix", "no-git-suffix"},
	}
	for _, tt := range tests {
		if got := deriveName(tt.url); got != tt.want {
			t.Errorf("deriveName(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
}

func TestExpandTilde(t *testing.T) {
	t.Setenv("HOME", "/home/test")
	cases := map[string]string{
		"~/code":      "/home/test/code",
		"~":           "/home/test",
		"/abs/path":   "/abs/path",
		"/etc/~weird": "/etc/~weird",
		"relative":    "relative",
	}
	for in, want := range cases {
		if got := expandTilde(in); got != want {
			t.Errorf("expandTilde(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMatchOne(t *testing.T) {
	rs := Repos{
		{Name: "repo", Path: "/a/repo"},
		{Name: "Repos-shared", Path: "/a/Repos-shared"},
		{Name: "loom", Path: "/b/loom"},
	}

	caseInsensitive := rs.MatchOne("REPO")
	if len(caseInsensitive) != 2 {
		t.Fatalf("expected 2 matches for case-insensitive 'REPO', got %d", len(caseInsensitive))
	}

	all := rs.MatchOne("")
	if len(all) != 3 {
		t.Fatalf("expected all 3 repos for empty query, got %d", len(all))
	}

	none := rs.MatchOne("zzz")
	if len(none) != 0 {
		t.Fatalf("expected 0 matches for 'zzz', got %d", len(none))
	}
}
