// Copyright 2026 Todd Dailey and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Bambu AMS runway command wiring tests.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestNovelAmsRunwayHelpWires smoke-tests that the ams runway command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelAmsRunwayHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"ams", "runway", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("ams runway --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "runway"} {
		if !strings.Contains(help, want) {
			t.Fatalf("ams runway --help missing %q in output:\n%s", want, help)
		}
	}
}
