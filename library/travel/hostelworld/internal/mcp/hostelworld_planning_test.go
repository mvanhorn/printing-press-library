package mcp

import (
	"github.com/mark3labs/mcp-go/server"
	"testing"
)

func TestPlanningToolsDeclareOptionalLocalSaveAndLiveSource(t *testing.T) {
	s := server.NewMCPServer("test", "test")
	RegisterTools(s)
	for _, name := range []string{"hostels_inspect", "hostels_offers", "hostels_compare", "hostels_dates", "hostels_search"} {
		entry := s.GetTool(name)
		if entry == nil {
			t.Fatalf("missing %s", name)
		}
		a := entry.Tool.Annotations
		if a.ReadOnlyHint == nil || *a.ReadOnlyHint || a.DestructiveHint == nil || *a.DestructiveHint || a.OpenWorldHint == nil || !*a.OpenWorldHint {
			t.Fatalf("%s: misleading annotations %#v", name, a)
		}
		if _, ok := entry.Tool.InputSchema.Properties["save"]; !ok {
			t.Fatalf("%s save is not exposed", name)
		}
	}
	entry := s.GetTool("hostels_compare")
	for _, key := range []string{"id", "id2", "id3", "id4", "id5"} {
		if _, ok := entry.Tool.InputSchema.Properties[key]; !ok {
			t.Fatalf("missing distinct compare slot %s", key)
		}
	}
}
