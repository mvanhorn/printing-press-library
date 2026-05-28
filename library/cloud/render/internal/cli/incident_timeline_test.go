// Copyright 2026 Giuliano Giacaglia and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"
	"time"
)

func TestParseTimelineWindow(t *testing.T) {
	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		in      string
		want    time.Time
		wantErr bool
	}{
		{"hours", "2h", now.Add(-2 * time.Hour), false},
		{"minutes", "30m", now.Add(-30 * time.Minute), false},
		{"days", "1d", now.AddDate(0, 0, -1), false},
		{"multi-days", "7d", now.AddDate(0, 0, -7), false},
		{"now", "now", now, false},
		{"empty_means_now", "", now, false},
		{"rfc3339", "2026-05-09T09:00:00Z", time.Date(2026, 5, 9, 9, 0, 0, 0, time.UTC), false},
		{"malformed", "yesterday", time.Time{}, true},
		{"nonsense_d", "abcd", time.Time{}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseTimelineWindow(tc.in, now)
			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error for %q, got nil (got time %v)", tc.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tc.in, err)
			}
			if !got.Equal(tc.want) {
				t.Errorf("parseTimelineWindow(%q): got %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}
