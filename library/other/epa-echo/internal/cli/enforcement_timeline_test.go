// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestNovelEnforcementTimelineHelpWires smoke-tests that the enforcement timeline command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelEnforcementTimelineHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"enforcement", "timeline", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("enforcement timeline --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "timeline"} {
		if !strings.Contains(help, want) {
			t.Fatalf("enforcement timeline --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestTimelineEntriesExcludesCoverageWindows(t *testing.T) {
	report := map[string]any{
		"EnforcementComplianceSummaries": map[string]any{
			"ProgramDates": []any{map[string]any{
				"StartDate": "07/11/2021",
				"EndDate":   "06/30/2026",
			}},
			"Actions": []any{map[string]any{
				"EnforcementActionDate": "03/04/2025",
				"ActionType":            "Formal",
			}},
		},
	}

	entries := timelineEntries(report)
	if len(entries) != 1 {
		t.Fatalf("timelineEntries() returned %d entries, want 1: %#v", len(entries), entries)
	}
	if got := entries[0]["date"]; got != "2025-03-04" {
		t.Fatalf("date = %v, want 2025-03-04", got)
	}
	if got := entries[0]["date_field"]; got != "EnforcementActionDate" {
		t.Fatalf("date_field = %v, want EnforcementActionDate", got)
	}
}
