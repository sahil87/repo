// Package repos models the in-memory list of repositories derived from a
// *config.Config. Each Repo has Name, Group, Dir, URL, and Path fields. Name
// and the (per-URL) org component are derived from each URL by DeriveName and
// DeriveOrg respectively; org is not stored on Repo, but is used by FromConfig
// to compute Dir and Path for convention-driven (flat) groups.
package repos

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/sahil87/repo/internal/config"
)

// Repo describes a single repository entry. Path is the on-disk location
// (parent dir joined with name). Group identifies which configured group the
// repo came from.
type Repo struct {
	Name  string
	Group string
	Dir   string
	URL   string
	Path  string
}

// Repos is an ordered list of Repo.
type Repos []Repo

// FromConfig converts a *config.Config to a flat list of Repo entries.
//
// Order: groups in cfg.Groups order (which preserves YAML source order); URLs
// within each group in source order. For convention-driven (flat) groups, the
// per-repo Dir is `<expanded-code_root>/<org>` (or just `<expanded-code_root>`
// when org is empty); for `dir:`-overridden groups, Dir is the expanded `dir`
// value. Path is filepath.Join(Dir, Name).
func FromConfig(cfg *config.Config) (Repos, error) {
	if cfg == nil {
		return Repos{}, nil
	}
	codeRoot := ExpandDir(cfg.CodeRoot, "")
	if codeRoot == "" {
		codeRoot = ExpandDir("~", "")
	}

	var out Repos
	for _, g := range cfg.Groups {
		for _, url := range g.URLs {
			name := DeriveName(url)
			var dir string
			if g.Dir != "" {
				dir = ExpandDir(g.Dir, cfg.CodeRoot)
			} else {
				org := DeriveOrg(url)
				if org == "" {
					dir = codeRoot
				} else {
					dir = filepath.Join(codeRoot, org)
				}
			}
			out = append(out, Repo{
				Name:  name,
				Group: g.Name,
				Dir:   dir,
				URL:   url,
				Path:  filepath.Join(dir, name),
			})
		}
	}
	return out, nil
}

// MatchOne filters by case-insensitive substring match on Name.
// An empty query returns all repos (full list).
func (rs Repos) MatchOne(query string) Repos {
	if query == "" {
		return rs
	}
	q := strings.ToLower(query)
	var out Repos
	for _, r := range rs {
		if strings.Contains(strings.ToLower(r.Name), q) {
			out = append(out, r)
		}
	}
	return out
}

// List returns all repos (identity function — exists for API symmetry).
func (rs Repos) List() Repos {
	return rs
}

// ExpandDir expands a directory string to an absolute path, applying the
// following rules in order:
//
//  1. Empty string → "" (caller decides default).
//  2. "~" alone → $HOME.
//  3. "~/..." → $HOME/...
//  4. Starts with "/" → verbatim (absolute).
//  5. Starts with "~user/" or "~user" → verbatim (no Linux-style user lookup).
//  6. Otherwise (relative path with no leading "~" or "/"): if codeRootHint
//     is set and non-empty, treat dir as relative to the *expanded* codeRoot;
//     else treat as relative to $HOME.
//
// codeRootHint may itself be "~"-prefixed or absolute; ExpandDir recursively
// expands it (with empty codeRootHint to break the loop).
func ExpandDir(dir, codeRootHint string) string {
	if dir == "" {
		return ""
	}
	if dir == "~" {
		if home := os.Getenv("HOME"); home != "" {
			return home
		}
		return dir
	}
	if strings.HasPrefix(dir, "~/") {
		if home := os.Getenv("HOME"); home != "" {
			return filepath.Join(home, dir[2:])
		}
		return dir
	}
	if filepath.IsAbs(dir) {
		return dir
	}
	if strings.HasPrefix(dir, "~") {
		// "~user/..." or "~user" — verbatim per v0.0.1 behavior.
		return dir
	}
	// Relative path. If a code-root hint is provided (and is not the same as
	// dir itself, to avoid infinite recursion), resolve relative to it.
	if codeRootHint != "" && codeRootHint != dir {
		base := ExpandDir(codeRootHint, "")
		if base != "" {
			return filepath.Join(base, dir)
		}
	}
	if home := os.Getenv("HOME"); home != "" {
		return filepath.Join(home, dir)
	}
	return dir
}

// DeriveName extracts the repo name from a git URL: last /-separated token,
// trailing .git stripped. Both SSH (git@host:owner/name.git) and HTTPS
// (https://host/owner/name.git) URL forms work because both end in /name.git.
func DeriveName(url string) string {
	last := url
	if idx := strings.LastIndex(url, "/"); idx >= 0 {
		last = url[idx+1:]
	}
	last = strings.TrimSuffix(last, ".git")
	return last
}

// DeriveOrg extracts the organization (owner / namespace) component from a
// git URL. Returns "" when no owner component can be identified (e.g., bare
// names, malformed URLs).
//
// Algorithm:
//
//  1. Strip a trailing .git if present.
//  2. SSH form (git@host:path): take the substring after the first ':'.
//  3. HTTPS form (https://host/path): take the substring after the first '/'
//     following "://".
//  4. The org is everything before the last '/' in that path. May contain
//     additional '/' separators (e.g., nested GitLab groups).
func DeriveOrg(url string) string {
	stripped := strings.TrimSuffix(url, ".git")
	var path string
	switch {
	case strings.Contains(stripped, "://"):
		// HTTPS form. Drop scheme + host.
		afterScheme := stripped[strings.Index(stripped, "://")+3:]
		slash := strings.Index(afterScheme, "/")
		if slash < 0 {
			return ""
		}
		path = afterScheme[slash+1:]
	case strings.Contains(stripped, "@") && strings.Contains(stripped, ":"):
		// SSH form. Drop user@host:.
		colon := strings.Index(stripped, ":")
		path = stripped[colon+1:]
	default:
		// Bare path or unknown form.
		path = stripped
	}
	last := strings.LastIndex(path, "/")
	if last < 0 {
		return ""
	}
	return path[:last]
}
