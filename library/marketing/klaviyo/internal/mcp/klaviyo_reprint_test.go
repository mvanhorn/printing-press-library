// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package mcp

import (
	"encoding/json"
	"github.com/mark3labs/mcp-go/server"
	"os"
	"testing"
)

func TestKlaviyoReprintRegisteredToolCount(t *testing.T) {
	t.Setenv("KLAVIYO_MCP_READ_ONLY", "")
	s := server.NewMCPServer("klaviyo-reprint", "test")
	RegisterTools(s)
	data, err := os.ReadFile("../../.printing-press.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Total  int `json:"mcp_tool_count"`
		Public int `json:"mcp_public_tool_count"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	count := len(s.ListTools())
	if count != manifest.Total || count != manifest.Public {
		t.Fatalf("registered tools=%d, manifest=%+v", count, manifest)
	}
	t.Logf("registered MCP tools: %d", count)
}
