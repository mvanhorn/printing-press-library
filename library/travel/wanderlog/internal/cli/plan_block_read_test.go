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
	"strings"
	"sync/atomic"
	"testing"
)

func blockReadFixture() map[string]any {
	text, _ := compileMarkdownDelta("**Read all of this**\n- first\n- second\n" + strings.Repeat("A long complete note. ", 80))
	return map[string]any{"itinerary": map[string]any{"sections": []any{
		map[string]any{"id": 10, "mode": "dayPlan", "date": "2027-01-01", "blocks": []any{map[string]any{"id": 101, "type": "place", "text": text, "startTime": "09:00", "endTime": "10:00", "durationMinutes": 60, "attachments": []any{map[string]any{"title": "Ticket", "url": "https://example.com/ticket"}}, "place": map[string]any{"name": "Named museum", "place_id": "synthetic-place", "opening_hours": map[string]any{"weekday_text": []any{"Monday: 09:00–17:00"}}, "business_status": "OPERATIONAL", "photos": []any{strings.Repeat("unwanted", 1000)}}}}},
		map[string]any{"id": 11, "mode": "dayPlan", "date": "2027-01-02", "blocks": []any{map[string]any{"id": 102, "type": "note", "text": map[string]any{"ops": []any{map[string]any{"insert": "Original note"}}}}}},
	}}}
}

func TestPlanBlockGetGlobalIDAndUsefulRead(t *testing.T) {
	trip := blockReadFixture()
	sec, block, idx, err := resolveUniquePlanBlock(trip, 102)
	if err != nil || sec.Index != 1 || idx != 0 {
		t.Fatalf("global resolve: %#v %d %v", sec, idx, err)
	}
	_, block, _, err = resolveUniquePlanBlock(trip, 101)
	if err != nil {
		t.Fatal(err)
	}
	result := readablePlanBlock(block, true, false)
	if !strings.Contains(result["text"].(string), strings.Repeat("A long complete note. ", 79)) {
		t.Fatal("note truncated")
	}
	if !strings.Contains(result["markdown"].(string), "**Read all of this**\n- first\n- second") {
		t.Fatalf("formatting lost: %s", result["markdown"])
	}
	place := result["place"].(map[string]any)
	if place["photos"] != nil || place["opening_hours"] == nil || place["business_status"] != "OPERATIONAL" {
		t.Fatalf("projection wrong: %#v", place)
	}
	if result["raw_text"] != nil || result["attachments"] == nil {
		t.Fatal("raw opt-in/attachments contract")
	}
	if readablePlanBlock(block, false, true)["raw_text"] == nil {
		t.Fatal("missing opt-in Quill")
	}
	var out bytes.Buffer
	if err := printJSONFiltered(&out, map[string]any{"block": result}, &rootFlags{asJSON: true, compact: true}); err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(out.Bytes(), &parsed); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "A long complete note.") {
		t.Fatal("agent mode stripped requested note")
	}
}

func TestPlanBlockGetMissingAndAmbiguous(t *testing.T) {
	trip := blockReadFixture()
	if _, _, _, err := resolveUniquePlanBlock(trip, 999); err == nil {
		t.Fatal("missing id accepted")
	}
	sec := sections(trip)[1].(map[string]any)
	sec["blocks"].([]any)[0].(map[string]any)["id"] = 101
	if _, _, _, err := resolveUniquePlanBlock(trip, 101); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("duplicate id accepted: %v", err)
	}
}

func TestPlanBlockReadChecklistAndLinks(t *testing.T) {
	delta := map[string]any{"ops": []any{map[string]any{"insert": "Reserve tickets", "attributes": map[string]any{"link": "https://example.com/book", "bold": true}}}}
	block := map[string]any{"id": 8, "type": "checklist", "items": []any{map[string]any{"id": 9, "checked": false, "text": delta}, map[string]any{"id": 10, "checked": true, "text": "Legacy item"}}, "text": delta}
	result := readablePlanBlock(block, true, true)
	items := result["items"].([]map[string]any)
	if len(items) != 2 || items[0]["text"] != "Reserve tickets" || items[1]["text"] != "Legacy item" || items[1]["checked"] != true {
		t.Fatal(items)
	}
	if items[0]["markdown"] != "[**Reserve tickets**](https://example.com/book)" || items[0]["raw_text"] == nil {
		t.Fatal(items[0])
	}
	var out bytes.Buffer
	if err := printJSONFiltered(&out, map[string]any{"block": result}, &rootFlags{compact: true, asJSON: true}); err != nil {
		t.Fatal(err)
	}
	var envelope map[string]any
	if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	read := envelope["block"].(map[string]any)
	if len(read["items"].([]any)) != 2 || read["links"].([]any)[0] != "https://example.com/book" {
		t.Fatal(read)
	}
	defaultRead := readablePlanBlock(block, false, false)
	if defaultRead["links"].([]string)[0] != "https://example.com/book" {
		t.Fatal("default read loses link target")
	}
}

