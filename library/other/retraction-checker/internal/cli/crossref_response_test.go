// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored regression tests for Crossref response envelopes.

package cli

import (
	"encoding/json"
	"testing"
)

func TestExtractSearchResultsCrossrefMessageItems(t *testing.T) {
	got := extractSearchResults(json.RawMessage(`{"message":{"items":[{"DOI":"10.1000/example","title":["Example"]}]}}`))
	if len(got) != 1 {
		t.Fatalf("results = %d, want 1", len(got))
	}
	var work map[string]json.RawMessage
	if err := json.Unmarshal(got[0], &work); err != nil {
		t.Fatal(err)
	}
	if string(work["DOI"]) != `"10.1000/example"` {
		t.Fatalf("DOI = %s, want %q", work["DOI"], "10.1000/example")
	}
}

func TestExtractPageItemsCrossrefMessageItems(t *testing.T) {
	items, _, _ := extractPageItems(json.RawMessage(`{"message":{"items":[{"DOI":"10.1000/example","title":["Example"]}]}}`), "cursor")
	if len(items) != 1 {
		t.Fatalf("items = %d, want 1", len(items))
	}
	var work map[string]any
	if err := json.Unmarshal(items[0], &work); err != nil {
		t.Fatal(err)
	}
	if got := extractID("works", work); got != "10.1000/example" {
		t.Fatalf("work ID = %q, want %q", got, "10.1000/example")
	}
}
