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

// Decodes a realistic googleAds:search payload (int64 fields as strings,
// conversions as a bare number) and confirms the rows survive the round-trip.
func TestGAQLSearchResponseDecode(t *testing.T) {
	raw := `{
      "results": [
        {"searchTermView": {"searchTerm": "blue widgets"},
         "metrics": {"costMicros": "8500000", "clicks": "40", "conversions": 0}},
        {"searchTermView": {"searchTerm": "widget reviews"},
         "metrics": {"costMicros": "2000000", "clicks": "12", "conversions": 3}}
      ],
      "nextPageToken": "abc"
    }`
	var resp gaqlSearchResponse
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

func TestScoreWastedSpend(t *testing.T) {
	mk := func(term string, micros int64, clicks int64, conv float64) gaqlSearchTermRow {
		var r gaqlSearchTermRow
		r.SearchTermView.SearchTerm = term
		r.Metrics.CostMicros = flexInt(micros)
		r.Metrics.Clicks = flexInt(clicks)
		r.Metrics.Conversions = conv
		return r
	}
	rows := []gaqlSearchTermRow{
		mk("cheap-no-conv", 500000, 5, 0),     // $0.50 < min, dropped
		mk("converter", 9000000, 30, 2),        // has conversions, dropped
		mk("waste-big", 8000000, 40, 0),        // $8.00 wasted, kept
		mk("waste-small", 2000000, 10, 0),      // $2.00 wasted, kept
		mk("waste-mid", 5000000, 20, 0),        // $5.00 wasted, kept
	}

	got := scoreWastedSpend(rows, 1.0, 0)
	if len(got) != 3 {
		t.Fatalf("got %d kept, want 3 (%v)", len(got), got)
	}
	// Ranked by cost desc: waste-big, waste-mid, waste-small.
	wantOrder := []string{"waste-big", "waste-mid", "waste-small"}
	for i, w := range wantOrder {
		if got[i].SearchTerm != w {
			t.Fatalf("rank %d: got %q, want %q", i, got[i].SearchTerm, w)
		}
	}
	if got[0].Cost != 8.0 {
		t.Fatalf("cost conversion wrong: got %v, want 8.0", got[0].Cost)
	}

	// Limit caps the result set.
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
	// Row missing search term + metrics must not panic; cost 0 < min so dropped.
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
	// 7-day inclusive window ending 2026-06-03 starts 2026-05-28.
	if !strings.Contains(q, "BETWEEN '2026-05-28' AND '2026-06-03'") {
		t.Fatalf("query window wrong: %q", q)
	}
	if !strings.Contains(q, "ORDER BY metrics.cost_micros DESC") {
		t.Fatalf("query missing ordering: %q", q)
	}
}

// Dry-run contract: the command returns before touching the network and emits
// nothing on stdout.
func TestWastedSpendDryRunEmitsNothing(t *testing.T) {
	flags := &rootFlags{dryRun: true}
	cmd := newWastedSpendCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--customer-id", "1234567890"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("dry-run execute: %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("dry-run should emit nothing, got %q", out.String())
	}
}

func TestWastedSpendRequiresCustomerID(t *testing.T) {
	flags := &rootFlags{}
	cmd := newWastedSpendCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error when --customer-id missing")
	}
}
