// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestNovelSignalsHelpWires smoke-tests that the signals command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelSignalsHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"signals", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("signals --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "signals"} {
		if !strings.Contains(help, want) {
			t.Fatalf("signals --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestNormalizeNHTSADateNeverReturnsRawUnsortableValues(t *testing.T) {
	for raw, want := range map[string]string{
		"1/15/2020":            "2020-01-15",
		"2020-01-15":           "2020-01-15",
		"2020-01-15T12:00:00Z": "2020-01-15",
		"not-a-date":           "",
	} {
		if got := normalizeNHTSADate(raw, "01/02/2006"); got != want {
			t.Fatalf("normalizeNHTSADate(%q) = %q, want %q", raw, got, want)
		}
	}
}
