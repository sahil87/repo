package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/sahil87/hop/internal/config"
	"github.com/sahil87/hop/internal/proc"
	"github.com/sahil87/hop/internal/repos"
	"github.com/sahil87/hop/internal/scan"
	"github.com/sahil87/hop/internal/yamled"
)

// scanCmdName is the CLI prefix used in stderr messages (matches the spec's
// "hop config scan: ..." wording).
const scanCmdName = "hop config scan"

// minScanDepth is the smallest valid value for --depth (per spec assumption #24).
const minScanDepth = 1

// inventedSuffixStart is the smallest integer suffix used when slug collides
// with an existing group whose dir does not match (per spec § "Conflict
// resolution").
const inventedSuffixStart = 2

// scanGroupNameRe is the schema regex from yaml-schema.md / config.go. The
// slugify output MUST conform.
var scanGroupNameRe = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)

// runConfigScan is the top-level RunE wired by newConfigScanCmd. It validates
// args, resolves hop.yaml, runs the walk, builds the merge plan, and dispatches
// to print or write mode. Returns errSilent on user-visible failures (cobra
// translates to exit 1 / 2 via translateExit + errExitCode).
func runConfigScan(cmd *cobra.Command, userArg string, depth int, write bool) error {
	stderr := cmd.ErrOrStderr()

	// 1. Validate --depth (spec assumption #24).
	if depth < minScanDepth {
		fmt.Fprintf(stderr, "%s: --depth must be >= %d.\n", scanCmdName, minScanDepth)
		return &errExitCode{code: 2}
	}

	// 2. Validate <dir>: filepath.Clean → EvalSymlinks → os.Stat (directory).
	canonicalDir, ok := validateScanDir(userArg, stderr)
	if !ok {
		return &errExitCode{code: 2}
	}

	// 3. Resolve hop.yaml. On miss, emit the scan-specific two-line message
	//    pointing at ResolveWriteTarget (per spec § "hop.yaml precondition").
	configPath, err := config.Resolve()
	if err != nil {
		bootstrap, werr := config.ResolveWriteTarget()
		if werr != nil {
			bootstrap = "$XDG_CONFIG_HOME/hop/hop.yaml"
		}
		fmt.Fprintf(stderr, "%s: no hop.yaml found at %s.\nRun 'hop config init' first, then re-run scan.\n", scanCmdName, bootstrap)
		return errSilent
	}

	// 4. Load existing config (used for the convention check + dedup).
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", scanCmdName, err)
		return errSilent
	}

	// 5. Walk.
	found, skips, err := scan.Walk(context.Background(), canonicalDir, scan.Options{
		Depth:     depth,
		GitRunner: gitRunner,
	})
	if err != nil {
		// Lazy git-missing check: only surfaces when Walk had to invoke git.
		if errors.Is(err, proc.ErrNotFound) {
			fmt.Fprintln(stderr, gitMissingHint)
			return errSilent
		}
		fmt.Fprintf(stderr, "%s: %v\n", scanCmdName, err)
		return errSilent
	}

	// 6. Build the merge plan. Slugify failures emit `skip:` lines and are
	//    counted as a generic skip; they do NOT block other repos.
	plan, planSummary := buildScanPlan(cfg, found, stderr)

	// 7. Render or write.
	if write {
		if err := yamled.MergeScan(configPath, plan); err != nil {
			fmt.Fprintf(stderr, "%s: write %s: %v\n", scanCmdName, configPath, err)
			return errSilent
		}
	} else {
		bytes, err := yamled.RenderScan(configPath, plan)
		if err != nil {
			fmt.Fprintf(stderr, "%s: render: %v\n", scanCmdName, err)
			return errSilent
		}
		header := scanHeaderComment(userArg, configPath)
		fmt.Fprint(cmd.OutOrStdout(), header)
		_, _ = cmd.OutOrStdout().Write(bytes)
	}

	// 8. Stderr summary block (last, after stdout in print mode).
	emitScanSummary(stderr, userArg, depth, found, skips, planSummary, configPath, write)
	return nil
}

