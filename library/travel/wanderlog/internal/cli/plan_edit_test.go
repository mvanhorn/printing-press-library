// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"strings"
	"testing"
	"time"
)

func TestPlanEditSectionReportsAndResolveDay(t *testing.T) {
	trip := testPlanTrip("Edit target")
	reports := sectionReports(trip)
	if len(reports) != 4 {
		t.Fatalf("len(reports) = %d", len(reports))
	}
	if reports[0].Day != 1 || reports[1].Day != 2 || reports[2].Day != 3 || reports[3].Day != 3 {
		t.Fatalf("unexpected day numbering: %#v", reports)
	}
	sec, err := resolveSection(trip, 2, -1, 0)
	if err != nil {
		t.Fatalf("resolveSection day 2: %v", err)
	}
	if sec.Index != 1 || sec.Report.Date != "2026-08-31" || len(sec.Blocks) != 1 {
		t.Fatalf("resolved section = %#v", sec)
	}
}

func TestPlanEditResolveBlockByIDAndIndex(t *testing.T) {
	trip := testPlanTrip("Edit target")
	sec, block, idx, err := resolveBlock(trip, 1, -1, 0, 2002, -1)
	if err != nil {
		t.Fatalf("resolveBlock by id: %v", err)
	}
	if sec.Index != 0 || idx != 1 || stringField(block, "type") != "note" {
		t.Fatalf("resolved block = sec %#v idx %d block %#v", sec.Report, idx, block)
	}
	_, block, idx, err = resolveBlock(trip, 2, -1, 0, 0, 0)
	if err != nil {
		t.Fatalf("resolveBlock by index: %v", err)
	}
	if idx != 0 || intAny(block["id"]) != 2003 {
		t.Fatalf("resolved block by index = idx %d block %#v", idx, block)
	}
}

func TestPlanEditNewNoteBlockShape(t *testing.T) {
	block := newNoteBlock("Book ferry tickets")
	if stringField(block, "type") != "note" || intAny(block["id"]) == 0 {
		t.Fatalf("bad note block identity: %#v", block)
	}
	if _, ok := block["attachments"].([]any); !ok {
		t.Fatalf("attachments missing or wrong type: %#v", block["attachments"])
	}
	addedBy := mapField(block, "addedBy")
	if stringField(addedBy, "type") != "user" {
		t.Fatalf("addedBy = %#v", addedBy)
	}
	if got := plainRichText(mapField(block, "text")); got != "Book ferry tickets" {
		t.Fatalf("plain text = %q", got)
	}
}

func TestPlanEditPlaceClosedOnDateWarningFromPeriods(t *testing.T) {
	place := map[string]any{
		"name": "Yunangi",
		"opening_hours": map[string]any{"periods": []any{
			map[string]any{
				"open":  map[string]any{"day": 1, "time": "1100"},
				"close": map[string]any{"day": 1, "time": "2200"},
			},
		}},
	}
	warning, closed := placeClosedOnDateWarning(place, "2026-08-30")
	if !closed {
		t.Fatalf("closed = false, warning = %q", warning)
	}
	if !strings.Contains(warning, "Yunangi appears closed on Sunday 2026-08-30") {
		t.Fatalf("warning = %q", warning)
	}
}

func TestPlanEditPlaceClosedOnDateWarningAllowsOpenDay(t *testing.T) {
	place := map[string]any{
		"name": "Naha Cafe",
		"opening_hours": map[string]any{"periods": []any{
			map[string]any{
				"open":  map[string]any{"day": 0, "time": "0900"},
				"close": map[string]any{"day": 0, "time": "1800"},
			},
		}},
	}
	warning, closed := placeClosedOnDateWarning(place, "2026-08-30")
	if closed || warning != "" {
		t.Fatalf("closed = %v, warning = %q", closed, warning)
	}
}

func TestPlanEditPlaceClosedOnDateWarningFromWeekdayText(t *testing.T) {
	place := map[string]any{
		"name": "Fallback Restaurant",
		"opening_hours": map[string]any{"weekday_text": []any{
			"Monday: 11:00 AM - 9:00 PM",
			"Tuesday: 11:00 AM - 9:00 PM",
			"Wednesday: 11:00 AM - 9:00 PM",
			"Thursday: 11:00 AM - 9:00 PM",
			"Friday: 11:00 AM - 9:00 PM",
			"Saturday: 11:00 AM - 9:00 PM",
			"Sunday: Closed",
		}},
	}
	warning, closed := placeClosedOnDateWarning(place, "2026-08-30")
	if !closed {
		t.Fatalf("closed = false, warning = %q", warning)
	}
	if !strings.Contains(warning, "Fallback Restaurant appears closed on Sunday 2026-08-30") {
		t.Fatalf("warning = %q", warning)
	}
}

