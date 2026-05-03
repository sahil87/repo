package main

import (
	"github.com/spf13/cobra"
)

const rootLong = `repo — locate, open, or list repos from repos.yaml.

Usage:
  repo <name>            echo abs path of matching repo
  repo path <name>       same, explicit form
  repo code <name>       open VSCode at the repo
  repo open <name>       open the repo in the OS file manager (Finder on macOS)
  repo cd <name>         cd into the repo (shell function — needs ` + "`eval \"$(repo shell-init zsh)\"`" + `)
  repo clone <name>      git clone the repo if it isn't already on disk
  repo clone --all       clone every repo from repos.yaml that isn't already on disk
  repo ls                list all repos
  repo shell-init zsh    emit zsh shell integration (use: eval "$(repo shell-init zsh)")
  repo config init       bootstrap a starter repos.yaml
  repo config path       print the resolved repos.yaml path
  repo                   fzf picker, print selection
  repo code              fzf picker, then open VSCode
  repo open              fzf picker, then open in OS file manager
  repo clone             fzf picker, then clone if missing
  repo -h | --help       show this help
  repo -v | --version    print version

Notes:
  - ` + "`repo cd`" + ` requires the shell integration (a binary can't change its parent shell's cwd).
    Without it, use:  cd "$(repo <name>)"
  - On ambiguous or no-match queries, fzf opens prefilled with your query.
  - Config: $REPOS_YAML, then $XDG_CONFIG_HOME/repo/repos.yaml.
    Run ` + "`repo config init`" + ` to bootstrap one.`

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "repo",
		Short:         "locate, open, or list repos from repos.yaml.",
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
		newPathCmd(),
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
