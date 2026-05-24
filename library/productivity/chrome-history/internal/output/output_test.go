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

func TestApplyCompactRows(t *testing.T) {
	rows := []map[string]any{{
		"url": "u", "title": "t", "last_visit_time": "x", "rank": 1,
	}}
	got := applyCompact("search", rows).([]map[string]any)
	if len(got) != 1 {
		t.Fatalf("unexpected row count: %d", len(got))
	}
	if len(got[0]) != 3 {
		t.Fatalf("expected 3 keys, got %d (%#v)", len(got[0]), got[0])
	}
	if _, ok := got[0]["rank"]; ok {
		t.Fatalf("rank should be dropped in compact mode")
	}
}

func TestApplyCompactReport(t *testing.T) {
	in := map[string]any{
		"productivity_split": map[string]any{"productive": 1},
		"peak_hours":         []int{1, 2},                  // profile-only
		"busiest_weekday":    []int{3},                     // profile-only
		"totals":             map[string]any{"visits": 10}, // profile-only
		"top_domains":        []map[string]any{{"domain": "x"}},
		"per_day":            []int{1, 2, 3},
		"hour_of_day":        []int{0, 1},
	}
	got := applyCompact("report", in).(map[string]any)
	// report's primary breakdowns must survive compact mode.
	for _, k := range []string{"per_day", "hour_of_day", "productivity_split", "top_domains"} {
		if _, ok := got[k]; !ok {
			t.Fatalf("report compact dropped %q", k)
		}
	}
	// profile-only keys must not leak into report's compact output.
	for _, k := range []string{"peak_hours", "busiest_weekday", "totals"} {
		if _, ok := got[k]; ok {
			t.Fatalf("report compact should not include profile-only key %q", k)
		}
	}
}
