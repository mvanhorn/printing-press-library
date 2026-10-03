// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package mcp

import "github.com/mark3labs/mcp-go/server"

// These commands read a public source but optionally persist normalized local
// observations with --save. The generic local-write tier's closed-world default
// is inappropriate for a live provider request, so specialize only its hints.
func applyHostelworldPlanningToolSemantics(s *server.MCPServer) {
	for _, name := range []string{"hostels_inspect", "hostels_offers", "hostels_compare", "hostels_dates", "hostels_search"} {
		entry := s.GetTool(name)
		if entry == nil {
			continue
		}
		no, yes := false, true
		entry.Tool.Annotations.ReadOnlyHint = &no
		entry.Tool.Annotations.DestructiveHint = &no
		entry.Tool.Annotations.OpenWorldHint = &yes
		s.AddTool(entry.Tool, entry.Handler)
	}
}
