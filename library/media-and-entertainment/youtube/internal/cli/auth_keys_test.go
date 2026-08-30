// Copyright 2026 Justin and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestNovelAuthKeysHelpWires smoke-tests that the auth keys command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelAuthKeysHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"auth", "keys", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("auth keys --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "keys"} {
		if !strings.Contains(help, want) {
			t.Fatalf("auth keys --help missing %q in output:\n%s", want, help)
		}
	}
}
