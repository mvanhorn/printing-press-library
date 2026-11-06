// Copyright 2026 Greg Ceccarelli and contributors. Licensed under Apache-2.0. See LICENSE.

package mcp

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/server"
)

func TestReprintRetainsWorkflowToolsAndEverySpecOperation(t *testing.T) {
	s := server.NewMCPServer("tella", "test")
	RegisterTools(s)
	mirrored := map[string]bool{}
	for _, tool := range s.ListTools() {
		if tool.Tool.Meta != nil {
			if command, ok := tool.Tool.Meta.AdditionalFields["pp:cli-command"].(string); ok {
				mirrored[command] = true
			}
		}
	}
	for _, command := range []string{
		"transcripts search", "transcripts sync", "videos viewed", "webhooks tail", "webhooks replay",
		"clips edit-pass", "clips transcript-diff", "clips captions", "sources upload-file", "exports wait", "workspace stats",
		"videos storyboard", "videos apply-storyboard", "videos clips insert-file", "videos clips add-broll",
		"videos clips cut-words", "videos clips replace-word-ranges", "videos audit", "playlists edit-pass",
		"videos clips clean", "videos clips undo-last-cuts", "videos clips restore-cuts",
	} {
		if !mirrored[command] {
			t.Errorf("retained workflow %q missing from MCP runtime mirror", command)
		}
	}
	data, err := os.ReadFile("../../spec.json")
	if err != nil {
		t.Fatal(err)
	}
	var spec struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(data, &spec); err != nil {
		t.Fatal(err)
	}
	covered := map[string]bool{}
	for _, ep := range codeOrchEndpoints {
		if strings.HasSuffix(ep.Path, "/transcript/cut") || strings.HasSuffix(ep.Path, "/transcript/uncut") {
			t.Errorf("dead transcript handler: %s", ep.ID)
		}
		covered[strings.ToUpper(ep.Method)+" "+ep.Path] = true
	}
	count := 0
	for path, methods := range spec.Paths {
		for method := range methods {
			switch method {
			case "get", "post", "put", "patch", "delete", "head", "options":
				count++
				if !covered[strings.ToUpper(method)+" "+path] {
					t.Errorf("missing orchestration operation: %s %s", method, path)
				}
			}
		}
	}
	if count != 157 {
		t.Fatalf("spec operation count=%d, want 157", count)
	}
}
