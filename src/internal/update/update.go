// Package update implements `hop update` — self-upgrade via Homebrew.
//
// All subprocess invocations route through internal/proc per Constitution
// Principle I (no direct os/exec outside internal/proc). The brew formula is
// referenced by its fully-qualified name (sahil87/tap/hop) to avoid a name
// collision with the Homebrew core `hop` cask.
package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sahil87/hop/internal/proc"
)

// brewFormula is the fully-qualified tap formula. The fully-qualified form
// disambiguates against the `hop` cask (an HWP document viewer) that would
// otherwise shadow it on `brew info hop`.
const brewFormula = "sahil87/tap/hop"

const (
	brewUpdateTimeout  = 30 * time.Second
	brewInfoTimeout    = 30 * time.Second
	brewUpgradeTimeout = 120 * time.Second
)

// Run self-updates the hop binary via Homebrew.
//
// currentVersion is the binary's reported version (e.g. "v0.0.3"). The leading
// "v" is stripped before comparison since `brew info` reports the bare form.
//
// out and errOut receive only the WRAPPER messages this package emits ("Current
// version:", "Already up to date", error hints, etc.). Subprocess stdout/stderr
// from `brew update`, `brew info`, and `brew upgrade` is intentionally NOT
// routed through these writers — internal/proc owns subprocess stream routing
// (proc.Run pipes child stderr to the parent's os.Stderr; proc.RunForeground
// inherits all three streams). The split is deliberate: subprocess streams
// are large and tty-aware (brew prints colored progress); the wrapper messages
// are small and may be redirected for tests or embedding. Callers in production
// should pass os.Stdout / os.Stderr to keep the two consistent.
//
// Returns nil on success or no-op (not a brew install, already up to date).
// Returns proc.ErrNotFound when brew is missing on PATH (callers should map
// this to errSilent so cobra does not double-print). Returns a wrapped error
// for other brew failures.
func Run(currentVersion string, out, errOut io.Writer) error {
	if !isBrewInstalled() {
		fmt.Fprintf(out, "hop %s was not installed via Homebrew.\n", currentVersion)
		fmt.Fprintln(out, "Update manually, or reinstall with: brew install "+brewFormula)
		return nil
	}

	fmt.Fprintf(out, "Current version: %s\n", currentVersion)
	fmt.Fprintln(out, "Checking for updates...")

	ctx, cancel := context.WithTimeout(context.Background(), brewUpdateTimeout)
	_, err := proc.Run(ctx, "brew", "update", "--quiet")
	cancel()
	if err != nil {
		if errors.Is(err, proc.ErrNotFound) {
			fmt.Fprintln(errOut, "hop update: brew not found on PATH.")
			return err
		}
		return fmt.Errorf("brew update failed: %w", err)
	}

	latest, err := brewLatestVersion()
	if err != nil {
		if errors.Is(err, proc.ErrNotFound) {
			fmt.Fprintln(errOut, "hop update: brew not found on PATH.")
			return err
		}
		return fmt.Errorf("could not determine latest version: %w", err)
	}

	if normalizeVersion(latest) == normalizeVersion(currentVersion) {
		fmt.Fprintf(out, "Already up to date (%s).\n", currentVersion)
		return nil
	}

	fmt.Fprintf(out, "Updating %s → v%s...\n", currentVersion, normalizeVersion(latest))

	upCtx, upCancel := context.WithTimeout(context.Background(), brewUpgradeTimeout)
	defer upCancel()
	code, err := proc.RunForeground(upCtx, "", "brew", "upgrade", brewFormula)
	if err != nil {
		if errors.Is(err, proc.ErrNotFound) {
			fmt.Fprintln(errOut, "hop update: brew not found on PATH.")
			return err
		}
		return fmt.Errorf("brew upgrade failed: %w", err)
	}
	if code != 0 {
		return fmt.Errorf("brew upgrade exited with code %d", code)
	}

	fmt.Fprintf(out, "Updated to v%s.\n", normalizeVersion(latest))
	return nil
}

// brewLatestVersion queries Homebrew for the latest stable version of the
// tap formula. Returns the bare version string (e.g. "0.0.3") with no `v`
// prefix — that's how brew reports it in `versions.stable`.
func brewLatestVersion() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), brewInfoTimeout)
	defer cancel()
	out, err := proc.Run(ctx, "brew", "info", "--json=v2", brewFormula)
	if err != nil {
		return "", err
	}
	var info struct {
		Formulae []struct {
			Versions struct {
				Stable string `json:"stable"`
			} `json:"versions"`
		} `json:"formulae"`
	}
	if err := json.Unmarshal(out, &info); err != nil {
		return "", err
	}
	if len(info.Formulae) == 0 || info.Formulae[0].Versions.Stable == "" {
		return "", errors.New("no stable version found in brew info output")
	}
	return info.Formulae[0].Versions.Stable, nil
}

// isBrewInstalled checks whether the running binary lives under a Cellar
// directory, which is the canonical signature of a Homebrew install. The
// symlink at /opt/homebrew/bin/hop (or /usr/local/bin/hop on Intel) resolves
// through to .../Cellar/hop/<version>/bin/hop.
func isBrewInstalled() bool {
	self, err := os.Executable()
	if err != nil {
		return false
	}
	real, err := filepath.EvalSymlinks(self)
	if err != nil {
		return false
	}
	return strings.Contains(real, "/Cellar/")
}

// normalizeVersion strips a single leading "v" so we can compare the binary's
// `git describe`-derived version (e.g. "v0.0.3") against brew's bare report
// ("0.0.3"). It does NOT do semver parsing — string equality after normalize
// is sufficient because both sides come from the same canonical source (the
// release tag).
func normalizeVersion(v string) string {
	return strings.TrimPrefix(v, "v")
}
