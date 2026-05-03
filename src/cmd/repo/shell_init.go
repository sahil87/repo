package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

const zshInit = `# repo zsh integration — emit via: eval "$(repo shell-init zsh)"
repo() {
  if [[ "$1" == "cd" ]]; then
    if [[ -z "$2" ]]; then
      command repo cd
      return $?
    fi
    local target
    target="$(command repo path "$2")" || return $?
    cd -- "$target"
  else
    command repo "$@"
  fi
}

_repo() {
  _files
}

if (( $+functions[compdef] )); then
  compdef _repo repo
fi
`

func newShellInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "shell-init <shell>",
		Short: "emit shell integration (currently: zsh)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return &errExitCode{code: 2, msg: "repo shell-init: missing shell. Supported: zsh"}
			}
			shell := args[0]
			if shell != "zsh" {
				return &errExitCode{code: 2, msg: fmt.Sprintf("repo shell-init: unsupported shell '%s'. Supported: zsh", shell)}
			}
			fmt.Fprint(cmd.OutOrStdout(), zshInit)
			return nil
		},
	}
}
