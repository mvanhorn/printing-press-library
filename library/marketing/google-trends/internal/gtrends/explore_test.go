// Copyright 2026 Kerry Morrison and contributors. Licensed under Apache-2.0. See LICENSE.

package gtrends

import (
	"encoding/json"
	"testing"
)

func TestParseExplore(t *testing.T) {
	fixture := []byte(`)]}'
{"widgets":[
  {"id":"TIMESERIES","token":"tok-ts","request":{"comparisonItem":[{"time":"today 12-m"}]}},
  {"id":"GEO_MAP","token":"tok-geo","request":{"resolution":"COUNTRY"}}
]}`)
	result, err := parseExplore(fixture)
	if err != nil {
		t.Fatalf("parseExplore: %v", err)
	}
	if len(result.Widgets) != 2 {
		t.Fatalf("expected 2 widgets, got %d", len(result.Widgets))
	}
	w, ok := FindWidget(result.Widgets, "TIMESERIES")
	if !ok {
		t.Fatalf("expected TIMESERIES widget to be found")
	}
	if w.Token != "tok-ts" {
		t.Errorf("token = %q, want %q", w.Token, "tok-ts")
	}
	if _, ok := FindWidget(result.Widgets, "NOT_A_WIDGET"); ok {
		t.Errorf("expected NOT_A_WIDGET to be absent")
	}
}

func TestParseExploreEmptyWidgetsIsEmptySliceNotNil(t *testing.T) {
	result, err := parseExplore([]byte(`{}`))
	if err != nil {
		t.Fatalf("parseExplore: %v", err)
	}
	if result.Widgets == nil {
		t.Fatalf("expected Widgets to be an empty slice, got nil")
	}
	out, err := json.Marshal(result.Widgets)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(out) != "[]" {
		t.Errorf("marshaled Widgets = %s, want []", out)
	}
}

func TestParseExploreInvalidJSON(t *testing.T) {
	if _, err := parseExplore([]byte(`not json`)); err == nil {
		t.Fatalf("expected error for invalid JSON")
	}
}

func TestParseMultilineSingleKeyword(t *testing.T) {
	fixture := []byte(`{"default":{"timelineData":[
		{"time":"1700000000","formattedTime":"Nov 14, 2023","value":[10]},
		{"time":"1700086400","formattedTime":"Nov 15, 2023","value":[20]}
	]}}`)
	points, err := parseMultiline(fixture)
	if err != nil {
		t.Fatalf("parseMultiline: %v", err)
	}
	if len(points) != 2 {
		t.Fatalf("expected 2 points, got %d", len(points))
	}
	if points[0].Values[0] != 10 || points[1].Values[0] != 20 {
		t.Errorf("unexpected values: %+v", points)
	}
	if points[0].Time.Unix() != 1700000000 {
		t.Errorf("Time = %v, want unix 1700000000", points[0].Time)
	}
}

// TestParseMultilineMultiKeywordIndexing verifies the multi-compare shape:
// each timelineData row's "value" array is per-keyword, parallel to the
// comparisonItem order the caller passed to Explore.
func TestParseMultilineMultiKeywordIndexing(t *testing.T) {
	fixture := []byte(`)]}'
{"default":{"timelineData":[
	{"time":"1700000000","formattedTime":"Nov 14, 2023","value":[10,55,3]},
	{"time":"1700086400","formattedTime":"Nov 15, 2023","value":[20,60,4]}
]}}`)
	points, err := parseMultiline(fixture)
	if err != nil {
		t.Fatalf("parseMultiline: %v", err)
	}
	if len(points) != 2 {
		t.Fatalf("expected 2 points, got %d", len(points))
	}
	for i, want := range [][]int{{10, 55, 3}, {20, 60, 4}} {
		if len(points[i].Values) != len(want) {
			t.Fatalf("point %d: got %d values, want %d", i, len(points[i].Values), len(want))
		}
		for k := range want {
			if points[i].Values[k] != want[k] {
				t.Errorf("point %d value[%d] = %d, want %d", i, k, points[i].Values[k], want[k])
			}
		}
	}
}

