package mcp

import (
	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Refresh writes only to the local notebook, but it also retrieves public
// remote details. Preserve the local-write tier while describing that I/O.
func applyTabelogToolHints(s *server.MCPServer) {
	registered := s.GetTool("lists_refresh")
	if registered == nil {
		return
	}
	tool := registered.Tool
	mcplib.WithOpenWorldHintAnnotation(true)(&tool)
	s.AddTool(tool, registered.Handler)
}
