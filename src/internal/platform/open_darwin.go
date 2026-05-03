//go:build darwin

package platform

import (
	"context"

	"github.com/sahil87/repo/internal/proc"
)

// Open opens path in the OS file manager (Finder on macOS).
func Open(ctx context.Context, path string) error {
	_, err := proc.Run(ctx, "open", path)
	return err
}

// OpenTool returns the binary name used to open paths on this OS.
func OpenTool() string {
	return "open"
}
