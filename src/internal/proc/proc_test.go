package proc

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestRunEcho(t *testing.T) {
	ctx := context.Background()
	out, err := Run(ctx, "echo", "hello")
	if err != nil {
		t.Fatalf("Run echo: %v", err)
	}
	if got := strings.TrimSpace(string(out)); got != "hello" {
		t.Fatalf("expected 'hello', got %q", got)
	}
}

func TestRunNotFound(t *testing.T) {
	ctx := context.Background()
	_, err := Run(ctx, "this-binary-does-not-exist-xyz123")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestRunInteractiveStdin(t *testing.T) {
	ctx := context.Background()
	out, err := RunInteractive(ctx, strings.NewReader("piped-input\n"), "cat")
	if err != nil {
		t.Fatalf("RunInteractive cat: %v", err)
	}
	if got := strings.TrimSpace(out); got != "piped-input" {
		t.Fatalf("expected 'piped-input', got %q", got)
	}
}

func TestRunInteractiveNotFound(t *testing.T) {
	ctx := context.Background()
	_, err := RunInteractive(ctx, strings.NewReader(""), "this-binary-does-not-exist-xyz123")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestRunContextCancel(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err := Run(ctx, "sleep", "5")
	if err == nil {
		t.Fatalf("expected error from context cancellation, got nil")
	}
}

func TestRunForegroundFalse(t *testing.T) {
	ctx := context.Background()
	code, err := RunForeground(ctx, "/", "false")
	if err != nil {
		t.Fatalf("RunForeground false: %v", err)
	}
	if code != 1 {
		t.Fatalf("expected exit 1 from false, got %d", code)
	}
}

func TestRunForegroundTrue(t *testing.T) {
	ctx := context.Background()
	code, err := RunForeground(ctx, "/", "true")
	if err != nil {
		t.Fatalf("RunForeground true: %v", err)
	}
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
}

func TestRunForegroundNotFound(t *testing.T) {
	ctx := context.Background()
	code, err := RunForeground(ctx, "/", "this-binary-does-not-exist-xyz123")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	if code != -1 {
		t.Fatalf("expected code -1 on missing binary, got %d", code)
	}
}

func TestRunForegroundDirMissing(t *testing.T) {
	ctx := context.Background()
	code, err := RunForeground(ctx, "/no/such/dir", "true")
	if err == nil {
		t.Fatalf("expected error for missing dir, got nil")
	}
	if code != -1 {
		t.Fatalf("expected code -1, got %d", code)
	}
}

func TestRunCaptureSuccess(t *testing.T) {
	ctx := context.Background()
	out, err := RunCapture(ctx, "/", "echo", "captured")
	if err != nil {
		t.Fatalf("RunCapture echo: %v", err)
	}
	if got := strings.TrimSpace(string(out)); got != "captured" {
		t.Fatalf("expected 'captured', got %q", got)
	}
}

func TestRunCaptureSetsDir(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	out, err := RunCapture(ctx, dir, "pwd")
	if err != nil {
		t.Fatalf("RunCapture pwd: %v", err)
	}
	// On macOS /tmp resolves to /private/tmp; pwd inside RunCapture should
	// reflect cmd.Dir verbatim (no EvalSymlinks). Compare against the dir
	// passed in.
	if got := strings.TrimSpace(string(out)); got != dir {
		t.Fatalf("expected cwd %q, got %q", dir, got)
	}
}

func TestRunCaptureNonZeroExit(t *testing.T) {
	ctx := context.Background()
	_, err := RunCapture(ctx, "/", "false")
	if err == nil {
		t.Fatal("expected error from non-zero exit, got nil")
	}
	// Should be an *exec.ExitError; ExitCode helper extracts the code.
	if code, ok := ExitCode(err); !ok {
		t.Fatalf("expected ExitCode to detect *exec.ExitError, got %v", err)
	} else if code == 0 {
		t.Fatalf("expected non-zero exit code, got %d", code)
	}
}

func TestRunCaptureNotFound(t *testing.T) {
	ctx := context.Background()
	_, err := RunCapture(ctx, "/", "this-binary-does-not-exist-xyz123")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestRunCaptureContextTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err := RunCapture(ctx, "/", "sleep", "5")
	if err == nil {
		t.Fatal("expected error from context timeout, got nil")
	}
}
