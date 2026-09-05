// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

// PATCH(amend-2026-06-17): regression coverage for the designs records-path
// unwrap. Hand-authored patch test, not generator output.

package cli

import (
	"encoding/json"
	"testing"
)

// designsTestEnvelope is the shape the sync loop sees for both design resources
// once the client has stripped the outer results wrapper: two sibling object
// arrays (designs, folders) plus a hasMore flag. The generic extractor bails on
// this because extractSingleObjectArraySibling refuses to guess between two
// arrays, which is why a deterministic records-path is required.
const designsTestEnvelope = `{"designs":[{"id":57786554,"positive_prompt":"a majestic still life"},{"id":57786549,"positive_prompt":"a vintage frame"},{"id":57786521,"positive_prompt":"autumn books"}],"folders":[{"id":1,"name":"Inbox"}],"hasMore":false}`

// TestExtractItemsByRecordsPath_Designs guards the amend-2026-06-17 sync fix:
// before the records-path, extractPageItems returned 0 items for the two-array
// envelope, the loop fell back to the single-object path, and UpsertDesigns
// failed "missing id for designs" — leaving the cache empty and offline search
// dead.
func TestExtractItemsByRecordsPath_Designs(t *testing.T) {
	for _, resource := range []string{"designs", "designs-fetch-personal-designs"} {
		path, ok := resourceRecordsPath(resource)
		if !ok {
			t.Fatalf("resourceRecordsPath(%q) returned ok=false; designs resources must declare a records-path", resource)
		}
		items, ok := extractItemsByRecordsPath(json.RawMessage(designsTestEnvelope), path)
		if !ok {
			t.Fatalf("%s: extractItemsByRecordsPath returned ok=false; designs[] not unwrapped", resource)
		}
		if len(items) != 3 {
			t.Fatalf("%s: got %d items, want 3 (designs[] array)", resource, len(items))
		}
		var first map[string]json.RawMessage
		if err := json.Unmarshal(items[0], &first); err != nil {
			t.Fatalf("%s: first item not a JSON object: %v", resource, err)
		}
		if _, ok := first["id"]; !ok {
			t.Fatalf("%s: first item missing top-level id; unwrap pulled the wrong level: %s", resource, items[0])
		}
	}
}

// TestExtractItemsByRecordsPath_MissingKey ensures the records-path unwrap fails
// closed (ok=false, so the caller keeps the generic-extractor result) when the
// terminal key is absent rather than panicking or returning a bogus array.
func TestExtractItemsByRecordsPath_MissingKey(t *testing.T) {
	if _, ok := extractItemsByRecordsPath(json.RawMessage(`{"folders":[{"id":1}]}`), []string{"designs"}); ok {
		t.Fatal("expected ok=false when the records-path key is absent")
	}
}

// TestExtractDesignsPagination reads the top-level hasMore flag that drives the
// page-int paginator fallback for the cursorless designs endpoints.
func TestExtractDesignsPagination(t *testing.T) {
	if _, more := extractDesignsPagination(json.RawMessage(designsTestEnvelope)); more {
		t.Fatal("hasMore=false envelope should report more=false")
	}
	if _, more := extractDesignsPagination(json.RawMessage(`{"designs":[],"hasMore":true}`)); !more {
		t.Fatal("hasMore=true envelope should report more=true")
	}
}
