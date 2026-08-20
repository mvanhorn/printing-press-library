// Copyright 2026 Todd Dailey and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Bambu repeat-job command wiring tests.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestNovelJobRepeatsHelpWires smoke-tests that the job repeats command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelJobRepeatsHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"job", "repeats", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("job repeats --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "repeats"} {
		if !strings.Contains(help, want) {
			t.Fatalf("job repeats --help missing %q in output:\n%s", want, help)
		}
	}
}
