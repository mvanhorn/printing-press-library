// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

// TestFormatLocalTimeCarriesZoneOffset pins the whois local_time contract: the
// stamp must name the user's zone offset, not "Z". The regression it guards is
// now.Add(offset).Format(RFC3339), which shifts a UTC-located instant and so
// prints the user's wall clock under a UTC label - a value that parses back
// tz_offset seconds away from the true moment.
func TestFormatLocalTimeCarriesZoneOffset(t *testing.T) {
	now := time.Date(2026, 8, 3, 16, 29, 27, 0, time.UTC)

	tests := []struct {
		name     string
		offset   int
		label    string
		want     string
		wantSame bool // formatted value must refer to the same instant as now
	}{
		{"america/new_york dst", -4 * 3600, "EDT", "2026-08-03T12:29:27-04:00", true},
		{"india half hour", 5*3600 + 1800, "IST", "2026-08-03T21:59:27+05:30", true},
		{"utc user", 0, "UTC", "2026-08-03T16:29:27Z", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := formatLocalTime(now, tc.offset, tc.label)
			if got != tc.want {
				t.Errorf("formatLocalTime(%d) = %q, want %q", tc.offset, got, tc.want)
			}
			if strings.HasSuffix(got, "Z") && tc.offset != 0 {
				t.Errorf("formatLocalTime(%d) = %q: non-UTC zone must not be stamped Z", tc.offset, got)
			}
			parsed, err := time.Parse(time.RFC3339, got)
			if err != nil {
				t.Fatalf("formatLocalTime(%d) = %q: not valid RFC3339: %v", tc.offset, got, err)
			}
			if tc.wantSame && !parsed.Equal(now) {
				t.Errorf("formatLocalTime(%d) = %q parses to %v, want the same instant as %v", tc.offset, got, parsed.UTC(), now)
			}
		})
	}
}

// TestNovelUsersWhoisHelpWires smoke-tests that the users whois command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelUsersWhoisHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"users", "whois", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("users whois --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "whois"} {
		if !strings.Contains(help, want) {
			t.Fatalf("users whois --help missing %q in output:\n%s", want, help)
		}
	}
}
