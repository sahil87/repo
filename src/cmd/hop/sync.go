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

// defaultSyncCommitMessage is the fixed default commit message used when
// `hop sync` auto-commits a dirty tree and the user has not passed
// `-m / --message`. Conventional-Commits friendly (`chore:` prefix) and
// greppable (`git log --grep "via hop"` finds every auto-commit). This
// constant is intentionally fixed in code — Constitution Principle III
// (Convention Over Configuration) — and is NOT configurable via `hop.yaml`
// or environment variables.
const defaultSyncCommitMessage = "chore: sync via hop"

// newSyncCmd builds the `hop sync` subcommand.
//
//	hop sync [<name-or-group>] [--all] [-m / --message <msg>]
//
// Wraps `git pull --rebase` then `git push` over a single repo, every cloned
// repo in a named group, or every cloned repo in the registry. The signature
// and resolution rules mirror `hop pull`.
//
// Per-repo flow:
//   - Clean working tree: identical to today — `git pull --rebase` then
//     `git push`. No auto-stash, no auto-resolve on rebase conflict, no
//     force-push (git's errors surface verbatim — Constitution IV).
//   - Dirty working tree (`git status --porcelain` returns content): hop runs
//     `git add --all` then `git commit -m <msg>` BEFORE the existing rebase +
//     push. The default commit message is `chore: sync via hop`; pass
//     `-m / --message <msg>` to override. User-installed hooks (pre-commit,
//     commit-msg, pre-push) are respected — hop never passes `--no-verify`.
//
// `hop push` is intentionally NOT extended with this auto-commit behavior:
// pushing without rebasing first is the riskier op, so commit-and-push without
// an upstream sync stays opt-in via `git` directly or `hop -R <name> git ...`.
func newSyncCmd() *cobra.Command {
	var (
		all     bool
		message string
	)
	cmd := &cobra.Command{
		Use:               "sync [<name-or-group>] [--all] [-m <msg>]",
		Short:             "Auto-commit dirty trees, then 'git pull --rebase' and 'git push' in a repo, group, or every cloned repo with --all",
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

			op := func(cmd *cobra.Command, r repos.Repo) (ok, gitMissing bool, err error) {
				return syncOne(cmd, r, message)
			}
			if mode == modeSingle {
				return syncSingle(cmd, targets[0], op)
			}
			return syncBatch(cmd, targets, op)
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "run sync (pull --rebase + push) in every cloned repo from hop.yaml")
	cmd.Flags().StringVarP(&message, "message", "m", defaultSyncCommitMessage, "commit message used when auto-committing a dirty tree (no effect on clean trees)")
	return cmd
}

// syncSingle handles single-repo mode (rule 3 substring match → one Repo).
// Skip-not-cloned and any failure (commit, rebase, or push) exits 1; success
// is 0. The op closure is built by newSyncCmd to thread the resolved
// `-m / --message` value into the per-repo flow without changing the runBatch
// contract.
func syncSingle(cmd *cobra.Command, r repos.Repo, op batchOp) error {
	state, err := cloneState(r.Path)
	if err != nil {
		return err
	}
	if state != stateAlreadyCloned {
		fmt.Fprintf(cmd.ErrOrStderr(), "skip: %s not cloned\n", r.Name)
		return errSilent
	}
	ok, gitMissing, _ := op(cmd, r)
	if gitMissing {
		fmt.Fprintln(cmd.ErrOrStderr(), gitMissingHint)
		return errSilent
	}
	if !ok {
		return errSilent
	}
	return nil
}

// syncBatch iterates targets sequentially via runBatch, counting outcomes and
// emitting `summary: synced=N skipped=N failed=N` on stderr. Returns errSilent
// on any failure. On `git` missing, runBatch aborts immediately per spec
// assumption #17. The op closure carries the resolved commit message into
// each per-repo invocation.
func syncBatch(cmd *cobra.Command, targets repos.Repos, op batchOp) error {
	return runBatch(cmd, targets, "sync", "synced", op)
}

