package main

import (
	"github.com/spf13/cobra"
)

const rootLong = `hop — locate, open, and operate on repos from hop.yaml.

Usage:
  hop <name>             echo abs path of matching repo
  hop where <name>       same, explicit form
  hop code <name>        open VSCode at the repo
  hop open <name>        open the repo in the OS file manager (Finder on macOS)
  hop cd <name>          cd into the repo (shell function — needs ` + "`eval \"$(hop shell-init zsh)\"`" + `)
  hop clone <name>       git clone the repo if it isn't already on disk
  hop clone <url>        ad-hoc clone: clone the URL, register it in hop.yaml, print landed path
  hop clone --all        clone every repo from hop.yaml that isn't already on disk
  hop ls                 list all repos
  hop shell-init zsh     emit zsh shell integration (use: eval "$(hop shell-init zsh)")
  hop config init        bootstrap a starter hop.yaml
  hop config where       print the resolved hop.yaml path
  hop -C <name> <cmd>... run <cmd>... with the working directory set to <name>'s repo dir
  hop                    fzf picker, print selection
  hop code               fzf picker, then open VSCode
  hop open               fzf picker, then open in OS file manager
  hop clone              fzf picker, then clone if missing
  hop -h | --help        show this help
  hop -v | --version     print version

Notes:
  - ` + "`hop cd`" + ` requires the shell integration (a binary can't change its parent shell's cwd).
    Without it, use:  cd "$(hop where <name>)"
  - On ambiguous or no-match queries, fzf opens prefilled with your query.
  - Config: $HOP_CONFIG, then $XDG_CONFIG_HOME/hop/hop.yaml, then $HOME/.config/hop/hop.yaml.
    Run ` + "`hop config init`" + ` to bootstrap one.`

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "hop",
		Short:         "locate, open, and operate on repos from hop.yaml.",
		Long:          rootLong,
		SilenceUsage:  true,
		SilenceErrors: true,
		// Bare-form: 0 args → fzf picker; 1 arg → resolve and print path.
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := ""
			if len(args) == 1 {
				query = args[0]
			}
			return resolveAndPrint(cmd, query)
		},
	}

	cmd.AddCommand(
		newWhereCmd(),
		newCodeCmd(),
		newOpenCmd(),
		newCdCmd(),
		newCloneCmd(),
		newLsCmd(),
		newShellInitCmd(),
		newConfigCmd(),
	)

	return cmd
}
