// Copyright 2026 Michael Schreiber and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestNovelComplaintsNewHelpWires smoke-tests that the complaints new command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelComplaintsNewHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"complaints", "new", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("complaints new --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "new"} {
		if !strings.Contains(help, want) {
			t.Fatalf("complaints new --help missing %q in output:\n%s", want, help)
		}
	}
}

// TestIsIDKey guards against the "id" substring bug: a plain
// strings.Contains(lower, "id") check also matches fields like
// "confidential", "provided", and "residentState" that have nothing to do
// with an identifier.
func TestIsIDKey(t *testing.T) {
	t.Parallel()

	cases := []struct {
		key  string
		want bool
	}{
		{"id", true},
		{"ID", true},
		{"filingId", true},
		{"complaint_id", true},
		{"COMPLAINT_ID", true},
		{"complaint-id", true},
		{"confidential", false},
		{"provided", false},
		{"residentState", false},
		{"description", false},
	}

	for _, tc := range cases {
		t.Run(tc.key, func(t *testing.T) {
			t.Parallel()
			if got := isIDKey(tc.key); got != tc.want {
				t.Fatalf("isIDKey(%q) = %v, want %v", tc.key, got, tc.want)
			}
		})
	}
}

// TestSummarizeFilingDeterministic ensures the id/date/description picks are
// stable across repeated calls even with multiple candidate keys per field —
// map iteration order in Go is randomized per run, so a bug here would only
// show up intermittently.
func TestSummarizeFilingDeterministic(t *testing.T) {
	t.Parallel()

	rec := map[string]any{
		"filingId":      "F-2",
		"complaint_id":  "F-1",
		"filingDate":    "2026-07-02",
		"receivedDate":  "2026-07-01",
		"description":   "late trade report",
		"confidential":  true,
		"residentState": "NY",
	}

	want := summarizeFiling(rec)
	for i := 0; i < 20; i++ {
		if got := summarizeFiling(rec); got != want {
			t.Fatalf("summarizeFiling not deterministic: run %d got %q, want %q", i, got, want)
		}
	}
}
