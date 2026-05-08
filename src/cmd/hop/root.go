package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

const rootLong = `hop — locate, open, and operate on repos from hop.yaml.

Getting started:
  1. Run ` + "`hop config init`" + ` to create a starter hop.yaml.
  2. Edit it to list your repos (each entry: name + git URL + parent dir).
  3. Optional: set $HOP_CONFIG in your shell rc to point at a tracked file
     (git-tracked dotfile, Dropbox, etc.) so it follows you across machines.
  4. For interactive use, install the shim: eval "$(hop shell-init zsh)"

Usage:
  hop                       fzf picker, print selection
  hop <name>                cd into the repo (shell function — needs ` + "`eval \"$(hop shell-init zsh)\"`" + `)
  hop <name> cd             same — explicit verb form
  hop <name> where          echo abs path of matching repo
  hop <name> open           open the repo in an app (delegates to wt's menu; "Open here" cds the parent shell)
  hop <name> -R <cmd>...    shim-only — run <cmd>... with cwd = <name>'s repo dir
  hop -R <name> <cmd>...    binary-direct exec form (also reached via the shim)
  hop <name> <tool>...      shim-only sugar for ` + "`hop -R <name> <tool> ...`" + ` (e.g., ` + "`hop dotfiles cursor`" + `)
  hop clone <name>          git clone the repo if it isn't already on disk
  hop clone <url>           ad-hoc clone: clone the URL, register it in hop.yaml, print landed path
  hop clone --all           clone every repo from hop.yaml that isn't already on disk
  hop clone                 fzf picker, then clone if missing
  hop pull <name>           Run 'git pull' in the named repo
  hop pull <group>          Run 'git pull' in every cloned repo of <group>
  hop pull --all            Run 'git pull' in every cloned repo
  hop push <name>           Run 'git push' in the named repo
  hop push <group>          Run 'git push' in every cloned repo of <group>
  hop push --all            Run 'git push' in every cloned repo
  hop sync <name>           Run 'git pull --rebase' then 'git push' in <name>
  hop sync <group>          Run sync in every cloned repo of <group>
  hop sync --all            Run sync in every cloned repo
  hop ls                    list all repos
  hop shell-init <shell>    emit shell integration (zsh or bash). Use: eval "$(hop shell-init zsh)"
  hop config init           bootstrap a starter hop.yaml
  hop config where          print the resolved hop.yaml path
  hop config print          print the resolved hop.yaml contents to stdout
  hop config scan <dir>     scan a directory for git repos and populate hop.yaml
  hop update                self-update the hop binary via Homebrew
  hop -h | --help           show this help
  hop -v | --version        print version

Notes:
  - ` + "`hop <name>`" + ` and ` + "`hop <name> cd`" + ` require shell integration (a binary can't change
    its parent shell's cwd). Without it, use:  cd "$(hop <name> where)"
  - ` + "`hop <name> open`" + `'s "Open here" choice also requires the shell shim to cd. Other
    menu choices (editors, terminals, file managers) launch their app directly.
  - The repo-first ` + "`hop <name> -R <cmd>...`" + ` and ` + "`hop <name> <tool>...`" + ` forms are shim-only
    rewrites — the binary's ` + "`-R`" + ` parser expects ` + "`hop -R <name> <cmd>...`" + `. Scripts and CI
    that bypass the shim must use the binary-direct form ` + "`hop -R <name> <cmd>...`" + ` (and
    ` + "`hop <name> where`" + ` for path resolution, which the binary handles directly).
  - ` + "`pull`" + `, ` + "`push`" + `, and ` + "`sync`" + ` accept a repo name OR a group name (exact match) as the
    positional, plus ` + "`--all`" + ` for the full registry. ` + "`sync`" + ` is ` + "`pull --rebase`" + ` + ` + "`push`" + ` —
    linear history, no auto-resolve on conflict.
  - On ambiguous or no-match queries, fzf opens prefilled with your query.
  - Grammar: first positional is a repo OR a subcommand (mutually exclusive). When it's
    a repo, second positional is a verb (` + "`cd`, `where`, `open`" + `), ` + "`-R`" + `, or a tool name.
  - Config search order: $HOP_CONFIG, then $XDG_CONFIG_HOME/hop/hop.yaml, then $HOME/.config/hop/hop.yaml.`

// bareNameHint is the exact stderr line printed when the binary is invoked
// with a single positional (the bare-name `hop <name>` shorthand for `hop <name> cd`).
// Both forms are shell-only — the shim runs `_hop_dispatch cd "$1"`; the binary errors.
const bareNameHint = `hop: bare-name dispatch is shell-only. Add 'eval "$(hop shell-init zsh)"' to your zshrc, or use: hop "<name>" where`

// cdHint is the exact stderr line printed when the binary is invoked as
// `hop <name> cd` (explicit cd verb at $2). Same shape as bareNameHint —
// shell-only, error in the binary.
const cdHint = `hop: 'cd' is shell-only. Add 'eval "$(hop shell-init zsh)"' to your zshrc, or use: cd "$(hop "<name>" where)"`

// toolFormHintFmt is the format string for the tool-form error: when the
// binary receives `hop <name> <tool>` (2 args, $2 is neither `where` nor `cd`
// nor `-R`). Tool-form is shim-only — the binary errors with this hint.
// %s is replaced with args[1] (the would-be tool name).
const toolFormHintFmt = `hop: '%s' is not a hop verb (cd, where, open). For tool-form, install the shim: eval "$(hop shell-init zsh)", or use: hop -R "<name>" <tool> [args...]`

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "hop",
		Short:         "locate, open, and operate on repos from hop.yaml.",
		Long:          rootLong,
		SilenceUsage:  true,
		SilenceErrors: true,
		// Repo-verb grammar:
		//   0 args            → bare picker
		//   1 arg             → bare-name dispatch (shell-only) → error in binary
		//   2 args, $2=where  → resolve $1 and print path
		//   2 args, $2=cd     → cd-verb (shell-only) → error in binary
		//   2 args, otherwise → tool-form (shim-only) → error in binary
		Args:              cobra.MaximumNArgs(2),
		ValidArgsFunction: completeRepoNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			switch len(args) {
			case 0:
				return resolveAndPrint(cmd, "")
			case 1:
				return &errExitCode{code: 2, msg: bareNameHint}
			case 2:
				switch args[1] {
				case "where":
					return resolveAndPrint(cmd, args[0])
				case "cd":
					return &errExitCode{code: 2, msg: cdHint}
				case "open":
					return runOpen(cmd, args[0])
				default:
					return &errExitCode{code: 2, msg: fmt.Sprintf(toolFormHintFmt, args[1])}
				}
			}
			// Unreachable: cobra.MaximumNArgs(2) blocks 3+ args before RunE.
			return nil
		},
	}

	cmd.AddCommand(
		newCloneCmd(),
		newPullCmd(),
		newPushCmd(),
		newSyncCmd(),
		newLsCmd(),
		newShellInitCmd(),
		newConfigCmd(),
		newUpdateCmd(),
	)

	return cmd
}
