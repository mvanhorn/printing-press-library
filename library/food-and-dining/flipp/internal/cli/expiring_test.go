// Copyright 2026 mlabrenz and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

// TestNovelExpiringHelpWires smoke-tests that the expiring command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelExpiringHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"expiring", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expiring --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "expiring"} {
		if !strings.Contains(help, want) {
			t.Fatalf("expiring --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestWithinExpiringWindowExcludesExpiredItems(t *testing.T) {
	now := time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)
	deadline := now.Add(72 * time.Hour)

	if withinExpiringWindow(now.Add(-time.Hour), now, deadline) {
		t.Fatal("already-expired items should not be included")
	}
	if !withinExpiringWindow(now.Add(24*time.Hour), now, deadline) {
		t.Fatal("items expiring inside the window should be included")
	}
	if withinExpiringWindow(now.Add(96*time.Hour), now, deadline) {
		t.Fatal("items beyond the requested window should not be included")
	}
}
