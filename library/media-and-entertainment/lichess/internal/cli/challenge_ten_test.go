// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestNovelChallengeTenHelpWires smoke-tests that the challenge ten command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelChallengeTenHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"challenge", "ten", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("challenge ten --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "ten"} {
		if !strings.Contains(help, want) {
			t.Fatalf("challenge ten --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestNovelChallengeTenDryRunDoesNotNeedCredentials(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"challenge", "ten", "alice", "--dry-run", "--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("challenge ten --dry-run error = %v", err)
	}
	for _, want := range []string{`"time_control": "10+0"`, `"dry_run": true`} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("dry-run output missing %q: %s", want, out.String())
		}
	}
}

func TestNovelChallengeTenRejectsExtraArguments(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"challenge", "ten", "alice", "typo", "--dry-run"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("challenge ten accepted an extra positional argument")
	}
}
