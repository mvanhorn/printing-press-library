// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

package mcp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mvanhorn/printing-press-library/library/travel/japan-guide/internal/cli"
)

func TestGuideMCPRejectsSnapshotDirectoryOverride(t *testing.T) {
	resetMCPPathEnv(t)
	companion := filepath.Join(t.TempDir(), "unusable-companion")
	if err := os.WriteFile(companion, []byte("not an executable"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("JAPAN_GUIDE_CLI_PATH", companion)
	s := server.NewMCPServer("guide-boundary", "test")
	RegisterTools(s)
	tool := s.ListTools()["guide_inspect"]
	if _, exists := tool.Tool.InputSchema.Properties["cache-dir"]; exists {
		t.Error("MCP schema permits a caller-selected snapshot directory")
	}
	for _, allowed := range []string{"cache", "offline"} {
		if _, exists := tool.Tool.InputSchema.Properties[allowed]; !exists {
			t.Errorf("MCP schema lost legitimate %s parameter", allowed)
		}
	}
	command, _, err := cli.RootCmd().Find([]string{"guide", "inspect"})
	if err != nil || command.Flags().Lookup("cache-dir") == nil || command.Flags().Lookup("cache-dir").Hidden {
		t.Fatal("direct CLI must retain its operator-selected cache directory")
	}
	destination := filepath.Join(t.TempDir(), "outside-cache")
	result, err := tool.Handler(context.Background(), mcplib.CallToolRequest{Params: mcplib.CallToolParams{
		Name: "guide_inspect", Arguments: map[string]any{"page": "e3001", "cache": true, "cache-dir": destination},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError || !strings.Contains(mcpTextContent(t, result), `unknown MCP parameter "cache-dir"`) {
		t.Fatalf("caller-selected directory was not rejected before CLI execution: %s", mcpTextContent(t, result))
	}
	if _, err := os.Stat(destination); !os.IsNotExist(err) {
		t.Fatalf("rejected MCP request created a destination: %v", err)
	}
}
