package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sahil87/hop/internal/config"
	"github.com/sahil87/hop/internal/fzf"
	"github.com/sahil87/hop/internal/proc"
	"github.com/sahil87/hop/internal/repos"
)

// fzfMissingHint is the exact stderr line printed when fzf is required but absent.
const fzfMissingHint = "hop: fzf is not installed. Install it: brew install fzf (macOS) or apt install fzf (Debian)."

// errFzfCancelled signals fzf user cancellation (Esc / Ctrl-C). The handler maps
// this to exit code 130. We use a sentinel so callers can distinguish from other
// errors without parsing exit codes from string output.
var errFzfCancelled = errors.New("fzf cancelled")

// errFzfMissing signals that fzf is not on PATH. resolveByName returns this so
// callers can write the install hint to their own stderr (cobra's stderr for
// subcommands, os.Stderr for the -C path).
var errFzfMissing = errors.New("fzf missing")

// errSilent is a sentinel error returned to cobra when stderr has already been
// written and we just want to exit 1 without re-emitting the error message.
var errSilent = errors.New("silent")

// loadRepos resolves hop.yaml and parses it into a Repos list.
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

// resolveByName resolves a single Repo via the match-or-fzf algorithm without
// writing to any stderr. It returns errFzfMissing when fzf is needed but not
// on PATH; the caller is responsible for writing fzfMissingHint to the
// appropriate stderr. Returns errFzfCancelled on Esc/Ctrl-C.
func resolveByName(query string) (*repos.Repo, error) {
	rs, err := loadRepos()
	if err != nil {
		return nil, err
	}

	if query != "" {
		candidates := rs.MatchOne(query)
		if len(candidates) == 1 {
			return &candidates[0], nil
		}
	}

	pickerLines := buildPickerLines(rs)

	selected, err := fzf.Pick(context.Background(), pickerLines, query)
	if err != nil {
		if errors.Is(err, proc.ErrNotFound) {
			return nil, errFzfMissing
		}
		// fzf returns exit 130 on Esc/Ctrl-C → treat as cancellation.
		// Any other failure (non-130 exit, I/O error) surfaces as a real error.
		if code, ok := proc.ExitCode(err); ok && code == 130 {
			return nil, errFzfCancelled
		}
		return nil, fmt.Errorf("hop: fzf failed: %w", err)
	}

	// Match the full selected line back to its source Repo. fzf returns the same
	// tab-delimited triple we piped in (name\tpath\turl); Path is unique per repo,
	// so matching on it disambiguates the case where two repos share a derived name.
	parts := strings.SplitN(selected, "\t", 3)
	if len(parts) < 2 {
		return nil, fmt.Errorf("hop: malformed fzf selection %q", selected)
	}
	chosenPath := parts[1]
	for i := range rs {
		if rs[i].Path == chosenPath {
			return &rs[i], nil
		}
	}
	return nil, fmt.Errorf("hop: selection %q not found in repo list", selected)
}

// buildPickerLines builds the tab-separated lines piped to fzf. When two or
// more repos share a Name, the displayed first column is suffixed with
// " [<group>]" so the user can disambiguate. The path column (used for
// match-back) and URL column are always the second and third columns.
func buildPickerLines(rs repos.Repos) []string {
	nameCount := make(map[string]int, len(rs))
	for _, r := range rs {
		nameCount[r.Name]++
	}
	out := make([]string, 0, len(rs))
	for _, r := range rs {
		display := r.Name
		if nameCount[r.Name] > 1 {
			display = r.Name + " [" + r.Group + "]"
		}
		out = append(out, display+"\t"+r.Path+"\t"+r.URL)
	}
	return out
}

