// Command repo is a CLI for locating, opening, and cloning repos from repos.yaml.
// See docs/specs/cli-surface.md for the canonical contract.
package main

import (
	"errors"
	"fmt"
	"os"
)

// version is the binary version, overridden via -ldflags "-X main.version=..." at build time.
var version = "dev"

func main() {
	rootCmd := newRootCmd()
	rootCmd.Version = version
	if err := rootCmd.Execute(); err != nil {
		os.Exit(translateExit(err))
	}
}

// translateExit maps errors returned from RunE to the spec's exit codes.
// Exit codes per docs/specs/cli-surface.md §"Exit Code Conventions":
//
//	0 success, 1 application error, 2 usage error, 130 user cancelled.
//
// Sentinels:
//   - errFzfCancelled  → 130
//   - errSilent        → 1 (caller already wrote stderr)
//   - errExitCode{...} → custom code (used by `repo cd` to exit 2, `shell-init` for 2, etc.)
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
// (e.g. `repo cd` exits 2, `repo shell-init bash` exits 2).
type errExitCode struct {
	code int
	msg  string
}

func (e *errExitCode) Error() string { return e.msg }
