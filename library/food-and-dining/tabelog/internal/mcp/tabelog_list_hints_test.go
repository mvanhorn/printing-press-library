package mcp

import (
	"encoding/json"
	"testing"

	"github.com/mark3labs/mcp-go/server"
)

func TestActualListMCPToolsExposeLocalWriteHints(t *testing.T) {
	resetMCPPathEnv(t)
	s := server.NewMCPServer("list-classification", "test")
	RegisterTools(s)
	for _, name := range []string{"lists_add", "lists_note", "lists_remove", "lists_refresh", "lists_compare"} {
		t.Run(name, func(t *testing.T) {
			tool := s.GetTool(name)
			if tool == nil {
				t.Fatal("actual list MCP tool is missing")
			}
			encoded, err := json.Marshal(tool.Tool)
			if err != nil {
				t.Fatal(err)
			}
			var wire struct {
				Annotations struct {
					ReadOnly    *bool `json:"readOnlyHint"`
					Destructive *bool `json:"destructiveHint"`
					OpenWorld   *bool `json:"openWorldHint"`
				} `json:"annotations"`
			}
			if err := json.Unmarshal(encoded, &wire); err != nil {
				t.Fatal(err)
			}
			hints := wire.Annotations
			if hints.Destructive == nil || *hints.Destructive {
				t.Fatalf("local notebook tool lacks nondestructive hint: %s", encoded)
			}
			if name == "lists_compare" {
				if hints.ReadOnly == nil || !*hints.ReadOnly {
					t.Fatal("offline comparison lost its read-only hint")
				}
				return
			}
			remote := name == "lists_refresh"
			if (hints.ReadOnly != nil && *hints.ReadOnly) || hints.OpenWorld == nil || *hints.OpenWorld != remote {
				t.Fatalf("local notebook mutation has misleading MCP scope: %s", encoded)
			}
		})
	}
}

func TestActualRecipeMCPHintsMatchOfflineComparisonAndRemoteRefresh(t *testing.T) {
	resetMCPPathEnv(t)
	s := server.NewMCPServer("recipe-classification", "test")
	RegisterTools(s)
	for _, tc := range []struct {
		name               string
		readOnly, external bool
	}{{"compare_saved_candidates_offline", true, false}, {"refresh_one_saved_candidate", false, true}} {
		t.Run(tc.name, func(t *testing.T) {
			tool := s.GetTool(tc.name)
			if tool == nil {
				t.Fatal("actual recipe tool is missing")
			}
			b, err := json.Marshal(tool.Tool)
			if err != nil {
				t.Fatal(err)
			}
			var wire struct {
				Annotations struct {
					ReadOnly    *bool `json:"readOnlyHint"`
					Destructive *bool `json:"destructiveHint"`
					OpenWorld   *bool `json:"openWorldHint"`
				} `json:"annotations"`
			}
			if err := json.Unmarshal(b, &wire); err != nil {
				t.Fatal(err)
			}
			h := wire.Annotations
			if h.ReadOnly == nil || *h.ReadOnly != tc.readOnly || h.Destructive == nil || *h.Destructive || h.OpenWorld == nil || *h.OpenWorld != tc.external {
				t.Fatalf("recipe hints misstate its behavior: %s", b)
			}
		})
	}
}
