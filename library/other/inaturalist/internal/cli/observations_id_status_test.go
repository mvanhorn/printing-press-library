// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestNovelObservationsIdStatusHelpWires smoke-tests that the observations id-status command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelObservationsIdStatusHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"observations", "id-status", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("observations id-status --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "id-status"} {
		if !strings.Contains(help, want) {
			t.Fatalf("observations id-status --help missing %q in output:\n%s", want, help)
		}
	}
}
