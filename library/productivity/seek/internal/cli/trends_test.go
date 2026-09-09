// Copyright 2026 Paul Siola and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

// TestBucketRangeMaterializesEmptyPeriods is the regression for the
// "Trend Deltas Skip Empty Periods" review finding: every calendar period in
// the window must be present so a zero-listing month still gets a bucket and
// deltas compare adjacent periods.
func TestBucketRangeMaterializesEmptyPeriods(t *testing.T) {
	start := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)
	got := bucketRange(start, end, "month")
	want := []string{"2026-01", "2026-02", "2026-03", "2026-04"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("bucketRange month = %v, want %v", got, want)
	}

	weeks := bucketRange(start, start.AddDate(0, 0, 21), "week")
	if len(weeks) < 3 || len(weeks) > 4 {
		t.Fatalf("bucketRange week over 21 days = %v (want 3-4 buckets)", weeks)
	}
	for i := 1; i < len(weeks); i++ {
		if weeks[i] <= weeks[i-1] {
			t.Fatalf("bucketRange week not sorted/unique: %v", weeks)
		}
	}

	if bucketRange(time.Time{}, end, "month") != nil {
		t.Error("bucketRange with zero start should return nil")
	}
	if bucketRange(end, start, "month") != nil {
		t.Error("bucketRange with end before start should return nil")
	}
}

// TestNovelTrendsHelpWires smoke-tests that the trends command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelTrendsHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"trends", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("trends --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "trends"} {
		if !strings.Contains(help, want) {
			t.Fatalf("trends --help missing %q in output:\n%s", want, help)
		}
	}
}
