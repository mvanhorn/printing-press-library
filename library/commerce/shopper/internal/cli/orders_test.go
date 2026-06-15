// Copyright 2026 educrvz and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written tests for the orders spend/history helpers (PATCH: orders-history).

package cli

import (
	"testing"
	"time"
)

func TestParseBRL(t *testing.T) {
	cases := map[string]float64{
		"R$ 2.124,65":  2124.65,
		"R$ 660,23":    660.23,
		"R$ 0,00":      0,
		"1.533,47":     1533.47,
		"R$ 12.978,76": 12978.76,
		"":             0,
	}
	for in, want := range cases {
		if got := parseBRL(in); got != want {
			t.Errorf("parseBRL(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestFormatBRL(t *testing.T) {
	cases := map[float64]string{
		2124.65:  "2.124,65",
		660.23:   "660,23",
		0:        "0,00",
		12978.76: "12.978,76",
		1000000:  "1.000.000,00",
		-50.5:    "-50,50",
	}
	for in, want := range cases {
		if got := formatBRL(in); got != want {
			t.Errorf("formatBRL(%v) = %q, want %q", in, got, want)
		}
	}
}

func TestParseBRLFormatRoundTrip(t *testing.T) {
	// A value pulled from the API and reformatted must read back identically.
	if got := formatBRL(parseBRL("R$ 2.908,54")); got != "2.908,54" {
		t.Errorf("round trip = %q, want 2.908,54", got)
	}
}

func TestMonthKeyFromBR(t *testing.T) {
	if k, ok := monthKeyFromBR("23/04/2026"); !ok || k != "2026-04" {
		t.Errorf("monthKeyFromBR(23/04/2026) = %q,%v want 2026-04,true", k, ok)
	}
	if _, ok := monthKeyFromBR("garbage"); ok {
		t.Error("monthKeyFromBR(garbage) should fail")
	}
	if _, ok := monthKeyFromBR("4/4/26"); ok {
		t.Error("monthKeyFromBR with non-padded parts should fail")
	}
}

func TestLastNMonths(t *testing.T) {
	now := time.Date(2026, 6, 4, 0, 0, 0, 0, time.UTC)
	got := lastNMonths(now, 3)
	want := []string{"2026-04", "2026-05", "2026-06"}
	if len(got) != len(want) {
		t.Fatalf("lastNMonths len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("lastNMonths[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	// Crossing a year boundary.
	jan := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	g := lastNMonths(jan, 2)
	if g[0] != "2025-12" || g[1] != "2026-01" {
		t.Errorf("year-boundary lastNMonths = %v, want [2025-12 2026-01]", g)
	}
}

func TestMonthLabel(t *testing.T) {
	if monthLabel("2026-04") != "Apr/26" {
		t.Errorf("monthLabel(2026-04) = %q, want Apr/26", monthLabel("2026-04"))
	}
	if monthLabel("bad") != "bad" {
		t.Error("monthLabel should pass through malformed keys")
	}
}
