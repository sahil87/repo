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

func TestAppendURLAtomicLeavesOriginalOnRenameFail(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("test relies on permission denial; skipping when running as root")
	}
	// Make the directory read-only after creating the file, so rename fails.
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
