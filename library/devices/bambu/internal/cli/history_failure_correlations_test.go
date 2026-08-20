// Copyright 2026 Todd Dailey and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Bambu failure-correlation command wiring tests.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestNovelHistoryFailureCorrelationsHelpWires smoke-tests that the history failure-correlations command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelHistoryFailureCorrelationsHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"history", "failure-correlations", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("history failure-correlations --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "failure-correlations"} {
		if !strings.Contains(help, want) {
			t.Fatalf("history failure-correlations --help missing %q in output:\n%s", want, help)
		}
	}
}
