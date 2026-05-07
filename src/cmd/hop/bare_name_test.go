package main

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// TestBareNameHint verifies the binary's 1-arg form returns a code-2
// errExitCode whose message is the bareNameHint constant verbatim, with
// empty stdout (no path leak on the error path).
func TestBareNameHint(t *testing.T) {
	writeReposFixture(t, singleRepoYAML)

	stdout, _, err := runArgs(t, "hop")
	if err == nil {
		t.Fatalf("expected error from 1-arg bare form (cobra positional `hop`)")
	}
	var withCode *errExitCode
	if !errors.As(err, &withCode) {
		t.Fatalf("expected *errExitCode, got %T: %v", err, err)
	}
	if withCode.code != 2 {
		t.Fatalf("expected exit code 2, got %d", withCode.code)
	}
	if withCode.msg != bareNameHint {
		t.Fatalf("hint mismatch:\n  want: %s\n  got:  %s", bareNameHint, withCode.msg)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected empty stdout on bare-name error, got: %q", stdout.String())
	}
}

// TestBareNameCdVerb verifies `hop <name> cd` returns a code-2 errExitCode
// whose message equals the cdHint constant verbatim and contains the new
// `cd "$(hop "<name>" where)"` fallback example. Stdout is empty.
func TestBareNameCdVerb(t *testing.T) {
	writeReposFixture(t, singleRepoYAML)

	stdout, _, err := runArgs(t, "hop", "cd")
	if err == nil {
		t.Fatalf("expected error from 2-arg cd verb (cobra positionals `hop cd`)")
	}
	var withCode *errExitCode
	if !errors.As(err, &withCode) {
		t.Fatalf("expected *errExitCode, got %T: %v", err, err)
	}
	if withCode.code != 2 {
		t.Fatalf("expected exit code 2, got %d", withCode.code)
	}
	if withCode.msg != cdHint {
		t.Fatalf("hint mismatch:\n  want: %s\n  got:  %s", cdHint, withCode.msg)
	}
	// Pin the new wording — the fallback example uses the new repo-first form.
	if !strings.Contains(withCode.msg, `cd "$(hop "<name>" where)"`) {
		t.Fatalf("expected hint to contain new repo-first fallback example, got: %s", withCode.msg)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected empty stdout on cd-verb error, got: %q", stdout.String())
	}
}

// TestBareNameWhereVerb verifies `hop <name> where` resolves and prints the
// path to stdout, with empty stderr (no diagnostic noise on the happy path).
func TestBareNameWhereVerb(t *testing.T) {
	writeReposFixture(t, singleRepoYAML)

	stdout, stderr, err := runArgs(t, "hop", "where")
	if err != nil {
		t.Fatalf("hop hop where: %v", err)
	}
	got := strings.TrimSpace(stdout.String())
	if got != "/tmp/test-repos/hop" {
		t.Fatalf("expected /tmp/test-repos/hop, got %q", got)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr on where-verb happy path, got: %q", stderr.String())
	}
}

// TestBareNameToolForm verifies tool-form attempts at the binary error with
// the parameterized tool-form hint matching `fmt.Sprintf(toolFormHintFmt, args[1])`
// byte-for-byte. Stdout is empty.
func TestBareNameToolForm(t *testing.T) {
	writeReposFixture(t, singleRepoYAML)

	stdout, _, err := runArgs(t, "hop", "cursor")
	if err == nil {
		t.Fatalf("expected error from tool-form attempt (`hop hop cursor`)")
	}
	var withCode *errExitCode
	if !errors.As(err, &withCode) {
		t.Fatalf("expected *errExitCode, got %T: %v", err, err)
	}
	if withCode.code != 2 {
		t.Fatalf("expected exit code 2, got %d", withCode.code)
	}
	want := fmt.Sprintf(toolFormHintFmt, "cursor")
	if withCode.msg != want {
		t.Fatalf("tool-form hint mismatch:\n  want: %s\n  got:  %s", want, withCode.msg)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected empty stdout on tool-form error, got: %q", stdout.String())
	}
}

