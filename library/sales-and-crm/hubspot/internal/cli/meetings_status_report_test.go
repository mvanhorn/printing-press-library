// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"
	"time"
)

func TestCanonicalizeStatus(t *testing.T) {
	cases := map[string]string{
		"scheduled":  "SCHEDULED",
		"no-show":    "NO_SHOW",
		"no_show":    "NO_SHOW",
		"noshow":     "NO_SHOW",
		"completed":  "COMPLETED",
		"cancelled":  "CANCELED",
		"canceled":   "CANCELED",
		"CUSTOM_VAL": "CUSTOM_VAL", // pass-through
	}
	for in, want := range cases {
		if got := canonicalizeStatus(in); got != want {
			t.Errorf("canonicalizeStatus(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseMonth(t *testing.T) {
	from, to, err := parseMonth("2026-05")
	if err != nil {
		t.Fatalf("parseMonth: %v", err)
	}
	wantFrom := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	wantTo := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	if !from.Equal(wantFrom) {
		t.Errorf("from = %v, want %v", from, wantFrom)
	}
	if !to.Equal(wantTo) {
		t.Errorf("to = %v, want %v", to, wantTo)
	}
}

func TestParseMonth_BadInput(t *testing.T) {
	if _, _, err := parseMonth("2026-99"); err == nil {
		t.Fatal("expected error on invalid month")
	}
	if _, _, err := parseMonth(""); err == nil {
		t.Fatal("expected error on empty month")
	}
}
