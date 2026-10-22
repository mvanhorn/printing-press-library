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
