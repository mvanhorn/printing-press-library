// Copyright 2026 Stephan Stoeber and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"testing"
	"time"
)

func TestParseDayDuration(t *testing.T) {
	cases := map[string]time.Duration{
		"24h": 24 * time.Hour,
		"7d":  7 * 24 * time.Hour,
		"30d": 30 * 24 * time.Hour,
		"15m": 15 * time.Minute,
	}
	for in, want := range cases {
		got, err := parseDayDuration(in)
		if err != nil {
			t.Errorf("parseDayDuration(%q) error: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("parseDayDuration(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestAggregateByReason(t *testing.T) {
	rows := []failureMessageRow{
		{ID: "1", Reason: "carrier_blocked"},
		{ID: "2", Reason: "carrier_blocked"},
		{ID: "3", Reason: ""},
		{ID: "4", Reason: "expired"},
	}
	agg := aggregateByReason(rows)
	if len(agg) != 3 {
		t.Fatalf("expected 3 distinct reasons, got %d: %+v", len(agg), agg)
	}
	if agg[0].Reason != "carrier_blocked" || agg[0].Count != 2 {
		t.Errorf("expected carrier_blocked count=2 first, got %+v", agg[0])
	}
}
