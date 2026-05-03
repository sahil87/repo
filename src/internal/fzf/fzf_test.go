package fzf

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/sahil87/repo/internal/proc"
)

func TestBuildArgsWithQuery(t *testing.T) {
	got := buildArgs("foo")
	want := []string{
		"--query", "foo",
		"--select-1",
		"--height", "40%",
		"--reverse",
		"--with-nth", "1",
		"--delimiter", "\t",
	}
	if !equalStringSlice(got, want) {
		t.Fatalf("buildArgs('foo'):\n  got:  %v\n  want: %v", got, want)
	}
}

func TestBuildArgsEmptyQuery(t *testing.T) {
	got := buildArgs("")
	want := []string{
		"--select-1",
		"--height", "40%",
		"--reverse",
		"--with-nth", "1",
		"--delimiter", "\t",
	}
	if !equalStringSlice(got, want) {
		t.Fatalf("buildArgs(''):\n  got:  %v\n  want: %v", got, want)
	}
}

func TestPickPropagatesNotFound(t *testing.T) {
	original := runInteractive
	defer func() { runInteractive = original }()
	runInteractive = func(ctx context.Context, stdin io.Reader, name string, args ...string) (string, error) {
		return "", proc.ErrNotFound
	}

	_, err := Pick(context.Background(), []string{"a\tb"}, "")
	if !errors.Is(err, proc.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestPickPipesAndArgs(t *testing.T) {
	original := runInteractive
	defer func() { runInteractive = original }()

	var capturedArgs []string
	var capturedStdin string
	runInteractive = func(ctx context.Context, stdin io.Reader, name string, args ...string) (string, error) {
		capturedArgs = args
		b, _ := io.ReadAll(stdin)
		capturedStdin = string(b)
		return "selected\tline\n", nil
	}

	got, err := Pick(context.Background(), []string{"a\tb", "c\td"}, "q")
	if err != nil {
		t.Fatalf("Pick: %v", err)
	}
	if got != "selected\tline" {
		t.Fatalf("trimmed output: %q", got)
	}
	if !contains(capturedArgs, "--query") || !contains(capturedArgs, "q") {
		t.Fatalf("expected --query q in args, got %v", capturedArgs)
	}
	if capturedStdin != "a\tb\nc\td" {
		t.Fatalf("stdin: got %q", capturedStdin)
	}
}

func contains(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}

func equalStringSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Suppress unused import warning if any
var _ = strings.Join
