// Copyright 2026 Milos Mladenovic and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"math"
	"testing"
)

func TestParseWindowDays(t *testing.T) {
	tests := []struct {
		in   string
		def  int
		want int
	}{
		{"", 90, 90},
		{"90", 7, 90},
		{"90d", 7, 90},
		{"6w", 7, 42},
		{"48h", 7, 2},
		{"garbage", 30, 30},
		{"0", 7, 1}, // clamped to >= 1
	}
	for _, tc := range tests {
		if got := parseWindowDays(tc.in, tc.def); got != tc.want {
			t.Errorf("parseWindowDays(%q,%d)=%d want %d", tc.in, tc.def, got, tc.want)
		}
	}
}

func TestPearson(t *testing.T) {
	// Perfect positive, perfect negative, and flat (zero variance) series.
	if r := pearson([]float64{1, 2, 3}, []float64{2, 4, 6}); math.Abs(r-1) > 1e-9 {
		t.Errorf("perfect positive: got %v want 1", r)
	}
	if r := pearson([]float64{1, 2, 3}, []float64{6, 4, 2}); math.Abs(r+1) > 1e-9 {
		t.Errorf("perfect negative: got %v want -1", r)
	}
	if r := pearson([]float64{1, 1, 1}, []float64{2, 4, 6}); r != 0 {
		t.Errorf("zero variance: got %v want 0", r)
	}
	if r := pearson(nil, nil); r != 0 {
		t.Errorf("empty: got %v want 0", r)
	}
}

func TestExtractCurveAndValueAtSecs(t *testing.T) {
	raw := json.RawMessage(`{"secs":[5,60,300],"values":[900,400,300]}`)
	secs, vals := extractCurve(raw)
	if len(secs) != 3 || len(vals) != 3 {
		t.Fatalf("extractCurve got secs=%v vals=%v", secs, vals)
	}
	if v, ok := valueAtSecs(secs, vals, 60); !ok || v != 400 {
		t.Errorf("valueAtSecs(60)=%v,%v want 400,true", v, ok)
	}
	// Closest <= target: 120 has no exact bucket, falls back to 60.
	if v, ok := valueAtSecs(secs, vals, 120); !ok || v != 400 {
		t.Errorf("valueAtSecs(120)=%v,%v want 400,true", v, ok)
	}
	// Below smallest bucket -> not found.
	if _, ok := valueAtSecs(secs, vals, 1); ok {
		t.Errorf("valueAtSecs(1) should be not found")
	}
	// Unrecognized shape -> nil slices.
	if s, _ := extractCurve(json.RawMessage(`{"foo":1}`)); s != nil {
		t.Errorf("unrecognized shape should yield nil secs, got %v", s)
	}
	// intervals.icu's real {"list":[{secs,values}]} wrapper.
	wrapped := json.RawMessage(`{"list":[{"secs":[5,60],"values":[800,350]}]}`)
	ws, wv := extractCurve(wrapped)
	if len(ws) != 2 || len(wv) != 2 || wv[1] != 350 {
		t.Errorf("list-wrapped extractCurve got secs=%v vals=%v", ws, wv)
	}
}

func TestGearDue(t *testing.T) {
	overdue := map[string]json.RawMessage{
		"reminders": json.RawMessage(`[{"text":"new chain","distance":5000000}]`),
	}
	if reason, due := gearDue(overdue, 6000000); !due || reason != "new chain" {
		t.Errorf("overdue gear: got %q,%v want new chain,true", reason, due)
	}
	under := map[string]json.RawMessage{
		"reminders": json.RawMessage(`[{"text":"new chain","distance":5000000}]`),
	}
	if _, due := gearDue(under, 1000000); due {
		t.Errorf("under-threshold gear should not be due")
	}
	none := map[string]json.RawMessage{}
	if _, due := gearDue(none, 9e9); due {
		t.Errorf("gear without reminders should not be due")
	}
}

func TestDayOf(t *testing.T) {
	if got := dayOf("2026-06-02T10:30:00"); got != "2026-06-02" {
		t.Errorf("dayOf datetime: got %q", got)
	}
	if got := dayOf("2026-06-02"); got != "2026-06-02" {
		t.Errorf("dayOf date: got %q", got)
	}
	if got := dayOf("x"); got != "x" {
		t.Errorf("dayOf short: got %q", got)
	}
}

func TestJSONStrAndRound1(t *testing.T) {
	m := map[string]json.RawMessage{"name": json.RawMessage(`"Morning Ride"`), "n": json.RawMessage(`5`)}
	if got := jsonStr(m, "name"); got != "Morning Ride" {
		t.Errorf("jsonStr string: got %q", got)
	}
	if got := jsonStr(m, "missing"); got != "" {
		t.Errorf("jsonStr missing: got %q", got)
	}
	if got := round1(3.14159); got != 3.1 {
		t.Errorf("round1: got %v", got)
	}
}
