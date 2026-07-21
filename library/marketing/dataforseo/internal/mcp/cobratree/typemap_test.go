// Copyright 2026 mazzsterr. Licensed under Apache-2.0. See LICENSE.

package cobratree

import (
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/spf13/cobra"
)

func TestToolOptionsExcludeFilesystemDatabasePath(t *testing.T) {
	cmd := &cobra.Command{Use: "analytics"}
	cmd.Flags().String("db", "", "Database path")
	cmd.Flags().Int("limit", 25, "Maximum rows")

	tool := mcplib.NewTool("analytics", toolOptionsForFlags(cmd)...)
	if _, ok := tool.InputSchema.Properties["db"]; ok {
		t.Fatal("MCP tool schema exposes blocked --db filesystem path")
	}
	if _, ok := tool.InputSchema.Properties["limit"]; !ok {
		t.Fatal("MCP tool schema dropped ordinary --limit flag")
	}
}