// TestVerbW_BinaryFormPrintsHint verifies `hop <name> w` (direct binary
// invocation, no shim) returns a code-2 errExitCode whose message equals
// `wHint` verbatim, with empty stdout. The shim handles `w` entirely shell-side
// (calling tmux); the binary's role is to point unaware users at the shim.
func TestVerbW_BinaryFormPrintsHint(t *testing.T) {
	writeReposFixture(t, singleRepoYAML)

	stdout, _, err := runArgs(t, "hop", "w")
	if err == nil {
		t.Fatalf("expected error from 2-arg w verb (cobra positionals `hop w`)")
	}
	var withCode *errExitCode
	if !errors.As(err, &withCode) {
		t.Fatalf("expected *errExitCode, got %T: %v", err, err)
	}
	if withCode.code != 2 {
		t.Fatalf("expected exit code 2, got %d", withCode.code)
	}
	if withCode.msg != wHint {
		t.Fatalf("hint mismatch:\n  want: %s\n  got:  %s", wHint, withCode.msg)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected empty stdout on w-verb error, got: %q", stdout.String())
	}
}

// TestVerbS_BinaryFormPrintsHint verifies `hop <name> s` (direct binary
// invocation, no shim) returns a code-2 errExitCode whose message equals
// `sHint` verbatim, with empty stdout. Same shape as TestVerbW_BinaryFormPrintsHint.
func TestVerbS_BinaryFormPrintsHint(t *testing.T) {
	writeReposFixture(t, singleRepoYAML)

	stdout, _, err := runArgs(t, "hop", "s")
	if err == nil {
		t.Fatalf("expected error from 2-arg s verb (cobra positionals `hop s`)")
	}
	var withCode *errExitCode
	if !errors.As(err, &withCode) {
		t.Fatalf("expected *errExitCode, got %T: %v", err, err)
	}
	if withCode.code != 2 {
		t.Fatalf("expected exit code 2, got %d", withCode.code)
	}
	if withCode.msg != sHint {
		t.Fatalf("hint mismatch:\n  want: %s\n  got:  %s", sHint, withCode.msg)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected empty stdout on s-verb error, got: %q", stdout.String())
	}
}

// TestToolFormHintEnumeratesAllVerbs verifies the tool-form fall-through error
// message enumerates all four hop verbs (`cd, where, w, s`). This pins the
// `toolFormHintFmt` text — TestBareNameToolForm does byte-equality on the
// formatted result, but a substring assertion makes the verb-enumeration
// intent explicit and survives reformatting of the surrounding text.
func TestToolFormHintEnumeratesAllVerbs(t *testing.T) {
	writeReposFixture(t, singleRepoYAML)

	_, _, err := runArgs(t, "hop", "notreal")
	if err == nil {
		t.Fatalf("expected error from tool-form attempt with unknown verb")
	}
	var withCode *errExitCode
	if !errors.As(err, &withCode) {
		t.Fatalf("expected *errExitCode, got %T: %v", err, err)
	}
	if withCode.code != 2 {
		t.Fatalf("expected exit code 2, got %d", withCode.code)
	}
	if !strings.Contains(withCode.msg, "(cd, where, w, s)") {
		t.Fatalf("expected tool-form hint to enumerate all four verbs `(cd, where, w, s)`, got: %s", withCode.msg)
	}
}

// TestBareNameMaxArgs verifies cobra's MaximumNArgs(2) cap rejects 3+
// positional args BEFORE RunE runs — the error is cobra's args-validator
// message ("accepts at most 2 arg(s)..."), NOT a hop-owned errExitCode.
// Pinning the cobra shape catches accidental cap removal: a future bump to
// MaximumNArgs(3) would reroute through RunE's tool-form branch and silently
// pass a loose `err != nil` check.
func TestBareNameMaxArgs(t *testing.T) {
	writeReposFixture(t, singleRepoYAML)

	_, _, err := runArgs(t, "hop", "cursor", "extra")
	if err == nil {
		t.Fatalf("expected error from 3 positionals (cap = 2)")
	}
	// The cap is enforced by cobra's args validator, not RunE — assert the
	// error is NOT an *errExitCode (which would mean RunE ran).
	var withCode *errExitCode
	if errors.As(err, &withCode) {
		t.Fatalf("expected cobra args-validator error, got *errExitCode (RunE ran — cap may have been bumped): %v", err)
	}
	if !strings.Contains(err.Error(), "accepts at most 2 arg") {
		t.Fatalf("expected cobra `accepts at most 2 arg` message, got: %v", err)
	}
}
