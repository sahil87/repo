package main

import (
	"strings"
	"testing"
)

// TestUpdateCobraWiring asserts that `hop update` is registered, accepts no
// args, and reaches the internal/update.Run code path. We exploit the fact
// that `go test` binaries do not live under /Cellar/, so the function
// short-circuits to the "not installed via Homebrew" branch — exercising the
// cobra plumbing without hitting brew.
//
// runArgs captures cmd.OutOrStdout() via SetOut, and update.Run writes its
// wrapper messages to the writer the cobra wrapper passes in (cmd.OutOrStdout()),
// so the captured buffer reflects the user-visible output.
func TestUpdateCobraWiring(t *testing.T) {
	stdout, _, err := runArgs(t, "update")
	if err != nil {
		t.Fatalf("hop update: %v", err)
	}
	if got := stdout.String(); !strings.Contains(got, "was not installed via Homebrew") {
		t.Fatalf("expected non-brew hint in stdout, got:\n%s", got)
	}
}

func TestUpdateRejectsArgs(t *testing.T) {
	_, _, err := runArgs(t, "update", "extra")
	if err == nil {
		t.Fatalf("expected error from `update extra` (cobra.NoArgs)")
	}
}

func TestUpdateAppearsInHelp(t *testing.T) {
	stdout, _, err := runArgs(t, "--help")
	if err != nil {
		t.Fatalf("hop --help: %v", err)
	}
	if !strings.Contains(stdout.String(), "hop update") {
		t.Fatalf("expected `hop update` line in --help output, got:\n%s", stdout.String())
	}
}
