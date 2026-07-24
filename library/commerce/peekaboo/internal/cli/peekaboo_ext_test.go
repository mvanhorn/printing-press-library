// Copyright 2026 and contributors. Licensed under Apache-2.0. See LICENSE.
// Behavior tests for the hand-authored Peekaboo helpers.

package cli

import (
	"math"
	"testing"
)

func TestExtractGuestToken(t *testing.T) {
	cases := []struct {
		name string
		html string
		want string
	}{
		{
			name: "typical embed with nested preferences object",
			html: `<script guest>window.__guest__={"email":"guest@peekaboo.guru","role":"guest","associations":[],"preferences":{},"token":"abc.def.ghi"}</script><script src="/x.js">`,
			want: "abc.def.ghi",
		},
		{
			name: "no marker",
			html: `<html><body>nothing here</body></html>`,
			want: "",
		},
		{
			name: "marker but no closing script",
			html: `window.__guest__={"token":"zzz"`,
			want: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := extractGuestToken(tc.html); got != tc.want {
				t.Fatalf("extractGuestToken() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestMapsDirectionsURL(t *testing.T) {
	got := mapsDirectionsURL(31.5546, 74.3572)
	want := "https://www.google.com/maps?daddr=31.5546,74.3572"
	if got != want {
		t.Fatalf("mapsDirectionsURL() = %q, want %q", got, want)
	}
}

func TestHaversineKm(t *testing.T) {
	// Lahore center to a branch ~23 km away (Lake City).
	d := haversineKm(31.5546, 74.3572, 31.3672941, 74.2536989)
	if d < 15 || d > 30 {
		t.Fatalf("haversineKm() = %.2f, expected roughly 15-30 km", d)
	}
	// Identical points -> 0.
	if z := haversineKm(10, 10, 10, 10); math.Abs(z) > 1e-9 {
		t.Fatalf("haversineKm(identical) = %.6f, want 0", z)
	}
}

func TestParseLatLong(t *testing.T) {
	cases := []struct {
		in   string
		lat  float64
		long float64
		ok   bool
	}{
		{"31.55,74.35", 31.55, 74.35, true},
		{" 31.55 , 74.35 ", 31.55, 74.35, true},
		{"lahore", 0, 0, false},
		{"31.55", 0, 0, false},
		{"a,b", 0, 0, false},
	}
	for _, tc := range cases {
		lat, long, ok := parseLatLong(tc.in)
		if ok != tc.ok || (ok && (lat != tc.lat || long != tc.long)) {
			t.Fatalf("parseLatLong(%q) = (%v,%v,%v), want (%v,%v,%v)", tc.in, lat, long, ok, tc.lat, tc.long, tc.ok)
		}
	}
}

func TestParseDealTime(t *testing.T) {
	if _, ok := parseDealTime("2027-06-16T23:59:00"); !ok {
		t.Fatal("parseDealTime failed on naive datetime")
	}
	if _, ok := parseDealTime("2027-06-16"); !ok {
		t.Fatal("parseDealTime failed on date-only")
	}
	if _, ok := parseDealTime(""); ok {
		t.Fatal("parseDealTime should fail on empty string")
	}
	if _, ok := parseDealTime("not-a-date"); ok {
		t.Fatal("parseDealTime should fail on garbage")
	}
}

func TestSortDealsByDiscountDesc(t *testing.T) {
	deals := []dealWithMerchant{
		{pkbDeal: pkbDeal{PercentageValue: 20}},
		{pkbDeal: pkbDeal{PercentageValue: 50}},
		{pkbDeal: pkbDeal{PercentageValue: 35}},
	}
	sortDealsByDiscountDesc(deals)
	if deals[0].PercentageValue != 50 || deals[1].PercentageValue != 35 || deals[2].PercentageValue != 20 {
		t.Fatalf("sortDealsByDiscountDesc order wrong: %d,%d,%d", deals[0].PercentageValue, deals[1].PercentageValue, deals[2].PercentageValue)
	}
}
