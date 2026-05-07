// Package scan walks a directory tree and discovers git repositories.
//
// It is a UI-free discovery layer: it knows how to recognize git working trees
// (vs worktrees, submodules, bare repos, no-remote repos), how to follow
// symlinks safely (with (dev, inode) loop dedup), and how to invoke `git` via
// an injectable runner so callers (and tests) can substitute a fake. Group
// assignment, slugification, conflict resolution, and YAML rendering live in
// the CLI / yamled layers — scan stays minimal.
//
// All `git` invocations route through Options.GitRunner, which defaults to
// internal/proc.RunCapture (Constitution Principle I — no direct os/exec
// outside internal/proc). Each `git` invocation uses a 5-second
// context.WithTimeout per spec § "Git invocation contract".
package scan

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/sahil87/hop/internal/proc"
)

// Skip reasons. The set is closed — the CLI summary counts on these.
// Slugify-failure is NOT a Skip reason; the CLI surfaces it as a stderr
// `skip:` line per spec assumption #28.
const (
	ReasonNoRemote  = "no remote"
	ReasonBareRepo  = "bare repo"
	ReasonWorktree  = "worktree"
	ReasonSubmodule = "submodule"
)

// gitTimeout is the per-invocation deadline applied to every `git` subprocess.
// Spec § "Git invocation contract": "Each `git` invocation SHALL use a
// dedicated `context.Context` with a 5-second timeout."
const gitTimeout = 5 * time.Second

// originRemote is the conventional remote name preferred when present (matches
// `git`'s own conventions per spec assumption #4).
const originRemote = "origin"

// Found is one discovered repository.
type Found struct {
	Path string // canonical (EvalSymlinks-resolved) path to the working tree
	URL  string // remote URL (origin if present, else first remote)
}

// Skip is one discovered candidate intentionally excluded from Found.
type Skip struct {
	Path   string
	Reason string // one of: "no remote", "bare repo", "worktree", "submodule"
}

// GitRunner is the injectable seam for `git` invocations. Defaults to
// internal/proc.RunCapture-bound (set by the CLI layer).
type GitRunner func(ctx context.Context, dir string, args ...string) ([]byte, error)

// Options configures Walk.
type Options struct {
	Depth     int       // 0 means "root only"; CLI passes 3 by default
	GitRunner GitRunner // injectable; defaults to defaultGitRunner (proc.RunCapture-bound)
}

// defaultGitRunner binds proc.RunCapture for `git`. Used when
// Options.GitRunner is nil.
func defaultGitRunner(ctx context.Context, dir string, args ...string) ([]byte, error) {
	return proc.RunCapture(ctx, dir, "git", args...)
}

// stackEntry is one DFS frame: a path to visit and its depth.
//
// Submodule detection relies solely on the no-descent invariant (spec
// assumption #17 explicitly permits this): once Walk classifies a dir as
// classNormalRepo it never enqueues the dir's children, so a nested `.git`
// inside a parent repo is unreachable through DFS. There's no need to track
// whether an ancestor was a repo — the invariant is enforced by control flow.
type stackEntry struct {
	path  string
	depth int
}