// resolveOne is the cobra-friendly wrapper around resolveByName: on
// errFzfMissing it writes fzfMissingHint to the cobra command's stderr and
// returns errSilent so cobra exits 1 cleanly. Other errors propagate.
func resolveOne(cmd *cobra.Command, query string) (*repos.Repo, error) {
	repo, err := resolveByName(query)
	if err != nil {
		if errors.Is(err, errFzfMissing) {
			fmt.Fprintln(cmd.ErrOrStderr(), fzfMissingHint)
			return nil, errSilent
		}
		return nil, err
	}
	return repo, nil
}

// resolveAndPrint resolves a single repo via resolveOne and prints its absolute path to stdout.
// Used by the bare-form root command and the `hop <name> where` two-arg dispatch.
func resolveAndPrint(cmd *cobra.Command, query string) error {
	repo, err := resolveOne(cmd, query)
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), repo.Path)
	return nil
}

// resolveMode discriminates single-repo vs. batch resolution outcomes for the
// name-or-group resolver used by `hop pull` and `hop sync`. Single-mode emerges
// from a substring repo-name match (rule 3); batch-mode emerges from `--all`
// (rule 1) or an exact group-name match (rule 2). The mode determines exit-code
// policy in the calling subcommand: single-repo failure → exit 1; batch → exit
// 1 only if any repo failed (per spec assumption #19).
type resolveMode int

const (
	modeSingle resolveMode = iota
	modeBatch
)

// resolveTargets is the name-or-group resolver shared by `hop pull` and `hop sync`.
// Resolution rules (first match wins):
//
//  1. all == true  → return every repo from hop.yaml in source order; mode = batch.
//  2. query exactly matches a configured group name (case-sensitive) → return
//     every URL in that group resolved to a Repo; mode = batch. Empty groups
//     (groups with no URLs in hop.yaml) match here and yield an empty batch.
//  3. otherwise → fall through to resolveByName (case-insensitive substring on
//     Name, with fzf for ambiguous/zero matches); mode = single.
//
// Pre-conditions enforced by callers:
//   - all && query != ""   → caller must reject as usage error before calling
//     this function. resolveTargets ignores query when all is true.
//   - !all && query == ""  → caller must reject as usage error before calling.
//
// Returns errFzfMissing/errFzfCancelled (via resolveByName), or any underlying
// config-load error. Callers map errFzfMissing → fzfMissingHint + errSilent.
func resolveTargets(query string, all bool) (repos.Repos, resolveMode, error) {
	// Load the raw config so we can recognize group names that exist in
	// hop.yaml even when their `urls:` list is null/empty (the projected
	// Repos slice loses those because FromConfig only emits per-URL entries).
	path, err := config.Resolve()
	if err != nil {
		return nil, modeSingle, err
	}
	cfg, err := config.Load(path)
	if err != nil {
		return nil, modeSingle, err
	}
	rs, err := repos.FromConfig(cfg)
	if err != nil {
		return nil, modeSingle, err
	}

	if all {
		return rs, modeBatch, nil
	}

	// Rule 2: exact group-name match against the configured group list (not
	// the projected repos), so empty groups still resolve as a batch.
	if hasConfiguredGroup(cfg, query) {
		var batch repos.Repos
		for _, r := range rs {
			if r.Group == query {
				batch = append(batch, r)
			}
		}
		return batch, modeBatch, nil
	}

	// Rule 3: substring repo-name match (with fzf fallback).
	repo, err := resolveByName(query)
	if err != nil {
		return nil, modeSingle, err
	}
	return repos.Repos{*repo}, modeSingle, nil
}

// hasConfiguredGroup reports whether cfg defines a group named exactly query
// (case-sensitive), regardless of whether that group has any URLs. This lets
// `hop pull <empty-group>` / `hop sync <empty-group>` resolve as an empty
// batch instead of falling through to single-repo name matching.
func hasConfiguredGroup(cfg *config.Config, query string) bool {
	if cfg == nil || query == "" {
		return false
	}
	for _, g := range cfg.Groups {
		if g.Name == query {
			return true
		}
	}
	return false
}
