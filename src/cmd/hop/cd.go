package main

import (
	"github.com/spf13/cobra"
)

const cdHint = `hop: 'cd' is shell-only. Add 'eval "$(hop shell-init zsh)"' to your zshrc, or use: cd "$(hop where "<name>")"`

func newCdCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cd <name>",
		Short: "cd into the resolved repo (shell-only — needs `eval \"$(hop shell-init zsh)\"`)",
		// Accept any args (or none); the binary form just prints the hint.
		RunE: func(cmd *cobra.Command, args []string) error {
			return &errExitCode{code: 2, msg: cdHint}
		},
	}
}
