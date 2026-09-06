// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func dayFixture() map[string]any {
	trip := blockReadFixture()
	first := sections(trip)[0].(map[string]any)
	copyBlock := cloneJSONMap(first["blocks"].([]any)[0].(map[string]any))
	copyBlock["id"] = 103
	copyBlock["text"] = map[string]any{"ops": []any{map[string]any{"insert": "Remember the evening booking."}}}
	copyBlock["place"].(map[string]any)["name"] = "Evening display name"
	first["blocks"] = append(first["blocks"].([]any), copyBlock)
	first["text"] = map[string]any{"ops": []any{map[string]any{"insert": "Day constraints: meet by 08:30."}}}
	flight := map[string]any{"id": 201, "type": "flight", "confirmationNumber": "SYNTHETIC", "travelerNames": []any{"Example traveler"}, "depart": map[string]any{"date": "2027-01-01", "time": "06:00", "airport": map[string]any{"iata": "AAA", "googlePlace": map[string]any{"place_id": "airport-a", "name": "Airport A", "photos": []any{"huge photo"}}}}, "arrive": map[string]any{"date": "2027-01-01", "time": "08:00", "airport": map[string]any{"iata": "BBB"}}, "text": map[string]any{"ops": []any{map[string]any{"insert": "Check-in closes 45 minutes before departure."}}}}
	trip["itinerary"].(map[string]any)["sections"] = append(sections(trip), map[string]any{"id": 12, "mode": "list", "blocks": []any{flight}})
	trip["_resources"] = map[string]json.RawMessage{"placeMetadata": json.RawMessage(`[{"placeId":"synthetic-place","minMinutesSpent":30,"maxMinutesSpent":90}]`)}
	return trip
}

