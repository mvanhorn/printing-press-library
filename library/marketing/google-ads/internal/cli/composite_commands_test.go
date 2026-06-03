// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestFlexIntUnmarshal(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want int64
	}{
		{"string int64", `"12300000"`, 12300000},
		{"bare number", `45`, 45},
		{"empty string", `""`, 0},
		{"null", `null`, 0},
		{"float string", `"12.0"`, 12},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var f flexInt
			if err := json.Unmarshal([]byte(tc.in), &f); err != nil {
				t.Fatalf("unmarshal %s: %v", tc.in, err)
			}
			if int64(f) != tc.want {
				t.Fatalf("got %d, want %d", int64(f), tc.want)
			}
		})
	}
}

func TestFlexIntUnmarshalError(t *testing.T) {
	var f flexInt
	if err := json.Unmarshal([]byte(`"not-a-number"`), &f); err == nil {
		t.Fatal("expected error for non-numeric string, got nil")
	}
}

func TestNormalizeQueryKey(t *testing.T) {
	cases := map[string]string{
		"Blue Widgets":      "blue widgets",
		"  blue   widgets ": "blue widgets",
		"WIDGET":            "widget",
		"":                  "",
	}
	for in, want := range cases {
		if got := normalizeQueryKey(in); got != want {
			t.Fatalf("normalizeQueryKey(%q) = %q, want %q", in, got, want)
		}
	}
}

