package cobratree

import (
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/spf13/cobra"
)

func TestToolOptionsForFlagsOmitsBlockedRootFlags(t *testing.T) {
	root := &cobra.Command{Use: "root"}
	for name := range blockedRootFlags {
		root.PersistentFlags().String(name, "", "blocked")
	}
	root.PersistentFlags().Bool("json", false, "json output")
	child := &cobra.Command{Use: "find"}
	child.Flags().String("format", "", "file format")
	root.AddCommand(child)

	tool := mcplib.NewTool("find", toolOptionsForFlags(child)...)
	props := tool.InputSchema.Properties
	for name := range blockedRootFlags {
		if _, ok := props[name]; ok {
			t.Errorf("blocked flag %q advertised in MCP schema: %#v", name, props)
		}
	}
	if _, ok := props["format"]; !ok {
		t.Fatalf("command-local --format missing from schema: %#v", props)
	}
	if _, ok := props["json"]; !ok {
		t.Fatalf("inherited --json missing from schema: %#v", props)
	}
}
