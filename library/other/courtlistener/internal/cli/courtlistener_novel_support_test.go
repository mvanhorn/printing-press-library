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

func TestDocketTimelineUsesNumericEntryAndDocumentTieBreaks(t *testing.T) {
	entries := []map[string]any{
		{"id": "100", "date_filed": "2026-07-01"},
		{"id": "90", "date_filed": "2026-07-01"},
	}
	timeline := clDocketTimeline(entries, nil)
	if got := clReferenceID(timeline[0]["entry_id"]); got != "90" {
		t.Fatalf("first entry = %q, want numeric ID 90 before 100", got)
	}

	documents := []map[string]any{
		{"id": "100", "date_filed": "2026-07-01"},
		{"id": "90", "date_filed": "2026-07-01"},
	}
	timeline = clDocketTimeline(nil, documents)
	if got := clReferenceID(timeline[0]["document_id"]); got != "90" {
		t.Fatalf("first document = %q, want numeric ID 90 before 100", got)
	}
}
