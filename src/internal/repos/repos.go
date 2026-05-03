// Package repos models the in-memory list of repositories derived from a *config.Config.
// Each Repo has Name, Dir, URL, and Path fields with Name derived from the URL basename.
package repos

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sahil87/repo/internal/config"
)

// Repo describes a single repository entry. Path = Dir + "/" + Name (with ~ expanded in Dir).
type Repo struct {
	Name string
	Dir  string
	URL  string
	Path string
}

// Repos is an ordered list of Repo.
type Repos []Repo

// FromConfig converts a *config.Config to a flat list of Repo entries.
// Directory keys with a leading ~ are expanded to $HOME at load time.
// Repo names are derived as the last /-separated component of the URL with
// any trailing .git stripped.
//
// Output is sorted by directory key, then by URL within each directory, so
// `repo ls` and the bare-form picker present a stable order across invocations
// (yaml.v3 unmarshal into map[string][]string discards source order, so a
// deterministic sort is the next-best contract).
func FromConfig(cfg *config.Config) (Repos, error) {
	if cfg == nil || cfg.Entries == nil {
		return Repos{}, nil
	}
	dirs := make([]string, 0, len(cfg.Entries))
	for dir := range cfg.Entries {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)

	var out Repos
	for _, dir := range dirs {
		expanded := expandTilde(dir)
		urls := append([]string(nil), cfg.Entries[dir]...)
		sort.Strings(urls)
		for _, url := range urls {
			name := deriveName(url)
			out = append(out, Repo{
				Name: name,
				Dir:  expanded,
				URL:  url,
				Path: filepath.Join(expanded, name),
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

// expandTilde returns dir with a leading ~ expanded to $HOME.
// Only matches when ~ is the literal first character followed by / or end-of-string.
func expandTilde(dir string) string {
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
	}
	return dir
}

// deriveName extracts the repo name from a git URL: last /-separated token,
// trailing .git stripped. Both SSH (git@host:owner/name.git) and HTTPS
// (https://host/owner/name.git) URL forms work because both end in /name.git.
func deriveName(url string) string {
	last := url
	if idx := strings.LastIndex(url, "/"); idx >= 0 {
		last = url[idx+1:]
	}
	last = strings.TrimSuffix(last, ".git")
	return last
}
