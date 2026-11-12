package mcp

import (
	"testing"

	"github.com/mark3labs/mcp-go/server"
)

// Digg removed /api/trending/status in October 2026; the MCP server must
// not advertise a tool that can only fail.
func TestRegisterToolsOmitsTrendingStatus(t *testing.T) {
	s := server.NewMCPServer("digg-test", "0")
	RegisterTools(s)
	if len(s.ListTools()) == 0 {
		t.Fatal("RegisterTools registered nothing")
	}
	if s.GetTool("trending_status") != nil {
		t.Fatal("trending_status is still registered")
	}
}

// Commands removed from the CLI because Digg stopped serving their data
// must not survive as MCP tools.
func TestRegisterToolsOmitsRemovedCommands(t *testing.T) {
	s := server.NewMCPServer("digg-test", "0")
	RegisterTools(s)
	for _, name := range []string{"rankings_list", "top", "history"} {
		if s.GetTool(name) == nil {
			t.Errorf("%s should still be registered", name)
		}
	}
	for _, name := range []string{"events", "evidence", "sentiment", "crossref", "replaced", "rising", "rankings_emerging", "rankings_movers"} {
		if s.GetTool(name) != nil {
			t.Errorf("%s is still registered", name)
		}
	}
}
