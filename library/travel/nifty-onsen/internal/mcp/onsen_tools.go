package mcp

import (
	"context"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mvanhorn/printing-press-library/library/travel/nifty-onsen/internal/cli"
	"github.com/mvanhorn/printing-press-library/library/travel/nifty-onsen/internal/mcp/cobratree"
)

// registerOnsenTools keeps MCP on the exact parsed CLI workflow and its bounds.
// The spec's generic HTML handlers cannot provide facility-local domain evidence.
func registerOnsenTools(s *server.MCPServer) {
	root := cli.RootCmd()
	for _, cmd := range root.Commands() {
		switch cmd.Name() {
		case "bath", "regions", "filters":
		default:
			root.RemoveCommand(cmd)
		}
	}
	s.AddTool(mcplib.NewTool("context", mcplib.WithDescription("Read Nifty Onsen domain rules and bounded workflow coverage before discovering day-use facilities or inspecting public coupon terms."), mcplib.WithReadOnlyHintAnnotation(true), mcplib.WithDestructiveHintAnnotation(false)), func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		return toolResultJSON(map[string]any{"provider": "Nifty Onsen", "tool_count": len(s.ListTools()), "source": "https://onsen.nifty.com/", "auth": "none", "read_only": true, "tool_surface": "Seven parsed provider workflows mirror the companion CLI; no raw HTML endpoint handlers or sync/SQL inventory.", "query_tips": []string{"Search defaults to source day-use classification; region is a prefecture slug or Japanese name.", "Search fetches one organic page: default limit 10, cap 30. page selects a source page, not a cursor; preserve the page tail before advancing.", "Nearby returns a bounded source map window of at most 20 candidates sorted by straight-line distance; coverage is partial.", "Use show/coupons lazily and compare 2 to 5 unique IDs; field selection keeps coverage/freshness metadata.", "Natural hot spring, private bath vs room, admission weekday/holiday/extras and coupon eligibility require explicit facility source evidence.", "Tattoo, child/accessibility and reservable inventory remain unknown unless explicit; public information links do not book or redeem anything."}})
	})
	cobratree.RegisterAll(s, root, cobratree.SiblingCLIPath)
}
