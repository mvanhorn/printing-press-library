// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"encoding/json"
	"testing"
)

func TestPlanPreviewReportCountsSharedPlanShape(t *testing.T) {
	source := testPlanTrip("Okinawa template")
	source["_resources"] = map[string]json.RawMessage{
		"geos": json.RawMessage(`[{"id":86696,"name":"Okinawa Prefecture"}]`),
		"tips": json.RawMessage(`[]`),
	}

	report := buildPlanReport("plan preview", planCopyOptions{sourceURL: "https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared"}, source, "naertjcoixqrgrfc", nil)

	if report.SourceKey != "naertjcoixqrgrfc" {
		t.Fatalf("SourceKey = %q", report.SourceKey)
	}
	if report.SourceTitle != "Okinawa template" || report.Title != "Okinawa template (copy)" {
		t.Fatalf("unexpected titles: source=%q title=%q", report.SourceTitle, report.Title)
	}
	if report.GeoID != 86696 {
		t.Fatalf("GeoID = %d", report.GeoID)
	}
	if report.StartDate != "2026-08-30" || report.EndDate != "2026-09-01" || report.Days != 3 {
		t.Fatalf("date summary = start %q end %q days %d", report.StartDate, report.EndDate, report.Days)
	}
	if report.Sections != 4 || report.DaySections != 3 || report.Blocks != 3 || report.PlaceBlocks != 2 || report.NoteBlocks != 1 {
		t.Fatalf("counts = sections %d days %d blocks %d place %d note %d", report.Sections, report.DaySections, report.Blocks, report.PlaceBlocks, report.NoteBlocks)
	}
	if len(report.Resources) != 2 || report.Resources[0] != "geos" || report.Resources[1] != "tips" {
		t.Fatalf("resources = %#v", report.Resources)
	}
	if len(report.Warnings) != 0 {
		t.Fatalf("warnings = %#v", report.Warnings)
	}
}

func testPlanTrip(title string) map[string]any {
	return map[string]any{
		"title":     title,
		"startDate": "2026-08-30",
		"endDate":   "2026-09-01",
		"days":      3,
		"itinerary": map[string]any{
			"sections": []any{
				map[string]any{
					"id":   1001,
					"mode": "dayPlan",
					"date": "2026-08-30",
					"blocks": []any{
						map[string]any{"id": 2001, "type": "place", "placeId": "okinawa-1", "addedBy": map[string]any{"type": "user", "userId": 42}},
						map[string]any{"id": 2002, "type": "note", "text": "arrive"},
					},
				},
				map[string]any{
					"id":     1002,
					"mode":   "dayPlan",
					"date":   "2026-08-31",
					"blocks": []any{map[string]any{"id": 2003, "type": "place", "placeId": "okinawa-2"}},
				},
				map[string]any{"id": 1003, "mode": "dayPlan", "date": "2026-09-01", "blocks": []any{}},
				map[string]any{"id": 1004, "mode": "notes", "blocks": []any{}},
			},
		},
	}
}
