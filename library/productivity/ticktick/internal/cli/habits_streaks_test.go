// Copyright 2026 Harvey The AI Guy and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestNovelHabitsStreaksHelpWires smoke-tests that the habits streaks command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelHabitsStreaksHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"habits", "streaks", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("habits streaks --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "streaks"} {
		if !strings.Contains(help, want) {
			t.Fatalf("habits streaks --help missing %q in output:\n%s", want, help)
		}
	}
}
