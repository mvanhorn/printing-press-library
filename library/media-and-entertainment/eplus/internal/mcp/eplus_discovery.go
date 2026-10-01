package mcp

import (
	"github.com/mark3labs/mcp-go/server"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/eplus/internal/cli"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/eplus/internal/mcp/cobratree"
)

// registerEplusDiscoveryTools gives canonical MCP names the same domain contract as CLI.
// Generic HTML endpoint handlers cannot preserve eplus sale/eligibility semantics.
func registerEplusDiscoveryTools(s *server.MCPServer) {
	s.DeleteTools("events_search", "events_detail")
	cobratree.RegisterAll(s, cli.RootCmd(), cobratree.SiblingCLIPath)
}
