// Copyright 2026 Kerry Morrison and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestNovelItemsBulkSetHelpWires smoke-tests that the items bulk-set command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelItemsBulkSetHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"items", "bulk-set", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("items bulk-set --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "bulk-set"} {
		if !strings.Contains(help, want) {
			t.Fatalf("items bulk-set --help missing %q in output:\n%s", want, help)
		}
	}
}
