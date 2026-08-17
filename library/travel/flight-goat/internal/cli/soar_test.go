// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestSoarPricePreservesCents(t *testing.T) {
	cases := []struct {
		currency string
		price    float64
		want     string
	}{
		{"USD", 94.98, "USD 94.98"},
		{"USD", 309.4, "USD 309.40"},
		{"usd", 144, "USD 144.00"},
		{"", 87.4, "USD 87.40"},
	}
	for _, c := range cases {
		if got := soarPrice(c.currency, c.price); got != c.want {
			t.Errorf("soarPrice(%q, %v) = %q, want %q", c.currency, c.price, got, c.want)
		}
	}
}

// runSoar executes the soar command with args and returns combined output.
func runSoar(t *testing.T, flags *rootFlags, args ...string) (string, error) {
	t.Helper()
	cmd := newSoarCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func TestSoarMultiCityDryRun(t *testing.T) {
	out, err := runSoar(t, &rootFlags{dryRun: true},
		"--segment", "IAH>FCO@2026-09-27", "--segment", "CAI>SEA@2026-10-04", "--class", "first")
	if err != nil {
		t.Fatalf("dry run: %v\n%s", err, out)
	}
	for _, want := range []string{
		"multi-city IAH>FCO@2026-09-27 CAI>SEA@2026-10-04",
		"trip=multicity",
		"slices=IAH-FCO-260927-first%3BCAI-SEA-261004-first",
		"/flights/iah/sea/260927/?",
		"(dry run - no request sent)",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("dry-run output missing %q:\n%s", want, out)
		}
	}
}

func TestSoarSingleSegmentRejected(t *testing.T) {
	// One --segment plus positionals must not silently ignore the segment.
	_, err := runSoar(t, &rootFlags{dryRun: true},
		"SEA", "DEN", "2026-09-27", "--segment", "IAH>FCO@2026-09-27")
	if err == nil || !strings.Contains(err.Error(), ">= 2") {
		t.Fatalf("want single-segment error, got %v", err)
	}
}

func TestSoarSegmentsRejectReturn(t *testing.T) {
	_, err := runSoar(t, &rootFlags{dryRun: true},
		"--segment", "IAH>FCO@2026-09-27", "--segment", "CAI>SEA@2026-10-04", "--return", "2026-10-10")
	if err == nil || !strings.Contains(err.Error(), "--return") {
		t.Fatalf("want --return conflict error, got %v", err)
	}
}

func TestSoarPositionalsStillRequired(t *testing.T) {
	// Without segments the classic 3-positional contract still applies.
	if _, err := runSoar(t, &rootFlags{dryRun: true}, "SEA", "DEN"); err == nil {
		t.Fatal("want arg-count error for 2 positionals without --segment")
	}
}
