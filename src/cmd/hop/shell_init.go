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
// Resolution order in the hop() function (5-step ladder, first match wins):
//
//  1. No args                                           → bare picker
//  2. $1 is __complete*                                 → forward to binary (cobra completion)
//  3. $1 is a known subcommand (no `cd`/`where` —       → _hop_dispatch
//     those are now $2 verbs, not $1 subcommands)
//  4. $1 is a flag                                      → forward to binary
//  5. otherwise ($1 is treated as a repo name) — dispatch on $2:
//     $# == 1                                         → _hop_dispatch cd "$1" (bare-name → cd)
//     $# >= 2 and $2 == "cd"                          → _hop_dispatch cd "$1" (explicit cd verb)
//     $# >= 2 and $2 == "where"                       → command hop "$1" where (binary handles)
//     $# >= 2 and $2 == "open"                        → _hop_dispatch open "$1" (open-verb; "Open here" cds the parent shell via stdout capture)
//     $# >= 2 and $2 == "-R"                          → command hop -R "$1" "${@:3}" (canonical exec form)
//     otherwise                                       → command hop -R "$1" "$2" "${@:3}" (tool-form sugar)
//
// The shim does NOT inspect PATH for $1 or $2 — the grammar is "subcommand
// xor repo" in $1, and $2 is either a verb (`cd`, `where`, `open`), `-R`, or a tool name.
// Missing tools surface via the binary's `hop: -R: '<cmd>' not found.` error.
// The shim rewrites the user-facing `hop <name> -R <cmd>...` form to
// `command hop -R <name> <cmd>...` so the binary's extractDashR continues to
// see the existing internal shape.
const posixInit = `# hop shell integration — emit via: eval "$(hop shell-init <shell>)"
# Installs: hop function (with bare-name dispatch + verb dispatch + tool-form), h alias, hi alias, completion.

hop() {
  # HOP_WRAPPER=1 signals to the binary that the shell shim is wrapping the
  # call. Used by the open verb's "Open here" path to suppress the no-shim
  # hint when the shim will handle the parent-shell cd via stdout capture.
  # Scoped to hop() (and _hop_dispatch, which inherits this function-local
  # exported var) so that hi() — which deliberately bypasses the shim — does
  # not inherit it. local -x is supported by both zsh and bash and unsets the
  # var on function return.
  local -x HOP_WRAPPER=1
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
    clone|pull|sync|ls|shell-init|config|update|help|--help|-h|--version|completion)
      _hop_dispatch "$@"
      ;;
    -*)
      command hop "$@"
      ;;
    *)
      # $1 is a repo name (the grammar is "subcommand xor repo" — never a tool,
      # never a verb at $1). Dispatch on $2.
      if [[ $# -eq 1 ]]; then
        # Bare-name dispatch: hop <name> -> cd into the repo (shorthand for hop <name> cd).
        _hop_dispatch cd "$1"
      elif [[ "$2" == "cd" || "$2" == "where" || "$2" == "open" ]]; then
        # Verb dispatch at $2. The verbs are 2-arg only — extra args (e.g.
        # hop <name> cd extra) are forwarded to the binary so cobra's
        # MaximumNArgs(2) rejects with the right error rather than silently
        # dropping args.
        if [[ $# -gt 2 ]]; then
          command hop "$@"
        elif [[ "$2" == "cd" ]]; then
          # hop <name> cd -> cd into the repo (shim handles, parent shell mutates).
          _hop_dispatch cd "$1"
        elif [[ "$2" == "open" ]]; then
          # hop <name> open -> delegate to wt's app menu (binary handles).
          # If user picks "Open here", the binary prints the path on stdout;
          # the shim's _hop_dispatch open arm captures it and cds the parent.
          _hop_dispatch open "$1"
        else
          # hop <name> where -> binary resolves and prints the path.
          command hop "$1" where
        fi
      elif [[ "$2" == "-R" ]]; then
        # Canonical exec form: hop <name> -R <cmd>... → command hop -R <name> <cmd>...
        # The shim flips the user-facing form to the binary's internal shape
        # (extractDashR scans for -R followed by <name> followed by <cmd>...).
        command hop -R "$1" "${@:3}"
      else
        # Tool-form sugar: hop <name> <tool> [args...] → command hop -R <name> <tool> [args...]
        # Missing tools surface via the binary's "hop: -R: '<cmd>' not found." error.
        command hop -R "$1" "$2" "${@:3}"
      fi
      ;;
  esac
}

_hop_dispatch() {
  case "$1" in
    cd)
      # Callers always pass $2 (the repo name) — both the 1-arg bare-name branch
      # and the 2-arg explicit-cd branch in hop() pass "$1" as the single argument.
      local target
      target="$(command hop "$2" where)" || return $?
      cd -- "$target"
      ;;
    open)
      # Caller passes $2 as the repo name (verb-dispatch in hop() pre-validated $#==2).
      # The binary delegates to wt's menu and prints a path on stdout iff the user
      # picked "Open here"; for editor/terminal/etc. choices stdout is empty and we
      # do not cd.
      local target
      target="$(command hop "$2" open)" || return $?
      if [[ -n "$target" ]]; then
        cd -- "$target"
      fi
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
