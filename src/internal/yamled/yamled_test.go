package yamled

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppendURLFlatList(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hop.yaml")
	original := `# top comment
config:
  code_root: ~/code

repos:
  default:
    - git@github.com:sahil87/hop.git    # the locator tool
    - git@github.com:sahil87/wt.git
`
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := AppendURL(path, "default", "git@github.com:sahil87/outbox.git"); err != nil {
		t.Fatalf("AppendURL: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	gotStr := string(got)

	if !strings.Contains(gotStr, "# top comment") {
		t.Errorf("top comment lost; got:\n%s", gotStr)
	}
	if !strings.Contains(gotStr, "# the locator tool") {
		t.Errorf("inline comment lost; got:\n%s", gotStr)
	}
	if !strings.Contains(gotStr, "git@github.com:sahil87/outbox.git") {
		t.Errorf("appended URL missing; got:\n%s", gotStr)
	}
	// Order check: outbox should appear after wt
	wtIdx := strings.Index(gotStr, "wt.git")
	outboxIdx := strings.Index(gotStr, "outbox.git")
	if wtIdx < 0 || outboxIdx < 0 || outboxIdx < wtIdx {
		t.Errorf("appended URL not at end of list; got:\n%s", gotStr)
	}
}

func TestAppendURLMapGroup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hop.yaml")
	original := `repos:
  vendor:
    dir: ~/vendor
    urls:
      - git@github.com:vendor/tool-a.git    # first vendor tool
`
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := AppendURL(path, "vendor", "git@github.com:vendor/tool-b.git"); err != nil {
		t.Fatalf("AppendURL: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	gotStr := string(got)

	if !strings.Contains(gotStr, "# first vendor tool") {
		t.Errorf("inline comment lost; got:\n%s", gotStr)
	}
	if !strings.Contains(gotStr, "tool-b.git") {
		t.Errorf("appended URL missing; got:\n%s", gotStr)
	}
	if !strings.Contains(gotStr, "dir: ~/vendor") {
		t.Errorf("dir field lost; got:\n%s", gotStr)
	}
}

func TestAppendURLMissingGroup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hop.yaml")
	original := `repos:
  default:
    - git@github.com:sahil87/hop.git
`
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	err := AppendURL(path, "experiments", "git@github.com:foo/bar.git")
	if err == nil {
		t.Fatal("expected error for missing group, got nil")
	}
	if !strings.Contains(err.Error(), "experiments") || !strings.Contains(err.Error(), "not found") {
		t.Errorf("unexpected error message: %v", err)
	}
	if !errors.Is(err, ErrGroupNotFound) {
		t.Errorf("expected errors.Is(err, ErrGroupNotFound), got %v", err)
	}

	// Verify file is unchanged
	got, _ := os.ReadFile(path)
	if string(got) != original {
		t.Errorf("file was modified despite error; got:\n%s", got)
	}
}

func TestAppendURLMapGroupNoUrls(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hop.yaml")
	original := `repos:
  vendor:
    dir: ~/vendor
`
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	err := AppendURL(path, "vendor", "git@github.com:vendor/tool.git")
	if err == nil {
		t.Fatal("expected error for map-shaped group missing urls, got nil")
	}
	if !strings.Contains(err.Error(), "no 'urls' field") {
		t.Errorf("unexpected error message: %v", err)
	}

	got, _ := os.ReadFile(path)
	if string(got) != original {
		t.Errorf("file was modified despite error; got:\n%s", got)
	}
}

// TestAppendURLNonDefaultIndentRoundTrips verifies that even when the source
// file uses non-default indentation, the round-trip succeeds and the resulting
// file remains valid YAML containing both the original and appended URLs.
// (Indentation itself is normalized to yaml.v3's defaults — see yamled.go.)
func TestAppendURLNonDefaultIndentRoundTrips(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hop.yaml")
	original := "repos:\n    default:\n        - git@github.com:foo/a.git\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := AppendURL(path, "default", "git@github.com:foo/b.git"); err != nil {
		t.Fatalf("AppendURL: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	gotStr := string(got)
	if !strings.Contains(gotStr, "b.git") {
		t.Errorf("appended URL missing; got:\n%s", gotStr)
	}
	if !strings.Contains(gotStr, "a.git") {
		t.Errorf("original URL missing; got:\n%s", gotStr)
	}
}

// TestAppendURLLeavesOriginalOnTempCreateFail verifies that when the parent
// directory is read-only (so os.CreateTemp fails before any write), AppendURL
// returns an error and the original file is left untouched. This exercises the
// "tmp creation fails" branch of atomicWrite, not the rename-failure branch —
// rename failure is much harder to provoke deterministically across platforms.
func TestAppendURLLeavesOriginalOnTempCreateFail(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("test relies on permission denial; skipping when running as root")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "hop.yaml")
	original := `repos:
  default:
    - git@github.com:foo/a.git
`
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// chmod the directory to read-only — temp file creation will fail.
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	defer os.Chmod(dir, 0o755)

	err := AppendURL(path, "default", "git@github.com:foo/b.git")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Restore perms and verify original unchanged.
	os.Chmod(dir, 0o755)
	got, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("read: %v", readErr)
	}
	if string(got) != original {
		t.Errorf("file was modified despite error; got:\n%s", got)
	}
}

// TestAppendURLPreservesFileMode verifies that AppendURL retains the original
// file's permissions on the replacement, instead of inheriting os.CreateTemp's
// 0600 default. Regression guard for the perm-downgrade reported in PR #5.
func TestAppendURLPreservesFileMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hop.yaml")
	original := `repos:
  default:
    - git@github.com:foo/a.git
`
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := AppendURL(path, "default", "git@github.com:foo/b.git"); err != nil {
		t.Fatalf("AppendURL: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o644 {
		t.Errorf("file mode = %o, want 0644 (yamled must preserve original perms, not adopt CreateTemp's 0600)", got)
	}
}

// --- MergeScan / RenderScan tests ----------------------------------------

func TestMergeScanCreatesDefaultGroupWhenAbsent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hop.yaml")
	original := `repos:
  vendor:
    dir: ~/vendor
    urls:
      - git@github.com:vendor/tool.git
`
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	plan := ScanPlan{
		DefaultURLs: []string{"git@github.com:foo/bar.git"},
	}
	if err := MergeScan(path, plan); err != nil {
		t.Fatalf("MergeScan: %v", err)
	}
	got, _ := os.ReadFile(path)
	gotStr := string(got)

	if !strings.Contains(gotStr, "default:") {
		t.Errorf("default group not created; got:\n%s", gotStr)
	}
	if !strings.Contains(gotStr, "git@github.com:foo/bar.git") {
		t.Errorf("URL not appended; got:\n%s", gotStr)
	}
	// vendor preserved.
	if !strings.Contains(gotStr, "vendor:") || !strings.Contains(gotStr, "git@github.com:vendor/tool.git") {
		t.Errorf("vendor group lost; got:\n%s", gotStr)
	}
}

func TestMergeScanAppendsToExistingDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hop.yaml")
	original := `repos:
  default:
    - git@github.com:foo/a.git
`
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	plan := ScanPlan{
		DefaultURLs: []string{"git@github.com:foo/b.git"},
	}
	if err := MergeScan(path, plan); err != nil {
		t.Fatalf("MergeScan: %v", err)
	}
	got, _ := os.ReadFile(path)
	gotStr := string(got)
	aIdx := strings.Index(gotStr, "a.git")
	bIdx := strings.Index(gotStr, "b.git")
	if aIdx < 0 || bIdx < 0 || bIdx < aIdx {
		t.Errorf("expected b after a in default group; got:\n%s", gotStr)
	}
}

func TestMergeScanDedupesAcrossAllGroups(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hop.yaml")
	original := `repos:
  vendor:
    - git@github.com:foo/bar.git
`
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	plan := ScanPlan{
		DefaultURLs: []string{"git@github.com:foo/bar.git"}, // already in vendor
	}
	if err := MergeScan(path, plan); err != nil {
		t.Fatalf("MergeScan: %v", err)
	}
	got, _ := os.ReadFile(path)
	gotStr := string(got)
	// URL must NOT appear under default — silently dropped by dedup.
	if strings.Contains(gotStr, "default:") {
		t.Errorf("default group should not have been created (URL was a dup); got:\n%s", gotStr)
	}
	// And the URL is still in vendor.
	if strings.Count(gotStr, "git@github.com:foo/bar.git") != 1 {
		t.Errorf("expected URL exactly once (no dup); got:\n%s", gotStr)
	}
}

func TestMergeScanInventedGroupAsMapShape(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hop.yaml")
	original := `repos:
  default:
    - git@github.com:owner/known.git
`
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	plan := ScanPlan{
		InventedGroups: []InventedGroup{
			{Name: "vendor", Dir: "~/vendor", URLs: []string{"git@github.com:vendor/tool.git"}},
		},
	}
	if err := MergeScan(path, plan); err != nil {
		t.Fatalf("MergeScan: %v", err)
	}
	got, _ := os.ReadFile(path)
	gotStr := string(got)
	if !strings.Contains(gotStr, "vendor:") {
		t.Errorf("vendor group missing; got:\n%s", gotStr)
	}
	if !strings.Contains(gotStr, "dir: ~/vendor") {
		t.Errorf("vendor.dir missing; got:\n%s", gotStr)
	}
	if !strings.Contains(gotStr, "git@github.com:vendor/tool.git") {
		t.Errorf("vendor URL missing; got:\n%s", gotStr)
	}
	// Order: default before vendor (existing groups first; invented after).
	defIdx := strings.Index(gotStr, "default:")
	venIdx := strings.Index(gotStr, "vendor:")
	if defIdx < 0 || venIdx < 0 || venIdx < defIdx {
		t.Errorf("expected default before vendor; got:\n%s", gotStr)
	}
}

