package mcp

import (
	"context"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mvanhorn/printing-press-library/library/travel/repark/internal/repark"
)

// Validate before client construction so invalid typed MCP requests cannot
// reach configuration, caches or the provider.
func boundedReparkMarkers(next server.ToolHandlerFunc) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		window, ok := req.GetArguments()["range"].(string)
		if !ok {
			return mcpToolError("range must be an explicit bounded marker-window string"), nil
		}
		if err := repark.ValidateMarkerRange(window); err != nil {
			return mcpToolError(err.Error()), nil
		}
		return next(ctx, req)
	}
}
