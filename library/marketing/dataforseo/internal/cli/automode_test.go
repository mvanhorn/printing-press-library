// Copyright 2026 mazzsterr. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestResolveMode(t *testing.T) {
	cases := []struct {
		name      string
		batch     int
		threshold int
		override  string
		want      string
	}{
		{"small_default_threshold_live", 3, 0, "", "live"},
		{"at_threshold_live", 5, 0, "", "live"},
		{"above_default_threshold_standard", 6, 0, "", "standard"},
		{"large_batch_standard", 100, 0, "", "standard"},
		{"custom_threshold_10_live", 9, 10, "", "live"},
		{"custom_threshold_10_standard", 11, 10, "", "standard"},
		{"override_live_wins_on_large_batch", 1000, 0, "live", "live"},
		{"override_standard_wins_on_small_batch", 1, 0, "standard", "standard"},
		{"override_auto_falls_through", 3, 0, "auto", "live"},
		{"unknown_override_passes_through", 5, 0, "sandbox", "sandbox"},
		{"empty_string_equals_auto", 5, 0, "", "live"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ResolveMode(tc.batch, tc.threshold, tc.override)
			if got != tc.want {
				t.Fatalf("ResolveMode(%d, %d, %q) = %q, want %q",
					tc.batch, tc.threshold, tc.override, got, tc.want)
			}
		})
	}
}