func TestPlanBlockReadContactAndReservationDetails(t *testing.T) {
	block := map[string]any{"id": 1, "type": "place", "place": map[string]any{"international_phone_number": "+1 555 0100"}, "nameForReservation": "Synthetic guest", "shipName": "Synthetic ship", "voyageNumber": "V1"}
	got := readablePlanBlock(block, false, false)
	if got["place"].(map[string]any)["international_phone_number"] != "+1 555 0100" || got["nameForReservation"] != "Synthetic guest" || got["voyageNumber"] != "V1" {
		t.Fatal(got)
	}
}

func TestPlanBlockGetJoinsSavedPlanningMetadata(t *testing.T) {
	trip := blockReadFixture()
	trip["_resources"] = map[string]any{"placeMetadata": []any{
		map[string]any{"placeId": "other-place", "minMinutesSpent": 999},
		map[string]any{"placeId": "synthetic-place", "minMinutesSpent": 30, "maxMinutesSpent": 90, "description": "Not requested"},
	}}
	_, block, _, err := resolveUniquePlanBlock(trip, 101)
	if err != nil {
		t.Fatal(err)
	}
	result := readablePlanBlockWithPlanning(trip, block, false, false)
	planning := result["planning"].(map[string]any)
	if planning["minMinutesSpent"] != 30 || planning["maxMinutesSpent"] != 90 || planning["freshness"] != "unknown" {
		t.Fatal(planning)
	}
	if planning["description"] != nil {
		t.Fatal("unrequested metadata prose leaked")
	}
}

func TestMultiBlockReadOneFetchPreservesNotesReservationsAndOrder(t *testing.T) {
	trip := blockReadFixture()
	first := sections(trip)[0].(map[string]any)
	first["blocks"] = append(first["blocks"].([]any), map[string]any{"id": 103, "type": "place", "place": map[string]any{"name": "Hotel", "business_status": "CLOSED_TEMPORARILY"}, "hotel": map[string]any{"checkIn": "2027-01-01", "checkOut": "2027-01-03", "confirmationNumber": "SYNTHETIC"}, "text": richText("Keep reservation note"), "warnings": []any{"Verify temporary closure before arrival"}})
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method != "GET" || r.URL.Path != "/api/tripPlans/abcdefghijklmnop" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(400)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "tripPlan": trip})
	}))
	defer server.Close()
	configFile := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(configFile, []byte(fmt.Sprintf("base_url = %q\n", server.URL)), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WANDERLOG_COOKIE", "")
	root := newRootCmd(&rootFlags{})
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetErr(&output)
	root.SetArgs([]string{"plan", "block", "get", "--target-key", "abcdefghijklmnop", "--block-ids", "103,101,102", "--markdown", "--raw-text", "--agent", "--config", configFile})
	if err := root.Execute(); err != nil {
		t.Fatalf("%v: %s", err, output.String())
	}
	if calls.Load() != 1 {
		t.Fatalf("got %d fetches", calls.Load())
	}
	var result struct {
		Blocks   []map[string]any    `json:"blocks"`
		Sections []planSectionReport `json:"sections"`
		Count    int                 `json:"count"`
	}
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatalf("%v: %s", err, output.String())
	}
	if result.Count != 3 || len(result.Blocks) != 3 || len(result.Sections) != 2 {
		t.Fatalf("incorrect shared metadata/count: %s", output.String())
	}
	for i, id := range []int{103, 101, 102} {
		if intAny(result.Blocks[i]["id"]) != id {
			t.Fatal("input order lost")
		}
		if _, ok := result.Blocks[i]["section"]; ok {
			t.Fatal("duplicated section metadata")
		}
	}
	hotel := result.Blocks[0]
	if hotel["hotel"].(map[string]any)["confirmationNumber"] != "SYNTHETIC" || hotel["place"].(map[string]any)["business_status"] != "CLOSED_TEMPORARILY" || hotel["text"] != "Keep reservation note" || hotel["warnings"].([]any)[0] != "Verify temporary closure before arrival" {
		t.Fatal(hotel)
	}
	museum := result.Blocks[1]
	if !strings.Contains(museum["text"].(string), strings.Repeat("A long complete note. ", 79)) || museum["attachments"] == nil || museum["raw_text"] == nil || !strings.Contains(museum["markdown"].(string), "**Read all of this**") {
		t.Fatal("multi-block read lost content")
	}
	if intAny(museum["section_index"]) != 0 || intAny(result.Blocks[2]["section_index"]) != 1 {
		t.Fatal("section references wrong")
	}
}

func TestMultiBlockReadRejectsInvalidSelectionBeforeFetch(t *testing.T) {
	for _, args := range [][]string{{"--block-ids", "101,101"}, {"--block-ids", "0,101"}, {"--block-ids", "101", "--block-id", "102"}, {"--block-ids", ""}} {
		command := newNovelPlanBlockGetCmd(&rootFlags{})
		command.SetArgs(args)
		command.SilenceErrors = true
		command.SilenceUsage = true
		if err := command.Execute(); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
	trip := blockReadFixture()
	if _, err := resolveUniquePlanBlocks(trip, []int{101, 999}); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatal(err)
	}
	sections(trip)[1].(map[string]any)["blocks"].([]any)[0].(map[string]any)["id"] = 101
	if _, err := resolveUniquePlanBlocks(trip, []int{101}); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatal(err)
	}
}
