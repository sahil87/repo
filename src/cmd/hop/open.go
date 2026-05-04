package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/sahil87/hop/internal/platform"
	"github.com/sahil87/hop/internal/proc"
)

func newOpenCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "open [<name>]",
		Short: "open the resolved repo in the OS file manager",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := ""
			if len(args) == 1 {
				query = args[0]
			}
			repo, err := resolveOne(cmd, query)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := platform.Open(ctx, repo.Path); err != nil {
				if errors.Is(err, proc.ErrNotFound) {
					fmt.Fprintf(cmd.ErrOrStderr(), "hop open: '%s' not found.\n", platform.OpenTool())
					return errSilent
				}
				return err
			}
			return nil
		},
	}
}
