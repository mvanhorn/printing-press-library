// Copyright 2026 Cole Grolmus and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestNovelDailyReportHelpWires smoke-tests that the daily-report command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelDailyReportHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"daily-report", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("daily-report --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "daily-report"} {
		if !strings.Contains(help, want) {
			t.Fatalf("daily-report --help missing %q in output:\n%s", want, help)
		}
	}
}
