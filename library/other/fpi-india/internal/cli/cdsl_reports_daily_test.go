// Copyright 2026 Mayank Lavania and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestCdslReportsDailyHelpWires and TestCdslReportsMonthlyHelpWires
// smoke-test that the daily/monthly commands resolve at runtime and
// render useful --help output.
func TestCdslReportsDailyHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"cdsl-reports", "daily", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("cdsl-reports daily --help error = %v (command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "daily"} {
		if !strings.Contains(help, want) {
			t.Fatalf("cdsl-reports daily --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestCdslReportsMonthlyHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"cdsl-reports", "monthly", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("cdsl-reports monthly --help error = %v (command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "monthly"} {
		if !strings.Contains(help, want) {
			t.Fatalf("cdsl-reports monthly --help missing %q in output:\n%s", want, help)
		}
	}
}