func dayStateForTest(t *testing.T, trip map[string]any) planDayState {
	t.Helper()
	snapshot, err := buildPlanDay(trip, "abcdefghijklmnop", 1, []string{"walking"}, "")
	if err != nil {
		t.Fatal(err)
	}
	state, err := makePlanDayState("abcdefghijklmnop", planDayQuery{Day: 1, Modes: []string{"walking"}, ClientSchemaVersion: 2}, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func TestPlanDayRetainsNotesReservationsAndDeduplicatesMetadata(t *testing.T) {
	state := dayStateForTest(t, dayFixture())
	blocks := mapField(state.Snapshot, "blocks")
	a, b := blocks["101"].(map[string]any), blocks["103"].(map[string]any)
	if a["place_ref"] != b["place_ref"] {
		t.Fatal("identical place metadata not shared")
	}
	if a["name"] == b["name"] {
		t.Fatal("per-stop overridden labels collapsed")
	}
	if !strings.Contains(a["text"].(string), strings.Repeat("A long complete note. ", 79)) {
		t.Fatal("day note truncated")
	}
	flight := blocks["201"].(map[string]any)
	if flight["confirmationNumber"] != "SYNTHETIC" || flight["travelerNames"] == nil || !strings.Contains(flight["text"].(string), "45 minutes") {
		t.Fatal("booking constraints missing", flight)
	}
	airport := mapField(mapField(flight, "depart"), "airport")
	if airport["iata"] != "AAA" || airport["googlePlace_ref"] == nil || airport["googlePlace_name"] != "Airport A" {
		t.Fatal("airport constraints lost", airport)
	}
	data, _ := json.Marshal(state.Snapshot)
	if strings.Contains(string(data), "huge photo") || strings.Contains(string(data), "unwantedunwanted") {
		t.Fatal("irrelevant media exported")
	}
	if !strings.Contains(string(data), "minMinutesSpent") || !strings.Contains(string(data), "meet by 08:30") {
		t.Fatal("planning metadata/section text omitted")
	}
	if mapField(state.Snapshot, "travel")["mode_source"] != "unselected" {
		t.Fatal("mode inferred")
	}
}

func TestPlanDayUnequalPlaceMetadataNotMerged(t *testing.T) {
	trip := dayFixture()
	first := sections(trip)[0].(map[string]any)
	second := first["blocks"].([]any)[1].(map[string]any)
	second["place"].(map[string]any)["business_status"] = "CLOSED_TEMPORARILY"
	blocks := mapField(dayStateForTest(t, trip).Snapshot, "blocks")
	if blocks["101"].(map[string]any)["place_ref"] == blocks["103"].(map[string]any)["place_ref"] {
		t.Fatal("unequal metadata merged")
	}
}

func TestPlanDayDeltaChangedDeletedAndOrder(t *testing.T) {
	trip := dayFixture()
	previous := dayStateForTest(t, trip)
	first := sections(trip)[0].(map[string]any)
	rows := first["blocks"].([]any)
	rows[0].(map[string]any)["endTime"] = "11:00"
	first["blocks"] = []any{rows[0]}
	current := dayStateForTest(t, trip)
	current.Snapshot["warnings"] = []string{"Always preserve this warning"}
	current, _ = makePlanDayState(current.TargetKey, current.Query, current.Snapshot)
	delta := planDayResponse(current, &previous, "")
	if delta["mode"] != "delta" || mapField(delta, "changed_blocks")["101"] == nil {
		t.Fatal(delta)
	}
	deleted := delta["deleted_block_ids"].([]int)
	if len(deleted) != 1 || deleted[0] != 103 {
		t.Fatal(deleted)
	}
	if delta["order"].([]int)[0] != 101 || delta["warnings"].([]string)[0] != "Always preserve this warning" {
		t.Fatal("delta dropped order/warnings")
	}
	if mapField(delta, "changed_components")["travel"] == nil {
		t.Fatal("travel changes omitted")
	}
	// Reordering without edits must communicate the complete new order.
	trip = dayFixture()
	previous = dayStateForTest(t, trip)
	first = sections(trip)[0].(map[string]any)
	rows = first["blocks"].([]any)
	first["blocks"] = []any{rows[1], rows[0]}
	current = dayStateForTest(t, trip)
	delta = planDayResponse(current, &previous, "")
	if delta["mode"] != "delta" || delta["order"].([]int)[0] != 103 {
		t.Fatal("reorder lost", delta)
	}
}

func TestPlanDayStateFallbackAndPrivateAtomicPersistence(t *testing.T) {
	state := dayStateForTest(t, dayFixture())
	path := filepath.Join(t.TempDir(), "private-state.json")
	if err := savePlanDayState(path, state); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("private state mode: %v %v", info, err)
	}
	previous, reason := readPlanDayState(path)
	if reason != "" || previous == nil {
		t.Fatal(reason)
	}
	if got := planDayResponse(state, previous, ""); got["mode"] != "delta" || len(mapField(got, "changed_blocks")) != 0 {
		t.Fatal("round-trip should be empty delta", got)
	}
	previous.Query.SelectedMode = "walking"
	if got := planDayResponse(state, previous, ""); got["mode"] != "full" || got["state_warning"] == nil {
		t.Fatal("query mismatch not full", got)
	}
	previous.Query = state.Query
	previous.TargetKey = "ponmlkjihgfedcba"
	if planDayResponse(state, previous, "")["mode"] != "full" {
		t.Fatal("target mismatch not full")
	}
	if err := os.WriteFile(path, []byte(`{"schema_version":1,"digest":"bogus","snapshot":{}}`), 0600); err != nil {
		t.Fatal(err)
	}
	previous, reason = readPlanDayState(path)
	if previous != nil || reason != "previous_state_invalid" || planDayResponse(state, previous, reason)["mode"] != "full" {
		t.Fatal("invalid state not full")
	}
	if err := savePlanDayState(path, state); err != nil {
		t.Fatal("atomic replacement failed", err)
	}
	files, _ := os.ReadDir(filepath.Dir(path))
	if len(files) != 1 {
		t.Fatal("temporary state leaked", files)
	}
}

func TestPlanDayDuplicateIDsFallBackAndNoArgsNeverSave(t *testing.T) {
	trip := dayFixture()
	previous := dayStateForTest(t, trip)
	rows := sections(trip)[0].(map[string]any)["blocks"].([]any)
	rows[1].(map[string]any)["id"] = 101
	current := dayStateForTest(t, trip)
	if planDayResponse(current, &previous, "")["mode"] != "full" {
		t.Fatal("duplicate IDs yielded unsafe delta")
	}
	if len(current.Snapshot["order_keys"].([]string)) != 2 || len(mapField(current.Snapshot, "blocks")) != 3 {
		t.Fatal("duplicate full view lost content")
	}
	path := filepath.Join(t.TempDir(), "must-not-exist.json")
	cmd := newNovelPlanDayCmd(&rootFlags{})
	cmd.SetArgs([]string{"--save-state", path})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	if err := cmd.Execute(); err == nil {
		t.Fatal("missing day accepted")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("no-args path wrote state", err)
	}
}

func TestPlanDayGlobalConstraintsAndExactDeltaReconstruction(t *testing.T) {
	trip := dayFixture()
	global := sections(trip)[2].(map[string]any)
	global["text"] = map[string]any{"ops": []any{map[string]any{"insert": "Global booking rule: no car today."}}}
	global["blocks"] = append(global["blocks"].([]any), map[string]any{"id": 301, "type": "note", "text": "Always take passports."}, map[string]any{"id": 302, "type": "checklist", "items": []any{map[string]any{"id": 1, "text": "Collect room keys", "checked": false}}})
	previous := dayStateForTest(t, trip)
	if len(previous.Snapshot["context"].([]map[string]any)) != 1 || mapField(previous.Snapshot, "blocks")["301"] == nil {
		t.Fatal("global constraints omitted")
	}
	// Change shared metadata, edit global context, add, delete and reorder in one update.
	first := sections(trip)[0].(map[string]any)
	rows := first["blocks"].([]any)
	rows[1].(map[string]any)["place"].(map[string]any)["business_status"] = "CLOSED_TEMPORARILY"
	first["blocks"] = []any{map[string]any{"id": 401, "type": "note", "text": "New complete note"}, rows[1]}
	global["text"] = "Updated global booking constraint"
	global["blocks"].([]any)[1].(map[string]any)["text"] = "Passport and boarding pass."
	current := dayStateForTest(t, trip)
	delta := planDayResponse(current, &previous, "")
	reconstructed := cloneJSONMap(previous.Snapshot)
	blocks := mapField(reconstructed, "blocks")
	for _, id := range delta["deleted_block_ids"].([]int) {
		delete(blocks, strconv.Itoa(id))
	}
	for id, block := range mapField(delta, "changed_blocks") {
		blocks[id] = block
	}
	for key, value := range mapField(delta, "changed_components") {
		reconstructed[key] = value
	}
	for _, key := range []string{"order", "order_keys", "reservation_ids", "warnings"} {
		if value, present := delta[key]; present {
			reconstructed[key] = value
		}
	}
	digest, err := planDayDigest(reconstructed)
	if err != nil || digest != current.Digest {
		t.Fatalf("delta cannot reconstruct full snapshot: %s vs %s %v", digest, current.Digest, err)
	}
	if mapField(delta, "changed_components")["context"] == nil || mapField(delta, "changed_components")["places"] == nil {
		t.Fatal("shared/context changes omitted")
	}
}

func TestPlanDayStateRejectsProjectedOrSuppressedOutput(t *testing.T) {
	for _, flags := range []*rootFlags{{selectFields: "digest"}, {quiet: true}, {csv: true}, {plain: true}} {
		path := filepath.Join(t.TempDir(), "not-written.json")
		cmd := newNovelPlanDayCmd(flags)
		cmd.SetArgs([]string{"--day", "1", "--save-state", path})
		cmd.SilenceErrors = true
		cmd.SilenceUsage = true
		if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "complete JSON") {
			t.Fatalf("projection accepted in state mode: %v", err)
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatal("projection wrote state")
		}
	}
}

