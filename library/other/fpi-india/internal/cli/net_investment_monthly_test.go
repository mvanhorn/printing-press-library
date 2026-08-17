// Copyright 2026 Mayank Lavania and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestNetInvestmentMonthlyHelpWires smoke-tests that the monthly command
// resolves at runtime and renders useful --help output.
func TestNetInvestmentMonthlyHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"net-investment", "monthly", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("net-investment monthly --help error = %v (command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "monthly"} {
		if !strings.Contains(help, want) {
			t.Fatalf("net-investment monthly --help missing %q in output:\n%s", want, help)
		}
	}
}
