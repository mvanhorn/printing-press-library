// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

// TestNovelNewFilingsHelpWires smoke-tests that the new-filings command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelNewFilingsHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"new-filings", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("new-filings --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "new-filings"} {
		if !strings.Contains(help, want) {
			t.Fatalf("new-filings --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestMergeSeenFilingsRetainsHistoryBeyondCurrentWindow(t *testing.T) {
	previous := map[string]string{"old": "2026-01-01T00:00:00Z"}
	added, next := mergeSeenFilings(previous, map[string]bool{"new": true}, time.Date(2026, 7, 18, 0, 0, 0, 0, time.UTC), 5000)
	if len(added) != 1 || added[0] != "new" || next["old"] == "" {
		t.Fatalf("added=%v next=%v", added, next)
	}
	added, _ = mergeSeenFilings(next, map[string]bool{"old": true}, time.Date(2026, 7, 19, 0, 0, 0, 0, time.UTC), 5000)
	if len(added) != 0 {
		t.Fatalf("previously seen filing was reported again: %v", added)
	}
}
