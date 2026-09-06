// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/spf13/cobra"
)

func multiDayFixture() map[string]any {
	trip := dayFixture()
	trip["title"] = "Synthetic multi-day trip"
	trip["itinerary"].(map[string]any)["options"] = map[string]any{"avoidTolls": true}
	first := sections(trip)[0].(map[string]any)
	for _, raw := range first["blocks"].([]any) {
		raw.(map[string]any)["place"].(map[string]any)["geometry"] = map[string]any{"location": map[string]any{"lat": 26.2, "lng": 127.7}}
	}
	second := sections(trip)[1].(map[string]any)
	different := cloneJSONMap(first["blocks"].([]any)[0].(map[string]any))
	different["id"] = 104
	different["text"] = richText("Second day cutoff at 17:00")
	different["place"].(map[string]any)["business_status"] = "CLOSED_TEMPORARILY"
	second["blocks"] = append(second["blocks"].([]any), different)
	hotel := func(id int, name, start, end string) map[string]any {
		return map[string]any{"id": id, "type": "place", "place": map[string]any{"place_id": name, "name": name}, "hotel": map[string]any{"checkIn": start, "checkOut": end, "confirmationNumber": "SYNTHETIC"}, "text": richText("Keep late check-in constraint")}
	}
	global := map[string]any{"id": 20, "mode": "placeList", "heading": "Group constraints", "text": richText("No stairs. Ferry cancellation cutoff 18:00."), "blocks": []any{
		map[string]any{"id": 300, "type": "note", "text": richText("Global medical packing reminder")},
		map[string]any{"id": 303, "type": "place", "text": richText("Candidate needs booking 3 days ahead"), "place": map[string]any{"place_id": "candidate", "name": "Candidate"}},
		hotel(301, "hotel-a", "2027-01-01", "2027-01-02"), hotel(302, "hotel-b", "2027-01-02", "2027-01-03"),
	}}
	trip["itinerary"].(map[string]any)["sections"] = append(sections(trip), global)
	return trip
}
func TestPlanDaysSharedContextAndDistinctVariants(t *testing.T) {
	trip := multiDayFixture()
	result, err := buildPlanDays(trip, "abcdefghijklmnop", []int{2, 1}, []string{"walking"}, "")
	if err != nil {
		t.Fatal(err)
	}
	days := result["days"].([]map[string]any)
	if intAny(mapField(days[0], "section")["day"]) != 2 || len(days) != 2 {
		t.Fatal("selection order changed")
	}
	blocks := mapField(result, "blocks")
	if blocks["301"] == nil || blocks["302"] == nil || blocks["300"] == nil || blocks["303"] == nil {
		t.Fatal("global constraints lost")
	}
	if len(result["context"].([]map[string]any)) != 2 {
		t.Fatal("global sections repeated per day")
	}
	for _, context := range result["context"].([]map[string]any) {
		if context["day"] != 0 || context["mode"] == nil {
			t.Fatal("global constraint falsely assigned day", context)
		}
	}
	a, b := mapField(blocks, "101"), mapField(blocks, "104")
	if a["place_ref"] == b["place_ref"] {
		t.Fatal("unequal metadata variants merged")
	}
	if !strings.Contains(a["text"].(string), strings.Repeat("A long complete note. ", 79)) {
		t.Fatal("full note truncated")
	}
	raw, _ := json.Marshal(result)
	for _, want := range []string{"No stairs", "3 days ahead", "avoidTolls", "late check-in", "45 minutes", "meet by 08:30"} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("lost constraint %s", want)
		}
	}
	if strings.Count(string(raw), "Global medical packing reminder") != 1 {
		t.Fatal("shared global note duplicated")
	}
	if strings.Count(string(raw), "Check-in closes 45 minutes before departure.") != 1 {
		t.Fatal("reservation content duplicated")
	}
}
func TestPlanDaysDuplicateIDsNeverOverwrite(t *testing.T) {
	trip := multiDayFixture()
	rows := sections(trip)[1].(map[string]any)["blocks"].([]any)
	rows[1].(map[string]any)["id"] = 101
	result, err := buildPlanDays(trip, "abcdefghijklmnop", []int{1, 2}, []string{"walking"}, "")
	if err != nil {
		t.Fatal(err)
	}
	blocks := mapField(result, "blocks")
	if result["stable_ids"] != false || blocks["position:0:0"] == nil || blocks["position:1:1"] == nil {
		t.Fatal("ambiguous IDs collapsed", blocks)
	}
	if mapField(blocks, "position:0:0")["place_ref"] == mapField(blocks, "position:1:1")["place_ref"] {
		t.Fatal("different metadata collapsed")
	}
}
func TestPlanDaysSelectionValidation(t *testing.T) {
	ids, err := parsePlanDays("3,1-2")
	if err != nil || !reflect.DeepEqual(ids, []int{3, 1, 2}) {
		t.Fatal(ids, err)
	}
	for _, value := range []string{"", "0", "-1", "2-1", "1,1", "1-9999999999", "one", "1,"} {
		if _, err := parsePlanDays(value); err == nil {
			t.Errorf("accepted %q", value)
		}
	}
	if _, err := buildPlanDays(multiDayFixture(), "key", []int{1, 3}, []string{"walking"}, ""); err == nil {
		t.Fatal("missing requested day silently dropped")
	}
}
func TestPlanDaysAndOverviewCommandsFetchExactlyOnce(t *testing.T) {
	trip := multiDayFixture()
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method != "GET" {
			t.Error("unexpected write")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "tripPlan": trip})
	}))
	defer server.Close()
	configPath := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(configPath, []byte(fmt.Sprintf("base_url = %q\n", server.URL)), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WANDERLOG_COOKIE", "")
	for _, tc := range []struct {
		name string
		make func(*rootFlags) *cobra.Command
		args []string
	}{{"days", newNovelPlanDaysCmd, []string{"--days", "1,2"}}, {"overview", newNovelPlanOverviewCmd, nil}} {
		t.Run(tc.name, func(t *testing.T) {
			before := calls.Load()
			flags := &rootFlags{configPath: configPath, asJSON: true, compact: true, noCache: true}
			cmd := tc.make(flags)
			var output bytes.Buffer
			cmd.SetOut(&output)
			cmd.SetErr(&output)
			cmd.SetArgs(append([]string{"--target-key", "abcdefghijklmnop"}, tc.args...))
			if err := cmd.Execute(); err != nil {
				t.Fatal(err, output.String())
			}
			if calls.Load()-before != 1 {
				t.Fatal("repeated snapshot fetch")
			}
			var parsed map[string]any
			if err := json.Unmarshal(output.Bytes(), &parsed); err != nil {
				t.Fatal(err)
			}
			if len(parsed["days"].([]any)) != 2 {
				t.Fatal("agent compact output lost days")
			}
		})
	}
}
