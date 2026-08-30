// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// Behavior tests for the hand-authored Peekaboo helpers.

package cli

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
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

// TestParseDealTimeReadsNaiveValuesAsPakistanTime pins the timezone contract.
// Peekaboo omits the offset on its validity timestamps, so reading them as UTC
// moved every expiry window five hours and skewed days_left around the cutoff.
// Values that do carry an offset must keep it.
func TestParseDealTimeReadsNaiveValuesAsPakistanTime(t *testing.T) {
	const wantOffset = 5 * 60 * 60 // PKT is UTC+05:00

	for _, in := range []string{"2027-06-16T23:59:00", "2027-06-16"} {
		got, ok := parseDealTime(in)
		if !ok {
			t.Fatalf("parseDealTime(%q) failed", in)
		}
		if _, offset := got.Zone(); offset != wantOffset {
			t.Fatalf("parseDealTime(%q) offset = %ds, want %ds: timezone-less values are Pakistan local, not UTC",
				in, offset, wantOffset)
		}
	}

	// A midnight-PKT deadline is 19:00 UTC the previous day. Read as UTC it
	// would land five hours late, which is what pulled deals across the cutoff.
	got, ok := parseDealTime("2027-06-17T00:00:00")
	if !ok {
		t.Fatal("parseDealTime failed on a midnight deadline")
	}
	if want := time.Date(2027, 6, 16, 19, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Fatalf("midnight PKT resolved to %s, want %s", got.UTC(), want)
	}

	// An explicit offset must survive untouched.
	got, ok = parseDealTime("2027-06-16T23:59:00Z")
	if !ok {
		t.Fatal("parseDealTime failed on an RFC3339 value")
	}
	if _, offset := got.Zone(); offset != 0 {
		t.Fatalf("explicit UTC offset was overridden: got %ds, want 0s", offset)
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

func TestListCityEntitiesPreservesPartialPageFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v8/entities" {
			http.NotFound(w, r)
			return
		}
		var body struct {
			Offset int `json:"offset"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		if body.Offset == 0 {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"nextPage":true,"entities":[{"id":7,"name":"First Merchant"}]}`))
			return
		}
		http.Error(w, "page unavailable", http.StatusBadGateway)
	}))
	defer server.Close()
	t.Setenv("PEEKABOO_BASE_URL", server.URL)
	t.Setenv("PEEKABOO_TOKEN", "test-token")
	t.Setenv("PEEKABOO_CONFIG_DIR", t.TempDir())
	t.Setenv("PEEKABOO_DATA_DIR", t.TempDir())
	t.Setenv("PEEKABOO_CACHE_DIR", t.TempDir())

	entities, scanned, err := listCityEntities(context.Background(), &rootFlags{timeout: time.Second}, pkbLocation{City: "Lahore", Country: "Pakistan"}, 1, 2, 1)
	if err == nil {
		t.Fatal("listCityEntities() error = nil, want page failure")
	}
	if len(entities) != 1 || entities[0].ID != 7 {
		t.Fatalf("listCityEntities() entities = %#v, want partial first page", entities)
	}
	if scanned != 1 {
		t.Fatalf("listCityEntities() scanned = %d, want 1", scanned)
	}
}
