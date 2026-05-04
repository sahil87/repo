package main

import (
	"github.com/spf13/cobra"
)

// completeRepoNames is a cobra ValidArgsFunction that returns repo names from
// hop.yaml for shell tab-completion. Used by every subcommand whose first
// positional is a repo name (where, cd, code, open, clone) and by the root
// bare-form (hop <name>). The generated shell scripts do prefix-matching
// against toComplete on the candidate set — we just hand back every name.
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
	if len(args) > 0 {
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