// gitRunner is the production scan.GitRunner, bound to proc.RunCapture.
// Defined as a package var so tests can override if needed; production code
// uses the default. Spec § "Git invocation contract" Constitution I.
var gitRunner scan.GitRunner = func(ctx context.Context, dir string, args ...string) ([]byte, error) {
	return proc.RunCapture(ctx, dir, "git", args...)
}

// validateScanDir applies the order from spec § "Argument validation":
// filepath.Clean → filepath.EvalSymlinks → os.Stat (must be directory). On
// any failure, emits the not-a-directory message (with userArg verbatim) and
// returns ok=false so the caller can exit 2.
func validateScanDir(userArg string, stderr io.Writer) (canonical string, ok bool) {
	cleaned := filepath.Clean(userArg)
	resolved, err := filepath.EvalSymlinks(cleaned)
	if err != nil {
		fmt.Fprintf(stderr, "%s: '%s' is not a directory.\n", scanCmdName, userArg)
		return "", false
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.IsDir() {
		fmt.Fprintf(stderr, "%s: '%s' is not a directory.\n", scanCmdName, userArg)
		return "", false
	}
	return resolved, true
}

// scanHeaderComment returns the two-line print-mode header per spec § "Print
// mode header" / assumption #23. Date is UTC (spec assumption #30).
func scanHeaderComment(userArg, configPath string) string {
	date := time.Now().UTC().Format("2006-01-02")
	return fmt.Sprintf("# hop config — generated by 'hop config scan %s' on %s (UTC).\n# Run with --write to merge into %s.\n",
		userArg, date, configPath)
}

// scanPlanSummary aggregates the counters needed for the stderr summary block
// per spec § "Stderr summary".
type scanPlanSummary struct {
	defaultMatched        int      // count of Found assigned to default group
	defaultNew            int      // subset of defaultMatched not already in hop.yaml
	defaultExisting       int      // subset of defaultMatched already registered
	inventedGroups        []string // names of invented groups (in caller order)
	inventedURLCount      int      // total URLs assigned to invented groups
	skipNoGroupName       int      // CLI-layer slugify-empty skips
	skipAlreadyRegistered int      // CLI-layer dedup skips for non-convention URLs already in hop.yaml
}

// buildScanPlan converts the Walk's Found list into a yamled.ScanPlan,
// applying the convention check (spec § "Convention check"), slugify (spec §
// "Invented group naming"), per-parent-dir granularity (spec § "Per-parent-dir
// granularity"), and conflict resolution (spec § "Conflict resolution").
func buildScanPlan(cfg *config.Config, found []scan.Found, stderr io.Writer) (yamled.ScanPlan, scanPlanSummary) {
	var summary scanPlanSummary
	plan := yamled.ScanPlan{}

	// Pre-collect existing URLs so we can split default-matched into "new" vs
	// "already registered" for the summary line.
	existingURLs := make(map[string]struct{})
	for _, g := range cfg.Groups {
		for _, u := range g.URLs {
			existingURLs[u] = struct{}{}
		}
	}

	// Pre-collect existing flat-group canonical dirs keyed by group name (so
	// conflict resolution can match dir-equality without re-deriving on every
	// hit).
	existingGroupDirs := buildExistingGroupDirs(cfg)

	// Track invented groups by canonical parent dir → group name (so two
	// repos under the same parent share a group; per spec § "Per-parent-dir
	// granularity").
	inventedByDir := make(map[string]int) // dir → index into plan.InventedGroups

	// Track invented group names already used (existing + invented this scan)
	// so suffix collision detection works.
	usedNames := make(map[string]struct{})
	for _, g := range cfg.Groups {
		usedNames[g.Name] = struct{}{}
	}

	for _, f := range found {
		if matchesConvention(cfg, f) {
			summary.defaultMatched++
			if _, in := existingURLs[f.URL]; in {
				summary.defaultExisting++
			} else {
				summary.defaultNew++
				plan.DefaultURLs = append(plan.DefaultURLs, f.URL)
			}
			continue
		}

		// Non-convention URL already registered somewhere in hop.yaml:
		// yamled.MergeScan would silently dedup it, so the plan + summary
		// would otherwise overstate what's actually being added (and the
		// re-scan would needlessly rewrite the file). Drop it from the plan
		// here and surface a skip: line for visibility.
		if _, in := existingURLs[f.URL]; in {
			fmt.Fprintf(stderr, "skip: %s: %s already registered in hop.yaml\n", f.Path, f.URL)
			summary.skipAlreadyRegistered++
			continue
		}

		parentDir := filepath.Dir(f.Path)
		if idx, ok := inventedByDir[parentDir]; ok {
			plan.InventedGroups[idx].URLs = append(plan.InventedGroups[idx].URLs, f.URL)
			summary.inventedURLCount++
			continue
		}

		base := filepath.Base(parentDir)
		slug, ok := slugifyGroupName(base)
		if !ok {
			fmt.Fprintf(stderr, "skip: %s: cannot derive group name from parent dir '%s'\n", f.Path, base)
			summary.skipNoGroupName++
			continue
		}

		renderedDir := homeSubstitute(parentDir)
		finalName := resolveInventedName(slug, renderedDir, existingGroupDirs, usedNames, stderr)

		plan.InventedGroups = append(plan.InventedGroups, yamled.InventedGroup{
			Name: finalName,
			Dir:  renderedDir,
			URLs: []string{f.URL},
		})
		inventedByDir[parentDir] = len(plan.InventedGroups) - 1
		summary.inventedURLCount++
		usedNames[finalName] = struct{}{}
	}

	// Sort invented groups alphabetically per spec § "Group ordering" #3.
	sort.SliceStable(plan.InventedGroups, func(i, j int) bool {
		return plan.InventedGroups[i].Name < plan.InventedGroups[j].Name
	})
	for _, ig := range plan.InventedGroups {
		summary.inventedGroups = append(summary.inventedGroups, ig.Name)
	}

	return plan, summary
}

// matchesConvention returns true when a Found's canonical Path equals the
// canonical convention path for its URL: <expanded-code_root>/<org>/<name>
// (org dropped when empty). Per spec § "Convention check".
//
// Both sides are run through filepath.EvalSymlinks before comparison to handle
// platforms where $HOME (or its ancestors) is itself symlinked — e.g., macOS,
// where /tmp resolves to /private/tmp and t.TempDir() output threads through
// /var/folders → /private/var/folders. f.Path comes pre-canonical from
// scan.Walk, but the convention path is built from configured strings and
// must be canonicalized to compare apples to apples. EvalSymlinks failure
// (path doesn't exist on disk yet — common, the convention is hypothetical)
// falls back to filepath.Clean.
func matchesConvention(cfg *config.Config, f scan.Found) bool {
	codeRoot := repos.ExpandDir(cfg.CodeRoot, "")
	if codeRoot == "" {
		codeRoot = repos.ExpandDir("~", "")
	}
	org := repos.DeriveOrg(f.URL)
	name := repos.DeriveName(f.URL)
	var convention string
	if org == "" {
		convention = filepath.Join(codeRoot, name)
	} else {
		convention = filepath.Join(codeRoot, org, name)
	}
	return canonicalForCompare(f.Path) == canonicalForCompare(convention)
}

// buildExistingGroupDirs returns a map from group name → canonical dir for
// every existing group in cfg. Flat groups have no dir → "" sentinel.
// Map-shaped groups have their dir expanded via ExpandDir.
func buildExistingGroupDirs(cfg *config.Config) map[string]string {
	out := make(map[string]string, len(cfg.Groups))
	for _, g := range cfg.Groups {
		if g.Dir == "" {
			out[g.Name] = ""
			continue
		}
		expanded := repos.ExpandDir(g.Dir, cfg.CodeRoot)
		// Canonicalize for comparison (matches Found.Path's canonical form).
		if c, err := filepath.EvalSymlinks(expanded); err == nil {
			out[g.Name] = filepath.Clean(c)
		} else {
			out[g.Name] = filepath.Clean(expanded)
		}
	}
	return out
}

// resolveInventedName applies spec § "Conflict resolution" rules. If slug is
// unused → use slug. If slug exists with matching dir → reuse slug. If slug
// exists with differing dir (or already used by another invented group in
// this scan) → suffix with smallest non-colliding -N (>=2) and emit `note:`.
func resolveInventedName(slug, renderedDir string, existingGroupDirs map[string]string, usedNames map[string]struct{}, stderr io.Writer) string {
	canonicalRendered := canonicalForCompare(renderedDir)

	if existingDir, exists := existingGroupDirs[slug]; exists {
		if existingDir == canonicalRendered {
			return slug // dir match → reuse
		}
		// Slug match, dir differs → suffix.
		suffixed := nextAvailableSuffix(slug, usedNames)
		fmt.Fprintf(stderr, "note: invented group '%s' already exists in hop.yaml with a different dir; using '%s' for %s.\n",
			slug, suffixed, renderedDir)
		return suffixed
	}

	if _, used := usedNames[slug]; used {
		// Intra-scan collision: distinct parent dir slugifying to same name.
		suffixed := nextAvailableSuffix(slug, usedNames)
		fmt.Fprintf(stderr, "note: invented group '%s' already exists in hop.yaml with a different dir; using '%s' for %s.\n",
			slug, suffixed, renderedDir)
		return suffixed
	}

	return slug
}

// nextAvailableSuffix returns slug-N for the smallest N >= inventedSuffixStart
// that is not already in usedNames.
func nextAvailableSuffix(slug string, usedNames map[string]struct{}) string {
	for n := inventedSuffixStart; ; n++ {
		candidate := fmt.Sprintf("%s-%d", slug, n)
		if _, in := usedNames[candidate]; !in {
			return candidate
		}
	}
}

// canonicalForCompare resolves p through EvalSymlinks for dir-equality
// comparison; falls back to a clean path on failure (e.g., the dir doesn't
// exist yet).
func canonicalForCompare(p string) string {
	expanded := repos.ExpandDir(p, "")
	if expanded == "" {
		expanded = p
	}
	if c, err := filepath.EvalSymlinks(expanded); err == nil {
		return filepath.Clean(c)
	}
	return filepath.Clean(expanded)
}

// slugifyGroupName applies the 5-step slugify rule from spec § "Invented
// group naming". Returns ok=false when the slug ends up empty after trim
// (pathological input — examples in the spec: "///", "___", all-symbols).
//
// The trim step covers both `-` and `_` because the spec lists `___` as a
// pathological input that SHALL be skipped; treating `_` as a separator-class
// character at the boundary is the only reading that produces empty for
// `___` while keeping the regex-conformance step trivial. Internal `_` runs
// are preserved (they conform to the schema regex).
func slugifyGroupName(base string) (slug string, ok bool) {
	// Step 2: lowercase.
	s := strings.ToLower(base)
	// Step 3: replace any run of non-[a-z0-9_-] with a single '-'.
	var b strings.Builder
	prevDash := false
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			b.WriteRune(r)
			prevDash = false
			continue
		}
		if !prevDash {
			b.WriteByte('-')
			prevDash = true
		}
	}
	s = b.String()
	// Step 4: trim leading and trailing separator characters (`-` and `_`).
	// `_` is included so all-underscore input ("___") trims to empty per the
	// spec's pathological-input examples.
	s = strings.Trim(s, "-_")
	if s == "" {
		return "", false
	}
	// Step 5: ensure leading char is [a-z]; otherwise prefix 'g'.
	if s[0] < 'a' || s[0] > 'z' {
		s = "g" + s
	}
	if !scanGroupNameRe.MatchString(s) {
		// Defensive: the algorithm should always produce a conforming slug,
		// but if something exotic slips through (e.g., locale-dependent
		// uppercase) bail out as if empty.
		return "", false
	}
	return s, true
}

