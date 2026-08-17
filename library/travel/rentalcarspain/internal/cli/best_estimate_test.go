// Copyright 2026 serranoX and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

// Only aggregator base rates without a stated zero excess are estimated (their
// total adds an assumed excess-cover figure). Direct quotes and aggregator
// zero-excess offers are fully quoted, so --real-only must keep them.
func TestStrategyIsEstimated(t *testing.T) {
	cases := map[string]bool{
		"direct":                      false,
		"aggregator-zero-excess":      false,
		"aggregator+standalone-cover": true,
	}
	for strategy, want := range cases {
		if got := strategyIsEstimated(strategy); got != want {
			t.Errorf("strategyIsEstimated(%q) = %v, want %v", strategy, got, want)
		}
	}
}
