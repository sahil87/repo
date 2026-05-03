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
// If the binary is not on PATH, the returned error wraps ErrNotFound.
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
