package cli

import (
	"encoding/json"
	"testing"
)

// idOf decodes a write-through row and returns its top-level "id" as resolved by
// the same generic extractor UpsertBatch uses. A row that fails to yield an id
// would be skipped by the cache ("not cached locally; no extractable ID field").
func idOf(t *testing.T, row json.RawMessage) string {
	t.Helper()
	var obj map[string]any
	if err := json.Unmarshal(row, &obj); err != nil {
		t.Fatalf("row is not a JSON object: %v\nrow=%s", err, string(row))
	}
	if v, ok := obj["id"]; ok {
		switch n := v.(type) {
		case float64:
			// JSON numbers decode to float64; designs ids are integers.
			return jsonNumberString(n)
		case string:
			return n
		}
	}
	return ""
}

func jsonNumberString(f float64) string {
	b, _ := json.Marshal(f)
	return string(b)
}

// Artistly's design list endpoints return a two-sibling-array envelope
// ({"designs":[...],"folders":[...],"hasMore":bool}) after the client strips the
// outer results wrapper. The generic single-array-sibling heuristic can't
// disambiguate designs[] from folders[], so before the records-path fix the
// write-through cache fell through to the single-object branch and tried to
// cache the whole wrapper — which has no extractable id. This test pins the fix.
func TestWriteThroughCacheRows_DesignsEnvelope(t *testing.T) {
	envelope := json.RawMessage(`{
		"designs": [
			{"id": 58223649, "uuid": "019eefc8", "positive_prompt": "a fox"},
			{"id": 58223650, "uuid": "019eefc9", "positive_prompt": "a hound"}
		],
		"folders": [{"id": 7, "name": "Holiday"}],
		"hasMore": false
	}`)

	rows := writeThroughCacheRows("designs", envelope)

	if len(rows) != 2 {
		t.Fatalf("expected 2 design rows unwrapped, got %d: %s", len(rows), rows)
	}
	for i, row := range rows {
		if id := idOf(t, row); id == "" {
			t.Errorf("row %d has no extractable id (would not be cached): %s", i, string(row))
		}
	}
	// Guard against caching the wrapper: no row should carry sibling keys.
	for i, row := range rows {
		var obj map[string]json.RawMessage
		_ = json.Unmarshal(row, &obj)
		if _, ok := obj["designs"]; ok {
			t.Errorf("row %d is the wrapper, not a design record: %s", i, string(row))
		}
		if _, ok := obj["folders"]; ok {
			t.Errorf("row %d carries the sibling folders array: %s", i, string(row))
		}
	}
}

// A single design detail object (no list envelope) must still cache as one row.
func TestWriteThroughCacheRows_SingleDetail(t *testing.T) {
	detail := json.RawMessage(`{"id": 58223649, "uuid": "019eefc8", "positive_prompt": "a fox"}`)
	rows := writeThroughCacheRows("designs", detail)
	if len(rows) != 1 {
		t.Fatalf("expected 1 row for a single detail object, got %d: %s", len(rows), rows)
	}
	if id := idOf(t, rows[0]); id == "" {
		t.Errorf("single detail row has no extractable id: %s", string(rows[0]))
	}
}

// A conventional single-array list envelope (results[]) still unwraps for any
// resource, including those with a records-path declared.
func TestWriteThroughCacheRows_GenericResultsEnvelope(t *testing.T) {
	envelope := json.RawMessage(`{"results": [{"id": 1}, {"id": 2}], "hasMore": false}`)
	rows := writeThroughCacheRows("styles", envelope)
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows from results[] envelope, got %d: %s", len(rows), rows)
	}
}

// An empty list envelope caches nothing (no spurious wrapper row).
func TestWriteThroughCacheRows_EmptyListEnvelope(t *testing.T) {
	envelope := json.RawMessage(`{"designs": [], "folders": [], "hasMore": false}`)
	rows := writeThroughCacheRows("designs", envelope)
	if len(rows) != 0 {
		t.Fatalf("expected no rows for an empty designs envelope, got %d: %s", len(rows), rows)
	}
}