// homeSubstitute returns p with $HOME replaced by ~ when p begins under
// $HOME; otherwise p is returned verbatim. Spec § "Group dir rendering".
func homeSubstitute(p string) string {
	home := os.Getenv("HOME")
	if home == "" {
		return p
	}
	if p == home {
		return "~"
	}
	if strings.HasPrefix(p, home+string(os.PathSeparator)) {
		return "~" + p[len(home):]
	}
	return p
}

// emitScanSummary writes the stderr summary block per spec § "Stderr
// summary". One header line + indented breakdown + trailing tip/wrote.
func emitScanSummary(stderr io.Writer, userArg string, depth int, found []scan.Found, skips []scan.Skip, summary scanPlanSummary, configPath string, write bool) {
	totalFound := len(found)
	if totalFound == 0 {
		fmt.Fprintf(stderr, "%s: scanned %s (depth %d), found 0 repos. Nothing to add.\n",
			scanCmdName, userArg, depth)
		if write {
			fmt.Fprintf(stderr, "wrote: %s\n", configPath)
		} else {
			fmt.Fprintf(stderr, "Run with --write to merge into %s.\n", configPath)
		}
		return
	}

	fmt.Fprintf(stderr, "%s: scanned %s (depth %d), found %d %s.\n",
		scanCmdName, userArg, depth, totalFound, pluralize(totalFound, "repo", "repos"))

	// Convention-default line.
	if summary.defaultMatched > 0 {
		if write {
			fmt.Fprintf(stderr, "  matched convention (default): %d (%d new, %d already registered)\n",
				summary.defaultMatched, summary.defaultNew, summary.defaultExisting)
		} else {
			fmt.Fprintf(stderr, "  matched convention (default): %d\n", summary.defaultMatched)
		}
	}

	// Invented-groups line.
	if len(summary.inventedGroups) > 0 {
		fmt.Fprintf(stderr, "  invented groups: %d (%s)\n",
			len(summary.inventedGroups), strings.Join(summary.inventedGroups, ", "))
	}

	// Skipped line.
	skipParts := buildSkipParts(skips, summary.skipNoGroupName, summary.skipAlreadyRegistered)
	if len(skipParts) > 0 {
		fmt.Fprintf(stderr, "  skipped: %s\n", strings.Join(skipParts, ", "))
	}

	if write {
		fmt.Fprintf(stderr, "wrote: %s\n", configPath)
	} else {
		fmt.Fprintf(stderr, "Run with --write to merge into %s.\n", configPath)
	}
}

