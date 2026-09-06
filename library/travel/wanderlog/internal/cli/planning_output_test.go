// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestPlanningNumericProjection(t *testing.T) {
	input := json.RawMessage(`{"tripPlan":{"itinerary":{"sections":[{"blocks":[{"id":1,"text":"first","place":{"large":"unused"}}]},{"blocks":[{"id":2,"text":"second"},{"id":3,"text":"third"}]}]}},"resources":{"large":"unused"}}`)
	got := filterFields(input, "tripPlan.itinerary.sections.1.blocks.0.id,tripPlan.itinerary.sections.1.blocks.0.text")
	want := `{"tripPlan":{"itinerary":{"sections":[{"blocks":[{"id":2,"text":"second"}]}]}}}`
	var a, b any
	json.Unmarshal(got, &a)
	json.Unmarshal([]byte(want), &b)
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("got %s", got)
	}
	for _, path := range []string{"tripPlan.itinerary.sections.99.blocks.0.id", "tripPlan.itinerary.sections.1.blocks.0.nonexistent"} {
		if selected := filterFields(input, path); !selectLooksEmpty(selected) {
			t.Fatalf("invalid path %s looked valid: %s", path, selected)
		}
	}
}

func TestPlanningProjectionDropsUnrelatedResources(t *testing.T) {
	got := filterFields(json.RawMessage(`{"items":[{"id":1,"name":"one"}],"resources":{"huge":"private"},"total_count":1}`), "id")
	var obj map[string]any
	json.Unmarshal(got, &obj)
	if _, ok := obj["resources"]; ok {
		t.Fatalf("unrequested object leaked: %s", got)
	}
	if obj["total_count"] != float64(1) {
		t.Fatalf("lost count: %s", got)
	}
}
func TestPlanningProjectionWildcard(t *testing.T) {
	got := filterFields(json.RawMessage(`{"data":[{"id":1,"name":"one"},{"id":2,"name":"two"}]}`), "data.*.id")
	if string(got) != `{"data":[{"id":1},{"id":2}]}` {
		t.Fatal(string(got))
	}
}
func TestPlanningAutocompleteEnvelope(t *testing.T) {
	raw, err := autocompleteQueryEnvelope("博物館", 0, 0, true, true, 1000, "ja")
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	json.Unmarshal([]byte(raw), &body)
	if body["input"] != "博物館" || body["location"] == nil || len(body["sessiontoken"].(string)) != 32 {
		t.Fatal(body)
	}
	for _, tc := range []struct {
		query          string
		lat, lng       float64
		hasLat, hasLng bool
		radius         int
	}{{"", 1, 1, true, true, 100}, {"museum", 1, 1, true, false, 100}, {"museum", 91, 1, true, true, 100}, {"museum", 1, 1, true, true, -1}} {
		if _, err := autocompleteQueryEnvelope(tc.query, tc.lat, tc.lng, tc.hasLat, tc.hasLng, tc.radius, "en"); err == nil {
			t.Fatalf("accepted invalid input %+v", tc)
		}
	}
}
func TestPlanningDiscoveryReadAndCreate(t *testing.T) {
	for query, want := range map[string]string{"create a blank trip": "trips create", "show full details and note text for an itinerary block": "plan block get", "travel time driving walking between itinerary stops": "plan route legs"} {
		got := rankWhich(whichIndex, query, 1)
		if len(got) != 1 || got[0].Entry.Command != want {
			t.Fatalf("%q: %+v", query, got)
		}
	}
}

func TestPlanningCompactPreservesAttachmentResults(t *testing.T) {
	raw := compactFields(json.RawMessage(`{"command":"plan block attachment list","attachments":[{"id":123,"title":"Ticket"}]}`))
	var obj map[string]any
	json.Unmarshal(raw, &obj)
	if a, ok := obj["attachments"].([]any); !ok || len(a) != 1 {
		t.Fatalf("missing primary result: %s", raw)
	}
}

func TestPlanningTripCreationValidation(t *testing.T) {
	for _, raw := range []string{`{"geoIds":[]}`, `{"geoIds":[1.5]}`, `{"geoIds":"1"}`, `{"geoIds":[1],"privacy":"typo"}`, `{"geoIds":[1],"startDate":"2026-10-05"}`, `{"geoIds":[1],"startDate":"2026-10-05","endDate":"2026-10-04"}`} {
		var body map[string]any
		json.Unmarshal([]byte(raw), &body)
		if validateTripCreation(body) == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	var body map[string]any
	json.Unmarshal([]byte(`{"geoIds":[1],"privacy":"private","startDate":"2026-10-05","endDate":"2026-10-07"}`), &body)
	if err := validateTripCreation(body); err != nil {
		t.Fatal(err)
	}
}

func TestAgentWhitespaceCompactionPreservesValues(t *testing.T) {
	raw := json.RawMessage(`{"id":9007199254740993,"note":"First line\nSecond line","unknown":null,"available":false,"amount":1.25}`)
	var out bytes.Buffer
	if err := printOutputWithFlags(&out, raw, &rootFlags{agent: true, asJSON: true}); err != nil {
		t.Fatal(err)
	}
	if strings.Count(out.String(), "\n") != 1 || !strings.Contains(out.String(), "9007199254740993") {
		t.Fatal(out.String())
	}
	var want, got any
	a := json.NewDecoder(bytes.NewReader(raw))
	a.UseNumber()
	a.Decode(&want)
	b := json.NewDecoder(&out)
	b.UseNumber()
	b.Decode(&got)
	if !reflect.DeepEqual(want, got) {
		t.Fatal("JSON values changed")
	}
}
