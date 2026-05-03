// Package platform isolates OS-specific behavior behind build tags.
// The Open function is defined in open_darwin.go (uses `open`) and
// open_linux.go (uses `xdg-open`); other platforms fail at link time
// (Constitution Cross-Platform Behavior section).
package platform
