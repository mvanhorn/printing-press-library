// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPlanOverviewOrientationConstraintsAndTransfers(t *testing.T) {
	result, err := buildPlanOverview(multiDayFixture(), "abcdefghijklmnop", []string{"walking", "driving"}, "walking")
	if err != nil {
		t.Fatal(err)
	}
	days := result["days"].([]map[string]any)
	if len(days) != 2 || result["block_details_omitted"] != true || result["detail_command"] == nil {
		t.Fatal("overview scope missing")
	}
	if !strings.Contains(mapField(days[0], "section")["text"].(string), "08:30") {
		t.Fatal("day cutoff omitted")
	}
	stops := days[0]["stops"].([]map[string]any)
	if stops[0]["coordinates"] == nil || stops[0]["startTime"] != "09:00" || stops[0]["details_omitted"] != true {
		t.Fatal("missing coordinates/time/omission marker", stops[0])
	}
	if len(mapField(days[0], "lodging")["night_keys"].([]string)) != 1 || mapField(days[1], "lodging")["changed_from_previous_day"] != true {
		t.Fatal("lodging transition missing")
	}
	if mapField(days[0], "lodging")["night_keys"].([]string)[0] != "301" || mapField(days[1], "lodging")["night_keys"].([]string)[0] != "302" {
		t.Fatal("checkout counted as overnight stay")
	}
	transitions := result["transfers_between_days"].([]map[string]any)
	if len(transitions) != 1 || transitions[0]["from_block_key"] != "103" || transitions[0]["to_block_key"] != "104" || transitions[0]["schedule_status"] != "not_checked_cross_day" {
		t.Fatal("cross-day gap hidden", transitions)
	}
	for _, estimate := range transitions[0]["estimates"].([]map[string]any) {
		if estimate["available"] != false || estimate["reason"] == nil {
			t.Fatal("missing estimate falsely certified", estimate)
		}
	}
	raw, _ := json.Marshal(result)
	for _, want := range []string{"No stairs", "3 days ahead", "Global medical", "45 minutes", "avoidTolls", "no_saved_estimate"} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("overview lost %s", want)
		}
	}
}
func TestPlanOverviewUndatedGuideAndUnknownEndpoints(t *testing.T) {
	trip := multiDayFixture()
	for _, raw := range sections(trip) {
		section := raw.(map[string]any)
		if planningDayMode(stringField(section, "mode")) {
			delete(section, "date")
			section["mode"] = "guideDayPlan"
		}
	}
	result, err := buildPlanOverview(trip, "key", []string{"walking"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(result["days"].([]map[string]any)) != 2 {
		t.Fatal("undated day sections dropped")
	}
	for _, day := range result["days"].([]map[string]any) {
		if mapField(day, "lodging")["changed_from_previous_day"] != nil {
			t.Fatal("unknown lodging claimed changed/unchanged")
		}
	}
	// A guide composed entirely of place lists still exposes every constraint.
	for _, raw := range sections(trip) {
		raw.(map[string]any)["mode"] = "placeList"
	}
	result, err = buildPlanOverview(trip, "key", []string{"walking"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(result["days"].([]map[string]any)) != 0 || len(result["context"].([]map[string]any)) != 4 || len(mapField(result, "blocks")) == 0 {
		t.Fatal("guide without day sections lost its content")
	}
	trip = multiDayFixture()
	sections(trip)[1].(map[string]any)["blocks"] = []any{}
	transitions := overviewDayTransfers(trip, []string{"walking"}, "")
	if transitions[0]["endpoints_status"] != "unavailable" || transitions[0]["estimates"].([]map[string]any)[0]["reason"] != "missing_day_endpoint" {
		t.Fatal("empty day endpoint silently omitted")
	}
}

func TestPlanOverviewTransportBoundaryIsUnknown(t *testing.T) {
	for _, kind := range []string{"flight", "rentalCar", "train", "bus", "ferry", "cruise"} {
		for _, atStart := range []bool{false, true} {
			trip := multiDayFixture()
			section := sections(trip)[0].(map[string]any)
			if atStart {
				section = sections(trip)[1].(map[string]any)
			}
			booking := map[string]any{"id": 901, "type": kind}
			blocks := section["blocks"].([]any)
			if atStart {
				section["blocks"] = append([]any{booking}, blocks...)
			} else {
				section["blocks"] = append(blocks, booking)
			}
			transfer := overviewDayTransfers(trip, []string{"walking", "driving"}, "driving")[0]
			if transfer["endpoints_status"] != "unknown_transport_boundary" || transfer["link_kind"] != "last_first_routable_place" {
				t.Fatalf("%s atStart=%v: misleading boundary: %v", kind, atStart, transfer)
			}
			if keys := transfer["intervening_transport_keys"].([]string); len(keys) != 1 || keys[0] != "901" {
				t.Fatal("booking reference missing", transfer)
			}
			for _, estimate := range transfer["estimates"].([]map[string]any) {
				if estimate["available"] != false || estimate["reason"] != "intervening_transport_reservation" {
					t.Fatal("transport route inferred", estimate)
				}
			}
		}
	}
}

func TestPlanOverviewGlobalTransportDates(t *testing.T) {
	for _, tc := range []struct {
		name, depart, arrive string
		guarded, unknown     bool
	}{
		{"spanning", "2026-12-31", "2027-01-03", true, false},
		{"matching", "2027-01-01", "2027-01-02", true, false},
		{"unrelated", "2027-02-01", "2027-02-02", false, false},
		{"unknown", "", "", true, true},
		{"partial", "2027-02-01", "", true, true},
		{"invalid", "2027-02-01", "not-a-date", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			trip := multiDayFixture()
			flight := sections(trip)[2].(map[string]any)["blocks"].([]any)[0].(map[string]any)
			flight["depart"] = map[string]any{"date": tc.depart}
			flight["arrive"] = map[string]any{"date": tc.arrive}
			transfer := overviewDayTransfers(trip, []string{"driving"}, "driving")[0]
			if (transfer["endpoints_status"] == "unknown_transport_boundary") != tc.guarded {
				t.Fatal(transfer)
			}
			if (transfer["transport_date_unknown_keys"] != nil) != tc.unknown {
				t.Fatal("date uncertainty lost", transfer)
			}
			if tc.guarded {
				if transfer["estimates"].([]map[string]any)[0]["reason"] != "intervening_transport_reservation" {
					t.Fatal(transfer)
				}
				if transfer["intervening_transport_keys"].([]string)[0] != "201" {
					t.Fatal("global booking reference lost")
				}
			}
		})
	}
	for _, kind := range []string{"rentalCar", "train", "bus", "ferry", "cruise"} {
		relevant, unknown := overviewGlobalTransportRelevant(map[string]any{"type": kind, "pickUp": map[string]any{"date": "2026-12-31"}, "dropOff": map[string]any{"date": "2027-01-03"}}, "2027-01-01", "2027-01-02")
		if !relevant || unknown {
			t.Fatal("spanning booking missed", kind)
		}
	}
}

func TestPlanOverviewGlobalMorningFlightExcludedByClock(t *testing.T) {
	trip := multiDayFixture()
	transfer := overviewDayTransfers(trip, []string{"driving"}, "driving")[0]
	if transfer["endpoints_status"] != "available" || transfer["intervening_transport_keys"] != nil {
		t.Fatal("morning arrival before final visit treated as overnight", transfer)
	}
}

func TestPlanOverviewGlobalTransportMissingEndpointDate(t *testing.T) {
	for _, booking := range []map[string]any{
		{"depart": map[string]any{"date": "2027-02-01"}},
		{"dropOff": map[string]any{"date": "2026-12-01"}},
		{"startDate": "2027-02-01"},
	} {
		relevant, unknown := overviewGlobalTransportRelevant(booking, "2027-01-01", "2027-01-02")
		if !relevant || !unknown {
			t.Fatal("missing interval endpoint treated as complete", booking)
		}
	}
}
