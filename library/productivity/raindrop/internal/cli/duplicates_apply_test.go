// Copyright 2026 srijits and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestDuplicateMergePreservesUniqueNotesAndHighlights(t *testing.T) {
	items := []localBookmark{
		{Note: "keeper note", Highlights: []map[string]any{{"_id": "old", "text": "same", "note": "n", "color": "yellow"}}},
		{Note: "duplicate note", Highlights: []map[string]any{{"_id": "other", "text": "same", "note": "n", "color": "yellow"}, {"text": "unique", "color": "blue"}}},
	}
	if got, want := mergedNote(items), "keeper note\n\nduplicate note"; got != want {
		t.Fatalf("mergedNote() = %q, want %q", got, want)
	}
	got := mergedHighlights(items)
	if len(got) != 2 {
		t.Fatalf("mergedHighlights() len = %d, want 2: %#v", len(got), got)
	}
	if _, leaked := got[0]["_id"]; leaked {
		t.Fatalf("merged highlight leaked immutable id: %#v", got[0])
	}
}

func TestHighlightSignaturesFromResponseSupportsItemsEnvelope(t *testing.T) {
	data := json.RawMessage(`{"items":[{"text":"saved","note":"n","color":"green"}]}`)
	got := highlightSignaturesFromResponse(data)
	want := map[string]struct{}{highlightSignature(map[string]any{"text": "saved", "note": "n", "color": "green"}): {}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("highlight signatures = %#v, want %#v", got, want)
	}
}

func TestRaindropAccountResourcesUseGlobalReconciliation(t *testing.T) {
	for _, resource := range []string{"collections", "highlights", "raindrops", "tags"} {
		if got := resourceReconcileMode(resource); got != "global" {
			t.Errorf("resourceReconcileMode(%q) = %q, want global", resource, got)
		}
	}
}
