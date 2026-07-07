// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"
	"time"
)

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("bad fixture time %q: %v", s, err)
	}
	return parsed
}

func TestFilterChangedSince(t *testing.T) {
	cutoff := mustTime(t, "2026-06-10T00:00:00Z")

	rows := []entityRow{
		{ResourceType: "orders", ID: "1", Name: "old", UpdateTime: mustTime(t, "2026-06-01T00:00:00Z")},
		{ResourceType: "orders", ID: "2", Name: "new", UpdateTime: mustTime(t, "2026-06-15T12:00:00Z")},
		{ResourceType: "ad-units", ID: "3", Name: "exactly-at-cutoff", UpdateTime: cutoff},
		{ResourceType: "ad-units", ID: "4", Name: "newest", UpdateTime: mustTime(t, "2026-06-16T09:00:00Z")},
		{ResourceType: "line-items", ID: "5", Name: "no-update-time"}, // zero time -> excluded
	}

	got := filterChangedSince(rows, cutoff)

	// Expect newest-first: id 4 (06-16), id 2 (06-15), id 3 (at cutoff).
	wantIDs := []string{"4", "2", "3"}
	if len(got) != len(wantIDs) {
		t.Fatalf("got %d rows, want %d: %+v", len(got), len(wantIDs), got)
	}
	for i, want := range wantIDs {
		if got[i].ID != want {
			t.Errorf("row[%d].ID = %q, want %q (order: %+v)", i, got[i].ID, want, got)
		}
	}

	// The old row and the undated row must be excluded.
	for _, r := range got {
		if r.ID == "1" {
			t.Error("row older than cutoff was not excluded")
		}
		if r.ID == "5" {
			t.Error("row with zero updateTime was not excluded")
		}
	}
}

func TestFilterChangedSinceEmpty(t *testing.T) {
	if got := filterChangedSince(nil, time.Now()); got == nil {
		t.Error("filterChangedSince(nil, ...) should return a non-nil empty slice")
	} else if len(got) != 0 {
		t.Errorf("filterChangedSince(nil, ...) = %+v, want empty", got)
	}
}

func TestParseUpdateTime(t *testing.T) {
	tests := []struct {
		in   string
		ok   bool
		want string // RFC3339 in UTC, only checked when ok
	}{
		{"2026-06-16T09:00:00Z", true, "2026-06-16T09:00:00Z"},
		{"2026-06-16T09:00:00.123456789Z", true, "2026-06-16T09:00:00Z"},
		{"2026-06-16T09:00:00+02:00", true, "2026-06-16T07:00:00Z"},
		{"2026-06-16 09:00:00", true, "2026-06-16T09:00:00Z"},
		{"2026-06-16", true, "2026-06-16T00:00:00Z"},
		{"", false, ""},
		{"not-a-time", false, ""},
	}
	for _, tc := range tests {
		got, ok := parseUpdateTime(tc.in)
		if ok != tc.ok {
			t.Errorf("parseUpdateTime(%q) ok = %v, want %v", tc.in, ok, tc.ok)
			continue
		}
		if ok {
			if g := got.UTC().Format(time.RFC3339); g != tc.want {
				t.Errorf("parseUpdateTime(%q) = %q, want %q", tc.in, g, tc.want)
			}
		}
	}
}