func TestParseMultilineSkipsMalformedTime(t *testing.T) {
	fixture := []byte(`{"default":{"timelineData":[
		{"time":"not-a-number","formattedTime":"???","value":[1]},
		{"time":"1700000000","formattedTime":"Nov 14, 2023","value":[10]}
	]}}`)
	points, err := parseMultiline(fixture)
	if err != nil {
		t.Fatalf("parseMultiline: %v", err)
	}
	if len(points) != 1 {
		t.Fatalf("expected malformed row to be skipped, got %d points", len(points))
	}
}

func TestParseComparedGeo(t *testing.T) {
	fixture := []byte(`)]}'
{"default":{"geoMapData":[
	{"geoCode":"US-CA","geoName":"California","value":[42]},
	{"geoCode":"US-NY","geoName":"New York","value":[7]}
]}}`)
	regions, err := parseComparedGeo(fixture)
	if err != nil {
		t.Fatalf("parseComparedGeo: %v", err)
	}
	if len(regions) != 2 {
		t.Fatalf("expected 2 regions, got %d", len(regions))
	}
	if regions[0].GeoCode != "US-CA" || regions[0].Values[0] != 42 {
		t.Errorf("unexpected region[0]: %+v", regions[0])
	}
}

// TestParseRelatedSearchesBreakout verifies the >=5000 breakout sentinel is
// only applied to the rising list, and top-list values (even large ones)
// never get flagged.
func TestParseRelatedSearchesBreakout(t *testing.T) {
	fixture := []byte(`{"default":{"rankedList":[
		{"rankedKeyword":[
			{"query":"top term one","value":100},
			{"query":"top term huge","value":9000}
		]},
		{"rankedKeyword":[
			{"query":"rising normal","value":250},
			{"query":"rising breakout","value":5000},
			{"query":"rising way over","value":8500}
		]}
	]}}`)
	top, rising, err := parseRelatedSearches(fixture)
	if err != nil {
		t.Fatalf("parseRelatedSearches: %v", err)
	}
	if len(top) != 2 {
		t.Fatalf("expected 2 top terms, got %d", len(top))
	}
	for _, term := range top {
		if term.IsBreakout {
			t.Errorf("top term %q must never be flagged breakout, value=%d", term.Query, term.Value)
		}
	}
	if len(rising) != 3 {
		t.Fatalf("expected 3 rising terms, got %d", len(rising))
	}
	wantBreakout := map[string]bool{
		"rising normal":   false,
		"rising breakout": true,
		"rising way over": true,
	}
	for _, term := range rising {
		if term.IsBreakout != wantBreakout[term.Query] {
			t.Errorf("rising term %q: IsBreakout = %v, want %v (value=%d)", term.Query, term.IsBreakout, wantBreakout[term.Query], term.Value)
		}
	}
}

func TestParseRelatedSearchesEmptyListsAreEmptySlicesNotNil(t *testing.T) {
	top, rising, err := parseRelatedSearches([]byte(`{}`))
	if err != nil {
		t.Fatalf("parseRelatedSearches: %v", err)
	}
	if top == nil || rising == nil {
		t.Fatalf("expected empty slices, not nil: top=%v rising=%v", top, rising)
	}
}

func TestBuildExploreRequestJSONGeoBroadcast(t *testing.T) {
	body, err := buildExploreRequestJSON([]string{"a", "b", "c"}, []string{"US"}, "today 12-m", 0, "")
	if err != nil {
		t.Fatalf("buildExploreRequestJSON: %v", err)
	}
	var parsed exploreRequestBody
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(parsed.ComparisonItem) != 3 {
		t.Fatalf("expected 3 comparisonItem entries, got %d", len(parsed.ComparisonItem))
	}
	for _, item := range parsed.ComparisonItem {
		if item.Geo != "US" {
			t.Errorf("geo = %q, want broadcast %q", item.Geo, "US")
		}
	}
}

func TestBuildExploreRequestJSONTooManyKeywords(t *testing.T) {
	_, err := buildExploreRequestJSON([]string{"a", "b", "c", "d", "e", "f"}, nil, "today 12-m", 0, "")
	if err == nil {
		t.Fatalf("expected error for 6 keywords (max 5)")
	}
}

func TestBuildExploreRequestJSONMismatchedGeos(t *testing.T) {
	_, err := buildExploreRequestJSON([]string{"a", "b", "c"}, []string{"US", "GB"}, "today 12-m", 0, "")
	if err == nil {
		t.Fatalf("expected error for geos length mismatch")
	}
}
