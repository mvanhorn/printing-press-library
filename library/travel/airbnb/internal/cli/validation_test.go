package cli

import (
	"strings"
	"testing"
	"time"
)

func TestValidatePropertyType(t *testing.T) {
	for _, ok := range []string{"", "entire_home", "private_room", "shared_room", "hotel_room"} {
		if err := validatePropertyType(ok); err != nil {
			t.Errorf("validatePropertyType(%q) = %v; want nil", ok, err)
		}
	}
	for _, bad := range []string{"bogus", "Entire_home", "ENTIRE_HOME", "house", "all"} {
		err := validatePropertyType(bad)
		if err == nil {
			t.Errorf("validatePropertyType(%q) = nil; want error", bad)
			continue
		}
		if !strings.Contains(err.Error(), "expected one of") {
			t.Errorf("validatePropertyType(%q) message %q does not list allowed values", bad, err.Error())
		}
	}
}

func TestValidateDates(t *testing.T) {
	// Pin "today" to 2026-05-03 so the past-date case is stable.
	dateNow = func() time.Time {
		return time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC)
	}
	defer func() {
		dateNow = func() time.Time {
			now := time.Now().Local()
			return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		}
	}()

	cases := []struct {
		name      string
		checkin   string
		checkout  string
		wantError string
	}{
		{"both empty ok", "", "", ""},
		{"valid future stay", "2026-05-06", "2026-05-09", ""},
		{"only checkin set", "2026-05-06", "", "must both be set"},
		{"only checkout set", "", "2026-05-09", "must both be set"},
		{"reversed dates", "2026-05-09", "2026-05-06", "must be after"},
		{"same day", "2026-05-06", "2026-05-06", "must be after"},
		{"checkin in past", "2025-01-01", "2025-01-04", "is in the past"},
		{"malformed checkin", "5/6/2026", "2026-05-09", "expected YYYY-MM-DD"},
		{"malformed checkout", "2026-05-06", "May 9", "expected YYYY-MM-DD"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateDates(tc.checkin, tc.checkout)
			if tc.wantError == "" {
				if err != nil {
					t.Fatalf("got error %v; want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("got nil; want error containing %q", tc.wantError)
			}
			if !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.wantError)
			}
		})
	}
}
