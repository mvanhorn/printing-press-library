// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

// doctor_graph_test.go: unit tests for the U4 stale-graph classifier in
// doctor.go. The full doctor flow has integration coverage in
// doctor_happenstance_api_test.go; this file is scoped to the
// classifyGraphFreshness boundary cases.

package cli

import (
	"testing"
	"time"
)

func TestClassifyGraphFreshness(t *testing.T) {
	cases := []struct {
		name    string
		elapsed time.Duration
		want    graphFreshness
	}{
		{"fresh: 1 day", 1 * 24 * time.Hour, graphFreshnessOK},
		{"fresh: 30 days (under stale threshold)", 30 * 24 * time.Hour, graphFreshnessOK},
		{"fresh: 89 days (just under stale)", 89 * 24 * time.Hour, graphFreshnessOK},
		{"stale boundary: exactly 90 days", graphStaleThreshold, graphFreshnessStale},
		{"stale: 120 days", 120 * 24 * time.Hour, graphFreshnessStale},
		{"stale: 179 days (just under very_stale)", 179 * 24 * time.Hour, graphFreshnessStale},
		{"very_stale boundary: exactly 180 days", graphVeryStaleThreshold, graphFreshnessVeryStale},
		{"very_stale: 9 months", 270 * 24 * time.Hour, graphFreshnessVeryStale},
		{"very_stale: 1 year", 365 * 24 * time.Hour, graphFreshnessVeryStale},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyGraphFreshness(tc.elapsed)
			if got != tc.want {
				t.Errorf("classifyGraphFreshness(%s) = %q, want %q", tc.elapsed, got, tc.want)
			}
		})
	}
}

// TestGraphFreshnessThresholds locks the 90/180-day choice. SF-task
// evidence (live session 2026-04-19) shows queries succeed on a
// 9-month-old graph, so a 30-day stale trigger would have falsely
// flagged a working setup. 90 days catches genuine degradation; 180
// days marks "definitely needs re-upload". If thresholds change, this
// test forces the contributor to revisit the rationale.
func TestGraphFreshnessThresholds(t *testing.T) {
	if graphStaleThreshold != 90*24*time.Hour {
		t.Errorf("graphStaleThreshold = %s, want 90 days (SF-task evidence; see U4 plan)", graphStaleThreshold)
	}
	if graphVeryStaleThreshold != 180*24*time.Hour {
		t.Errorf("graphVeryStaleThreshold = %s, want 180 days", graphVeryStaleThreshold)
	}
}
