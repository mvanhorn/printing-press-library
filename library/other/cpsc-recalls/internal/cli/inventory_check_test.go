// Copyright 2026 Avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestNovelInventoryCheckHelpWires smoke-tests that the inventory-check command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelInventoryCheckHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"inventory-check", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("inventory-check --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "inventory-check"} {
		if !strings.Contains(help, want) {
			t.Fatalf("inventory-check --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestSortInventoryCandidatesPrefersExactThenScore(t *testing.T) {
	candidates := []map[string]any{
		{"recall_id": "1", "exact_match": false, "token_overlap": 0.9},
		{"recall_id": "2", "exact_match": true, "token_overlap": 0.1},
		{"recall_id": "3", "exact_match": false, "token_overlap": 0.95},
	}
	sortInventoryCandidates(candidates)
	if candidates[0]["recall_id"] != "2" || candidates[1]["recall_id"] != "3" {
		t.Fatalf("candidates=%v", candidates)
	}
}
