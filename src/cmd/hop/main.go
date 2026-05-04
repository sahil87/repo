// Command hop is a CLI for locating, opening, and operating on repos from hop.yaml.
// See `hop --help` for the user-facing surface; the canonical contract for this
// binary lives in the active fab change spec (under fab/changes/) until hydrated.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/sahil87/hop/internal/proc"
)

// version is the binary version, overridden via -ldflags "-X main.version=..." at build time.
var version = "dev"

// rootForCompletion holds a reference to the root cobra.Command so shell-init
// can call GenZshCompletion without threading rootCmd through every factory.
var rootForCompletion *cobra.Command

func main() {
	rootCmd := newRootCmd()
	rootCmd.Version = version
	rootForCompletion = rootCmd

	// -C must be handled before cobra parses argv: the post-<name> argv is a
	// child command line, not a hop subcommand. We split os.Args into the hop
	// portion and the child portion before delegating.
	if target, child, ok, err := extractDashC(os.Args); ok {
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(2)
		}
		os.Exit(runDashC(target, child))
	}

	if err := rootCmd.Execute(); err != nil {
		os.Exit(translateExit(err))
	}
}

// extractDashC scans args (typically os.Args, including args[0] = binary name)
// for a `-C` global flag. It returns target (the value), child (everything
// after the target), ok (whether -C was found), and err (set when -C is
// present but malformed: missing value or missing child command).
//
// Accepted forms:
//
//	hop -C <name> <cmd>...
//	hop -C=<name> <cmd>...
//
// args before -C are ignored — -C is treated as a top-level flag with no
// other hop-side flags currently coexisting.
func extractDashC(args []string) (target string, child []string, ok bool, err error) {
	for i := 1; i < len(args); i++ {
		a := args[i]
		if a == "-C" {
			if i+1 >= len(args) {
				return "", nil, true, fmt.Errorf("hop: -C requires a value. Usage: hop -C <name> <cmd>...")
			}
			target = args[i+1]
			rest := args[i+2:]
			if len(rest) == 0 {
				return target, nil, true, fmt.Errorf("hop: -C requires a command to execute. Usage: hop -C <name> <cmd>...")
			}
			return target, rest, true, nil
		}
		if len(a) > 3 && a[:3] == "-C=" {
			target = a[3:]
			if target == "" {
				return "", nil, true, fmt.Errorf("hop: -C requires a value. Usage: hop -C <name> <cmd>...")
			}
			rest := args[i+1:]
			if len(rest) == 0 {
				return target, nil, true, fmt.Errorf("hop: -C requires a command to execute. Usage: hop -C <name> <cmd>...")
			}
			return target, rest, true, nil
		}
	}
	return "", nil, false, nil
}

// runDashC resolves target to a repo directory and execs child[0] with
// child[1:] as argv there. Stdin/stdout/stderr are inherited. The child's
// exit code becomes hop's exit code.
func runDashC(target string, child []string) int {
	repo, err := resolveByName(target)
	if err != nil {
		if errors.Is(err, errFzfMissing) {
			fmt.Fprintln(os.Stderr, fzfMissingHint)
			return 1
		}
		if errors.Is(err, errFzfCancelled) {
			return 130
		}
		fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}

	code, err := proc.RunForeground(context.Background(), repo.Path, child[0], child[1:]...)
	if err != nil {
		if errors.Is(err, proc.ErrNotFound) {
			fmt.Fprintf(os.Stderr, "hop: -C: '%s' not found.\n", child[0])
			return 1
		}
		fmt.Fprintf(os.Stderr, "hop: -C: %v\n", err)
		return 1
	}
	return code
}

// translateExit maps errors returned from RunE to the spec's exit codes.
// Exit codes per docs/specs/cli-surface.md §"Exit Code Conventions":
//
//	0 success, 1 application error, 2 usage error, 130 user cancelled.
//
// Sentinels:
//   - errFzfCancelled  → 130
//   - errSilent        → 1 (caller already wrote stderr)
//   - errExitCode{...} → custom code (used by `hop cd` to exit 2, `shell-init` for 2, etc.)
//
// Default: print the error to stderr and exit 1.
func translateExit(err error) int {
	if err == nil {
		return 0
	}
	var withCode *errExitCode
	if errors.As(err, &withCode) {
		if withCode.msg != "" {
			fmt.Fprintln(os.Stderr, withCode.msg)
		}
		return withCode.code
	}
	if errors.Is(err, errFzfCancelled) {
		return 130
	}
	if errors.Is(err, errSilent) {
		return 1
	}
	fmt.Fprintln(os.Stderr, err)
	return 1
}

// errExitCode carries an explicit exit code plus an optional stderr message.
// Used by subcommands that need to exit with codes other than 0 or 1
// (e.g. `hop cd` exits 2, `hop shell-init bash` exits 2).
type errExitCode struct {
	code int
	msg  string
}

func (e *errExitCode) Error() string { return e.msg }
