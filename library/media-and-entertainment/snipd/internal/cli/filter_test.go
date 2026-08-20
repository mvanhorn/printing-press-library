// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestNovelFilterHelpWires smoke-tests that the filter command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelFilterHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"filter", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("filter --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "filter"} {
		if !strings.Contains(help, want) {
			t.Fatalf("filter --help missing %q in output:\n%s", want, help)
		}
	}
}

// TestNormalizeDateFlag guards --since/--until: valid dates and RFC3339 timestamps
// are normalized to the YYYY-MM-DD granularity the stored publish_date uses (so a
// boundary-day episode isn't lexically dropped when a timestamp is passed), and a
// typo that would otherwise become a silent SQL predicate is rejected up front.
func TestNormalizeDateFlag(t *testing.T) {
	valid := map[string]string{
		"":                              "",
		"2026-05-01":                    "2026-05-01",
		"2026-05-01T12:30:00Z":          "2026-05-01",
		"2026-05-01T23:59:59.500+02:00": "2026-05-01",
		"2026-05-01T00:00:00":           "2026-05-01",
	}
	for in, want := range valid {
		got, err := normalizeDateFlag("since", in)
		if err != nil {
			t.Errorf("normalizeDateFlag(since, %q) error = %v, want nil", in, err)
		}
		if got != want {
			t.Errorf("normalizeDateFlag(since, %q) = %q, want %q", in, got, want)
		}
	}
	invalid := []string{"zzzz", "2026-13-40", "last week", "05/01/2026"}
	for _, v := range invalid {
		if _, err := normalizeDateFlag("until", v); err == nil {
			t.Errorf("normalizeDateFlag(until, %q) = nil, want an input error", v)
		}
	}
}
