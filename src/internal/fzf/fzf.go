// Package fzf wraps the fzf binary for interactive selection.
// All exec calls go through internal/proc per Constitution Principle I.
package fzf

import (
	"context"
	"strings"

	"github.com/sahil87/hop/internal/proc"
)

// runInteractive is the seam for tests to inject a fake invocation.
var runInteractive = proc.RunInteractive

// Pick pipes lines (joined with \n) to fzf via stdin and returns the trimmed
// selected line. Flags follow docs/specs/cli-surface.md §"Match Resolution Algorithm":
//
//	fzf --query <q> --select-1 --height 40% --reverse --with-nth 1 --delimiter '\t'
//
// --query is omitted when query is empty.
//
// If fzf is not installed, returns an error matched by errors.Is(err, proc.ErrNotFound)
// so callers can produce the install-hint message.
func Pick(ctx context.Context, lines []string, query string) (string, error) {
	args := buildArgs(query)
	stdin := strings.NewReader(strings.Join(lines, "\n"))
	out, err := runInteractive(ctx, stdin, "fzf", args...)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(out, "\n"), nil
}

// buildArgs returns the fzf argv (excluding the binary name).
// Exposed for unit tests verifying flag composition.
func buildArgs(query string) []string {
	args := []string{}
	if query != "" {
		args = append(args, "--query", query)
	}
	args = append(args,
		"--select-1",
		"--height", "40%",
		"--reverse",
		"--with-nth", "1",
		"--delimiter", "\t",
	)
	return args
}