// Decodes a realistic googleAds:search payload (int64 fields as strings,
// conversions as a bare number) the way fetchGAQLRows does.
func TestGAQLSearchTermDecode(t *testing.T) {
	raw := `{
      "results": [
        {"searchTermView": {"searchTerm": "blue widgets"},
         "metrics": {"costMicros": "8500000", "clicks": "40", "conversions": 0}},
        {"searchTermView": {"searchTerm": "widget reviews"},
         "metrics": {"costMicros": "2000000", "clicks": "12", "conversions": 3}}
      ],
      "nextPageToken": "abc"
    }`
	var resp struct {
		Results       []gaqlSearchTermRow `json:"results"`
		NextPageToken string              `json:"nextPageToken"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Results) != 2 {
		t.Fatalf("got %d results, want 2", len(resp.Results))
	}
	if resp.Results[0].SearchTermView.SearchTerm != "blue widgets" {
		t.Fatalf("unexpected term: %q", resp.Results[0].SearchTermView.SearchTerm)
	}
	if int64(resp.Results[0].Metrics.CostMicros) != 8500000 {
		t.Fatalf("cost_micros decode wrong: %d", int64(resp.Results[0].Metrics.CostMicros))
	}
	if resp.NextPageToken != "abc" {
		t.Fatalf("next page token: %q", resp.NextPageToken)
	}
}

func mkSearchTerm(term string, micros, clicks int64, conv float64) gaqlSearchTermRow {
	var r gaqlSearchTermRow
	r.SearchTermView.SearchTerm = term
	r.Metrics.CostMicros = flexInt(micros)
	r.Metrics.Clicks = flexInt(clicks)
	r.Metrics.Conversions = conv
	return r
}

func TestScoreWastedSpend(t *testing.T) {
	rows := []gaqlSearchTermRow{
		mkSearchTerm("cheap-no-conv", 500000, 5, 0), // $0.50 < min, dropped
		mkSearchTerm("converter", 9000000, 30, 2),   // has conversions, dropped
		mkSearchTerm("waste-big", 8000000, 40, 0),   // $8.00 wasted, kept
		mkSearchTerm("waste-small", 2000000, 10, 0), // $2.00 wasted, kept
		mkSearchTerm("waste-mid", 5000000, 20, 0),   // $5.00 wasted, kept
	}

	got := scoreWastedSpend(rows, 1.0, 0)
	if len(got) != 3 {
		t.Fatalf("got %d kept, want 3 (%v)", len(got), got)
	}
	wantOrder := []string{"waste-big", "waste-mid", "waste-small"}
	for i, w := range wantOrder {
		if got[i].SearchTerm != w {
			t.Fatalf("rank %d: got %q, want %q", i, got[i].SearchTerm, w)
		}
	}
	if got[0].Cost != 8.0 {
		t.Fatalf("cost conversion wrong: got %v, want 8.0", got[0].Cost)
	}

	limited := scoreWastedSpend(rows, 1.0, 2)
	if len(limited) != 2 {
		t.Fatalf("limit=2 got %d", len(limited))
	}
	if limited[0].SearchTerm != "waste-big" {
		t.Fatalf("limited top: %q", limited[0].SearchTerm)
	}
}

func TestScoreWastedSpendEmptyAndMissingFields(t *testing.T) {
	if got := scoreWastedSpend(nil, 1.0, 10); len(got) != 0 {
		t.Fatalf("nil input should yield 0 rows, got %d", len(got))
	}
	var blank gaqlSearchTermRow
	if got := scoreWastedSpend([]gaqlSearchTermRow{blank}, 1.0, 10); len(got) != 0 {
		t.Fatalf("blank row should be filtered out, got %d", len(got))
	}
}

func TestBuildWastedSpendQuery(t *testing.T) {
	now := time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC)
	q := buildWastedSpendQuery(7, now)
	if !strings.Contains(q, "FROM search_term_view") {
		t.Fatalf("query missing resource: %q", q)
	}
	if !strings.Contains(q, "BETWEEN '2026-05-28' AND '2026-06-03'") {
		t.Fatalf("query window wrong: %q", q)
	}
	if !strings.Contains(q, "ORDER BY metrics.cost_micros DESC") {
		t.Fatalf("query missing ordering: %q", q)
	}
}

func mkKeyword(text string, micros int64, conv, value float64) gaqlKeywordRow {
	var r gaqlKeywordRow
	r.AdGroupCriterion.Keyword.Text = text
	r.AdGroupCriterion.Keyword.MatchType = "EXACT"
	r.Metrics.CostMicros = flexInt(micros)
	r.Metrics.Conversions = conv
	r.Metrics.ConversionsValue = value
	return r
}

func TestScoreKeywordTiers(t *testing.T) {
	rows := []gaqlKeywordRow{
		mkKeyword("hero", 10000000, 8, 60),   // $10 cost, $60 rev → ROAS 6 → scale
		mkKeyword("okay", 10000000, 3, 25),   // ROAS 2.5 → hold
		mkKeyword("dog", 10000000, 0, 0),     // ROAS 0 → cut
		mkKeyword("tiny", 100000, 1, 50),     // $0.10 < min-cost, dropped
		mkKeyword("edge", 10000000, 4, 40),   // ROAS 4 == scale threshold → scale
	}
	got := scoreKeywordTiers(rows, 1.0, 4.0, 1.0, 0)
	if len(got) != 4 {
		t.Fatalf("got %d kept, want 4", len(got))
	}
	tierByKw := map[string]string{}
	for _, r := range got {
		tierByKw[r.Keyword] = r.Tier
	}
	wantTier := map[string]string{"hero": "scale", "edge": "scale", "okay": "hold", "dog": "cut"}
	for kw, want := range wantTier {
		if tierByKw[kw] != want {
			t.Fatalf("keyword %q tier = %q, want %q", kw, tierByKw[kw], want)
		}
	}
	// ROAS math + CPA on the hero row.
	for _, r := range got {
		if r.Keyword == "hero" {
			if r.ROAS != 6.0 {
				t.Fatalf("hero ROAS = %v, want 6.0", r.ROAS)
			}
			if r.CPA != 10.0/8.0 {
				t.Fatalf("hero CPA = %v, want %v", r.CPA, 10.0/8.0)
			}
		}
	}
	counts := tierCounts(got)
	if counts["scale"] != 2 || counts["hold"] != 1 || counts["cut"] != 1 {
		t.Fatalf("tier counts wrong: %v", counts)
	}
}

func TestParseGSCExport(t *testing.T) {
	// API shape: {"rows":[{"keys":["term"],...}]}
	api := []byte(`{"rows":[
      {"keys":["blue widgets"],"position":3.2,"clicks":10,"impressions":100},
      {"keys":[""],"position":9.0},
      {"position":5.0}
    ]}`)
	rows, dropped, err := parseGSCExport(api)
	if err != nil {
		t.Fatalf("parse api shape: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 kept row, got %d", len(rows))
	}
	if dropped != 2 {
		t.Fatalf("want 2 dropped, got %d", dropped)
	}
	if rows[0].Query != "blue widgets" || rows[0].Position != 3.2 {
		t.Fatalf("unexpected row: %+v", rows[0])
	}

	// Flattened array shape with "query".
	flat := []byte(`[{"query":"widget reviews","position":7.0,"clicks":4}]`)
	rows2, _, err := parseGSCExport(flat)
	if err != nil {
		t.Fatalf("parse flat shape: %v", err)
	}
	if len(rows2) != 1 || rows2[0].Query != "widget reviews" {
		t.Fatalf("flat parse wrong: %+v", rows2)
	}

	// Garbage.
	if _, _, err := parseGSCExport([]byte(`"not an object"`)); err == nil {
		t.Fatal("expected error for non-array/object input")
	}
}

func TestComputeOverlap(t *testing.T) {
	paid := []gaqlSearchTermRow{
		mkSearchTerm("Blue Widgets", 8000000, 40, 1), // ranks organically pos 3 → overlap
		mkSearchTerm("rare term", 5000000, 10, 0),    // not in GSC → excluded
		mkSearchTerm("page two term", 3000000, 5, 0), // GSC pos 18 > max → excluded
		mkSearchTerm("blue widgets", 1000000, 2, 0),  // duplicate (normalized) → deduped
	}
	gsc := []gscQueryRow{
		{Query: "blue widgets", Position: 9.0, Clicks: 50},
		{Query: "blue widgets", Position: 3.0, Clicks: 80}, // best position wins
		{Query: "page two term", Position: 18.0, Clicks: 2},
	}
	got := computeOverlap(paid, gsc, 10.0, 0)
	if len(got) != 1 {
		t.Fatalf("want 1 overlap, got %d (%+v)", len(got), got)
	}
	if got[0].Query != "Blue Widgets" {
		t.Fatalf("unexpected query: %q", got[0].Query)
	}
	if got[0].OrganicPosition != 3.0 {
		t.Fatalf("dedup should keep best position 3.0, got %v", got[0].OrganicPosition)
	}
	if got[0].PaidCost != 8.0 {
		t.Fatalf("paid cost = %v, want 8.0", got[0].PaidCost)
	}
}

// Dry-run contract: each composite returns before touching the network/files
// and emits nothing on stdout.
func TestCompositesDryRunEmitNothing(t *testing.T) {
	cases := []struct {
		name string
		make func(*rootFlags) *bytes.Buffer
	}{
		{"wasted-spend", func(f *rootFlags) *bytes.Buffer {
			cmd := newWastedSpendCmd(f)
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetArgs([]string{"--customer-id", "1234567890"})
			_ = cmd.Execute()
			return &out
		}},
		{"keyword-roi-tiers", func(f *rootFlags) *bytes.Buffer {
			cmd := newKeywordRoiTiersCmd(f)
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetArgs([]string{"--customer-id", "1234567890"})
			_ = cmd.Execute()
			return &out
		}},
		{"paid-organic-overlap", func(f *rootFlags) *bytes.Buffer {
			cmd := newPaidOrganicOverlapCmd(f)
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetArgs([]string{"--customer-id", "1234567890", "--gsc-file", "/nonexistent.json"})
			_ = cmd.Execute()
			return &out
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := tc.make(&rootFlags{dryRun: true})
			if out.Len() != 0 {
				t.Fatalf("%s dry-run emitted %q, want nothing", tc.name, out.String())
			}
		})
	}
}

func TestCompositesRequireCustomerID(t *testing.T) {
	for _, mk := range []func(*rootFlags) error{
		func(f *rootFlags) error {
			c := newWastedSpendCmd(f)
			c.SetArgs([]string{})
			c.SetOut(&bytes.Buffer{})
			return c.Execute()
		},
		func(f *rootFlags) error {
			c := newKeywordRoiTiersCmd(f)
			c.SetArgs([]string{})
			c.SetOut(&bytes.Buffer{})
			return c.Execute()
		},
		func(f *rootFlags) error {
			c := newPaidOrganicOverlapCmd(f)
			c.SetArgs([]string{"--gsc-file", "x.json"})
			c.SetOut(&bytes.Buffer{})
			return c.Execute()
		},
	} {
		if err := mk(&rootFlags{}); err == nil {
			t.Fatal("expected error when --customer-id missing")
		}
	}
}
