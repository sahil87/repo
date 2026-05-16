package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/sahil87/hop/internal/proc"
)

// withListWorktrees swaps the package-level listWorktrees seam for the
// duration of a test, restoring the original on Cleanup.
func withListWorktrees(t *testing.T, fn func(ctx context.Context, repoPath string) ([]WtEntry, error)) {
	t.Helper()
	prev := listWorktrees
	listWorktrees = fn
	t.Cleanup(func() { listWorktrees = prev })
}

// TestDefaultListWorktreesUnmarshalsValidJSON verifies the default
// implementation parses wt's documented schema into []WtEntry. We swap the
// inner RunCapture would require touching proc; instead we exercise the
// unmarshal half via a small inline wrapper that mirrors defaultListWorktrees
// without spawning a process.
func TestWtEntryUnmarshalsValidJSON(t *testing.T) {
	const blob = `[
	  {"name":"main","branch":"main","path":"/repo","is_main":true,"is_current":false,"dirty":false,"unpushed":0},
	  {"name":"feat-x","branch":"feat-x","path":"/repo.worktrees/feat-x","is_main":false,"is_current":true,"dirty":true,"unpushed":2}
	]`
	entries, err := unmarshalWtEntries([]byte(blob))
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Name != "main" || !entries[0].IsMain {
		t.Errorf("entry 0 = %+v, want name=main is_main=true", entries[0])
	}
	if entries[1].Name != "feat-x" || !entries[1].Dirty || entries[1].Unpushed != 2 {
		t.Errorf("entry 1 = %+v, want name=feat-x dirty=true unpushed=2", entries[1])
	}
}

// TestWtEntryIgnoresUnknownFields locks in the forward-compat contract: future
// wt schema additions must not break hop's unmarshal.
func TestWtEntryIgnoresUnknownFields(t *testing.T) {
	const blob = `[{"name":"main","branch":"main","path":"/p","is_main":true,"is_current":false,"dirty":false,"unpushed":0,"future_field":"surprise","another":42}]`
	entries, err := unmarshalWtEntries([]byte(blob))
	if err != nil {
		t.Fatalf("unexpected unmarshal error: %v", err)
	}
	if len(entries) != 1 || entries[0].Name != "main" {
		t.Fatalf("expected single main entry, got: %+v", entries)
	}
}

// TestWtEntryMalformedJSONErrors verifies a non-JSON response surfaces as an
// error, not a silent empty slice.
func TestWtEntryMalformedJSONErrors(t *testing.T) {
	_, err := unmarshalWtEntries([]byte(`{not-json}`))
	if err == nil {
		t.Fatalf("expected error for malformed JSON, got nil")
	}
}

// TestListWorktreesSeamPropagatesErrNotFound verifies the seam returns
// proc.ErrNotFound when the underlying invocation does — callers (resolve.go,
// ls.go, completion) rely on errors.Is(err, proc.ErrNotFound) to produce the
// install hint.
func TestListWorktreesSeamPropagatesErrNotFound(t *testing.T) {
	withListWorktrees(t, func(ctx context.Context, repoPath string) ([]WtEntry, error) {
		return nil, proc.ErrNotFound
	})
	_, err := listWorktrees(context.Background(), "/anywhere")
	if !errors.Is(err, proc.ErrNotFound) {
		t.Fatalf("expected ErrNotFound through seam, got %v", err)
	}
}

// TestListWorktreesSeamReturnsInjectedEntries verifies the basic injection
// path: tests can swap the seam to return canned entries.
func TestListWorktreesSeamReturnsInjectedEntries(t *testing.T) {
	want := []WtEntry{
		{Name: "main", Path: "/repo", IsMain: true},
		{Name: "feat-x", Path: "/repo.worktrees/feat-x", Dirty: true},
	}
	withListWorktrees(t, func(ctx context.Context, repoPath string) ([]WtEntry, error) {
		if repoPath != "/repo" {
			t.Errorf("seam called with repoPath=%q, want /repo", repoPath)
		}
		return want, nil
	})
	got, err := listWorktrees(context.Background(), "/repo")
	if err != nil {
		t.Fatalf("listWorktrees: %v", err)
	}
	if len(got) != 2 || got[0].Name != "main" || got[1].Name != "feat-x" {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

// TestDefaultListWorktreesWrapsUnmarshalError verifies that the wrapping
// prefix ("wt list: ") is present so callers can route the error through the
// "hop: wt list: <err>" stderr line without further wrapping.
func TestDefaultListWorktreesWrapsUnmarshalError(t *testing.T) {
	_, err := unmarshalWtEntries([]byte(`not json`))
	if err == nil || !strings.HasPrefix(err.Error(), "wt list:") {
		t.Fatalf("expected error prefixed with 'wt list:', got %v", err)
	}
}
