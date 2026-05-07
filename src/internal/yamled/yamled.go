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

// ScanPlan describes a structured set of additions for MergeScan / RenderScan.
// The CLI layer assembles this from a scan.Walk result after running the
// convention check, slugification, conflict resolution, and HOME substitution.
type ScanPlan struct {
	// DefaultURLs is appended to the "default" flat group; the group is
	// created (as a flat list) if absent.
	DefaultURLs []string
	// InventedGroups is appended after existing groups in the order given by
	// the caller (caller pre-sorts alphabetically).
	InventedGroups []InventedGroup
}

// InventedGroup is one new group invented by the CLI layer for non-convention
// repos. Name must already conform to ^[a-z][a-z0-9_-]*$ (the caller slugifies
// before constructing this); Dir must already have HOME-substitution applied.
type InventedGroup struct {
	Name string
	Dir  string
	URLs []string
}

// defaultGroupName is the conventional name for the auto-created flat group
// that holds convention-matching scan results. Spec § "Convention check".
const defaultGroupName = "default"

// MergeScan applies a structured plan of scan additions to the YAML file at
// path in a single atomic write. Comments are preserved (yaml.v3 round-trip;
// indentation normalized to yaml.v3 defaults — same contract as AppendURL).
//
// Dedup: any URL in plan.DefaultURLs or plan.InventedGroups[i].URLs that
// already appears in any existing group of the loaded file is silently
// skipped (matches AppendURL's contract and the parser's URL-uniqueness rule).
// The CLI layer is responsible for surfacing skip-by-dedup messages — yamled
// stays UI-free.
//
// Group ordering: existing groups preserved in source order; default placed
// per "Group ordering" rules; invented groups appended after existing groups
// in the order given by plan.InventedGroups (caller pre-sorts alphabetically).
func MergeScan(path string, plan ScanPlan) error {
	out, err := RenderScan(path, plan)
	if err != nil {
		return err
	}
	return atomicWrite(path, out)
}

// RenderScan returns the YAML bytes that MergeScan would write. Used by the
// CLI's print mode so it shares the exact render path with --write.
func RenderScan(path string, plan ScanPlan) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("yamled: read %s: %w", path, err)
	}

	var root yaml.Node
	if len(data) > 0 {
		if err := yaml.Unmarshal(data, &root); err != nil {
			return nil, fmt.Errorf("yamled: parse %s: %w", path, err)
		}
	}

	if err := mergeScanIntoTree(&root, plan); err != nil {
		return nil, err
	}

	out, err := yaml.Marshal(&root)
	if err != nil {
		return nil, fmt.Errorf("yamled: marshal: %w", err)
	}
	return out, nil
}

// mergeScanIntoTree mutates the parsed tree in place per the spec's group
// ordering and dedup rules. When the document is empty (no Content), it
// synthesizes a minimal `repos:` mapping so the merge has somewhere to land.
func mergeScanIntoTree(root *yaml.Node, plan ScanPlan) error {
	// Ensure the document has a top-level mapping.
	if len(root.Content) == 0 {
		root.Kind = yaml.DocumentNode
		root.Content = []*yaml.Node{
			{Kind: yaml.MappingNode, Tag: "!!map"},
		}
	}
	top := root.Content[0]
	if top.Kind != yaml.MappingNode {
		return fmt.Errorf("yamled: top-level is not a mapping")
	}

	// Find or create the `repos:` mapping.
	reposNode := mappingValue(top, "repos")
	if reposNode == nil {
		reposNode = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		top.Content = append(top.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "repos"},
			reposNode,
		)
	} else if reposNode.Kind != yaml.MappingNode {
		return fmt.Errorf("yamled: 'repos' is not a mapping")
	}

	// Build the dedup set: every URL already present in any existing group.
	existingURLs := collectExistingURLs(reposNode)

	// Apply default-group additions. Find existing default group (preserved
	// in source order) or create a new one (appended after existing groups
	// per spec § "Group ordering" #2).
	defaultURLs := dedupNew(plan.DefaultURLs, existingURLs)
	if len(defaultURLs) > 0 {
		defaultNode := mappingValue(reposNode, defaultGroupName)
		if defaultNode == nil {
			// Create a new flat-list group at the end of the existing groups
			// (before invented groups, which we append next).
			defaultNode = &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
			reposNode.Content = append(reposNode.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: defaultGroupName},
				defaultNode,
			)
		}
		if err := appendURLsToGroup(defaultNode, defaultGroupName, defaultURLs, existingURLs); err != nil {
			return err
		}
	}

	// Apply invented-group additions, in caller-given order.
	for _, ig := range plan.InventedGroups {
		urls := dedupNew(ig.URLs, existingURLs)
		if len(urls) == 0 {
			continue
		}
		// If a group with this name already exists, reuse its node (the CLI
		// layer's conflict resolution should have already suffixed any
		// dir-mismatch case to a fresh name; reuse here only fires when the
		// CLI matched on dir).
		groupNode := mappingValue(reposNode, ig.Name)
		if groupNode == nil {
			groupNode = makeMapShapedGroup(ig.Dir)
			reposNode.Content = append(reposNode.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: ig.Name},
				groupNode,
			)
		}
		if err := appendURLsToGroup(groupNode, ig.Name, urls, existingURLs); err != nil {
			return err
		}
	}

	return nil
}

