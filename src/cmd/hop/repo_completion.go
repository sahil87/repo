package main

import (
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
// Names that collide with one of cmd's own subcommands are filtered out:
// cobra dispatches the first token to the subcommand before the bare-form
// resolver ever sees it, so advertising a repo named `clone` (for example)
// from the root would be misleading. For non-root commands this filter is a
// no-op since none of them have subcommands.
//
// On hop.yaml load failure we return ShellCompDirectiveNoFileComp with no
// candidates rather than ShellCompDirectiveError, so a missing config doesn't
// surface a noisy error during tab completion.
func completeRepoNames(cmd *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
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
