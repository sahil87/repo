package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// zshInit is the static portion of `hop shell-init zsh` output. The cobra-
// generated zsh completion script is appended at runtime so the embedded
// completion always matches the current subcommand surface.
const zshInit = `# hop zsh integration — emit via: eval "$(hop shell-init zsh)"
# Installs: hop function (with bare-name dispatch), h alias, hi alias, completion.

hop() {
  if [[ $# -eq 0 ]]; then
    command hop
    return $?
  fi
  case "$1" in
    __complete*)
      # Cobra-internal completion entrypoints (__complete, __completeNoDesc).
      # The cobra-generated _hop completion script invokes "hop __complete ..."
      # to fetch dynamic candidates; without this case the function would route
      # __complete through the bare-name dispatcher and treat it as a repo name.
      command hop "$@"
      ;;
    cd|clone|where|ls|code|open|shell-init|config|update|--help|-h|--version|completion)
      _hop_dispatch "$@"
      ;;
    -*)
      command hop "$@"
      ;;
    *)
      _hop_dispatch cd "$1"
      ;;
  esac
}

_hop_dispatch() {
  case "$1" in
    cd)
      if [[ -z "$2" ]]; then
        command hop cd
        return $?
      fi
      local target
      target="$(command hop where "$2")" || return $?
      cd -- "$target"
      ;;
    clone)
      # Detect URL form (contains :// or @host:path)
      if [[ "$2" == *"://"* ]] || [[ "$2" == *"@"*":"* ]]; then
        local target
        target="$(command hop clone "${@:2}")" || return $?
        if [[ -n "$target" ]]; then
          cd -- "$target"
        fi
      else
        command hop "$@"
      fi
      ;;
    *)
      command hop "$@"
      ;;
  esac
}

h() { hop "$@"; }
hi() { command hop "$@"; }

`

func newShellInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "shell-init <shell>",
		Short: "emit shell integration (currently: zsh)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return &errExitCode{code: 2, msg: "hop shell-init: missing shell. Supported: zsh"}
			}
			shell := args[0]
			if shell != "zsh" {
				return &errExitCode{code: 2, msg: fmt.Sprintf("hop shell-init: unsupported shell '%s'. Supported: zsh", shell)}
			}
			out := cmd.OutOrStdout()
			fmt.Fprint(out, zshInit)
			// Append cobra-generated zsh completion. rootForCompletion is set
			// in main(); in tests that run RunE without main(), it may be nil.
			if rootForCompletion != nil {
				if err := rootForCompletion.GenZshCompletion(out); err != nil {
					return fmt.Errorf("hop shell-init: zsh completion: %w", err)
				}
			}
			// Cobra registers the completion only for `hop`; share it with the
			// `h` and `hi` aliases so tab completion works on those too. The
			// `_hop` completion script handles the words[1] == "h" or "hi"
			// case correctly because the `h` function calls the hop shell
			// function (which routes __complete* to `command hop`), and `hi`
			// invokes the binary directly.
			fmt.Fprint(out, "\ncompdef _hop h hi\n")
			return nil
		},
	}
}
