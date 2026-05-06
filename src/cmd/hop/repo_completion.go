package main

import (
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

// completeRepoNames is a cobra ValidArgsFunction that returns repo names from
// hop.yaml for shell tab-completion. Used by every subcommand whose first
// positional is a repo name (where, cd, open, clone) and by the root
// bare-form (hop <name>). The generated shell scripts do prefix-matching
// against toComplete on the candidate set — we just hand back every name.
//
// Names that collide with one of cmd's own subcommands are filtered out:
// cobra dispatches the first token to the subcommand before the bare-form
// resolver ever sees it, so advertising a repo named `clone` (for example)
// from the root would be misleading. For non-root commands this filter is a
// no-op since none of them have subcommands.
//
// When invoked with len(args) > 0, we additionally complete the `$2` slot of
// `hop <tool> <TAB>` (tool-form sugar, where <tool> resolves on PATH and is
// not a hop subcommand). For all other len(args) > 0 calls we return no
// candidates: positions 3+ belong to the child command's argv, not hop's.
// `hop -R <TAB>` is handled separately by completeRepoNamesForFlag (cobra's
// flag-value completion machinery) since cobra consumes the `-R` token
// before ValidArgsFunction is invoked.
//
// `hop -R <name> <TAB>` (third position of -R form): cobra consumes
// `-R <name>` as a flag pair, so this function is invoked with args=[]
// and would otherwise look like a bare `hop <TAB>`. We detect this case via
// the root cmd's `R` flag's Changed bit and suppress candidates — the
// remaining argv is the child command's, not hop's.
//
// On hop.yaml load failure we return ShellCompDirectiveNoFileComp with no
// candidates rather than ShellCompDirectiveError, so a missing config doesn't
// surface a noisy error during tab completion.
func completeRepoNames(cmd *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 && !shouldCompleteRepoForSecondArg(cmd, args) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	if rFlag := cmd.Flag("R"); rFlag != nil && rFlag.Changed {
		// We're past `-R <name>` — at the child argv position. Cobra
		// already absorbed `-R name` from args, so args looks empty,
		// but the flag's Changed bit reveals the true context.
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

// completeRepoNamesForFlag is the cobra flag-value completion func registered
// against the root's `-R` flag. It returns repo names for the slot
// immediately after `-R` (e.g., `hop -R <TAB>`).
//
// Unlike completeRepoNames, it does NOT filter against subcommand names: the
// `-R` form runs an arbitrary child command in the named repo's directory,
// so a repo whose name happens to match a hop subcommand (e.g., `clone`) is
// still a valid `-R` target — cobra has already routed via the flag, not via
// the subcommand dispatcher.
//
// On hop.yaml load failure we return ShellCompDirectiveNoFileComp with no
// candidates, mirroring completeRepoNames' silent-failure policy.
func completeRepoNamesForFlag(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
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

// shouldCompleteRepoForSecondArg reports whether the current completion
// context is the `$2` slot of `hop <tool> <name>` (tool-form sugar dispatched
// by the shim). <tool> must be a binary on PATH and not a known hop
// subcommand. This mirrors shim rules 4 (subcommand check) and 6
// (leading-slash check on `command -v`) in shell_init.go::posixInit so that
// completion only suggests repo names when the shim will actually route the
// call as tool-form.
//
// `hop -R <TAB>` is NOT handled here — cobra's flag parser consumes `-R` and
// its value before ValidArgsFunction runs, so completion for the `-R` value
// slot is wired via cmd.RegisterFlagCompletionFunc("R", completeRepoNamesForFlag)
// in newRootCmd instead.
//
// Returns false for any other shape: len(args) != 1 (positions 3+ belong to
// the child argv), known subcommands, and binaries that aren't on PATH.
//
// filepath.IsAbs on exec.LookPath's result is defensive — exec.LookPath
// returns absolute paths on POSIX systems — but documents intent and mirrors
// the shim's leading-slash filter that excludes builtins/keywords/aliases.
func shouldCompleteRepoForSecondArg(cmd *cobra.Command, args []string) bool {
	if len(args) != 1 {
		return false
	}
	first := args[0]
	for _, sub := range cmd.Commands() {
		if sub.IsAvailableCommand() && sub.Name() == first {
			return false
		}
	}
	p, err := exec.LookPath(first)
	if err != nil {
		return false
	}
	return filepath.IsAbs(p)
}
