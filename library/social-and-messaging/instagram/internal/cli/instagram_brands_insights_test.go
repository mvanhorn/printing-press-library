// Copyright 2026 Mohammed Al Khamis and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

// Per-media insights ignore metric_type=total_value and return the time-series
// `values` shape. The pull collector previously read only `total_value`, so
// every per-media metric (reach, views, saved, shares, total_interactions) was
// silently persisted as 0. This guards against that regression.
func TestParseInsightTotals_MediaValuesShape(t *testing.T) {
	raw := []byte(`{"data":[
		{"name":"reach","period":"lifetime","values":[{"value":211}]},
		{"name":"views","period":"lifetime","values":[{"value":330}]},
		{"name":"saved","period":"lifetime","values":[{"value":4}]},
		{"name":"shares","period":"lifetime","values":[{"value":0}]},
		{"name":"total_interactions","period":"lifetime","values":[{"value":17}]}
	]}`)

	got, err := parseInsightTotals(raw)
	if err != nil {
		t.Fatalf("parseInsightTotals returned error: %v", err)
	}
	want := map[string]int64{"reach": 211, "views": 330, "saved": 4, "shares": 0, "total_interactions": 17}
	for k, v := range want {
		// Check presence, not just value: an absent key also reads as 0, so a
		// plain got[k]==0 assertion for "shares" would pass even if the metric
		// were silently skipped rather than parsed from values[].
		gv, ok := got[k]
		if !ok {
			t.Errorf("metric %q missing from result (want %d)", k, v)
			continue
		}
		if gv != v {
			t.Errorf("metric %q = %d, want %d", k, gv, v)
		}
	}
}

// Account insights honor metric_type=total_value and return a scalar; that path
// must keep working unchanged.
func TestParseInsightTotals_AccountTotalValueShape(t *testing.T) {
	raw := []byte(`{"data":[
		{"name":"reach","total_value":{"value":555}},
		{"name":"total_interactions","total_value":{"value":88}}
	]}`)

	got, err := parseInsightTotals(raw)
	if err != nil {
		t.Fatalf("parseInsightTotals returned error: %v", err)
	}
	if got["reach"] != 555 || got["total_interactions"] != 88 {
		t.Errorf("got %+v, want reach=555 total_interactions=88", got)
	}
}

// A present-but-zero total_value must be recorded as 0, not skipped in favor of
// a stray values[] entry.
func TestParseInsightTotals_ZeroTotalValueWins(t *testing.T) {
	raw := []byte(`{"data":[{"name":"reach","total_value":{"value":0},"values":[{"value":99}]}]}`)
	got, err := parseInsightTotals(raw)
	if err != nil {
		t.Fatalf("parseInsightTotals returned error: %v", err)
	}
	if got["reach"] != 0 {
		t.Errorf("reach = %d, want 0 (total_value takes precedence)", got["reach"])
	}
}

// A present total_value whose inner "value" sub-field is absent
// ({"total_value":{}}) must still be treated as present (recorded as 0), taking
// precedence over a stray values[] entry. This guards the presence check
// against relying on the inner value's empty-string sentinel.
func TestParseInsightTotals_EmptyTotalValueWins(t *testing.T) {
	raw := []byte(`{"data":[{"name":"reach","total_value":{},"values":[{"value":99}]}]}`)
	got, err := parseInsightTotals(raw)
	if err != nil {
		t.Fatalf("parseInsightTotals returned error: %v", err)
	}
	v, ok := got["reach"]
	if !ok {
		t.Fatalf("reach missing; a present total_value must be recorded")
	}
	if v != 0 {
		t.Errorf("reach = %d, want 0 (present total_value takes precedence over values[])", v)
	}
}

// A total_value of JSON null means the key carries no scalar (the media shape);
// the parser must fall through to the values[] entry rather than recording 0.
func TestParseInsightTotals_NullTotalValueFallsThrough(t *testing.T) {
	raw := []byte(`{"data":[{"name":"reach","total_value":null,"values":[{"value":211}]}]}`)
	got, err := parseInsightTotals(raw)
	if err != nil {
		t.Fatalf("parseInsightTotals returned error: %v", err)
	}
	if got["reach"] != 211 {
		t.Errorf("reach = %d, want 211 (null total_value should fall through to values[])", got["reach"])
	}
}
