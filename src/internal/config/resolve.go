package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ErrNoConfig is returned when none of the search-order candidates resolve.
var ErrNoConfig = errors.New("hop: no hop.yaml found")

// Resolve walks the search order from docs/specs/config-resolution.md (adapted
// for the rename to `hop`):
//  1. $HOP_CONFIG if set (hard-error if file is missing)
//  2. $XDG_CONFIG_HOME/hop/hop.yaml if $XDG_CONFIG_HOME is set
//  3. $HOME/.config/hop/hop.yaml
//
// Returns the resolved path or an error if none can be determined.
func Resolve() (string, error) {
	if p, ok := os.LookupEnv("HOP_CONFIG"); ok && p != "" {
		if _, err := os.Stat(p); err != nil {
			if os.IsNotExist(err) {
				return "", fmt.Errorf("hop: $HOP_CONFIG points to %s, which does not exist. Set $HOP_CONFIG to an existing file or unset it.", p)
			}
			return "", fmt.Errorf("hop: stat $HOP_CONFIG (%s): %w", p, err)
		}
		return p, nil
	}

	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		p := filepath.Join(xdg, "hop", "hop.yaml")
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}

	if home := os.Getenv("HOME"); home != "" {
		p := filepath.Join(home, ".config", "hop", "hop.yaml")
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}

	return "", fmt.Errorf("hop: no hop.yaml found. Set $HOP_CONFIG to a tracked file (e.g., a Dropbox path or a git-tracked dotfile), or run 'hop config init' to bootstrap one at $XDG_CONFIG_HOME/hop/hop.yaml.")
}

// ResolveWriteTarget returns the path that would be used as the config target
// for hop config init / hop config where. Unlike Resolve, this does NOT trigger
// the "$HOP_CONFIG set but missing" hard error — it returns the path that *would*
// be used regardless of whether the file currently exists.
//
// Returns an error only when no path can be determined (no env vars and no $HOME).
func ResolveWriteTarget() (string, error) {
	if p, ok := os.LookupEnv("HOP_CONFIG"); ok && p != "" {
		return p, nil
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "hop", "hop.yaml"), nil
	}
	if home := os.Getenv("HOME"); home != "" {
		return filepath.Join(home, ".config", "hop", "hop.yaml"), nil
	}
	return "", fmt.Errorf("hop: no config path resolvable. Set $HOP_CONFIG or ensure $XDG_CONFIG_HOME or $HOME is set.")
}
