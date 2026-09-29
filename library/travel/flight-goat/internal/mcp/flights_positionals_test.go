// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package mcp

import (
	"testing"

	"github.com/mark3labs/mcp-go/server"
)

// PATCH(amend-2026-09-29: one bracket per positional) — regression guard for
// the flights MCP tool. A single "[origin destination date]" bracket group in
// the command's Use string collapsed into one schema property whose value was
// passed to the CLI as ONE argv element, failing with "accepts 3 arg(s),
// received 1". The tool must expose origin, destination and date separately.
func TestMCPFlightsToolExposesSeparatePositionals(t *testing.T) {
	s := server.NewMCPServer("flight-goat", "test")
	RegisterTools(s)

	flights, ok := s.ListTools()["flights"]
	if !ok {
		t.Fatal("flights tool missing from registered tools")
	}
	props := flights.Tool.InputSchema.Properties
	for _, name := range []string{"origin", "destination", "date"} {
		if _, ok := props[name]; !ok {
			t.Errorf("flights tool schema missing positional property %q; got %v", name, keys(props))
		}
	}
	if _, ok := props["origin destination date"]; ok {
		t.Error(`flights tool schema still exposes the collapsed "origin destination date" property`)
	}
	for _, req := range flights.Tool.InputSchema.Required {
		switch req {
		case "origin", "destination", "date":
			t.Errorf("positional %q must stay optional (--trip and --segment replace the positional form)", req)
		}
	}
}

func keys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
