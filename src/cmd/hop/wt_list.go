package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/sahil87/hop/internal/proc"
)

// wtListTimeout bounds each `wt list --json` invocation. Matches the 5-second
// precedent set by `internal/scan` for `git remote` invocations (wt list is a
// local op — no network round-trip).
const wtListTimeout = 5 * time.Second

// WtEntry mirrors a single entry in wt's `list --json` output. Unknown JSON
// fields are silently ignored (Go's default — no DisallowUnknownFields) so
// future wt schema additions don't break hop.
type WtEntry struct {
	Name      string `json:"name"`
	Branch    string `json:"branch"`
	Path      string `json:"path"`
	IsMain    bool   `json:"is_main"`
	IsCurrent bool   `json:"is_current"`
	Dirty     bool   `json:"dirty"`
	Unpushed  int    `json:"unpushed"`
}

// listWorktrees invokes `wt list --json` in the given repo's main checkout
// and unmarshals the result. The default implementation builds a 5-second
// context and routes through `internal/proc.RunCapture` (Constitution
// Principle I — all subprocess execution goes through internal/proc).
//
// Exposed as a package-level var so tests can inject a fake without needing a
// real `wt` binary on PATH. Mirrors the seam pattern in
// `internal/fzf/fzf.go::runInteractive`.
//
// Errors:
//   - proc.ErrNotFound when `wt` is not on PATH (callers may match via
//     errors.Is to produce the install hint).
//   - Any other subprocess error is returned verbatim.
//   - JSON unmarshal failures are wrapped as "wt list: <err>".
var listWorktrees = defaultListWorktrees

func defaultListWorktrees(ctx context.Context, repoPath string) ([]WtEntry, error) {
	ctx, cancel := context.WithTimeout(ctx, wtListTimeout)
	defer cancel()
	out, err := proc.RunCapture(ctx, repoPath, "wt", "list", "--json")
	if err != nil {
		return nil, err
	}
	return unmarshalWtEntries(out)
}

// unmarshalWtEntries parses wt's `list --json` output into []WtEntry.
// Extracted from defaultListWorktrees so the JSON contract can be exercised
// in tests without spawning a process. Unmarshal failures are wrapped with a
// "wt list: " prefix so callers can route the error through the
// "hop: wt list: <err>" stderr line without further wrapping.
func unmarshalWtEntries(out []byte) ([]WtEntry, error) {
	var entries []WtEntry
	if err := json.Unmarshal(out, &entries); err != nil {
		return nil, fmt.Errorf("wt list: %w", err)
	}
	return entries, nil
}
