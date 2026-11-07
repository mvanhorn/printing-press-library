package store

import "testing"

// The surface decides whether a learned playbook is trusted. Only the MCP
// server's own marker may set it, so a stray exported UNIFI_LEARN_SURFACE
// cannot relabel plain CLI use.
func TestLearnEventSurfaceFollowsMCPChildMarker(t *testing.T) {
	t.Cleanup(func() { SetLearnEventSurface("") })
	SetLearnEventSurface("")
	t.Setenv("UNIFI_LEARN_SURFACE", "mcp")
	t.Setenv("UNIFI_MCP_CHILD", "")
	if got := LearnEventSurface(); got != LearnSurfaceCLI {
		t.Fatalf("exported surface var gave %q, want cli", got)
	}
	t.Setenv("UNIFI_MCP_CHILD", "1")
	if got := LearnEventSurface(); got != LearnSurfaceMCP {
		t.Fatalf("MCP child marker gave %q, want mcp", got)
	}
}
