package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/sahil87/hop/internal/proc"
)

const codeMissingHint = "hop code: 'code' command not found. Install VSCode and ensure 'code' is on your PATH."

func newCodeCmd() *cobra.Command {
	return &cobra.Command{
		Use:               "code [<name>]",
		Short:             "open VSCode at the resolved repo",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeRepoNames,
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
			if _, err := proc.Run(ctx, "code", repo.Path); err != nil {
				if errors.Is(err, proc.ErrNotFound) {
					fmt.Fprintln(cmd.ErrOrStderr(), codeMissingHint)
					return errSilent
				}
				return err
			}
			return nil
		},
	}
}
