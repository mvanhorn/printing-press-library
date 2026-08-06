// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestNovelDunningHelpWires smoke-tests that the dunning command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelDunningHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"dunning", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("dunning --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "dunning"} {
		if !strings.Contains(help, want) {
			t.Fatalf("dunning --help missing %q in output:\n%s", want, help)
		}
	}
}
