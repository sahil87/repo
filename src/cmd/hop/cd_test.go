package main

import (
	"errors"
	"strings"
	"testing"
)

func TestCdReturnsExitCodeError(t *testing.T) {
	_, _, err := runArgs(t, "cd", "anything")
	if err == nil {
		t.Fatalf("expected error from `hop cd`")
	}
	var withCode *errExitCode
	if !errors.As(err, &withCode) {
		t.Fatalf("expected *errExitCode, got %T: %v", err, err)
	}
	if withCode.code != 2 {
		t.Fatalf("expected exit code 2, got %d", withCode.code)
	}
	if !strings.Contains(withCode.msg, "'cd' is shell-only") {
		t.Fatalf("expected hint message, got %q", withCode.msg)
	}
	// Verify exact byte match against the spec.
	if withCode.msg != cdHint {
		t.Fatalf("hint mismatch:\n  want: %s\n  got:  %s", cdHint, withCode.msg)
	}
}
