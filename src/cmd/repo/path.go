package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sahil87/repo/internal/config"
	"github.com/sahil87/repo/internal/fzf"
	"github.com/sahil87/repo/internal/proc"
	"github.com/sahil87/repo/internal/repos"
)

// fzfMissingHint is the exact stderr line printed when fzf is required but absent.
const fzfMissingHint = "repo: fzf is not installed. Install it: brew install fzf (macOS) or apt install fzf (Debian)."

// errFzfCancelled signals fzf user cancellation (Esc / Ctrl-C). The handler maps
// this to exit code 130. We use a sentinel so callers can distinguish from other
// errors without parsing exit codes from string output.
var errFzfCancelled = errors.New("fzf cancelled")

// errSilent is a sentinel error returned to cobra when stderr has already been
// written and we just want to exit 1 without re-emitting the error message.
var errSilent = errors.New("silent")

// loadRepos resolves repos.yaml and parses it into a Repos list.
func loadRepos() (repos.Repos, error) {
	path, err := config.Resolve()
	if err != nil {
		return nil, err
	}
	cfg, err := config.Load(path)
	if err != nil {
		return nil, err
	}
	return repos.FromConfig(cfg)
}

// resolveOne resolves a single Repo via the match-or-fzf algorithm from
// docs/specs/cli-surface.md §"Match Resolution Algorithm".
//
// Returns:
//   - exact match (substring narrows to 1) → that repo, no fzf invocation
//   - 0 or 2+ matches → fzf with --query <query> --select-1 (full list piped to stdin)
//   - empty query → fzf with no --query (full picker)
//
// On fzf cancellation, returns errFzfCancelled. The caller maps this to exit 130.
// On fzf-missing (proc.ErrNotFound), writes the install hint to stderr and returns errSilent.
func resolveOne(cmd *cobra.Command, query string) (*repos.Repo, error) {
	rs, err := loadRepos()
	if err != nil {
		return nil, err
	}

	candidates := rs
	if query != "" {
		candidates = rs.MatchOne(query)
		if len(candidates) == 1 {
			return &candidates[0], nil
		}
	}

	// 0 or 2+ matches, OR empty query → fzf picker over the full repo list
	// (fzf does its own filtering via --query).
	pickerLines := make([]string, 0, len(rs))
	for _, r := range rs {
		// tab-separated for --with-nth/--delimiter; fzf displays only column 1
		pickerLines = append(pickerLines, r.Name+"\t"+r.Path+"\t"+r.URL)
	}

	selected, err := fzf.Pick(context.Background(), pickerLines, query)
	if err != nil {
		if errors.Is(err, proc.ErrNotFound) {
			fmt.Fprintln(cmd.ErrOrStderr(), fzfMissingHint)
			return nil, errSilent
		}
		// fzf returns exit 130 on Esc/Ctrl-C → treat as cancellation.
		// Any other failure (non-130 exit, I/O error) surfaces as a real error.
		if code, ok := proc.ExitCode(err); ok && code == 130 {
			return nil, errFzfCancelled
		}
		return nil, fmt.Errorf("repo: fzf failed: %w", err)
	}

	// Match the full selected line back to its source Repo. fzf returns the same
	// tab-delimited triple we piped in (name\tpath\turl); Path is unique per repo,
	// so matching on it disambiguates the case where two repos share a derived name.
	parts := strings.SplitN(selected, "\t", 3)
	if len(parts) < 2 {
		return nil, fmt.Errorf("repo: malformed fzf selection %q", selected)
	}
	chosenPath := parts[1]
	for i := range rs {
		if rs[i].Path == chosenPath {
			return &rs[i], nil
		}
	}
	return nil, fmt.Errorf("repo: selection %q not found in repo list", selected)
}

// resolveAndPrint resolves a single repo via resolveOne and prints its absolute path to stdout.
// Used by `repo <name>` (root bare form) and `repo path <name>`.
func resolveAndPrint(cmd *cobra.Command, query string) error {
	repo, err := resolveOne(cmd, query)
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), repo.Path)
	return nil
}

func newPathCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "path <name>",
		Short: "echo absolute path of matching repo",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return resolveAndPrint(cmd, args[0])
		},
	}
}
