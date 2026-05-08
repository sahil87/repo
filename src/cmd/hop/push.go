package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sahil87/hop/internal/proc"
	"github.com/sahil87/hop/internal/repos"
)

// newPushCmd builds the `hop push` subcommand.
//
//	hop push [<name-or-group>] [--all]
//
// Wraps `git push` over a single repo (substring match), every cloned repo in
// a named group (exact group match), or every cloned repo in the registry
// (--all). Mirrors `hop pull` exactly — same signature, flag set, resolution
// rules, and exit-code policy.
func newPushCmd() *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:               "push [<name-or-group>] [--all]",
		Short:             "Run 'git push' in a repo, group, or every cloned repo with --all",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeRepoOrGroupNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			query := ""
			if len(args) == 1 {
				query = args[0]
			}
			if all && query != "" {
				return &errExitCode{code: 2, msg: "hop push: --all conflicts with positional <name-or-group>"}
			}
			if !all && query == "" {
				return &errExitCode{code: 2, msg: "hop push: missing <name-or-group>. Pass a name, a group, or --all."}
			}

			targets, mode, err := resolveTargets(query, all)
			if err != nil {
				if errors.Is(err, errFzfMissing) {
					fmt.Fprintln(cmd.ErrOrStderr(), fzfMissingHint)
					return errSilent
				}
				return err
			}

			if mode == modeSingle {
				return pushSingle(cmd, targets[0])
			}
			return pushBatch(cmd, targets)
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "run 'git push' in every cloned repo from hop.yaml")
	return cmd
}

// pushSingle handles single-repo mode (rule 3 substring match → one Repo).
// Skip-not-cloned and push failures both exit 1; success is exit 0.
func pushSingle(cmd *cobra.Command, r repos.Repo) error {
	state, err := cloneState(r.Path)
	if err != nil {
		return err
	}
	if state != stateAlreadyCloned {
		fmt.Fprintf(cmd.ErrOrStderr(), "skip: %s not cloned\n", r.Name)
		return errSilent
	}
	ok, gitMissing, _ := pushOne(cmd, r)
	if gitMissing {
		fmt.Fprintln(cmd.ErrOrStderr(), gitMissingHint)
		return errSilent
	}
	if !ok {
		return errSilent
	}
	return nil
}

// pushBatch iterates targets sequentially via runBatch, counting outcomes and
// emitting `summary: pushed=N skipped=N failed=N` on stderr. Returns errSilent
// when any push failed. On `git` missing, runBatch aborts immediately (no
// further repos attempted, no summary line emitted) — same behavior as pull.
func pushBatch(cmd *cobra.Command, targets repos.Repos) error {
	return runBatch(cmd, targets, "push", "pushed", pushOne)
}

// pushOne runs `git push` in r.Path via proc.RunCapture with a 10-minute
// timeout. The returned tuple is:
//   - ok: true on a successful push
//   - gitMissing: true when git is not on PATH (caller emits the hint and
//     aborts the batch)
//   - err: the underlying error (informational; pushOne already wrote a status
//     line to stderr)
//
// pushOne writes its own per-repo status line ("push: <name> ✓ ..." or
// "push: <name> ✗ ...") to cmd's stderr. Git's own stderr is forwarded
// verbatim by proc.RunCapture (which sets cmd.Stderr = os.Stderr).
func pushOne(cmd *cobra.Command, r repos.Repo) (ok, gitMissing bool, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), cloneTimeout)
	defer cancel()
	out, err := proc.RunCapture(ctx, r.Path, "git", "push")
	if err != nil {
		if errors.Is(err, proc.ErrNotFound) {
			return false, true, err
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "push: %s ✗ %v\n", r.Name, err)
		return false, false, err
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "push: %s ✓ %s\n", r.Name, lastNonEmptyLine(string(out)))
	return true, false, nil
}
