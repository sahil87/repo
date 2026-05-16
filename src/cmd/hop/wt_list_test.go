package main

import (
	"context"
	"errors"
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

// TestWtEntryUnmarshalsValidJSON exercises the unmarshal half of the wt
// integration directly — swapping the inner RunCapture call in
// defaultListWorktrees would require touching internal/proc, so we test the
// pure JSON-to-WtEntry contract via unmarshalWtEntries instead.
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

// TestUnmarshalWtEntriesReturnsErrorVerbatim locks in the contract that
// unmarshalWtEntries does NOT prefix its error with "wt list:" — the prefix
// is owned by callers (resolveWorktreePath, runLsTrees) so the label appears
// at exactly one layer in the final user-facing message.
func TestUnmarshalWtEntriesReturnsErrorVerbatim(t *testing.T) {
	_, err := unmarshalWtEntries([]byte(`not json`))
	if err == nil {
		t.Fatalf("expected error for malformed JSON, got nil")
	}
	if msg := err.Error(); len(msg) >= 8 && msg[:8] == "wt list:" {
		t.Fatalf("error should not be prefixed with 'wt list:' (callers own that prefix); got %q", msg)
	}
}
