package mcp

import (
	"github.com/mark3labs/mcp-go/server"
	"testing"
)

func TestCanonicalEplusToolsMirrorDomainCommands(t *testing.T) {
	s := server.NewMCPServer("eplus", "test")
	RegisterTools(s)
	for _, name := range []string{"events_search", "events_detail", "international_search", "international_detail", "compare", "policies"} {
		r := s.GetTool(name)
		if r == nil || r.Tool.Meta == nil || r.Tool.Meta.AdditionalFields["pp:tenant-gate"] != "child-cli" {
			t.Fatalf("%s bypasses domain CLI", name)
		}
		if r.Tool.Annotations.ReadOnlyHint == nil || !*r.Tool.Annotations.ReadOnlyHint {
			t.Fatalf("%s missing read-only", name)
		}
	}
	r := s.GetTool("events_search")
	for _, flag := range []string{"artist", "from", "limit", "fresh", "pages"} {
		if _, ok := r.Tool.InputSchema.Properties[flag]; !ok {
			t.Fatalf("domain search lacks flag %s", flag)
		}
	}
}