func TestPlanEditItineraryIssuesReportsClosedPlaceInDatedSection(t *testing.T) {
	trip := map[string]any{"itinerary": map[string]any{"sections": []any{
		map[string]any{
			"id":   77,
			"mode": "dayPlan",
			"date": "2026-08-30",
			"blocks": []any{map[string]any{
				"id":   123,
				"type": "place",
				"place": map[string]any{
					"name":     "Yunangi",
					"place_id": "closed-place",
					"opening_hours": map[string]any{"periods": []any{
						map[string]any{
							"open":  map[string]any{"day": 1, "time": "1100"},
							"close": map[string]any{"day": 1, "time": "2200"},
						},
					}},
				},
			}},
		},
	}}}
	issues := itineraryIssues(trip)
	if len(issues) != 1 {
		t.Fatalf("issues = %#v", issues)
	}
	issue := issues[0]
	if issue.Code != "place_closed_on_section_date" || issue.Severity != "error" {
		t.Fatalf("issue = %#v", issue)
	}
	if issue.Day != 1 || issue.Date != "2026-08-30" || issue.SectionID != 77 || issue.BlockID != 123 || issue.PlaceName != "Yunangi" {
		t.Fatalf("issue target = %#v", issue)
	}
}

func TestPlanEditSameSectionMoveAdjustsForwardPosition(t *testing.T) {
	trip := testPlanTrip("Edit target")
	fromSec, block, fromIdx, err := resolveBlock(trip, 1, -1, 0, 2001, -1)
	if err != nil {
		t.Fatalf("resolveBlock: %v", err)
	}
	toSec, err := resolveSection(trip, 1, -1, 0)
	if err != nil {
		t.Fatalf("resolveSection: %v", err)
	}
	toIdx := normalizeInsertPosition(2, len(toSec.Blocks))
	if fromSec.Index == toSec.Index && toIdx > fromIdx {
		toIdx--
	}
	ops := []map[string]any{
		{"p": []any{"itinerary", "sections", fromSec.Index, "blocks", fromIdx}, "ld": block},
		{"p": []any{"itinerary", "sections", toSec.Index, "blocks", toIdx}, "li": block},
	}
	paths := opPaths(ops)
	if toIdx != 1 {
		t.Fatalf("toIdx = %d, want 1", toIdx)
	}
	if paths[0] != "itinerary.sections.0.blocks.0" || paths[1] != "itinerary.sections.0.blocks.1" {
		t.Fatalf("paths = %#v", paths)
	}
}

func TestPlanEditParseFieldMutationTextAndJSON(t *testing.T) {
	path, value, err := parseFieldMutation("text", "Updated note", "", false)
	if err != nil {
		t.Fatalf("parse text mutation: %v", err)
	}
	if len(path) != 1 || path[0] != "text" {
		t.Fatalf("path = %#v", path)
	}
	if got := plainRichText(value.(map[string]any)); got != "Updated note" {
		t.Fatalf("rich text = %q", got)
	}

	path, value, err = parseFieldMutation("reservation.durationMinutes", "", "90", false)
	if err != nil {
		t.Fatalf("parse json mutation: %v", err)
	}
	if len(path) != 2 || path[0] != "reservation" || path[1] != "durationMinutes" {
		t.Fatalf("json path = %#v", path)
	}
	if got, ok := value.(float64); !ok || got != 90 {
		t.Fatalf("json value = %#v", value)
	}
}

func TestPlanEditObjectSetOpAddReplaceRemove(t *testing.T) {
	add := objectSetOp([]any{"itinerary", "title"}, nil, false, "New", false)
	if _, ok := add["od"]; ok || add["oi"] != "New" {
		t.Fatalf("add op = %#v", add)
	}
	replace := objectSetOp([]any{"itinerary", "title"}, "Old", true, "New", false)
	if replace["od"] != "Old" || replace["oi"] != "New" {
		t.Fatalf("replace op = %#v", replace)
	}
	remove := objectSetOp([]any{"itinerary", "title"}, "Old", true, nil, true)
	if remove["od"] != "Old" {
		t.Fatalf("remove op missing old value: %#v", remove)
	}
	if _, ok := remove["oi"]; ok {
		t.Fatalf("remove op has insert value: %#v", remove)
	}
}

