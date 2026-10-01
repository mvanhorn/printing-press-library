// Copyright 2026 Ricardo Cabral and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestNovelDriftHelpWires smoke-tests that the drift command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelDriftHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"drift", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("drift --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "drift"} {
		if !strings.Contains(help, want) {
			t.Fatalf("drift --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestDriftRejectsLiveDataSource(t *testing.T) {
	cmd := newNovelDriftCmd(&rootFlags{dataSource: "live"})
	cmd.SetArgs([]string{"--site", "default"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "no live equivalent") {
		t.Fatalf("drift with live source must fail before reading or advancing snapshots, got %v", err)
	}
}
