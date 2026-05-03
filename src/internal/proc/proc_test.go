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
