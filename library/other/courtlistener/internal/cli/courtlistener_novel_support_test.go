// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import "testing"

func TestCLResultsUsesOnlyObjectRows(t *testing.T) {
	got := clResults(map[string]any{"results": []any{map[string]any{"id": 1}, "bad"}})
	if len(got) != 1 {
		t.Fatalf("got %d rows", len(got))
	}
}

func TestCLReferenceIDLessSortsNumericIDsNumerically(t *testing.T) {
	if !clReferenceIDLess("90", "100") || clReferenceIDLess("100", "90") {
		t.Fatal("numeric document IDs were compared lexicographically")
	}
}
