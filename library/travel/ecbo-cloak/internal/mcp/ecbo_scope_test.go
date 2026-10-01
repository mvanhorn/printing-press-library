package mcp

import (
	"encoding/json"
	"github.com/mark3labs/mcp-go/server"
	"testing"
)

func TestEcboMCPKeepsCacheDestinationWithOperator(t *testing.T) {
	s := server.NewMCPServer("ecbo-scope", "test")
	RegisterTools(s)
	tools := s.ListTools()
	found := false
	for name, entry := range tools {
		b, e := json.Marshal(entry.Tool)
		if e != nil {
			t.Fatal(e)
		}
		var tool map[string]any
		if e = json.Unmarshal(b, &tool); e != nil {
			t.Fatal(e)
		}
		schema, _ := tool["inputSchema"].(map[string]any)
		props, _ := schema["properties"].(map[string]any)
		if _, ok := props["cache-dir"]; ok {
			t.Fatalf("MCP %s exposes filesystem relocation", name)
		}
		if name == "inventory_refresh" {
			found = true
			annotations, _ := tool["annotations"].(map[string]any)
			if annotations["readOnlyHint"] == true || annotations["destructiveHint"] != false || annotations["openWorldHint"] != false {
				t.Fatalf("wrong local snapshot write hints: %v", annotations)
			}
		}
	}
	if !found {
		t.Fatal("inventory_refresh absent")
	}
}
