// Copyright 2026 Omar Shahine and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestNovelExpiringHelpWires smoke-tests that the expiring command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelExpiringHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"expiring", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expiring --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "expiring"} {
		if !strings.Contains(help, want) {
			t.Fatalf("expiring --help missing %q in output:\n%s", want, help)
		}
	}
}
