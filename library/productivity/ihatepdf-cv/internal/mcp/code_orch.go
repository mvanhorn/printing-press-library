package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mvanhorn/printing-press-library/library/productivity/ihatepdf-cv/internal/mcp/cobratree"
)

// RegisterCodeOrchestrationTools is the curated thin MCP surface. Full
// endpoint and Cobra-tree tools remain available with IHATEPDF_CV_MCP_SURFACE=full.
func RegisterCodeOrchestrationTools(s *server.MCPServer) {
	RegisterCodeOrchestration(s)
}

// RegisterCodeOrchestration adds one high-value read-only composition so an
// agent can inspect and fingerprint a PDF in one round trip instead of
// discovering and invoking two shell-out tools.
func RegisterCodeOrchestration(s *server.MCPServer) {
	s.AddTool(
		mcplib.NewTool("inspect_and_fingerprint_pdf",
			mcplib.WithDescription("Inspect a local PDF and compute its page, metadata, validity, and hash records in one call. Required: path. Returns an object containing inspect and fingerprint results; use this before privacy-scan or transformations when you need a compact artifact identity."),
			mcplib.WithString("path", mcplib.Required(), mcplib.Description("Local PDF path to inspect and fingerprint.")),
			mcplib.WithReadOnlyHintAnnotation(true),
			mcplib.WithDestructiveHintAnnotation(false),
		),
		handleInspectAndFingerprint,
	)
}

func handleInspectAndFingerprint(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	path := fmt.Sprint(req.GetArguments()["path"])
	if path == "" || path == "<nil>" {
		return mcplib.NewToolResultError("path is required"), nil
	}
	if recipeCLIPathErr != nil {
		return mcplib.NewToolResultError(fmt.Sprintf("companion CLI binary not found: %v", recipeCLIPathErr)), nil
	}
	inspect, err := cobratree.RunCLICommand(ctx, recipeCLIPath, []string{"inspect", path, "--json", "--no-learn"})
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	fingerprint, err := cobratree.RunCLICommand(ctx, recipeCLIPath, []string{"fingerprint", path, "--json", "--no-learn"})
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(fmt.Sprintf(`{"inspect":%s,"fingerprint":%s}`, inspect, fingerprint)), &result); err != nil {
		return mcplib.NewToolResultError(fmt.Sprintf("decode composed result: %v", err)), nil
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	return mcplib.NewToolResultText(string(encoded)), nil
}
