// Package proc is the centralized subprocess-execution wrapper for the repo binary.
// All external-tool invocations (git, fzf, code, open, xdg-open) MUST go through this
// package — Constitution Principle I (Security First) requires this. No package outside
// internal/proc may import os/exec directly.
package proc

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
)

// ErrNotFound is returned by Run/RunInteractive when the named binary is not on PATH.
// Callers can match this with errors.Is to produce install-hint messages.
var ErrNotFound = errors.New("binary not found on PATH")

// Run invokes name with args using exec.CommandContext, returns stdout as bytes.
// stderr passes through to the parent's stderr so subprocess error messages reach the user.
// If the binary is not on PATH, the returned error is ErrNotFound (callers can match
// it directly or via errors.Is).
func Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return nil, ErrNotFound
		}
		return stdout.Bytes(), err
	}
	return stdout.Bytes(), nil
}

// ExitCode reports the subprocess exit code carried by err. It returns (code, true)
// when err wraps an *exec.ExitError (i.e., the subprocess ran and exited with a
// non-zero status), and (0, false) otherwise (e.g., I/O error, ErrNotFound, nil).
// Callers use this to discriminate between "subprocess exited with code N" and
// other failure modes — e.g., fzf exit 130 is user cancellation, but an exec error
// or non-130 exit is a real failure that should not be silently swallowed.
//
// This helper exists so callers can stay outside os/exec (Constitution Principle I:
// only internal/proc imports os/exec).
func ExitCode(err error) (int, bool) {
	if err == nil {
		return 0, false
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), true
	}
	return 0, false
}

// RunInteractive invokes name with args using exec.CommandContext and pipes stdin from
// the provided reader. stdout is captured and returned as a string; stderr passes through
// to the parent's stderr. If the binary is not on PATH, returns ErrNotFound.
func RunInteractive(ctx context.Context, stdin io.Reader, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = stdin
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return "", ErrNotFound
		}
		return stdout.String(), err
	}
	return stdout.String(), nil
}