func TestPlanEditDayDatesExcludingSection(t *testing.T) {
	trip := testPlanTrip("Edit target")
	dates := dayDatesExcludingSection(trip, 0)
	want := []string{"2026-08-31", "2026-09-01"}
	if len(dates) != len(want) {
		t.Fatalf("dates = %#v", dates)
	}
	for i := range want {
		if dates[i] != want[i] {
			t.Fatalf("dates = %#v", dates)
		}
	}
}

func TestPlanCollabNewChecklistBlockShape(t *testing.T) {
	block := newChecklistBlock("Packing", []string{"Passport", "Sunscreen"})
	if stringField(block, "type") != "checklist" || intAny(block["id"]) == 0 {
		t.Fatalf("bad checklist identity: %#v", block)
	}
	items, ok := block["items"].([]any)
	if !ok || len(items) != 2 {
		t.Fatalf("items = %#v", block["items"])
	}
	first, _ := items[0].(map[string]any)
	if got := plainRichText(mapField(first, "text")); got != "Passport" {
		t.Fatalf("first item text = %q", got)
	}
	if first["checked"] != false || intAny(first["id"]) == 0 {
		t.Fatalf("first item = %#v", first)
	}
}

func TestPlanCollabRouteBodyForSectionFixtureAndLiveShape(t *testing.T) {
	trip := testPlanTrip("Route target")
	sec, err := resolveSection(trip, 1, -1, 0)
	if err != nil {
		t.Fatalf("resolveSection: %v", err)
	}
	body := routeBodyForSection(sec, "WALKING")
	places, _ := body["places"].([]any)
	if body["travelMode"] != "WALKING" || len(places) != 1 {
		t.Fatalf("body = %#v", body)
	}
	fixtureStop, _ := places[0].(map[string]any)
	if fixtureStop["id"] != "okinawa-1" || fixtureStop["blockId"] != 2001 {
		t.Fatalf("fixture stop = %#v", fixtureStop)
	}

	liveSec := resolvedSection{Blocks: []any{map[string]any{
		"id":   3001,
		"type": "place",
		"place": map[string]any{
			"place_id": "live-place",
			"geometry": map[string]any{"location": map[string]any{"lat": 1.25, "lng": 2.5}},
		},
	}}}
	body = routeBodyForSection(liveSec, "")
	places, _ = body["places"].([]any)
	liveStop, _ := places[0].(map[string]any)
	if body["travelMode"] != "DRIVING" || liveStop["id"] != "live-place" || liveStop["latitude"] != 1.25 || liveStop["longitude"] != 2.5 {
		t.Fatalf("live stop body = %#v", body)
	}
}

func TestPlanCollabValidClock(t *testing.T) {
	for _, value := range []string{"00:00", "09:30", "23:59"} {
		if !validClock(value) {
			t.Fatalf("validClock(%q) = false", value)
		}
	}
	for _, value := range []string{"9:30", "24:00", "12:60", "12345"} {
		if validClock(value) {
			t.Fatalf("validClock(%q) = true", value)
		}
	}
}

func TestPlanHistoryInvertJSON0Ops(t *testing.T) {
	block := map[string]any{"id": 99, "type": "note"}
	ops := []map[string]any{
		{"p": []any{"itinerary", "sections", 0, "blocks", 1}, "li": block},
		{"p": []any{"title"}, "od": "Old", "oi": "New"},
		{"p": []any{"obsolete"}, "od": true},
	}
	inv, err := invertJSON0Ops(ops)
	if err != nil {
		t.Fatalf("invertJSON0Ops: %v", err)
	}
	if len(inv) != 3 {
		t.Fatalf("len(inv) = %d", len(inv))
	}
	if inv[0]["oi"] != true {
		t.Fatalf("remove inverse = %#v", inv[0])
	}
	if inv[1]["od"] != "New" || inv[1]["oi"] != "Old" {
		t.Fatalf("replace inverse = %#v", inv[1])
	}
	if _, ok := inv[2]["ld"]; !ok {
		t.Fatalf("list insert inverse missing ld: %#v", inv[2])
	}
}

