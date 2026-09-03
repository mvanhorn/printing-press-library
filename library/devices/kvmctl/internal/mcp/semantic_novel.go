package mcp

import (
	"context"
	"os"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mvanhorn/printing-press-library/library/devices/kvmctl/internal/semantic"
)

func registerSemanticTool(s *server.MCPServer) {
	tool := mcp.NewTool("semantic_dispatch",
		mcp.WithDescription("Dispatch one structured semantic KVM operation with evidence envelope (operation, transport, read_only, ok, evidence, state). OCR observation loop: observe: no arguments; verify-text: arguments.text; click-text: arguments.text and arguments.observation_id; requires write_enabled=true; press-key: arguments.key and arguments.observation_id; requires write_enabled=true. Observation IDs come from observe and are rechecked against a fresh screen capture; do not plan or infer UI actions. Write-gated ops require KVMCTL_WRITE_ENABLED=1. Evidence is redacted via results.Build."),
		mcp.WithString("operation", mcp.Required(), mcp.Description("One of: "+strings.Join(semantic.Operations, ", "))),
		mcp.WithObject("arguments", mcp.Description("Operation arguments. OCR: observe has none; verify-text needs text; click-text needs text plus observation_id and write_enabled=true; press-key needs key plus observation_id and write_enabled=true. Exec requires transport=ssh; shell metachars rejected.")),
	)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		c, session, err := newMCPClient(ctx)
		if err != nil {
			return mcpToolError(err.Error()), nil
		}
		if session != nil {
			defer session.ZeroCredentials()
		}
		args := req.GetArguments()
		raw, _ := args["arguments"].(map[string]any)
		if raw == nil {
			raw = map[string]any{}
		}
		rawRequestedWrite := raw["write_enabled"]
		raw["write_enabled"] = mcpWriteEnabled(envTruthy(os.Getenv("KVMCTL_WRITE_ENABLED")), rawRequestedWrite)
		out, err := semantic.Dispatch(ctx, c, stringArg(args, "operation"), raw)
		if err != nil {
			return mcpToolError(err.Error()), nil
		}
		return mcp.NewToolResultText(string(out)), nil
	})
}

func stringArg(m map[string]any, k string) string { v, _ := m[k].(string); return v }

func mcpWriteEnabled(hostPolicy bool, raw any) bool {
	explicit, ok := raw.(bool)
	return hostPolicy && ok && explicit
}

func envTruthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
