package main

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/sahil87/hop/internal/proc"
	"github.com/sahil87/hop/internal/update"
)

func newUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "self-update the hop binary via Homebrew",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			err := update.Run(version, cmd.OutOrStdout(), cmd.ErrOrStderr())
			// internal/update writes its own "brew not found" hint to stderr
			// before returning proc.ErrNotFound. Map it to errSilent so
			// translateExit does not also print the underlying
			// "binary not found on PATH" message.
			if errors.Is(err, proc.ErrNotFound) {
				return errSilent
			}
			return err
		},
	}
}
