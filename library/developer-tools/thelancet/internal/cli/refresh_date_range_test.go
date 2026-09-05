// Hand-authored coverage for the refresh year-flag validation. Not generated.

package cli

import (
	"io"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

// quietCmd is a stand-in command whose usage output is discarded, so the
// validation helper can be exercised without noise on the test log.
func quietCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "refresh"}
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	return cmd
}

func TestRefreshYearBoundsAccepted(t *testing.T) {
	cases := []struct {
		name     string
		years    int
		yearsSet bool
		fromYear int
		toYear   int
		wantFrom int
		wantTo   int
	}{
		{"no flags", 0, false, 0, 0, 0, 0},
		{"from-year alone", 0, false, 1990, 0, 1990, 0},
		{"to-year alone", 0, false, 0, 2000, 0, 2000},
		{"both absolute bounds", 0, false, 1990, 2000, 1990, 2000},
		{"equal bounds", 0, false, 1995, 1995, 1995, 1995},
		{"rolling window", 3, true, 0, 0, time.Now().Year() - 2, 0},
		// An explicit "--years 0" on its own still means "no bound", so it
		// must keep working for scripts that pass the default through.
		{"explicit zero years alone", 0, true, 0, 0, 0, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			from, to, err := refreshYearBounds(quietCmd(), tc.years, tc.yearsSet, tc.fromYear, tc.toYear)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if from != tc.wantFrom || to != tc.wantTo {
				t.Errorf("bounds = (%d, %d), want (%d, %d)", from, to, tc.wantFrom, tc.wantTo)
			}
		})
	}
}

func TestRefreshYearBoundsRejected(t *testing.T) {
	nextYear := time.Now().Year() + 1
	cases := []struct {
		name     string
		years    int
		yearsSet bool
		fromYear int
		toYear   int
		wantMsg  string
	}{
		{"years with from-year", 5, true, 1990, 0, "cannot be combined"},
		{"years with to-year", 5, true, 0, 2000, "cannot be combined"},
		{"years with both", 5, true, 1990, 2000, "cannot be combined"},
		// Regression guard: an explicit "--years 0" is still the flag being
		// named, so it must collide with an absolute bound. Testing the value
		// instead of whether the flag was set let this combination run the
		// absolute range silently, contrary to the documented rule.
		{"explicit zero years with from-year", 0, true, 1990, 0, "cannot be combined"},
		{"explicit zero years with to-year", 0, true, 0, 1999, "cannot be combined"},
		{"from after to", 0, false, 2000, 1990, "must not be after"},
		{"from-year too old", 0, false, 1799, 0, "--from-year must be a calendar year"},
		{"to-year in the future", 0, false, 0, nextYear + 1, "--to-year must be a calendar year"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := refreshYearBounds(quietCmd(), tc.years, tc.yearsSet, tc.fromYear, tc.toYear)
			if err == nil {
				t.Fatalf("expected an error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantMsg) {
				t.Errorf("error = %q, want it to contain %q", err.Error(), tc.wantMsg)
			}
		})
	}
}
