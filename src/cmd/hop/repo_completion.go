package main

import (
	"github.com/spf13/cobra"
)

// completeRepoNames is a cobra ValidArgsFunction that returns repo names from
// hop.yaml for shell tab-completion. Used by every subcommand whose first
// positional is a repo name (where, cd, code, open, clone) and by the root
// bare-form (hop <name>). Cobra filters the returned list against toComplete
// automatically — we just hand back the full set.
//
// On hop.yaml load failure we return ShellCompDirectiveNoFileComp with no
// candidates rather than ShellCompDirectiveError, so a missing config doesn't
// surface a noisy error during tab completion.
func completeRepoNames(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	rs, err := loadRepos()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	names := make([]string, 0, len(rs))
	for _, r := range rs {
		names = append(names, r.Name)
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}

// completeCloneArg is the ValidArgsFunction for `hop clone`. The clone
// positional accepts either a repo name OR a git URL, so we suppress
// completion once the user has typed something that looks URL-shaped — repo
// names won't help them past that point.
func completeCloneArg(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if looksLikeURL(toComplete) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return completeRepoNames(cmd, args, toComplete)
}
