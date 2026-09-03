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

func TestMCPWriteGateRequiresHostPolicyAndExplicitArgument(t *testing.T) {
	for _, tc := range []struct {
		name string
		host bool
		raw  any
		want bool
	}{
		{"both true", true, true, true},
		{"host policy false", false, true, false},
		{"argument omitted", true, nil, false},
		{"argument false", true, false, false},
		{"argument wrong type", true, "true", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := mcpWriteEnabled(tc.host, tc.raw); got != tc.want {
				t.Fatalf("mcpWriteEnabled(%v, %#v) = %v, want %v", tc.host, tc.raw, got, tc.want)
			}
		})
	}
}
