// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestNovelWatchChangesHelpWires smoke-tests that the watch changes command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelWatchChangesHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"watch", "changes", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("watch changes --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "changes"} {
		if !strings.Contains(help, want) {
			t.Fatalf("watch changes --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestWatchObservationKeyIncludesLimit(t *testing.T) {
	left := watchObservationKey("Company", "Product", "NY", "30d", 20)
	right := watchObservationKey("Company", "Product", "NY", "30d", 100)
	if left == right {
		t.Fatal("different bounded samples shared one snapshot key")
	}
}
