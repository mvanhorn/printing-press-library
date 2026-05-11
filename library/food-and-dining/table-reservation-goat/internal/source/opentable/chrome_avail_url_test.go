// Copyright 2026 pejman-pour-moezzi. Licensed under Apache-2.0. See LICENSE.

package opentable

import (
	"strings"
	"testing"
)

// TestChromeAvailPageURL pins the navigation URL the chromedp fallback
// visits. PR #423 round-2 Greptile P1: when a caller passes a numeric
// OT ID (restSlug empty), the URL must gracefully fall back to
// /restaurant/profile/<id> instead of producing a broken /r/?covers=...
// URL. Akamai treats both routes as legitimate; the fallback ensures
// the numeric-ID path remains usable even when Akamai blocks the
// direct GraphQL path.
func TestChromeAvailPageURL(t *testing.T) {
	cases := []struct {
		name      string
		restID    int
		restSlug  string
		party     int
		date      string
		hhmm      string
		wantParts []string
		wantNot   []string
	}{
		{
			name:     "named slug uses /r/ route",
			restID:   3688,
			restSlug: "daniels-broiler-bellevue",
			party:    6, date: "2026-05-15", hhmm: "19:00",
			wantParts: []string{
				"/r/daniels-broiler-bellevue",
				"covers=6",
				"dateTime=2026-05-15T19:00",
			},
			wantNot: []string{"/restaurant/profile/"},
		},
		{
			name:     "empty slug falls back to /restaurant/profile/<id>",
			restID:   3688,
			restSlug: "",
			party:    6, date: "2026-05-15", hhmm: "19:00",
			wantParts: []string{
				"/restaurant/profile/3688",
				"covers=6",
				"dateTime=2026-05-15T19:00",
			},
			wantNot: []string{
				// Critical: numeric-ID path must NOT produce `/r/?covers=...`
				// (no slug between /r/ and ?). That would be a 404 on OT.
				"/r/?",
				// And must NOT produce `https://www.opentable.com/r/2026...`
				// (params bleeding into the path).
				"/r/2026",
			},
		},
		{
			name:     "slug with leading slash gets stripped",
			restID:   3688,
			restSlug: "/canlis",
			party:    2, date: "2026-05-15", hhmm: "19:00",
			wantParts: []string{"/r/canlis"},
			wantNot:   []string{"/r//canlis"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := chromeAvailPageURL(tc.restID, tc.restSlug, tc.party, tc.date, tc.hhmm)
			for _, want := range tc.wantParts {
				if !strings.Contains(got, want) {
					t.Errorf("URL missing %q\nfull: %s", want, got)
				}
			}
			for _, banned := range tc.wantNot {
				if strings.Contains(got, banned) {
					t.Errorf("URL contains banned substring %q\nfull: %s", banned, got)
				}
			}
		})
	}
}
