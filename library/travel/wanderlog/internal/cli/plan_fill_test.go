// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import "testing"

func TestPlanFillBuildsReplaceItineraryOps(t *testing.T) {
	source := testPlanTrip("Source trip")
	target := map[string]any{
		"startDate": "2026-07-01",
		"endDate":   "2026-07-02",
		"days":      2,
		"itinerary": map[string]any{
			"sections": []any{map[string]any{"id": 9001, "mode": "dayPlan", "date": "2026-07-01", "blocks": []any{}}},
		},
	}

	ops := buildFillOps(source, target)
	if len(ops) != 4 {
		t.Fatalf("len(ops) = %d, want 4: %#v", len(ops), ops)
	}
	if got := ops[0]["p"].([]any)[0]; got != "itinerary" {
		t.Fatalf("first op path = %#v", ops[0]["p"])
	}
	replacement := ops[0]["oi"].(map[string]any)
	sections := replacement["sections"].([]any)
	if len(sections) != 4 {
		t.Fatalf("replacement sections = %d", len(sections))
	}
	firstSection := sections[0].(map[string]any)
	if firstSection["id"] == float64(1001) || firstSection["id"] == 1001 {
		t.Fatalf("section id was not regenerated: %#v", firstSection["id"])
	}
	blocks := firstSection["blocks"].([]any)
	firstBlock := blocks[0].(map[string]any)
	if firstBlock["id"] == float64(2001) || firstBlock["id"] == 2001 {
		t.Fatalf("block id was not regenerated: %#v", firstBlock["id"])
	}
	addedBy := firstBlock["addedBy"].(map[string]any)
	if addedBy["type"] != "user" {
		t.Fatalf("addedBy.type = %#v", addedBy["type"])
	}
	if _, ok := addedBy["userId"]; ok {
		t.Fatalf("copied source userId into replacement: %#v", addedBy)
	}
	if ops[1]["oi"] != "2026-08-30" || ops[2]["oi"] != "2026-09-01" || ops[3]["oi"] != 3 {
		t.Fatalf("date/day ops = %#v", ops[1:])
	}
}
