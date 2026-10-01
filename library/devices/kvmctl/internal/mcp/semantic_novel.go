package mcp

import (
	"context"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mvanhorn/printing-press-library/library/devices/kvmctl/internal/semantic"
)

func registerSemanticTool(s *server.MCPServer) {
	tool := mcp.NewTool("semantic_dispatch",
		mcp.WithDescription("Dispatch a read-only semantic KVM observation or plan with an evidence envelope. observe: no arguments; verify-text: arguments.text. Direct physical actions, including click-text and press-key, are disabled through MCP because they lack target- and operation-bound authorization. Use an authorized sequence or workflow for supported actions."),
		mcp.WithString("operation", mcp.Required(), mcp.Description("One of: "+strings.Join(semanticMCPReadOperations, ", "))),
		mcp.WithObject("arguments", mcp.Description("Operation arguments. observe needs none; verify-text needs text. Planning and inspection operations use their documented arguments.")),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
	)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		operation := stringArg(args, "operation")
		if !semanticMCPReadAllowed(operation) {
			return mcpToolError(physicalWriteGuidance), nil
		}
		c, session, err := newMCPClient(ctx)
		if err != nil {
			return mcpToolError(err.Error()), nil
		}
		if session != nil {
			defer session.ZeroCredentials()
		}
		raw, _ := args["arguments"].(map[string]any)
		if raw == nil {
			raw = map[string]any{}
		}
		out, err := semantic.Dispatch(ctx, c, operation, raw)
		if err != nil {
			return mcpToolError(err.Error()), nil
		}
		return mcp.NewToolResultText(string(out)), nil
	})
}

func stringArg(m map[string]any, k string) string { v, _ := m[k].(string); return v }
