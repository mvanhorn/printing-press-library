// Copyright 2026 zjsng. Licensed under Apache-2.0.
package mcp

import (
	"testing"

	"github.com/mark3labs/mcp-go/server"
)

func TestJRSourcesUsesStructuredCommandMirror(t *testing.T) {
	s := server.NewMCPServer("jr-east-review-fixture", "test")
	RegisterTools(s)
	if s.GetTool("sources_guide") != nil {
		t.Fatal("raw HTML typed endpoint must remain hidden")
	}
	tool := s.GetTool("sources")
	if tool == nil || tool.Tool.Meta == nil || tool.Tool.Meta.AdditionalFields["pp:tenant-gate"] != "child-cli" {
		t.Fatal("bounded sources command mirror is missing")
	}
	for _, name := range []string{"areas", "lines", "status", "impact", "planned", "certificates", "coverage"} {
		if s.GetTool(name) == nil {
			t.Fatal("native domain command missing from MCP", name)
		}
	}
}
