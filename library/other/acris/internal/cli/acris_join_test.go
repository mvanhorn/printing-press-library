// Copyright 2026 not0xjarvis and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestSoqlQuote(t *testing.T) {
	cases := []struct{ in, want string }{
		{"MADISON", "MADISON"},
		{"O'BRIEN", "O''BRIEN"},
		{"A'B'C", "A''B''C"},
		{"", ""},
	}
	for _, c := range cases {
		if got := soqlQuote(c.in); got != c.want {
			t.Errorf("soqlQuote(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSoqlInClause(t *testing.T) {
	got := soqlInClause("document_id", []string{"A1", "B2"})
	want := "document_id in('A1','B2')"
	if got != want {
		t.Errorf("soqlInClause = %q, want %q", got, want)
	}
	// Single quotes in a value must be doubled so the clause stays valid SoQL.
	got = soqlInClause("name", []string{"O'BRIEN"})
	want = "name in('O''BRIEN')"
	if got != want {
		t.Errorf("soqlInClause with quote = %q, want %q", got, want)
	}
}

func TestStrField(t *testing.T) {
	row := map[string]any{
		"document_id": "2023012300123001",
		"amount":      float64(1500000),
		"flag":        true,
		"missing":     nil,
	}
	cases := []struct {
		key, want string
	}{
		{"document_id", "2023012300123001"},
		{"amount", "1500000"},
		{"flag", "true"},
		{"missing", ""},
		{"absent", ""},
	}
	for _, c := range cases {
		if got := strField(row, c.key); got != c.want {
			t.Errorf("strField(%q) = %q, want %q", c.key, got, c.want)
		}
	}
}

func TestDistinctDocumentIDs(t *testing.T) {
	rows := []map[string]any{
		{"document_id": "A"},
		{"document_id": "B"},
		{"document_id": "A"}, // duplicate
		{"document_id": ""},  // empty, skipped
		{"other": "x"},       // no document_id, skipped
		{"document_id": "C"},
	}
	got := distinctDocumentIDs(rows, 0)
	want := []string{"A", "B", "C"}
	if len(got) != len(want) {
		t.Fatalf("distinctDocumentIDs len = %d (%v), want %d", len(got), got, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("distinctDocumentIDs[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	// Cap applies and preserves first-seen order.
	if capped := distinctDocumentIDs(rows, 2); len(capped) != 2 || capped[0] != "A" || capped[1] != "B" {
		t.Errorf("distinctDocumentIDs cap=2 = %v, want [A B]", capped)
	}
}

func TestMortgageClassCodes(t *testing.T) {
	// Originating mortgage codes must be a subset of the mortgage-class codes,
	// otherwise the debt total would sum a doc type the history never lists.
	for code := range originatingMortgageCodes {
		if _, ok := mortgageClassCodes[code]; !ok {
			t.Errorf("originating code %q is not in mortgageClassCodes", code)
		}
	}
	if _, ok := mortgageClassCodes["MTGE"]; !ok {
		t.Error("mortgageClassCodes missing MTGE")
	}
	if originatingMortgageCodes["SAT"] {
		t.Error("SAT (satisfaction) must not count as originating mortgage principal")
	}
}