// syncOne runs the full per-repo sync flow in r.Path:
//
//  1. `git status --porcelain` — dirty detection (auto-commit helper).
//  2. `git add --all` — stage everything (only when dirty).
//  3. `git commit -m <message>` — auto-commit (only when dirty; respects
//     user-installed hooks; never `--no-verify`).
//  4. `git pull --rebase` — existing rebase step (always).
//  5. `git push` — existing push step (always, when prior steps succeed).
//
// Each git invocation runs under its own `context.WithTimeout` of duration
// `cloneTimeout` — no shared parent budget across the per-repo step sequence
// (spec A11). All invocations route through `internal/proc` with explicit
// argument slices (Constitution I).
//
// On a clean working tree (steps 1's stdout is empty), steps 2 and 3 are
// skipped and the per-repo status line matches today's pre-change baseline
// shape `sync: <name> ✓ <pull-summary> <push-summary>`. On a dirty tree where
// commit + rebase + push all succeed, the line is
// `sync: <name> ✓ committed, <pull-summary>, <push-summary>` — the
// `committed,` token signals to the user that hop made a commit on their
// behalf.
//
// Failure handling: any non-zero step aborts that repo's sync immediately, no
// subsequent step runs. `commitDirtyTree` writes its own
// `sync: <name> ✗ commit failed: <err>` line on commit-step failure. The
// existing rebase-conflict (`sync: <name> ✗ rebase conflict — resolve manually
// with: git -C <path> rebase --continue`) and push-failure
// (`sync: <name> ✗ push failed: <err>`) lines are emitted unchanged.
//
// Returns (ok, gitMissing, err) — same shape as pullOne. syncOne writes its
// own per-repo status line; git's stderr is also forwarded verbatim by
// `proc.RunCapture` / `proc.RunCaptureBoth`.
func syncOne(cmd *cobra.Command, r repos.Repo, message string) (ok, gitMissing bool, err error) {
	committed, gitMissing, commitErr := commitDirtyTree(cmd, r, message)
	if gitMissing {
		return false, true, commitErr
	}
	if commitErr != nil {
		// commitDirtyTree already wrote the per-repo "commit failed" line.
		return false, false, commitErr
	}

	pullCtx, pullCancel := context.WithTimeout(context.Background(), cloneTimeout)
	defer pullCancel()
	pullOut, pullErrOut, pullErr := proc.RunCaptureBoth(pullCtx, r.Path, "git", "pull", "--rebase")
	if pullErr != nil {
		if errors.Is(pullErr, proc.ErrNotFound) {
			return false, true, pullErr
		}
		// Inspect both captured streams (and the error surface) for git's
		// CONFLICT marker — git emits it on stderr, but checking stdout and
		// the error string too keeps the detection robust across versions.
		if mentionsConflict(string(pullOut), string(pullErrOut), pullErr) {
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
	if committed {
		fmt.Fprintf(cmd.ErrOrStderr(), "sync: %s ✓ committed, %s, %s\n", r.Name, pullSummary, pushSummary)
	} else {
		fmt.Fprintf(cmd.ErrOrStderr(), "sync: %s ✓ %s %s\n", r.Name, pullSummary, pushSummary)
	}
	return true, false, nil
}

// commitDirtyTree implements the auto-commit branch of `hop sync`'s per-repo
// flow. The sequence is:
//
//  1. `git status --porcelain` — if stdout is empty the tree is clean and the
//     helper returns (false, false, nil) so the caller proceeds straight to
//     pull/push.
//  2. `git add --all` — stages tracked modifications, deletions, AND
//     untracked files (matches xpush's `git add --all :/` semantic).
//  3. `git commit -m <message>` — respects user-installed hooks
//     (pre-commit, commit-msg, pre-push). hop never passes `--no-verify`.
//
// Each git invocation runs under its own `context.WithTimeout` of duration
// `cloneTimeout` — no shared parent budget across the three steps. All
// invocations route through `internal/proc` with explicit argument slices
// (Constitution I). The working directory is set via `proc.RunCapture`'s
// `dir` argument (`r.Path`), not via `git -C <path>`.
//
// Returns:
//   - committed:  true when a new commit was created (steps 2 and 3 ran and
//     succeeded); false when the tree was clean and no commit was attempted.
//   - gitMissing: true when `git` is not on PATH (caller propagates and
//     emits the install-git hint).
//   - err:        non-nil on a status/add/commit failure. On commit-step
//     failure the helper has already written
//     `sync: <name> ✗ commit failed: <err>` to stderr; the caller must NOT
//     emit a duplicate line and MUST skip the rebase + push steps.
//
// Empty messages (`-m ""`) are passed verbatim to git, which surfaces its own
// "Aborting commit due to empty commit message." error — hop does not
// preempt git's validation. Multi-line messages are passed as a single argv
// element (the proc.RunCapture contract), so subject + body separation flows
// through to git unchanged.
func commitDirtyTree(cmd *cobra.Command, r repos.Repo, message string) (committed, gitMissing bool, err error) {
	statusCtx, statusCancel := context.WithTimeout(context.Background(), cloneTimeout)
	defer statusCancel()
	statusOut, statusErr := proc.RunCapture(statusCtx, r.Path, "git", "status", "--porcelain")
	if statusErr != nil {
		if errors.Is(statusErr, proc.ErrNotFound) {
			return false, true, statusErr
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "sync: %s ✗ commit failed: %v\n", r.Name, statusErr)
		return false, false, statusErr
	}
	if len(strings.TrimSpace(string(statusOut))) == 0 {
		return false, false, nil
	}

	addCtx, addCancel := context.WithTimeout(context.Background(), cloneTimeout)
	defer addCancel()
	_, addStderr, addErr := proc.RunCaptureBoth(addCtx, r.Path, "git", "add", "--all")
	if addErr != nil {
		if errors.Is(addErr, proc.ErrNotFound) {
			return false, true, addErr
		}
		// Prefer git's own last stderr line over the bare exec error string —
		// matches the commit-step branch below and the existing `pull.go`
		// lastNonEmptyLine convention.
		detail := lastNonEmptyLine(string(addStderr))
		if detail == "" {
			detail = addErr.Error()
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "sync: %s ✗ commit failed: %s\n", r.Name, detail)
		return false, false, addErr
	}

	commitCtx, commitCancel := context.WithTimeout(context.Background(), cloneTimeout)
	defer commitCancel()
	_, commitStderr, commitErr := proc.RunCaptureBoth(commitCtx, r.Path, "git", "commit", "-m", message)
	if commitErr != nil {
		if errors.Is(commitErr, proc.ErrNotFound) {
			return false, true, commitErr
		}
		// Prefer git's own last stderr line over the bare exec error string —
		// matches the existing `pull.go` lastNonEmptyLine convention and
		// surfaces hook output (e.g., "gofmt: bad formatting in foo.go").
		detail := lastNonEmptyLine(string(commitStderr))
		if detail == "" {
			detail = commitErr.Error()
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "sync: %s ✗ commit failed: %s\n", r.Name, detail)
		return false, false, commitErr
	}
	return true, false, nil
}

// mentionsConflict reports whether git's captured stdout, stderr, or its error
// surface mentions a rebase CONFLICT marker. Git emits "CONFLICT" lines on
// stderr during a rebase failure; checking all three sources keeps detection
// robust. Used to decide whether to append the hop-specific "resolve manually"
// hint after git's own output.
func mentionsConflict(stdout, stderr string, err error) bool {
	if strings.Contains(stdout, "CONFLICT") {
		return true
	}
	if strings.Contains(stderr, "CONFLICT") {
		return true
	}
	if err != nil && strings.Contains(err.Error(), "CONFLICT") {
		return true
	}
	return false
}
