// Copyright 2026 Mayank Lavania and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestNovelNetInvestmentYoyHelpWires smoke-tests that the net-investment yoy command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelNetInvestmentYoyHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"net-investment", "yoy", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("net-investment yoy --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "yoy"} {
		if !strings.Contains(help, want) {
			t.Fatalf("net-investment yoy --help missing %q in output:\n%s", want, help)
		}
	}
}
