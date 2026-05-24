package mcp

import "testing"

// Read tools must advertise readOnlyHint; sync mutates local state (writes the
// snapshot DB + FTS index) and must not. A false readOnlyHint on a mutating
// tool is a real bug, so guard the count and the per-tool annotation.
func TestToolReadOnlyHints(t *testing.T) {
	ts := tools()
	if len(ts) != 19 {
		t.Fatalf("expected 19 tools, got %d", len(ts))
	}
	writeTools := map[string]bool{"sync": true}
	for _, spec := range ts {
		hint := spec.tool.Annotations.ReadOnlyHint
		if hint == nil {
			t.Fatalf("tool %q has no read-only annotation", spec.tool.Name)
		}
		wantReadOnly := !writeTools[spec.tool.Name]
		if *hint != wantReadOnly {
			t.Fatalf("tool %q readOnlyHint = %v, want %v", spec.tool.Name, *hint, wantReadOnly)
		}
	}
}
