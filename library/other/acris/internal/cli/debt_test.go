// Copyright 2026 not0xjarvis and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestNovelDebtHelpWires smoke-tests that the debt command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelDebtHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"debt", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("debt --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "debt"} {
		if !strings.Contains(help, want) {
			t.Fatalf("debt --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestCollectDebtDocumentsTotalsBeyondDisplayCap(t *testing.T) {
	ids := []string{"mortgage-a", "satisfaction", "mortgage-b"}
	masters := map[string]map[string]any{
		"mortgage-a": {
			"doc_type":      "MTGE",
			"document_amt":  "100000",
			"document_date": "2020-01-01",
		},
		"satisfaction": {
			"doc_type":     "SAT",
			"document_amt": "100000",
		},
		"mortgage-b": {
			"doc_type":      "M&CON",
			"document_amt":  "250000",
			"document_date": "2024-01-01",
		},
	}

	documents, total, capped := collectDebtDocuments(ids, masters, 1)
	if len(documents) != 1 {
		t.Fatalf("document count = %d, want 1", len(documents))
	}
	if total != 350000 {
		t.Fatalf("total = %d, want 350000", total)
	}
	if !capped {
		t.Fatal("capped = false, want true")
	}
}
