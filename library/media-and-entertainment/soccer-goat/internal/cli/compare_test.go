// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestNovelCompareHelpWires smoke-tests that the compare command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelCompareHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"compare", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("compare --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "compare"} {
		if !strings.Contains(help, want) {
			t.Fatalf("compare --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestNovelCompareRequiresExactlyTwoPlayers(t *testing.T) {
	cmd := newNovelCompareCmd(&rootFlags{})
	cmd.SetArgs([]string{"mbappe"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	err := cmd.Execute()
	if err == nil || ExitCode(err) != 2 {
		t.Fatalf("compare one-player error = %v, exit code = %d; want usage error", err, ExitCode(err))
	}
	if !strings.Contains(err.Error(), "exactly two") {
		t.Fatalf("compare error = %q, want exact-count guidance", err)
	}
}
