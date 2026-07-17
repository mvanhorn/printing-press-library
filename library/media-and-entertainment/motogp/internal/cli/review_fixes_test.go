// Copyright 2026 waterpig and contributors. Licensed under Apache-2.0. See LICENSE.
// Regression tests for the PR #1531 review fixes (Greptile):
//   - ICS all-day DTEND must be exclusive (issue 2)
//   - webhook delivery must refuse SSRF-sensitive destinations (issue 3)
//   - championship aggregation must key on a stable rider id, not display name (issue 6)

package cli

import (
	"net"
	"testing"
)

func TestICSDatePlusOne(t *testing.T) {
	cases := map[string]string{
		"20260614": "20260615", // normal day
		"20261231": "20270101", // year rollover
		"20260228": "20260301", // 2026 is not a leap year: Feb 28 -> Mar 1
		"20240228": "20240229", // 2024 is a leap year: Feb 28 -> Feb 29
		"garbage":  "garbage",  // unparseable falls back unchanged
		"":         "",         // empty falls back unchanged
	}
	for in, want := range cases {
		if got := icsDatePlusOne(in); got != want {
			t.Errorf("icsDatePlusOne(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestICSDatePlusOneNotEqualStart(t *testing.T) {
	// The whole point of the fix: for a single-day event the exclusive DTEND
	// must differ from DTSTART so clients don't import a zero-duration event.
	start := "20260614"
	if end := icsDatePlusOne(start); end == start {
		t.Fatalf("icsDatePlusOne(%q) == start; all-day DTEND must be exclusive (start+1)", start)
	}
}

func TestIsBlockedWebhookIP(t *testing.T) {
	blocked := []string{
		"127.0.0.1",       // loopback
		"::1",             // loopback v6
		"10.0.0.5",        // private
		"172.16.9.9",      // private
		"192.168.1.10",    // private
		"169.254.169.254", // cloud metadata (link-local)
		"0.0.0.0",         // unspecified
		"fe80::1",         // link-local v6
		"fc00::1",         // unique-local v6 (private)
		"224.0.0.1",       // multicast
	}
	for _, s := range blocked {
		ip := net.ParseIP(s)
		if ip == nil {
			t.Fatalf("test bug: cannot parse %q", s)
		}
		if !isBlockedWebhookIP(ip) {
			t.Errorf("isBlockedWebhookIP(%s) = false, want true (SSRF-sensitive)", s)
		}
	}
	allowed := []string{
		"1.1.1.1",
		"8.8.8.8",
		"93.184.216.34", // example.com
		"2606:4700:4700::1111",
	}
	for _, s := range allowed {
		ip := net.ParseIP(s)
		if ip == nil {
			t.Fatalf("test bug: cannot parse %q", s)
		}
		if isBlockedWebhookIP(ip) {
			t.Errorf("isBlockedWebhookIP(%s) = true, want false (public)", s)
		}
	}
	if !isBlockedWebhookIP(nil) {
		t.Errorf("isBlockedWebhookIP(nil) = false, want true (fail closed)")
	}
}

func TestRiderStableKeySurvivesDisplayNameVariance(t *testing.T) {
	// Same rider, different name shapes across rounds -> must share one key.
	a := mgpRider{ID: "abc-123", FullNameFlat: "Marc MARQUEZ"}
	b := mgpRider{ID: "abc-123", Name: "Marc", Surname: "Marquez"}
	if a.stableKey() != b.stableKey() {
		t.Errorf("stableKey mismatch for same UUID: %q vs %q", a.stableKey(), b.stableKey())
	}

	// Distinct riders -> distinct keys.
	c := mgpRider{ID: "def-456", FullNameFlat: "Marc Marquez"} // same name, different id
	if a.stableKey() == c.stableKey() {
		t.Errorf("distinct riders shared a key: %q", a.stableKey())
	}

	// Fallback chain: no UUID -> legacy id; no ids -> number; none -> name.
	if got := (mgpRider{LegacyID: 93}).stableKey(); got != "legacy:93" {
		t.Errorf("legacy fallback = %q, want legacy:93", got)
	}
	if got := (mgpRider{Number: 93}).stableKey(); got != "num:93" {
		t.Errorf("number fallback = %q, want num:93", got)
	}
	if got := (mgpRider{FullNameFlat: "  Marc   Marquez "}).stableKey(); got != "name:marc marquez" {
		t.Errorf("name fallback = %q, want name:marc marquez", got)
	}
}
