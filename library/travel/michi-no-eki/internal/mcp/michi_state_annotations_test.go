package mcp

import (
	"testing"

	"github.com/mark3labs/mcp-go/server"
)

func TestMichiRuntimeStateAnnotationsPreserveNativeAndExplicitHelpers(t *testing.T) {
	s := server.NewMCPServer("test", "test")
	RegisterTools(s)
	found := map[string]bool{}
	for _, registered := range s.ListTools() {
		if registered.Tool.Meta == nil {
			continue
		}
		command, _ := registered.Tool.Meta.AdditionalFields["pp:cli-command"].(string)
		if michiOptionalStateCommands[command] {
			found[command] = true
			if registered.Tool.Annotations.ReadOnlyHint == nil || *registered.Tool.Annotations.ReadOnlyHint {
				t.Fatalf("optional state helper still read-only: %s", command)
			}
		}
		if command == "catalog" || command == "find" || command == "compare" || command == "snapshot" {
			if registered.Tool.Annotations.ReadOnlyHint == nil || !*registered.Tool.Annotations.ReadOnlyHint {
				t.Fatalf("native hint lost: %s", command)
			}
		}
	}
	if len(found) != len(michiOptionalStateCommands) {
		t.Fatalf("state helper identities missing: %v", found)
	}
	for _, name := range []string{"sql", "context"} {
		tool := s.GetTool(name)
		if tool == nil || tool.Tool.Annotations.ReadOnlyHint == nil || !*tool.Tool.Annotations.ReadOnlyHint {
			t.Fatalf("direct read-only tool changed: %s", name)
		}
	}
	if len(s.ListTools()) != 29 {
		t.Fatalf("runtime catalog changed: %d", len(s.ListTools()))
	}
}
