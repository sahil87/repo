// Package config handles loading hop.yaml, resolving its location, and
// bootstrapping starter content. The schema is two top-level keys: `config`
// (currently with one optional field, `code_root`) and `repos` (a map of
// group_name → group_body). See `fab/changes/.../spec.md` "Config: Schema"
// requirements for the authoritative contract.
package config

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"gopkg.in/yaml.v3"
)

// Config is the parsed contents of hop.yaml.
//
// CodeRoot is `config.code_root` (defaults to "~"). Groups is the ordered list
// of group definitions, preserving source order from the YAML file.
type Config struct {
	CodeRoot string
	Groups   []Group
}

// Group is one named group of repos.
//
// Dir is empty for convention-driven (flat-list) groups; for map-shaped groups
// it carries the literal value from `dir:` (with no expansion applied — the
// caller, repos.FromConfig, handles tilde and relative-to-code_root expansion).
type Group struct {
	Name string
	Dir  string
	URLs []string
}

//go:embed starter.yaml
var starterContent []byte

// groupNameRe matches valid group names: lowercase letter, then any number of
// lowercase alphanumerics, underscores, or hyphens.
var groupNameRe = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)

// Load reads, parses, and validates the YAML file at path. Returns a populated
// *Config or a wrapped error.
//
// Empty file (zero bytes) returns &Config{CodeRoot: "~", Groups: nil}, no error.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("hop: read %s: %w", path, err)
	}

	if len(data) == 0 {
		return &Config{CodeRoot: "~"}, nil
	}

	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("hop: parse %s: %w", path, err)
	}

	if len(root.Content) == 0 {
		// Document with comments only / no content — treat as empty.
		return &Config{CodeRoot: "~"}, nil
	}

	top := root.Content[0]
	if top.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("hop: parse %s: top-level must be a mapping", path)
	}

	cfg := &Config{CodeRoot: "~"}
	var reposNode *yaml.Node
	seenRepos := false

	for i := 0; i+1 < len(top.Content); i += 2 {
		k := top.Content[i]
		v := top.Content[i+1]
		if k.Kind != yaml.ScalarNode {
			return nil, fmt.Errorf("hop: parse %s: top-level keys must be scalars", path)
		}
		switch k.Value {
		case "config":
			if err := parseConfigBlock(v, cfg, path); err != nil {
				return nil, err
			}
		case "repos":
			reposNode = v
			seenRepos = true
		default:
			return nil, fmt.Errorf("hop: parse %s: unknown top-level field '%s'. Valid: 'config', 'repos'.", path, k.Value)
		}
	}

	if !seenRepos {
		return nil, fmt.Errorf("hop: parse %s: missing required field 'repos'", path)
	}

	if err := parseReposBlock(reposNode, cfg, path); err != nil {
		return nil, err
	}

	if err := validateUniqueURLs(cfg, path); err != nil {
		return nil, err
	}

	return cfg, nil
}

// parseConfigBlock walks the `config:` mapping, validating field names and
// populating cfg.CodeRoot.
func parseConfigBlock(node *yaml.Node, cfg *Config, path string) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("hop: parse %s: 'config' must be a mapping", path)
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		k := node.Content[i]
		v := node.Content[i+1]
		if k.Kind != yaml.ScalarNode {
			return fmt.Errorf("hop: parse %s: config keys must be scalars", path)
		}
		switch k.Value {
		case "code_root":
			if v.Kind != yaml.ScalarNode {
				return fmt.Errorf("hop: parse %s: config.code_root must be a string", path)
			}
			cfg.CodeRoot = v.Value
		default:
			return fmt.Errorf("hop: parse %s: unknown config field '%s'", path, k.Value)
		}
	}
	return nil
}

// parseReposBlock walks the `repos:` mapping (in source order), validating
// each group name and body, and appending Group entries to cfg.Groups.
func parseReposBlock(node *yaml.Node, cfg *Config, path string) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("hop: parse %s: 'repos' must be a mapping of group_name → group_body", path)
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		k := node.Content[i]
		v := node.Content[i+1]
		if k.Kind != yaml.ScalarNode {
			return fmt.Errorf("hop: parse %s: repos keys must be scalars", path)
		}
		name := k.Value
		if !groupNameRe.MatchString(name) {
			return fmt.Errorf("hop: parse %s: invalid group name '%s'. Group names must match ^[a-z][a-z0-9_-]*$", path, name)
		}
		g, err := parseGroupBody(name, v, path)
		if err != nil {
			return err
		}
		cfg.Groups = append(cfg.Groups, g)
	}
	return nil
}

