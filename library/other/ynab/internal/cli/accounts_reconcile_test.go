// Copyright 2026 mikesnowbie and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestNovelAccountsReconcileHelpWires smoke-tests that the accounts reconcile command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelAccountsReconcileHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"accounts", "reconcile", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("accounts reconcile --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "reconcile"} {
		if !strings.Contains(help, want) {
			t.Fatalf("accounts reconcile --help missing %q in output:\n%s", want, help)
		}
	}
}
