// Copyright 2026 srijits and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"testing"
	"time"
)

func TestCanonicalBookmarkURL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"tracking and fragment", "HTTPS://Example.COM/post/?utm_source=newsletter&b=2&a=1#part", "https://example.com/post?a=1&b=2"},
		{"root slash retained", "https://example.com/", "https://example.com/"},
		{"opaque fallback", "  NOT A URL  ", "not a url"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := canonicalBookmarkURL(tt.in); got != tt.want {
				t.Fatalf("canonicalBookmarkURL(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestParseAge(t *testing.T) {
	t.Parallel()
	tests := []struct {
		raw     string
		want    time.Duration
		wantErr bool
	}{
		{"7d", 7 * 24 * time.Hour, false},
		{"2w", 14 * 24 * time.Hour, false},
		{"90m", 90 * time.Minute, false},
		{"-2d", 0, true},
	}
	for _, tt := range tests {
		got, err := parseAge(tt.raw, time.Hour)
		if (err != nil) != tt.wantErr {
			t.Fatalf("parseAge(%q) error = %v, wantErr %v", tt.raw, err, tt.wantErr)
		}
		if err == nil && got != tt.want {
			t.Fatalf("parseAge(%q) = %s, want %s", tt.raw, got, tt.want)
		}
	}
}

func TestOverlapScoreRewardsExplainableSignals(t *testing.T) {
	t.Parallel()
	base := localBookmark{Title: "SQLite bookmark research", Domain: "example.com", Tags: []string{"Go", "Databases"}}
	close := localBookmark{Title: "SQLite indexing notes", Domain: "example.com", Tags: []string{"go"}}
	unrelated := localBookmark{Title: "Garden flowers", Domain: "garden.test", Tags: []string{"plants"}}
	if got := overlapScore(base, close); got <= overlapScore(base, unrelated) {
		t.Fatalf("related score %v must exceed unrelated score", got)
	}
}
