// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func tokenMultiDayFixture() map[string]any {
	trip := tokenPlanningFixture()
	original := sections(trip)[1].(map[string]any)
	all := sections(trip)
	for day := 2; day <= 3; day++ {
		sec := cloneJSONMap(original)
		sec["id"] = 100 + day
		sec["date"] = fmt.Sprintf("2030-01-%02d", day+1)
		sec["text"] = richText(fmt.Sprintf("Day %d constraint: arrive before the evening gate closes.", day))
		for _, raw := range sec["blocks"].([]any) {
			block := raw.(map[string]any)
			block["id"] = intAny(block["id"]) + day*100
		}
		all = append(all, sec)
	}
	trip["itinerary"].(map[string]any)["sections"] = all
	trip["endDate"] = "2030-01-04"
	return trip
}

func TestPlanFlowTokenWorkload(t *testing.T) {
	trip := tokenMultiDayFixture()
	modes := []string{"walking", "driving"}
	full, err := buildPlanDays(trip, "naertjcoixqrgrfc", []int{1, 2, 3}, modes, "walking")
	if err != nil {
		t.Fatal(err)
	}
	output := tokenJSONOutput(t, full, true)
	for _, text := range []string{"GLOBAL CONSTRAINT", "BOOKING DEADLINE", "CLOSED_TEMPORARILY", "Bring printed ticket", "SYN-BOOK", "no_saved_estimate", "Day 3 constraint"} {
		if !bytes.Contains(output, []byte(text)) {
			t.Fatalf("missing constraint %q", text)
		}
	}
	overview, err := buildPlanOverview(trip, "naertjcoixqrgrfc", modes, "walking")
	if err != nil {
		t.Fatal(err)
	}
	dir := os.Getenv("WANDERLOG_TOKEN_BENCH_DIR")
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	write := func(name string, data []byte) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("flow-days.json", output)
	write("flow-overview.json", tokenJSONOutput(t, overview, true))
	for day := 1; day <= 3; day++ {
		snapshot, err := buildPlanDay(trip, "naertjcoixqrgrfc", day, modes, "walking")
		if err != nil {
			t.Fatal(err)
		}
		state, err := makePlanDayState("naertjcoixqrgrfc", planDayQuery{Day: day, Modes: modes, SelectedMode: "walking", ClientSchemaVersion: 2}, snapshot)
		if err != nil {
			t.Fatal(err)
		}
		write(fmt.Sprintf("flow-separate-day-%d.json", day), tokenJSONOutput(t, planDayResponse(state, nil, ""), true))
	}
}
