package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sahil87/hop/internal/proc"
)

// runOpen handles `hop <name> open`: resolve the repo and exec `wt open <path>`
// with stdio inherited so wt's interactive app menu reaches the user's terminal.
//
// The cd handoff for "Open here" is owned by the shell shim, not the binary:
// the shim creates a temp file, exports WT_CD_FILE pointing at it, invokes
// the binary, and reads the file after wt exits. Hop binary is a passthrough.
//
// Passing the resolved path explicitly (rather than via cmd.Dir) makes wt
// take its "path-first" branch — wt opens the app menu for that directory
// instead of the worktree-selection menu it would show for a main-repo cwd
// with no arg.
func runOpen(cmd *cobra.Command, name string) error {
	repo, err := resolveOne(cmd, name)
	if err != nil {
		return err
	}

	code, err := proc.RunForeground(context.Background(), "", "wt", "open", repo.Path)
	if err != nil {
		if errors.Is(err, proc.ErrNotFound) {
			fmt.Fprintln(cmd.ErrOrStderr(), wtMissingHint)
			return errSilent
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "hop: open: %v\n", err)
		return errSilent
	}

	if code != 0 {
		return &errExitCode{code: code}
	}
	return nil
}
