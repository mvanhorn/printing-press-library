// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"
	"testing"
)

func TestResolveVenue(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		vf        venueFlags
		wantVenue string
		wantErr   string // substring; empty = no error expected
	}{
		{"default is all", venueFlags{venue: "all"}, "all", ""},
		{"explicit --venue=polymarket", venueFlags{venue: "polymarket"}, "polymarket", ""},
		{"explicit --venue=kalshi", venueFlags{venue: "kalshi"}, "kalshi", ""},
		{"--polymarket shortcut", venueFlags{venue: "all", polymarket: true}, "polymarket", ""},
		{"--kalshi shortcut", venueFlags{venue: "all", kalshi: true}, "kalshi", ""},

		{"--polymarket with --venue=polymarket is compatible", venueFlags{venue: "polymarket", polymarket: true}, "polymarket", ""},
		{"--kalshi with --venue=kalshi is compatible", venueFlags{venue: "kalshi", kalshi: true}, "kalshi", ""},

		{"--polymarket --kalshi conflict", venueFlags{venue: "all", polymarket: true, kalshi: true}, "", "mutually exclusive"},
		{"--polymarket --venue=kalshi conflict", venueFlags{venue: "kalshi", polymarket: true}, "", "conflicts with --venue=kalshi"},
		{"--kalshi --venue=polymarket conflict", venueFlags{venue: "polymarket", kalshi: true}, "", "conflicts with --venue=polymarket"},
		{"invalid --venue value", venueFlags{venue: "fanduel"}, "", `invalid --venue "fanduel"`},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := resolveVenue(c.vf)
			if c.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if got != c.wantVenue {
					t.Errorf("venue = %q, want %q", got, c.wantVenue)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil (venue=%q)", c.wantErr, got)
			}
			if !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("error = %q, want substring %q", err.Error(), c.wantErr)
			}
		})
	}
}

// TestTopicVenueScopingRejectsConflicts is a fast smoke that the topic
// command surfaces resolveVenue errors via its flag wiring.
func TestTopicVenueScopingRejectsConflicts(t *testing.T) {
	t.Parallel()

	rootCmd := RootCmd()
	rootCmd.SetArgs([]string{"topic", "foo", "--polymarket", "--kalshi"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error from conflicting --polymarket --kalshi, got nil")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("error = %q, want substring %q", err.Error(), "mutually exclusive")
	}
}

// TestCompareRejectsSingleVenueScope locks the contract that compare
// errors when scoped to one venue, since pairing requires both.
func TestCompareRejectsSingleVenueScope(t *testing.T) {
	t.Parallel()

	rootCmd := RootCmd()
	rootCmd.SetArgs([]string{"compare", "election", "--polymarket"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error from compare --polymarket, got nil")
	}
	if !strings.Contains(err.Error(), "compare requires both venues") {
		t.Errorf("error = %q, want substring %q", err.Error(), "compare requires both venues")
	}
}
