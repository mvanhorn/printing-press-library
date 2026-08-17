// Copyright 2026 Allen Lew and contributors. Licensed under Apache-2.0. See LICENSE.
// Behavior tests for the pure logic backing Forkable's novel commands.

package cli

import (
	"testing"
	"time"
)

func TestParseLooseSince(t *testing.T) {
	day := 24 * time.Hour
	cases := []struct {
		in   string
		want time.Duration
	}{
		{"", 0},
		{"90d", 90 * day},
		{"12w", 12 * 7 * day},
		{"6mo", 6 * 30 * day},
		{"1y", 365 * day},
		{"24h", 24 * time.Hour},
		{"30", 30 * day}, // bare integer => days
		{"garbage", 0},
	}
	for _, c := range cases {
		if got := parseLooseSince(c.in); got != c.want {
			t.Errorf("parseLooseSince(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestDateOnOrAfter(t *testing.T) {
	cases := []struct {
		date, cutoff string
		want         bool
	}{
		{"2026-07-20", "", true}, // empty cutoff always matches
		{"2026-07-20T10:00:00Z", "2026-07-01", true},
		{"2026-06-01", "2026-07-01", false},
		{"2026-07-01", "2026-07-01", true}, // boundary inclusive
		{"bad", "2026-07-01", true},        // fail-open on short/unparseable
	}
	for _, c := range cases {
		if got := dateOnOrAfter(c.date, c.cutoff); got != c.want {
			t.Errorf("dateOnOrAfter(%q,%q) = %v, want %v", c.date, c.cutoff, got, c.want)
		}
	}
}

func TestConflictTerms(t *testing.T) {
	cases := []struct {
		meal  string
		terms []string
		want  int
	}{
		{"Grilled Chicken Salad", []string{"chicken"}, 1},
		{"Grilled Chicken Salad", []string{"CHICKEN"}, 1}, // case-insensitive
		{"Vegan Buddha Bowl", []string{"chicken", "beef"}, 0},
		{"Shrimp Pad Thai", []string{"shrimp", "peanut"}, 1},
		{"Cheese Pizza", []string{"", "  "}, 0}, // blank terms ignored
	}
	for _, c := range cases {
		if got := conflictTerms(c.meal, c.terms); len(got) != c.want {
			t.Errorf("conflictTerms(%q,%v) matched %d, want %d", c.meal, c.terms, len(got), c.want)
		}
	}
}

func TestPeriodKey(t *testing.T) {
	cases := []struct {
		iso, by, want string
	}{
		{"2026-07-20", "month", "2026-07"},
		{"2026-02-15", "month", "2026-02"},
		{"2026-07-20", "week", "2026-W30"},
	}
	for _, c := range cases {
		if got := periodKey(c.iso, c.by); got != c.want {
			t.Errorf("periodKey(%q,%q) = %q, want %q", c.iso, c.by, got, c.want)
		}
	}
}

func TestInt64sToList(t *testing.T) {
	if got := int64sToList([]int64{1, 2, 3}); got != "[1,2,3]" {
		t.Errorf("int64sToList = %q, want [1,2,3]", got)
	}
	if got := int64sToList(nil); got != "[]" {
		t.Errorf("int64sToList(nil) = %q, want []", got)
	}
}

func TestVenueLabel(t *testing.T) {
	if got := (rawVenue{Name: "Mixt", DisplayName: "MIXT SF"}).label(); got != "MIXT SF" {
		t.Errorf("label prefers displayName, got %q", got)
	}
	if got := (rawVenue{Name: "Mixt"}).label(); got != "Mixt" {
		t.Errorf("label falls back to name, got %q", got)
	}
}
