// Copyright 2026 Jet Sng and contributors. Licensed under Apache-2.0.
package mcp

import "github.com/mvanhorn/printing-press-library/library/travel/wheelog/internal/wheelog"

// wheelogContextEvidence replaces generic endpoint assumptions with the
// public source's bounded command contract.
func wheelogContextEvidence(value map[string]any) {
	value["tool_surface"] = "MCP exposes command mirrors through the companion wheelog-pp-cli binary; raw typed endpoint tools are hidden. Use the listed tool schemas and exact hyphenated flag names."
	value["query_tips"] = []string{
		"spots_search accepts query or a keyword argument; use --category with exact values from categories.",
		"Use --max-scan-pages (1..5, default 1) to bound source page scanning. --limit controls returned records (1..50, default 5); it is not an upstream page-size parameter.",
		"--details or --require-question fetch actual details for at most 5 candidates. spots_compare accepts 1..5 source IDs.",
		"--from and --to select inclusive source record dates in --timezone (default Asia/Tokyo), converted to UTC; they do not select individual report dates or trip availability.",
		"shortlist_list reads saved facilities only; --audit returns recheck reasons and --origin ranks by straight-line meters without establishing an accessible route.",
		"auto prefers live with labeled saved fallback; live requires source reads; local uses saved public-place observations.",
	}
	value["source_limits"] = map[string]any{"detail_reads": wheelog.MaxDetailReads, "search_scan_pages": 5, "search_output_records": 50, "saved_spots": wheelog.MaxSpots, "saved_observations_per_spot": 2}
	value["evidence_note"] = "Aggregate question reports are crowdsourced evidence, not a guarantee of accessibility or current conditions. Record and retrieval dates do not date individual reports. Contributor profiles and personal histories are excluded."
}
