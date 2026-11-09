package cliutil

import "testing"

func TestCheckPageSizeBounds(t *testing.T) {
	for _, v := range []int{1, 50, 100} {
		if err := CheckPageSize("limit", v, 100); err != nil {
			t.Fatalf("%d refused: %v", v, err)
		}
	}
	for _, v := range []int{0, -1, 101} {
		if err := CheckPageSize("limit", v, 100); err == nil {
			t.Fatalf("%d accepted", v)
		}
	}
}

func TestCheckCursorRefusesOnlyAGivenBlank(t *testing.T) {
	if err := CheckCursor("cursor", false, ""); err != nil {
		t.Fatalf("omitted cursor refused: %v", err)
	}
	if err := CheckCursor("cursor", true, "abc"); err != nil {
		t.Fatalf("real cursor refused: %v", err)
	}
	for _, v := range []string{"", "  "} {
		if err := CheckCursor("cursor", true, v); err == nil {
			t.Fatalf("blank %q accepted", v)
		}
	}
}

func TestCheckPagingParams(t *testing.T) {
	sizes := map[string]int{"limit": 100, "page": 100000}
	cursors := []string{"cursor"}
	ok := []map[string]any{{}, {"limit": float64(1)}, {"limit": float64(100), "page": float64(100000), "cursor": "c"}}
	for _, p := range ok {
		if err := CheckPagingParams(p, sizes, cursors); err != nil {
			t.Fatalf("%v refused: %v", p, err)
		}
	}
	bad := []map[string]any{{"limit": float64(0)}, {"limit": float64(101)}, {"limit": 2.5}, {"limit": "10"}, {"page": float64(100001)}, {"cursor": ""}, {"cursor": " "}, {"cursor": float64(3)}}
	for _, p := range bad {
		if err := CheckPagingParams(p, sizes, cursors); err == nil {
			t.Fatalf("%v accepted", p)
		}
	}
}
