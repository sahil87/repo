package platform

import "testing"

func TestOpenToolNotEmpty(t *testing.T) {
	tool := OpenTool()
	if tool == "" {
		t.Fatalf("OpenTool() returned empty string")
	}
	// Sanity: must be one of the two supported tools (depending on host build).
	if tool != "open" && tool != "xdg-open" {
		t.Fatalf("OpenTool() = %q, expected 'open' or 'xdg-open'", tool)
	}
}
