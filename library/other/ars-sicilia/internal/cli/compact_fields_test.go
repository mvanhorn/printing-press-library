package cli

import (
	"encoding/json"
	"testing"
)

// TestCompactListFields_KeepsFirmatari covers the --con-firmatari + --compact
// bug: the enriched "firmatari" array (added to rows only when the caller
// explicitly asked for it via --con-firmatari) used to be silently dropped by
// --compact/--agent, because compactListFields' static allow-list only ever
// kept scalar-shaped fields — an explicit opt-in should survive the
// allow-list regardless of shape. An unrelated array field not on the
// allow-list must still be dropped, so the fix isn't "keep every array".
func TestCompactListFields_KeepsFirmatari(t *testing.T) {
	in := `[
		{"doc_id": 1, "title": "DDL Osservatorio IA", "firmatari": [{"nome": "Scuvera Salvatore", "gruppo": "Fratelli d'Italia"}], "novel_array": ["x", "y"]},
		{"doc_id": 2, "title": "DDL Dirigenza", "firmatari": [{"nome": "Cracolici Antonino"}], "novel_array": ["z"]}
	]`
	out := compactFields(json.RawMessage(in))

	var items []map[string]any
	if err := json.Unmarshal(out, &items); err != nil {
		t.Fatalf("unmarshal compact output: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("want 2 items, got %d", len(items))
	}
	for _, item := range items {
		if _, ok := item["firmatari"]; !ok {
			t.Errorf("firmatari stripped by --compact despite explicit --con-firmatari request: %+v", item)
		}
		if _, ok := item["novel_array"]; ok {
			t.Errorf("unrelated array field should still be stripped by --compact: %+v", item)
		}
	}
}
