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

// Digg dropped the Emerging and Movers views from its rankings page; the
// CLI keeps the commands as fail-closed stubs, but MCP must not list them.
func TestRegisterToolsOmitsRemovedRankingsViews(t *testing.T) {
	s := server.NewMCPServer("digg-test", "0")
	RegisterTools(s)
	if s.GetTool("rankings_list") == nil {
		t.Fatal("rankings_list should still be registered")
	}
	for _, name := range []string{"rankings_emerging", "rankings_movers"} {
		if s.GetTool(name) != nil {
			t.Fatalf("%s is still registered", name)
		}
	}
}
