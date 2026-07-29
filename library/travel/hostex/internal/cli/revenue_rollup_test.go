// Copyright 2026 bust011r and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestNovelRevenueRollupHelpWires smoke-tests that the revenue-rollup command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelRevenueRollupHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"revenue-rollup", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("revenue-rollup --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "revenue-rollup", "--max-pages"} {
		if !strings.Contains(help, want) {
			t.Fatalf("revenue-rollup --help missing %q in output:\n%s", want, help)
		}
	}
}

// TestRevenueScanTruncated guards the truncation detector that keeps
// revenue-rollup from presenting a partial ledger as a complete one. The
// pagination loop is capped, so a large account can outrun it; when that
// happens the command must report truncated=true rather than silently
// understating income, expense and net.
func TestRevenueScanTruncated(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name          string
		scanned       int
		reportedTotal int
		hitPageCap    bool
		want          bool
	}{
		// No `total` from the API — the page cap is the only signal.
		{"short page, no total", 240, 0, false, false},
		{"page cap hit, no total", 5000, 0, true, true},
		// `total` present: it overrides the heuristic in both directions.
		{"page cap hit but ledger exactly fills it", 5000, 5000, true, false},
		{"page cap hit and ledger continues", 5000, 7200, true, true},
		{"complete scan under the cap", 120, 120, false, false},
		// A short page ended the loop yet the API says more records exist.
		// Something is off upstream; flag it rather than trust the sums.
		{"short page but total says more remain", 100, 120, false, true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := revenueScanTruncated(tc.scanned, tc.reportedTotal, tc.hitPageCap)
			if got != tc.want {
				t.Errorf("revenueScanTruncated(%d, %d, %v) = %v, want %v",
					tc.scanned, tc.reportedTotal, tc.hitPageCap, got, tc.want)
			}
		})
	}
}
