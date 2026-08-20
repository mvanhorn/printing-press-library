package output

import (
	"encoding/json"
	"testing"
)

func TestApplySelectArrayAndMap(t *testing.T) {
	arr := []map[string]any{{"a": map[string]any{"b": 1}, "x": 2}}
	got, err := applySelect("a.b", arr)
	if err != nil {
		t.Fatal(err)
	}
	rows := got.([]map[string]any)
	if rows[0]["a.b"].(int) != 1 {
		t.Fatalf("unexpected select array value: %#v", rows[0])
	}

	obj := map[string]any{"a": map[string]any{"b": "ok"}, "z": 3}
	got2, err := applySelect("a.b", obj)
	if err != nil {
		t.Fatal(err)
	}
	m := got2.(map[string]any)
	if m["a.b"].(string) != "ok" {
		t.Fatalf("unexpected select map value: %#v", m)
	}
}

func TestJSONValidity(t *testing.T) {
	v := []map[string]any{{"a": 1, "b": "x"}}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
}

func TestApplyCompactSearchWithoutSelect(t *testing.T) {
	flags := Flags{Compact: true, Command: "search"}
	in := []map[string]any{{
		"url":             "https://example.com",
		"title":           "Example",
		"last_visit_time": "2026-01-01T00:00:00Z",
		"rank":            42.0,
	}}
	got := applyCompact(flags, in).([]map[string]any)
	if len(got) != 1 {
		t.Fatalf("unexpected rows: %d", len(got))
	}
	if _, ok := got[0]["rank"]; ok {
		t.Fatalf("unexpected rank key in compact output: %#v", got[0])
	}
	if _, ok := got[0]["url"]; !ok {
		t.Fatalf("missing url key in compact output: %#v", got[0])
	}
}
