// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"
	"testing"
)

func TestParseMultiCitySegments_Valid(t *testing.T) {
	in := []string{
		"SFO>NRT@2026-08-15",
		" nrt > icn @ 2026-08-28 ",
		"ICN>SFO@2026-09-05",
	}
	got, err := parseMultiCitySegments(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len=%d want 3", len(got))
	}
	if got[1].Origin != "NRT" || got[1].Destination != "ICN" || got[1].DepartureDate != "2026-08-28" {
		t.Errorf("seg 2 normalize wrong: %+v", got[1])
	}
}

func TestParseMultiCitySegments_MissingDate(t *testing.T) {
	_, err := parseMultiCitySegments([]string{"SFO>NRT"})
	if err == nil || !strings.Contains(err.Error(), "@YYYY-MM-DD") {
		t.Errorf("expected missing-date error, got %v", err)
	}
}

func TestParseMultiCitySegments_BadRoute(t *testing.T) {
	_, err := parseMultiCitySegments([]string{"SFO-NRT@2026-08-15"})
	if err == nil || !strings.Contains(err.Error(), "ORIG>DEST") {
		t.Errorf("expected ORIG>DEST shape error, got %v", err)
	}
}

func TestParseMultiCitySegments_ShortCode(t *testing.T) {
	_, err := parseMultiCitySegments([]string{"AB>NRT@2026-08-15"})
	if err == nil || !strings.Contains(err.Error(), "3-letter") {
		t.Errorf("expected 3-letter error, got %v", err)
	}
}

func TestParseMultiCitySegments_RejectsBadDateFormat(t *testing.T) {
	// Regression: when --provider=kayak skips gflights.MultiCityBookingURL,
	// the parser is the only date-format validator. A slash-separated date
	// must produce a user-facing error rather than a misleading "Kayak
	// changed shape" downstream failure.
	for _, bad := range []string{"2026/08/15", "08-15-2026", "next-tuesday"} {
		_, err := parseMultiCitySegments([]string{"SFO>NRT@" + bad})
		if err == nil || !strings.Contains(err.Error(), "YYYY-MM-DD") {
			t.Errorf("date %q: expected YYYY-MM-DD error, got %v", bad, err)
		}
	}
}
