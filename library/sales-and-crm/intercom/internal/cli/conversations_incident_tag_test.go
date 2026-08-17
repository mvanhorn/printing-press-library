// Copyright 2026 Rob Zehner and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestNovelConversationsIncidentTagHelpWires smoke-tests that the conversations incident-tag command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelConversationsIncidentTagHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"conversations", "incident-tag", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("conversations incident-tag --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "incident-tag"} {
		if !strings.Contains(help, want) {
			t.Fatalf("conversations incident-tag --help missing %q in output:\n%s", want, help)
		}
	}
}
