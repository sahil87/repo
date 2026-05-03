package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ErrNoConfig is returned when none of the search-order candidates resolve.
var ErrNoConfig = errors.New("repo: no repos.yaml found")

// Resolve walks the search order from docs/specs/config-resolution.md:
//  1. $REPOS_YAML if set (hard-error if file is missing)
//  2. $XDG_CONFIG_HOME/repo/repos.yaml if $XDG_CONFIG_HOME is set
//  3. $HOME/.config/repo/repos.yaml
//
// Returns the resolved path or an error if none can be determined.
func Resolve() (string, error) {
	if p, ok := os.LookupEnv("REPOS_YAML"); ok && p != "" {
		if _, err := os.Stat(p); err != nil {
			if os.IsNotExist(err) {
				return "", fmt.Errorf("repo: $REPOS_YAML points to %s, which does not exist. Set $REPOS_YAML to an existing file or unset it.", p)
			}
			return "", fmt.Errorf("repo: stat $REPOS_YAML (%s): %w", p, err)
		}
		return p, nil
	}

	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		p := filepath.Join(xdg, "repo", "repos.yaml")
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}

	if home := os.Getenv("HOME"); home != "" {
		p := filepath.Join(home, ".config", "repo", "repos.yaml")
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}

	return "", fmt.Errorf("repo: no repos.yaml found. Set $REPOS_YAML to a tracked file (e.g., a Dropbox path or a git-tracked dotfile), or run 'repo config init' to bootstrap one at $XDG_CONFIG_HOME/repo/repos.yaml.")
}

// ResolveWriteTarget returns the path that would be used as the config target
// for repo config init / repo config path. Unlike Resolve, this does NOT trigger
// the "$REPOS_YAML set but missing" hard error — it returns the path that *would*
// be used regardless of whether the file currently exists.
//
// Returns an error only when no path can be determined (no env vars and no $HOME).
func ResolveWriteTarget() (string, error) {
	if p, ok := os.LookupEnv("REPOS_YAML"); ok && p != "" {
		return p, nil
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "repo", "repos.yaml"), nil
	}
	if home := os.Getenv("HOME"); home != "" {
		return filepath.Join(home, ".config", "repo", "repos.yaml"), nil
	}
	return "", fmt.Errorf("repo: no config path resolvable. Set $REPOS_YAML or ensure $XDG_CONFIG_HOME or $HOME is set.")
}
