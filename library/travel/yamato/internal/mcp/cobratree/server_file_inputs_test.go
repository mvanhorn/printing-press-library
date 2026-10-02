package cobratree

import (
	"context"
	"strings"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/spf13/cobra"
)

func TestMCPServerFileInputsAreRejected(t *testing.T) {
	cmd := &cobra.Command{Use: "teach-playbook"}
	for _, name := range []string{"notes-file", "playbook-file", "playbook-notes-file"} {
		cmd.Flags().String(name, "", "Read a server-side file")
	}
	cmd.Flags().String("notes", "", "Inline notes")
	blocked := blockedStructuredArgsForCommand(cmd)
	positionals := positionalArgsForCommand(cmd, blocked)
	allowed := allowedStructuredArgsForCommand(cmd, blocked, positionals, true)
	tool := mcplib.NewTool("teach-playbook", toolOptionsForFlags(cmd, blocked, positionals)...)
	if !allowed["notes"] {
		t.Fatal("inline notes must remain available")
	}
	handler := shellOutToCLI(func() (string, error) { return "unused-companion-cli", nil }, []string{"teach-playbook"}, blocked, allowed, positionals, false, nil)
	for _, name := range []string{"notes-file", "playbook-file", "playbook-notes-file"} {
		t.Run(name, func(t *testing.T) {
			if _, exists := tool.InputSchema.Properties[name]; exists {
				t.Errorf("server file input is exposed in the MCP schema")
			}
			result, err := handler(context.Background(), mcplib.CallToolRequest{Params: mcplib.CallToolParams{
				Arguments: map[string]any{name: "/server/private-file"},
			}})
			if err != nil || !result.IsError || !strings.Contains(toolResultText(result), "unknown MCP parameter") {
				t.Fatalf("file input was not rejected before shell-out: result=%v err=%v", result, err)
			}
		})
	}
}
