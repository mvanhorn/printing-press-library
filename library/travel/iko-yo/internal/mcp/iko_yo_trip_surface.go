// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package mcp

import (
	"context"
	"encoding/json"
	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mvanhorn/printing-press-library/library/travel/iko-yo/internal/trip"
)

// RegisterTripSurface replaces generic generated metadata through the public
// server registration API. Keep these source-specific contracts outside the
// generator/reserved packages; HTML Trip has no cursor, sync or search surface.
func RegisterTripSurface(s *server.MCPServer) {
	s.AddTool(mcplib.NewTool("context",
		mcplib.WithDescription("Get Iko-yo Trip planning scope, bounded scan limits, evidence rules and available commands. Call first when deciding whether this selected Trip source fits a request."),
		mcplib.WithReadOnlyHintAnnotation(true), mcplib.WithDestructiveHintAnnotation(false)),
		func(_ context.Context, _ mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
			v := map[string]any{
				"api": "iko-yo", "display_name": "Iko-yo Trip", "description": trip.ScopeNote, "tool_count": len(s.ListTools()),
				"resources": []map[string]any{{"name": "events", "syncable": false, "searchable": false}, {"name": "spots", "syncable": false, "searchable": false}},
				"commands":  []string{"trip_discover", "trip_inspect", "trip_compare", "trip_cached"},
				"query_tips": []string{
					"Use trip_discover for events/spots with kind, region, prefecture, keyword and dates. Source page=N pagination is followed only within max-pages (1–5 total); limit bounds returned matches separately.",
					"Listings are publication-ordered and include archived/ended events. Inspect scanned records, remaining pages and unknown counts; zero matches do not establish source-wide absence.",
					"Use trip_inspect for one spots/ID or events/ID; trip_compare accepts up to eight references, on/as-of dates, age-months and amenities.",
					"Age and amenity checks are supported/excluded/unknown published evidence, not admission or safety guarantees. Multi-day spans do not prove individual operation.",
					"Keep qualified child/adult fees separate. Application intervals, capacity and lottery terms do not establish live seats.",
					"Use trip_cached for selected saved facts with original observed_at; max-scan-records caps the local scan independently of limit. There is no complete catalog sync or standalone search tool.",
					"Successful Trip inputs disable automatic learning/journaling. Do not store child profiles, contributor identities or private travel histories.",
				},
			}
			raw, err := json.Marshal(v)
			if err != nil {
				return nil, err
			}
			return mcplib.NewToolResultText(string(raw)), nil
		})
	if entry := s.GetTool("sql"); entry != nil {
		entry.Tool.Description = "Run read-only SQL over selected public facts saved by trip_discover or trip_inspect. This is a partial local collection, not a complete catalog; resources(resource_type,id,data) holds normalized events/spots JSON. Prefer trip_cached for bounded keyword and evidence filters."
		s.AddTool(entry.Tool, handleTripSQL)
	}
	if entry := s.GetTool("published_family_facts"); entry != nil {
		mcplib.WithReadOnlyHintAnnotation(true)(&entry.Tool)
		mcplib.WithDestructiveHintAnnotation(false)(&entry.Tool)
		entry.Tool.Description = "Inspect one Iko-yo Trip spots/ID or events/ID supplied as path. Returns explicit age/facility, qualified fee and booking facts with unknowns and source URLs. Prefer trip_inspect for source-control flags."
		s.AddTool(entry.Tool, entry.Handler)
	}
}
