// Copyright 2026 Kieran Maynard and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestNovelEcosystemGameHelpWires smoke-tests that the ecosystem game command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelEcosystemGameHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"ecosystem", "game", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("ecosystem game --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "game"} {
		if !strings.Contains(help, want) {
			t.Fatalf("ecosystem game --help missing %q in output:\n%s", want, help)
		}
	}
}
