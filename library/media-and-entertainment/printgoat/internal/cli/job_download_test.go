// Copyright 2026 Nate Olson and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestNovelJobDownloadHelpWires smoke-tests that the job download command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelJobDownloadHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"job", "download", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("job download --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "download"} {
		if !strings.Contains(help, want) {
			t.Fatalf("job download --help missing %q in output:\n%s", want, help)
		}
	}
}
