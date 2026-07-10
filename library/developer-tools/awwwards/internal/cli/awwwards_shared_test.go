// Copyright 2026 and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import "testing"

func TestScoreColumn(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"", "score_overall", false},
		{"overall", "score_overall", false},
		{"Design", "score_design", false},
		{" usability ", "score_usability", false},
		{"creativity", "score_creativity", false},
		{"content", "score_content", false},
		{"design; DROP TABLE aw_sites", "", true},
		{"velocity", "", true},
	}
	for _, tt := range tests {
		got, err := scoreColumn(tt.in)
		if (err != nil) != tt.wantErr {
			t.Errorf("scoreColumn(%q) err = %v, wantErr %v", tt.in, err, tt.wantErr)
			continue
		}
		if got != tt.want {
			t.Errorf("scoreColumn(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestTopCounts(t *testing.T) {
	in := map[string]int{"b": 2, "a": 2, "c": 5, "d": 1}
	got := topCounts(in, 3)
	if len(got) != 3 {
		t.Fatalf("len = %d", len(got))
	}
	// Sorted by count desc, then name asc for ties.
	if got[0].Name != "c" || got[1].Name != "a" || got[2].Name != "b" {
		t.Errorf("order = %v", got)
	}
	if all := topCounts(in, 10); len(all) != 4 {
		t.Errorf("uncapped len = %d", len(all))
	}
	if none := topCounts(map[string]int{}, 5); len(none) != 0 {
		t.Errorf("empty len = %d", len(none))
	}
}
