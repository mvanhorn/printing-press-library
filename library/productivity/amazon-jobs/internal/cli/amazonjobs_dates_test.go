// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored tests for the amazon-jobs date/freshness helpers.

package cli

import (
	"testing"
	"time"
)

func TestParsePostedDate(t *testing.T) {
	tests := []struct {
		name   string
		in     string
		wantOK bool
		wantY  int
		wantM  time.Month
		wantD  int
	}{
		{name: "standard two-digit day", in: "July 24, 2026", wantOK: true, wantY: 2026, wantM: time.July, wantD: 24},
		{name: "double space single-digit day", in: "May  9, 2026", wantOK: true, wantY: 2026, wantM: time.May, wantD: 9},
		{name: "single space single-digit day", in: "May 9, 2026", wantOK: true, wantY: 2026, wantM: time.May, wantD: 9},
		{name: "leading and trailing space", in: "  October  8, 2025 ", wantOK: true, wantY: 2025, wantM: time.October, wantD: 8},
		{name: "empty", in: "", wantOK: false},
		{name: "blank", in: "   ", wantOK: false},
		{name: "iso format is not what the API sends", in: "2026-07-24", wantOK: false},
		{name: "garbage", in: "sometime last week", wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parsePostedDate(tt.in)
			if ok != tt.wantOK {
				t.Fatalf("parsePostedDate(%q) ok = %v, want %v", tt.in, ok, tt.wantOK)
			}
			if !tt.wantOK {
				return
			}
			if got.Year() != tt.wantY || got.Month() != tt.wantM || got.Day() != tt.wantD {
				t.Errorf("parsePostedDate(%q) = %v, want %d-%s-%d", tt.in, got, tt.wantY, tt.wantM, tt.wantD)
			}
			if h, m, s := got.Clock(); h != 0 || m != 0 || s != 0 {
				t.Errorf("parsePostedDate(%q) should land on midnight, got clock %02d:%02d:%02d", tt.in, h, m, s)
			}
		})
	}
}

func TestParseRelativeAge(t *testing.T) {
	tests := []struct {
		name   string
		in     string
		want   time.Duration
		wantOK bool
	}{
		{name: "about N hours", in: "about 21 hours", want: 21 * time.Hour, wantOK: true},
		{name: "about one hour", in: "about 1 hour", want: time.Hour, wantOK: true},
		{name: "N day", in: "1 day", want: 24 * time.Hour, wantOK: true},
		{name: "N minutes", in: "29 minutes", want: 29 * time.Minute, wantOK: true},
		{name: "plural days", in: "3 days", want: 72 * time.Hour, wantOK: true},
		{name: "article form", in: "about an hour", want: time.Hour, wantOK: true},
		{name: "weeks", in: "2 weeks", want: 14 * 24 * time.Hour, wantOK: true},
		{name: "months", in: "3 months", want: 90 * 24 * time.Hour, wantOK: true},
		{name: "case insensitive", in: "About 5 Hours", want: 5 * time.Hour, wantOK: true},
		{name: "empty", in: "", wantOK: false},
		{name: "unknown unit", in: "4 fortnights", wantOK: false},
		{name: "no unit", in: "21", wantOK: false},
		{name: "trailing junk", in: "about 21 hours ago", wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseRelativeAge(tt.in)
			if ok != tt.wantOK {
				t.Fatalf("parseRelativeAge(%q) ok = %v, want %v", tt.in, ok, tt.wantOK)
			}
			if tt.wantOK && got != tt.want {
				t.Errorf("parseRelativeAge(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// TestUpdatedDiverged uses the exact live shapes that motivated the marker: a
// req posted in August 2025 still reporting "about 21 hours" updated.
func TestUpdatedDiverged(t *testing.T) {
	now := time.Date(2026, time.July, 25, 12, 0, 0, 0, time.Local)

	tests := []struct {
		name    string
		posted  string
		updated string
		want    bool
	}{
		{name: "months old but updated hours ago", posted: "August 19, 2025", updated: "about 21 hours", want: true},
		{name: "months old, double-spaced day", posted: "October  8, 2025", updated: "1 day", want: true},
		{name: "posted today", posted: "July 25, 2026", updated: "about 2 hours", want: false},
		{name: "posted inside the 14 day window", posted: "July 20, 2026", updated: "1 day", want: false},
		{name: "old and also stale update", posted: "March 9, 2026", updated: "3 months", want: false},
		{name: "unparseable posted date is never marked", posted: "whenever", updated: "1 day", want: false},
		{name: "unparseable updated time is never marked", posted: "March 9, 2026", updated: "recently", want: false},
		{name: "both empty", posted: "", updated: "", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			j := Job{PostedDate: tt.posted, UpdatedTime: tt.updated}
			if got := updatedDiverged(j, now); got != tt.want {
				t.Errorf("updatedDiverged(posted=%q, updated=%q) = %v, want %v",
					tt.posted, tt.updated, got, tt.want)
			}
		})
	}
}

func TestPostedWithinCutoff(t *testing.T) {
	now := time.Date(2026, time.July, 25, 14, 30, 0, 0, time.Local)

	// 7d must land on the start of July 18 so the whole of that day counts.
	got := postedWithinCutoff(now, 7*24*time.Hour)
	want := time.Date(2026, time.July, 18, 0, 0, 0, 0, time.Local)
	if !got.Equal(want) {
		t.Errorf("postedWithinCutoff(7d) = %v, want %v", got, want)
	}

	// 24h anchors to the start of the previous day, not to 14:30 yesterday.
	got = postedWithinCutoff(now, 24*time.Hour)
	want = time.Date(2026, time.July, 24, 0, 0, 0, 0, time.Local)
	if !got.Equal(want) {
		t.Errorf("postedWithinCutoff(24h) = %v, want %v", got, want)
	}

	// A sub-day window still resolves to a whole date, because posted_date has
	// no sub-day component to compare against.
	got = postedWithinCutoff(now, time.Hour)
	want = time.Date(2026, time.July, 25, 0, 0, 0, 0, time.Local)
	if !got.Equal(want) {
		t.Errorf("postedWithinCutoff(1h) = %v, want %v", got, want)
	}
}

func TestAnnotateFreshness(t *testing.T) {
	jobs := []Job{
		{PostedDate: "August 19, 2025", UpdatedTime: "about 21 hours"},
		{PostedDate: time.Now().Format(postedDateLayout), UpdatedTime: "about 2 hours"},
	}
	annotateFreshness(jobs)
	if !jobs[0].UpdatedDiverged {
		t.Error("stale req with fresh updated_time should be marked")
	}
	if jobs[1].UpdatedDiverged {
		t.Error("freshly posted req should not be marked")
	}
}
