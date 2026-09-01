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
		mcp.WithDescription("Dispatch one structured semantic KVM operation with evidence envelope (operation, transport, read_only, ok, evidence, state). Full TOOL_SPEC: capabilities, snapshot, ocr, verify, host.identity.inspect, host.graphics.inspect, service.render_access.inspect, host.reboot, select, hid_reset, rearm_otg, kvm_send_text, kvm_send_keys, kvm_hold_key, kvm_release_all, kvm_mouse_move, kvm_mouse_move_pct, kvm_mouse_click, kvm_mouse_scroll, kvm_status, kvm_screenshot_to_file, kvm_ocr_screenshot, kvm_ocr_click, exec_command, kvm_sequence_plan, kvm_sequence_authorize, kvm_sequence_execute, kvm_workflow_authorize, kvm_workflow_list, kvm_workflow_inspect, kvm_workflow_execute (plus legacy aliases status/keyboard/mouse/target-switch/sequence). Write-gated ops require KVMCTL_WRITE_ENABLED=1. Evidence is redacted via results.Build."),
		mcp.WithString("operation", mcp.Required(), mcp.Description("One of: "+strings.Join(semantic.Operations, ", "))),
		mcp.WithObject("arguments", mcp.Description("Operation arguments; include write_enabled gating, bounded numeric validation, and secret-free evidence. Exec requires transport=ssh, shell metachars rejected.")),
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
		raw["write_enabled"] = envTruthy(os.Getenv("KVMCTL_WRITE_ENABLED"))
		// propagate per-invocation flags from arguments envelope
		if v, ok := args["write_enabled"].(bool); ok {
			raw["write_enabled"] = v
		}
		out, err := semantic.Dispatch(ctx, c, stringArg(args, "operation"), raw)
		if err != nil {
			return mcpToolError(err.Error()), nil
		}
		return mcp.NewToolResultText(string(out)), nil
	})
}

func stringArg(m map[string]any, k string) string { v, _ := m[k].(string); return v }

func envTruthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
