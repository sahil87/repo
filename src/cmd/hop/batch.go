package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sahil87/hop/internal/repos"
)

// batchOp is the per-repo operation invoked by runBatch. It MUST write its own
// per-repo status line and return:
//   - ok          — true on success, false on failure
//   - gitMissing  — true when git is not on PATH (caller aborts the batch)
type batchOp func(cmd *cobra.Command, r repos.Repo) (ok, gitMissing bool, err error)

// runBatch is the shared batch-iteration helper for `hop pull` and `hop sync`.
// It iterates targets sequentially in YAML source order and applies the same
// rules across both commands: skip-not-cloned counts as a skip; per-repo
// `cloneState` errors and per-repo op failures count as failed; a missing
// `git` binary aborts immediately (no further repos attempted, no summary
// emitted). On a non-zero failed count it returns errSilent; otherwise nil.
//
// verb is used in stderr formatting:
//   - per-repo error line:  "{verb}: {name} ✗ {err}"
//   - summary suffix:       "summary: {summaryLabel}={success} skipped={S} failed={F}"
//
// op writes its own per-repo status line on success or per-op failure;
// runBatch does NOT emit per-repo lines for those outcomes (only for
// cloneState errors and the summary).
func runBatch(cmd *cobra.Command, targets repos.Repos, verb, summaryLabel string, op batchOp) error {
	var success, skipped, failed int
	for _, r := range targets {
		state, err := cloneState(r.Path)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "%s: %s ✗ %v\n", verb, r.Name, err)
			failed++
			continue
		}
		if state != stateAlreadyCloned {
			fmt.Fprintf(cmd.ErrOrStderr(), "skip: %s not cloned\n", r.Name)
			skipped++
			continue
		}
		ok, gitMissing, _ := op(cmd, r)
		if gitMissing {
			fmt.Fprintln(cmd.ErrOrStderr(), gitMissingHint)
			return errSilent
		}
		if ok {
			success++
		} else {
			failed++
		}
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "summary: %s=%d skipped=%d failed=%d\n", summaryLabel, success, skipped, failed)
	if failed > 0 {
		return errSilent
	}
	return nil
}
