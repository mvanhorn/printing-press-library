package cli

import "testing"

func TestApplySelectMap(t *testing.T) {
	got := applySelect(map[string]any{"a": 1, "b": 2, "c": 3}, "a,c").(map[string]any)
	if len(got) != 2 || got["a"] != 1 || got["c"] != 3 {
		t.Fatalf("bad select: %#v", got)
	}
}

func TestApplySelectSliceOfStruct(t *testing.T) {
	rows := []Attempt{{ID: "a1", ScaledScore: int64(170), TestName: "PT"}}
	got := applySelect(rows, "id,scaled_score").([]any)
	m := got[0].(map[string]any)
	if len(m) != 2 || m["id"] != "a1" || m["scaled_score"] != int64(170) {
		t.Fatalf("bad select: %#v", m)
	}
}