// buildSkipParts groups Walk's Skip slice by reason and adds the CLI-layer
// no-group-name and already-registered counts (which never appear as Skip
// entries per spec assumption #28). Empty buckets are omitted.
func buildSkipParts(skips []scan.Skip, noGroupName, alreadyRegistered int) []string {
	counts := make(map[string]int)
	for _, s := range skips {
		counts[s.Reason]++
	}
	var parts []string
	// Ordered for stable output.
	for _, reason := range []string{scan.ReasonWorktree, scan.ReasonSubmodule, scan.ReasonBareRepo, scan.ReasonNoRemote} {
		if c := counts[reason]; c > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", c, pluralizeReason(c, reason)))
		}
	}
	if noGroupName > 0 {
		parts = append(parts, fmt.Sprintf("%d no group name", noGroupName))
	}
	if alreadyRegistered > 0 {
		parts = append(parts, fmt.Sprintf("%d already registered", alreadyRegistered))
	}
	return parts
}

// pluralizeReason produces the right singular/plural suffix for a Skip
// reason. The closed set has its own pluralization (e.g., "1 worktree" vs
// "2 worktrees", "1 bare repo" vs "2 bare repos").
func pluralizeReason(n int, reason string) string {
	if n == 1 {
		return reason
	}
	switch reason {
	case scan.ReasonWorktree:
		return "worktrees"
	case scan.ReasonSubmodule:
		return "submodules"
	case scan.ReasonBareRepo:
		return "bare repos"
	case scan.ReasonNoRemote:
		return "no remote" // structurally singular; "no remote" plural is awkward; keep verbatim
	}
	return reason
}

// pluralize returns singular when n == 1, else plural. Used for the "found N
// repo(s)" line.
func pluralize(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}
