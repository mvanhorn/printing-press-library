// Copyright 2026 Todd Dailey and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Bambu ETA command wiring tests.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestNovelJobEtaHelpWires smoke-tests that the job eta command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelJobEtaHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"job", "eta", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("job eta --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "eta"} {
		if !strings.Contains(help, want) {
			t.Fatalf("job eta --help missing %q in output:\n%s", want, help)
		}
	}
}