// Walk performs a stack-based DFS from root, classifying directories per the
// rules in spec § "Repo classification rules". Returns the discovered repos in
// DFS discovery order, the skipped candidates, and any walk-halting error
// (e.g., `git` missing on PATH after lazy detection).
//
// Submodule detection relies on the no-descent invariant: once a directory
// classifies as classNormalRepo, Walk never enqueues its children, so a
// nested `.git` inside a parent repo is unreachable through DFS. Per spec
// assumption #17, this is the chosen approach. ReasonSubmodule is therefore
// part of the public Skip enum (preserving forward compatibility) but is
// never emitted by the current implementation.
func Walk(ctx context.Context, root string, opts Options) ([]Found, []Skip, error) {
	runner := opts.GitRunner
	if runner == nil {
		runner = defaultGitRunner
	}

	// (dev, inode) visited set for symlink-loop dedup. Keyed by canonical
	// directory inode so the same repo reached via two paths is registered
	// exactly once.
	visited := make(map[devIno]struct{})

	var (
		found []Found
		skips []Skip
	)

	stack := []stackEntry{{path: root, depth: 0}}

	for len(stack) > 0 {
		// Pop.
		top := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		if top.depth > opts.Depth {
			continue
		}

		// Resolve through symlinks via os.Stat (Lstat is informational only —
		// we follow links per spec § "Symlinks and loop detection"). If Stat
		// fails or the entry isn't a directory, skip silently.
		info, err := os.Stat(top.path)
		if err != nil || !info.IsDir() {
			continue
		}

		// (dev, inode) dedup — silent loop suppression.
		key, ok := inodeKey(info)
		if ok {
			if _, seen := visited[key]; seen {
				continue
			}
			visited[key] = struct{}{}
		}

		// Canonicalize: Found.Path is the EvalSymlinks resolution.
		canonical, err := filepath.EvalSymlinks(top.path)
		if err != nil {
			// Treat unresolvable as not a repo and skip silently — the entry
			// must have vanished between Stat and EvalSymlinks.
			continue
		}

		// Classify.
		switch classifyDir(canonical) {
		case classWorktree:
			skips = append(skips, Skip{Path: canonical, Reason: ReasonWorktree})
			continue
		case classBareRepo:
			skips = append(skips, Skip{Path: canonical, Reason: ReasonBareRepo})
			continue
		case classNormalRepo:
			f, skipReason, err := inspectRepo(ctx, runner, canonical)
			if err != nil {
				return found, skips, err
			}
			if skipReason != "" {
				skips = append(skips, Skip{Path: canonical, Reason: skipReason})
				continue
			}
			found = append(found, f)
			continue
		case classPlainDir:
			// Recurse: enqueue immediate subdirectories at depth+1, in
			// reverse order so DFS visits them in lexical order (matches the
			// natural reading of a sorted directory listing — required for
			// reproducible test fixtures and discovery-order ties).
			if top.depth+1 > opts.Depth {
				continue
			}
			children, err := listSubdirs(canonical)
			if err != nil {
				// Permission errors etc. — skip silently. The walk continues
				// against siblings.
				continue
			}
			// Reverse-order push so the first child is on top of the stack.
			for i := len(children) - 1; i >= 0; i-- {
				stack = append(stack, stackEntry{
					path:  filepath.Join(canonical, children[i]),
					depth: top.depth + 1,
				})
			}
		}
	}

	return found, skips, nil
}

// dirClass enumerates the classifier outputs in spec § "Repo classification
// rules". Order matters: classifyDir applies the rules first-match-wins.
//
// Note: classSubmodule is omitted intentionally. Submodule skip is enforced
// implicitly by the no-descent invariant in Walk — once a normal repo is
// classified at any frame, its children are never enqueued, so nested `.git`
// dirs inside a parent repo are unreachable. Per spec assumption #17 this is
// the chosen approach.
type dirClass int

const (
	classPlainDir dirClass = iota
	classWorktree
	classNormalRepo
	classBareRepo
)

// classifyDir applies the spec § "Repo classification rules" first-match-wins
// against the filesystem at dir. The submodule rule is enforced by the
// no-descent invariant in Walk (see dirClass note above), not here.
func classifyDir(dir string) dirClass {
	gitPath := filepath.Join(dir, ".git")
	gitInfo, err := os.Lstat(gitPath)
	switch {
	case err == nil && gitInfo.Mode().IsRegular():
		// Rule 1: worktree (`.git` is a regular file containing `gitdir:`).
		return classWorktree
	case err == nil && gitInfo.IsDir():
		// Rule 2: normal repo (`.git` is a directory).
		return classNormalRepo
	case err != nil && !errors.Is(err, os.ErrNotExist):
		// Some other stat error (permissions, etc.) — treat as plain dir so
		// the walker doesn't blow up; siblings continue.
		return classPlainDir
	}

	// Rule 3: bare repo (HEAD + config + objects/ at top level, no .git).
	if isBareRepo(dir) {
		return classBareRepo
	}

	// Rule 4: plain directory — recurse.
	return classPlainDir
}

