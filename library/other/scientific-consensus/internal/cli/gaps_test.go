// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"sort"
	"testing"
)

// Regression: population-probe findings must be emitted in a deterministic
// order. Go map iteration is randomized, so the gaps command iterates probes
// via sortedProbeLabels — which must cover every probe exactly once, sorted.
func TestSortedProbeLabels_DeterministicAndComplete(t *testing.T) {
	labels := sortedProbeLabels()
	if len(labels) != len(populationProbes) {
		t.Fatalf("labels = %d, want %d (one per probe)", len(labels), len(populationProbes))
	}
	if !sort.StringsAreSorted(labels) {
		t.Errorf("labels not sorted: %v", labels)
	}
	seen := map[string]bool{}
	for _, l := range labels {
		if _, ok := populationProbes[l]; !ok {
			t.Errorf("label %q not in populationProbes", l)
		}
		if seen[l] {
			t.Errorf("label %q duplicated", l)
		}
		seen[l] = true
	}
	// Stability across calls.
	again := sortedProbeLabels()
	for i := range labels {
		if labels[i] != again[i] {
			t.Fatalf("ordering unstable at %d: %q vs %q", i, labels[i], again[i])
		}
	}
}
