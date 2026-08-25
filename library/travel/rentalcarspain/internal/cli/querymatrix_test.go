// Copyright 2026 serranoX and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"
	"testing"
)

// The matrix must always include the awkward shapes and record one-way as a
// documented skip rather than dropping it silently.
func TestBuildQueryShapes(t *testing.T) {
	shapes := buildQueryShapes(45)
	byName := map[string]queryShape{}
	for _, s := range shapes {
		byName[s.Name] = s
	}
	for _, want := range []string{"1-day", "7-day", "30-day", "far-future", "out-of-hours", "one-way"} {
		if _, ok := byName[want]; !ok {
			t.Errorf("matrix missing shape %q", want)
		}
	}
	if byName["1-day"].Days != 1 || byName["30-day"].Days != 30 {
		t.Errorf("day counts wrong: %+v / %+v", byName["1-day"], byName["30-day"])
	}
	// far-future must start well beyond the baseline.
	if byName["far-future"].OffsetDays <= byName["7-day"].OffsetDays {
		t.Errorf("far-future offset %d should exceed baseline %d",
			byName["far-future"].OffsetDays, byName["7-day"].OffsetDays)
	}
	// out-of-hours must actually use out-of-hours times.
	if byName["out-of-hours"].PickupTime == "10:00" {
		t.Errorf("out-of-hours shape should use a non-standard pickup time, got %q", byName["out-of-hours"].PickupTime)
	}
	// one-way is a recorded coverage gap, not a live probe.
	if !byName["one-way"].Skip || byName["one-way"].SkipReason == "" {
		t.Errorf("one-way should be a skip with a reason, got %+v", byName["one-way"])
	}
}

func TestCheckDurationMonotonic(t *testing.T) {
	// Healthy: totals grow with duration.
	ok := checkDurationMonotonic(map[string]float64{"1-day": 40, "7-day": 200, "30-day": 700})
	if ok.Status != selftestPass {
		t.Errorf("monotonic totals should pass, got %s: %s", ok.Status, ok.Detail)
	}
	// Regression: 30-day cheaper than 7-day (per-day/total confusion).
	bad := checkDurationMonotonic(map[string]float64{"1-day": 40, "7-day": 200, "30-day": 150})
	if bad.Status != selftestFail {
		t.Errorf("non-monotonic totals should fail, got %s: %s", bad.Status, bad.Detail)
	}
	// Missing a duration → skip, not fail.
	skip := checkDurationMonotonic(map[string]float64{"1-day": 40, "7-day": 200})
	if skip.Status != selftestSkip {
		t.Errorf("missing 30-day should skip, got %s", skip.Status)
	}
	if !strings.Contains(skip.Detail, "30-day") {
		t.Errorf("skip detail should mention the missing duration, got %q", skip.Detail)
	}
}