// isBareRepo applies the stat-based heuristic per spec § "Repo classification
// rules" #4: HEAD (regular), config (regular), objects/ (directory) at top
// level. Caller has already ruled out `.git` presence (rule 1/2/3).
func isBareRepo(dir string) bool {
	if !isRegularFile(filepath.Join(dir, "HEAD")) {
		return false
	}
	if !isRegularFile(filepath.Join(dir, "config")) {
		return false
	}
	if !isDirectory(filepath.Join(dir, "objects")) {
		return false
	}
	return true
}

func isRegularFile(p string) bool {
	info, err := os.Lstat(p)
	if err != nil {
		return false
	}
	return info.Mode().IsRegular()
}

func isDirectory(p string) bool {
	info, err := os.Stat(p)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// inspectRepo runs `git remote` then `git remote get-url <selected>` for a
// confirmed normal repo. Returns (Found, "", nil) on success; ("", reason,
// nil) when the repo has no remote (a soft skip); or a non-nil error when
// `git` is missing or another fatal condition halts the walk.
func inspectRepo(ctx context.Context, runner GitRunner, dir string) (Found, string, error) {
	remotes, err := runGit(ctx, runner, dir, "remote")
	if err != nil {
		return Found{}, "", err
	}
	selected := selectRemote(remotes)
	if selected == "" {
		return Found{}, ReasonNoRemote, nil
	}

	urlBytes, err := runGit(ctx, runner, dir, "remote", "get-url", selected)
	if err != nil {
		return Found{}, "", err
	}
	url := strings.TrimSpace(string(urlBytes))
	if url == "" {
		// Defensive: `git remote get-url` shouldn't return empty for a listed
		// remote, but don't add an empty URL to Found. Treat as no-remote.
		return Found{}, ReasonNoRemote, nil
	}
	return Found{Path: dir, URL: url}, "", nil
}

// selectRemote applies the per-spec assumption #4 selection rule: prefer
// "origin" if listed; else the first non-empty line. Returns "" when there
// are no remotes.
func selectRemote(remotes []byte) string {
	var first string
	for _, line := range strings.Split(string(remotes), "\n") {
		name := strings.TrimSpace(line)
		if name == "" {
			continue
		}
		if name == originRemote {
			return originRemote
		}
		if first == "" {
			first = name
		}
	}
	return first
}

// runGit invokes the GitRunner under a 5-second timeout per spec § "Git
// invocation contract". Wraps proc.ErrNotFound into a descriptive error so
// the CLI can match it via errors.Is and emit `hop: git is not installed.`.
func runGit(ctx context.Context, runner GitRunner, dir string, args ...string) ([]byte, error) {
	cctx, cancel := context.WithTimeout(ctx, gitTimeout)
	defer cancel()
	out, err := runner(cctx, dir, args...)
	if err != nil {
		return out, fmt.Errorf("scan: git %s in %s: %w", strings.Join(args, " "), dir, err)
	}
	return out, nil
}

// listSubdirs returns the immediate subdirectory names of dir, sorted
// lexically. Hidden entries (leading dot) are included so we don't miss
// dot-prefixed repo trees a user might intentionally place there. (`.git` is
// classified at the dir level — there's no special-case here.)
func listSubdirs(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() {
			// Note: ReadDir surfaces symlinks as their underlying entry — a
			// symlink to a directory has Type() == os.ModeSymlink (not
			// IsDir). Include those by re-statting so symlinks are followed
			// per spec § "Symlinks and loop detection".
			if e.Type()&os.ModeSymlink == 0 {
				continue
			}
			info, err := os.Stat(filepath.Join(dir, e.Name()))
			if err != nil || !info.IsDir() {
				continue
			}
		}
		out = append(out, e.Name())
	}
	sort.Strings(out)
	return out, nil
}

// devIno is the (device, inode) tuple used for the visited set. On
// linux/darwin both fields are populated by syscall.Stat_t.
type devIno struct {
	dev uint64
	ino uint64
}

// inodeKey extracts (dev, ino) from a FileInfo. Returns (zero, false) if the
// platform-specific stat is unavailable — falling back to no dedup is
// acceptable; the caller treats that as "always visit."
func inodeKey(info os.FileInfo) (devIno, bool) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return devIno{}, false
	}
	return devIno{dev: uint64(st.Dev), ino: uint64(st.Ino)}, true
}
