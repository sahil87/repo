package main

import (
	"github.com/spf13/cobra"

	"github.com/sahil87/hop/internal/update"
)

func newUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "self-update the hop binary via Homebrew",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return update.Run(version, cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
	}
}