func TestPlanDayUndatedGuideChecksUseResolvedSection(t *testing.T) {
	note := func(id int, text string) any { return map[string]any{"id": id, "type": "note", "text": text} }
	closed := map[string]any{"id": 804, "type": "place", "place": map[string]any{"place_id": "closed-synthetic", "name": "Closed example", "opening_hours": map[string]any{"weekday_text": []any{"Closed", "Closed", "Closed", "Closed", "Closed", "Closed", "Closed"}}}}
	trip := map[string]any{"itinerary": map[string]any{"sections": []any{
		map[string]any{"id": 90, "mode": "list", "blocks": []any{note(800, "Global reminder")}},
		map[string]any{"id": 91, "mode": "guideDayPlan", "heading": "Undated first day", "blocks": []any{note(801, "Selected guide note")}},
		map[string]any{"id": 92, "mode": "list", "blocks": []any{note(802, "Global later list")}},
		map[string]any{"id": 93, "mode": "dayPlan", "date": "2030-01-02", "blocks": []any{note(803, "Dated second day"), closed}},
	}}}
	first, err := buildPlanDay(trip, "abcdefghijklmnop", 1, []string{"walking"}, "")
	if err != nil {
		t.Fatal("undated guide day rejected", err)
	}
	checks := mapField(first, "checks")
	ids := checks["unformatted_block_ids"].([]int)
	if len(ids) != 1 || ids[0] != 801 {
		t.Fatalf("checks selected wrong section: %v", ids)
	}
	if checks["calendar_checks_status"] != "unknown_date" || len(checks["closed_places"].([]planIssueReport)) != 0 {
		t.Fatal("undated checks claimed sibling closures", checks)
	}
	if mapField(first, "section")["id"] != 91 || first["order"].([]int)[0] != 801 {
		t.Fatal("blocks differ from checked section")
	}
	foundWarning := false
	for _, warning := range first["warnings"].([]string) {
		foundWarning = foundWarning || strings.Contains(warning, "has no date")
	}
	if !foundWarning {
		t.Fatal("missing unknown-date warning")
	}
	second, err := buildPlanDay(trip, "abcdefghijklmnop", 2, []string{"walking"}, "")
	if err != nil {
		t.Fatal(err)
	}
	secondChecks := mapField(second, "checks")
	ids = secondChecks["unformatted_block_ids"].([]int)
	if len(ids) != 1 || ids[0] != 803 {
		t.Fatalf("mixed preceding undated section shifted checks: %v", ids)
	}
	issues := secondChecks["closed_places"].([]planIssueReport)
	if len(issues) != 1 || issues[0].BlockID != 804 || issues[0].SectionIndex != 3 {
		t.Fatalf("wrong closure section: %#v", issues)
	}
}

