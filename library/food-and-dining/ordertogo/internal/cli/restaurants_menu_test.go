// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"testing"
)

func TestFilterMenuBySearch(t *testing.T) {
	menu := []map[string]any{
		{"id": 19001, "name": "Salmon", "upper_category": "Nigiri"},
		{"id": 19030, "name": "Salmon Roll 8PC", "upper_category": "Roll"},
		{"id": 19049, "name": "Spicy Tuna Roll 8PC", "upper_category": "Roll"},
		{"id": 18998, "name": "Sushi Combo 14PC", "upper_category": "Combo",
			"itemsubtitle": "Salmon, Seared Salmon, Tuna, Unagi"},
		{"id": 99999, "name": "Edamame", "upper_category": "Appetizer"},
	}
	raw, _ := json.Marshal(menu)

	cases := []struct {
		name      string
		query     string
		wantCount int
		wantIDs   []int
	}{
		{"empty query passthrough", "", 5, nil},
		{"whitespace query passthrough", "   ", 5, nil},
		{"single token", "salmon", 3, []int{19001, 19030, 18998}},
		{"two tokens narrows", "salmon nigiri", 1, []int{19001}},
		{"category token alone", "roll", 2, []int{19030, 19049}},
		{"two tokens both must hit", "spicy tuna roll", 1, []int{19049}},
		{"no match", "zzz-not-a-thing", 0, nil},
		{"itemsubtitle hit", "unagi", 1, []int{18998}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := filterMenuBySearch(raw, tc.query)
			var got []map[string]any
			if err := json.Unmarshal(out, &got); err != nil {
				t.Fatalf("output is not an array: %v", err)
			}
			if len(got) != tc.wantCount {
				t.Errorf("got %d records, want %d (records: %v)", len(got), tc.wantCount, got)
			}
			if tc.wantIDs == nil {
				return
			}
			gotIDs := make(map[float64]bool)
			for _, rec := range got {
				if id, ok := rec["id"].(float64); ok {
					gotIDs[id] = true
				}
			}
			for _, want := range tc.wantIDs {
				if !gotIDs[float64(want)] {
					t.Errorf("expected id %d in results, missing", want)
				}
			}
		})
	}
}

func TestFilterMenuBySearch_invalidJSON(t *testing.T) {
	bad := json.RawMessage(`{"not": "an array"}`)
	out := filterMenuBySearch(bad, "salmon")
	if string(out) != string(bad) {
		t.Errorf("non-array input should pass through unchanged, got %s", string(out))
	}
}
