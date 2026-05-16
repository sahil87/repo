package main

import (
	"context"
	"strings"

	"github.com/spf13/cobra"
)

// completeRepoNames is a cobra ValidArgsFunction that returns repo names from
// hop.yaml for shell tab-completion. Used by `clone` (whose first positional
// is a repo name, via completeCloneArg) and by the root command's $1 slot
// (the repo-verb grammar — `hop <name>` and `hop <name> <verb>`). At $2 the
// root command surfaces the recognized verbs (cd, where, open); other commands
// using this helper (e.g. `clone`) suppress completion past $1. The generated
// shell scripts do prefix-matching against toComplete on the candidate set —
// for the default (non-root or no-unique-match) path we just hand back every
// name; for the root-command eager-fire path we hand back the unique repo plus
// its `<repo>/<wt>` candidates (see "Eager pre-slash branch" below).
//
// Worktree-prefix branch: when toComplete contains "/" the token is split on
// the first "/", the LHS resolves to a unique configured repo, and
// `wt list --json` provides candidates as `<repo>/<wt>` strings (the full
// token the user is composing — cobra prefix-matches against toComplete in
// the generated shell scripts, so bare wt names would mis-replace the LHS).
// Any failure in the worktree path (LHS unknown or ambiguous, repo uncloned,
// wt missing, JSON error) returns nil candidates silently — completion is a
// UX surface; stderr noise during TAB is a bug.
//
// Eager pre-slash branch: applies ONLY to the root command (the $1 slot of
// the repo-verb grammar). Non-root subcommands that delegate here (e.g.
// `clone` via completeCloneArg) accept a bare repo name or URL — surfacing
// `<repo>/<wt>` candidates there would be misleading, so the eager expansion
// is gated to `isRoot`. When on the root AND toComplete does NOT contain "/"
// AND rs.MatchOne(toComplete) returns exactly one non-collided repo that is
// cloned with >=2 worktrees, the candidate list is expanded to
// [<repo>, <repo>/<wt1>, ...] with directive
// ShellCompDirectiveNoFileComp|ShellCompDirectiveNoSpace so the user sees
// the worktree menu without typing "/". The unique-match guard is the cost
// gate — ambiguous prefixes never run `wt list --json` across N repos. Every
// failure mode silently falls back to today's candidate list and directive.
//
// Names that collide with one of cmd's own subcommands are filtered out:
// cobra dispatches the first token to the subcommand before the bare-form
// resolver ever sees it, so advertising a repo named `clone` (for example)
// from the root would be misleading. For non-root commands this filter is a
// no-op since none of them have subcommands.
//
// On hop.yaml load failure we return ShellCompDirectiveNoFileComp with no
// candidates rather than ShellCompDirectiveError, so a missing config doesn't
// surface a noisy error during tab completion.
func completeRepoNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// The verb-position behavior (cd/where/open at $2) only applies to the
	// root command — other commands that delegate here (e.g. `clone` via
	// completeCloneArg) accept at most one positional and must not surface
	// the repo-verb candidates as a second argument.
	isRoot := cmd.Parent() == nil
	if len(args) == 1 {
		if isRoot {
			return []string{"cd", "where", "open"}, cobra.ShellCompDirectiveNoFileComp
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	if len(args) > 1 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	if strings.Contains(toComplete, "/") {
		return completeWorktreeCandidates(toComplete)
	}
	rs, err := loadRepos()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	subNames := make(map[string]struct{}, len(cmd.Commands()))
	for _, sub := range cmd.Commands() {
		if sub.IsAvailableCommand() {
			subNames[sub.Name()] = struct{}{}
		}
	}
	names := make([]string, 0, len(rs))
	for _, r := range rs {
		if _, collides := subNames[r.Name]; collides {
			continue
		}
		names = append(names, r.Name)
	}
	// Eager pre-slash branch: root command only. When toComplete uniquely
	// identifies one non-collided repo that is cloned and has >=2 worktrees,
	// expand the candidate list to include `<repo>/<wt>` forms so the user
	// sees the worktree menu without typing `/`. NoSpace keeps the menu
	// interactive after a bare-repo pick. Non-root subcommands (e.g. `clone`)
	// accept only a repo name or URL — surfacing worktree-suffixed candidates
	// there would be misleading. Every failure mode silently falls back to
	// the default candidate list.
	if isRoot {
		if matches := rs.MatchOne(toComplete); len(matches) == 1 {
			repo := matches[0]
			if _, collides := subNames[repo.Name]; !collides {
				if state, err := cloneState(repo.Path); err == nil && state == stateAlreadyCloned {
					if entries, err := listWorktrees(context.Background(), repo.Path); err == nil && len(entries) >= 2 {
						out := make([]string, 0, 1+len(entries))
						out = append(out, repo.Name)
						for _, e := range entries {
							out = append(out, repo.Name+"/"+e.Name)
						}
						return out, cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace
					}
				}
			}
		}
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}

// completeWorktreeCandidates handles the `<repo>/<partial>` branch of
// completeRepoNames. Split toComplete on the first "/"; if the LHS resolves
// to exactly one configured repo (case-insensitive substring, matching
// MatchOne's tolerance) AND that repo is cloned, invoke `wt list --json` and
// return each worktree name prefixed with the LHS verbatim so cobra's
// prefix-match-on-toComplete works.
//
// Any failure mode (no LHS match, ambiguous LHS, repo uncloned, wt missing,
// JSON error) returns nil candidates silently — never writes to stderr.
func completeWorktreeCandidates(toComplete string) ([]string, cobra.ShellCompDirective) {
	idx := strings.Index(toComplete, "/")
	lhs := toComplete[:idx]
	if lhs == "" {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	rs, err := loadRepos()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	matches := rs.MatchOne(lhs)
	if len(matches) != 1 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	repo := matches[0]
	state, err := cloneState(repo.Path)
	if err != nil || state != stateAlreadyCloned {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	entries, err := listWorktrees(context.Background(), repo.Path)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, lhs+"/"+e.Name)
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}

// completeCloneArg is the ValidArgsFunction for `hop clone`. The clone
// positional accepts either a repo name OR a git URL, so we suppress
// completion once the user has typed something that looks URL-shaped — repo
// names won't help them past that point. We also suppress when --all is set,
// since `clone --all` ignores any positional argument.
func completeCloneArg(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if looksLikeURL(toComplete) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	if all := cmd.Flag("all"); all != nil && all.Changed {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return completeRepoNames(cmd, args, toComplete)
}

// completeRepoOrGroupNames is the ValidArgsFunction for `hop pull` and `hop sync`.
// The positional accepts a repo name OR a group name (resolved via
// resolveTargets, see resolve.go), so completion advertises both. When --all is
// set the positional is rejected by RunE, so we suppress completion. When the
// user has already typed a positional we also suppress. Names that appear as
// both a repo and a group are de-duplicated so each candidate surfaces once.
func completeRepoOrGroupNames(cmd *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	if all := cmd.Flag("all"); all != nil && all.Changed {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	rs, err := loadRepos()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	seen := make(map[string]struct{})
	var out []string
	for _, r := range rs {
		if _, ok := seen[r.Group]; !ok {
			seen[r.Group] = struct{}{}
			out = append(out, r.Group)
		}
	}
	for _, r := range rs {
		if _, ok := seen[r.Name]; ok {
			continue
		}
		seen[r.Name] = struct{}{}
		out = append(out, r.Name)
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}
