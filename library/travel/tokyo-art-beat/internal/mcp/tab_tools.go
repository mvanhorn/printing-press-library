package mcp

import (
	"context"
	"encoding/json"
	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mvanhorn/printing-press-library/library/travel/tokyo-art-beat/internal/cli"
	"github.com/mvanhorn/printing-press-library/library/travel/tokyo-art-beat/internal/mcp/cobratree"
)

// Only the bounded domain CLI tree is exposed; raw endpoint and unused mirror tools remain scaffold code.
func registerTABTools(s *server.MCPServer) {
	s.AddTool(mcplib.NewTool("context", mcplib.WithDescription("Get API domain context for public Tokyo Art Beat discovery: bilingual source identity, bounded search, conservative schedules and official-link handoff."), mcplib.WithReadOnlyHintAnnotation(true), mcplib.WithDestructiveHintAnnotation(false)), func(ctx context.Context, r mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		payload := map[string]any{"provider": "Tokyo Art Beat", "access": "public read-only; no credentials", "languages": "public English/Japanese fields; omissions stay null", "source": "undocumented published website feed", "discovery": "bounded event/venue search, catalog name resolution, lazy detail, 2..4 edition comparison and straight-line nearby shortlist", "dates": "full-year inclusive spans; actual open days require conservative detail assessment and official confirmation", "ticket_inventory": "unknown; no listing establishes availability", "membership": "public MuPon indicators only; redemption/member-only content excluded", "cache": "TTL response cache, exact-query offline, explicit freshness/partial/candidate coverage", "tools": "all discovery tools execute the companion bounded CLI; no raw source, SQL or bulk mirror tools"}
		b, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		return mcplib.NewToolResultText(string(b)), nil
	})
	cobratree.RegisterAll(s, cli.RootCmd(), cobratree.SiblingCLIPath)
}
