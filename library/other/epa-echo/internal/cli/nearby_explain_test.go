// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestNovelNearbyExplainHelpWires smoke-tests that the nearby explain command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelNearbyExplainHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"nearby", "explain", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("nearby explain --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "explain"} {
		if !strings.Contains(help, want) {
			t.Fatalf("nearby explain --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestNearbyExplainRequiresBothCoordinates(t *testing.T) {
	cmd := newNovelNearbyExplainCmd(&rootFlags{})
	cmd.SetArgs([]string{"--latitude", "41.83"})
	cmd.SilenceUsage = true
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "both --latitude and --longitude are required") {
		t.Fatalf("Execute() error = %v, want missing-coordinate error", err)
	}
}