func TestPlanHistoryPickJournalRecord(t *testing.T) {
	now := testNow()
	recs := []planEditJournalRecord{
		{ID: "a", TargetKey: "one", Status: "applied", CreatedAt: now},
		{ID: "b", TargetKey: "one", Status: "undone", CreatedAt: now.Add(time.Second)},
		{ID: "c", TargetKey: "one", Status: "applied", CreatedAt: now.Add(2 * time.Second)},
	}
	idx, rec, err := pickJournalRecord(recs, "one", "", "undo")
	if err != nil {
		t.Fatalf("pick undo: %v", err)
	}
	if idx != 2 || rec.ID != "c" {
		t.Fatalf("undo picked idx %d rec %#v", idx, rec)
	}
	idx, rec, err = pickJournalRecord(recs, "one", "", "redo")
	if err != nil {
		t.Fatalf("pick redo: %v", err)
	}
	if idx != 1 || rec.ID != "b" {
		t.Fatalf("redo picked idx %d rec %#v", idx, rec)
	}
}

func testNow() time.Time {
	return time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC)
}

func TestCompileMarkdownDeltaBoldBulletsHeading(t *testing.T) {
	delta, stripped := compileMarkdownDelta("# Stay\n- **Pool**\n* snack bar\nplain **x**")
	if len(stripped) != 0 {
		t.Fatalf("stripped = %#v", stripped)
	}
	ops, _ := delta["ops"].([]any)
	if len(ops) != 9 {
		t.Fatalf("len(ops) = %d, ops = %#v", len(ops), ops)
	}
	assertDeltaOp(t, ops[0], "Stay", map[string]any{"bold": true})
	assertDeltaOp(t, ops[1], "\n", nil)
	assertDeltaOp(t, ops[2], "Pool", map[string]any{"bold": true})
	assertDeltaOp(t, ops[3], "\n", map[string]any{"list": "bullet"})
	assertDeltaOp(t, ops[4], "snack bar", nil)
	assertDeltaOp(t, ops[5], "\n", map[string]any{"list": "bullet"})
	assertDeltaOp(t, ops[6], "plain ", nil)
	assertDeltaOp(t, ops[7], "x", map[string]any{"bold": true})
	assertDeltaOp(t, ops[8], "\n", nil)
	delta, stripped = compileMarkdownDelta("plain **x**")
	if len(stripped) != 0 {
		t.Fatalf("stripped = %#v", stripped)
	}
	ops, _ = delta["ops"].([]any)
	if len(ops) != 3 {
		t.Fatalf("plain bold ops = %#v", ops)
	}
	assertDeltaOp(t, ops[0], "plain ", nil)
	assertDeltaOp(t, ops[1], "x", map[string]any{"bold": true})
	assertDeltaOp(t, ops[2], "\n", nil)
}

func TestCompileMarkdownDeltaNeverEmitsHeaderAttributes(t *testing.T) {
	delta, stripped := compileMarkdownDelta("# Label\n## Nested")
	if len(stripped) != 0 {
		t.Fatalf("stripped = %#v, want empty because compiler must not emit header attributes", stripped)
	}
	ops, _ := delta["ops"].([]any)
	if len(ops) != 4 {
		t.Fatalf("ops = %#v", ops)
	}
	assertDeltaOp(t, ops[0], "Label", map[string]any{"bold": true})
	assertDeltaOp(t, ops[1], "\n", nil)
	assertDeltaOp(t, ops[2], "Nested", map[string]any{"bold": true})
	assertDeltaOp(t, ops[3], "\n", nil)
	for i, raw := range ops {
		if _, ok := mapField(raw.(map[string]any), "attributes")["header"]; ok {
			t.Fatalf("op %d emitted header attribute: %#v", i, raw)
		}
	}
}

func TestStripDeltaHeaderAttributesReportsStripped(t *testing.T) {
	ops := []any{
		map[string]any{"insert": "Title", "attributes": map[string]any{"header": 1, "bold": true}},
		map[string]any{"insert": "\n", "attributes": map[string]any{"header": 2}},
	}
	got, stripped := stripDeltaHeaderAttributes(ops)
	if len(stripped) != 1 || stripped[0] != "header" {
		t.Fatalf("stripped = %#v", stripped)
	}
	assertDeltaOp(t, got[0], "Title", map[string]any{"bold": true})
	assertDeltaOp(t, got[1], "\n", nil)
}

