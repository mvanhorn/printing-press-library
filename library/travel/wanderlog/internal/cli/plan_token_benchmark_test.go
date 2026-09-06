// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Synthetic planning workload, deliberately containing constraints that a short
// output must preserve. No personal transcript or live itinerary data is used.
func tokenPlanningFixture() map[string]any {
	blocks := []any{}
	for i := 1; i <= 4; i++ {
		b := travelTestBlock(i, "synthetic-place-"+strconv.Itoa(i))
		b["text"] = richText("Keep this full planning constraint: advance tickets required; last admission is 30 minutes before closing. " + strings.Repeat("A useful route note. ", 10))
		b["startTime"] = "10:00"
		b["endTime"] = "11:00"
		b["durationMinutes"] = 60
		p := mapField(b, "place")
		p["business_status"] = "OPERATIONAL"
		p["geometry"] = map[string]any{"location": map[string]any{"lat": 1.0, "lng": float64(i)}}
		p["opening_hours"] = map[string]any{"weekday_text": []any{"Monday: 09:00-17:00"}}
		if i == 3 {
			p["business_status"] = "CLOSED_TEMPORARILY"
		}
		blocks = append(blocks, b)
	}
	blocks = append(blocks, map[string]any{"id": 5, "type": "note", "text": richText("BOOKING DEADLINE: retain the whole sentence; an afternoon ferry requires check-in 45 minutes early.")})
	blocks = append(blocks, map[string]any{"id": 6, "type": "checklist", "title": "Before departure", "items": []any{map[string]any{"id": 61, "checked": false, "text": richText("Bring printed ticket")}}})
	flight := map[string]any{"id": 7, "type": "flight", "flightInfo": map[string]any{"number": 123, "airline": map[string]any{"iata": "XY"}}, "depart": map[string]any{"date": "2030-01-02", "time": "07:00", "airport": map[string]any{"iata": "AAA"}}, "arrive": map[string]any{"date": "2030-01-02", "time": "08:00", "airport": map[string]any{"iata": "BBB"}}, "confirmationNumber": "SYN-BOOK", "text": richText("Airport transfer is a fixed constraint")}
	return map[string]any{"title": "Synthetic planning benchmark", "startDate": "2030-01-02", "endDate": "2030-01-02", "itinerary": map[string]any{"sections": []any{map[string]any{"id": 100, "mode": "list", "blocks": []any{flight, map[string]any{"id": 8, "type": "note", "text": richText("GLOBAL CONSTRAINT: no rental car until tomorrow; use public transport today.")}}}, map[string]any{"id": 101, "mode": "dayPlan", "date": "2030-01-02", "blocks": blocks}}}, "_resources": map[string]any{"distancesBetweenPlaces": map[string]any{"one": travelTestEstimate("synthetic-place-1", "synthetic-place-2", "walking", 600)}, "placeMetadata": []any{map[string]any{"placeId": "synthetic-place-1", "minMinutesSpent": 30, "maxMinutesSpent": 60, "utcOffset": 540}}}}
}

func tokenJSONOutput(t *testing.T, value any, agent bool) []byte {
	t.Helper()
	var out bytes.Buffer
	if err := printJSONFiltered(&out, value, &rootFlags{asJSON: true, compact: true, agent: agent}); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func TestPlanDayTokenWorkloadRetainsConstraints(t *testing.T) {
	trip := tokenPlanningFixture()
	snapshot, err := buildPlanDay(trip, "naertjcoixqrgrfc", 1, []string{"walking", "driving"}, "walking")
	if err != nil {
		t.Fatal(err)
	}
	output := tokenJSONOutput(t, snapshot, true)
	for _, required := range []string{"BOOKING DEADLINE: retain the whole sentence", "last admission is 30 minutes before closing", "CLOSED_TEMPORARILY", "Bring printed ticket", "SYN-BOOK", "07:00", "08:00", "10:00", "11:00", "no_saved_estimate", "minMinutesSpent", "GLOBAL CONSTRAINT: no rental car until tomorrow"} {
		if !bytes.Contains(output, []byte(required)) {
			t.Errorf("compact day loses constraint %q", required)
		}
	}
	if dir := os.Getenv("WANDERLOG_TOKEN_BENCH_DIR"); dir != "" {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		write := func(name string, data []byte) {
			t.Helper()
			if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
				t.Fatal(err)
			}
		}
		// agent=false below reproduces the baseline pretty JSON rendering; candidate
		// day uses agent=true and the new whitespace-only compaction.
		// Equivalent old workflow: day outline, legs with planning metadata, and full
		// block reads for six day blocks plus the relevant undated flight and global note.
		outline, err := buildPlanOutline(trip, "naertjcoixqrgrfc", 1, "", false)
		if err != nil {
			t.Fatal(err)
		}
		write("before-day-01-outline.json", tokenJSONOutput(t, outline, false))
		sec, err := resolveSection(trip, 1, -1, 0)
		if err != nil {
			t.Fatal(err)
		}
		route := buildTravelLegs(trip, sec, []string{"walking", "driving"}, "walking", true)
		route["target_key"] = "naertjcoixqrgrfc"
		write("before-day-02-legs.json", tokenJSONOutput(t, route, false))
		for id := 1; id <= 8; id++ {
			section, block, index, err := resolveUniquePlanBlock(trip, id)
			if err != nil {
				t.Fatal(err)
			}
			read := map[string]any{"command": "plan block get", "target_key": "naertjcoixqrgrfc", "section": section.Report, "block_index": index, "block": readablePlanBlock(block, false, false)}
			write("before-day-block-"+strconv.Itoa(id)+".json", tokenJSONOutput(t, read, false))
		}
		write("after-day-snapshot.json", output)
		query := planDayQuery{Day: 1, Modes: []string{"walking", "driving"}, SelectedMode: "walking", ClientSchemaVersion: 2}
		state, err := makePlanDayState("naertjcoixqrgrfc", query, snapshot)
		if err != nil {
			t.Fatal(err)
		}
		write("after-day-envelope.json", tokenJSONOutput(t, planDayResponse(state, nil, ""), true))
		write("after-day-unchanged.json", tokenJSONOutput(t, planDayResponse(state, &state, ""), true))
		changedTrip := cloneJSONMap(trip)
		_, changedBlock, _, err := resolveUniquePlanBlock(changedTrip, 1)
		if err != nil {
			t.Fatal(err)
		}
		changedBlock["text"] = richText("Updated booking constraint: tickets must be collected 45 minutes early.")
		changedSec := sections(changedTrip)[1].(map[string]any)
		changedSec["blocks"].([]any)[0] = changedBlock
		changedSnapshot, err := buildPlanDay(changedTrip, "naertjcoixqrgrfc", 1, query.Modes, query.SelectedMode)
		if err != nil {
			t.Fatal(err)
		}
		changedState, err := makePlanDayState("naertjcoixqrgrfc", query, changedSnapshot)
		if err != nil {
			t.Fatal(err)
		}
		write("after-day-one-change.json", tokenJSONOutput(t, planDayResponse(changedState, &state, ""), true))
	}
}

func BenchmarkPlanDaySynthetic(b *testing.B) {
	trip := tokenPlanningFixture()
	for i := 0; i < b.N; i++ {
		snapshot, err := buildPlanDay(trip, "naertjcoixqrgrfc", 1, []string{"walking", "driving"}, "walking")
		if err != nil {
			b.Fatal(err)
		}
		if _, err := json.Marshal(snapshot); err != nil {
			b.Fatal(err)
		}
	}
}