func TestPlanDaySparseDeltaInheritsPersistedBaseline(t *testing.T) {
	previous := dayStateForTest(t, dayFixture())
	path := filepath.Join(t.TempDir(), "baseline.json")
	if err := savePlanDayState(path, previous); err != nil {
		t.Fatal(err)
	}
	loaded, reason := readPlanDayState(path)
	if reason != "" {
		t.Fatal(reason)
	}
	for _, change := range []string{"none", "note", "warnings-cleared"} {
		current := previous
		current.Snapshot = cloneJSONMap(previous.Snapshot)
		switch change {
		case "note":
			mapField(current.Snapshot, "blocks")["101"].(map[string]any)["text"] = "Complete new note"
		case "warnings-cleared":
			current.Snapshot["warnings"] = []string{}
		}
		current, _ = makePlanDayState(current.TargetKey, current.Query, current.Snapshot)
		delta := planDayResponse(current, loaded, "")
		if delta["mode"] != "delta" || delta["inheritance"] == nil {
			t.Fatal(delta)
		}
		for _, key := range []string{"order", "order_keys", "reservation_ids", "changed_components", "provenance"} {
			if _, ok := delta[key]; ok {
				t.Fatalf("%s repeated unchanged %s", change, key)
			}
		}
		if change != "warnings-cleared" && delta["warnings"] != nil {
			t.Fatal("repeated unchanged warnings")
		}
		reconstructed := cloneJSONMap(loaded.Snapshot)
		for key, value := range mapField(delta, "changed_blocks") {
			mapField(reconstructed, "blocks")[key] = value
		}
		if warnings, present := delta["warnings"]; present {
			reconstructed["warnings"] = warnings
		}
		digest, err := planDayDigest(reconstructed)
		if err != nil || digest != current.Digest {
			t.Fatalf("%s cannot reconstruct from disk baseline", change)
		}
	}
	loaded.Digest = "tampered"
	if got := planDayResponse(previous, loaded, ""); got["mode"] != "full" || got["state_warning"] != "previous_state_invalid" {
		t.Fatal(got)
	}
}

func TestPlanDayMalformedBaselineFallsBackFull(t *testing.T) {
	current := dayStateForTest(t, dayFixture())
	for _, change := range []string{"missing", "unknown", "wrong-type"} {
		bad := cloneJSONMap(current.Snapshot)
		switch change {
		case "missing":
			delete(bad, "warnings")
		case "unknown":
			bad["future_constraint"] = "must not silently lose"
		case "wrong-type":
			bad["context"] = "bad"
		}
		previous, _ := makePlanDayState(current.TargetKey, current.Query, bad)
		path := filepath.Join(t.TempDir(), "bad.json")
		if err := savePlanDayState(path, previous); err != nil {
			t.Fatal(err)
		}
		if state, reason := readPlanDayState(path); state != nil || reason != "previous_state_invalid" {
			t.Fatal(change, reason)
		}
		if got := planDayResponse(current, &previous, ""); got["mode"] != "full" {
			t.Fatal(change, got)
		}
	}
}