func TestBlockNoteTextFailClosedWithoutMarkdown(t *testing.T) {
	cases := []string{"**bold**", "- item", "* item", "# Heading", "  - indented"}
	for _, text := range cases {
		_, _, err := blockNoteText(text, false)
		if err == nil || !strings.Contains(err.Error(), "--markdown") {
			t.Fatalf("blockNoteText(%q, false) err = %v", text, err)
		}
	}
	delta, stripped, err := blockNoteText("plain note", false)
	if err != nil {
		t.Fatalf("plain text: %v", err)
	}
	if len(stripped) != 0 {
		t.Fatalf("stripped = %#v", stripped)
	}
	if got := plainRichText(delta); got != "plain note" {
		t.Fatalf("plain = %q", got)
	}
	if got, want := plainRichText(delta), plainRichText(richText("plain note")); got != want {
		t.Fatalf("plain delta %q != richText %q", got, want)
	}
}

func TestBlockNoteTextMarkdownCompiles(t *testing.T) {
	delta, stripped, err := blockNoteText("# Stay\n- one", true)
	if err != nil {
		t.Fatalf("markdown: %v", err)
	}
	if len(stripped) != 0 {
		t.Fatalf("stripped = %#v", stripped)
	}
	ops, _ := delta["ops"].([]any)
	if len(ops) != 4 {
		t.Fatalf("ops = %#v", ops)
	}
	assertDeltaOp(t, ops[0], "Stay", map[string]any{"bold": true})
	assertDeltaOp(t, ops[3], "\n", map[string]any{"list": "bullet"})
}

func TestPlanBlockRenameWritesPlaceName(t *testing.T) {
	trip := map[string]any{"itinerary": map[string]any{"sections": []any{
		map[string]any{
			"id":   1,
			"mode": "dayPlan",
			"date": "2026-08-30",
			"blocks": []any{map[string]any{
				"id":    9,
				"type":  "place",
				"place": map[string]any{"name": "Old Inn", "place_id": "x"},
			}},
		},
	}}}
	opts := planEditOptions{day: 1, sectionIndex: -1, blockID: 9, blockIndex: -1}
	result, err := buildPlanBlockRename(trip, opts, "Property")
	if err != nil {
		t.Fatalf("buildPlanBlockRename: %v", err)
	}
	if len(result.Ops) != 1 || result.Report.OpPaths[0] != "itinerary.sections.0.blocks.0.place.name" {
		t.Fatalf("ops = %#v paths = %#v", result.Ops, result.Report.OpPaths)
	}
	if result.Ops[0]["od"] != "Old Inn" || result.Ops[0]["oi"] != "Property" {
		t.Fatalf("op = %#v", result.Ops[0])
	}
	if result.Report.Operation != "rename place" || stringField(result.Report.Block, "place_name") != "Property" {
		t.Fatalf("report = %#v", result.Report)
	}
}

func TestPlanBlockRenameRequiresPlaceAndName(t *testing.T) {
	trip := testPlanTrip("Edit target")
	opts := planEditOptions{day: 1, sectionIndex: -1, blockID: 2002, blockIndex: -1}
	if _, err := buildPlanBlockRename(trip, opts, "Property"); err == nil || !strings.Contains(err.Error(), "no place") {
		t.Fatalf("note block rename err = %v", err)
	}
	placeTrip := map[string]any{"itinerary": map[string]any{"sections": []any{
		map[string]any{
			"id":     1,
			"mode":   "dayPlan",
			"date":   "2026-08-30",
			"blocks": []any{map[string]any{"id": 9, "type": "place", "place": map[string]any{"name": "Inn"}}},
		},
	}}}
	placeOpts := planEditOptions{day: 1, sectionIndex: -1, blockID: 9, blockIndex: -1}
	if _, err := buildPlanBlockRename(placeTrip, placeOpts, "  "); err == nil || !strings.Contains(err.Error(), "--name") {
		t.Fatalf("empty name err = %v", err)
	}
}

func assertDeltaOp(t *testing.T, raw any, insert string, attrs map[string]any) {
	t.Helper()
	op, _ := raw.(map[string]any)
	if stringAny(op["insert"]) != insert {
		t.Fatalf("insert = %#v want %q in %#v", op["insert"], insert, op)
	}
	got := mapField(op, "attributes")
	if attrs == nil {
		if len(got) != 0 {
			t.Fatalf("unexpected attributes %#v", got)
		}
		return
	}
	if len(got) != len(attrs) {
		t.Fatalf("attributes = %#v want %#v", got, attrs)
	}
	for k, v := range attrs {
		if got[k] != v {
			t.Fatalf("attr %s = %#v want %#v in %#v", k, got[k], v, op)
		}
	}
}
