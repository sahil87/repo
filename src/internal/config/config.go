// Package config handles loading repos.yaml, resolving its location, and
// bootstrapping starter content. The schema is a top-level map of
// directory → list of git URLs (see docs/specs/config-resolution.md).
package config

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config is the parsed contents of repos.yaml.
//
// Entries maps a directory string (with ~ unexpanded) to a list of git URLs.
// Directory expansion and name derivation happen in internal/repos.
type Config struct {
	Entries map[string][]string
}

//go:embed starter.yaml
var starterContent []byte

// Load reads, parses, and validates the YAML file at path.
// Errors are wrapped with the file path; YAML parse errors include line context
// from yaml.v3 directly.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("repo: read %s: %w", path, err)
	}

	cfg := &Config{Entries: map[string][]string{}}
	if len(data) == 0 {
		return cfg, nil
	}

	if err := yaml.Unmarshal(data, &cfg.Entries); err != nil {
		return nil, fmt.Errorf("repo: parse %s: %w", path, err)
	}
	if cfg.Entries == nil {
		cfg.Entries = map[string][]string{}
	}
	return cfg, nil
}

// WriteStarter writes the embedded starter.yaml content to path.
// Refuses to overwrite an existing file. Creates parent directories with mode 0755.
// File mode is 0644 (per docs/specs/config-resolution.md Design Decision 3).
func WriteStarter(path string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("repo config init: %s already exists. Delete it first or set $REPOS_YAML to a different path.", path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("repo config init: stat %s: %w", path, err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("repo config init: mkdir %s: %w", filepath.Dir(path), err)
	}

	if err := os.WriteFile(path, starterContent, 0o644); err != nil {
		return fmt.Errorf("repo config init: write %s: %w", path, err)
	}
	return nil
}

// StarterContent returns the embedded starter.yaml bytes.
// Exposed for tests that compare exact byte content.
func StarterContent() []byte {
	return starterContent
}

