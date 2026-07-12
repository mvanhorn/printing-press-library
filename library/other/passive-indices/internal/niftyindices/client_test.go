// Copyright 2026 Mayank Lavania and contributors. Licensed under Apache-2.0. See LICENSE.

package niftyindices

import (
	"encoding/json"
	"testing"
	"time"
)

func TestSlugify(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"upper with space", "NIFTY 50", "nifty50"},
		{"already lower", "nifty next 50", "niftynext50"},
		{"mixed case multi space", "NIFTY Bank", "niftybank"},
		{"no spaces", "SENSEX", "sensex"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Slugify(tc.in); got != tc.want {
				t.Errorf("Slugify(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	cases := []struct {
		name string
		in   string
		max  int
		want string
	}{
		{"under limit", "hello", 10, "hello"},
		{"over limit", "hello", 3, "hel..."},
		{"empty", "", 5, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := truncate(tc.in, tc.max); got != tc.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tc.in, tc.max, got, tc.want)
			}
		})
	}
}

// TestCinfoBody asserts the /BackPage/ request body shape: the outer
// object's "cinfo" value is itself a JSON-encoded string (not a nested
// object), matching the BackPage PageMethods calling convention. Getting
// this double-encoding wrong is exactly the kind of bug that produces a
// silent HTTP 200 with an unparseable body rather than a clean error.
func TestCinfoBody(t *testing.T) {
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)

	body, err := cinfoBody("NIFTY 50", from, to)
	if err != nil {
		t.Fatalf("cinfoBody returned error: %v", err)
	}

	var outer map[string]string
	if err := json.Unmarshal(body, &outer); err != nil {
		t.Fatalf("unmarshaling outer body: %v", err)
	}
	cinfo, ok := outer["cinfo"]
	if !ok {
		t.Fatalf("outer body missing %q key: %s", "cinfo", body)
	}

	var inner map[string]string
	if err := json.Unmarshal([]byte(cinfo), &inner); err != nil {
		t.Fatalf("cinfo value is not JSON-encoded: %v (value=%q)", err, cinfo)
	}
	want := map[string]string{
		"name":      "NIFTY 50",
		"indexName": "NIFTY 50",
		"startDate": "01-Jan-2026",
		"endDate":   "30-Jun-2026",
	}
	for k, wantV := range want {
		if inner[k] != wantV {
			t.Errorf("cinfo[%q] = %q, want %q", k, inner[k], wantV)
		}
	}
}

func TestParseSectorWeightLabel(t *testing.T) {
	cases := []struct {
		name       string
		label      string
		wantName   string
		wantWeight float64
	}{
		{"constituent label", "ABCAPITAL 2.25%", "ABCAPITAL", 2.25},
		{"sector label", "Financial Services 33.16%", "Financial Services", 33.16},
		{"integer-valued weight", "KARURVYSYA 2%", "KARURVYSYA", 2},
		{"no percent suffix falls back to zero weight", "All", "All", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			name, weight := parseSectorWeightLabel(tc.label)
			if name != tc.wantName || weight != tc.wantWeight {
				t.Errorf("parseSectorWeightLabel(%q) = (%q, %v), want (%q, %v)", tc.label, name, weight, tc.wantName, tc.wantWeight)
			}
		})
	}
}

// TestParseSectorWeightsJSONP covers the two real quirks of this feed found
// during live investigation: the modelDataAvailable(...) callback carries a
// SECOND, unrelated metadata argument after the data object (must be
// ignored, not fed to json.Unmarshal), and every array/object literal in
// the first argument uses JS-style trailing commas invalid in strict JSON.
func TestParseSectorWeightsJSONP(t *testing.T) {
	body := []byte(`modelDataAvailable({"groups":[{"label":"Financial Services 33.16%","colour":"","weight":33.16,"groups":[{"label":"ABCAPITAL 2.25%","weight":2.25,"id":"1","date":"30-06-2026"},],"id":"0"},]},{ label:  "NIFTY ALPHA 50 ",file: "SectorialIndexDataNIFTY ALPHA 50.jsonp "});`)

	wire, err := parseSectorWeightsJSONP(body)
	if err != nil {
		t.Fatalf("parseSectorWeightsJSONP returned error: %v", err)
	}
	if len(wire.Groups) != 1 {
		t.Fatalf("got %d top-level groups, want 1", len(wire.Groups))
	}
	g := wire.Groups[0]
	if g.Label != "Financial Services 33.16%" {
		t.Errorf("group label = %q, want %q", g.Label, "Financial Services 33.16%")
	}
	if len(g.Groups) != 1 || g.Groups[0].Label != "ABCAPITAL 2.25%" {
		t.Errorf("nested groups = %+v, want one entry labeled %q", g.Groups, "ABCAPITAL 2.25%")
	}
}

func TestParseSectorWeightsJSONPRejectsNonCallback(t *testing.T) {
	if _, err := parseSectorWeightsJSONP([]byte(`{"not":"a callback"}`)); err == nil {
		t.Error("expected an error for a body with no modelDataAvailable(...) wrapper")
	}
}