func TestMergeScanPreservesComments(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hop.yaml")
	original := `# top-level comment
config:
  code_root: ~/code

repos:
  default:
    - git@github.com:foo/a.git    # inline comment on a
`
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	plan := ScanPlan{DefaultURLs: []string{"git@github.com:foo/b.git"}}
	if err := MergeScan(path, plan); err != nil {
		t.Fatalf("MergeScan: %v", err)
	}
	got, _ := os.ReadFile(path)
	gotStr := string(got)
	if !strings.Contains(gotStr, "# top-level comment") {
		t.Errorf("top comment lost; got:\n%s", gotStr)
	}
	if !strings.Contains(gotStr, "# inline comment on a") {
		t.Errorf("inline comment lost; got:\n%s", gotStr)
	}
	if !strings.Contains(gotStr, "b.git") {
		t.Errorf("appended URL missing; got:\n%s", gotStr)
	}
}

func TestMergeScanInventedGroupOrderPreserved(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hop.yaml")
	if err := os.WriteFile(path, []byte("repos:\n  default: []\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Caller pre-sorts alphabetically per spec; we verify that the
	// caller-given order is preserved verbatim.
	plan := ScanPlan{
		InventedGroups: []InventedGroup{
			{Name: "alpha", Dir: "~/a", URLs: []string{"git@github.com:owner/alpha.git"}},
			{Name: "mango", Dir: "~/m", URLs: []string{"git@github.com:owner/mango.git"}},
			{Name: "zebra", Dir: "~/z", URLs: []string{"git@github.com:owner/zebra.git"}},
		},
	}
	if err := MergeScan(path, plan); err != nil {
		t.Fatalf("MergeScan: %v", err)
	}
	got, _ := os.ReadFile(path)
	gotStr := string(got)
	// Verify positional order alpha < mango < zebra.
	idxA := strings.Index(gotStr, "alpha:")
	idxM := strings.Index(gotStr, "mango:")
	idxZ := strings.Index(gotStr, "zebra:")
	if idxA < 0 || idxM < 0 || idxZ < 0 || !(idxA < idxM && idxM < idxZ) {
		t.Errorf("invented groups out of order; got:\n%s", gotStr)
	}
}

func TestRenderScanMatchesMergeScan(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hop.yaml")
	original := `repos:
  default:
    - git@github.com:foo/a.git
`
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	plan := ScanPlan{
		DefaultURLs: []string{"git@github.com:foo/b.git"},
		InventedGroups: []InventedGroup{
			{Name: "vendor", Dir: "~/vendor", URLs: []string{"git@github.com:vendor/tool.git"}},
		},
	}
	rendered, err := RenderScan(path, plan)
	if err != nil {
		t.Fatalf("RenderScan: %v", err)
	}

	if err := MergeScan(path, plan); err != nil {
		t.Fatalf("MergeScan: %v", err)
	}
	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(rendered) != string(written) {
		t.Errorf("RenderScan and MergeScan produced different bytes:\nRENDER:\n%s\n---\nWRITE:\n%s", rendered, written)
	}
}

func TestMergeScanPreservesFileMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hop.yaml")
	if err := os.WriteFile(path, []byte("repos:\n  default: []\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := MergeScan(path, ScanPlan{DefaultURLs: []string{"git@github.com:foo/x.git"}}); err != nil {
		t.Fatalf("MergeScan: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("file mode = %o, want 0600 (preserved from original)", got)
	}
}

func TestMergeScanLeavesOriginalOnTempCreateFail(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("test relies on permission denial; skipping when running as root")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "hop.yaml")
	original := "repos:\n  default:\n    - git@github.com:foo/a.git\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	defer os.Chmod(dir, 0o755)

	err := MergeScan(path, ScanPlan{DefaultURLs: []string{"git@github.com:foo/b.git"}})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	os.Chmod(dir, 0o755)
	got, _ := os.ReadFile(path)
	if string(got) != original {
		t.Errorf("file modified despite error; got:\n%s", got)
	}
}

func TestMergeScanReuseExistingInventedGroupByName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hop.yaml")
	original := `repos:
  vendor:
    dir: ~/vendor
    urls:
      - git@github.com:vendor/tool-a.git
`
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	// CLI-layer conflict resolution determined this matches existing vendor;
	// reuse that group rather than create a duplicate.
	plan := ScanPlan{
		InventedGroups: []InventedGroup{
			{Name: "vendor", Dir: "~/vendor", URLs: []string{"git@github.com:vendor/tool-b.git"}},
		},
	}
	if err := MergeScan(path, plan); err != nil {
		t.Fatalf("MergeScan: %v", err)
	}
	got, _ := os.ReadFile(path)
	gotStr := string(got)
	// Both URLs under the single vendor group.
	if strings.Count(gotStr, "vendor:") != 1 {
		t.Errorf("expected single vendor group; got:\n%s", gotStr)
	}
	if !strings.Contains(gotStr, "tool-a.git") || !strings.Contains(gotStr, "tool-b.git") {
		t.Errorf("expected both tools under vendor; got:\n%s", gotStr)
	}
}
