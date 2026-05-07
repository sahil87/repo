package main

import (
	"strings"
	"testing"
)

func TestSyncUsageErrorWhenNoArgsAndNoAll(t *testing.T) {
	_, _, _ = pullSyncYAMLFixture(t)

	_, _, err := runArgs(t, "sync")
	if err == nil {
		t.Fatalf("expected usage error")
	}
	if !strings.Contains(err.Error(), "missing <name-or-group>") {
		t.Fatalf("expected missing-arg hint, got %q", err.Error())
	}
}

func TestSyncUsageErrorWhenAllAndPositional(t *testing.T) {
	_, _, _ = pullSyncYAMLFixture(t)

	_, _, err := runArgs(t, "sync", "alpha", "--all")
	if err == nil {
		t.Fatalf("expected usage error for --all + positional")
	}
	if !strings.Contains(err.Error(), "--all conflicts with positional") {
		t.Fatalf("expected conflict hint, got %q", err.Error())
	}
}

func TestSyncSingleNotClonedExitsWithSkipMessage(t *testing.T) {
	_, _, _ = pullSyncYAMLFixture(t)

	_, stderr, err := runArgs(t, "sync", "alpha")
	if err == nil {
		t.Fatalf("expected error for not-cloned single repo")
	}
	if !strings.Contains(stderr.String(), "skip: alpha not cloned") {
		t.Fatalf("expected skip line, got stderr=%q", stderr.String())
	}
}

func TestSyncBatchGroupSkipsAllNotClonedAndReportsSummary(t *testing.T) {
	_, _, _ = pullSyncYAMLFixture(t)

	_, stderr, err := runArgs(t, "sync", "default")
	if err != nil {
		t.Fatalf("expected nil err for all-skipped batch, got %v", err)
	}
	got := stderr.String()
	if !strings.Contains(got, "skip: alpha not cloned") {
		t.Errorf("expected skip alpha, got: %s", got)
	}
	if !strings.Contains(got, "skip: beta not cloned") {
		t.Errorf("expected skip beta, got: %s", got)
	}
	if !strings.Contains(got, "summary: synced=0 skipped=2 failed=0") {
		t.Errorf("expected sync summary line, got: %s", got)
	}
}

func TestSyncStdoutIsEmpty(t *testing.T) {
	_, _, _ = pullSyncYAMLFixture(t)

	stdout, _, _ := runArgs(t, "sync", "--all")
	if got := stdout.String(); got != "" {
		t.Fatalf("expected empty stdout, got %q", got)
	}
}

func TestSyncCobraRejectsTwoPositionals(t *testing.T) {
	_, _, _ = pullSyncYAMLFixture(t)

	_, _, err := runArgs(t, "sync", "alpha", "beta")
	if err == nil {
		t.Fatalf("expected cobra to reject 2 positionals")
	}
	if !strings.Contains(err.Error(), "accepts at most 1 arg") {
		t.Fatalf("expected cobra MaximumNArgs error, got: %v", err)
	}
}

func TestMentionsConflictDetectsRebaseMarker(t *testing.T) {
	if !mentionsConflict("error: could not apply 1234... CONFLICT (content)", nil) {
		t.Errorf("expected stdout CONFLICT to be detected")
	}
	if mentionsConflict("Already up to date.", nil) {
		t.Errorf("did not expect false positive on clean output")
	}
}
