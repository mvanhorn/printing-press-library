// Copyright 2026 Ade Amos and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestNovelBlueprintApplyHelpWires smoke-tests that the blueprint apply command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelBlueprintApplyHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"blueprint", "apply", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("blueprint apply --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "apply"} {
		if !strings.Contains(help, want) {
			t.Fatalf("blueprint apply --help missing %q in output:\n%s", want, help)
		}
	}
}
