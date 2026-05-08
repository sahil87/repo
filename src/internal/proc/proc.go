// Package proc is the centralized subprocess-execution wrapper for the hop binary.
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

// RunCapture is Run with an explicit working directory: cmd.Dir is set to dir
// (equivalent to running `git -C dir ...` but driven via cmd.Dir rather than a
// `-C` argument, so the subprocess sees the canonical cwd directly). Captures
// stdout to bytes; stderr passes through to the parent. Used by internal/scan
// for per-repo `git remote` / `git remote get-url` invocations.
//
// If the binary is not on PATH, the returned error is ErrNotFound. dir is
// passed verbatim to exec — callers SHOULD validate it (e.g., via os.Stat)
// before calling, per Constitution Principle I.
func RunCapture(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
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

// RunCaptureBoth is RunCapture with stderr also captured into a returned
// buffer while still streaming it verbatim to the parent's stderr (via
// io.MultiWriter). Use this when the caller needs to inspect stderr content
// (e.g., to detect markers like git's "CONFLICT") without suppressing the
// user-visible output.
//
// If the binary is not on PATH, the returned error is ErrNotFound. dir is
// passed verbatim to exec — callers SHOULD validate it (e.g., via os.Stat)
// before calling, per Constitution Principle I.
func RunCaptureBoth(ctx context.Context, dir, name string, args ...string) (stdout, stderr []byte, err error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = io.MultiWriter(&errBuf, os.Stderr)
	if err := cmd.Run(); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return nil, nil, ErrNotFound
		}
		return outBuf.Bytes(), errBuf.Bytes(), err
	}
	return outBuf.Bytes(), errBuf.Bytes(), nil
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

// RunForeground invokes name+args with Dir set to dir and stdin/stdout/stderr
// inherited from the parent. The exit code of the subprocess is returned via
// the (code, error) pair: when the subprocess runs to completion, code is its
// exit code and error is nil. When exec fails before the subprocess starts
// (binary not found, dir does not exist, or other I/O error), code is -1 and
// error is non-nil. Use errors.Is(err, ErrNotFound) to detect missing binary.
//
// Used by `hop -C <name> <cmd>...` to delegate to a child command in a
// resolved repo's directory.
func RunForeground(ctx context.Context, dir, name string, args ...string) (int, error) {
	return RunForegroundEnv(ctx, dir, nil, name, args...)
}

// RunForegroundEnv is RunForeground with an explicit env override. When env is
// nil, the subprocess inherits the parent's environment (identical to
// RunForeground). When env is non-nil, the subprocess sees exactly those
// entries — callers SHOULD start from os.Environ() and append/override entries
// to extend the parent env rather than replace it.
//
// Used by `hop <name> open` to set WT_CD_FILE and WT_WRAPPER on top of the
// parent env when delegating to wt.
func RunForegroundEnv(ctx context.Context, dir string, env []string, name string, args ...string) (int, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = env
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return -1, ErrNotFound
		}
		if code, ok := ExitCode(err); ok {
			return code, nil
		}
		return -1, err
	}
	return 0, nil
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