// makeMapShapedGroup builds the node tree for a fresh map-shaped group
// `{ dir: <dir>, urls: [] }`. URLs are appended later via appendURLsToGroup.
func makeMapShapedGroup(dir string) *yaml.Node {
	urlsSeq := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	return &yaml.Node{
		Kind: yaml.MappingNode,
		Tag:  "!!map",
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: "dir"},
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: dir},
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: "urls"},
			urlsSeq,
		},
	}
}

// appendURLsToGroup appends urls to groupNode (sequence or map-with-urls
// shape), updating the existingURLs dedup set so subsequent groups in the
// same merge pass don't re-add a URL that landed earlier in this same plan.
func appendURLsToGroup(groupNode *yaml.Node, groupName string, urls []string, existingURLs map[string]struct{}) error {
	switch groupNode.Kind {
	case yaml.SequenceNode:
		for _, u := range urls {
			groupNode.Content = append(groupNode.Content, &yaml.Node{
				Kind: yaml.ScalarNode, Tag: "!!str", Value: u,
			})
			existingURLs[u] = struct{}{}
		}
		return nil
	case yaml.MappingNode:
		urlsNode := mappingValue(groupNode, "urls")
		if urlsNode == nil {
			// Create an empty urls sequence.
			urlsNode = &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
			groupNode.Content = append(groupNode.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "urls"},
				urlsNode,
			)
		}
		if urlsNode.Kind == yaml.ScalarNode && urlsNode.Tag == "!!null" {
			// `urls:` with null value — convert to a sequence in place.
			urlsNode.Kind = yaml.SequenceNode
			urlsNode.Tag = "!!seq"
			urlsNode.Value = ""
		}
		if urlsNode.Kind != yaml.SequenceNode {
			return fmt.Errorf("yamled: group '%s' has 'urls' field but it is not a list; cannot append", groupName)
		}
		for _, u := range urls {
			urlsNode.Content = append(urlsNode.Content, &yaml.Node{
				Kind: yaml.ScalarNode, Tag: "!!str", Value: u,
			})
			existingURLs[u] = struct{}{}
		}
		return nil
	default:
		return fmt.Errorf("yamled: group '%s' has unexpected shape; cannot append", groupName)
	}
}

// collectExistingURLs walks reposNode and returns the set of URLs across all
// groups (both flat-list and map-shaped). Used for silent dedup per spec §
// "MergeScan signature".
func collectExistingURLs(reposNode *yaml.Node) map[string]struct{} {
	out := make(map[string]struct{})
	if reposNode == nil || reposNode.Kind != yaml.MappingNode {
		return out
	}
	for i := 0; i+1 < len(reposNode.Content); i += 2 {
		body := reposNode.Content[i+1]
		switch body.Kind {
		case yaml.SequenceNode:
			for _, c := range body.Content {
				if c.Kind == yaml.ScalarNode {
					out[c.Value] = struct{}{}
				}
			}
		case yaml.MappingNode:
			urls := mappingValue(body, "urls")
			if urls != nil && urls.Kind == yaml.SequenceNode {
				for _, c := range urls.Content {
					if c.Kind == yaml.ScalarNode {
						out[c.Value] = struct{}{}
					}
				}
			}
		}
	}
	return out
}

// dedupNew returns the subset of urls not present in seen, preserving
// caller-given order. URLs already in seen are silently dropped per the
// MergeScan dedup contract.
func dedupNew(urls []string, seen map[string]struct{}) []string {
	if len(urls) == 0 {
		return nil
	}
	var out []string
	added := make(map[string]struct{}, len(urls))
	for _, u := range urls {
		if _, in := seen[u]; in {
			continue
		}
		if _, in := added[u]; in {
			continue
		}
		out = append(out, u)
		added[u] = struct{}{}
	}
	return out
}
