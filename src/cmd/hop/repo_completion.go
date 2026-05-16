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
// we just hand back every name.
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
