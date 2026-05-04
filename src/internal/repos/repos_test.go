package repos

import (
	"testing"

	"github.com/sahil87/hop/internal/config"
)

func TestFromConfigFlatGroup(t *testing.T) {
	t.Setenv("HOME", "/home/test")
	cfg := &config.Config{
		CodeRoot: "~/code",
		Groups: []config.Group{
			{
				Name: "default",
				URLs: []string{
					"git@github.com:sahil87/hop.git",
					"git@github.com:sahil87/wt.git",
				},
			},
		},
	}
	rs, err := FromConfig(cfg)
	if err != nil {
		t.Fatalf("FromConfig: %v", err)
	}
	if len(rs) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(rs))
	}

	if rs[0].Name != "hop" {
		t.Errorf("rs[0].Name = %q, want hop", rs[0].Name)
	}
	if rs[0].Group != "default" {
		t.Errorf("rs[0].Group = %q, want default", rs[0].Group)
	}
	if rs[0].Path != "/home/test/code/sahil87/hop" {
		t.Errorf("rs[0].Path = %q", rs[0].Path)
	}
	if rs[1].Path != "/home/test/code/sahil87/wt" {
		t.Errorf("rs[1].Path = %q", rs[1].Path)
	}
}

func TestFromConfigMapGroupAbsoluteDir(t *testing.T) {
	t.Setenv("HOME", "/home/test")
	cfg := &config.Config{
		CodeRoot: "~/code",
		Groups: []config.Group{
			{
				Name: "vendor",
				Dir:  "~/vendor",
				URLs: []string{"git@github.com:vendor/their-tool.git"},
			},
		},
	}
	rs, _ := FromConfig(cfg)
	if len(rs) != 1 {
		t.Fatalf("expected 1 repo, got %d", len(rs))
	}
	if rs[0].Path != "/home/test/vendor/their-tool" {
		t.Errorf("Path = %q", rs[0].Path)
	}
	if rs[0].Group != "vendor" {
		t.Errorf("Group = %q", rs[0].Group)
	}
}

func TestFromConfigMapGroupRelativeDir(t *testing.T) {
	t.Setenv("HOME", "/home/test")
	cfg := &config.Config{
		CodeRoot: "~/code",
		Groups: []config.Group{
			{
				Name: "experiments",
				Dir:  "experiments",
				URLs: []string{"git@github.com:sahil87/sandbox.git"},
			},
		},
	}
	rs, _ := FromConfig(cfg)
	if len(rs) != 1 {
		t.Fatalf("expected 1 repo, got %d", len(rs))
	}
	if rs[0].Path != "/home/test/code/experiments/sandbox" {
		t.Errorf("Path = %q (relative dir should resolve relative to code_root)", rs[0].Path)
	}
}

func TestFromConfigGroupOrderPreserved(t *testing.T) {
	cfg := &config.Config{
		CodeRoot: "/r",
		Groups: []config.Group{
			{Name: "experiments", Dir: "/x", URLs: []string{"git@h:/c.git"}},
			{Name: "default", URLs: []string{"git@h:o/a.git"}},
			{Name: "vendor", Dir: "/v", URLs: []string{"git@h:/b.git"}},
		},
	}
	rs, _ := FromConfig(cfg)
	if len(rs) != 3 {
		t.Fatalf("expected 3, got %d", len(rs))
	}
	wantOrder := []string{"experiments", "default", "vendor"}
	for i, w := range wantOrder {
		if rs[i].Group != w {
			t.Errorf("rs[%d].Group = %q, want %q", i, rs[i].Group, w)
		}
	}
}

func TestFromConfigDefaultCodeRoot(t *testing.T) {
	t.Setenv("HOME", "/home/test")
	cfg := &config.Config{
		CodeRoot: "~",
		Groups: []config.Group{
			{Name: "default", URLs: []string{"git@github.com:foo/bar.git"}},
		},
	}
	rs, _ := FromConfig(cfg)
	if len(rs) != 1 {
		t.Fatal("expected 1 repo")
	}
	if rs[0].Path != "/home/test/foo/bar" {
		t.Errorf("Path = %q", rs[0].Path)
	}
}

func TestDeriveName(t *testing.T) {
	tests := []struct {
		url, want string
	}{
		{"git@github.com:sahil87/hop.git", "hop"},
		{"https://github.com/wvrdz/loom.git", "loom"},
		{"git@gitlab.com:org/group/sub/proj.git", "proj"},
		{"https://example.com/no-git-suffix", "no-git-suffix"},
	}
	for _, tt := range tests {
		if got := DeriveName(tt.url); got != tt.want {
			t.Errorf("DeriveName(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
}

func TestDeriveOrg(t *testing.T) {
	tests := []struct {
		url, want string
	}{
		{"git@github.com:sahil87/hop.git", "sahil87"},
		{"https://github.com/sahil87/hop.git", "sahil87"},
		{"git@gitlab.com:org/group/sub/proj.git", "org/group/sub"},
		{"https://github.com/sahil87/hop", "sahil87"},
		{"file:///tmp/local-repo.git", "tmp"},
		{"plain-name", ""},
	}
	for _, tt := range tests {
		if got := DeriveOrg(tt.url); got != tt.want {
			t.Errorf("DeriveOrg(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
}

func TestExpandDir(t *testing.T) {
	t.Setenv("HOME", "/home/test")
	cases := []struct {
		dir, hint, want string
	}{
		{"~", "", "/home/test"},
		{"~/code", "", "/home/test/code"},
		{"/abs/path", "", "/abs/path"},
		{"experiments", "~/code", "/home/test/code/experiments"},
		{"experiments", "/srv/code", "/srv/code/experiments"},
		{"~user/foo", "", "~user/foo"},
		{"", "", ""},
	}
	for _, c := range cases {
		if got := ExpandDir(c.dir, c.hint); got != c.want {
			t.Errorf("ExpandDir(%q, %q) = %q, want %q", c.dir, c.hint, got, c.want)
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
		t.Fatalf("expected 2 matches for 'REPO', got %d", len(caseInsensitive))
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
