package main

import (
	"github.com/spf13/cobra"
)

const rootLong = `hop — locate, open, and operate on repos from hop.yaml.

Getting started:
  1. Run ` + "`hop config init`" + ` to create a starter hop.yaml.
  2. Edit it to list your repos (each entry: name + git URL + parent dir).
  3. Optional: set $HOP_CONFIG in your shell rc to point at a tracked file
     (git-tracked dotfile, Dropbox, etc.) so it follows you across machines.

Usage:
  hop <name>             echo abs path of matching repo
  hop where <name>       same, explicit form
  hop open <name>        open the repo in the OS file manager (Finder on macOS)
  hop cd <name>          cd into the repo (shell function — needs ` + "`eval \"$(hop shell-init zsh)\"`" + `)
  hop clone <name>       git clone the repo if it isn't already on disk
  hop clone <url>        ad-hoc clone: clone the URL, register it in hop.yaml, print landed path
  hop clone --all        clone every repo from hop.yaml that isn't already on disk
  hop ls                 list all repos
  hop shell-init <shell> emit shell integration (zsh or bash). Use: eval "$(hop shell-init zsh)"
  hop config init        bootstrap a starter hop.yaml
  hop config where       print the resolved hop.yaml path
  hop update             self-update the hop binary via Homebrew
  hop -R <name> <cmd>... run <cmd>... with the working directory set to <name>'s repo dir
  hop <tool> <name>...   shim-only sugar for ` + "`hop -R <name> <tool> ...`" + ` (e.g., ` + "`hop cursor dotfiles`" + `)
  hop                    fzf picker, print selection
  hop open               fzf picker, then open in OS file manager
  hop clone              fzf picker, then clone if missing
  hop -h | --help        show this help
  hop -v | --version     print version

Notes:
  - ` + "`hop cd`" + ` requires the shell integration (a binary can't change its parent shell's cwd).
    Without it, use:  cd "$(hop where <name>)"
  - On ambiguous or no-match queries, fzf opens prefilled with your query.
  - The ` + "`hop <tool> <name>`" + ` form (e.g. ` + "`hop cursor dotfiles`" + `) is implemented in the shell shim;
    invoking the binary directly with that argv won't dispatch as tool-form.
  - Config search order: $HOP_CONFIG, then $XDG_CONFIG_HOME/hop/hop.yaml, then $HOME/.config/hop/hop.yaml.`

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "hop",
		Short:         "locate, open, and operate on repos from hop.yaml.",
		Long:          rootLong,
		SilenceUsage:  true,
		SilenceErrors: true,
		// Bare-form: 0 args → fzf picker; 1 arg → resolve and print path.
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeRepoNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			query := ""
			if len(args) == 1 {
				query = args[0]
			}
			return resolveAndPrint(cmd, query)
		},
	}

	// Register `-R` as a hidden string flag purely so cobra's completion
	// machinery can flag-complete its value slot. In normal execution
	// `extractDashR` (main.go) intercepts `-R` before cobra ever parses
	// argv, so this flag is dormant — it never holds a real value. During
	// `__complete -R <TAB>`, main skips extractDashR and cobra parses
	// `-R`; without the registration, cobra's parser would fail on the
	// unknown shorthand and abort completion. The completion func returns
	// repo names — same source as the bare-form `$1` completion.
	cmd.Flags().StringP("R", "R", "", "")
	_ = cmd.Flags().MarkHidden("R")
	_ = cmd.RegisterFlagCompletionFunc("R", completeRepoNamesForFlag)

	cmd.AddCommand(
		newWhereCmd(),
		newOpenCmd(),
		newCdCmd(),
		newCloneCmd(),
		newLsCmd(),
		newShellInitCmd(),
		newConfigCmd(),
		newUpdateCmd(),
	)

	return cmd
}
