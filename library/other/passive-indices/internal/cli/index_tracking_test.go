// Copyright 2026 Mayank Lavania and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestNovelIndexTrackingHelpWires smoke-tests that the index tracking command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelIndexTrackingHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"index", "tracking", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("index tracking --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "tracking"} {
		if !strings.Contains(help, want) {
			t.Fatalf("index tracking --help missing %q in output:\n%s", want, help)
		}
	}
	if !strings.Contains(help, "disclosed expense ratio") || strings.Contains(help, "NAV fidelity") {
		t.Fatalf("index tracking help must describe expense-ratio ordering without claiming calculated fidelity:\n%s", help)
	}
}

func TestSortTrackingMetricsUsesExpenseRatioOnly(t *testing.T) {
	rows := []trackingFidelityRow{
		{SchemeID: "worse-tracking", ExpenseRatio: 0.67, TrackingError: 0.12},
		{SchemeID: "better-tracking", ExpenseRatio: 1.06, TrackingError: 0.11},
		{SchemeID: "undisclosed", ExpenseRatio: 0, TrackingError: 0.01},
		{SchemeID: "cheapest", ExpenseRatio: 0.20, TrackingError: 0.50},
	}
	sortTrackingMetrics(rows)
	want := []string{"cheapest", "worse-tracking", "better-tracking", "undisclosed"}
	for i := range want {
		if rows[i].SchemeID != want[i] {
			t.Fatalf("ranked scheme %d = %q, want %q", i, rows[i].SchemeID, want[i])
		}
	}
}
