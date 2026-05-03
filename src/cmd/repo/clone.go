package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/sahil87/repo/internal/proc"
	"github.com/sahil87/repo/internal/repos"
)

const (
	gitMissingHint = "repo: git is not installed."
	cloneTimeout   = 10 * time.Minute
)

func newCloneCmd() *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:   "clone [<name> | --all]",
		Short: "git clone the resolved repo (or all missing repos with --all)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if all {
				return cloneAll(cmd)
			}
			query := ""
			if len(args) == 1 {
				query = args[0]
			}
			repo, err := resolveOne(cmd, query)
			if err != nil {
				return err
			}
			return cloneOne(cmd, *repo)
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "clone every repo from repos.yaml that isn't already on disk")
	return cmd
}

// cloneOne handles a single repo's clone state. Returns nil on success or skip,
// errSilent (after writing stderr) for path-conflict errors.
func cloneOne(cmd *cobra.Command, r repos.Repo) error {
	state, err := cloneState(r.Path)
	if err != nil {
		return err
	}
	switch state {
	case stateAlreadyCloned:
		fmt.Fprintf(cmd.ErrOrStderr(), "skip: already cloned at %s\n", r.Path)
		return nil
	case statePathExistsNotGit:
		fmt.Fprintf(cmd.ErrOrStderr(), "repo clone: %s exists but is not a git repo\n", r.Path)
		return errSilent
	}

	fmt.Fprintf(cmd.ErrOrStderr(), "clone: %s → %s\n", r.URL, r.Path)
	ctx, cancel := context.WithTimeout(context.Background(), cloneTimeout)
	defer cancel()
	if _, err := proc.Run(ctx, "git", "clone", r.URL, r.Path); err != nil {
		if errors.Is(err, proc.ErrNotFound) {
			fmt.Fprintln(cmd.ErrOrStderr(), gitMissingHint)
			return errSilent
		}
		return err
	}
	return nil
}

func cloneAll(cmd *cobra.Command) error {
	rs, err := loadRepos()
	if err != nil {
		return err
	}

	var cloned, skipped, failed int
	for _, r := range rs {
		state, err := cloneState(r.Path)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "repo clone: %s: %v\n", r.Path, err)
			failed++
			continue
		}
		switch state {
		case stateAlreadyCloned:
			fmt.Fprintf(cmd.ErrOrStderr(), "skip: already cloned at %s\n", r.Path)
			skipped++
			continue
		case statePathExistsNotGit:
			fmt.Fprintf(cmd.ErrOrStderr(), "repo clone: %s exists but is not a git repo\n", r.Path)
			failed++
			continue
		}

		fmt.Fprintf(cmd.ErrOrStderr(), "clone: %s → %s\n", r.URL, r.Path)
		ctx, cancel := context.WithTimeout(context.Background(), cloneTimeout)
		_, err = proc.Run(ctx, "git", "clone", r.URL, r.Path)
		cancel()
		if err != nil {
			if errors.Is(err, proc.ErrNotFound) {
				fmt.Fprintln(cmd.ErrOrStderr(), gitMissingHint)
				return errSilent
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "repo clone: %s: %v\n", r.URL, err)
			failed++
			continue
		}
		cloned++
	}

	fmt.Fprintf(cmd.ErrOrStderr(), "summary: cloned=%d skipped=%d failed=%d\n", cloned, skipped, failed)
	if failed > 0 {
		return errSilent
	}
	return nil
}

type cloneStatus int

const (
	stateMissing cloneStatus = iota
	stateAlreadyCloned
	statePathExistsNotGit
)

// cloneState classifies the on-disk state of path: missing → can clone;
// already-cloned → has .git; path-exists-not-git → conflicts.
func cloneState(path string) (cloneStatus, error) {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return stateMissing, nil
		}
		return stateMissing, fmt.Errorf("stat %s: %w", path, err)
	}
	if !info.IsDir() {
		return statePathExistsNotGit, nil
	}
	gitPath := filepath.Join(path, ".git")
	if _, err := os.Stat(gitPath); err == nil {
		return stateAlreadyCloned, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return stateMissing, fmt.Errorf("stat %s: %w", gitPath, err)
	}
	return statePathExistsNotGit, nil
}
