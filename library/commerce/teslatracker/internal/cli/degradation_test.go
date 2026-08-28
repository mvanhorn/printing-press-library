// Copyright 2026 michegz and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// TestNovelDegradationHelpWires smoke-tests that the degradation command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelDegradationHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"degradation", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("degradation --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "degradation"} {
		if !strings.Contains(help, want) {
			t.Fatalf("degradation --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestDegradationAggregate_BelowMinCohortWithholdsStats(t *testing.T) {
	dbPath := seedVehicleMirror(t, rangeCars(3)...)
	stdout, stderr, err := runRootArgs(t, "degradation", "--db", dbPath, "--json", "--no-learn")
	if err != nil {
		t.Fatalf("degradation: %v (stderr=%q stdout=%q)", err, stderr, stdout)
	}
	var view map[string]any
	if err := json.Unmarshal([]byte(stdout), &view); err != nil {
		t.Fatalf("decode degradation JSON: %v (stdout=%q)", err, stdout)
	}
	for _, key := range []string{"median_retained_pct", "worst_retained_pct", "best_retained_pct"} {
		if _, ok := view[key]; ok {
			t.Fatalf("undersized cohort still reported %s = %v", key, view[key])
		}
	}
	note, _ := view["cohort_note"].(string)
	if !strings.Contains(note, "below the floor") {
		t.Fatalf("cohort_note = %q, want a min-cohort floor explanation", note)
	}
	if n, _ := view["n"].(float64); int(n) != 3 {
		t.Fatalf("n = %v, want 3", view["n"])
	}
}

func TestDegradationAggregate_MinCohortReportsStats(t *testing.T) {
	dbPath := seedVehicleMirror(t, rangeCars(minCohort)...)
	stdout, stderr, err := runRootArgs(t, "degradation", "--db", dbPath, "--json", "--no-learn")
	if err != nil {
		t.Fatalf("degradation: %v (stderr=%q stdout=%q)", err, stderr, stdout)
	}
	var view map[string]any
	if err := json.Unmarshal([]byte(stdout), &view); err != nil {
		t.Fatalf("decode degradation JSON: %v (stdout=%q)", err, stdout)
	}
	for _, key := range []string{"median_retained_pct", "worst_retained_pct", "best_retained_pct"} {
		if _, ok := view[key]; !ok {
			t.Fatalf("n=%d cohort missing %s (stdout=%q)", minCohort, key, stdout)
		}
	}
	if _, ok := view["cohort_note"]; ok {
		t.Fatalf("n=%d cohort should not carry cohort_note: %v", minCohort, view["cohort_note"])
	}
}

func rangeCars(n int) []Vehicle {
	out := make([]Vehicle, 0, n)
	for i := 0; i < n; i++ {
		rated, actual := 310, 270+i
		out = append(out, Vehicle{
			VIN:         fmt.Sprintf("5YJ3E1EA7LF0000%02d", i+20),
			Year:        intPtr(2021),
			Model:       "Model 3",
			Range:       intPtr(rated),
			ActualRange: intPtr(actual),
		})
	}
	return out
}
