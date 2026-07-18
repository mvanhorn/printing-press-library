// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestNovelPlayerHelpWires smoke-tests that the player command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelPlayerHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"player", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("player --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "player"} {
		if !strings.Contains(help, want) {
			t.Fatalf("player --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestNovelPlayerDryRunJoinsMultiwordName(t *testing.T) {
	flags := &rootFlags{dryRun: true}
	cmd := newNovelPlayerCmd(flags)
	cmd.SetArgs([]string{"andreas", "schjelderup"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("player dry-run error = %v", err)
	}
	if got, want := out.String(), "would resolve player \"andreas schjelderup\"\n"; got != want {
		t.Fatalf("player dry-run output = %q, want %q", got, want)
	}
	if got := cmd.Annotations["mcp:read-only"]; got != "true" {
		t.Fatalf("mcp:read-only annotation = %q, want true", got)
	}
}
