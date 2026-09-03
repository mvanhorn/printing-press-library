package mcp

import (
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/server"
)

func TestSemanticDispatchDescribesOCRObservationLoopArguments(t *testing.T) {
	s := server.NewMCPServer("kvmctl", "test")
	RegisterTools(s)
	registered, ok := s.ListTools()["semantic_dispatch"]
	if !ok {
		t.Fatal("semantic_dispatch tool is not registered")
	}
	description := registered.Tool.Description
	for _, want := range []string{
		"observe: no arguments",
		"verify-text: arguments.text",
		"click-text: arguments.text and arguments.observation_id; requires write_enabled=true",
		"press-key: arguments.key and arguments.observation_id; requires write_enabled=true",
	} {
		if !strings.Contains(description, want) {
			t.Fatalf("semantic_dispatch description missing %q: %s", want, description)
		}
	}
}
