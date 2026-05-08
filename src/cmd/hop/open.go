package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/sahil87/hop/internal/proc"
)

// openHereNoShimHint is the stderr line printed when the user picks "Open here"
// from wt's menu but the hop shell shim is not loaded (HOP_WRAPPER unset). The
// binary cannot mutate the parent shell's cwd; the path is still printed to
// stdout so the user can compose `cd "$(hop <name> open)"` manually.
const openHereNoShimHint = `hop: 'Open here' requires the shell shim to cd. Add 'eval "$(hop shell-init zsh)"' to your zshrc, or use: cd "$(hop "<name>" open)"`

// runOpen handles `hop <name> open`: resolve the repo, delegate to `wt open`
// inside the repo dir, and re-emit the path on stdout if the user picked
// "Open here" (the shim then cds the parent shell). Other menu choices
// produce no stdout output.
func runOpen(cmd *cobra.Command, name string) error {
	repo, err := resolveOne(cmd, name)
	if err != nil {
		return err
	}

	cdFile, err := os.CreateTemp("", "hop-open-cd-*")
	if err != nil {
		return fmt.Errorf("hop: open: create temp file: %w", err)
	}
	cdPath := cdFile.Name()
	cdFile.Close()
	defer os.Remove(cdPath)

	env := append(os.Environ(), "WT_CD_FILE="+cdPath, "WT_WRAPPER=1")

	code, err := proc.RunForegroundEnv(context.Background(), repo.Path, env, "wt", "open")
	if err != nil {
		if errors.Is(err, proc.ErrNotFound) {
			fmt.Fprintln(cmd.ErrOrStderr(), "hop: wt: not found on PATH.")
			return errSilent
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "hop: open: %v\n", err)
		return errSilent
	}

	contents, readErr := os.ReadFile(cdPath)
	if readErr == nil && len(contents) > 0 {
		fmt.Fprint(cmd.OutOrStdout(), string(contents))
		if os.Getenv("HOP_WRAPPER") != "1" {
			fmt.Fprintln(cmd.ErrOrStderr(), openHereNoShimHint)
		}
	}

	if code != 0 {
		return &errExitCode{code: code}
	}
	return nil
}
