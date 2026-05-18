// Copyright 2026 chris-drit. Licensed under Apache-2.0. See LICENSE.

// Package mcp exposes airframe's CLI surface as MCP tools.
//
// airframe has no upstream REST API, so unlike sec-edgar (which pre-registers
// typed tools per endpoint), every tool here is a shell-out to the companion
// `airframe-pp-cli` binary. The cobratree walker (generator-owned, copied
// verbatim from the press's emitted template) auto-registers every Cobra
// command as an MCP tool with sensible safety hints.
//
// A single hand-authored `context` tool front-loads airframe's domain model
// (the two-tier sync, the flight-goat composition, the 72-hour caveat) so
// agents can call it first and reason about which tier they need before
// invoking any of the data tools.
package mcp

import (
	"context"
	"encoding/json"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/airframe/internal/cli"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/airframe/internal/mcp/cobratree"
)

// RegisterTools wires every airframe capability as an MCP tool on s.
func RegisterTools(s *server.MCPServer) {
	// Front-loaded domain context. Agents should call this first to learn
	// the tier model and which tools they need for a given question.
	s.AddTool(
		mcplib.NewTool("context",
			mcplib.WithDescription("Get airframe domain context: two-tier sync model, command surface, install requirements (mdbtools for NTSB, flight-goat for flight ident lookups), and 72-hour caveat on commercial flight queries. Call this first."),
			mcplib.WithReadOnlyHintAnnotation(true),
			mcplib.WithDestructiveHintAnnotation(false),
		),
		handleContext,
	)

	// Runtime Cobra-tree walker — auto-registers every user-facing airframe
	// command (sync, tail, owner, model, event, flight, search, doctor) as
	// a shell-out MCP tool. Read-only hint annotations are inherited from
	// each command's `Annotations["mcp:read-only"]`.
	cobratree.RegisterAll(s, cli.RootCmd(), cobratree.SiblingCLIPath)
}

// handleContext returns a static JSON blob describing airframe's tier model
// and command surface. Kept inline (not loaded from a file) so the MCP
// binary is self-contained.
func handleContext(_ context.Context, _ mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	ctx := map[string]any{
		"name":        "airframe",
		"description": "Aircraft forensics from open public records — tail-number dossiers, fleet research, model-level safety, and NTSB event archaeology.",
		"data_sources": []map[string]any{
			{
				"name":     "FAA Aircraft Registry",
				"tier":     1,
				"install":  "none",
				"size_mb":  80,
				"url":      "https://registry.faa.gov/database/ReleasableAircraft.zip",
				"refresh":  "daily 23:30 CT",
				"covers":   "Every US-registered aircraft: tail number → owner, make/model, year, engine, status, Mode-S hex.",
				"commands": []string{"tail", "owner", "doctor"},
			},
			{
				"name":        "NTSB CAROL accident database",
				"tier":        2,
				"install":     "mdbtools package (brew/apt/dnf install mdbtools, or AUR on Arch)",
				"size_mb":     100,
				"url":         "https://data.ntsb.gov/avdata/FileDirectory/DownloadFile?fileID=C%3A%5Cavdata%5Cavall.zip",
				"refresh":     "monthly avall.zip + weekly up*.zip increments",
				"covers":      "Every NTSB-investigated US aviation event since 1982: probable cause, narrative, injuries, weather, phase of flight.",
				"commands":    []string{"model", "event", "search (with --with-fts)"},
				"file_format": "Microsoft Access .mdb (requires mdb-export from mdbtools)",
			},
		},
		"composition": map[string]any{
			"flight_goat": map[string]any{
				"command":          "airframe-pp-cli flight <ident>",
				"requires":         []string{"flight-goat-pp-cli on PATH", "FLIGHT_GOAT_API_KEY_AUTH env var (paid AeroAPI key)"},
				"caveat":           "Airlines don't assign a specific tail to a flight until ~48–72 hours before departure. For shopping or planning >72 hours out, use the model-level lookup with the equipment type from the booking site instead.",
				"fallback_command": "airframe-pp-cli model \"<Boeing 737 MAX 8 | Airbus A321neo | etc.>\"",
			},
		},
		"common_questions": []map[string]string{
			{"q": "Who owns this tail?", "tier": "1", "command": "tail <N-number>"},
			{"q": "How safe is this aircraft model?", "tier": "2", "command": "model \"<make> [model]\""},
			{"q": "What aircraft does this company own?", "tier": "1", "command": "owner \"<name>\""},
			{"q": "What was NTSB event ERA22LA001?", "tier": "2", "command": "event <event-id>"},
			{"q": "Is my flight today safe?", "tier": "1 + flight-goat (≤72h)", "command": "flight <ident>"},
			{"q": "Find aviation accidents matching a keyword", "tier": "2 + FTS", "command": "search \"<query>\""},
			{"q": "What's installed and ready?", "tier": "—", "command": "doctor"},
		},
		"caveats": []string{
			"FAA owner data may be a Delaware LLC shell — airframe reports the registry value verbatim, it is not a beneficial-ownership tool.",
			"NTSB only investigates incidents meeting their reporting threshold — \"no events\" is not \"perfect record.\"",
			"event_aircraft.make_model_code is NULL for NTSB-origin rows in v1; `model` aggregation joins through the FAA registry, undercounting foreign tails.",
		},
		"json_envelope": map[string]any{
			"shape":          map[string]string{"meta": "{source, synced_at, db_path, query, row_count?, truncated?}", "results": "command-specific payload"},
			"select_example": "airframe-pp-cli tail N628TS --select results.aircraft.owner_name → \"FALCON LANDING LLC\"",
		},
	}
	b, err := json.MarshalIndent(ctx, "", "  ")
	if err != nil {
		return mcplib.NewToolResultError("failed to marshal context: " + err.Error()), nil
	}
	return mcplib.NewToolResultText(string(b)), nil
}
