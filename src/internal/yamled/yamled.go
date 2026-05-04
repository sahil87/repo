// Package yamled provides comment-preserving YAML node-level edits.
//
// Used by `hop clone <url>` to append a URL to a group's URL list in hop.yaml
// while preserving the user's comments. Indentation is normalized to yaml.v3's
// defaults on round-trip — comment preservation is the contract; byte-perfect
// formatting is not.
//
// The package operates at the yaml.Node level (not via Marshal/Unmarshal of
// concrete types) because the standard round-trip through Go structs loses
// comments and source-order information. Schema validation is the caller's
// responsibility — yamled only knows how to navigate and mutate.
package yamled

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// AppendURL loads the YAML file at path as a yaml.Node tree, locates
// repos.<group>, appends url to its URL list (handling both flat-list and
// map-with-urls shapes), and writes the result back atomically.
//
// Errors:
//   - "yamled: read <path>: ..." if the file cannot be read
//   - "yamled: parse <path>: ..." if the YAML is malformed
//   - "yamled: group '<group>' not found in <path>" if the group key is absent
//   - "yamled: group '<group>' is map-shaped but has no 'urls' field; cannot append"
//   - "yamled: group '<group>' has unexpected shape; cannot append"
//   - I/O errors from the temp-file write/rename
//
// Comments in unmodified portions of the file are preserved. Indentation is
// normalized to yaml.v3's defaults on round-trip — comment preservation is
// the contract, byte-perfect formatting is not.
func AppendURL(path, group, url string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("yamled: read %s: %w", path, err)
	}

	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("yamled: parse %s: %w", path, err)
	}

	if err := appendURLToTree(&root, group, url, path); err != nil {
		return err
	}

	out, err := yaml.Marshal(&root)
	if err != nil {
		return fmt.Errorf("yamled: marshal: %w", err)
	}

	return atomicWrite(path, out)
}

// appendURLToTree mutates the parsed tree in place: locate repos.<group>, append url.
func appendURLToTree(root *yaml.Node, group, url, path string) error {
	if len(root.Content) == 0 {
		return fmt.Errorf("yamled: group '%s' not found in %s: %w", group, path, ErrGroupNotFound)
	}
	top := root.Content[0]
	if top.Kind != yaml.MappingNode {
		return fmt.Errorf("yamled: parse %s: top-level is not a mapping", path)
	}

	reposNode := mappingValue(top, "repos")
	if reposNode == nil || reposNode.Kind != yaml.MappingNode {
		return fmt.Errorf("yamled: group '%s' not found in %s: %w", group, path, ErrGroupNotFound)
	}

	groupNode := mappingValue(reposNode, group)
	if groupNode == nil {
		return fmt.Errorf("yamled: group '%s' not found in %s: %w", group, path, ErrGroupNotFound)
	}

	scalar := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: url}

	switch groupNode.Kind {
	case yaml.SequenceNode:
		groupNode.Content = append(groupNode.Content, scalar)
		return nil
	case yaml.MappingNode:
		urlsNode := mappingValue(groupNode, "urls")
		if urlsNode == nil {
			return fmt.Errorf("yamled: group '%s' is map-shaped but has no 'urls' field; cannot append", group)
		}
		if urlsNode.Kind != yaml.SequenceNode {
			return fmt.Errorf("yamled: group '%s' has 'urls' field but it is not a list; cannot append", group)
		}
		urlsNode.Content = append(urlsNode.Content, scalar)
		return nil
	default:
		return fmt.Errorf("yamled: group '%s' has unexpected shape; cannot append", group)
	}
}

// mappingValue returns the value node for key in a mapping node, or nil if the
// key is absent or the node is not a mapping. Mapping nodes store keys and
// values interleaved in Content (key, value, key, value, ...).
func mappingValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		k := node.Content[i]
		if k.Kind == yaml.ScalarNode && k.Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

// atomicWrite writes data to path via a temp file in the same directory and a
// rename. The original file's mode is preserved on the replacement (defaulting
// to 0644 if the original cannot be stat'd), so an append never silently
// downgrades the user's permissions. On rename failure, the temp file is
// removed (best-effort) and the original is left unchanged.
func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)

	// Capture the original file's mode so we can preserve it. os.CreateTemp
	// defaults to 0600, which would silently restrict hop.yaml from 0644 to
	// 0600 on first append.
	mode := os.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}

	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp.*")
	if err != nil {
		return fmt.Errorf("yamled: create temp in %s: %w", dir, err)
	}
	tmpPath := tmp.Name()

	cleanup := func() {
		_ = os.Remove(tmpPath)
	}

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("yamled: write %s: %w", tmpPath, err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("yamled: sync %s: %w", tmpPath, err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("yamled: close %s: %w", tmpPath, err)
	}

	if err := os.Chmod(tmpPath, mode); err != nil {
		cleanup()
		return fmt.Errorf("yamled: chmod %s: %w", tmpPath, err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		cleanup()
		return fmt.Errorf("yamled: rename %s → %s: %w", tmpPath, path, err)
	}
	return nil
}

// ErrGroupNotFound is wrapped by AppendURL when the named group is absent.
// Callers can detect this case with errors.Is(err, ErrGroupNotFound).
var ErrGroupNotFound = errors.New("yamled: group not found")
