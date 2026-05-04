package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/sahil87/hop/internal/config"
	"github.com/sahil87/hop/internal/proc"
	"github.com/sahil87/hop/internal/repos"
	"github.com/sahil87/hop/internal/yamled"
)

const (
	gitMissingHint = "hop: git is not installed."
	cloneTimeout   = 10 * time.Minute
)

func newCloneCmd() *cobra.Command {
	var (
		all          bool
		group        string
		noAdd        bool
		noCD         bool
		nameOverride string
	)
	cmd := &cobra.Command{
		Use:   "clone [<name> | <url> | --all]",
		Short: "git clone the resolved repo, an ad-hoc URL, or all missing repos with --all",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if all {
				return cloneAll(cmd)
			}
			if len(args) == 1 && looksLikeURL(args[0]) {
				return cloneURL(cmd, args[0], group, noAdd, noCD, nameOverride)
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
	cmd.Flags().BoolVar(&all, "all", false, "clone every repo from hop.yaml that isn't already on disk")
	cmd.Flags().StringVar(&group, "group", "default", "target group for ad-hoc URL clone (only used with <url>)")
	cmd.Flags().BoolVar(&noAdd, "no-add", false, "skip the hop.yaml write-back (only used with <url>)")
	cmd.Flags().BoolVar(&noCD, "no-cd", false, "suppress the printed path so the shell shim does not cd (only used with <url>)")
	cmd.Flags().StringVar(&nameOverride, "name", "", "override the URL-derived name for the on-disk path (only used with <url>)")
	return cmd
}

// looksLikeURL returns true when arg looks like a git URL: contains "://" or
// contains "@" AND ":" (the SSH `git@host:owner/repo.git` form).
func looksLikeURL(arg string) bool {
	if strings.Contains(arg, "://") {
		return true
	}
	if strings.Contains(arg, "@") && strings.Contains(arg, ":") {
		return true
	}
	return false
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
		fmt.Fprintf(cmd.ErrOrStderr(), "hop clone: %s exists but is not a git repo\n", r.Path)
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

// cloneURL implements `hop clone <url>` with auto-registration. Resolves the
// target group, computes the on-disk path, clones (or skips), appends the URL
// to hop.yaml (unless --no-add), and prints the path on stdout (unless --no-cd).
func cloneURL(cmd *cobra.Command, url, group string, noAdd, noCD bool, nameOverride string) error {
	configPath, err := config.Resolve()
	if err != nil {
		return err
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}

	g := findGroup(cfg, group)
	if g == nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "hop: no '%s' group in %s. Pass --group <existing-group> or add '%s:' to your config.\n", group, configPath, group)
		return errSilent
	}

	name := nameOverride
	if name == "" {
		name = repos.DeriveName(url)
	}

	path := resolveAdHocPath(cfg, g, url, name)

	state, err := cloneState(path)
	if err != nil {
		return err
	}

	switch state {
	case statePathExistsNotGit:
		fmt.Fprintf(cmd.ErrOrStderr(), "hop clone: %s exists but is not a git repo\n", path)
		return errSilent

	case stateAlreadyCloned:
		fmt.Fprintf(cmd.ErrOrStderr(), "skip: already cloned at %s\n", path)
		if !noAdd {
			if err := registerURL(cmd, configPath, g, group, url); err != nil {
				return err
			}
		}
		if !noCD {
			fmt.Fprintln(cmd.OutOrStdout(), path)
		}
		return nil

	case stateMissing:
		fmt.Fprintf(cmd.ErrOrStderr(), "clone: %s → %s\n", url, path)
		ctx, cancel := context.WithTimeout(context.Background(), cloneTimeout)
		defer cancel()
		if _, err := proc.Run(ctx, "git", "clone", url, path); err != nil {
			if errors.Is(err, proc.ErrNotFound) {
				fmt.Fprintln(cmd.ErrOrStderr(), gitMissingHint)
				return errSilent
			}
			return err
		}
		if !noAdd {
			if err := registerURL(cmd, configPath, g, group, url); err != nil {
				return err
			}
		}
		if !noCD {
			fmt.Fprintln(cmd.OutOrStdout(), path)
		}
		return nil
	}

	return nil
}

// findGroup returns a pointer to the named group in cfg, or nil if absent.
func findGroup(cfg *config.Config, name string) *config.Group {
	for i := range cfg.Groups {
		if cfg.Groups[i].Name == name {
			return &cfg.Groups[i]
		}
	}
	return nil
}

// resolveAdHocPath computes the on-disk path for a URL+name landing in group g
// using cfg.CodeRoot. Mirrors repos.FromConfig's per-URL resolution.
func resolveAdHocPath(cfg *config.Config, g *config.Group, url, name string) string {
	if g.Dir != "" {
		return filepath.Join(repos.ExpandDir(g.Dir, cfg.CodeRoot), name)
	}
	// Mirror repos.FromConfig: when cfg.CodeRoot is empty (e.g. user wrote
	// `code_root: ""`), fall back to "~" so we never produce a relative path
	// that would land the clone in $PWD.
	codeRoot := repos.ExpandDir(cfg.CodeRoot, "")
	if codeRoot == "" {
		codeRoot = repos.ExpandDir("~", "")
	}
	org := repos.DeriveOrg(url)
	if org == "" {
		return filepath.Join(codeRoot, name)
	}
	return filepath.Join(codeRoot, org, name)
}

// registerURL appends url to g's URL list in the config file. If the URL is
// already in g, emits a "skip: already registered" stderr line and returns nil
// (no YAML write). Other yamled errors result in errSilent after writing stderr.
func registerURL(cmd *cobra.Command, configPath string, g *config.Group, groupName, url string) error {
	for _, existing := range g.URLs {
		if existing == url {
			fmt.Fprintf(cmd.ErrOrStderr(), "skip: %s already registered in '%s'\n", url, groupName)
			return nil
		}
	}
	if err := yamled.AppendURL(configPath, groupName, url); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "hop clone: registering to %s failed: %v\n", configPath, err)
		return errSilent
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
			fmt.Fprintf(cmd.ErrOrStderr(), "hop clone: %s: %v\n", r.Path, err)
			failed++
			continue
		}
		switch state {
		case stateAlreadyCloned:
			fmt.Fprintf(cmd.ErrOrStderr(), "skip: already cloned at %s\n", r.Path)
			skipped++
			continue
		case statePathExistsNotGit:
			fmt.Fprintf(cmd.ErrOrStderr(), "hop clone: %s exists but is not a git repo\n", r.Path)
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
			fmt.Fprintf(cmd.ErrOrStderr(), "hop clone: %s: %v\n", r.URL, err)
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
