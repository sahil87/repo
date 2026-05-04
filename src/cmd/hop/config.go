package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sahil87/hop/internal/config"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "config helpers (init, where)",
	}
	cmd.AddCommand(newConfigInitCmd(), newConfigWhereCmd())
	return cmd
}

func newConfigInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "bootstrap a starter hop.yaml at the resolved write target",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			target, err := config.ResolveWriteTarget()
			if err != nil {
				return err
			}
			if err := config.WriteStarter(target); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created %s\n", target)
			fmt.Fprintln(cmd.ErrOrStderr(), "Edit the file to add your repos. Tip: set $HOP_CONFIG in your shell rc to point at a version-tracked location (a git-tracked dotfile, Dropbox, etc.) so this config moves with you across machines.")
			return nil
		},
	}
}

func newConfigWhereCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "where",
		Short: "print the resolved hop.yaml path (regardless of file existence)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			target, err := config.ResolveWriteTarget()
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), target)
			return nil
		},
	}
}
