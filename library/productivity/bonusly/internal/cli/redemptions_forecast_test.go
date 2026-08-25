// Copyright 2026 Allen Lew and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestNovelRedemptionsForecastHelpWires smoke-tests that the redemptions forecast command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelRedemptionsForecastHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"redemptions", "forecast", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("redemptions forecast --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "forecast"} {
		if !strings.Contains(help, want) {
			t.Fatalf("redemptions forecast --help missing %q in output:\n%s", want, help)
		}
	}
}
