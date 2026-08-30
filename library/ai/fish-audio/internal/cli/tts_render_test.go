// Copyright 2026 Jon Gouveia and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestNovelTtsRenderHelpWires smoke-tests that the tts render command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelTtsRenderHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"tts", "render", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("tts render --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "render"} {
		if !strings.Contains(help, want) {
			t.Fatalf("tts render --help missing %q in output:\n%s", want, help)
		}
	}
}
