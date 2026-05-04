package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newLsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "list all repos as aligned name/path columns",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			rs, err := loadRepos()
			if err != nil {
				return err
			}
			if len(rs) == 0 {
				return nil
			}
			// Compute column width from the longest name.
			maxName := 0
			for _, r := range rs {
				if n := len(r.Name); n > maxName {
					maxName = n
				}
			}
			out := cmd.OutOrStdout()
			for _, r := range rs {
				fmt.Fprintln(out, r.Name+strings.Repeat(" ", maxName-len(r.Name)+2)+r.Path)
			}
			return nil
		},
	}
}
