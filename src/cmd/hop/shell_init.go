package main

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

// posixInit is the shared portion of `hop shell-init zsh` and `hop shell-init bash`.
// Both shells understand `[[ ]]`, `${@:2}` slicing, and `local`. The completion
// suffix (cobra-generated zsh or bash completion) is appended per-shell at the end.
//
// Resolution order in the hop() function (precedence ladder):
//
//  1. No args                                           → bare picker
//  2. $1 is a flag                                      → forward to binary
//  3. $1 is __complete*                                 → forward to binary (cobra completion)
//  4. $1 is a known subcommand                          → _hop_dispatch (subcommand wins over tool)
//  5. $1 is the only arg                                → _hop_dispatch cd "$1" (bare-name → cd, repo > tool)
//  6. $1 is on PATH AND $2 is non-flag                  → tool-form: command hop -R "$2" "$1" "${@:3}"
//  7. otherwise                                         → forward to binary (let cobra emit error)
//
// Tool-form (rule 6) requires $1 to be an executable on PATH — not a builtin,
// keyword, alias, or function. POSIX `command -v` returns the absolute path
// for a real binary, but only the bare name for builtins/keywords (e.g. `pwd`,
// `if`); the leading-slash check filters those out cleanly in both zsh and
// bash. Repo > tool is enforced by rule 5 running before rule 6: if
// `hop cursor` (1 arg, cursor is a repo), it never reaches the tool-form
// check. With 2+ args (`hop cursor dotfiles`), there's no competing repo
// interpretation, so tool-form wins.
const posixInit = `# hop shell integration — emit via: eval "$(hop shell-init <shell>)"
# Installs: hop function (with bare-name dispatch + tool-form), h alias, hi alias, completion.

hop() {
  if [[ $# -eq 0 ]]; then
    command hop
    return $?
  fi
  case "$1" in
    __complete*)
      # Cobra-internal completion entrypoints (__complete, __completeNoDesc).
      # The cobra-generated completion script invokes "hop __complete ..." to
      # fetch dynamic candidates; without this case the function would route
      # __complete through the bare-name dispatcher and treat it as a repo name.
      command hop "$@"
      ;;
    cd|clone|where|ls|open|shell-init|config|update|help|--help|-h|--version|completion)
      _hop_dispatch "$@"
      ;;
    -*)
      command hop "$@"
      ;;
    *)
      if [[ $# -eq 1 ]]; then
        # Bare-name dispatch: hop <name> → cd into the repo.
        # Repo > tool: with one arg, always treat as repo name even if the
        # token also happens to be a binary on PATH.
        _hop_dispatch cd "$1"
      else
        # Tool-form: hop <tool> <repo> [args...]. Leading-slash check on
        # the command -v output filters builtins/keywords (which return bare
        # names) and missing binaries (empty result). $2 must not be a flag.
        local _hop_tool_path
        _hop_tool_path="$(command -v "$1" 2>/dev/null)"
        if [[ "$2" != -* ]] && [[ "$_hop_tool_path" == /* ]]; then
          # Binary's -R resolves the repo (match or fzf-prompt) and execs
          # $1 with cwd = repo dir.
          command hop -R "$2" "$1" "${@:3}"
        elif [[ "$2" != -* ]] && [[ -n "$_hop_tool_path" ]]; then
          # $1 has a command -v entry but no leading slash — it's a shell
          # builtin, keyword, alias, or function (not a binary on PATH).
          # Tool-form would silently invoke a different thing (or fail) so
          # we stop and explain. Use the shell type builtin to label what
          # kind of name it actually is — shells word it slightly
          # differently (zsh: "shell builtin"/"reserved word"/"alias for"/
          # "shell function"; bash: "shell builtin"/"shell keyword"/
          # "function") but the first line of type output is descriptive
          # enough either way.
          local _hop_kind
          _hop_kind="$(type "$1" 2>&1 | head -1)"
          if [[ -z "$_hop_kind" ]]; then
            _hop_kind="$1 is a shell name (alias, function, builtin, or keyword)"
          fi
          printf "hop: %s — not a binary, so it can't run as a tool inside a repo.\n" "$_hop_kind" >&2
          printf "  - To get the path: hop where %s\n" "$2" >&2
          printf "  - To run a binary by that name: hop -R %s /full/path/to/%s\n" "$2" "$1" >&2
          return 1
        elif [[ "$2" != -* ]] && [[ -z "$_hop_tool_path" ]]; then
          # $1 is not on PATH and not a builtin — likely a tool-form typo.
          # The fall-through to the binary would surface cobra's terse
          # "accepts at most 1 arg(s)" error; this is more helpful.
          printf "hop: '%s' is not a known subcommand or a binary on PATH.\n" "$1" >&2
          printf "  - If you meant tool-form: install '%s' or check the spelling.\n" "$1" >&2
          printf "  - If you meant the path of '%s': hop where %s\n" "$1" "$1" >&2
          return 1
        else
          # $2 is a flag — let the binary surface the error.
          command hop "$@"
        fi
      fi
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
		Short: "emit shell integration (zsh or bash)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return &errExitCode{code: 2, msg: "hop shell-init: missing shell. Supported: zsh, bash"}
			}
			shell := args[0]
			if shell != "zsh" && shell != "bash" {
				return &errExitCode{code: 2, msg: fmt.Sprintf("hop shell-init: unsupported shell '%s'. Supported: zsh, bash", shell)}
			}
			out := cmd.OutOrStdout()
			// io.WriteString avoids vet's printf-args check (posixInit contains
			// shell `printf` directives like %s, which vet would otherwise flag
			// as a missing format argument to fmt.Fprint).
			if _, err := io.WriteString(out, posixInit); err != nil {
				return fmt.Errorf("hop shell-init: write: %w", err)
			}
			// Append cobra-generated completion. rootForCompletion is set in main();
			// in tests that run RunE without main(), it may be nil.
			if rootForCompletion != nil {
				switch shell {
				case "zsh":
					if err := rootForCompletion.GenZshCompletion(out); err != nil {
						return fmt.Errorf("hop shell-init: zsh completion: %w", err)
					}
					// Cobra registers the completion only for `hop`; share it with the
					// `h` and `hi` aliases so tab completion works on those too.
					fmt.Fprint(out, "\ncompdef _hop h hi\n")
				case "bash":
					if err := rootForCompletion.GenBashCompletionV2(out, true); err != nil {
						return fmt.Errorf("hop shell-init: bash completion: %w", err)
					}
					// Bash's `complete` accepts multiple command names; mirror compdef
					// for the h and hi aliases. The cobra-generated function is __start_hop.
					fmt.Fprint(out, "\ncomplete -o default -F __start_hop h hi\n")
				}
			}
			return nil
		},
	}
}
