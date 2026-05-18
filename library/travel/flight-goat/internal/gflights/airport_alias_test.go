// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

// PATCH(library): tests for the airport-alias-table patch. See
// airport_alias.go for the production code.

package gflights

import (
	"encoding/json"
	"testing"
)

func TestRemapAirport(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		wantTo  string
		changed bool
	}{
		{"retired PNH remaps to KTI", "PNH", "KTI", true},
		{"retired REP remaps to SAI", "REP", "SAI", true},
		{"lowercase input is normalized", "pnh", "KTI", true},
		{"whitespace is trimmed", "  REP  ", "SAI", true},
		{"current code passes through", "SEA", "SEA", false},
		{"current KTI does not bounce", "kti", "KTI", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := remapAirport(tc.in)
			if got.To != tc.wantTo {
				t.Errorf("remapAirport(%q).To = %q, want %q", tc.in, got.To, tc.wantTo)
			}
			if got.Changed != tc.changed {
				t.Errorf("remapAirport(%q).Changed = %v, want %v", tc.in, got.Changed, tc.changed)
			}
		})
	}
}

func TestRemapAirportEmptyInput(t *testing.T) {
	got := remapAirport("")
	if got.Changed {
		t.Errorf("remapAirport(\"\").Changed = true, want false")
	}
	if got.From != "" || got.To != "" {
		t.Errorf("remapAirport(\"\") = %+v, want zero value", got)
	}
}

func TestRemapAirportPair(t *testing.T) {
	cases := []struct {
		name        string
		origin      string
		destination string
		wantOrigin  string
		wantDest    string
		wantNote    bool
	}{
		{"neither remapped", "SEA", "LAX", "SEA", "LAX", false},
		{"destination only", "SEA", "PNH", "SEA", "KTI", true},
		{"origin only", "PNH", "SEA", "KTI", "SEA", true},
		{"both remapped", "PNH", "REP", "KTI", "SAI", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o, d, note := remapAirportPair(tc.origin, tc.destination)
			if o.To != tc.wantOrigin {
				t.Errorf("origin.To = %q, want %q", o.To, tc.wantOrigin)
			}
			if d.To != tc.wantDest {
				t.Errorf("destination.To = %q, want %q", d.To, tc.wantDest)
			}
			if (note != nil) != tc.wantNote {
				t.Errorf("note presence = %v, want %v", note != nil, tc.wantNote)
			}
		})
	}
}

func TestAirportRemapNoteOmitsEmptySides(t *testing.T) {
	_, _, note := remapAirportPair("PNH", "SEA")
	if note == nil {
		t.Fatal("expected non-nil note for one-sided remap")
	}
	if note.Origin == nil {
		t.Error("Origin should be populated when origin was remapped")
	}
	if note.Destination != nil {
		t.Error("Destination should be nil when destination was not remapped")
	}

	b, err := json.Marshal(note)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(b)
	if contains(got, `"destination"`) {
		t.Errorf("JSON should omit destination key when nil; got %s", got)
	}
	if !contains(got, `"origin"`) {
		t.Errorf("JSON should include origin key; got %s", got)
	}
}

func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestSearchResultOmitsAirportRemappedWhenNil(t *testing.T) {
	r := SearchResult{
		Success: true,
		Source:  "native-go",
		Query:   SearchQuery{Origin: "SEA", Destination: "LAX"},
	}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if contains(string(b), "airport_remapped") {
		t.Errorf("SearchResult JSON should omit airport_remapped when nil; got %s", b)
	}
}

func TestSearchResultIncludesAirportRemappedWhenPopulated(t *testing.T) {
	_, _, note := remapAirportPair("PNH", "SEA")
	r := SearchResult{
		Success:         true,
		Source:          "native-go",
		Query:           SearchQuery{Origin: "PNH", Destination: "SEA"},
		AirportRemapped: note,
	}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(b)
	if !contains(got, `"airport_remapped"`) {
		t.Errorf("expected airport_remapped key; got %s", got)
	}
	if !contains(got, `"from":"PNH"`) || !contains(got, `"to":"KTI"`) {
		t.Errorf("expected origin remap fields; got %s", got)
	}
	if contains(got, `"changed"`) {
		t.Errorf("Changed field should be omitted from JSON; got %s", got)
	}
	// User's original input is preserved in Query, not the remapped code.
	if !contains(got, `"origin":"PNH"`) {
		t.Errorf("Query.Origin should echo user input PNH; got %s", got)
	}
}

func TestDatesResultIncludesAirportRemapped(t *testing.T) {
	_, _, note := remapAirportPair("SEA", "REP")
	r := DatesResult{
		Success:         true,
		Query:           SearchQuery{Origin: "SEA", Destination: "REP"},
		AirportRemapped: note,
	}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(b)
	if !contains(got, `"airport_remapped"`) {
		t.Errorf("DatesResult should include airport_remapped; got %s", got)
	}
	if !contains(got, `"destination"`) || !contains(got, `"to":"SAI"`) {
		t.Errorf("expected destination remap fields; got %s", got)
	}
}
