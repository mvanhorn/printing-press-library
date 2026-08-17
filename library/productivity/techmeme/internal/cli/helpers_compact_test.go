// Copyright 2026 Dave Morin and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"testing"
)

// decodeList unmarshals a compactFields result back into a slice of maps so
// tests can assert on the surviving fields.
func decodeList(t *testing.T, raw json.RawMessage) []map[string]any {
	t.Helper()
	var items []map[string]any
	if err := json.Unmarshal(raw, &items); err != nil {
		t.Fatalf("result is not a JSON array: %v (raw=%s)", err, string(raw))
	}
	return items
}

// A Techmeme headline record keys its content off headline/link/source/num,
// none of which were in the legacy allow-list. Compaction must keep those
// fields rather than emitting an empty object (the --agent regression).
func TestCompactListFields_KeepsTechmemeHeadlineFields(t *testing.T) {
	raw := json.RawMessage(`[{"num":1,"source":"techcrunch.com","headline":"Sakana AI ships Fugu","link":"https://www.techmeme.com/x"}]`)
	got := decodeList(t, compactFields(raw))
	if len(got) != 1 {
		t.Fatalf("want 1 item, got %d", len(got))
	}
	item := got[0]
	if len(item) == 0 {
		t.Fatalf("compaction blanked the record: %v", item)
	}
	for _, k := range []string{"headline", "link", "source", "num"} {
		if _, ok := item[k]; !ok {
			t.Errorf("expected field %q to survive compaction, item=%v", k, item)
		}
	}
}

// Defense in depth: a record whose fields are all outside the allow-list must
// pass through intact rather than collapsing to {}.
func TestCompactListFields_NeverBlanksARecord(t *testing.T) {
	raw := json.RawMessage(`[{"some_unknown_field":"value","another":42}]`)
	got := decodeList(t, compactFields(raw))
	if len(got) != 1 {
		t.Fatalf("want 1 item, got %d", len(got))
	}
	if len(got[0]) == 0 {
		t.Fatalf("record with no allow-listed fields was blanked: %v", got[0])
	}
	if got[0]["some_unknown_field"] != "value" {
		t.Errorf("original field not preserved, item=%v", got[0])
	}
}

// Compaction still drops verbose fields when a high-gravity field is present,
// so token savings are preserved for records that do match the allow-list.
func TestCompactListFields_StripsVerboseWhenIdentifierPresent(t *testing.T) {
	raw := json.RawMessage(`[{"title":"A","description":"long verbose body that should be stripped"}]`)
	got := decodeList(t, compactFields(raw))
	if _, ok := got[0]["title"]; !ok {
		t.Errorf("expected title to survive, item=%v", got[0])
	}
	if _, ok := got[0]["description"]; ok {
		t.Errorf("expected description to be stripped, item=%v", got[0])
	}
}

// When a record carries a verbose field but no allow-listed field at all, the
// no-blank guard fires and the whole record is preserved intact -- including
// the verbose field. Documents that --compact only strips verbose fields when
// it has an allow-listed field to fall back on.
func TestCompactListFields_GuardPreservesVerboseWhenNoIdentifier(t *testing.T) {
	raw := json.RawMessage(`[{"description":"long text","some_novel_key":"x"}]`)
	got := decodeList(t, compactFields(raw))
	if len(got[0]) == 0 {
		t.Fatalf("record was blanked: %v", got[0])
	}
	for _, k := range []string{"description", "some_novel_key"} {
		if _, ok := got[0][k]; !ok {
			t.Errorf("expected field %q to be preserved by the no-blank guard, item=%v", k, got[0])
		}
	}
}
