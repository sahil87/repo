package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sahil87/hop/internal/proc"
	"github.com/sahil87/hop/internal/repos"
)

// Per-worktree glyphs used in `hop ls --trees` rows. Single-rune so width
// stays predictable across terminals.
const (
	wtDirtyGlyph    = "*"
	wtUnpushedGlyph = "↑"
)

func newLsCmd() *cobra.Command {
	var trees bool
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "list all repos as aligned name/path columns",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			rs, err := loadRepos()
			if err != nil {
				return err
			}
			if len(rs) == 0 {
				return nil
			}
			padWidth := longestName(rs) + 2
			if trees {
				return runLsTrees(cmd, rs, padWidth)
			}
			return runLsPlain(cmd.OutOrStdout(), rs, padWidth)
		},
	}
	cmd.Flags().BoolVar(&trees, "trees", false, "list worktrees per repo via `wt list --json`")
	return cmd
}

// longestName returns the byte length of the longest Name in rs. Used for
// the left-aligned name column shared between `hop ls` and `hop ls --trees`.
func longestName(rs repos.Repos) int {
	max := 0
	for _, r := range rs {
		if n := len(r.Name); n > max {
			max = n
		}
	}
	return max
}

// padName left-aligns name to width totalWidth (= longestName + 2). The two
// trailing spaces preserve the existing `hop ls` separator convention.
func padName(name string, totalWidth int) string {
	if pad := totalWidth - len(name); pad > 0 {
		return name + strings.Repeat(" ", pad)
	}
	return name + "  "
}

func runLsPlain(out io.Writer, rs repos.Repos, padWidth int) error {
	for _, r := range rs {
		fmt.Fprintln(out, padName(r.Name, padWidth)+r.Path)
	}
	return nil
}

// runLsTrees fans `wt list --json` across each cloned repo in source order
// and emits a per-row summary. Non-cloned repos surface `(not cloned)`
// without invoking wt. Per-repo wt-list failures degrade gracefully as
// inline `(wt list failed: <err>)` rows — the table is never aborted by a
// single corrupt `.git`.
//
// The exception is the missing-`wt` case: if the FIRST `wt list` invocation
// returns proc.ErrNotFound, we fail fast with the standard `wtMissingHint`
// (matches `hop <name> open`'s wording) and exit 1. Subsequent invocations
// within the same run can't hit ErrNotFound — we abort on the first.
func runLsTrees(cmd *cobra.Command, rs repos.Repos, padWidth int) error {
	out := cmd.OutOrStdout()
	for _, r := range rs {
		state, err := cloneState(r.Path)
		if err != nil {
			return err
		}
		if state != stateAlreadyCloned {
			fmt.Fprintln(out, padName(r.Name, padWidth)+"(not cloned)")
			continue
		}
		entries, err := listWorktrees(context.Background(), r.Path)
		if err != nil {
			if errors.Is(err, proc.ErrNotFound) {
				fmt.Fprintln(cmd.ErrOrStderr(), wtMissingHint)
				return errSilent
			}
			fmt.Fprintln(out, padName(r.Name, padWidth)+fmt.Sprintf("(wt list failed: %v)", err))
			continue
		}
		fmt.Fprintln(out, padName(r.Name, padWidth)+formatTreesRow(entries))
	}
	return nil
}

// formatTreesRow renders the per-repo summary `<N> tree(s)  (<wt-list>)`.
// Each wt is `name[*][↑N]`: `*` if dirty, `↑N` if Unpushed > 0.
func formatTreesRow(entries []WtEntry) string {
	noun := "trees"
	if len(entries) == 1 {
		noun = "tree"
	}
	parts := make([]string, len(entries))
	for i, e := range entries {
		flags := ""
		if e.Dirty {
			flags += wtDirtyGlyph
		}
		if e.Unpushed > 0 {
			flags += fmt.Sprintf("%s%d", wtUnpushedGlyph, e.Unpushed)
		}
		parts[i] = e.Name + flags
	}
	return fmt.Sprintf("%d %s  (%s)", len(entries), noun, strings.Join(parts, ", "))
}
