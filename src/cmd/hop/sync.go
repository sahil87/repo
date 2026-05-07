package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sahil87/hop/internal/proc"
	"github.com/sahil87/hop/internal/repos"
)

// newSyncCmd builds the `hop sync` subcommand.
//
//	hop sync [<name-or-group>] [--all]
//
// Wraps `git pull --rebase` then `git push` over a single repo, every cloned
// repo in a named group, or every cloned repo in the registry. The signature,
// flag set, and resolution rules mirror `hop pull`. No auto-stash, no
// auto-resolve on rebase conflict, no force-push — git's errors surface
// verbatim (Constitution IV).
func newSyncCmd() *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:               "sync [<name-or-group>] [--all]",
		Short:             "Run 'git pull --rebase' then 'git push' in a repo, group, or every cloned repo with --all",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeRepoOrGroupNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			query := ""
			if len(args) == 1 {
				query = args[0]
			}
			if all && query != "" {
				return &errExitCode{code: 2, msg: "hop sync: --all conflicts with positional <name-or-group>"}
			}
			if !all && query == "" {
				return &errExitCode{code: 2, msg: "hop sync: missing <name-or-group>. Pass a name, a group, or --all."}
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
				return syncSingle(cmd, targets[0])
			}
			return syncBatch(cmd, targets)
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "run sync (pull --rebase + push) in every cloned repo from hop.yaml")
	return cmd
}

// syncSingle handles single-repo mode (rule 3 substring match → one Repo).
// Skip-not-cloned and any failure (rebase or push) exits 1; success is 0.
func syncSingle(cmd *cobra.Command, r repos.Repo) error {
	state, err := cloneState(r.Path)
	if err != nil {
		return err
	}
	if state != stateAlreadyCloned {
		fmt.Fprintf(cmd.ErrOrStderr(), "skip: %s not cloned\n", r.Name)
		return errSilent
	}
	ok, gitMissing, _ := syncOne(cmd, r)
	if gitMissing {
		fmt.Fprintln(cmd.ErrOrStderr(), gitMissingHint)
		return errSilent
	}
	if !ok {
		return errSilent
	}
	return nil
}

// syncBatch iterates targets sequentially in YAML source order, counts
// outcomes, emits a summary line, and returns errSilent on any failure.
// Aborts immediately on `git` missing per spec assumption #17.
func syncBatch(cmd *cobra.Command, targets repos.Repos) error {
	var synced, skipped, failed int
	for _, r := range targets {
		state, err := cloneState(r.Path)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "sync: %s ✗ %v\n", r.Name, err)
			failed++
			continue
		}
		if state != stateAlreadyCloned {
			fmt.Fprintf(cmd.ErrOrStderr(), "skip: %s not cloned\n", r.Name)
			skipped++
			continue
		}
		ok, gitMissing, _ := syncOne(cmd, r)
		if gitMissing {
			fmt.Fprintln(cmd.ErrOrStderr(), gitMissingHint)
			return errSilent
		}
		if ok {
			synced++
		} else {
			failed++
		}
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "summary: synced=%d skipped=%d failed=%d\n", synced, skipped, failed)
	if failed > 0 {
		return errSilent
	}
	return nil
}

// syncOne runs `git pull --rebase` then `git push` in r.Path. Both invocations
// route through proc.RunCapture with independent 10-minute timeouts. On rebase
// conflict (CONFLICT substring in any error output), the command emits a
// hop-specific resolve-manually hint instead of the verbatim git error and
// does NOT proceed to push. On any other rebase failure, emits the git error
// and skips push. On rebase success, runs push and reports the combined
// result.
//
// Returns (ok, gitMissing, err) — same shape as pullOne. syncOne writes its
// own per-repo status line; git's stderr is forwarded verbatim by
// proc.RunCapture (cmd.Stderr = os.Stderr).
func syncOne(cmd *cobra.Command, r repos.Repo) (ok, gitMissing bool, err error) {
	pullCtx, pullCancel := context.WithTimeout(context.Background(), cloneTimeout)
	defer pullCancel()
	pullOut, pullErr := proc.RunCapture(pullCtx, r.Path, "git", "pull", "--rebase")
	if pullErr != nil {
		if errors.Is(pullErr, proc.ErrNotFound) {
			return false, true, pullErr
		}
		// proc.RunCapture forwards stderr verbatim to the parent's stderr; if
		// the captured stdout (or the error string) mentions CONFLICT, surface
		// the hop-specific resolve-manually hint per spec.
		if mentionsConflict(string(pullOut), pullErr) {
			fmt.Fprintf(cmd.ErrOrStderr(), "sync: %s ✗ rebase conflict — resolve manually with: git -C %s rebase --continue\n", r.Name, r.Path)
		} else {
			fmt.Fprintf(cmd.ErrOrStderr(), "sync: %s ✗ %v\n", r.Name, pullErr)
		}
		return false, false, pullErr
	}

	pushCtx, pushCancel := context.WithTimeout(context.Background(), cloneTimeout)
	defer pushCancel()
	pushOut, pushErr := proc.RunCapture(pushCtx, r.Path, "git", "push")
	if pushErr != nil {
		if errors.Is(pushErr, proc.ErrNotFound) {
			return false, true, pushErr
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "sync: %s ✗ push failed: %v\n", r.Name, pushErr)
		return false, false, pushErr
	}

	pullSummary := lastNonEmptyLine(string(pullOut))
	pushSummary := lastNonEmptyLine(string(pushOut))
	fmt.Fprintf(cmd.ErrOrStderr(), "sync: %s ✓ %s %s\n", r.Name, pullSummary, pushSummary)
	return true, false, nil
}

// mentionsConflict reports whether either git's captured stdout or its error
// surface mentions a rebase CONFLICT marker. Used to decide whether to surface
// the hop-specific "resolve manually" hint vs. the verbatim git error.
func mentionsConflict(stdout string, err error) bool {
	if strings.Contains(stdout, "CONFLICT") {
		return true
	}
	if err != nil && strings.Contains(err.Error(), "CONFLICT") {
		return true
	}
	return false
}