// parseGroupBody classifies the body shape (sequence | mapping) and extracts
// dir/urls. Errors on unknown keys, unexpected shapes, empty dir, or duplicate
// URLs within the group.
func parseGroupBody(name string, node *yaml.Node, path string) (Group, error) {
	g := Group{Name: name}

	switch node.Kind {
	case yaml.SequenceNode:
		urls, err := scalarSlice(node, name, path)
		if err != nil {
			return g, err
		}
		if dup := firstDuplicate(urls); dup != "" {
			return g, fmt.Errorf("hop: parse %s: URL '%s' is listed twice in group '%s'.", path, dup, name)
		}
		g.URLs = urls
		return g, nil

	case yaml.MappingNode:
		for i := 0; i+1 < len(node.Content); i += 2 {
			k := node.Content[i]
			v := node.Content[i+1]
			if k.Kind != yaml.ScalarNode {
				return g, fmt.Errorf("hop: parse %s: group '%s' keys must be scalars", path, name)
			}
			switch k.Value {
			case "dir":
				if v.Kind != yaml.ScalarNode {
					return g, fmt.Errorf("hop: parse %s: group '%s' 'dir' must be a string", path, name)
				}
				if v.Value == "" {
					return g, fmt.Errorf("hop: parse %s: group '%s' has empty 'dir'", path, name)
				}
				g.Dir = v.Value
			case "urls":
				if v.Kind == yaml.ScalarNode && v.Tag == "!!null" {
					// `urls:` with no value (null) — treat as empty list.
					g.URLs = nil
					continue
				}
				if v.Kind != yaml.SequenceNode {
					return g, fmt.Errorf("hop: parse %s: group '%s' 'urls' must be a list", path, name)
				}
				urls, err := scalarSlice(v, name, path)
				if err != nil {
					return g, err
				}
				if dup := firstDuplicate(urls); dup != "" {
					return g, fmt.Errorf("hop: parse %s: URL '%s' is listed twice in group '%s'.", path, dup, name)
				}
				g.URLs = urls
			default:
				return g, fmt.Errorf("hop: parse %s: group '%s' has unknown field '%s'. Valid: 'dir', 'urls'.", path, name, k.Value)
			}
		}
		return g, nil

	default:
		return g, fmt.Errorf("hop: parse %s: group '%s' must be a list of URLs or a map with 'dir' and 'urls'.", path, name)
	}
}

// scalarSlice extracts a list of URL strings from a sequence node.
func scalarSlice(node *yaml.Node, groupName, path string) ([]string, error) {
	out := make([]string, 0, len(node.Content))
	for _, c := range node.Content {
		if c.Kind != yaml.ScalarNode {
			return nil, fmt.Errorf("hop: parse %s: group '%s' contains a non-string entry", path, groupName)
		}
		out = append(out, c.Value)
	}
	return out, nil
}

// firstDuplicate returns the first repeated string in xs, or "" if all are
// unique.
func firstDuplicate(xs []string) string {
	seen := make(map[string]struct{}, len(xs))
	for _, x := range xs {
		if _, ok := seen[x]; ok {
			return x
		}
		seen[x] = struct{}{}
	}
	return ""
}

// validateUniqueURLs checks that no URL appears in more than one group.
func validateUniqueURLs(cfg *Config, path string) error {
	owner := make(map[string]string)
	for _, g := range cfg.Groups {
		for _, u := range g.URLs {
			if prev, ok := owner[u]; ok && prev != g.Name {
				return fmt.Errorf("hop: parse %s: URL '%s' appears in groups '%s' and '%s'; a URL must belong to exactly one group.", path, u, prev, g.Name)
			}
			owner[u] = g.Name
		}
	}
	return nil
}

// WriteStarter writes the embedded starter.yaml content to path.
// Refuses to overwrite an existing file. Creates parent directories with mode 0755.
// File mode is 0644 (per docs/specs/config-resolution.md Design Decision 3).
func WriteStarter(path string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("hop config init: %s already exists. Delete it first or set $HOP_CONFIG to a different path.", path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("hop config init: stat %s: %w", path, err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("hop config init: mkdir %s: %w", filepath.Dir(path), err)
	}

	if err := os.WriteFile(path, starterContent, 0o644); err != nil {
		return fmt.Errorf("hop config init: write %s: %w", path, err)
	}
	return nil
}

// StarterContent returns the embedded starter.yaml bytes.
// Exposed for tests that compare exact byte content.
func StarterContent() []byte {
	return starterContent
}
